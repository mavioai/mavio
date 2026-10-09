package streaming

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/planner"
)

func TestPlaylistGeneratorCases(t *testing.T) {
	portedCases(t, "playlist/dynamic_hls_playlist_generator.json", ported{
		run: map[string]func(*testing.T, args){
			"ComputeSegments_Valid_Success": func(t *testing.T, a args) {
				var kf struct {
					Args []json.RawMessage `json:"args"`
				}
				a.decode(t, "keyframeData", &kf)
				var raw []json.RawMessage
				if err := json.Unmarshal(kf.Args[1], &raw); err != nil {
					t.Fatal(err)
				}
				keyframes := make([]time.Duration, len(raw))
				for i, r := range raw {
					keyframes[i] = ticks(t, r)
				}
				var desired int
				a.decode(t, "desiredSegmentLengthMs", &desired)
				got := KeyframeSegments(keyframes, ticks(t, kf.Args[0]), time.Duration(desired)*time.Millisecond)
				if want := a.seconds(t, "segments"); !slices.Equal(got, want) {
					t.Errorf("KeyframeSegments() = %v, want = %v", got, want)
				}
			},
			"ComputeEqualLengthSegments_Valid_Success": func(t *testing.T, a args) {
				var desired int
				a.decode(t, "desiredSegmentLengthMs", &desired)
				got, err := EqualSegments(time.Duration(desired)*time.Millisecond, ticks(t, a["totalRuntimeTicks"]))
				if want := a.seconds(t, "segments"); err != nil || !slices.Equal(got, want) {
					t.Errorf("EqualSegments() = %v, %v, want = %v", got, err, want)
				}
			},
			"ComputeEqualLengthSegments_Invalid_ThrowsInvalidOperationException": func(t *testing.T, a args) {
				var desired int
				a.decode(t, "desiredSegmentLengthMs", &desired)
				if _, err := EqualSegments(time.Duration(desired)*time.Millisecond, ticks(t, a["totalRuntimeTicks"])); !errors.Is(err, ErrNoSegments) {
					t.Errorf("EqualSegments() error = %v, want = %v", err, ErrNoSegments)
				}
			},
			"IsExtractionAllowedForFile_Valid_Success": func(t *testing.T, a args) {
				var exts []string
				var want bool
				a.decode(t, "allowedExtensions", &exts)
				a.decode(t, "isAllowed", &want)
				if got := KeyframeExtractionAllowed(a.str(t, "filePath"), exts); got != want {
					t.Errorf("KeyframeExtractionAllowed() = %v, want = %v", got, want)
				}
			},
			"IsExtractionAllowedForFile_Invalid_ReturnsFalse": func(t *testing.T, a args) {
				var exts []string
				a.decode(t, "allowedExtensions", &exts)
				if KeyframeExtractionAllowed(a.str(t, "filePath"), exts) {
					t.Error("KeyframeExtractionAllowed() = true, want = false")
				}
			},
		},
		facts: map[string]string{
			"ComputeSegments_ZeroDurationOvershoot_ClampsToDuration":  "TestKeyframeSegmentsOvershoot",
			"ComputeSegments_MinorDurationOvershoot_ClampsToDuration": "TestKeyframeSegmentsOvershoot",
		},
	})
}

func TestKeyframeSegmentsOvershoot(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	for _, tt := range []struct {
		name      string
		duration  time.Duration
		keyframes []time.Duration
	}{
		{"zero duration", 0, []time.Duration{ms(10000)}},
		{"minor overshoot", ms(9900), []time.Duration{0, ms(5000), ms(10000)}},
	} {
		got := KeyframeSegments(tt.keyframes, tt.duration, ms(6000))
		if want := []time.Duration{ms(10000)}; !slices.Equal(got, want) {
			t.Errorf("%s: KeyframeSegments() = %v, want = %v", tt.name, got, want)
		}
	}
}

func TestDynamicHLSControllerCases(t *testing.T) {
	portedCases(t, "dynamic_hls_controller.json", ported{
		run: map[string]func(*testing.T, args){
			"GetDolbyVisionHevcCodecTag_SelectsTagForProfile": func(t *testing.T, a args) {
				var profile, compat *int
				a.decode(t, "profile", &profile)
				a.decode(t, "compatibilityId", &compat)
				st := &core.MediaStream{Kind: core.StreamVideo, Codec: "hevc", ColorTransfer: a.str(t, "transfer")}
				if profile != nil {
					st.DolbyVision = &core.DolbyVision{Profile: *profile, RPUPresent: true, BLPresent: true}
					if compat != nil {
						st.DolbyVision.BLCompatibilityID = *compat
					}
				}
				if got, want := planner.DolbyVisionHEVCTag(st), a.str(t, "expected"); got != want {
					t.Errorf("DolbyVisionHEVCTag() = %q, want = %q", got, want)
				}
			},
			// Jellyfin's transcoding playlists have no segments for an
			// unknown runtime, where EqualSegments reports none.
			"GetSegmentLengths_Success": func(t *testing.T, a args) {
				var length int
				a.decode(t, "segmentlength", &length)
				got, err := EqualSegments(time.Duration(length)*time.Second, ticks(t, a["runtimeTicks"]))
				if want := a.seconds(t, "expected"); !slices.Equal(got, want) || (err != nil && len(want) > 0) {
					t.Errorf("EqualSegments() = %v, %v, want = %v", got, err, want)
				}
			},
		},
		facts: map[string]string{
			"WaitForActiveTranscodingRequests_WaitsUntilRequestCompletes":    "TestStreamSerializesRequests",
			"WaitForActiveTranscodingRequests_WaitsForEveryRequest":          "TestStreamSerializesRequests",
			"WaitForActiveTranscodingRequests_ReturnsWithoutAnActiveRequest": "TestStreamServesCachedSegments",
			"WaitForActiveTranscodingRequests_ObservesCancellation":          "TestStreamRequestCanceled",
			"ActiveRequestCount_UpdatesAtomically":                           "TestStreamSerializesRequests",
		},
	})
}

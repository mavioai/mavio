package planner

import (
	"slices"
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/media/hwaccel"
)

// jfRangeTypes maps Jellyfin's range type names to Mavio's.
var jfRangeTypes = map[string]core.VideoRangeType{
	"SDR": core.RangeTypeSDR, "HDR10": core.RangeTypeHDR10, "DOVI": core.RangeTypeDOVI, "DOVIWithEL": core.RangeTypeDOVIWithEL,
}

// doviJob ports CreateState: a 1080p 10-bit Dolby Vision stream (profile 7
// for HEVC, 10 for AV1) whose BT.709 colors make the configuration invalid
// unless the test changes them.
func doviJob(codec, transfer string) *Job {
	profile, compat := 10, 1
	if codec == "hevc" {
		profile, compat = 7, 6
	}
	v := core.MediaStream{
		Kind: core.StreamVideo, Codec: codec, Width: 1920, Height: 1080, BitDepth: 10,
		DolbyVision:    &core.DolbyVision{Profile: profile, BLCompatibilityID: compat, RPUPresent: true, BLPresent: true},
		ColorSpace:     "bt709",
		ColorPrimaries: "bt709",
		ColorTransfer:  transfer,
	}
	src := &decision.Source{MediaSource: &core.MediaSource{Container: "mkv", Streams: []core.MediaStream{v}}}
	return &Job{Delivery: HLS, Source: src, Video: &src.Streams[0], VideoCodec: Copy, Request: &decision.Decision{}, AllowVideoCopy: true}
}

// requestRanges sets the range types the client accepts, given in
// Jellyfin's comma-separated names.
func requestRanges(j *Job, ranges string) {
	var names []string
	for r := range strings.SplitSeq(ranges, ",") {
		if r != "" {
			names = append(names, string(jfRangeTypes[r]))
		}
	}
	j.Request = &decision.Decision{}
	j.Request.SetOption("", "rangetype", strings.Join(names, ","))
}

func doviPlanner(removal bool) *Planner {
	caps := &hwaccel.Capabilities{
		Version:    hwaccel.NewVersion(8, 1),
		Filters:    []string{"tonemapx"},
		BSFOptions: map[hwaccel.BSFOption]bool{},
	}
	for _, o := range []hwaccel.BSFOption{
		hwaccel.HEVCMetadataRemoveDOVI, hwaccel.HEVCMetadataRemoveHDR10Plus,
		hwaccel.AV1MetadataRemoveDOVI, hwaccel.AV1MetadataRemoveHDR10Plus, hwaccel.DOVIRPUStrip,
	} {
		caps.BSFOptions[o] = removal
	}
	return &Planner{Options: DefaultOptions(), Caps: caps}
}

func transferArg(t *testing.T, a args) string {
	if a.null("transfer") {
		return ""
	}
	return a.str(t, "transfer")
}

func TestDoviCases(t *testing.T) {
	portedCases(t, "encoding_helper_dovi.json", ported{
		run: map[string]func(t *testing.T, a args){
			"GetSwVidFilterChain_InvalidDovi_OnlyTonemapsHdrBaseLayer": func(t *testing.T, a args) {
				transfer := transferArg(t, a)
				j := doviJob("hevc", transfer)
				j.VideoCodec = "h264"
				g := doviPlanner(true).SoftwareFilters(j, false)
				filters := g.Main.String()
				if rt := j.Video.VideoRangeType(); rt != core.RangeTypeDOVIInvalid {
					t.Fatalf("range type: got = %s, want = %s", rt, core.RangeTypeDOVIInvalid)
				}
				tonemap := a.boolean(t, "tonemap")
				if got := strings.Contains(filters, "tonemapx="); got != tonemap {
					t.Errorf("tonemap: got = %v, want = %v in %s", got, tonemap, filters)
				}
				want := "color_trc=bt709"
				if tonemap {
					want = "color_trc=" + transfer
				}
				if !strings.Contains(filters, want) {
					t.Errorf("got = %s, want it to contain %s", filters, want)
				}
			},
			"IsDoviWithHdr10Bl_InvalidDovi_RequiresPq": func(t *testing.T, a args) {
				v := doviJob("hevc", transferArg(t, a)).Video
				if !IsDOVI(v) {
					t.Error("IsDOVI: got = false")
				}
				if got, want := IsDOVIWithHDR10BL(v), a.boolean(t, "expected"); got != want {
					t.Errorf("got = %v, want = %v", got, want)
				}
			},
			"GetBitStreamArgs_InvalidDovi_PreservesClientDependentRemoval": func(t *testing.T, a args) {
				codec := a.str(t, "codec")
				j := doviJob(codec, transferArg(t, a))
				p := doviPlanner(true)
				for _, c := range []struct {
					ranges string
					remove bool
				}{{"", false}, {"SDR", false}, {"HDR10", false}, {"DOVIWithEL", false}, {"DOVI", true}, {"SDR,DOVI", true}} {
					requestRanges(j, c.ranges)
					if got := p.DOVIRemoved(j); got != c.remove {
						t.Errorf("%q: removed: got = %v, want = %v", c.ranges, got, c.remove)
					}
					args := joined(p.BitstreamArgs(j, core.StreamVideo))
					if c.remove {
						if !strings.Contains(args, a.str(t, "expected")) {
							t.Errorf("%q: got = %q, want %q", c.ranges, args, a.str(t, "expected"))
						}
					} else {
						want := ""
						if codec == "hevc" {
							want = "-bsf:v hevc_mp4toannexb"
						}
						if args != want {
							t.Errorf("%q: got = %q, want = %q", c.ranges, args, want)
						}
					}
					if doviPlanner(false).DOVIRemoved(j) {
						t.Errorf("%q: removed without support", c.ranges)
					}
				}
			},
			"CanStreamCopyVideo_InvalidDovi_RequiresRemovalSupportOnlyForDoviClients": func(t *testing.T, a args) {
				ranges := ""
				if !a.null("requestedRanges") {
					ranges = a.str(t, "requestedRanges")
				}
				for _, codec := range []string{"hevc", "av1"} {
					for _, transfer := range []string{"bt709", "smpte2084"} {
						j := doviJob(codec, transfer)
						requestRanges(j, ranges)
						if !doviPlanner(true).CanCopyVideo(j) {
							t.Errorf("%s/%s: copy with removal support: got = false", codec, transfer)
						}
						if got, want := doviPlanner(false).CanCopyVideo(j), a.boolean(t, "copyWithoutRemovalSupport"); got != want {
							t.Errorf("%s/%s: copy without removal support: got = %v, want = %v", codec, transfer, got, want)
						}
					}
				}
			},
		},
		facts: map[string]string{"GetBitStreamArgs_ValidDovi_PreservesMetadata": "TestValidDoviKeepsMetadata"},
	})
}

func TestValidDoviKeepsMetadata(t *testing.T) {
	j := doviJob("hevc", "smpte2084")
	j.Video.ColorSpace, j.Video.ColorPrimaries = "bt2020nc", "bt2020"
	requestRanges(j, "DOVIWithEL")
	p := doviPlanner(true)
	if p.DOVIRemoved(j) {
		t.Error("removed: got = true")
	}
	if got := p.BitstreamArgs(j, core.StreamVideo); !slices.Equal(got, []string{"-bsf:v", "hevc_mp4toannexb"}) {
		t.Errorf("got = %v", got)
	}
}

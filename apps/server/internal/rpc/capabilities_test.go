package rpc

import (
	"reflect"
	"strings"
	"testing"
	"unicode"

	"buf.build/go/protovalidate"

	"github.com/mavioai/mavio/libs/media/decision"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
)

// TestEnumsMapped checks that every enum value a validated request can
// carry has a decision counterpart.
func TestEnumsMapped(t *testing.T) {
	check := func(enum string, names map[int32]string, mapped func(int32) bool, unspecifiedAllowed bool) {
		for v, name := range names {
			if v == 0 && !unspecifiedAllowed {
				continue
			}
			if !mapped(v) {
				t.Errorf("%s %s has no decision counterpart", enum, name)
			}
		}
	}
	check("MediaKind", playbackv1.MediaKind_name, func(v int32) bool { _, ok := mediaKinds[playbackv1.MediaKind(v)]; return ok }, false)
	check("Context", playbackv1.Context_name, func(v int32) bool { _, ok := contexts[playbackv1.Context(v)]; return ok }, true)
	check("Protocol", playbackv1.Protocol_name, func(v int32) bool { _, ok := protocols[playbackv1.Protocol(v)]; return ok }, true)
	check("SeekInfo", playbackv1.SeekInfo_name, func(v int32) bool { _, ok := seekInfos[playbackv1.SeekInfo(v)]; return ok }, true)
	check("SubtitleMethod", playbackv1.SubtitleMethod_name, func(v int32) bool { _, ok := subtitleMethods[playbackv1.SubtitleMethod(v)]; return ok }, false)
	check("CodecKind", playbackv1.CodecKind_name, func(v int32) bool { _, ok := codecKinds[playbackv1.CodecKind(v)]; return ok }, false)
	check("Property", playbackv1.Property_name, func(v int32) bool { _, ok := properties[playbackv1.Property(v)]; return ok }, false)
	check("Op", playbackv1.Op_name, func(v int32) bool { _, ok := ops[playbackv1.Op(v)]; return ok }, false)

	// Properties keep their names.
	for p, d := range properties {
		if got, want := strings.TrimPrefix(p.String(), "PROPERTY_"), upperSnake(string(d)); got != want {
			t.Errorf("%v maps to %q", p, d)
		}
	}
}

// TestReasons checks that every transcode reason has the enum value of
// the same name.
func TestReasons(t *testing.T) {
	n := 0
	for i := range 32 {
		r := decision.Reasons(1 << i)
		if r.String() == "" {
			break
		}
		n++
		got := reasonsToProto(r)
		if want := "TRANSCODE_REASON_" + upperSnake(r.String()); len(got) != 1 || got[0].String() != want {
			t.Errorf("reasonsToProto(%v) = %v, want %s", r, got, want)
		}
	}
	if want := len(playbackv1.TranscodeReason_name) - 1; n != want {
		t.Errorf("decision has %d reasons, the contract %d", n, want)
	}
	if got := reasonsToProto(decision.VideoCodecNotSupported | decision.AudioIsExternal); len(got) != 2 ||
		got[0] != playbackv1.TranscodeReason_TRANSCODE_REASON_VIDEO_CODEC_NOT_SUPPORTED ||
		got[1] != playbackv1.TranscodeReason_TRANSCODE_REASON_AUDIO_IS_EXTERNAL {
		t.Errorf("reasonsToProto(two) = %v", got)
	}
}

func upperSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

func TestCapabilitiesFromProto(t *testing.T) {
	video := playbackv1.MediaKind_MEDIA_KIND_VIDEO
	in := playbackv1.ClientCapabilities_builder{
		Name:                new("hls.js"),
		MaxStreamingBitrate: new(int64(20_000_000)),
		DirectPlay: []*playbackv1.DirectPlayProfile{playbackv1.DirectPlayProfile_builder{
			Kind: &video, Container: new("mp4,m4v"), VideoCodec: new("h264,hevc"), AudioCodec: new("aac,mp3"),
		}.Build()},
		Transcoding: []*playbackv1.TranscodingProfile{playbackv1.TranscodingProfile_builder{
			Kind: &video, Protocol: new(playbackv1.Protocol_PROTOCOL_HLS), Container: new("mp4"),
			VideoCodec: new("h264"), AudioCodec: new("aac"), MaxAudioChannels: new(int32(2)),
			SegmentLengthSeconds: new(int32(6)), MinSegments: new(int32(1)), EnableSubtitlesInManifest: new(true),
			Conditions: []*playbackv1.Condition{playbackv1.Condition_builder{
				Property: new(playbackv1.Property_PROPERTY_WIDTH), Op: new(playbackv1.Op_OP_LESS_THAN_EQUAL), Value: new("1920"),
			}.Build()},
		}.Build()},
		Codecs: []*playbackv1.CodecProfile{playbackv1.CodecProfile_builder{
			Kind: new(playbackv1.CodecKind_CODEC_KIND_VIDEO), Codec: new("h264"),
			Conditions: []*playbackv1.Condition{playbackv1.Condition_builder{
				Property: new(playbackv1.Property_PROPERTY_VIDEO_RANGE_TYPE), Op: new(playbackv1.Op_OP_EQUALS_ANY), Value: new("sdr"), Optional: new(true),
			}.Build()},
		}.Build()},
		Subtitles: []*playbackv1.SubtitleProfile{playbackv1.SubtitleProfile_builder{
			Format: new("vtt"), Method: new(playbackv1.SubtitleMethod_SUBTITLE_METHOD_EXTERNAL),
		}.Build()},
	}.Build()
	v, err := protovalidate.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Validate(in); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	want := &decision.ClientCapabilities{
		Name:                "hls.js",
		MaxStreamingBitrate: 20_000_000,
		DirectPlay:          []decision.DirectPlayProfile{{Kind: decision.Video, Container: "mp4,m4v", VideoCodec: "h264,hevc", AudioCodec: "aac,mp3"}},
		Transcoding: []decision.TranscodingProfile{{
			Kind: decision.Video, Context: decision.Streaming, Protocol: decision.HLS, Container: "mp4",
			VideoCodec: "h264", AudioCodec: "aac", MaxAudioChannels: 2, SeekInfo: decision.SeekAuto,
			EnableSubtitlesInManifest: true, MinSegments: 1, SegmentLength: 6,
			Conditions: []decision.Condition{{Property: decision.Width, Op: decision.LessThanEqual, Value: "1920"}},
		}},
		Codecs: []decision.CodecProfile{{
			Kind: decision.CodecVideo, Codec: "h264",
			Conditions: []decision.Condition{{Property: decision.VideoRangeType, Op: decision.EqualsAny, Value: "sdr", Optional: true}},
		}},
		Subtitles: []decision.SubtitleProfile{{Format: "vtt", Method: decision.SubtitleExternal}},
	}
	if got := capabilitiesFromProto(in); !reflect.DeepEqual(got, want) {
		t.Errorf("capabilitiesFromProto =\n%+v\nwant\n%+v", got, want)
	}

	// Profiles must name their kind.
	bad := playbackv1.ClientCapabilities_builder{
		DirectPlay: []*playbackv1.DirectPlayProfile{playbackv1.DirectPlayProfile_builder{Container: new("mp4")}.Build()},
	}.Build()
	if err := v.Validate(bad); err == nil {
		t.Error("Validate(profile without kind) = nil, want an error")
	}
}

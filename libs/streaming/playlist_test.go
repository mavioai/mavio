package streaming

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/planner"
)

func TestMediaPlaylist(t *testing.T) {
	uri := func(i int) string {
		if i < 0 {
			return "init.mp4?t=x"
		}
		return strconv.Itoa(i) + ".mp4?t=x"
	}
	var b strings.Builder
	segments := []time.Duration{6 * time.Second, 6 * time.Second, 2123 * time.Millisecond}
	if _, err := (MediaPlaylist{Segments: segments, Container: "mp4", URI: uri}).WriteTo(&b); err != nil {
		t.Fatal(err)
	}
	want := `#EXTM3U
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-MAP:URI="init.mp4?t=x"
#EXTINF:6.000000,
0.mp4?t=x
#EXTINF:6.000000,
1.mp4?t=x
#EXTINF:2.123000,
2.mp4?t=x
#EXT-X-ENDLIST
`
	if got := b.String(); got != want {
		t.Errorf("playlist = %q, want = %q", got, want)
	}

	// MPEG-TS needs no initialization segment; the target duration rounds
	// up.
	b.Reset()
	if _, err := (MediaPlaylist{Segments: []time.Duration{6500 * time.Millisecond}, Container: "ts", URI: uri}).WriteTo(&b); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); !strings.Contains(got, "#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:7\n") || strings.Contains(got, "EXT-X-MAP") {
		t.Errorf("ts playlist = %q", got)
	}
}

func TestMasterPlaylist(t *testing.T) {
	var b strings.Builder
	p := MasterPlaylist{
		Subtitles: []Subtitle{{Group: "subs", Name: "English", Language: "en", URI: "subs/2.m3u8", Default: true}},
		Variants: []Variant{{
			URI: "main.m3u8", Bandwidth: 8_192_000, Codecs: []string{"avc1.640028", "mp4a.40.2"},
			Width: 1920, Height: 1080, FrameRate: 23.976, VideoRange: "SDR", Subtitles: "subs",
		}},
	}
	if _, err := p.WriteTo(&b); err != nil {
		t.Fatal(err)
	}
	want := `#EXTM3U
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="English",LANGUAGE="en",DEFAULT=YES,AUTOSELECT=YES,FORCED=NO,URI="subs/2.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=8192000,AVERAGE-BANDWIDTH=8192000,VIDEO-RANGE=SDR,CODECS="avc1.640028,mp4a.40.2",RESOLUTION=1920x1080,FRAME-RATE=23.976,SUBTITLES="subs"
main.m3u8
`
	if got := b.String(); got != want {
		t.Errorf("playlist = %q, want = %q", got, want)
	}
}

func TestCodecStrings(t *testing.T) {
	for _, tt := range []struct{ got, want string }{
		{H264Codec("High", 41), "avc1.640029"},
		{H264Codec("main", 40), "avc1.4D4028"},
		{H264Codec("baseline", 30), "avc1.42E01E"},
		{H264Codec("", 31), "avc1.42401F"},
		{HEVCCodec("Main 10", 150), "hvc1.2.4.L150.B0"},
		{HEVCCodec("main", 120), "hvc1.1.4.L120.B0"},
		{AV1Codec("Main", 8, false, 10), "av01.0.08M.10"},
		{AV1Codec("High", 0, true, 7), "av01.1.19H.08"},
		{VP9Codec(1920, 1080, "yuv420p", 24, 8), "vp09.00.40.08"},
		{VP9Codec(3840, 2160, "yuv420p10le", 60, 10), "vp09.02.51.10"},
		{DolbyVisionCodec(8, 6, "hevc"), "dvh1.08.06"},
		{DolbyVisionCodec(10, 9, "av1"), "dav1.10.09"},
		{AudioCodec("aac", "HE-AAC"), "mp4a.40.5"},
		{AudioCodec("aac", "LC"), "mp4a.40.2"},
		{AudioCodec("eac3", ""), "ec-3"},
		{AudioCodec("dts", "DTS-HD MA"), "dtsh"},
		{AudioCodec("dts", ""), "dtsc"},
		{AudioCodec("pcm_s16le", ""), ""},
	} {
		if tt.got != tt.want {
			t.Errorf("codec string = %q, want = %q", tt.got, tt.want)
		}
	}
}

func TestVariantOf(t *testing.T) {
	transcoded := planner.Output{
		VideoCodec: "h264", VideoProfile: "high", VideoLevel: 41, BitDepth: 8, Width: 1280, Height: 720, FrameRate: 25,
		Range: core.RangeSDR, AudioCodec: "aac", AudioProfile: "LC", VideoBitrate: 4_000_000, AudioBitrate: 192_000,
	}
	v := VariantOf(transcoded, 20_000_000, "yuv420p")
	if v.Bandwidth != 4_192_000 || strings.Join(v.Codecs, ",") != "avc1.640029,mp4a.40.2" || v.VideoRange != "SDR" || v.SupplementalCodecs != "" {
		t.Errorf("transcoded variant = %+v", v)
	}

	dv := planner.Output{
		VideoCodec: "hevc", VideoProfile: "Main 10", VideoLevel: 153, BitDepth: 10, Width: 3840, Height: 2160,
		Range: core.RangeHDR, RangeType: core.RangeTypeDOVIWithHDR10, DolbyVision: &core.DolbyVision{Profile: 8, Level: 6},
		AudioCodec: "eac3", VideoCopied: true, AudioCopied: true,
	}
	v = VariantOf(dv, 50_000_000, "yuv420p10le")
	if v.Bandwidth != 50_000_000 || strings.Join(v.Codecs, ",") != "hvc1.2.4.L153.B0,ec-3" || v.VideoRange != "PQ" ||
		v.SupplementalCodecs != "dvh1.08.06/db1p" {
		t.Errorf("Dolby Vision variant = %+v", v)
	}

	hlg := planner.Output{VideoCodec: "hevc", VideoProfile: "Main 10", VideoLevel: 150, Range: core.RangeHDR, RangeType: core.RangeTypeHLG}
	if v := VariantOf(hlg, 1, ""); v.VideoRange != "HLG" {
		t.Errorf("HLG variant = %+v", v)
	}
	hdr10Plus := planner.Output{VideoCodec: "hevc", VideoProfile: "Main 10", VideoLevel: 150, Range: core.RangeHDR, RangeType: core.RangeTypeHDR10Plus, HDR10Plus: true}
	if v := VariantOf(hdr10Plus, 1, ""); v.SupplementalCodecs != "hvc1.2.4.L150.B0/cdm4" {
		t.Errorf("HDR10+ variant = %+v", v)
	}
}

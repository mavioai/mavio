package organize

import (
	"strings"
	"testing"
)

// Ported from Radarr's FormatAudioCodecFixture; each pack is "format,
// codec ID, profile". The scene names do not affect audio codecs.
func TestFormatAudioCodec(t *testing.T) {
	cases := []struct{ pack, sceneName, want string }{
		{"mp2, ,  ", "droned.s01e03.swedish.720p.hdtv.x264-prince", "MP2"},
		{"vorbis, ,  ", "DB Super HDTV", "Vorbis"},
		{"pcm_s16le, ,  ", "DW DVDRip XviD-idTV, ", "PCM"},
		{"truehd, ,  ", "", "TrueHD"},
		{"truehd, ,  ", "TrueHD", "TrueHD"},
		{"truehd, thd+,  ", "Atmos", "TrueHD Atmos"},
		{"truehd, thd+,  ", "TrueHD.Atmos.7.1", "TrueHD Atmos"},
		{"truehd, thd+,  ", "", "TrueHD Atmos"},
		{"wmav1, , ", "Droned.wmv", "WMA"},
		{"wmav2, , ", "B.N.S04E18.720p.WEB-DL", "WMA"},
		{"opus, ,  ", "Roadkill Ep3x11 - YouTube.webm", "Opus"},
		{"mp3, ,  ", "climbing.mp4", "MP3"},
		{"dts, , DTS-HD MA", "DTS-HD.MA", "DTS-HD MA"},
		{"dts, , DTS:X", "DTS-X", "DTS-X"},
		{"dts, , DTS-HD MA", "DTS-HD.MA", "DTS-HD MA"},
		{"dts, , DTS-ES", "DTS-ES", "DTS-ES"},
		{"dts, , DTS-ES", "DTS", "DTS-ES"},
		{"dts, , DTS-HD HRA", "DTSHD-HRA", "DTS-HD HRA"},
		{"dts, , DTS", "DTS", "DTS"},
		{"eac3, ec+3,  ", "EAC3.Atmos", "EAC3 Atmos"},
		{"eac3, ,  ", "DDP5.1", "EAC3"},
		{"ac3, ,  ", "DD5.1", "AC3"},
		{"adpcm_ima_qt, ,  ", "Custom?", "PCM"},
		{"adpcm_ms, ,  ", "Custom", "PCM"},
		// should_return_AudioFormat_by_default
		{"Other Audio Format, Other Audio Codec, ", "My.Series.S01E01-Sonarr", "Other Audio Format"},
	}
	for _, c := range cases {
		p := strings.Split(c.pack, ", ")
		if got := FormatAudioCodec(p[0], p[1], p[2]); got != c.want {
			t.Errorf("FormatAudioCodec(%q) = %q, want = %q", c.pack, got, c.want)
		}
	}
}

// Ported from Radarr's FormatVideoCodecFixture; each pack is "format,
// codec ID".
func TestFormatVideoCodec(t *testing.T) {
	cases := []struct{ pack, sceneName, want string }{
		{"mpeg2video, ", "Droned.S01E02.1080i.HDTV.DD5.1.MPEG2-NTb", "MPEG2"},
		{"mpeg2video, ", "", "MPEG2"},
		{"mpeg1video, ", "The.Series.S13E04.INTERNAL-ANiVCD.mpg", "MPEG"},
		{"vc1, WVC1", "T.N.S04E18.720p.WEB-DL", "VC1"},
		{"vc1, V_MS/VFW/FOURCC/WVC1", "", "VC1"},
		{"vc1, WMV3", "Series Title S09E13 The Radarr's RevengeHDTV.XviD-2HD.avi", "VC1"},
		{"h264, V.MPEG4/ISO/AVC", "Series.2015.S03E08.720p.iP.WEBRip.AAC2.0.H264-BTW", "h264"},
		{"h264, V_MPEG4/ISO/AVC", "Serie.2019.S01E03.1080p.RTE.WEB-DL.AAC2.0.x264-RTN", "x264"},
		{"wmv1, WMV1", "Droned.wmv", "WMV"},
		{"wmv2, WMV2", "Droned.wmv", "WMV"},
		{"mpeg4, XVID", "", "XviD"},
		{"mpeg4, DIVX", "", "DivX"},
		{"mpeg4, divx", "", "DivX"},
		{"mpeg4, DIV3", "spsm.dvdrip.divx.avi'.", "DivX"},
		{"msmpeg4, DIV3", "Movie the Title (1976) 360p MPEG Audio.avi", "DivX"},
		{"msmpeg4v2, DIV3", "Movie the Title (1976) 360p MPEG Audio.avi", "DivX"},
		{"msmpeg4v3, DIV3", "Movie the Title (1976) 360p MPEG Audio.avi", "DivX"},
		{"vp6f, 4", "Movie the Title (1976) 360p MPEG Audio.flv", "VP6"},
		{"vp6, 4", "Top Gear - S12E01 - Lorries - SD TV.flv", "VP6"},
		{"vp7, VP70", "Movie the Title.avi", "VP7"},
		{"vp8, V_VP8", "Movie the Title.mkv", "VP8"},
		{"vp9, V_VP9", "Series the Title Ep3x11 - YouTube.webm", "VP9"},
		{"h264, x264", "Series the Title - S04E05 - Stanley Hotel SDTV.avi", "x264"},
		{"hevc, V_MPEGH/ISO/HEVC", "Series the Title S11E12 The Radarr Metric 1080p 10bit AMZN WEB-DL", "h265"},
		{"mpeg4, mp4v-20", "", ""},
		{"mpeg4, XVID", "Series the Title.S06E07.Mountain.Radarr.DSR.XviD-KRS", "XviD"},
		{"rpza, V_QUICKTIME", "Custom", ""},
		{"mpeg4, FMP4", "", ""},
		{"mpeg4, MP42", "", ""},
		{"mpeg4, mp43", "Series the Title.S01E13.480p.WEB-DL.H.264-BTN-Custom", ""},
		// should_format_video_format_fallbacks
		{"h264, x264", "Some.Video.S01E01.h264", "x264"},
		{"hevc, x265", "Some.Video.S01E01.h265", "x265"},
		{"h264, ", "Some.Video.S01E01.x264", "x264"},
		{"hevc, ", "Some.Video.S01E01.x265", "x265"},
		{"h264, ", "Some.Video.S01E01", "h264"},
		{"hevc, ", "Some.Video.S01E01", "h265"},
		// should_return_VideoFormat_by_default
		{"VideoCodec, ", "", "VideoCodec"},
	}
	for _, c := range cases {
		p := strings.Split(c.pack, ", ")
		if got := FormatVideoCodec(p[0], p[1], c.sceneName); got != c.want {
			t.Errorf("FormatVideoCodec(%q, %q) = %q, want = %q", c.pack, c.sceneName, got, c.want)
		}
	}
}

// Ported from Radarr's FormatVideoDynamicRangeFixture, with each HdrFormat
// given as the dynamic range type Radarr formats it as.
func TestVideoDynamicRange(t *testing.T) {
	cases := []struct{ hdrType, want string }{
		{"", ""},       // None
		{"HLG", "HDR"}, // Hlg10
		{"PQ", "HDR"},  // Pq10
		{"HDR10", "HDR"},
		{"HDR10Plus", "HDR"},
		{"DV", "HDR"}, // DolbyVision
	}
	for _, c := range cases {
		if got := (MediaInfo{VideoDynamicRangeType: c.hdrType}).VideoDynamicRange(); got != c.want {
			t.Errorf("VideoDynamicRange(%q) = %q, want = %q", c.hdrType, got, c.want)
		}
	}
}

// Radarr's FormatVideoDynamicRangeTypeFixture is not ported: it maps
// Radarr's HdrFormat enumeration to strings, and MediaInfo takes the
// formatted string.
func TestVideoDynamicRangeTypeSkipped(t *testing.T) {
	t.Skip("FormatVideoDynamicRangeTypeFixture (10 cases): MediaInfo.VideoDynamicRangeType is the formatted string; there is no HdrFormat enumeration to map")
}

func TestFormatAudioChannels(t *testing.T) {
	cases := []struct {
		positions string
		channels  float64
		want      float64
	}{
		{"5.1(side)", 6, 5.1},
		{"7.1", 8, 7.1},
		{"stereo", 2, 2},
		{"", 6, 6},
		{"0.0", 1, 1},
	}
	for _, c := range cases {
		if got := FormatAudioChannels(c.positions, c.channels); got != c.want {
			t.Errorf("FormatAudioChannels(%q, %v) = %v, want = %v", c.positions, c.channels, got, c.want)
		}
	}
}

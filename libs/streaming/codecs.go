package streaming

import (
	"fmt"
	"strings"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/planner"
)

// RFC 6381 codec strings of HLS master playlists, as Jellyfin writes them.

// H264Codec returns the codec string of H.264 at a profile and level (41
// for 4.1); unknown profiles are taken as constrained baseline.
func H264Codec(profile string, level int) string {
	p := "4240"
	switch strings.ToLower(strings.ReplaceAll(profile, " ", "")) {
	case "high":
		p = "6400"
	case "main":
		p = "4D40"
	case "baseline":
		p = "42E0"
	}
	return fmt.Sprintf("avc1.%s%02X", p, level)
}

// HEVCCodec returns the codec string of HEVC at a profile and level (120
// for 4.0).
func HEVCCodec(profile string, level int) string {
	p := "1.4"
	switch strings.ToLower(profile) {
	case "main10", "main 10":
		p = "2.4"
	}
	return fmt.Sprintf("hvc1.%s.L%d.B0", p, level)
}

// AV1Codec returns the codec string of AV1; levels outside 1–31 are taken
// as 6.3, bit depths other than 8, 10 and 12 as 8.
func AV1Codec(profile string, level int, highTier bool, bitDepth int) string {
	p := 0
	switch strings.ToLower(profile) {
	case "high":
		p = 1
	case "professional":
		p = 2
	}
	if level <= 0 || level > 31 {
		level = 19
	}
	tier := "M"
	if highTier {
		tier = "H"
	}
	return fmt.Sprintf("av01.%d.%02d%s.%02d", p, level, tier, validBitDepth(bitDepth))
}

// VP9Codec returns the codec string of VP9 from the picture size, pixel
// format, frame rate and bit depth.
func VP9Codec(width, height int, pixelFormat string, frameRate float64, bitDepth int) string {
	profile := "00"
	switch pixelFormat {
	case "yuv422p", "yuv444p":
		profile = "01"
	case "yuv420p10le", "yuv420p12le":
		profile = "02"
	case "yuv422p10le", "yuv422p12le", "yuv444p10le", "yuv444p12le":
		profile = "03"
	}
	luma := width * height
	rate := float64(luma) * frameRate
	level := "00"
	switch {
	case luma <= 0:
	case luma <= 36864:
		level = "10"
	case luma <= 73728:
		level = "11"
	case luma <= 122880:
		level = "20"
	case luma <= 245760:
		level = "21"
	case luma <= 552960:
		level = "30"
	case luma <= 983040:
		level = "31"
	case luma <= 2228224:
		level = pick(rate, []float64{83558400}, "40", "41")
	case luma <= 8912896:
		level = pick(rate, []float64{311951360, 588251136}, "50", "51", "52")
	case luma <= 35651584:
		level = pick(rate, []float64{1176502272, 4706009088}, "60", "61", "62")
	}
	return fmt.Sprintf("vp09.%s.%s.%02d", profile, level, validBitDepth(bitDepth))
}

// pick returns the first level whose bound the sample rate stays within,
// and the last otherwise.
func pick(rate float64, bounds []float64, levels ...string) string {
	for i, b := range bounds {
		if rate <= b {
			return levels[i]
		}
	}
	return levels[len(levels)-1]
}

func validBitDepth(d int) int {
	if d != 8 && d != 10 && d != 12 {
		return 8
	}
	return d
}

// DolbyVisionCodec returns the codec string of Dolby Vision on HEVC
// (dvh1) or AV1 (dav1).
func DolbyVisionCodec(profile, level int, codec string) string {
	fourCC := "dvh1"
	if strings.EqualFold(codec, "av1") {
		fourCC = "dav1"
	}
	return fmt.Sprintf("%s.%02d.%02d", fourCC, profile, level)
}

// AudioCodec returns the codec string of an audio codec, with the profile
// telling HE-AAC and the DTS variants apart; "" for codecs HLS lacks.
func AudioCodec(codec, profile string) string {
	switch strings.ToLower(codec) {
	case "aac":
		if strings.EqualFold(profile, "HE-AAC") {
			return "mp4a.40.5"
		}
		return "mp4a.40.2"
	case "mp3":
		return "mp4a.40.34"
	case "ac3":
		return "ac-3"
	case "eac3":
		return "ec-3"
	case "flac":
		return "fLaC"
	case "alac":
		return "alac"
	case "opus":
		return "Opus"
	case "truehd":
		return "mlpa"
	case "dts":
		switch strings.ToLower(profile) {
		case "dts-hd hra", "dts-hd ma", "dts-hd ma + dts:x", "dts-hd ma + dts:x imax":
			return "dtsh"
		case "dts express":
			return "dtse"
		}
		return "dtsc"
	}
	return ""
}

// VariantOf describes a job's output as a master playlist variant.
// Bandwidth falls back to the source's bit rate when the output's is
// unknown. Dolby Vision kept on an HDR10 or HLG base layer and HDR10+ are
// announced as supplemental codecs, which clients without them ignore.
func VariantOf(o planner.Output, sourceBitrate int, pixelFormat string) Variant {
	v := Variant{Width: o.Width, Height: o.Height, FrameRate: o.FrameRate}
	v.Bandwidth = o.VideoBitrate + o.AudioBitrate
	if o.VideoBitrate == 0 {
		v.Bandwidth = max(v.Bandwidth, sourceBitrate)
	}
	video := ""
	switch o.VideoCodec {
	case "h264":
		video = H264Codec(o.VideoProfile, o.VideoLevel)
	case "hevc":
		video = HEVCCodec(o.VideoProfile, o.VideoLevel)
	case "av1":
		video = AV1Codec(o.VideoProfile, o.VideoLevel, false, o.BitDepth)
	case "vp9":
		video = VP9Codec(o.Width, o.Height, pixelFormat, o.FrameRate, o.BitDepth)
	}
	if o.VideoCodec != "" {
		v.VideoRange = "SDR"
		if o.Range == core.RangeHDR {
			v.VideoRange = "PQ"
			if o.RangeType == core.RangeTypeHLG || o.RangeType == core.RangeTypeDOVIWithHLG {
				v.VideoRange = "HLG"
			}
		}
	}
	if (o.VideoCodec == "h264" || o.VideoCodec == "hevc" || o.VideoCodec == "av1") && o.VideoLevel == 0 {
		// Without a level the string would be invalid.
		video = ""
	}
	if video != "" {
		v.Codecs = append(v.Codecs, video)
	}
	if a := AudioCodec(o.AudioCodec, o.AudioProfile); a != "" {
		v.Codecs = append(v.Codecs, a)
	}
	switch dv := o.DolbyVision; {
	case dv != nil:
		layer := map[core.VideoRangeType]string{
			core.RangeTypeDOVIWithHDR10: "db1p", core.RangeTypeDOVIWithHLG: "db4h", core.RangeTypeDOVIWithHDR10Plus: "db1p",
		}[o.RangeType]
		if layer != "" {
			v.SupplementalCodecs = DolbyVisionCodec(dv.Profile, dv.Level, o.VideoCodec) + "/" + layer
		}
	case o.HDR10Plus && video != "":
		v.SupplementalCodecs = video + "/cdm4"
	}
	return v
}

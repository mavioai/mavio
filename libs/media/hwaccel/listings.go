package hwaccel

import (
	"regexp"
	"slices"
	"strings"
)

// The codecs and filters the planner can use; ffmpeg listings are reduced
// to these.
var (
	knownDecoders = []string{
		"h264", "hevc", "vp8", "libvpx", "vp9", "libvpx-vp9", "av1", "libdav1d",
		"mpeg2video", "mpeg4", "msmpeg4", "dca", "ac3", "ac4", "aac", "mp3", "flac", "truehd",
		"h264_qsv", "hevc_qsv", "mpeg2_qsv", "vc1_qsv", "vp8_qsv", "vp9_qsv", "av1_qsv",
		"h264_cuvid", "hevc_cuvid", "mpeg2_cuvid", "vc1_cuvid", "mpeg4_cuvid", "vp8_cuvid", "vp9_cuvid", "av1_cuvid",
		"h264_rkmpp", "hevc_rkmpp", "mpeg1_rkmpp", "mpeg2_rkmpp", "mpeg4_rkmpp", "vp8_rkmpp", "vp9_rkmpp", "av1_rkmpp",
	}
	knownEncoders = []string{
		"libx264", "libx265", "libsvtav1",
		"aac", "aac_at", "libfdk_aac", "ac3", "alac", "dca", "libmp3lame", "libopus", "libvorbis", "flac", "truehd", "srt",
		"h264_amf", "hevc_amf", "av1_amf",
		"h264_qsv", "hevc_qsv", "mjpeg_qsv", "av1_qsv",
		"h264_nvenc", "hevc_nvenc", "av1_nvenc",
		"h264_vaapi", "hevc_vaapi", "av1_vaapi", "mjpeg_vaapi",
		"h264_v4l2m2m",
		"h264_videotoolbox", "hevc_videotoolbox", "mjpeg_videotoolbox",
		"h264_rkmpp", "hevc_rkmpp", "mjpeg_rkmpp",
	}
	knownFilters = []string{
		// software
		"alphasrc", "zscale", "tonemapx",
		// qsv
		"scale_qsv", "vpp_qsv", "deinterlace_qsv", "overlay_qsv",
		// cuda
		"scale_cuda", "yadif_cuda", "bwdif_cuda", "tonemap_cuda", "overlay_cuda", "transpose_cuda", "hwupload_cuda",
		// opencl
		"scale_opencl", "tonemap_opencl", "overlay_opencl", "transpose_opencl", "yadif_opencl", "bwdif_opencl",
		// vaapi
		"scale_vaapi", "deinterlace_vaapi", "tonemap_vaapi", "procamp_vaapi", "overlay_vaapi", "transpose_vaapi", "hwupload_vaapi",
		// vulkan
		"libplacebo", "scale_vulkan", "overlay_vulkan", "transpose_vulkan", "flip_vulkan",
		// videotoolbox
		"yadif_videotoolbox", "bwdif_videotoolbox", "scale_vt", "transpose_vt", "overlay_videotoolbox", "tonemap_videotoolbox",
		// rkrga
		"scale_rkrga", "vpp_rkrga", "overlay_rkrga",
	}
)

// FilterOption is a filter option whose presence the planner depends on.
type FilterOption string

// Filter options probed with "ffmpeg -h filter=<name>".
const (
	ScaleCUDAFormat          FilterOption = "scale_cuda_format"
	TonemapCUDAName          FilterOption = "tonemap_cuda_name"
	TonemapOpenCLBT2390      FilterOption = "tonemap_opencl_bt2390"
	OverlayOpenCLFrameSync   FilterOption = "overlay_opencl_framesync"
	OverlayVAAPIFrameSync    FilterOption = "overlay_vaapi_framesync"
	OverlayVulkanFrameSync   FilterOption = "overlay_vulkan_framesync"
	TransposeOpenCLReversal  FilterOption = "transpose_opencl_reversal"
	OverlayOpenCLAlphaFormat FilterOption = "overlay_opencl_alpha_format"
	OverlayCUDAAlphaFormat   FilterOption = "overlay_cuda_alpha_format"
)

// BSFOption is a bitstream filter option whose presence the planner depends
// on.
type BSFOption string

// Bitstream filter options probed with "ffmpeg -h bsf=<name>".
const (
	HEVCMetadataRemoveDOVI      BSFOption = "hevc_metadata_remove_dovi"
	HEVCMetadataRemoveHDR10Plus BSFOption = "hevc_metadata_remove_hdr10plus"
	AV1MetadataRemoveDOVI       BSFOption = "av1_metadata_remove_dovi"
	AV1MetadataRemoveHDR10Plus  BSFOption = "av1_metadata_remove_hdr10plus"
	DOVIRPUStrip                BSFOption = "dovi_rpu_strip"
)

// optionProbe names a filter and the text its help must contain.
type optionProbe struct{ filter, text string }

var (
	filterOptionProbes = map[FilterOption]optionProbe{
		ScaleCUDAFormat:          {"scale_cuda", "format"},
		TonemapCUDAName:          {"tonemap_cuda", "GPU accelerated HDR to SDR tonemapping"},
		TonemapOpenCLBT2390:      {"tonemap_opencl", "bt2390"},
		OverlayOpenCLFrameSync:   {"overlay_opencl", "Action to take when encountering EOF from secondary input"},
		OverlayVAAPIFrameSync:    {"overlay_vaapi", "Action to take when encountering EOF from secondary input"},
		OverlayVulkanFrameSync:   {"overlay_vulkan", "Action to take when encountering EOF from secondary input"},
		TransposeOpenCLReversal:  {"transpose_opencl", "rotate by half-turn"},
		OverlayOpenCLAlphaFormat: {"overlay_opencl", "alpha_format"},
		OverlayCUDAAlphaFormat:   {"overlay_cuda", "alpha_format"},
	}
	bsfOptionProbes = map[BSFOption]optionProbe{
		HEVCMetadataRemoveDOVI:      {"hevc_metadata", "remove_dovi"},
		HEVCMetadataRemoveHDR10Plus: {"hevc_metadata", "remove_hdr10plus"},
		AV1MetadataRemoveDOVI:       {"av1_metadata", "remove_dovi"},
		AV1MetadataRemoveHDR10Plus:  {"av1_metadata", "remove_hdr10plus"},
		DOVIRPUStrip:                {"dovi_rpu", "strip"},
	}
)

var (
	codecLine  = regexp.MustCompile(`(?m)^\s\S{6}\s([\w|-]+)\s+.+$`)
	filterLine = regexp.MustCompile(`(?m)^\s\S{2,3}\s([\w|-]+)\s+.+$`)
)

// ParseCodecs returns the known codecs listed in the output of
// "ffmpeg -encoders" or "ffmpeg -decoders", in listing order.
func ParseCodecs(output string, encoders bool) []string {
	known := knownDecoders
	if encoders {
		known = knownEncoders
	}
	return listed(codecLine, output, known)
}

// ParseFilters returns the known filters listed in the output of
// "ffmpeg -filters", in listing order.
func ParseFilters(output string) []string {
	return listed(filterLine, output, knownFilters)
}

func listed(line *regexp.Regexp, output string, known []string) []string {
	var found []string
	for _, m := range line.FindAllStringSubmatch(output, -1) {
		if slices.Contains(known, m[1]) {
			found = append(found, m[1])
		}
	}
	return found
}

// ParseHwaccels returns the methods listed in the output of
// "ffmpeg -hwaccels", which follow a heading line.
func ParseHwaccels(output string) []string {
	var found []string
	lines := strings.FieldsFunc(output, func(r rune) bool { return r == '\r' || r == '\n' })
	for i, l := range lines {
		if l = strings.TrimSpace(l); i > 0 && l != "" && !slices.Contains(found, l) {
			found = append(found, l)
		}
	}
	return found
}

// hasOption reports whether the help text of a filter ("Filter <name>") or
// bitstream filter ("Bit stream filter <name>") mentions text.
func hasOption(help, heading string, p optionProbe) bool {
	return strings.Contains(help, heading+" "+p.filter) && strings.Contains(help, p.text)
}

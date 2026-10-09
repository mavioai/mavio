package planner

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// VideoEncoder returns the ffmpeg encoder for the job's video codec: the
// hardware encoder of the configured acceleration when the build has it,
// else the software one.
func (p *Planner) VideoEncoder(j *Job) string {
	codec := strings.ToLower(j.VideoCodec)
	var sw, hw string
	switch codec {
	case "", Copy:
		return Copy
	case "h264":
		sw, hw = "libx264", "h264"
	case "h265", "hevc":
		sw, hw = "libx265", "hevc"
	case "av1":
		sw, hw = "libsvtav1", "av1"
	default:
		if validName.MatchString(codec) {
			return codec
		}
		return Copy
	}
	disc := j.Source != nil && j.Source.Disc
	if p.Options.Hardware != "" && p.Options.HardwareEncoding && !disc {
		if enc := hw + "_" + p.Options.Hardware; p.caps().SupportsEncoder(enc) {
			return enc
		}
	}
	return sw
}

// colorBitDepth returns the video's bit depth, from its pixel format when
// unknown.
func colorBitDepth(v *core.MediaStream) int {
	if v == nil {
		return 0
	}
	if v.BitDepth != 0 {
		return v.BitDepth
	}
	f := strings.ToLower(v.PixelFormat)
	switch {
	case strings.HasSuffix(f, "p10le"):
		return 10
	case strings.HasSuffix(f, "p12le"):
		return 12
	}
	return 8
}

// swTonemap reports whether HDR video is tone mapped in software, which
// needs jellyfin-ffmpeg's tonemapx filter.
func (p *Planner) swTonemap(j *Job) bool {
	return j.Video != nil && colorBitDepth(j.Video) >= 10 && p.caps().SupportsFilter("tonemapx") &&
		j.Video.VideoRange() == core.RangeHDR
}

// vtTonemap reports whether VideoToolbox tone maps the video.
func (p *Planner) vtTonemap(j *Job) bool {
	v := j.Video
	if v == nil || !p.Options.VideoToolboxTonemapping || colorBitDepth(v) < 10 || v.VideoRange() != core.RangeHDR ||
		!p.caps().SupportsFilter("tonemap_videotoolbox") {
		return false
	}
	rt := v.VideoRangeType()
	// Some profile 5 Dolby Vision maps wrongly.
	return rt == core.RangeTypeHDR10 || IsHDR10Plus(v) || IsDOVIWithHDR10BL(v) || rt == core.RangeTypeHLG || rt == core.RangeTypeDOVIInvalid
}

// colorParams overrides the color properties: the HDR input's when tone
// mapping, else BT.709.
func (p *Planner) colorParams(j *Job, tonemap bool) Filter {
	if tonemap {
		trc := "smpte2084"
		if j.Video != nil && strings.EqualFold(j.Video.ColorTransfer, "arib-std-b67") {
			trc = "arib-std-b67"
		}
		return F("setparams", "color_primaries", "bt2020", "color_trc", trc, "colorspace", "bt2020nc")
	}
	f := F("setparams", "color_primaries", "bt709", "color_trc", "bt709", "colorspace", "bt709")
	if r := strings.ToLower(p.Options.TonemapRange); r == "tv" || r == "pc" {
		f.Args = append(f.Args, Arg{"range", r})
	}
	return f
}

// deinterlace reports whether encoded video is deinterlaced: always when
// it is interlaced.
func (j *Job) deinterlace() bool { return j.Video != nil && j.Video.Interlaced }

func (p *Planner) deinterlaceFilter(j *Job, name string) Filter {
	rate := "0"
	if fr, ok := referenceFrameRate(j.Video); p.Options.DeinterlaceDoubleRate && ok && fr <= 30 {
		rate = "1"
	}
	if name == "" {
		name = strings.ToLower(p.Options.DeinterlaceMethod)
		if name == "" {
			name = "yadif"
		}
		return Filter{Name: name, Raw: rate + ":-1:0"}
	}
	return F(name, "mode", rate)
}

// scaleExpr returns the size expression keeping the aspect ratio within
// the job's bounds, with even dimensions, or "" when unbounded.
func scaleExpr(j *Job) string {
	switch w, h := j.MaxWidth, j.MaxHeight; {
	case w > 0 && h > 0:
		return fmt.Sprintf(`trunc(min(max(iw\,ih*a)\,min(%d\,%d*a))/2)*2:trunc(min(max(iw/a\,ih)\,min(%d/a\,%d))/2)*2`, w, h, w, h)
	case w > 0:
		return fmt.Sprintf(`trunc(min(max(iw\,ih*a)\,%d)/2)*2:trunc(ow/a/2)*2`, w)
	case h > 0:
		return fmt.Sprintf(`trunc(oh*a/2)*2:min(max(iw/a\,ih)\,%d)`, h)
	}
	return ""
}

// outputSize returns the output dimensions for hardware scalers, which
// take numbers: the input fitted into the bounds, at most 4096, even.
func outputSize(j *Job) (int, int, bool) {
	v := j.Video
	if v == nil || v.Width == 0 || v.Height == 0 {
		return 0, 0, false
	}
	w, h := v.Width, v.Height
	if abs(v.Rotation) == 90 {
		w, h = h, w
	}
	maxW, maxH := min(w, 4096), min(h, 4096)
	if j.MaxWidth > 0 {
		maxW = min(j.MaxWidth, 4096)
	}
	if j.MaxHeight > 0 {
		maxH = min(j.MaxHeight, 4096)
	}
	if w > maxW || h > maxH {
		scale := math.Min(float64(maxW)/float64(w), float64(maxH)/float64(h))
		w = min(maxW, int(math.RoundToEven(float64(w)*scale)))
		h = min(maxH, int(math.RoundToEven(float64(h)*scale)))
	}
	return 2 * (w / 2), 2 * (h / 2), true
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// SoftwareFilters returns the video graph for software processing:
// color properties, deinterlacing, scaling, tone mapping or the output
// pixel format, and burned-in subtitles. hwDecoded says whether frames
// come from a hardware decoder and are downloaded first.
func (p *Planner) SoftwareFilters(j *Job, hwDecoded bool) VideoGraph {
	var g VideoGraph
	tonemap := p.swTonemap(j)
	if hwDecoded {
		format := "nv12"
		if colorBitDepth(j.Video) > 8 {
			format = "p010le"
		}
		g.Main = append(g.Main, Filter{Name: "hwdownload"}, F("format", "pix_fmts", format))
	}
	g.Main = append(g.Main, p.colorParams(j, tonemap))
	if j.deinterlace() {
		g.Main = append(g.Main, p.deinterlaceFilter(j, ""))
	}
	if expr := scaleExpr(j); expr != "" {
		g.Main = append(g.Main, Filter{Name: "scale", Raw: expr})
	}
	out := "yuv420p"
	if tonemap {
		// Dolby Vision reshaping needs 10-bit input; ffmpeg converts.
		f := F("tonemapx", "tonemap", p.Options.TonemapAlgorithm,
			"desat", formatFloat(p.Options.TonemapDesat), "peak", formatFloat(p.Options.TonemapPeak),
			"t", "bt709", "m", "bt709", "p", "bt709", "format", out)
		if p.Options.TonemapParam != 0 {
			f.Args = append(f.Args, Arg{"param", formatFloat(p.Options.TonemapParam)})
		}
		if r := strings.ToLower(p.Options.TonemapRange); r == "tv" || r == "pc" {
			f.Args = append(f.Args, Arg{"range", r})
		}
		g.Main = append(g.Main, f)
	} else {
		g.Main = append(g.Main, F("format", "pix_fmts", out))
	}
	p.addSubtitleBurnIn(j, &g)
	return g
}

func formatFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// addSubtitleBurnIn renders text subtitles with libass, or overlays
// graphical ones.
func (p *Planner) addSubtitleBurnIn(j *Job, g *VideoGraph) {
	s := j.Subtitle
	if s == nil || !p.burnsInSubtitle(j) {
		return
	}
	if s.IsTextSubtitle() {
		path, index := s.ExternalPath, 0
		if path == "" && j.Source != nil && j.Source.MediaSource != nil {
			path = j.Source.Path
			index = subtitleOrdinal(j.Source.Streams, s)
		}
		f := F("subtitles", "f", escapeFilterPath(path))
		if s.ExternalPath == "" {
			f.Args = append(f.Args, Arg{"si", strconv.Itoa(index)})
		}
		f.Args = append(f.Args, Arg{"alpha", "1"})
		g.Main = append(g.Main, f)
		return
	}
	if expr := scaleExpr(j); expr != "" {
		g.Subtitle = Chain{Filter{Name: "scale", Raw: expr}}
	}
	g.Overlay = Chain{F("overlay", "eof_action", "pass", "repeatlast", "0")}
}

// subtitleOrdinal returns the position of an embedded subtitle stream
// among the embedded subtitle streams, as the subtitles filter counts.
func subtitleOrdinal(streams []core.MediaStream, s *core.MediaStream) int {
	n := 0
	for i := range streams {
		st := &streams[i]
		if st.Kind != core.StreamSubtitle || st.ExternalPath != "" {
			continue
		}
		if st.Index == s.Index {
			return n
		}
		n++
	}
	return 0
}

// escapeFilterPath escapes a path for a filter option value.
func escapeFilterPath(path string) string {
	r := strings.NewReplacer(`\`, `\\\\`, `:`, `\\:`, `'`, `'\\\''`, `[`, `\[`, `]`, `\]`, `,`, `\,`, `;`, `\;`)
	return "'" + r.Replace(path) + "'"
}

// videoToolboxDecodes reports whether VideoToolbox decodes the video.
func (p *Planner) videoToolboxDecodes(j *Job) bool {
	v := j.Video
	if p.Options.Hardware != "videotoolbox" || v == nil || !p.caps().SupportsHwaccel("videotoolbox") {
		return false
	}
	switch {
	case isH264(v):
		return colorBitDepth(v) == 8
	case isH265(v):
		return colorBitDepth(v) <= 10
	case isAV1(v):
		return p.caps().VideoToolboxAV1 && colorBitDepth(v) <= 10
	case strings.EqualFold(v.Codec, "vp9"):
		return colorBitDepth(v) <= 10
	}
	return false
}

// hwDecodeArgs returns the input options for hardware decoding.
func (p *Planner) hwDecodeArgs(j *Job) []string {
	if !j.isVideo() || j.VideoCodec == Copy || !p.videoToolboxDecodes(j) {
		return nil
	}
	return []string{"-hwaccel", "videotoolbox", "-hwaccel_output_format", "videotoolbox_vld"}
}

// VideoFilters returns the video graph for the job and encoder: on
// VideoToolbox, frames stay on the GPU when decoder, filters and encoder
// all can; otherwise processing falls back to software.
func (p *Planner) VideoFilters(j *Job, encoder string) VideoGraph {
	hwDecoded := p.videoToolboxDecodes(j)
	if !hwDecoded {
		return p.SoftwareFilters(j, false)
	}
	vtEncoder := strings.HasSuffix(encoder, "_videotoolbox")
	needsSW := p.burnsInSubtitle(j) && j.Subtitle != nil ||
		(j.Video.VideoRange() == core.RangeHDR && !p.vtTonemap(j)) ||
		(j.deinterlace() && !p.caps().SupportsFilter("yadif_videotoolbox")) ||
		!p.caps().SupportsFilter("scale_vt")
	if !vtEncoder || needsSW {
		return p.SoftwareFilters(j, true)
	}
	g := VideoGraph{HWDevice: "videotoolbox"}
	if j.deinterlace() {
		g.Main = append(g.Main, p.deinterlaceFilter(j, "yadif_videotoolbox"))
	}
	scale := Filter{Name: "scale_vt"}
	if w, h, ok := outputSize(j); ok && (w != j.Video.Width || h != j.Video.Height) {
		scale.Args = append(scale.Args, Arg{"w", strconv.Itoa(w)}, Arg{"h", strconv.Itoa(h)})
	}
	if p.vtTonemap(j) {
		g.Main = append(g.Main, F("tonemap_videotoolbox", "format", "nv12", "p", "bt709", "t", "bt709", "m", "bt709"))
	}
	if len(scale.Args) > 0 {
		g.Main = append(g.Main, scale)
	}
	return g
}

// videoQualityArgs returns the encoder's rate control and speed options.
func (p *Planner) videoQualityArgs(j *Job, encoder string) []string {
	var args []string
	preset := p.Options.Preset
	if preset == "" {
		preset = "veryfast"
	}
	switch encoder {
	case "libx264":
		args = append(args, "-preset", preset, "-crf", strconv.Itoa(p.Options.H264CRF))
	case "libx265":
		args = append(args, "-preset", preset, "-crf", strconv.Itoa(p.Options.H265CRF), "-x265-params", "no-scenecut=1:no-open-gop=1:no-info=1")
	case "libsvtav1":
		args = append(args, "-preset", "10", "-crf", "30")
	case "h264_videotoolbox", "hevc_videotoolbox":
		args = append(args, "-allow_sw", "1", "-realtime", "1")
	}
	if b := j.VideoBitrate; b > 0 {
		bufsize := strconv.Itoa(int(min(int64(b)*2, math.MaxInt32)))
		switch encoder {
		case "libx264", "libx265":
			args = append(args, "-maxrate", strconv.Itoa(b), "-bufsize", bufsize)
		case "libsvtav1":
			args = append(args, "-b:v", strconv.Itoa(b), "-bufsize", bufsize)
		default:
			args = append(args, "-b:v", strconv.Itoa(b), "-maxrate", strconv.Itoa(b), "-bufsize", bufsize)
		}
	}
	if j.MaxFramerate > 0 {
		if fr, ok := referenceFrameRate(j.Video); ok && fr > j.MaxFramerate {
			args = append(args, "-r", formatFloat(float64(j.MaxFramerate)))
		}
	}
	return append(args, p.profileArgs(j, encoder)...)
}

// profileArgs sets the output profile, level and tag clients need.
func (p *Planner) profileArgs(j *Job, encoder string) []string {
	var args []string
	switch {
	case strings.HasPrefix(encoder, "libx264") || encoder == "h264_videotoolbox":
		profile := "high"
		if req := splitList(j.option("h264", "profile")); len(req) > 0 {
			profile = strings.ToLower(strings.ReplaceAll(req[0], " ", ""))
		}
		// Only libx264 encodes High 10.
		if profile == "high10" && encoder != "libx264" {
			profile = "high"
		}
		if profile == "constrainedbaseline" {
			profile = "baseline"
		}
		args = append(args, "-profile:v:0", profile)
		if encoder == "libx264" {
			args = append(args, "-pix_fmt", pixFmtFor(profile))
		}
		if l, ok := h264Level(j.option("h264", "level")); ok {
			switch {
			case encoder != "h264_videotoolbox":
				args = append(args, "-level", strconv.FormatFloat(float64(l)/10, 'f', -1, 64))
			case slices.Contains(videoToolboxH264Levels, l):
				// The encoder only knows its levels by name, e.g. "3.0".
				args = append(args, "-level", strconv.FormatFloat(float64(l)/10, 'f', 1, 64))
			}
		}
	case encoder == "libx265" || encoder == "hevc_videotoolbox":
		args = append(args, "-tag:v", "hvc1")
		if encoder == "libx265" {
			args = append(args, "-pix_fmt", "yuv420p")
		}
	}
	return args
}

// videoToolboxH264Levels are the H.264 levels VideoToolbox encodes to;
// for others it picks a level itself.
var videoToolboxH264Levels = []int{30, 31, 32, 40, 41, 42, 50, 51, 52}

// h264Level parses a requested H.264 level such as "41", capped at 5.1 as
// Jellyfin does: higher levels break fMP4 playback in Safari.
func h264Level(s string) (int, bool) {
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	if n < 0 || n >= 51 {
		return 51, true
	}
	return int(n), true
}

func pixFmtFor(profile string) string {
	if profile == "high10" {
		return "yuv420p10le"
	}
	return "yuv420p"
}

// keyframeArgs aligns keyframes with segment boundaries so that every
// segment starts with one.
func (p *Planner) keyframeArgs(j *Job, encoder string) []string {
	seconds := j.SegmentLength.Seconds()
	if seconds <= 0 {
		seconds = 6
	}
	force := []string{"-force_key_frames:0", fmt.Sprintf("expr:gte(t,n_forced*%s)", formatFloat(seconds))}
	var gop []string
	if j.Video != nil && j.Video.RealFrameRate.Den != 0 {
		// Forcing alone lets scene cuts move the next keyframe past the
		// segment end.
		n := strconv.Itoa(int(math.Ceil(seconds * j.Video.RealFrameRate.Float())))
		gop = []string{"-g:v:0", n, "-keyint_min:v:0", n}
	}
	switch encoder {
	case "libsvtav1":
		return gop
	case "libx264":
		// Scene cut detection would break the forced keyframes.
		return append(force, "-sc_threshold:v:0", "0")
	case "libx265":
		return force
	}
	return append(force, gop...)
}

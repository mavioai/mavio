package planner

import (
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/hwaccel"
)

func isH264(st *core.MediaStream) bool {
	c := strings.ToLower(st.Codec)
	return strings.Contains(c, "264") || strings.Contains(c, "avc")
}

func isH265(st *core.MediaStream) bool {
	c := strings.ToLower(st.Codec)
	return strings.Contains(c, "265") || strings.Contains(c, "hevc")
}

func isAV1(st *core.MediaStream) bool { return strings.Contains(strings.ToLower(st.Codec), "av1") }

// IsDOVIWithHDR10BL reports whether a Dolby Vision stream has an HDR10
// base layer; an invalid configuration only counts when it signals PQ.
func IsDOVIWithHDR10BL(st *core.MediaStream) bool {
	switch st.VideoRangeType() {
	case core.RangeTypeDOVIWithHDR10, core.RangeTypeDOVIWithEL, core.RangeTypeDOVIWithHDR10Plus, core.RangeTypeDOVIWithELHDR10Plus:
		return true
	case core.RangeTypeDOVIInvalid:
		return strings.EqualFold(st.ColorTransfer, "smpte2084")
	}
	return false
}

// IsDOVI reports whether a stream carries Dolby Vision metadata.
func IsDOVI(st *core.MediaStream) bool {
	switch st.VideoRangeType() {
	case core.RangeTypeDOVI, core.RangeTypeDOVIWithHLG, core.RangeTypeDOVIWithSDR, core.RangeTypeDOVIInvalid:
		return true
	}
	return IsDOVIWithHDR10BL(st)
}

// IsHDR10Plus reports whether a stream carries HDR10+ metadata.
func IsHDR10Plus(st *core.MediaStream) bool {
	switch st.VideoRangeType() {
	case core.RangeTypeHDR10Plus, core.RangeTypeDOVIWithHDR10Plus, core.RangeTypeDOVIWithELHDR10Plus:
		return true
	}
	return false
}

// removal is dynamic HDR metadata a copied stream must lose for the client.
type removal int

const (
	removeNone removal = iota
	removeDOVI
	removeHDR10Plus
)

// requestedRangeTypes returns the range types the client accepts for codec.
func (j *Job) requestedRangeTypes(codec string) []string {
	return splitList(j.option(codec, "rangetype"))
}

func splitList(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '|' })
}

func containsFold(list []string, s string) bool {
	return slices.ContainsFunc(list, func(v string) bool { return strings.EqualFold(v, s) })
}

// dynamicMetadataRemoval decides which dynamic metadata a copied video
// stream must lose: Dolby Vision for HDR10 clients that cannot take an
// enhancement layer, invalid Dolby Vision for Dolby Vision clients, and
// HDR10+ for Dolby Vision clients.
func (j *Job) dynamicMetadataRemoval() removal {
	v := j.Video
	rt := v.VideoRangeType()
	if v.VideoRange() != core.RangeHDR && rt != core.RangeTypeDOVIInvalid {
		return removeNone
	}
	req := j.requestedRangeTypes(v.Codec)
	if len(req) == 0 {
		return removeNone
	}
	hasHDR10 := containsFold(req, string(core.RangeTypeHDR10))
	hasDOVI := containsFold(req, string(core.RangeTypeDOVI))
	hasDOVIWithEL := containsFold(req, string(core.RangeTypeDOVIWithEL))
	hasDOVIWithELHDR10Plus := containsFold(req, string(core.RangeTypeDOVIWithELHDR10Plus))

	removeHDR10P := false
	removeDV := (!hasDOVIWithEL && hasHDR10) && rt == core.RangeTypeDOVIWithEL
	// Clients without Dolby Vision may copy broken configurations: HDR10
	// players ignore them.
	removeDV = removeDV || (hasDOVI && rt == core.RangeTypeDOVIInvalid)
	if rt == core.RangeTypeDOVIWithELHDR10Plus {
		removeHDR10P = hasDOVIWithEL && !hasDOVIWithELHDR10Plus
		removeDV = removeDV || !removeHDR10P
	}
	if removeDV {
		return removeDOVI
	}
	if removeHDR10P || (hasDOVI && rt == core.RangeTypeDOVIWithHDR10Plus) {
		return removeHDR10Plus
	}
	return removeNone
}

func canRemove(caps *hwaccel.Capabilities, r removal, v *core.MediaStream) bool {
	switch r {
	case removeDOVI:
		return caps.SupportsBSFOption(hwaccel.DOVIRPUStrip) ||
			(isH265(v) && caps.SupportsBSFOption(hwaccel.HEVCMetadataRemoveDOVI)) ||
			(isAV1(v) && caps.SupportsBSFOption(hwaccel.AV1MetadataRemoveDOVI))
	case removeHDR10Plus:
		return (isH265(v) && caps.SupportsBSFOption(hwaccel.HEVCMetadataRemoveHDR10Plus)) ||
			(isAV1(v) && caps.SupportsBSFOption(hwaccel.AV1MetadataRemoveHDR10Plus))
	}
	return true
}

// DOVIRemoved reports whether copied video loses its Dolby Vision metadata.
func (p *Planner) DOVIRemoved(j *Job) bool {
	return j.Video != nil && j.dynamicMetadataRemoval() == removeDOVI && canRemove(p.caps(), removeDOVI, j.Video)
}

// HDR10PlusRemoved reports whether copied video loses its HDR10+ metadata.
func (p *Planner) HDR10PlusRemoved(j *Job) bool {
	return j.Video != nil && j.dynamicMetadataRemoval() == removeHDR10Plus && canRemove(p.caps(), removeHDR10Plus, j.Video)
}

// BitstreamArgs returns the bitstream filters for a copied stream of kind:
// Annex B conversion for H.264 and HEVC, ADTS conversion for AAC, and the
// removal of dynamic HDR metadata.
func (p *Planner) BitstreamArgs(j *Job, kind core.StreamKind) []string {
	st := j.Video
	if kind == core.StreamAudio {
		st = j.Audio
	}
	if st == nil {
		return nil
	}
	caps := p.caps()
	switch {
	case isH264(st):
		return []string{"-bsf:v", "h264_mp4toannexb"}
	case isAAC(st):
		return []string{"-bsf:a", "aac_adtstoasc"}
	case isH265(st):
		filter := "hevc_mp4toannexb"
		switch j.dynamicMetadataRemoval() {
		case removeDOVI:
			if caps.SupportsBSFOption(hwaccel.HEVCMetadataRemoveDOVI) {
				filter += ",hevc_metadata=remove_dovi=1"
			} else {
				filter += ",dovi_rpu=strip=1"
			}
		case removeHDR10Plus:
			filter += ",hevc_metadata=remove_hdr10plus=1"
		}
		return []string{"-bsf:v", filter}
	case isAV1(st):
		switch j.dynamicMetadataRemoval() {
		case removeDOVI:
			if caps.SupportsBSFOption(hwaccel.AV1MetadataRemoveDOVI) {
				return []string{"-bsf:v", "av1_metadata=remove_dovi=1"}
			}
			return []string{"-bsf:v", "dovi_rpu=strip=1"}
		case removeHDR10Plus:
			return []string{"-bsf:v", "av1_metadata=remove_hdr10plus=1"}
		}
	}
	return nil
}

// Profiles in increasing order of capability, for comparing a source's
// profile with the one requested.
var (
	h264Profiles = []string{"ConstrainedBaseline", "Baseline", "Extended", "Main", "High", "ProgressiveHigh", "ConstrainedHigh", "High10"}
	h265Profiles = []string{"Main", "Main10"}
	av1Profiles  = []string{"Main", "High", "Professional"}
)

func profileScore(codec, profile string) int {
	profile = strings.ReplaceAll(profile, " ", "")
	var list []string
	switch strings.ToLower(codec) {
	case "h264":
		list = h264Profiles
	case "hevc":
		list = h265Profiles
	case "av1":
		list = av1Profiles
	}
	return slices.IndexFunc(list, func(p string) bool { return strings.EqualFold(p, profile) })
}

// CanCopyVideo reports whether the video stream can be copied rather than
// encoded: the client must accept its codec, profile, range type, rotation,
// size, frame rate, bitrate, bit depth, reference frames and level, and
// nothing may need to be burned in or deinterlaced.
func (p *Planner) CanCopyVideo(j *Job) bool {
	v := j.Video
	if v == nil || !j.AllowVideoCopy {
		return false
	}
	if v.Interlaced && (j.Deinterlace || strings.EqualFold(j.option(v.Codec, "deinterlace"), "true")) {
		return false
	}
	if v.Anamorphic && j.RequireNonAnamorphic {
		return false
	}
	if j.Subtitle != nil && j.SubtitleMethod == "encode" {
		return false
	}
	if strings.EqualFold(v.Codec, "h264") && !v.AVC && j.RequireAVC {
		return false
	}
	if v.Codec == "" {
		return false
	}
	if j.Request != nil && len(j.Request.VideoCodecs) > 0 && !containsFold(j.Request.VideoCodecs, v.Codec) {
		return false
	}
	if req := splitList(j.option(v.Codec, "profile")); len(req) > 0 && v.Profile != "" &&
		!containsFold(req, strings.ReplaceAll(v.Profile, " ", "")) {
		if cur := profileScore(v.Codec, v.Profile); cur == -1 || cur > profileScore(v.Codec, req[0]) {
			return false
		}
	}
	if !p.rangeTypeCopyable(j) {
		return false
	}
	if req := splitList(j.option(v.Codec, "rotation")); len(req) > 0 && v.Rotation != 0 &&
		!slices.Contains(req, strconv.Itoa(v.Rotation)) {
		return false
	}
	if (j.MaxWidth > 0 && (v.Width == 0 || v.Width > j.MaxWidth)) ||
		(j.MaxHeight > 0 && (v.Height == 0 || v.Height > j.MaxHeight)) {
		return false
	}
	if j.MaxFramerate > 0 {
		// Some files record a rate slightly above the intended one.
		if fr, ok := referenceFrameRate(v); !ok || fr > j.MaxFramerate+0.05 {
			return false
		}
	}
	if j.VideoBitrate > 0 && (v.Bitrate == 0 || v.Bitrate > int64(j.VideoBitrate)) {
		// Live streams have no bitrate; let them try.
		if j.Source == nil || !j.Source.Infinite || v.Bitrate != 0 {
			return false
		}
	}
	if n, err := strconv.Atoi(j.option(v.Codec, "videobitdepth")); err == nil && v.BitDepth > n {
		return false
	}
	if n, err := strconv.Atoi(j.option(v.Codec, "maxrefframes")); err == nil && v.RefFrames > n {
		return false
	}
	if l, err := strconv.ParseFloat(j.option(v.Codec, "level"), 64); err == nil && v.Level != 0 && float64(v.Level) > l {
		return false
	}
	// H.264 in AVI without AVC framing cannot be remuxed.
	if strings.EqualFold(j.inputContainer(), "avi") && strings.EqualFold(v.Codec, "h264") && !v.AVC {
		return false
	}
	return true
}

func (p *Planner) rangeTypeCopyable(j *Job) bool {
	v := j.Video
	req := j.requestedRangeTypes(v.Codec)
	if len(req) == 0 {
		return true
	}
	rt := v.VideoRangeType()
	if rt == "" {
		return false
	}
	has := func(t core.VideoRangeType) bool { return containsFold(req, string(t)) }
	// With SDR only, no HDR stream is copied.
	if len(req) == 1 && has(core.RangeTypeSDR) && rt != core.RangeTypeSDR {
		return false
	}
	if !has(core.RangeTypeDOVI) && rt == core.RangeTypeDOVI {
		return false
	}
	// Dolby Vision with a fallback base layer plays as that layer.
	compatible := has(rt) ||
		(has(core.RangeTypeHDR10) && rt == core.RangeTypeDOVIWithHDR10) ||
		(has(core.RangeTypeHLG) && rt == core.RangeTypeDOVIWithHLG) ||
		(has(core.RangeTypeSDR) && rt == core.RangeTypeDOVIWithSDR) ||
		(has(core.RangeTypeHDR10) && rt == core.RangeTypeHDR10Plus)
	if compatible {
		return true
	}
	if rt == core.RangeTypeHDR10Plus || rt == core.RangeTypeHDR10 || rt == core.RangeTypeHLG {
		return false
	}
	// Copy only when the encoder can remove what the client must not get.
	return canRemove(p.caps(), j.dynamicMetadataRemoval(), v)
}

// referenceFrameRate is the average frame rate unless implausibly high,
// then the real frame rate.
func referenceFrameRate(v *core.MediaStream) (float32, bool) {
	if v.FrameRate.Den != 0 && v.FrameRate.Float32() < 1000 {
		return v.FrameRate.Float32(), true
	}
	if v.RealFrameRate.Den != 0 {
		return v.RealFrameRate.Float32(), true
	}
	return 0, false
}

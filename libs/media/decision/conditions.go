package decision

import (
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// Condition is a requirement on a property of a stream or file.
type Condition struct {
	Property Property
	Op       Op
	// Value is compared with the property; EqualsAny takes values separated
	// by "|". Range types use the [core.VideoRangeType] names.
	Value string
	// Optional conditions hold when the property is unknown; required ones
	// fail.
	Optional bool
}

// Property is a stream or file property a condition tests.
type Property string

// Properties.
const (
	AudioChannels    Property = "AudioChannels"
	AudioBitrate     Property = "AudioBitrate"
	AudioProfile     Property = "AudioProfile"
	AudioSampleRate  Property = "AudioSampleRate"
	AudioBitDepth    Property = "AudioBitDepth"
	IsSecondaryAudio Property = "IsSecondaryAudio"
	Width            Property = "Width"
	Height           Property = "Height"
	VideoBitDepth    Property = "VideoBitDepth"
	VideoBitrate     Property = "VideoBitrate"
	VideoFramerate   Property = "VideoFramerate"
	VideoLevel       Property = "VideoLevel"
	VideoProfile     Property = "VideoProfile"
	VideoRangeType   Property = "VideoRangeType"
	VideoCodecTag    Property = "VideoCodecTag"
	VideoRotation    Property = "VideoRotation"
	IsAnamorphic     Property = "IsAnamorphic"
	IsInterlaced     Property = "IsInterlaced"
	IsAVC            Property = "IsAvc"
	RefFrames        Property = "RefFrames"
	NumStreams       Property = "NumStreams"
	NumAudioStreams  Property = "NumAudioStreams"
	NumVideoStreams  Property = "NumVideoStreams"
)

// Op is a comparison.
type Op string

// Comparisons.
const (
	Equals           Op = "Equals"
	NotEquals        Op = "NotEquals"
	LessThanEqual    Op = "LessThanEqual"
	GreaterThanEqual Op = "GreaterThanEqual"
	EqualsAny        Op = "EqualsAny"
)

// opt is an optional value.
type opt[T any] struct {
	v  T
	ok bool
}

func some[T any](v T) opt[T] { return opt[T]{v, true} }

// nonZero treats the zero value as unknown, as stream fields do.
func nonZero[T comparable](v T) opt[T] {
	var zero T
	return opt[T]{v, v != zero}
}

// videoFacts are the properties video conditions test.
type videoFacts struct {
	width, height, bitDepth, bitrate, refFrames, rotation opt[int]
	level                                                 opt[float64]
	framerate                                             opt[float32]
	profile, codecTag                                     string
	rangeType                                             core.VideoRangeType
	anamorphic, interlaced, avc                           opt[bool]
	numStreams                                            int
	numVideoStreams, numAudioStreams                      opt[int]
}

func newVideoFacts(src *Source, v *core.MediaStream) videoFacts {
	f := videoFacts{numStreams: len(src.Streams)}
	if len(src.Streams) > 0 {
		f.numVideoStreams = some(len(src.StreamsOf(core.StreamVideo)))
		f.numAudioStreams = some(len(src.StreamsOf(core.StreamAudio)))
	}
	// A missing frame rate counts as 0, not as unknown.
	f.framerate = some(float32(0))
	if v == nil {
		return f
	}
	f.width, f.height = nonZero(v.Width), nonZero(v.Height)
	f.bitDepth, f.bitrate = nonZero(v.BitDepth), nonZero(int(v.Bitrate))
	f.refFrames, f.rotation = nonZero(v.RefFrames), nonZero(v.Rotation)
	if v.Level != 0 {
		f.level = some(float64(v.Level))
	}
	if r, ok := referenceFrameRate(v); ok {
		f.framerate = some(r)
	}
	f.profile, f.codecTag = v.Profile, v.CodecTag
	f.rangeType = v.VideoRangeType()
	f.anamorphic, f.interlaced, f.avc = some(v.Anamorphic), some(v.Interlaced), some(v.AVC)
	return f
}

func (f *videoFacts) satisfies(c Condition) bool {
	switch c.Property {
	case IsInterlaced:
		return boolSatisfies(c, f.interlaced)
	case IsAnamorphic:
		return boolSatisfies(c, f.anamorphic)
	case IsAVC:
		return boolSatisfies(c, f.avc)
	case VideoFramerate:
		return floatSatisfies(c, opt[float64]{float64(f.framerate.v), f.framerate.ok})
	case VideoLevel:
		return floatSatisfies(c, f.level)
	case VideoProfile:
		return stringSatisfies(c, f.profile)
	case VideoRangeType:
		return rangeSatisfies(c, f.rangeType)
	case VideoCodecTag:
		return stringSatisfies(c, f.codecTag)
	case VideoBitDepth:
		return intSatisfies(c, f.bitDepth)
	case VideoBitrate:
		return intSatisfies(c, f.bitrate)
	case Height:
		return intSatisfies(c, f.height)
	case Width:
		return intSatisfies(c, f.width)
	case RefFrames:
		return intSatisfies(c, f.refFrames)
	case NumStreams:
		return intSatisfies(c, some(f.numStreams))
	case NumAudioStreams:
		return intSatisfies(c, f.numAudioStreams)
	case NumVideoStreams:
		return intSatisfies(c, f.numVideoStreams)
	case VideoRotation:
		return intSatisfies(c, f.rotation)
	}
	return true
}

// failing returns the conditions f does not satisfy.
func (f *videoFacts) failing(conds []Condition) []Condition {
	var out []Condition
	for _, c := range conds {
		if !f.satisfies(c) {
			out = append(out, c)
		}
	}
	return out
}

// audioFacts are the properties audio conditions test.
type audioFacts struct {
	channels, bitrate, sampleRate, bitDepth opt[int]
	profile                                 string
	secondary                               opt[bool]
}

func newAudioFacts(a *core.MediaStream, secondary opt[bool]) audioFacts {
	return audioFacts{
		channels:   nonZero(a.Channels),
		bitrate:    nonZero(int(a.Bitrate)),
		sampleRate: nonZero(a.SampleRate),
		bitDepth:   nonZero(a.BitDepth),
		profile:    a.Profile,
		secondary:  secondary,
	}
}

// satisfies evaluates a condition on an audio stream; inVideo adds the
// properties only audio streams of video files have. Other properties fail.
func (f *audioFacts) satisfies(c Condition, inVideo bool) bool {
	switch c.Property {
	case AudioBitrate:
		return intSatisfies(c, f.bitrate)
	case AudioChannels:
		return intSatisfies(c, f.channels)
	case AudioSampleRate:
		return intSatisfies(c, f.sampleRate)
	case AudioBitDepth:
		return intSatisfies(c, f.bitDepth)
	case AudioProfile:
		return inVideo && stringSatisfies(c, f.profile)
	case IsSecondaryAudio:
		return inVideo && boolSatisfies(c, f.secondary)
	}
	return false
}

func intSatisfies(c Condition, v opt[int]) bool {
	if !v.ok {
		return c.Optional
	}
	if c.Op == EqualsAny {
		for s := range strings.SplitSeq(c.Value, "|") {
			if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n == v.v {
				return true
			}
		}
		return false
	}
	n, err := strconv.Atoi(strings.TrimSpace(c.Value))
	if err != nil {
		return false
	}
	return compare(c.Op, v.v, n)
}

func floatSatisfies(c Condition, v opt[float64]) bool {
	if !v.ok {
		return c.Optional
	}
	if c.Op == EqualsAny {
		for s := range strings.SplitSeq(c.Value, "|") {
			if f, ok := parseFloat(s); ok && f == v.v {
				return true
			}
		}
		return false
	}
	f, ok := parseFloat(c.Value)
	if !ok {
		return false
	}
	return compare(c.Op, v.v, f)
}

// parseFloat parses an invariant-culture number, allowing thousands
// separators.
func parseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", ""), 64)
	return f, err == nil
}

func compare[T int | float64](op Op, v, want T) bool {
	switch op {
	case Equals:
		return v == want
	case NotEquals:
		return v != want
	case LessThanEqual:
		return v <= want
	case GreaterThanEqual:
		return v >= want
	}
	return false
}

func stringSatisfies(c Condition, v string) bool {
	if v == "" {
		return c.Optional
	}
	switch c.Op {
	case EqualsAny:
		for s := range strings.SplitSeq(c.Value, "|") {
			if strings.EqualFold(s, v) {
				return true
			}
		}
		return false
	case Equals:
		return strings.EqualFold(v, c.Value)
	case NotEquals:
		return !strings.EqualFold(v, c.Value)
	}
	return false
}

func boolSatisfies(c Condition, v opt[bool]) bool {
	if !v.ok {
		return c.Optional
	}
	want, ok := parseBool(c.Value)
	if !ok {
		return false
	}
	switch c.Op {
	case Equals:
		return v.v == want
	case NotEquals:
		return v.v != want
	}
	return false
}

// parseBool accepts "true" and "false" in any case.
func parseBool(s string) (bool, bool) {
	switch s = strings.TrimSpace(s); {
	case strings.EqualFold(s, "true"):
		return true, true
	case strings.EqualFold(s, "false"):
		return false, true
	}
	return false, false
}

// rangeTypes are the range types conditions may name.
var rangeTypes = []core.VideoRangeType{
	core.RangeTypeSDR, core.RangeTypeHDR10, core.RangeTypeHDR10Plus, core.RangeTypeHLG,
	core.RangeTypeDOVI, core.RangeTypeDOVIWithHDR10, core.RangeTypeDOVIWithHLG, core.RangeTypeDOVIWithSDR,
	core.RangeTypeDOVIWithEL, core.RangeTypeDOVIWithHDR10Plus, core.RangeTypeDOVIWithELHDR10Plus,
	core.RangeTypeDOVIInvalid,
}

// parseRangeType returns the range type named s, ignoring case.
func parseRangeType(s string) (core.VideoRangeType, bool) {
	s = strings.TrimSpace(s)
	for _, t := range rangeTypes {
		if strings.EqualFold(s, string(t)) {
			return t, true
		}
	}
	return "", false
}

// rangeSatisfies compares range types; HDR10+ also satisfies HDR10.
func rangeSatisfies(c Condition, v core.VideoRangeType) bool {
	if v == "" {
		return c.Optional
	}
	if v == core.RangeTypeHDR10Plus && rangeSatisfies(c, core.RangeTypeHDR10) {
		return true
	}
	if c.Op == EqualsAny {
		for s := range strings.SplitSeq(c.Value, "|") {
			if t, ok := parseRangeType(s); ok && t == v {
				return true
			}
		}
		return false
	}
	want, ok := parseRangeType(c.Value)
	if !ok {
		return false
	}
	switch c.Op {
	case Equals:
		return v == want
	case NotEquals:
		return v != want
	}
	return false
}

// reason returns the transcode reason for a failed condition.
func (c Condition) reason() Reasons {
	switch c.Property {
	case AudioBitrate:
		return AudioBitrateNotSupported
	case AudioChannels:
		return AudioChannelsNotSupported
	case AudioProfile:
		return AudioProfileNotSupported
	case AudioSampleRate:
		return AudioSampleRateNotSupported
	case AudioBitDepth:
		return AudioBitDepthNotSupported
	case IsSecondaryAudio:
		return SecondaryAudioNotSupported
	case Width, Height:
		return VideoResolutionNotSupported
	case IsAnamorphic:
		return AnamorphicVideoNotSupported
	case IsInterlaced:
		return InterlacedVideoNotSupported
	case NumStreams:
		return StreamCountExceedsLimit
	case RefFrames:
		return RefFramesNotSupported
	case VideoBitDepth:
		return VideoBitDepthNotSupported
	case VideoBitrate:
		return VideoBitrateNotSupported
	case VideoCodecTag:
		return VideoCodecTagNotSupported
	case VideoFramerate:
		return VideoFramerateNotSupported
	case VideoLevel:
		return VideoLevelNotSupported
	case VideoProfile:
		return VideoProfileNotSupported
	case VideoRangeType:
		return VideoRangeTypeNotSupported
	case VideoRotation:
		return VideoRotationNotSupported
	}
	return 0
}

func reasonsOf(failed []Condition) Reasons {
	var r Reasons
	for _, c := range failed {
		r |= c.reason()
	}
	return r
}

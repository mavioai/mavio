package decision

import (
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// Method is how a source is played.
type Method string

// Play methods.
const (
	// Transcode re-encodes video or audio.
	Transcode Method = "transcode"
	// DirectStream remuxes, copying video and possibly audio into a
	// container the client plays.
	DirectStream Method = "direct_stream"
	// DirectPlay serves the file as is.
	DirectPlay Method = "direct_play"
)

// rank orders play methods from transcode (0) to direct play (2).
func (m Method) rank() int {
	switch m {
	case DirectStream:
		return 1
	case DirectPlay:
		return 2
	}
	return 0
}

// Decision is how a source is played and, unless played directly, what it
// is transcoded or remuxed to.
type Decision struct {
	Source  *Source
	Kind    MediaKind
	Context Context
	Method  Method
	// Reasons says why the source is not played directly.
	Reasons Reasons

	// Container and Protocol are the output format.
	Container string
	Protocol  Protocol
	// VideoCodecs and AudioCodecs are the output codecs, preferred first.
	VideoCodecs []string
	AudioCodecs []string
	// AudioStreamIndex and SubtitleStreamIndex select streams; nil plays
	// the defaults, a subtitle index of -1 none.
	AudioStreamIndex    *int
	SubtitleStreamIndex *int

	// TranscodingMaxAudioChannels and GlobalMaxAudioChannels cap the audio
	// channels; 0 means no cap.
	TranscodingMaxAudioChannels int
	GlobalMaxAudioChannels      int
	// AudioBitrate, AudioSampleRate and VideoBitrate are output targets; 0
	// means unset.
	AudioBitrate    int
	AudioSampleRate int
	VideoBitrate    int
	// MaxWidth, MaxHeight and MaxFramerate bound the output video; 0 means
	// unbounded.
	MaxWidth     int
	MaxHeight    int
	MaxFramerate float32

	RequireAVC           bool
	RequireNonAnamorphic bool
	CopyTimestamps       bool
	EnableMpegtsM2TsMode bool
	// EnableSubtitlesInManifest adds subtitle renditions to HLS playlists.
	EnableSubtitlesInManifest bool
	DisableAudioVBR           bool
	SeekInfo                  SeekInfo
	EstimateContentLength     bool
	MinSegments               int
	SegmentLength             int

	SubtitleMethod                      SubtitleMethod
	SubtitleFormat                      string
	SubtitleCodecs                      []string
	AlwaysBurnInSubtitleWhenTranscoding bool

	// options hold per-codec targets such as "h264-level", keyed in lower
	// case.
	options map[string]string
}

// Option returns a target option for codec, such as "level" or "profile",
// falling back to the option for any codec.
func (d *Decision) Option(codec, name string) string {
	if v := d.options[strings.ToLower(codec+"-"+name)]; v != "" {
		return v
	}
	return d.options[strings.ToLower(name)]
}

func (d *Decision) setOption(codec, name, value string) {
	if d.options == nil {
		d.options = map[string]string{}
	}
	if codec != "" {
		name = codec + "-" + name
	}
	d.options[strings.ToLower(name)] = value
}

func (d *Decision) intOption(codec, name string) opt[int] {
	n, err := strconv.Atoi(strings.TrimSpace(d.Option(codec, name)))
	return opt[int]{n, err == nil}
}

// IsDirect reports whether video and audio are copied: the source is
// direct played or direct streamed and is not a disc.
func (d *Decision) IsDirect() bool {
	return (d.Source == nil || !d.Source.Disc) && (d.Method == DirectStream || d.Method == DirectPlay)
}

// TargetVideoStream is the source's video stream.
func (d *Decision) TargetVideoStream() *core.MediaStream {
	if d.Source == nil {
		return nil
	}
	return d.Source.videoStream()
}

// TargetAudioStream is the selected audio stream of the source.
func (d *Decision) TargetAudioStream() *core.MediaStream {
	if d.Source == nil {
		return nil
	}
	return d.Source.defaultAudioStream(d.AudioStreamIndex)
}

// TargetVideoCodec is the output video codec: the source's when it is
// copied or among the output codecs, else all output codecs.
func (d *Decision) TargetVideoCodec() []string {
	return targetCodec(d, d.TargetVideoStream(), d.VideoCodecs)
}

// TargetAudioCodec is TargetVideoCodec for audio.
func (d *Decision) TargetAudioCodec() []string {
	return targetCodec(d, d.TargetAudioStream(), d.AudioCodecs)
}

func targetCodec(d *Decision, st *core.MediaStream, codecs []string) []string {
	var input string
	if st != nil {
		input = st.Codec
	}
	if d.IsDirect() {
		if input == "" {
			return nil
		}
		return []string{input}
	}
	for _, c := range codecs {
		if strings.EqualFold(c, input) {
			if c == "" {
				return nil
			}
			return []string{c}
		}
	}
	return codecs
}

func (d *Decision) firstTargetVideoCodec() string {
	if c := d.TargetVideoCodec(); len(c) > 0 {
		return c[0]
	}
	return ""
}

// TargetVideoProfile is the output video profile.
func (d *Decision) TargetVideoProfile() string {
	v := d.TargetVideoStream()
	if c := d.firstTargetVideoCodec(); !d.IsDirect() && c != "" {
		return d.Option(c, "profile")
	}
	if v == nil {
		return ""
	}
	return v.Profile
}

// TargetVideoLevel is the output video level, 0 when unknown.
func (d *Decision) TargetVideoLevel() float64 {
	v := d.TargetVideoStream()
	if c := d.firstTargetVideoCodec(); !d.IsDirect() && c != "" {
		f, _ := parseFloat(d.Option(c, "level"))
		return f
	}
	if v == nil {
		return 0
	}
	return float64(v.Level)
}

// TargetVideoBitDepth is the output video bit depth, 0 when unknown.
func (d *Decision) TargetVideoBitDepth() int {
	v := d.TargetVideoStream()
	if c := d.firstTargetVideoCodec(); !d.IsDirect() && c != "" {
		return d.intOption(c, "videobitdepth").v
	}
	if v == nil {
		return 0
	}
	return v.BitDepth
}

// TargetVideoRangeType is the output range type.
func (d *Decision) TargetVideoRangeType() core.VideoRangeType {
	v := d.TargetVideoStream()
	if c := d.firstTargetVideoCodec(); !d.IsDirect() && c != "" {
		if t, ok := parseRangeType(d.Option(c, "rangetype")); ok {
			return t
		}
	}
	if v == nil {
		return ""
	}
	return v.VideoRangeType()
}

// TargetAudioChannels is the output channel count, 0 when unknown.
func (d *Decision) TargetAudioChannels() int {
	a := d.TargetAudioStream()
	if c := d.TargetAudioCodec(); !d.IsDirect() && len(c) > 0 && c[0] != "" {
		return d.targetAudioChannels(c[0]).v
	}
	if a == nil {
		return 0
	}
	return a.Channels
}

// targetAudioChannels is the channel option for codec capped by the
// global and transcoding limits.
func (d *Decision) targetAudioChannels(codec string) opt[int] {
	def := nonZero(d.GlobalMaxAudioChannels)
	if !def.ok {
		def = nonZero(d.TranscodingMaxAudioChannels)
	}
	v := d.Option(codec, "audiochannels")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	if def.ok {
		n = min(n, def.v)
	}
	return some(n)
}

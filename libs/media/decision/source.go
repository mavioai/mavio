package decision

import (
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// Source is a media source as offered for one playback, with the defaults
// and limits that apply to it.
type Source struct {
	*core.MediaSource
	// Remote sources are not local files; their bitrate is not capped and
	// they rank after local files.
	Remote bool
	// Disc sources are DVD or Blu-ray folders, which cannot be direct
	// played.
	Disc bool
	// Infinite sources, such as live streams, have no end; their subtitles
	// cannot be converted ahead of playback.
	Infinite bool
	// NoDirectPlay, NoDirectStream and NoTranscoding rule out play methods.
	NoDirectPlay, NoDirectStream, NoTranscoding bool
	// MostCompatible restricts transcoding to MPEG-TS.
	MostCompatible bool

	// DefaultAudioIndex and DefaultSubtitleIndex are the streams to play
	// when the request names none, see [Selector]; nil leaves the choice to
	// the stream flags. A subtitle index of -1 means no subtitles.
	DefaultAudioIndex, DefaultSubtitleIndex *int
	// AudioIndexSource says how DefaultAudioIndex was chosen.
	AudioIndexSource AudioIndexSource
	// SubtitleScores rank subtitle streams by index, see
	// [Selector.SubtitleScores].
	SubtitleScores map[int]int
}

// AudioIndexSource says how a default audio stream was chosen.
type AudioIndexSource uint8

// Audio index sources; a stream chosen by language and default flag has
// both bits.
const (
	AudioIndexDefault  AudioIndexSource = 1 << iota // the stream's default flag
	AudioIndexLanguage                              // the user's preferred language
	AudioIndexUser                                  // the user's remembered choice
)

// videoStream returns the first video stream, or nil.
func (s *Source) videoStream() *core.MediaStream {
	for i := range s.Streams {
		if s.Streams[i].Kind == core.StreamVideo {
			return &s.Streams[i]
		}
	}
	return nil
}

// stream returns the stream of the kind with the given index, or nil.
func (s *Source) stream(kind core.StreamKind, index int) *core.MediaStream {
	for i := range s.Streams {
		if st := &s.Streams[i]; st.Kind == kind && st.Index == index {
			return st
		}
	}
	return nil
}

// defaultAudioStream returns the audio stream with the given index when
// there is one, else the first default audio stream, else the first audio
// stream.
func (s *Source) defaultAudioStream(index *int) *core.MediaStream {
	if index != nil && *index != -1 {
		if st := s.stream(core.StreamAudio, *index); st != nil {
			return st
		}
	}
	for i := range s.Streams {
		if st := &s.Streams[i]; st.Kind == core.StreamAudio && st.Default {
			return st
		}
	}
	for i := range s.Streams {
		if st := &s.Streams[i]; st.Kind == core.StreamAudio {
			return st
		}
	}
	return nil
}

// isSecondaryAudio reports whether an audio stream is not the first
// embedded one; it is unknown when there is no embedded audio.
func (s *Source) isSecondaryAudio(a *core.MediaStream) opt[bool] {
	if isExternal(a) {
		return some(false)
	}
	for i := range s.Streams {
		if st := &s.Streams[i]; st.Kind == core.StreamAudio && !isExternal(st) {
			return some(st.Index != a.Index)
		}
	}
	return opt[bool]{}
}

func isExternal(st *core.MediaStream) bool { return st.ExternalPath != "" }

// supportsExternalStream reports whether a stream can be served as a
// separate file: external streams, text subtitles and PGS and VobSub
// subtitles, which can be extracted.
func supportsExternalStream(st *core.MediaStream) bool {
	return isExternal(st) || st.IsTextSubtitle() || st.IsPGSSubtitle() || st.IsVobSubSubtitle()
}

// referenceFrameRate is the average frame rate unless it is implausibly
// high, which some demuxers report, then the real frame rate.
func referenceFrameRate(v *core.MediaStream) (float32, bool) {
	if avg := v.FrameRate.Float32(); v.FrameRate.Den != 0 && avg < 1000 {
		return avg, true
	}
	if v.RealFrameRate.Den != 0 {
		return v.RealFrameRate.Float32(), true
	}
	return 0, false
}

// canConvertSubtitle reports whether a text subtitle stream can be
// converted to format; ASS and SSA are neither converted from nor to.
func canConvertSubtitle(st *core.MediaStream, format string) bool {
	if !st.IsTextSubtitle() {
		return false
	}
	for _, c := range []string{"ass", "ssa"} {
		if strings.EqualFold(st.Codec, c) || strings.EqualFold(format, c) {
			return false
		}
	}
	return true
}

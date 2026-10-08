package core

import (
	"fmt"
	"slices"
	"time"
)

// MediaSource is one playable version of an item: a media file (or a
// sidecar-augmented file) with its container-level facts and streams. An item
// can have several sources, e.g. a 4K and a 1080p version of a movie.
type MediaSource struct {
	ID     ID
	ItemID ID
	Path   string
	// Name distinguishes versions of the same item, e.g. "4K HDR".
	Name      string
	Container string // ffprobe format name, e.g. "matroska,webm"
	Size      int64  // bytes
	Duration  time.Duration
	Bitrate   int64 // bits per second, whole file
	Streams   []MediaStream
	Chapters  []Chapter
	// Keyframes are the video keyframe timestamps used to cut HLS segments;
	// nil until extracted.
	Keyframes []time.Duration
	ProbedAt  time.Time
}

// Validate checks the source's invariants.
func (s *MediaSource) Validate() error {
	switch {
	case s.ID.IsZero() || s.ItemID.IsZero():
		return fmt.Errorf("%w: media source requires ID and item ID", ErrInvalid)
	case s.Path == "":
		return fmt.Errorf("%w: media source %s has no path", ErrInvalid, s.ID)
	case s.Duration < 0 || s.Size < 0 || s.Bitrate < 0:
		return fmt.Errorf("%w: media source %s has negative size, duration or bitrate", ErrInvalid, s.ID)
	}
	for i := range s.Streams {
		if err := s.Streams[i].Validate(); err != nil {
			return fmt.Errorf("media source %s stream %d: %w", s.ID, i, err)
		}
	}
	return nil
}

// StreamsOf returns the streams of the given kind in index order.
func (s *MediaSource) StreamsOf(kind StreamKind) []MediaStream {
	var out []MediaStream
	for _, st := range s.Streams {
		if st.Kind == kind {
			out = append(out, st)
		}
	}
	return out
}

// StreamKind is the type of a media stream.
type StreamKind string

// Stream kinds.
const (
	StreamVideo      StreamKind = "video"
	StreamAudio      StreamKind = "audio"
	StreamSubtitle   StreamKind = "subtitle"
	StreamAttachment StreamKind = "attachment" // fonts and other embedded files
	StreamImage      StreamKind = "image"      // embedded cover art
	StreamData       StreamKind = "data"
)

// StreamKinds lists every stream kind.
var StreamKinds = []StreamKind{StreamVideo, StreamAudio, StreamSubtitle, StreamAttachment, StreamImage, StreamData}

// Valid reports whether k is a known stream kind.
func (k StreamKind) Valid() bool { return slices.Contains(StreamKinds, k) }

// VideoRange is the dynamic range format of a video stream.
type VideoRange string

// Video ranges.
const (
	RangeSDR         VideoRange = "sdr"
	RangeHDR10       VideoRange = "hdr10"
	RangeHDR10Plus   VideoRange = "hdr10plus"
	RangeHLG         VideoRange = "hlg"
	RangeDolbyVision VideoRange = "dolby_vision"
)

// DolbyVision describes a Dolby Vision configuration record.
type DolbyVision struct {
	Profile int
	Level   int
	// BLCompatibilityID identifies the base layer's fallback: 0 none,
	// 1 HDR10, 2 SDR, 4 HLG (ETSI GS CCM 001).
	BLCompatibilityID int
	RPUPresent        bool
	ELPresent         bool // enhancement layer
	BLPresent         bool
}

// MediaStream is one elementary stream of a media source, or a sidecar
// subtitle file presented as a stream.
type MediaStream struct {
	// Index is the stream index within its container, or a synthetic index
	// after the embedded streams for sidecar files.
	Index int
	Kind  StreamKind
	Codec string // ffprobe codec name, e.g. "hevc", "eac3", "subrip"
	// CodecTag is the container's codec tag, e.g. "hvc1" vs "hev1".
	CodecTag string
	Profile  string
	Level    int
	Bitrate  int64  // bits per second
	Language string // ISO 639-2/B, e.g. "eng"
	Title    string

	Default         bool
	Forced          bool
	HearingImpaired bool
	// ExternalPath is set for sidecar files such as "Movie.en.srt".
	ExternalPath string

	// Video.
	Width          int
	Height         int
	FrameRate      Rational
	PixelFormat    string // e.g. "yuv420p10le"
	BitDepth       int
	ColorRange     string // "tv" or "pc"
	ColorPrimaries string // e.g. "bt2020"
	ColorTransfer  string // e.g. "smpte2084"
	ColorSpace     string // e.g. "bt2020nc"
	Range          VideoRange
	DolbyVision    *DolbyVision
	Interlaced     bool
	Rotation       int // degrees, clockwise
	// SampleAspectRatio is the pixel aspect ratio; zero means square pixels.
	SampleAspectRatio Rational

	// Audio.
	Channels      int
	ChannelLayout string // e.g. "5.1(side)"
	SampleRate    int    // Hz

	// Subtitle.
	// TextBased is false for bitmap formats such as PGS and VobSub, which can
	// only be burned in.
	TextBased bool
}

// Validate checks the stream's invariants.
func (s *MediaStream) Validate() error {
	switch {
	case !s.Kind.Valid():
		return fmt.Errorf("%w: unknown stream kind %q", ErrInvalid, s.Kind)
	case s.Index < 0:
		return fmt.Errorf("%w: negative stream index", ErrInvalid)
	case s.Kind == StreamVideo && (s.Width < 0 || s.Height < 0):
		return fmt.Errorf("%w: negative video dimensions", ErrInvalid)
	case s.Kind == StreamAudio && s.Channels < 0:
		return fmt.Errorf("%w: negative channel count", ErrInvalid)
	}
	return nil
}

// Rational is a fraction such as a frame rate (24000/1001) or an aspect ratio.
type Rational struct {
	Num, Den int64
}

// Float returns the value of r, or 0 when the denominator is 0.
func (r Rational) Float() float64 {
	if r.Den == 0 {
		return 0
	}
	return float64(r.Num) / float64(r.Den)
}

// IsZero reports whether r is unset.
func (r Rational) IsZero() bool { return r.Num == 0 || r.Den == 0 }

// String formats r as "num/den".
func (r Rational) String() string { return fmt.Sprintf("%d/%d", r.Num, r.Den) }

// Chapter is a named position in a media source.
type Chapter struct {
	Start time.Duration
	Title string
	// ImagePath is the extracted chapter thumbnail, if any.
	ImagePath string
}

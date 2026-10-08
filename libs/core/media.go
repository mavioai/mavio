package core

import (
	"fmt"
	"slices"
	"strings"
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
	Name string
	// Container is the container format, as ffprobe names it with Matroska
	// as "mkv" and MPEG-TS as "ts", e.g. "mkv" or "mov,mp4,m4a,3gp,3g2,mj2".
	Container string
	Size      int64 // bytes
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

// VideoRange is the dynamic range of a video stream.
type VideoRange string

// Video ranges.
const (
	RangeSDR VideoRange = "sdr"
	RangeHDR VideoRange = "hdr"
)

// VideoRangeType is the dynamic range format of a video stream, telling
// clients what they must support to play it as is.
type VideoRangeType string

// Video range types.
const (
	RangeTypeSDR       VideoRangeType = "sdr"
	RangeTypeHDR10     VideoRangeType = "hdr10"
	RangeTypeHDR10Plus VideoRangeType = "hdr10plus"
	RangeTypeHLG       VideoRangeType = "hlg"
	// Dolby Vision without a compatible base layer (profile 5).
	RangeTypeDOVI VideoRangeType = "dovi"
	// Dolby Vision with an HDR10, HLG or SDR compatible base layer.
	RangeTypeDOVIWithHDR10 VideoRangeType = "dovi_hdr10"
	RangeTypeDOVIWithHLG   VideoRangeType = "dovi_hlg"
	RangeTypeDOVIWithSDR   VideoRangeType = "dovi_sdr"
	// Dolby Vision with an enhancement layer (profile 7).
	RangeTypeDOVIWithEL VideoRangeType = "dovi_el"
	// Dolby Vision whose base layer also carries HDR10+.
	RangeTypeDOVIWithHDR10Plus   VideoRangeType = "dovi_hdr10plus"
	RangeTypeDOVIWithELHDR10Plus VideoRangeType = "dovi_el_hdr10plus"
	// Dolby Vision signalling that does not match the stream.
	RangeTypeDOVIInvalid VideoRangeType = "dovi_invalid"
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
	// VersionMajor and VersionMinor are the record's version.
	VersionMajor, VersionMinor int
}

// AudioSpatialFormat is an object-based surround format carried in an
// audio stream.
type AudioSpatialFormat string

// Spatial audio formats; empty means none.
const (
	SpatialDolbyAtmos AudioSpatialFormat = "dolby_atmos"
	SpatialDTSX       AudioSpatialFormat = "dtsx"
)

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
	Bitrate  int64  // bits per second; zero when unknown
	Language string // ISO 639-2/B, e.g. "eng"
	Title    string
	Comment  string
	// TimeBase and CodecTimeBase are ffprobe's time bases, e.g. "1/1000".
	TimeBase      string
	CodecTimeBase string

	Default         bool
	Forced          bool
	HearingImpaired bool
	// Original marks the original-language audio track.
	Original bool
	// ExternalPath is set for sidecar files such as "Movie.en.srt".
	ExternalPath string

	// Video.
	Width  int
	Height int
	// FrameRate is the average frame rate; RealFrameRate the lowest rate
	// that represents all timestamps (ffprobe's r_frame_rate).
	FrameRate      Rational
	RealFrameRate  Rational
	PixelFormat    string // e.g. "yuv420p10le"
	BitDepth       int
	ColorRange     string // "tv" or "pc"
	ColorPrimaries string // e.g. "bt2020"
	ColorTransfer  string // e.g. "smpte2084"
	ColorSpace     string // e.g. "bt2020nc"
	DolbyVision    *DolbyVision
	Interlaced     bool
	Rotation       int // degrees, clockwise
	// SampleAspectRatio is the pixel aspect ratio; zero means square pixels.
	SampleAspectRatio Rational
	// AspectRatio is the display aspect ratio, e.g. "16:9" or "2.40:1".
	AspectRatio string
	// Anamorphic is set for non-square pixels.
	Anamorphic bool
	// AVC is set for H.264 in length-prefixed (avcC) form, whose NAL units
	// are NALLengthSize bytes long; Annex B streams have neither.
	AVC           bool
	NALLengthSize string
	RefFrames     int
	// HDR10Plus is set when frames carry HDR10+ dynamic metadata.
	HDR10Plus bool

	// Audio.
	Channels      int
	ChannelLayout string // e.g. "5.1(side)"
	SampleRate    int    // Hz
}

// VideoRange returns the dynamic range of a video stream; empty for other
// streams.
func (s *MediaStream) VideoRange() VideoRange {
	r, _ := s.videoRange()
	return r
}

// VideoRangeType returns the dynamic range format of a video stream;
// empty for other streams. It is derived, as in Jellyfin, from the color
// transfer, the Dolby Vision configuration and its codec tag, and the
// HDR10+ flag.
func (s *MediaStream) VideoRangeType() VideoRangeType {
	_, t := s.videoRange()
	return t
}

func (s *MediaStream) videoRange() (VideoRange, VideoRangeType) {
	if s.Kind != StreamVideo {
		return "", ""
	}
	isPQ := strings.EqualFold(s.ColorTransfer, "smpte2084")
	isHLG := strings.EqualFold(s.ColorTransfer, "arib-std-b67")
	// Invalid Dolby Vision keeps HDR only when the base layer signals it.
	base := RangeSDR
	if isPQ || isHLG {
		base = RangeHDR
	}
	var dv DolbyVision
	if s.DolbyVision != nil {
		dv = *s.DolbyVision
	}
	isDoViProfile := dv.Profile == 5 || dv.Profile == 7 || dv.Profile == 8 || dv.Profile == 10
	compat := dv.BLCompatibilityID
	isDoViFlag := dv.RPUPresent && dv.BLPresent && (compat == 0 || compat == 1 || compat == 2 || compat == 4 || compat == 6)
	tag := strings.ToLower(s.CodecTag)
	if (isDoViProfile && isDoViFlag) || tag == "dovi" || tag == "dvh1" || tag == "dvhe" || tag == "dav1" {
		r, t := RangeSDR, RangeTypeSDR
		switch dv.Profile {
		case 5:
			r, t = RangeHDR, RangeTypeDOVI
		case 7:
			r, t = RangeHDR, RangeTypeDOVIWithEL
		case 8, 10:
			switch {
			case compat == 0 && dv.Profile == 10:
				r, t = RangeHDR, RangeTypeDOVI
			case compat == 1:
				r, t = RangeHDR, RangeTypeDOVIWithHDR10
			case compat == 4:
				r, t = RangeHDR, RangeTypeDOVIWithHLG
			case compat == 2:
				r, t = RangeSDR, RangeTypeDOVIWithSDR
			default:
				r, t = base, RangeTypeDOVIInvalid
			}
		}
		expected := ""
		switch t {
		case RangeTypeDOVIWithHDR10, RangeTypeDOVIWithEL:
			expected = "smpte2084"
		case RangeTypeDOVIWithHLG:
			expected = "arib-std-b67"
		}
		if expected != "" && (!strings.EqualFold(s.ColorSpace, "bt2020nc") ||
			!strings.EqualFold(s.ColorTransfer, expected) || !strings.EqualFold(s.ColorPrimaries, "bt2020")) {
			return base, RangeTypeDOVIInvalid
		}
		if s.HDR10Plus {
			switch t {
			case RangeTypeDOVIWithHDR10:
				return RangeHDR, RangeTypeDOVIWithHDR10Plus
			case RangeTypeDOVIWithEL:
				return RangeHDR, RangeTypeDOVIWithELHDR10Plus
			}
		}
		return r, t
	}
	switch {
	case isPQ && s.HDR10Plus:
		return RangeHDR, RangeTypeHDR10Plus
	case isPQ:
		return RangeHDR, RangeTypeHDR10
	case isHLG:
		return RangeHDR, RangeTypeHLG
	}
	return RangeSDR, RangeTypeSDR
}

// IsTextSubtitle reports whether a subtitle stream is text-based and can be
// converted or delivered as text; bitmap formats such as PGS and VobSub can
// only be burned in.
func (s *MediaStream) IsTextSubtitle() bool {
	return s.Kind == StreamSubtitle && (s.Codec != "" || s.ExternalPath != "") && IsTextSubtitleCodec(s.Codec)
}

// IsPGSSubtitle reports whether a subtitle stream is a Blu-ray PGS stream.
func (s *MediaStream) IsPGSSubtitle() bool {
	return s.Kind == StreamSubtitle && (s.Codec != "" || s.ExternalPath != "") && IsPGSSubtitleCodec(s.Codec)
}

// IsVobSubSubtitle reports whether a subtitle stream is a DVD VobSub
// stream.
func (s *MediaStream) IsVobSubSubtitle() bool {
	return s.Kind == StreamSubtitle && (s.Codec != "" || s.ExternalPath != "") && IsVobSubSubtitleCodec(s.Codec)
}

// IsTextSubtitleCodec reports whether a subtitle codec or file format is
// text-based. MicroDVD shares the .sub extension with VobSub but is text.
func IsTextSubtitleCodec(codec string) bool {
	c := strings.ToLower(codec)
	return strings.Contains(c, "microdvd") ||
		(!strings.Contains(c, "pgs") && !strings.Contains(c, "dvdsub") && !strings.Contains(c, "vobsub") &&
			!strings.Contains(c, "dvbsub") && c != "sup" && c != "sub")
}

// IsPGSSubtitleCodec reports whether a subtitle codec or file format is
// PGS.
func IsPGSSubtitleCodec(codec string) bool {
	c := strings.ToLower(codec)
	return strings.Contains(c, "pgs") || c == "sup"
}

// IsVobSubSubtitleCodec reports whether a subtitle codec or file format is
// VobSub.
func IsVobSubSubtitleCodec(codec string) bool {
	c := strings.ToLower(codec)
	return strings.Contains(c, "dvdsub") || strings.Contains(c, "vobsub")
}

// SpatialFormat returns the spatial audio format named by an audio
// stream's profile, such as "Dolby TrueHD + Dolby Atmos".
func (s *MediaStream) SpatialFormat() AudioSpatialFormat {
	if s.Kind != StreamAudio {
		return ""
	}
	p := strings.ToLower(s.Profile)
	switch {
	case strings.Contains(p, "dolby atmos"):
		return SpatialDolbyAtmos
	case strings.Contains(p, "dts:x"):
		return SpatialDTSX
	}
	return ""
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

// Float32 returns the value of r in single precision, or 0 when the
// denominator is 0.
func (r Rational) Float32() float32 {
	if r.Den == 0 {
		return 0
	}
	return float32(r.Num) / float32(r.Den)
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

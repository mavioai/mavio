package decision

import "strings"

// ClientCapabilities describes what a client can play: which containers
// and codecs it plays directly, which formats it can receive transcodes in,
// the limits of its decoders and how it shows subtitles.
type ClientCapabilities struct {
	// Name identifies the client in logs.
	Name string
	// MaxStreamingBitrate caps streams played over the network, in bits per
	// second; 0 means no cap.
	MaxStreamingBitrate int64
	// MaxStaticBitrate caps downloads ([Static] context).
	MaxStaticBitrate int64
	// MusicStreamingTranscodingBitrate is the bitrate audio is transcoded to
	// for streaming.
	MusicStreamingTranscodingBitrate int64
	// MaxStaticMusicBitrate caps audio downloads; 0 falls back to
	// MaxStaticBitrate.
	MaxStaticMusicBitrate int64

	DirectPlay  []DirectPlayProfile
	Transcoding []TranscodingProfile
	Containers  []ContainerProfile
	Codecs      []CodecProfile
	Subtitles   []SubtitleProfile
}

// MediaKind is the kind of media a profile applies to.
type MediaKind string

// Media kinds.
const (
	Audio MediaKind = "audio"
	Video MediaKind = "video"
	Photo MediaKind = "photo"
)

// Context says whether media is streamed for playback or downloaded.
type Context string

// Contexts.
const (
	Streaming Context = "streaming"
	Static    Context = "static"
)

// Protocol is how transcoded media is delivered.
type Protocol string

// Protocols.
const (
	HTTP Protocol = "http" // a progressive byte stream
	HLS  Protocol = "hls"  // an HLS playlist
)

// SeekInfo says how a client seeks within a transcode.
type SeekInfo string

// Seek modes.
const (
	SeekAuto  SeekInfo = "auto"
	SeekBytes SeekInfo = "bytes"
)

// SubtitleMethod is how a subtitle stream reaches the client.
type SubtitleMethod string

// Subtitle delivery methods.
const (
	SubtitleEncode   SubtitleMethod = "encode"   // burned into the video
	SubtitleEmbed    SubtitleMethod = "embed"    // muxed into the output container
	SubtitleExternal SubtitleMethod = "external" // served as a separate file
	SubtitleHLS      SubtitleMethod = "hls"      // a subtitle rendition of the HLS playlist
	SubtitleDrop     SubtitleMethod = "drop"     // not delivered
)

// DirectPlayProfile is a combination of container and codecs the client
// plays as is.
type DirectPlayProfile struct {
	Kind       MediaKind
	Container  Names
	VideoCodec Names
	AudioCodec Names
}

func (p *DirectPlayProfile) supportsContainer(container string) bool {
	return p.Container.Contains(container)
}

func (p *DirectPlayProfile) supportsVideoCodec(codec string) bool {
	return p.Kind == Video && p.VideoCodec.Contains(codec)
}

// supportsAudioCodec also holds for video profiles, which may restrict
// their audio.
func (p *DirectPlayProfile) supportsAudioCodec(codec string) bool {
	return (p.Kind == Audio || p.Kind == Video) && p.AudioCodec.Contains(codec)
}

// TranscodingProfile is a format the client accepts transcodes in.
type TranscodingProfile struct {
	Kind       MediaKind
	Context    Context
	Protocol   Protocol
	Container  string
	VideoCodec Names
	AudioCodec Names
	// MaxAudioChannels caps the transcoded audio; 0 means no cap.
	MaxAudioChannels      int
	EstimateContentLength bool
	EnableMpegtsM2TsMode  bool
	SeekInfo              SeekInfo
	CopyTimestamps        bool
	// EnableSubtitlesInManifest adds subtitle renditions to HLS playlists.
	EnableSubtitlesInManifest bool
	MinSegments               int
	SegmentLength             int
	// Conditions limit the transcoded video, e.g. its width.
	Conditions []Condition
	// DisableAudioVBR asks for constant bitrate audio.
	DisableAudioVBR bool
}

// ContainerProfile applies conditions to every file in a container.
type ContainerProfile struct {
	Kind         MediaKind
	Container    Names
	SubContainer Names
	Conditions   []Condition
}

func (p *ContainerProfile) containsContainer(container string) bool {
	return p.Container.containsSpan(container)
}

// CodecKind says which streams a codec profile applies to.
type CodecKind string

// Codec profile kinds.
const (
	CodecVideo      CodecKind = "video"
	CodecVideoAudio CodecKind = "video_audio" // audio streams of video files
	CodecAudio      CodecKind = "audio"       // audio files
)

// CodecProfile limits a codec: Conditions must hold for direct play, and
// transcodes are made to satisfy them. The profile only applies to streams
// that satisfy ApplyConditions.
type CodecProfile struct {
	Kind            CodecKind
	Codec           Names
	Container       Names
	SubContainer    Names
	Conditions      []Condition
	ApplyConditions []Condition
}

// containsAnyCodec reports whether the profile applies to any of codecs in
// container. With useSubContainer, an "hls" profile is matched by its
// segment container. Unlike containers, a codec list is never negated.
func (p *CodecProfile) containsAnyCodec(codecs []string, container string, useSubContainer bool) bool {
	c := p.Container
	if useSubContainer && strings.EqualFold(string(p.Container), "hls") {
		c = p.SubContainer
	}
	if !c.Contains(container) {
		return false
	}
	for _, codec := range codecs {
		if containsAny(string(p.Codec), false, codec) {
			return true
		}
	}
	return false
}

// containsCodec is containsAnyCodec for one codec, where an empty codec
// matches a profile without codecs.
func (p *CodecProfile) containsCodec(codec, container string, useSubContainer bool) bool {
	c := p.Container
	if useSubContainer && strings.EqualFold(string(p.Container), "hls") {
		c = p.SubContainer
	}
	if !c.Contains(container) {
		return false
	}
	return (codec == "" && p.Codec == "") || containsAny(string(p.Codec), false, codec)
}

// SubtitleProfile is a subtitle format the client shows and how it gets it.
type SubtitleProfile struct {
	Format    string
	Method    SubtitleMethod
	Language  Names
	Container Names
}

func (p *SubtitleProfile) supportsLanguage(lang string) bool {
	if p.Language == "" {
		return true
	}
	if lang == "" {
		lang = "und"
	}
	return p.Language.Contains(lang)
}

//! What a client plays (`docs/clients.md` §3), as the apps probe it with
//! the platform's APIs; the server decides from it how to play an item.
//! The records follow `mavio.playback.v1.ClientCapabilities`.

use mavio_api::proto::mavio::playback::v1 as pb;

/// The kind of media of a profile.
#[derive(Debug, Clone, Copy, uniffi::Enum)]
pub enum MediaType {
    /// Audio.
    Audio,
    /// Video.
    Video,
    /// Photos.
    Photo,
}

impl From<MediaType> for pb::MediaKind {
    fn from(k: MediaType) -> Self {
        match k {
            MediaType::Audio => pb::MediaKind::MEDIA_KIND_AUDIO,
            MediaType::Video => pb::MediaKind::MEDIA_KIND_VIDEO,
            MediaType::Photo => pb::MediaKind::MEDIA_KIND_PHOTO,
        }
    }
}

/// A stream or file property a condition tests.
#[derive(Debug, Clone, Copy, uniffi::Enum)]
#[allow(missing_docs)]
pub enum Property {
    AudioChannels,
    AudioBitrate,
    AudioProfile,
    AudioSampleRate,
    AudioBitDepth,
    IsSecondaryAudio,
    Width,
    Height,
    VideoBitDepth,
    VideoBitrate,
    VideoFramerate,
    VideoLevel,
    VideoProfile,
    /// Values are range types: sdr, hdr10, hdr10plus, hlg, dovi, …
    VideoRangeType,
    VideoCodecTag,
    VideoRotation,
    IsAnamorphic,
    IsInterlaced,
    IsAvc,
    RefFrames,
    NumStreams,
    NumAudioStreams,
    NumVideoStreams,
}

impl From<Property> for pb::Property {
    fn from(p: Property) -> Self {
        use pb::Property as P;
        match p {
            Property::AudioChannels => P::PROPERTY_AUDIO_CHANNELS,
            Property::AudioBitrate => P::PROPERTY_AUDIO_BITRATE,
            Property::AudioProfile => P::PROPERTY_AUDIO_PROFILE,
            Property::AudioSampleRate => P::PROPERTY_AUDIO_SAMPLE_RATE,
            Property::AudioBitDepth => P::PROPERTY_AUDIO_BIT_DEPTH,
            Property::IsSecondaryAudio => P::PROPERTY_IS_SECONDARY_AUDIO,
            Property::Width => P::PROPERTY_WIDTH,
            Property::Height => P::PROPERTY_HEIGHT,
            Property::VideoBitDepth => P::PROPERTY_VIDEO_BIT_DEPTH,
            Property::VideoBitrate => P::PROPERTY_VIDEO_BITRATE,
            Property::VideoFramerate => P::PROPERTY_VIDEO_FRAMERATE,
            Property::VideoLevel => P::PROPERTY_VIDEO_LEVEL,
            Property::VideoProfile => P::PROPERTY_VIDEO_PROFILE,
            Property::VideoRangeType => P::PROPERTY_VIDEO_RANGE_TYPE,
            Property::VideoCodecTag => P::PROPERTY_VIDEO_CODEC_TAG,
            Property::VideoRotation => P::PROPERTY_VIDEO_ROTATION,
            Property::IsAnamorphic => P::PROPERTY_IS_ANAMORPHIC,
            Property::IsInterlaced => P::PROPERTY_IS_INTERLACED,
            Property::IsAvc => P::PROPERTY_IS_AVC,
            Property::RefFrames => P::PROPERTY_REF_FRAMES,
            Property::NumStreams => P::PROPERTY_NUM_STREAMS,
            Property::NumAudioStreams => P::PROPERTY_NUM_AUDIO_STREAMS,
            Property::NumVideoStreams => P::PROPERTY_NUM_VIDEO_STREAMS,
        }
    }
}

/// A comparison.
#[derive(Debug, Clone, Copy, uniffi::Enum)]
#[allow(missing_docs)]
pub enum Op {
    Equals,
    NotEquals,
    LessThanEqual,
    GreaterThanEqual,
    /// Values separated by "|".
    EqualsAny,
}

impl From<Op> for pb::Op {
    fn from(o: Op) -> Self {
        match o {
            Op::Equals => pb::Op::OP_EQUALS,
            Op::NotEquals => pb::Op::OP_NOT_EQUALS,
            Op::LessThanEqual => pb::Op::OP_LESS_THAN_EQUAL,
            Op::GreaterThanEqual => pb::Op::OP_GREATER_THAN_EQUAL,
            Op::EqualsAny => pb::Op::OP_EQUALS_ANY,
        }
    }
}

/// A condition on a property.
#[derive(Debug, Clone, uniffi::Record)]
pub struct Condition {
    /// The property tested.
    pub property: Property,
    /// How.
    pub op: Op,
    /// Against what.
    pub value: String,
    /// Holds when the property is unknown.
    #[uniffi(default = false)]
    pub optional: bool,
}

impl From<Condition> for pb::Condition {
    fn from(c: Condition) -> Self {
        pb::Condition {
            property: Some(pb::Property::from(c.property).into()),
            op: Some(pb::Op::from(c.op).into()),
            value: Some(c.value),
            optional: c.optional.then_some(true),
            ..Default::default()
        }
    }
}

fn conditions(c: Vec<Condition>) -> Vec<pb::Condition> {
    c.into_iter().map(Into::into).collect()
}

/// A combination of container and codecs the client plays as is; the
/// fields are comma-separated lists, empty for any.
#[derive(Debug, Clone, uniffi::Record)]
pub struct DirectPlayProfile {
    /// The kind of media.
    pub kind: MediaType,
    /// Containers, such as "mp4,mov".
    pub container: String,
    /// Video codecs, such as "h264,hevc".
    #[uniffi(default = "")]
    pub video_codec: String,
    /// Audio codecs, such as "aac,ac3".
    #[uniffi(default = "")]
    pub audio_codec: String,
}

/// How a transcode reaches the client.
#[derive(Debug, Clone, Copy, uniffi::Enum)]
pub enum Delivery {
    /// A progressive byte stream.
    Http,
    /// An HLS playlist.
    Hls,
}

/// A format the client accepts transcodes in.
#[derive(Debug, Clone, uniffi::Record)]
pub struct TranscodingProfile {
    /// The kind of media.
    pub kind: MediaType,
    /// For downloads rather than streaming.
    #[uniffi(default = false)]
    pub download: bool,
    /// How the transcode is delivered.
    pub delivery: Delivery,
    /// A single container, such as "mp4" for fMP4 HLS segments.
    pub container: String,
    /// Video codecs, comma-separated.
    #[uniffi(default = "")]
    pub video_codec: String,
    /// Audio codecs, comma-separated.
    #[uniffi(default = "")]
    pub audio_codec: String,
    /// Caps the transcoded audio's channels; 0 for none.
    #[uniffi(default = 0)]
    pub max_audio_channels: i32,
    /// Adds subtitle renditions to HLS playlists.
    #[uniffi(default = false)]
    pub subtitles_in_manifest: bool,
    /// HLS segment length in seconds; 0 for the server's.
    #[uniffi(default = 0)]
    pub segment_length_seconds: i32,
    /// Limits of the transcoded video, such as its width.
    #[uniffi(default = [])]
    pub conditions: Vec<Condition>,
}

/// Conditions on every file in a container.
#[derive(Debug, Clone, uniffi::Record)]
pub struct ContainerProfile {
    /// The kind of media.
    pub kind: MediaType,
    /// Containers, comma-separated.
    pub container: String,
    /// The conditions.
    pub conditions: Vec<Condition>,
}

/// The streams a codec profile applies to.
#[derive(Debug, Clone, Copy, uniffi::Enum)]
pub enum CodecType {
    /// Video streams.
    Video,
    /// Audio streams of video files.
    VideoAudio,
    /// Audio files.
    Audio,
}

/// Limits of a codec: conditions must hold for direct play, and
/// transcodes are made to satisfy them.
#[derive(Debug, Clone, uniffi::Record)]
pub struct CodecProfile {
    /// The streams it applies to.
    pub kind: CodecType,
    /// Codecs, comma-separated.
    pub codec: String,
    /// Containers, comma-separated; empty for any.
    #[uniffi(default = "")]
    pub container: String,
    /// The conditions.
    pub conditions: Vec<Condition>,
    /// The profile applies only to streams that satisfy these.
    #[uniffi(default = [])]
    pub apply_conditions: Vec<Condition>,
}

/// How a subtitle format reaches the client.
#[derive(Debug, Clone, Copy, uniffi::Enum)]
pub enum SubtitleDelivery {
    /// Burned into the video.
    Encode,
    /// Muxed into the output container.
    Embed,
    /// A separate file.
    External,
    /// A subtitle rendition of the HLS playlist.
    Hls,
}

/// A subtitle format the client shows, and how it gets it.
#[derive(Debug, Clone, uniffi::Record)]
pub struct SubtitleProfile {
    /// Such as "vtt", "srt", "ass" or "pgssub".
    pub format: String,
    /// How it reaches the client.
    pub delivery: SubtitleDelivery,
}

/// What a client plays.
#[derive(Debug, Clone, uniffi::Record)]
pub struct Capabilities {
    /// Names the client in the server's logs.
    pub name: String,
    /// Caps streaming, in bits per second; 0 for none.
    #[uniffi(default = 0)]
    pub max_streaming_bitrate: i64,
    /// Caps downloads, in bits per second; 0 for none.
    #[uniffi(default = 0)]
    pub max_static_bitrate: i64,
    /// The combinations played as they are.
    pub direct_play: Vec<DirectPlayProfile>,
    /// The formats of transcodes.
    pub transcoding: Vec<TranscodingProfile>,
    /// Conditions on containers.
    #[uniffi(default = [])]
    pub containers: Vec<ContainerProfile>,
    /// Limits of codecs.
    #[uniffi(default = [])]
    pub codecs: Vec<CodecProfile>,
    /// The subtitle formats shown.
    pub subtitles: Vec<SubtitleProfile>,
}

impl From<Capabilities> for pb::ClientCapabilities {
    fn from(c: Capabilities) -> Self {
        let kind = |k: MediaType| Some(pb::MediaKind::from(k).into());
        let text = |s: String| Some(s).filter(|s| !s.is_empty());
        pb::ClientCapabilities {
            name: text(c.name),
            max_streaming_bitrate: Some(c.max_streaming_bitrate).filter(|b| *b > 0),
            max_static_bitrate: Some(c.max_static_bitrate).filter(|b| *b > 0),
            direct_play: c
                .direct_play
                .into_iter()
                .map(|p| pb::DirectPlayProfile {
                    kind: kind(p.kind),
                    container: text(p.container),
                    video_codec: text(p.video_codec),
                    audio_codec: text(p.audio_codec),
                    ..Default::default()
                })
                .collect(),
            transcoding: c
                .transcoding
                .into_iter()
                .map(|p| pb::TranscodingProfile {
                    kind: kind(p.kind),
                    context: Some(
                        if p.download {
                            pb::Context::CONTEXT_STATIC
                        } else {
                            pb::Context::CONTEXT_STREAMING
                        }
                        .into(),
                    ),
                    protocol: Some(
                        match p.delivery {
                            Delivery::Http => pb::Protocol::PROTOCOL_HTTP,
                            Delivery::Hls => pb::Protocol::PROTOCOL_HLS,
                        }
                        .into(),
                    ),
                    container: text(p.container),
                    video_codec: text(p.video_codec),
                    audio_codec: text(p.audio_codec),
                    max_audio_channels: Some(p.max_audio_channels).filter(|n| *n > 0),
                    enable_subtitles_in_manifest: p.subtitles_in_manifest.then_some(true),
                    segment_length_seconds: Some(p.segment_length_seconds).filter(|n| *n > 0),
                    conditions: conditions(p.conditions),
                    ..Default::default()
                })
                .collect(),
            containers: c
                .containers
                .into_iter()
                .map(|p| pb::ContainerProfile {
                    kind: kind(p.kind),
                    container: text(p.container),
                    conditions: conditions(p.conditions),
                    ..Default::default()
                })
                .collect(),
            codecs: c
                .codecs
                .into_iter()
                .map(|p| pb::CodecProfile {
                    kind: Some(
                        match p.kind {
                            CodecType::Video => pb::CodecKind::CODEC_KIND_VIDEO,
                            CodecType::VideoAudio => pb::CodecKind::CODEC_KIND_VIDEO_AUDIO,
                            CodecType::Audio => pb::CodecKind::CODEC_KIND_AUDIO,
                        }
                        .into(),
                    ),
                    codec: text(p.codec),
                    container: text(p.container),
                    conditions: conditions(p.conditions),
                    apply_conditions: conditions(p.apply_conditions),
                    ..Default::default()
                })
                .collect(),
            subtitles: c
                .subtitles
                .into_iter()
                .map(|p| pb::SubtitleProfile {
                    format: text(p.format),
                    method: Some(
                        match p.delivery {
                            SubtitleDelivery::Encode => pb::SubtitleMethod::SUBTITLE_METHOD_ENCODE,
                            SubtitleDelivery::Embed => pb::SubtitleMethod::SUBTITLE_METHOD_EMBED,
                            SubtitleDelivery::External => {
                                pb::SubtitleMethod::SUBTITLE_METHOD_EXTERNAL
                            }
                            SubtitleDelivery::Hls => pb::SubtitleMethod::SUBTITLE_METHOD_HLS,
                        }
                        .into(),
                    ),
                    ..Default::default()
                })
                .collect(),
            ..Default::default()
        }
    }
}

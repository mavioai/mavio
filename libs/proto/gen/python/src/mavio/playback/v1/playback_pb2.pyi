import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from mavio.library.v1 import item_pb2 as _item_pb2
from mavio.library.v1 import media_pb2 as _media_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class MediaKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MEDIA_KIND_UNSPECIFIED: _ClassVar[MediaKind]
    MEDIA_KIND_AUDIO: _ClassVar[MediaKind]
    MEDIA_KIND_VIDEO: _ClassVar[MediaKind]
    MEDIA_KIND_PHOTO: _ClassVar[MediaKind]

class Context(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CONTEXT_UNSPECIFIED: _ClassVar[Context]
    CONTEXT_STREAMING: _ClassVar[Context]
    CONTEXT_STATIC: _ClassVar[Context]

class Protocol(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PROTOCOL_UNSPECIFIED: _ClassVar[Protocol]
    PROTOCOL_HTTP: _ClassVar[Protocol]
    PROTOCOL_HLS: _ClassVar[Protocol]

class SeekInfo(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SEEK_INFO_UNSPECIFIED: _ClassVar[SeekInfo]
    SEEK_INFO_AUTO: _ClassVar[SeekInfo]
    SEEK_INFO_BYTES: _ClassVar[SeekInfo]

class SubtitleMethod(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SUBTITLE_METHOD_UNSPECIFIED: _ClassVar[SubtitleMethod]
    SUBTITLE_METHOD_ENCODE: _ClassVar[SubtitleMethod]
    SUBTITLE_METHOD_EMBED: _ClassVar[SubtitleMethod]
    SUBTITLE_METHOD_EXTERNAL: _ClassVar[SubtitleMethod]
    SUBTITLE_METHOD_HLS: _ClassVar[SubtitleMethod]
    SUBTITLE_METHOD_DROP: _ClassVar[SubtitleMethod]

class CodecKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CODEC_KIND_UNSPECIFIED: _ClassVar[CodecKind]
    CODEC_KIND_VIDEO: _ClassVar[CodecKind]
    CODEC_KIND_VIDEO_AUDIO: _ClassVar[CodecKind]
    CODEC_KIND_AUDIO: _ClassVar[CodecKind]

class Property(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PROPERTY_UNSPECIFIED: _ClassVar[Property]
    PROPERTY_AUDIO_CHANNELS: _ClassVar[Property]
    PROPERTY_AUDIO_BITRATE: _ClassVar[Property]
    PROPERTY_AUDIO_PROFILE: _ClassVar[Property]
    PROPERTY_AUDIO_SAMPLE_RATE: _ClassVar[Property]
    PROPERTY_AUDIO_BIT_DEPTH: _ClassVar[Property]
    PROPERTY_IS_SECONDARY_AUDIO: _ClassVar[Property]
    PROPERTY_WIDTH: _ClassVar[Property]
    PROPERTY_HEIGHT: _ClassVar[Property]
    PROPERTY_VIDEO_BIT_DEPTH: _ClassVar[Property]
    PROPERTY_VIDEO_BITRATE: _ClassVar[Property]
    PROPERTY_VIDEO_FRAMERATE: _ClassVar[Property]
    PROPERTY_VIDEO_LEVEL: _ClassVar[Property]
    PROPERTY_VIDEO_PROFILE: _ClassVar[Property]
    PROPERTY_VIDEO_RANGE_TYPE: _ClassVar[Property]
    PROPERTY_VIDEO_CODEC_TAG: _ClassVar[Property]
    PROPERTY_VIDEO_ROTATION: _ClassVar[Property]
    PROPERTY_IS_ANAMORPHIC: _ClassVar[Property]
    PROPERTY_IS_INTERLACED: _ClassVar[Property]
    PROPERTY_IS_AVC: _ClassVar[Property]
    PROPERTY_REF_FRAMES: _ClassVar[Property]
    PROPERTY_NUM_STREAMS: _ClassVar[Property]
    PROPERTY_NUM_AUDIO_STREAMS: _ClassVar[Property]
    PROPERTY_NUM_VIDEO_STREAMS: _ClassVar[Property]

class Op(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    OP_UNSPECIFIED: _ClassVar[Op]
    OP_EQUALS: _ClassVar[Op]
    OP_NOT_EQUALS: _ClassVar[Op]
    OP_LESS_THAN_EQUAL: _ClassVar[Op]
    OP_GREATER_THAN_EQUAL: _ClassVar[Op]
    OP_EQUALS_ANY: _ClassVar[Op]

class PlayMethod(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PLAY_METHOD_UNSPECIFIED: _ClassVar[PlayMethod]
    PLAY_METHOD_TRANSCODE: _ClassVar[PlayMethod]
    PLAY_METHOD_DIRECT_STREAM: _ClassVar[PlayMethod]
    PLAY_METHOD_DIRECT_PLAY: _ClassVar[PlayMethod]

class TranscodeReason(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    TRANSCODE_REASON_UNSPECIFIED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_CONTAINER_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_VIDEO_CODEC_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_AUDIO_CODEC_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_SUBTITLE_CODEC_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_AUDIO_IS_EXTERNAL: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_SECONDARY_AUDIO_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_VIDEO_PROFILE_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_VIDEO_LEVEL_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_VIDEO_RESOLUTION_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_VIDEO_BIT_DEPTH_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_VIDEO_FRAMERATE_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_REF_FRAMES_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_ANAMORPHIC_VIDEO_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_INTERLACED_VIDEO_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_AUDIO_CHANNELS_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_AUDIO_PROFILE_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_AUDIO_SAMPLE_RATE_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_AUDIO_BIT_DEPTH_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_CONTAINER_BITRATE_EXCEEDS_LIMIT: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_VIDEO_BITRATE_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_AUDIO_BITRATE_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_UNKNOWN_VIDEO_STREAM_INFO: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_UNKNOWN_AUDIO_STREAM_INFO: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_DIRECT_PLAY_ERROR: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_VIDEO_RANGE_TYPE_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_VIDEO_CODEC_TAG_NOT_SUPPORTED: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_STREAM_COUNT_EXCEEDS_LIMIT: _ClassVar[TranscodeReason]
    TRANSCODE_REASON_VIDEO_ROTATION_NOT_SUPPORTED: _ClassVar[TranscodeReason]

class HardwareAcceleration(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    HARDWARE_ACCELERATION_UNSPECIFIED: _ClassVar[HardwareAcceleration]
    HARDWARE_ACCELERATION_AUTO: _ClassVar[HardwareAcceleration]
    HARDWARE_ACCELERATION_NONE: _ClassVar[HardwareAcceleration]
    HARDWARE_ACCELERATION_VIDEOTOOLBOX: _ClassVar[HardwareAcceleration]
MEDIA_KIND_UNSPECIFIED: MediaKind
MEDIA_KIND_AUDIO: MediaKind
MEDIA_KIND_VIDEO: MediaKind
MEDIA_KIND_PHOTO: MediaKind
CONTEXT_UNSPECIFIED: Context
CONTEXT_STREAMING: Context
CONTEXT_STATIC: Context
PROTOCOL_UNSPECIFIED: Protocol
PROTOCOL_HTTP: Protocol
PROTOCOL_HLS: Protocol
SEEK_INFO_UNSPECIFIED: SeekInfo
SEEK_INFO_AUTO: SeekInfo
SEEK_INFO_BYTES: SeekInfo
SUBTITLE_METHOD_UNSPECIFIED: SubtitleMethod
SUBTITLE_METHOD_ENCODE: SubtitleMethod
SUBTITLE_METHOD_EMBED: SubtitleMethod
SUBTITLE_METHOD_EXTERNAL: SubtitleMethod
SUBTITLE_METHOD_HLS: SubtitleMethod
SUBTITLE_METHOD_DROP: SubtitleMethod
CODEC_KIND_UNSPECIFIED: CodecKind
CODEC_KIND_VIDEO: CodecKind
CODEC_KIND_VIDEO_AUDIO: CodecKind
CODEC_KIND_AUDIO: CodecKind
PROPERTY_UNSPECIFIED: Property
PROPERTY_AUDIO_CHANNELS: Property
PROPERTY_AUDIO_BITRATE: Property
PROPERTY_AUDIO_PROFILE: Property
PROPERTY_AUDIO_SAMPLE_RATE: Property
PROPERTY_AUDIO_BIT_DEPTH: Property
PROPERTY_IS_SECONDARY_AUDIO: Property
PROPERTY_WIDTH: Property
PROPERTY_HEIGHT: Property
PROPERTY_VIDEO_BIT_DEPTH: Property
PROPERTY_VIDEO_BITRATE: Property
PROPERTY_VIDEO_FRAMERATE: Property
PROPERTY_VIDEO_LEVEL: Property
PROPERTY_VIDEO_PROFILE: Property
PROPERTY_VIDEO_RANGE_TYPE: Property
PROPERTY_VIDEO_CODEC_TAG: Property
PROPERTY_VIDEO_ROTATION: Property
PROPERTY_IS_ANAMORPHIC: Property
PROPERTY_IS_INTERLACED: Property
PROPERTY_IS_AVC: Property
PROPERTY_REF_FRAMES: Property
PROPERTY_NUM_STREAMS: Property
PROPERTY_NUM_AUDIO_STREAMS: Property
PROPERTY_NUM_VIDEO_STREAMS: Property
OP_UNSPECIFIED: Op
OP_EQUALS: Op
OP_NOT_EQUALS: Op
OP_LESS_THAN_EQUAL: Op
OP_GREATER_THAN_EQUAL: Op
OP_EQUALS_ANY: Op
PLAY_METHOD_UNSPECIFIED: PlayMethod
PLAY_METHOD_TRANSCODE: PlayMethod
PLAY_METHOD_DIRECT_STREAM: PlayMethod
PLAY_METHOD_DIRECT_PLAY: PlayMethod
TRANSCODE_REASON_UNSPECIFIED: TranscodeReason
TRANSCODE_REASON_CONTAINER_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_VIDEO_CODEC_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_AUDIO_CODEC_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_SUBTITLE_CODEC_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_AUDIO_IS_EXTERNAL: TranscodeReason
TRANSCODE_REASON_SECONDARY_AUDIO_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_VIDEO_PROFILE_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_VIDEO_LEVEL_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_VIDEO_RESOLUTION_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_VIDEO_BIT_DEPTH_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_VIDEO_FRAMERATE_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_REF_FRAMES_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_ANAMORPHIC_VIDEO_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_INTERLACED_VIDEO_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_AUDIO_CHANNELS_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_AUDIO_PROFILE_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_AUDIO_SAMPLE_RATE_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_AUDIO_BIT_DEPTH_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_CONTAINER_BITRATE_EXCEEDS_LIMIT: TranscodeReason
TRANSCODE_REASON_VIDEO_BITRATE_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_AUDIO_BITRATE_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_UNKNOWN_VIDEO_STREAM_INFO: TranscodeReason
TRANSCODE_REASON_UNKNOWN_AUDIO_STREAM_INFO: TranscodeReason
TRANSCODE_REASON_DIRECT_PLAY_ERROR: TranscodeReason
TRANSCODE_REASON_VIDEO_RANGE_TYPE_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_VIDEO_CODEC_TAG_NOT_SUPPORTED: TranscodeReason
TRANSCODE_REASON_STREAM_COUNT_EXCEEDS_LIMIT: TranscodeReason
TRANSCODE_REASON_VIDEO_ROTATION_NOT_SUPPORTED: TranscodeReason
HARDWARE_ACCELERATION_UNSPECIFIED: HardwareAcceleration
HARDWARE_ACCELERATION_AUTO: HardwareAcceleration
HARDWARE_ACCELERATION_NONE: HardwareAcceleration
HARDWARE_ACCELERATION_VIDEOTOOLBOX: HardwareAcceleration

class ClientCapabilities(_message.Message):
    __slots__ = ("name", "max_streaming_bitrate", "max_static_bitrate", "music_streaming_transcoding_bitrate", "max_static_music_bitrate", "direct_play", "transcoding", "containers", "codecs", "subtitles")
    NAME_FIELD_NUMBER: _ClassVar[int]
    MAX_STREAMING_BITRATE_FIELD_NUMBER: _ClassVar[int]
    MAX_STATIC_BITRATE_FIELD_NUMBER: _ClassVar[int]
    MUSIC_STREAMING_TRANSCODING_BITRATE_FIELD_NUMBER: _ClassVar[int]
    MAX_STATIC_MUSIC_BITRATE_FIELD_NUMBER: _ClassVar[int]
    DIRECT_PLAY_FIELD_NUMBER: _ClassVar[int]
    TRANSCODING_FIELD_NUMBER: _ClassVar[int]
    CONTAINERS_FIELD_NUMBER: _ClassVar[int]
    CODECS_FIELD_NUMBER: _ClassVar[int]
    SUBTITLES_FIELD_NUMBER: _ClassVar[int]
    name: str
    max_streaming_bitrate: int
    max_static_bitrate: int
    music_streaming_transcoding_bitrate: int
    max_static_music_bitrate: int
    direct_play: _containers.RepeatedCompositeFieldContainer[DirectPlayProfile]
    transcoding: _containers.RepeatedCompositeFieldContainer[TranscodingProfile]
    containers: _containers.RepeatedCompositeFieldContainer[ContainerProfile]
    codecs: _containers.RepeatedCompositeFieldContainer[CodecProfile]
    subtitles: _containers.RepeatedCompositeFieldContainer[SubtitleProfile]
    def __init__(self, name: _Optional[str] = ..., max_streaming_bitrate: _Optional[int] = ..., max_static_bitrate: _Optional[int] = ..., music_streaming_transcoding_bitrate: _Optional[int] = ..., max_static_music_bitrate: _Optional[int] = ..., direct_play: _Optional[_Iterable[_Union[DirectPlayProfile, _Mapping]]] = ..., transcoding: _Optional[_Iterable[_Union[TranscodingProfile, _Mapping]]] = ..., containers: _Optional[_Iterable[_Union[ContainerProfile, _Mapping]]] = ..., codecs: _Optional[_Iterable[_Union[CodecProfile, _Mapping]]] = ..., subtitles: _Optional[_Iterable[_Union[SubtitleProfile, _Mapping]]] = ...) -> None: ...

class DirectPlayProfile(_message.Message):
    __slots__ = ("kind", "container", "video_codec", "audio_codec")
    KIND_FIELD_NUMBER: _ClassVar[int]
    CONTAINER_FIELD_NUMBER: _ClassVar[int]
    VIDEO_CODEC_FIELD_NUMBER: _ClassVar[int]
    AUDIO_CODEC_FIELD_NUMBER: _ClassVar[int]
    kind: MediaKind
    container: str
    video_codec: str
    audio_codec: str
    def __init__(self, kind: _Optional[_Union[MediaKind, str]] = ..., container: _Optional[str] = ..., video_codec: _Optional[str] = ..., audio_codec: _Optional[str] = ...) -> None: ...

class TranscodingProfile(_message.Message):
    __slots__ = ("kind", "context", "protocol", "container", "video_codec", "audio_codec", "max_audio_channels", "estimate_content_length", "enable_mpegts_m2ts_mode", "seek_info", "copy_timestamps", "enable_subtitles_in_manifest", "min_segments", "segment_length_seconds", "conditions", "disable_audio_vbr")
    KIND_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    CONTAINER_FIELD_NUMBER: _ClassVar[int]
    VIDEO_CODEC_FIELD_NUMBER: _ClassVar[int]
    AUDIO_CODEC_FIELD_NUMBER: _ClassVar[int]
    MAX_AUDIO_CHANNELS_FIELD_NUMBER: _ClassVar[int]
    ESTIMATE_CONTENT_LENGTH_FIELD_NUMBER: _ClassVar[int]
    ENABLE_MPEGTS_M2TS_MODE_FIELD_NUMBER: _ClassVar[int]
    SEEK_INFO_FIELD_NUMBER: _ClassVar[int]
    COPY_TIMESTAMPS_FIELD_NUMBER: _ClassVar[int]
    ENABLE_SUBTITLES_IN_MANIFEST_FIELD_NUMBER: _ClassVar[int]
    MIN_SEGMENTS_FIELD_NUMBER: _ClassVar[int]
    SEGMENT_LENGTH_SECONDS_FIELD_NUMBER: _ClassVar[int]
    CONDITIONS_FIELD_NUMBER: _ClassVar[int]
    DISABLE_AUDIO_VBR_FIELD_NUMBER: _ClassVar[int]
    kind: MediaKind
    context: Context
    protocol: Protocol
    container: str
    video_codec: str
    audio_codec: str
    max_audio_channels: int
    estimate_content_length: bool
    enable_mpegts_m2ts_mode: bool
    seek_info: SeekInfo
    copy_timestamps: bool
    enable_subtitles_in_manifest: bool
    min_segments: int
    segment_length_seconds: int
    conditions: _containers.RepeatedCompositeFieldContainer[Condition]
    disable_audio_vbr: bool
    def __init__(self, kind: _Optional[_Union[MediaKind, str]] = ..., context: _Optional[_Union[Context, str]] = ..., protocol: _Optional[_Union[Protocol, str]] = ..., container: _Optional[str] = ..., video_codec: _Optional[str] = ..., audio_codec: _Optional[str] = ..., max_audio_channels: _Optional[int] = ..., estimate_content_length: _Optional[bool] = ..., enable_mpegts_m2ts_mode: _Optional[bool] = ..., seek_info: _Optional[_Union[SeekInfo, str]] = ..., copy_timestamps: _Optional[bool] = ..., enable_subtitles_in_manifest: _Optional[bool] = ..., min_segments: _Optional[int] = ..., segment_length_seconds: _Optional[int] = ..., conditions: _Optional[_Iterable[_Union[Condition, _Mapping]]] = ..., disable_audio_vbr: _Optional[bool] = ...) -> None: ...

class ContainerProfile(_message.Message):
    __slots__ = ("kind", "container", "sub_container", "conditions")
    KIND_FIELD_NUMBER: _ClassVar[int]
    CONTAINER_FIELD_NUMBER: _ClassVar[int]
    SUB_CONTAINER_FIELD_NUMBER: _ClassVar[int]
    CONDITIONS_FIELD_NUMBER: _ClassVar[int]
    kind: MediaKind
    container: str
    sub_container: str
    conditions: _containers.RepeatedCompositeFieldContainer[Condition]
    def __init__(self, kind: _Optional[_Union[MediaKind, str]] = ..., container: _Optional[str] = ..., sub_container: _Optional[str] = ..., conditions: _Optional[_Iterable[_Union[Condition, _Mapping]]] = ...) -> None: ...

class CodecProfile(_message.Message):
    __slots__ = ("kind", "codec", "container", "sub_container", "conditions", "apply_conditions")
    KIND_FIELD_NUMBER: _ClassVar[int]
    CODEC_FIELD_NUMBER: _ClassVar[int]
    CONTAINER_FIELD_NUMBER: _ClassVar[int]
    SUB_CONTAINER_FIELD_NUMBER: _ClassVar[int]
    CONDITIONS_FIELD_NUMBER: _ClassVar[int]
    APPLY_CONDITIONS_FIELD_NUMBER: _ClassVar[int]
    kind: CodecKind
    codec: str
    container: str
    sub_container: str
    conditions: _containers.RepeatedCompositeFieldContainer[Condition]
    apply_conditions: _containers.RepeatedCompositeFieldContainer[Condition]
    def __init__(self, kind: _Optional[_Union[CodecKind, str]] = ..., codec: _Optional[str] = ..., container: _Optional[str] = ..., sub_container: _Optional[str] = ..., conditions: _Optional[_Iterable[_Union[Condition, _Mapping]]] = ..., apply_conditions: _Optional[_Iterable[_Union[Condition, _Mapping]]] = ...) -> None: ...

class SubtitleProfile(_message.Message):
    __slots__ = ("format", "method", "language", "container")
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    CONTAINER_FIELD_NUMBER: _ClassVar[int]
    format: str
    method: SubtitleMethod
    language: str
    container: str
    def __init__(self, format: _Optional[str] = ..., method: _Optional[_Union[SubtitleMethod, str]] = ..., language: _Optional[str] = ..., container: _Optional[str] = ...) -> None: ...

class Condition(_message.Message):
    __slots__ = ("property", "op", "value", "optional")
    PROPERTY_FIELD_NUMBER: _ClassVar[int]
    OP_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    OPTIONAL_FIELD_NUMBER: _ClassVar[int]
    property: Property
    op: Op
    value: str
    optional: bool
    def __init__(self, property: _Optional[_Union[Property, str]] = ..., op: _Optional[_Union[Op, str]] = ..., value: _Optional[str] = ..., optional: _Optional[bool] = ...) -> None: ...

class SubtitleTrack(_message.Message):
    __slots__ = ("stream_index", "language", "title", "default", "forced", "method", "format", "url")
    STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_FIELD_NUMBER: _ClassVar[int]
    FORCED_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    stream_index: int
    language: str
    title: str
    default: bool
    forced: bool
    method: SubtitleMethod
    format: str
    url: str
    def __init__(self, stream_index: _Optional[int] = ..., language: _Optional[str] = ..., title: _Optional[str] = ..., default: _Optional[bool] = ..., forced: _Optional[bool] = ..., method: _Optional[_Union[SubtitleMethod, str]] = ..., format: _Optional[str] = ..., url: _Optional[str] = ...) -> None: ...

class StartPlaybackRequest(_message.Message):
    __slots__ = ("item_id", "media_source_id", "capabilities", "audio_stream_index", "subtitle_stream_index", "max_bitrate", "start_position", "download")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    MEDIA_SOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    CAPABILITIES_FIELD_NUMBER: _ClassVar[int]
    AUDIO_STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    MAX_BITRATE_FIELD_NUMBER: _ClassVar[int]
    START_POSITION_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    media_source_id: str
    capabilities: ClientCapabilities
    audio_stream_index: int
    subtitle_stream_index: int
    max_bitrate: int
    start_position: _duration_pb2.Duration
    download: bool
    def __init__(self, item_id: _Optional[str] = ..., media_source_id: _Optional[str] = ..., capabilities: _Optional[_Union[ClientCapabilities, _Mapping]] = ..., audio_stream_index: _Optional[int] = ..., subtitle_stream_index: _Optional[int] = ..., max_bitrate: _Optional[int] = ..., start_position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., download: _Optional[bool] = ...) -> None: ...

class StartPlaybackResponse(_message.Message):
    __slots__ = ("playback_id", "media_source_id", "method", "transcode_reasons", "url", "audio_stream_index", "subtitle_stream_index", "subtitles", "attachments", "start_position")
    PLAYBACK_ID_FIELD_NUMBER: _ClassVar[int]
    MEDIA_SOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    TRANSCODE_REASONS_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    AUDIO_STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    SUBTITLES_FIELD_NUMBER: _ClassVar[int]
    ATTACHMENTS_FIELD_NUMBER: _ClassVar[int]
    START_POSITION_FIELD_NUMBER: _ClassVar[int]
    playback_id: str
    media_source_id: str
    method: PlayMethod
    transcode_reasons: _containers.RepeatedScalarFieldContainer[TranscodeReason]
    url: str
    audio_stream_index: int
    subtitle_stream_index: int
    subtitles: _containers.RepeatedCompositeFieldContainer[SubtitleTrack]
    attachments: _containers.RepeatedCompositeFieldContainer[Attachment]
    start_position: _duration_pb2.Duration
    def __init__(self, playback_id: _Optional[str] = ..., media_source_id: _Optional[str] = ..., method: _Optional[_Union[PlayMethod, str]] = ..., transcode_reasons: _Optional[_Iterable[_Union[TranscodeReason, str]]] = ..., url: _Optional[str] = ..., audio_stream_index: _Optional[int] = ..., subtitle_stream_index: _Optional[int] = ..., subtitles: _Optional[_Iterable[_Union[SubtitleTrack, _Mapping]]] = ..., attachments: _Optional[_Iterable[_Union[Attachment, _Mapping]]] = ..., start_position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class Attachment(_message.Message):
    __slots__ = ("index", "file_name", "mime_type", "url")
    INDEX_FIELD_NUMBER: _ClassVar[int]
    FILE_NAME_FIELD_NUMBER: _ClassVar[int]
    MIME_TYPE_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    index: int
    file_name: str
    mime_type: str
    url: str
    def __init__(self, index: _Optional[int] = ..., file_name: _Optional[str] = ..., mime_type: _Optional[str] = ..., url: _Optional[str] = ...) -> None: ...

class ReportProgressRequest(_message.Message):
    __slots__ = ("playback_id", "position", "paused")
    PLAYBACK_ID_FIELD_NUMBER: _ClassVar[int]
    POSITION_FIELD_NUMBER: _ClassVar[int]
    PAUSED_FIELD_NUMBER: _ClassVar[int]
    playback_id: str
    position: _duration_pb2.Duration
    paused: bool
    def __init__(self, playback_id: _Optional[str] = ..., position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., paused: _Optional[bool] = ...) -> None: ...

class ReportProgressResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StopPlaybackRequest(_message.Message):
    __slots__ = ("playback_id", "position")
    PLAYBACK_ID_FIELD_NUMBER: _ClassVar[int]
    POSITION_FIELD_NUMBER: _ClassVar[int]
    playback_id: str
    position: _duration_pb2.Duration
    def __init__(self, playback_id: _Optional[str] = ..., position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class StopPlaybackResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListIntrosRequest(_message.Message):
    __slots__ = ("item_id",)
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    def __init__(self, item_id: _Optional[str] = ...) -> None: ...

class ListIntrosResponse(_message.Message):
    __slots__ = ("items",)
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[_item_pb2.Item]
    def __init__(self, items: _Optional[_Iterable[_Union[_item_pb2.Item, _Mapping]]] = ...) -> None: ...

class ListMediaSourcesRequest(_message.Message):
    __slots__ = ("item_id",)
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    def __init__(self, item_id: _Optional[str] = ...) -> None: ...

class ListMediaSourcesResponse(_message.Message):
    __slots__ = ("media_sources",)
    MEDIA_SOURCES_FIELD_NUMBER: _ClassVar[int]
    media_sources: _containers.RepeatedCompositeFieldContainer[_media_pb2.MediaSource]
    def __init__(self, media_sources: _Optional[_Iterable[_Union[_media_pb2.MediaSource, _Mapping]]] = ...) -> None: ...

class TranscodingSettings(_message.Message):
    __slots__ = ("hardware_acceleration", "hardware_encoding", "encoder_preset", "h264_crf", "h265_crf", "threads", "tonemap_algorithm", "tonemap_range", "tonemap_desat", "tonemap_peak", "deinterlace_method", "deinterlace_double_rate", "downmix_boost", "crop_black_borders", "transcode_dir")
    HARDWARE_ACCELERATION_FIELD_NUMBER: _ClassVar[int]
    HARDWARE_ENCODING_FIELD_NUMBER: _ClassVar[int]
    ENCODER_PRESET_FIELD_NUMBER: _ClassVar[int]
    H264_CRF_FIELD_NUMBER: _ClassVar[int]
    H265_CRF_FIELD_NUMBER: _ClassVar[int]
    THREADS_FIELD_NUMBER: _ClassVar[int]
    TONEMAP_ALGORITHM_FIELD_NUMBER: _ClassVar[int]
    TONEMAP_RANGE_FIELD_NUMBER: _ClassVar[int]
    TONEMAP_DESAT_FIELD_NUMBER: _ClassVar[int]
    TONEMAP_PEAK_FIELD_NUMBER: _ClassVar[int]
    DEINTERLACE_METHOD_FIELD_NUMBER: _ClassVar[int]
    DEINTERLACE_DOUBLE_RATE_FIELD_NUMBER: _ClassVar[int]
    DOWNMIX_BOOST_FIELD_NUMBER: _ClassVar[int]
    CROP_BLACK_BORDERS_FIELD_NUMBER: _ClassVar[int]
    TRANSCODE_DIR_FIELD_NUMBER: _ClassVar[int]
    hardware_acceleration: HardwareAcceleration
    hardware_encoding: bool
    encoder_preset: str
    h264_crf: int
    h265_crf: int
    threads: int
    tonemap_algorithm: str
    tonemap_range: str
    tonemap_desat: float
    tonemap_peak: float
    deinterlace_method: str
    deinterlace_double_rate: bool
    downmix_boost: float
    crop_black_borders: bool
    transcode_dir: str
    def __init__(self, hardware_acceleration: _Optional[_Union[HardwareAcceleration, str]] = ..., hardware_encoding: _Optional[bool] = ..., encoder_preset: _Optional[str] = ..., h264_crf: _Optional[int] = ..., h265_crf: _Optional[int] = ..., threads: _Optional[int] = ..., tonemap_algorithm: _Optional[str] = ..., tonemap_range: _Optional[str] = ..., tonemap_desat: _Optional[float] = ..., tonemap_peak: _Optional[float] = ..., deinterlace_method: _Optional[str] = ..., deinterlace_double_rate: _Optional[bool] = ..., downmix_boost: _Optional[float] = ..., crop_black_borders: _Optional[bool] = ..., transcode_dir: _Optional[str] = ...) -> None: ...

class GetTranscodingSettingsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetTranscodingSettingsResponse(_message.Message):
    __slots__ = ("settings", "available_hardware_accelerations")
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_HARDWARE_ACCELERATIONS_FIELD_NUMBER: _ClassVar[int]
    settings: TranscodingSettings
    available_hardware_accelerations: _containers.RepeatedScalarFieldContainer[HardwareAcceleration]
    def __init__(self, settings: _Optional[_Union[TranscodingSettings, _Mapping]] = ..., available_hardware_accelerations: _Optional[_Iterable[_Union[HardwareAcceleration, str]]] = ...) -> None: ...

class UpdateTranscodingSettingsRequest(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: TranscodingSettings
    def __init__(self, settings: _Optional[_Union[TranscodingSettings, _Mapping]] = ...) -> None: ...

class UpdateTranscodingSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: TranscodingSettings
    def __init__(self, settings: _Optional[_Union[TranscodingSettings, _Mapping]] = ...) -> None: ...

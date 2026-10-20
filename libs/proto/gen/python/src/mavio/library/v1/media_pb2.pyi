import datetime

from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class StreamKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    STREAM_KIND_UNSPECIFIED: _ClassVar[StreamKind]
    STREAM_KIND_VIDEO: _ClassVar[StreamKind]
    STREAM_KIND_AUDIO: _ClassVar[StreamKind]
    STREAM_KIND_SUBTITLE: _ClassVar[StreamKind]
    STREAM_KIND_ATTACHMENT: _ClassVar[StreamKind]
    STREAM_KIND_IMAGE: _ClassVar[StreamKind]
    STREAM_KIND_DATA: _ClassVar[StreamKind]

class VideoRange(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    VIDEO_RANGE_UNSPECIFIED: _ClassVar[VideoRange]
    VIDEO_RANGE_SDR: _ClassVar[VideoRange]
    VIDEO_RANGE_HDR10: _ClassVar[VideoRange]
    VIDEO_RANGE_HDR10_PLUS: _ClassVar[VideoRange]
    VIDEO_RANGE_HLG: _ClassVar[VideoRange]
    VIDEO_RANGE_DOLBY_VISION: _ClassVar[VideoRange]
STREAM_KIND_UNSPECIFIED: StreamKind
STREAM_KIND_VIDEO: StreamKind
STREAM_KIND_AUDIO: StreamKind
STREAM_KIND_SUBTITLE: StreamKind
STREAM_KIND_ATTACHMENT: StreamKind
STREAM_KIND_IMAGE: StreamKind
STREAM_KIND_DATA: StreamKind
VIDEO_RANGE_UNSPECIFIED: VideoRange
VIDEO_RANGE_SDR: VideoRange
VIDEO_RANGE_HDR10: VideoRange
VIDEO_RANGE_HDR10_PLUS: VideoRange
VIDEO_RANGE_HLG: VideoRange
VIDEO_RANGE_DOLBY_VISION: VideoRange

class MediaSource(_message.Message):
    __slots__ = ("id", "item_id", "path", "name", "container", "size_bytes", "duration", "bitrate", "streams", "chapters", "probe_time", "remote")
    ID_FIELD_NUMBER: _ClassVar[int]
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CONTAINER_FIELD_NUMBER: _ClassVar[int]
    SIZE_BYTES_FIELD_NUMBER: _ClassVar[int]
    DURATION_FIELD_NUMBER: _ClassVar[int]
    BITRATE_FIELD_NUMBER: _ClassVar[int]
    STREAMS_FIELD_NUMBER: _ClassVar[int]
    CHAPTERS_FIELD_NUMBER: _ClassVar[int]
    PROBE_TIME_FIELD_NUMBER: _ClassVar[int]
    REMOTE_FIELD_NUMBER: _ClassVar[int]
    id: str
    item_id: str
    path: str
    name: str
    container: str
    size_bytes: int
    duration: _duration_pb2.Duration
    bitrate: int
    streams: _containers.RepeatedCompositeFieldContainer[MediaStream]
    chapters: _containers.RepeatedCompositeFieldContainer[Chapter]
    probe_time: _timestamp_pb2.Timestamp
    remote: bool
    def __init__(self, id: _Optional[str] = ..., item_id: _Optional[str] = ..., path: _Optional[str] = ..., name: _Optional[str] = ..., container: _Optional[str] = ..., size_bytes: _Optional[int] = ..., duration: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., bitrate: _Optional[int] = ..., streams: _Optional[_Iterable[_Union[MediaStream, _Mapping]]] = ..., chapters: _Optional[_Iterable[_Union[Chapter, _Mapping]]] = ..., probe_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., remote: _Optional[bool] = ...) -> None: ...

class DolbyVision(_message.Message):
    __slots__ = ("profile", "level", "bl_compatibility_id", "rpu_present", "el_present", "bl_present")
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    LEVEL_FIELD_NUMBER: _ClassVar[int]
    BL_COMPATIBILITY_ID_FIELD_NUMBER: _ClassVar[int]
    RPU_PRESENT_FIELD_NUMBER: _ClassVar[int]
    EL_PRESENT_FIELD_NUMBER: _ClassVar[int]
    BL_PRESENT_FIELD_NUMBER: _ClassVar[int]
    profile: int
    level: int
    bl_compatibility_id: int
    rpu_present: bool
    el_present: bool
    bl_present: bool
    def __init__(self, profile: _Optional[int] = ..., level: _Optional[int] = ..., bl_compatibility_id: _Optional[int] = ..., rpu_present: _Optional[bool] = ..., el_present: _Optional[bool] = ..., bl_present: _Optional[bool] = ...) -> None: ...

class Rational(_message.Message):
    __slots__ = ("num", "den")
    NUM_FIELD_NUMBER: _ClassVar[int]
    DEN_FIELD_NUMBER: _ClassVar[int]
    num: int
    den: int
    def __init__(self, num: _Optional[int] = ..., den: _Optional[int] = ...) -> None: ...

class MediaStream(_message.Message):
    __slots__ = ("index", "kind", "codec", "codec_tag", "profile", "level", "bitrate", "language", "title", "default", "forced", "hearing_impaired", "external_path", "width", "height", "frame_rate", "pixel_format", "bit_depth", "color_range", "color_primaries", "color_transfer", "color_space", "range", "dolby_vision", "interlaced", "rotation", "sample_aspect_ratio", "channels", "channel_layout", "sample_rate", "text_based")
    INDEX_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    CODEC_FIELD_NUMBER: _ClassVar[int]
    CODEC_TAG_FIELD_NUMBER: _ClassVar[int]
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    LEVEL_FIELD_NUMBER: _ClassVar[int]
    BITRATE_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_FIELD_NUMBER: _ClassVar[int]
    FORCED_FIELD_NUMBER: _ClassVar[int]
    HEARING_IMPAIRED_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_PATH_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    FRAME_RATE_FIELD_NUMBER: _ClassVar[int]
    PIXEL_FORMAT_FIELD_NUMBER: _ClassVar[int]
    BIT_DEPTH_FIELD_NUMBER: _ClassVar[int]
    COLOR_RANGE_FIELD_NUMBER: _ClassVar[int]
    COLOR_PRIMARIES_FIELD_NUMBER: _ClassVar[int]
    COLOR_TRANSFER_FIELD_NUMBER: _ClassVar[int]
    COLOR_SPACE_FIELD_NUMBER: _ClassVar[int]
    RANGE_FIELD_NUMBER: _ClassVar[int]
    DOLBY_VISION_FIELD_NUMBER: _ClassVar[int]
    INTERLACED_FIELD_NUMBER: _ClassVar[int]
    ROTATION_FIELD_NUMBER: _ClassVar[int]
    SAMPLE_ASPECT_RATIO_FIELD_NUMBER: _ClassVar[int]
    CHANNELS_FIELD_NUMBER: _ClassVar[int]
    CHANNEL_LAYOUT_FIELD_NUMBER: _ClassVar[int]
    SAMPLE_RATE_FIELD_NUMBER: _ClassVar[int]
    TEXT_BASED_FIELD_NUMBER: _ClassVar[int]
    index: int
    kind: StreamKind
    codec: str
    codec_tag: str
    profile: str
    level: int
    bitrate: int
    language: str
    title: str
    default: bool
    forced: bool
    hearing_impaired: bool
    external_path: str
    width: int
    height: int
    frame_rate: Rational
    pixel_format: str
    bit_depth: int
    color_range: str
    color_primaries: str
    color_transfer: str
    color_space: str
    range: VideoRange
    dolby_vision: DolbyVision
    interlaced: bool
    rotation: int
    sample_aspect_ratio: Rational
    channels: int
    channel_layout: str
    sample_rate: int
    text_based: bool
    def __init__(self, index: _Optional[int] = ..., kind: _Optional[_Union[StreamKind, str]] = ..., codec: _Optional[str] = ..., codec_tag: _Optional[str] = ..., profile: _Optional[str] = ..., level: _Optional[int] = ..., bitrate: _Optional[int] = ..., language: _Optional[str] = ..., title: _Optional[str] = ..., default: _Optional[bool] = ..., forced: _Optional[bool] = ..., hearing_impaired: _Optional[bool] = ..., external_path: _Optional[str] = ..., width: _Optional[int] = ..., height: _Optional[int] = ..., frame_rate: _Optional[_Union[Rational, _Mapping]] = ..., pixel_format: _Optional[str] = ..., bit_depth: _Optional[int] = ..., color_range: _Optional[str] = ..., color_primaries: _Optional[str] = ..., color_transfer: _Optional[str] = ..., color_space: _Optional[str] = ..., range: _Optional[_Union[VideoRange, str]] = ..., dolby_vision: _Optional[_Union[DolbyVision, _Mapping]] = ..., interlaced: _Optional[bool] = ..., rotation: _Optional[int] = ..., sample_aspect_ratio: _Optional[_Union[Rational, _Mapping]] = ..., channels: _Optional[int] = ..., channel_layout: _Optional[str] = ..., sample_rate: _Optional[int] = ..., text_based: _Optional[bool] = ...) -> None: ...

class Chapter(_message.Message):
    __slots__ = ("start", "title", "has_image", "image_url")
    START_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    HAS_IMAGE_FIELD_NUMBER: _ClassVar[int]
    IMAGE_URL_FIELD_NUMBER: _ClassVar[int]
    start: _duration_pb2.Duration
    title: str
    has_image: bool
    image_url: str
    def __init__(self, start: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., title: _Optional[str] = ..., has_image: _Optional[bool] = ..., image_url: _Optional[str] = ...) -> None: ...

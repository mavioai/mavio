import datetime

from google.protobuf import duration_pb2 as _duration_pb2
from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SegmentKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SEGMENT_KIND_UNSPECIFIED: _ClassVar[SegmentKind]
    SEGMENT_KIND_INTRO: _ClassVar[SegmentKind]
    SEGMENT_KIND_OUTRO: _ClassVar[SegmentKind]
    SEGMENT_KIND_RECAP: _ClassVar[SegmentKind]
    SEGMENT_KIND_PREVIEW: _ClassVar[SegmentKind]
    SEGMENT_KIND_COMMERCIAL: _ClassVar[SegmentKind]
SEGMENT_KIND_UNSPECIFIED: SegmentKind
SEGMENT_KIND_INTRO: SegmentKind
SEGMENT_KIND_OUTRO: SegmentKind
SEGMENT_KIND_RECAP: SegmentKind
SEGMENT_KIND_PREVIEW: SegmentKind
SEGMENT_KIND_COMMERCIAL: SegmentKind

class SegmentChapter(_message.Message):
    __slots__ = ("start", "title")
    START_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    start: _duration_pb2.Duration
    title: str
    def __init__(self, start: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., title: _Optional[str] = ...) -> None: ...

class GetSegmentsRequest(_message.Message):
    __slots__ = ("kind", "name", "external_ids", "series_external_ids", "season_number", "episode_number", "file_name", "duration", "chapters")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    class SeriesExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    KIND_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    SERIES_EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    EPISODE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    FILE_NAME_FIELD_NUMBER: _ClassVar[int]
    DURATION_FIELD_NUMBER: _ClassVar[int]
    CHAPTERS_FIELD_NUMBER: _ClassVar[int]
    kind: _metadata_pb2.MediaKind
    name: str
    external_ids: _containers.ScalarMap[str, str]
    series_external_ids: _containers.ScalarMap[str, str]
    season_number: int
    episode_number: int
    file_name: str
    duration: _duration_pb2.Duration
    chapters: _containers.RepeatedCompositeFieldContainer[SegmentChapter]
    def __init__(self, kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., name: _Optional[str] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., series_external_ids: _Optional[_Mapping[str, str]] = ..., season_number: _Optional[int] = ..., episode_number: _Optional[int] = ..., file_name: _Optional[str] = ..., duration: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., chapters: _Optional[_Iterable[_Union[SegmentChapter, _Mapping]]] = ...) -> None: ...

class Segment(_message.Message):
    __slots__ = ("kind", "start", "end")
    KIND_FIELD_NUMBER: _ClassVar[int]
    START_FIELD_NUMBER: _ClassVar[int]
    END_FIELD_NUMBER: _ClassVar[int]
    kind: SegmentKind
    start: _duration_pb2.Duration
    end: _duration_pb2.Duration
    def __init__(self, kind: _Optional[_Union[SegmentKind, str]] = ..., start: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., end: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class GetSegmentsResponse(_message.Message):
    __slots__ = ("segments",)
    SEGMENTS_FIELD_NUMBER: _ClassVar[int]
    segments: _containers.RepeatedCompositeFieldContainer[Segment]
    def __init__(self, segments: _Optional[_Iterable[_Union[Segment, _Mapping]]] = ...) -> None: ...

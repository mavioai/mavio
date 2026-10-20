from buf.validate import validate_pb2 as _validate_pb2
from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RatingSource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    RATING_SOURCE_UNSPECIFIED: _ClassVar[RatingSource]
    RATING_SOURCE_ROTTEN_TOMATOES_CRITICS: _ClassVar[RatingSource]
    RATING_SOURCE_ROTTEN_TOMATOES_AUDIENCE: _ClassVar[RatingSource]
    RATING_SOURCE_IMDB: _ClassVar[RatingSource]
RATING_SOURCE_UNSPECIFIED: RatingSource
RATING_SOURCE_ROTTEN_TOMATOES_CRITICS: RatingSource
RATING_SOURCE_ROTTEN_TOMATOES_AUDIENCE: RatingSource
RATING_SOURCE_IMDB: RatingSource

class GetRatingsRequest(_message.Message):
    __slots__ = ("media_kind", "external_ids", "name", "year")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    media_kind: _metadata_pb2.MediaKind
    external_ids: _containers.ScalarMap[str, str]
    name: str
    year: int
    def __init__(self, media_kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., name: _Optional[str] = ..., year: _Optional[int] = ...) -> None: ...

class GetRatingsResponse(_message.Message):
    __slots__ = ("ratings",)
    RATINGS_FIELD_NUMBER: _ClassVar[int]
    ratings: _containers.RepeatedCompositeFieldContainer[Rating]
    def __init__(self, ratings: _Optional[_Iterable[_Union[Rating, _Mapping]]] = ...) -> None: ...

class Rating(_message.Message):
    __slots__ = ("source", "score", "votes", "url", "certified")
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    VOTES_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    CERTIFIED_FIELD_NUMBER: _ClassVar[int]
    source: RatingSource
    score: float
    votes: int
    url: str
    certified: bool
    def __init__(self, source: _Optional[_Union[RatingSource, str]] = ..., score: _Optional[float] = ..., votes: _Optional[int] = ..., url: _Optional[str] = ..., certified: _Optional[bool] = ...) -> None: ...

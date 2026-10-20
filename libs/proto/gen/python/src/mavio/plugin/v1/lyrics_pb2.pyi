import datetime

from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SearchLyricsRequest(_message.Message):
    __slots__ = ("name", "artists", "album", "duration", "external_ids")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    ARTISTS_FIELD_NUMBER: _ClassVar[int]
    ALBUM_FIELD_NUMBER: _ClassVar[int]
    DURATION_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    name: str
    artists: _containers.RepeatedScalarFieldContainer[str]
    album: str
    duration: _duration_pb2.Duration
    external_ids: _containers.ScalarMap[str, str]
    def __init__(self, name: _Optional[str] = ..., artists: _Optional[_Iterable[str]] = ..., album: _Optional[str] = ..., duration: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., external_ids: _Optional[_Mapping[str, str]] = ...) -> None: ...

class RemoteLyrics(_message.Message):
    __slots__ = ("id", "name", "artists", "album", "duration", "synced", "score")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ARTISTS_FIELD_NUMBER: _ClassVar[int]
    ALBUM_FIELD_NUMBER: _ClassVar[int]
    DURATION_FIELD_NUMBER: _ClassVar[int]
    SYNCED_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    artists: _containers.RepeatedScalarFieldContainer[str]
    album: str
    duration: _duration_pb2.Duration
    synced: bool
    score: float
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., artists: _Optional[_Iterable[str]] = ..., album: _Optional[str] = ..., duration: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., synced: _Optional[bool] = ..., score: _Optional[float] = ...) -> None: ...

class SearchLyricsResponse(_message.Message):
    __slots__ = ("lyrics",)
    LYRICS_FIELD_NUMBER: _ClassVar[int]
    lyrics: _containers.RepeatedCompositeFieldContainer[RemoteLyrics]
    def __init__(self, lyrics: _Optional[_Iterable[_Union[RemoteLyrics, _Mapping]]] = ...) -> None: ...

class DownloadLyricsRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DownloadLyricsResponse(_message.Message):
    __slots__ = ("content", "synced")
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    SYNCED_FIELD_NUMBER: _ClassVar[int]
    content: str
    synced: bool
    def __init__(self, content: _Optional[str] = ..., synced: _Optional[bool] = ...) -> None: ...

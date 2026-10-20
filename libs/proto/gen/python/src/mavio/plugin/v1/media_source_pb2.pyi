from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetMediaSourcesRequest(_message.Message):
    __slots__ = ("item_id", "kind", "name", "external_ids")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    kind: _metadata_pb2.MediaKind
    name: str
    external_ids: _containers.ScalarMap[str, str]
    def __init__(self, item_id: _Optional[str] = ..., kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., name: _Optional[str] = ..., external_ids: _Optional[_Mapping[str, str]] = ...) -> None: ...

class RemoteMediaSource(_message.Message):
    __slots__ = ("id", "name", "url")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    url: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., url: _Optional[str] = ...) -> None: ...

class GetMediaSourcesResponse(_message.Message):
    __slots__ = ("sources",)
    SOURCES_FIELD_NUMBER: _ClassVar[int]
    sources: _containers.RepeatedCompositeFieldContainer[RemoteMediaSource]
    def __init__(self, sources: _Optional[_Iterable[_Union[RemoteMediaSource, _Mapping]]] = ...) -> None: ...

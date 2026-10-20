from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class LocalFile(_message.Message):
    __slots__ = ("name", "content")
    NAME_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    name: str
    content: bytes
    def __init__(self, name: _Optional[str] = ..., content: _Optional[bytes] = ...) -> None: ...

class ReadMetadataRequest(_message.Message):
    __slots__ = ("lookup", "media_name", "files")
    LOOKUP_FIELD_NUMBER: _ClassVar[int]
    MEDIA_NAME_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    lookup: _metadata_pb2.Lookup
    media_name: str
    files: _containers.RepeatedCompositeFieldContainer[LocalFile]
    def __init__(self, lookup: _Optional[_Union[_metadata_pb2.Lookup, _Mapping]] = ..., media_name: _Optional[str] = ..., files: _Optional[_Iterable[_Union[LocalFile, _Mapping]]] = ...) -> None: ...

class ReadMetadataResponse(_message.Message):
    __slots__ = ("found", "metadata")
    FOUND_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    found: bool
    metadata: _metadata_pb2.Metadata
    def __init__(self, found: _Optional[bool] = ..., metadata: _Optional[_Union[_metadata_pb2.Metadata, _Mapping]] = ...) -> None: ...

class SaveMetadataRequest(_message.Message):
    __slots__ = ("kind", "media_name", "metadata")
    KIND_FIELD_NUMBER: _ClassVar[int]
    MEDIA_NAME_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    kind: _metadata_pb2.MediaKind
    media_name: str
    metadata: _metadata_pb2.Metadata
    def __init__(self, kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., media_name: _Optional[str] = ..., metadata: _Optional[_Union[_metadata_pb2.Metadata, _Mapping]] = ...) -> None: ...

class SaveMetadataResponse(_message.Message):
    __slots__ = ("files",)
    FILES_FIELD_NUMBER: _ClassVar[int]
    files: _containers.RepeatedCompositeFieldContainer[LocalFile]
    def __init__(self, files: _Optional[_Iterable[_Union[LocalFile, _Mapping]]] = ...) -> None: ...

from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ProcessMetadataRequest(_message.Message):
    __slots__ = ("kind", "metadata")
    KIND_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    kind: _metadata_pb2.MediaKind
    metadata: _metadata_pb2.Metadata
    def __init__(self, kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., metadata: _Optional[_Union[_metadata_pb2.Metadata, _Mapping]] = ...) -> None: ...

class ProcessMetadataResponse(_message.Message):
    __slots__ = ("changed", "metadata")
    CHANGED_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    changed: bool
    metadata: _metadata_pb2.Metadata
    def __init__(self, changed: _Optional[bool] = ..., metadata: _Optional[_Union[_metadata_pb2.Metadata, _Mapping]] = ...) -> None: ...

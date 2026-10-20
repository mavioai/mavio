from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetIntrosRequest(_message.Message):
    __slots__ = ("item_id", "kind", "name", "user_id")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    kind: _metadata_pb2.MediaKind
    name: str
    user_id: str
    def __init__(self, item_id: _Optional[str] = ..., kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., name: _Optional[str] = ..., user_id: _Optional[str] = ...) -> None: ...

class GetIntrosResponse(_message.Message):
    __slots__ = ("item_ids",)
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, item_ids: _Optional[_Iterable[str]] = ...) -> None: ...

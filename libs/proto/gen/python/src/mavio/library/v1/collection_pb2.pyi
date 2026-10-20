from buf.validate import validate_pb2 as _validate_pb2
from mavio.library.v1 import item_pb2 as _item_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class CreateCollectionRequest(_message.Message):
    __slots__ = ("name", "item_ids")
    NAME_FIELD_NUMBER: _ClassVar[int]
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    name: str
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, name: _Optional[str] = ..., item_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class CreateCollectionResponse(_message.Message):
    __slots__ = ("collection",)
    COLLECTION_FIELD_NUMBER: _ClassVar[int]
    collection: _item_pb2.Item
    def __init__(self, collection: _Optional[_Union[_item_pb2.Item, _Mapping]] = ...) -> None: ...

class UpdateCollectionRequest(_message.Message):
    __slots__ = ("id", "name")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ...) -> None: ...

class UpdateCollectionResponse(_message.Message):
    __slots__ = ("collection",)
    COLLECTION_FIELD_NUMBER: _ClassVar[int]
    collection: _item_pb2.Item
    def __init__(self, collection: _Optional[_Union[_item_pb2.Item, _Mapping]] = ...) -> None: ...

class DeleteCollectionRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteCollectionResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class AddToCollectionRequest(_message.Message):
    __slots__ = ("id", "item_ids")
    ID_FIELD_NUMBER: _ClassVar[int]
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    id: str
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[str] = ..., item_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class AddToCollectionResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RemoveFromCollectionRequest(_message.Message):
    __slots__ = ("id", "item_ids")
    ID_FIELD_NUMBER: _ClassVar[int]
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    id: str
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[str] = ..., item_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class RemoveFromCollectionResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

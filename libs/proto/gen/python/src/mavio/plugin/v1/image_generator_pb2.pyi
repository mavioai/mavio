from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GenerateImagesRequest(_message.Message):
    __slots__ = ("item_id", "kind", "name", "image_kinds")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    IMAGE_KINDS_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    kind: _metadata_pb2.MediaKind
    name: str
    image_kinds: _containers.RepeatedScalarFieldContainer[_metadata_pb2.ImageKind]
    def __init__(self, item_id: _Optional[str] = ..., kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., name: _Optional[str] = ..., image_kinds: _Optional[_Iterable[_Union[_metadata_pb2.ImageKind, str]]] = ...) -> None: ...

class GeneratedImage(_message.Message):
    __slots__ = ("kind", "content")
    KIND_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    kind: _metadata_pb2.ImageKind
    content: bytes
    def __init__(self, kind: _Optional[_Union[_metadata_pb2.ImageKind, str]] = ..., content: _Optional[bytes] = ...) -> None: ...

class GenerateImagesResponse(_message.Message):
    __slots__ = ("images",)
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    images: _containers.RepeatedCompositeFieldContainer[GeneratedImage]
    def __init__(self, images: _Optional[_Iterable[_Union[GeneratedImage, _Mapping]]] = ...) -> None: ...

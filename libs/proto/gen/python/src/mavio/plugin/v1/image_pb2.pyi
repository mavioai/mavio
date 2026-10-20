from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetImagesRequest(_message.Message):
    __slots__ = ("lookup",)
    LOOKUP_FIELD_NUMBER: _ClassVar[int]
    lookup: _metadata_pb2.Lookup
    def __init__(self, lookup: _Optional[_Union[_metadata_pb2.Lookup, _Mapping]] = ...) -> None: ...

class GetImagesResponse(_message.Message):
    __slots__ = ("images",)
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    images: _containers.RepeatedCompositeFieldContainer[_metadata_pb2.RemoteImage]
    def __init__(self, images: _Optional[_Iterable[_Union[_metadata_pb2.RemoteImage, _Mapping]]] = ...) -> None: ...

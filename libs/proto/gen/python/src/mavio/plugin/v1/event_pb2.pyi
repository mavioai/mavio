from mavio.plugin.v1 import provider_pb2 as _provider_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ConsumeRequest(_message.Message):
    __slots__ = ("events",)
    EVENTS_FIELD_NUMBER: _ClassVar[int]
    events: _containers.RepeatedCompositeFieldContainer[_provider_pb2.Event]
    def __init__(self, events: _Optional[_Iterable[_Union[_provider_pb2.Event, _Mapping]]] = ...) -> None: ...

class ConsumeResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

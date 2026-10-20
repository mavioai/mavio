import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class StartResetRequest(_message.Message):
    __slots__ = ("user_id", "user_name", "pin", "expires")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    USER_NAME_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    user_name: str
    pin: str
    expires: _timestamp_pb2.Timestamp
    def __init__(self, user_id: _Optional[str] = ..., user_name: _Optional[str] = ..., pin: _Optional[str] = ..., expires: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class StartResetResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

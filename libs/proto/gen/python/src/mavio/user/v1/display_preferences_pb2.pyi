import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DisplayPreferences(_message.Message):
    __slots__ = ("client", "view", "values", "update_time")
    class ValuesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    CLIENT_FIELD_NUMBER: _ClassVar[int]
    VIEW_FIELD_NUMBER: _ClassVar[int]
    VALUES_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    client: str
    view: str
    values: _containers.ScalarMap[str, str]
    update_time: _timestamp_pb2.Timestamp
    def __init__(self, client: _Optional[str] = ..., view: _Optional[str] = ..., values: _Optional[_Mapping[str, str]] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetDisplayPreferencesRequest(_message.Message):
    __slots__ = ("client", "view")
    CLIENT_FIELD_NUMBER: _ClassVar[int]
    VIEW_FIELD_NUMBER: _ClassVar[int]
    client: str
    view: str
    def __init__(self, client: _Optional[str] = ..., view: _Optional[str] = ...) -> None: ...

class GetDisplayPreferencesResponse(_message.Message):
    __slots__ = ("preferences",)
    PREFERENCES_FIELD_NUMBER: _ClassVar[int]
    preferences: DisplayPreferences
    def __init__(self, preferences: _Optional[_Union[DisplayPreferences, _Mapping]] = ...) -> None: ...

class SetDisplayPreferencesRequest(_message.Message):
    __slots__ = ("client", "view", "values")
    class ValuesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    CLIENT_FIELD_NUMBER: _ClassVar[int]
    VIEW_FIELD_NUMBER: _ClassVar[int]
    VALUES_FIELD_NUMBER: _ClassVar[int]
    client: str
    view: str
    values: _containers.ScalarMap[str, str]
    def __init__(self, client: _Optional[str] = ..., view: _Optional[str] = ..., values: _Optional[_Mapping[str, str]] = ...) -> None: ...

class SetDisplayPreferencesResponse(_message.Message):
    __slots__ = ("preferences",)
    PREFERENCES_FIELD_NUMBER: _ClassVar[int]
    preferences: DisplayPreferences
    def __init__(self, preferences: _Optional[_Union[DisplayPreferences, _Mapping]] = ...) -> None: ...

import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SyncPlayState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SYNC_PLAY_STATE_UNSPECIFIED: _ClassVar[SyncPlayState]
    SYNC_PLAY_STATE_IDLE: _ClassVar[SyncPlayState]
    SYNC_PLAY_STATE_WAITING: _ClassVar[SyncPlayState]
    SYNC_PLAY_STATE_PAUSED: _ClassVar[SyncPlayState]
    SYNC_PLAY_STATE_PLAYING: _ClassVar[SyncPlayState]

class SyncPlayCommandKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SYNC_PLAY_COMMAND_KIND_UNSPECIFIED: _ClassVar[SyncPlayCommandKind]
    SYNC_PLAY_COMMAND_KIND_UNPAUSE: _ClassVar[SyncPlayCommandKind]
    SYNC_PLAY_COMMAND_KIND_PAUSE: _ClassVar[SyncPlayCommandKind]
    SYNC_PLAY_COMMAND_KIND_SEEK: _ClassVar[SyncPlayCommandKind]
    SYNC_PLAY_COMMAND_KIND_STOP: _ClassVar[SyncPlayCommandKind]
SYNC_PLAY_STATE_UNSPECIFIED: SyncPlayState
SYNC_PLAY_STATE_IDLE: SyncPlayState
SYNC_PLAY_STATE_WAITING: SyncPlayState
SYNC_PLAY_STATE_PAUSED: SyncPlayState
SYNC_PLAY_STATE_PLAYING: SyncPlayState
SYNC_PLAY_COMMAND_KIND_UNSPECIFIED: SyncPlayCommandKind
SYNC_PLAY_COMMAND_KIND_UNPAUSE: SyncPlayCommandKind
SYNC_PLAY_COMMAND_KIND_PAUSE: SyncPlayCommandKind
SYNC_PLAY_COMMAND_KIND_SEEK: SyncPlayCommandKind
SYNC_PLAY_COMMAND_KIND_STOP: SyncPlayCommandKind

class SyncPlayGroup(_message.Message):
    __slots__ = ("id", "name", "state", "members", "queue", "current_index", "position", "position_time")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    MEMBERS_FIELD_NUMBER: _ClassVar[int]
    QUEUE_FIELD_NUMBER: _ClassVar[int]
    CURRENT_INDEX_FIELD_NUMBER: _ClassVar[int]
    POSITION_FIELD_NUMBER: _ClassVar[int]
    POSITION_TIME_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    state: SyncPlayState
    members: _containers.RepeatedCompositeFieldContainer[SyncPlayMember]
    queue: _containers.RepeatedCompositeFieldContainer[SyncPlayQueueEntry]
    current_index: int
    position: _duration_pb2.Duration
    position_time: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., state: _Optional[_Union[SyncPlayState, str]] = ..., members: _Optional[_Iterable[_Union[SyncPlayMember, _Mapping]]] = ..., queue: _Optional[_Iterable[_Union[SyncPlayQueueEntry, _Mapping]]] = ..., current_index: _Optional[int] = ..., position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., position_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class SyncPlayMember(_message.Message):
    __slots__ = ("session_id", "user_name", "device_name", "buffering")
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    USER_NAME_FIELD_NUMBER: _ClassVar[int]
    DEVICE_NAME_FIELD_NUMBER: _ClassVar[int]
    BUFFERING_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    user_name: str
    device_name: str
    buffering: bool
    def __init__(self, session_id: _Optional[str] = ..., user_name: _Optional[str] = ..., device_name: _Optional[str] = ..., buffering: _Optional[bool] = ...) -> None: ...

class SyncPlayQueueEntry(_message.Message):
    __slots__ = ("id", "item_id")
    ID_FIELD_NUMBER: _ClassVar[int]
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    item_id: str
    def __init__(self, id: _Optional[str] = ..., item_id: _Optional[str] = ...) -> None: ...

class SyncPlayCommand(_message.Message):
    __slots__ = ("kind", "when", "position", "entry_id")
    KIND_FIELD_NUMBER: _ClassVar[int]
    WHEN_FIELD_NUMBER: _ClassVar[int]
    POSITION_FIELD_NUMBER: _ClassVar[int]
    ENTRY_ID_FIELD_NUMBER: _ClassVar[int]
    kind: SyncPlayCommandKind
    when: _timestamp_pb2.Timestamp
    position: _duration_pb2.Duration
    entry_id: str
    def __init__(self, kind: _Optional[_Union[SyncPlayCommandKind, str]] = ..., when: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., entry_id: _Optional[str] = ...) -> None: ...

class SyncPlayUpdate(_message.Message):
    __slots__ = ("group", "command")
    GROUP_FIELD_NUMBER: _ClassVar[int]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    group: SyncPlayGroup
    command: SyncPlayCommand
    def __init__(self, group: _Optional[_Union[SyncPlayGroup, _Mapping]] = ..., command: _Optional[_Union[SyncPlayCommand, _Mapping]] = ...) -> None: ...

class GetTimeRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetTimeResponse(_message.Message):
    __slots__ = ("receive_time", "send_time")
    RECEIVE_TIME_FIELD_NUMBER: _ClassVar[int]
    SEND_TIME_FIELD_NUMBER: _ClassVar[int]
    receive_time: _timestamp_pb2.Timestamp
    send_time: _timestamp_pb2.Timestamp
    def __init__(self, receive_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., send_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListGroupsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListGroupsResponse(_message.Message):
    __slots__ = ("groups",)
    GROUPS_FIELD_NUMBER: _ClassVar[int]
    groups: _containers.RepeatedCompositeFieldContainer[SyncPlayGroup]
    def __init__(self, groups: _Optional[_Iterable[_Union[SyncPlayGroup, _Mapping]]] = ...) -> None: ...

class CreateGroupRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class CreateGroupResponse(_message.Message):
    __slots__ = ("group",)
    GROUP_FIELD_NUMBER: _ClassVar[int]
    group: SyncPlayGroup
    def __init__(self, group: _Optional[_Union[SyncPlayGroup, _Mapping]] = ...) -> None: ...

class JoinGroupRequest(_message.Message):
    __slots__ = ("group_id",)
    GROUP_ID_FIELD_NUMBER: _ClassVar[int]
    group_id: str
    def __init__(self, group_id: _Optional[str] = ...) -> None: ...

class JoinGroupResponse(_message.Message):
    __slots__ = ("group",)
    GROUP_FIELD_NUMBER: _ClassVar[int]
    group: SyncPlayGroup
    def __init__(self, group: _Optional[_Union[SyncPlayGroup, _Mapping]] = ...) -> None: ...

class LeaveGroupRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class LeaveGroupResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SetQueueRequest(_message.Message):
    __slots__ = ("item_ids", "start_index", "start_position")
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    START_INDEX_FIELD_NUMBER: _ClassVar[int]
    START_POSITION_FIELD_NUMBER: _ClassVar[int]
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    start_index: int
    start_position: _duration_pb2.Duration
    def __init__(self, item_ids: _Optional[_Iterable[str]] = ..., start_index: _Optional[int] = ..., start_position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class SetQueueResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class PauseRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class PauseResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class UnpauseRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class UnpauseResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SeekRequest(_message.Message):
    __slots__ = ("position",)
    POSITION_FIELD_NUMBER: _ClassVar[int]
    position: _duration_pb2.Duration
    def __init__(self, position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class SeekResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StopRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StopResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class NextItemRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class NextItemResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class PreviousItemRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class PreviousItemResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ReportStateRequest(_message.Message):
    __slots__ = ("entry_id", "position", "when", "playing", "ready")
    ENTRY_ID_FIELD_NUMBER: _ClassVar[int]
    POSITION_FIELD_NUMBER: _ClassVar[int]
    WHEN_FIELD_NUMBER: _ClassVar[int]
    PLAYING_FIELD_NUMBER: _ClassVar[int]
    READY_FIELD_NUMBER: _ClassVar[int]
    entry_id: str
    position: _duration_pb2.Duration
    when: _timestamp_pb2.Timestamp
    playing: bool
    ready: bool
    def __init__(self, entry_id: _Optional[str] = ..., position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., when: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., playing: _Optional[bool] = ..., ready: _Optional[bool] = ...) -> None: ...

class ReportStateResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ReportPingRequest(_message.Message):
    __slots__ = ("ping",)
    PING_FIELD_NUMBER: _ClassVar[int]
    ping: _duration_pb2.Duration
    def __init__(self, ping: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class ReportPingResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

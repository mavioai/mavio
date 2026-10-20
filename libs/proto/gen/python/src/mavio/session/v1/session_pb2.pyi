import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.library.v1 import item_pb2 as _item_pb2
from mavio.library.v1 import library_pb2 as _library_pb2
from mavio.playback.v1 import playback_pb2 as _playback_pb2
from mavio.session.v1 import syncplay_pb2 as _syncplay_pb2
from mavio.system.v1 import activity_pb2 as _activity_pb2
from mavio.system.v1 import plugin_pb2 as _plugin_pb2
from mavio.user.v1 import user_pb2 as _user_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class JobState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    JOB_STATE_UNSPECIFIED: _ClassVar[JobState]
    JOB_STATE_RUNNING: _ClassVar[JobState]
    JOB_STATE_SUCCEEDED: _ClassVar[JobState]
    JOB_STATE_FAILED: _ClassVar[JobState]

class PlayStateCommand(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PLAY_STATE_COMMAND_UNSPECIFIED: _ClassVar[PlayStateCommand]
    PLAY_STATE_COMMAND_PAUSE: _ClassVar[PlayStateCommand]
    PLAY_STATE_COMMAND_UNPAUSE: _ClassVar[PlayStateCommand]
    PLAY_STATE_COMMAND_STOP: _ClassVar[PlayStateCommand]
    PLAY_STATE_COMMAND_NEXT: _ClassVar[PlayStateCommand]
    PLAY_STATE_COMMAND_PREVIOUS: _ClassVar[PlayStateCommand]
JOB_STATE_UNSPECIFIED: JobState
JOB_STATE_RUNNING: JobState
JOB_STATE_SUCCEEDED: JobState
JOB_STATE_FAILED: JobState
PLAY_STATE_COMMAND_UNSPECIFIED: PlayStateCommand
PLAY_STATE_COMMAND_PAUSE: PlayStateCommand
PLAY_STATE_COMMAND_UNPAUSE: PlayStateCommand
PLAY_STATE_COMMAND_STOP: PlayStateCommand
PLAY_STATE_COMMAND_NEXT: PlayStateCommand
PLAY_STATE_COMMAND_PREVIOUS: PlayStateCommand

class SubscribeRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SubscribeResponse(_message.Message):
    __slots__ = ("event",)
    EVENT_FIELD_NUMBER: _ClassVar[int]
    event: Event
    def __init__(self, event: _Optional[_Union[Event, _Mapping]] = ...) -> None: ...

class Event(_message.Message):
    __slots__ = ("time", "connected", "heartbeat", "library_changed", "user_data_changed", "sessions_changed", "command", "job_changed", "plugin_changed", "sync_play", "notification", "scan_progress_changed")
    TIME_FIELD_NUMBER: _ClassVar[int]
    CONNECTED_FIELD_NUMBER: _ClassVar[int]
    HEARTBEAT_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_CHANGED_FIELD_NUMBER: _ClassVar[int]
    USER_DATA_CHANGED_FIELD_NUMBER: _ClassVar[int]
    SESSIONS_CHANGED_FIELD_NUMBER: _ClassVar[int]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    JOB_CHANGED_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_CHANGED_FIELD_NUMBER: _ClassVar[int]
    SYNC_PLAY_FIELD_NUMBER: _ClassVar[int]
    NOTIFICATION_FIELD_NUMBER: _ClassVar[int]
    SCAN_PROGRESS_CHANGED_FIELD_NUMBER: _ClassVar[int]
    time: _timestamp_pb2.Timestamp
    connected: Connected
    heartbeat: Heartbeat
    library_changed: LibraryChanged
    user_data_changed: UserDataChanged
    sessions_changed: SessionsChanged
    command: CommandReceived
    job_changed: JobChanged
    plugin_changed: PluginChanged
    sync_play: _syncplay_pb2.SyncPlayUpdate
    notification: NotificationReceived
    scan_progress_changed: ScanProgressChanged
    def __init__(self, time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., connected: _Optional[_Union[Connected, _Mapping]] = ..., heartbeat: _Optional[_Union[Heartbeat, _Mapping]] = ..., library_changed: _Optional[_Union[LibraryChanged, _Mapping]] = ..., user_data_changed: _Optional[_Union[UserDataChanged, _Mapping]] = ..., sessions_changed: _Optional[_Union[SessionsChanged, _Mapping]] = ..., command: _Optional[_Union[CommandReceived, _Mapping]] = ..., job_changed: _Optional[_Union[JobChanged, _Mapping]] = ..., plugin_changed: _Optional[_Union[PluginChanged, _Mapping]] = ..., sync_play: _Optional[_Union[_syncplay_pb2.SyncPlayUpdate, _Mapping]] = ..., notification: _Optional[_Union[NotificationReceived, _Mapping]] = ..., scan_progress_changed: _Optional[_Union[ScanProgressChanged, _Mapping]] = ...) -> None: ...

class Connected(_message.Message):
    __slots__ = ("session_id",)
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    def __init__(self, session_id: _Optional[str] = ...) -> None: ...

class Heartbeat(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class LibraryChanged(_message.Message):
    __slots__ = ("library_ids", "changed_item_ids", "removed_item_ids")
    LIBRARY_IDS_FIELD_NUMBER: _ClassVar[int]
    CHANGED_ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    REMOVED_ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    library_ids: _containers.RepeatedScalarFieldContainer[str]
    changed_item_ids: _containers.RepeatedScalarFieldContainer[str]
    removed_item_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, library_ids: _Optional[_Iterable[str]] = ..., changed_item_ids: _Optional[_Iterable[str]] = ..., removed_item_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class UserDataChanged(_message.Message):
    __slots__ = ("user_data",)
    USER_DATA_FIELD_NUMBER: _ClassVar[int]
    user_data: _containers.RepeatedCompositeFieldContainer[_user_pb2.UserData]
    def __init__(self, user_data: _Optional[_Iterable[_Union[_user_pb2.UserData, _Mapping]]] = ...) -> None: ...

class SessionsChanged(_message.Message):
    __slots__ = ("session_id",)
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    def __init__(self, session_id: _Optional[str] = ...) -> None: ...

class CommandReceived(_message.Message):
    __slots__ = ("from_session_id", "from_user_name", "command")
    FROM_SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    FROM_USER_NAME_FIELD_NUMBER: _ClassVar[int]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    from_session_id: str
    from_user_name: str
    command: Command
    def __init__(self, from_session_id: _Optional[str] = ..., from_user_name: _Optional[str] = ..., command: _Optional[_Union[Command, _Mapping]] = ...) -> None: ...

class JobChanged(_message.Message):
    __slots__ = ("job_id", "kind", "library_id", "state")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_ID_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    kind: str
    library_id: str
    state: JobState
    def __init__(self, job_id: _Optional[str] = ..., kind: _Optional[str] = ..., library_id: _Optional[str] = ..., state: _Optional[_Union[JobState, str]] = ...) -> None: ...

class ScanProgressChanged(_message.Message):
    __slots__ = ("library_id", "progress")
    LIBRARY_ID_FIELD_NUMBER: _ClassVar[int]
    PROGRESS_FIELD_NUMBER: _ClassVar[int]
    library_id: str
    progress: _library_pb2.ScanProgress
    def __init__(self, library_id: _Optional[str] = ..., progress: _Optional[_Union[_library_pb2.ScanProgress, _Mapping]] = ...) -> None: ...

class NotificationReceived(_message.Message):
    __slots__ = ("activity",)
    ACTIVITY_FIELD_NUMBER: _ClassVar[int]
    activity: _activity_pb2.Activity
    def __init__(self, activity: _Optional[_Union[_activity_pb2.Activity, _Mapping]] = ...) -> None: ...

class PluginChanged(_message.Message):
    __slots__ = ("plugin",)
    PLUGIN_FIELD_NUMBER: _ClassVar[int]
    plugin: _plugin_pb2.Plugin
    def __init__(self, plugin: _Optional[_Union[_plugin_pb2.Plugin, _Mapping]] = ...) -> None: ...

class Session(_message.Message):
    __slots__ = ("id", "user_id", "user_name", "device_id", "device_name", "client", "client_version", "last_seen_time", "online", "now_playing", "plugin_id", "create_time", "current")
    ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    USER_NAME_FIELD_NUMBER: _ClassVar[int]
    DEVICE_ID_FIELD_NUMBER: _ClassVar[int]
    DEVICE_NAME_FIELD_NUMBER: _ClassVar[int]
    CLIENT_FIELD_NUMBER: _ClassVar[int]
    CLIENT_VERSION_FIELD_NUMBER: _ClassVar[int]
    LAST_SEEN_TIME_FIELD_NUMBER: _ClassVar[int]
    ONLINE_FIELD_NUMBER: _ClassVar[int]
    NOW_PLAYING_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    CURRENT_FIELD_NUMBER: _ClassVar[int]
    id: str
    user_id: str
    user_name: str
    device_id: str
    device_name: str
    client: str
    client_version: str
    last_seen_time: _timestamp_pb2.Timestamp
    online: bool
    now_playing: NowPlaying
    plugin_id: str
    create_time: _timestamp_pb2.Timestamp
    current: bool
    def __init__(self, id: _Optional[str] = ..., user_id: _Optional[str] = ..., user_name: _Optional[str] = ..., device_id: _Optional[str] = ..., device_name: _Optional[str] = ..., client: _Optional[str] = ..., client_version: _Optional[str] = ..., last_seen_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., online: _Optional[bool] = ..., now_playing: _Optional[_Union[NowPlaying, _Mapping]] = ..., plugin_id: _Optional[str] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., current: _Optional[bool] = ...) -> None: ...

class NowPlaying(_message.Message):
    __slots__ = ("playback_id", "item", "position", "paused", "method")
    PLAYBACK_ID_FIELD_NUMBER: _ClassVar[int]
    ITEM_FIELD_NUMBER: _ClassVar[int]
    POSITION_FIELD_NUMBER: _ClassVar[int]
    PAUSED_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    playback_id: str
    item: _item_pb2.Item
    position: _duration_pb2.Duration
    paused: bool
    method: _playback_pb2.PlayMethod
    def __init__(self, playback_id: _Optional[str] = ..., item: _Optional[_Union[_item_pb2.Item, _Mapping]] = ..., position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., paused: _Optional[bool] = ..., method: _Optional[_Union[_playback_pb2.PlayMethod, str]] = ...) -> None: ...

class Command(_message.Message):
    __slots__ = ("play", "play_state", "seek", "message", "volume")
    PLAY_FIELD_NUMBER: _ClassVar[int]
    PLAY_STATE_FIELD_NUMBER: _ClassVar[int]
    SEEK_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    VOLUME_FIELD_NUMBER: _ClassVar[int]
    play: Play
    play_state: PlayState
    seek: Seek
    message: Message
    volume: Volume
    def __init__(self, play: _Optional[_Union[Play, _Mapping]] = ..., play_state: _Optional[_Union[PlayState, _Mapping]] = ..., seek: _Optional[_Union[Seek, _Mapping]] = ..., message: _Optional[_Union[Message, _Mapping]] = ..., volume: _Optional[_Union[Volume, _Mapping]] = ...) -> None: ...

class Play(_message.Message):
    __slots__ = ("item_ids", "start_index", "start_position")
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    START_INDEX_FIELD_NUMBER: _ClassVar[int]
    START_POSITION_FIELD_NUMBER: _ClassVar[int]
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    start_index: int
    start_position: _duration_pb2.Duration
    def __init__(self, item_ids: _Optional[_Iterable[str]] = ..., start_index: _Optional[int] = ..., start_position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class PlayState(_message.Message):
    __slots__ = ("command",)
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    command: PlayStateCommand
    def __init__(self, command: _Optional[_Union[PlayStateCommand, str]] = ...) -> None: ...

class Seek(_message.Message):
    __slots__ = ("position",)
    POSITION_FIELD_NUMBER: _ClassVar[int]
    position: _duration_pb2.Duration
    def __init__(self, position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class Message(_message.Message):
    __slots__ = ("text", "timeout")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_FIELD_NUMBER: _ClassVar[int]
    text: str
    timeout: _duration_pb2.Duration
    def __init__(self, text: _Optional[str] = ..., timeout: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class Volume(_message.Message):
    __slots__ = ("level", "muted")
    LEVEL_FIELD_NUMBER: _ClassVar[int]
    MUTED_FIELD_NUMBER: _ClassVar[int]
    level: int
    muted: bool
    def __init__(self, level: _Optional[int] = ..., muted: _Optional[bool] = ...) -> None: ...

class ListSessionsRequest(_message.Message):
    __slots__ = ("all_users",)
    ALL_USERS_FIELD_NUMBER: _ClassVar[int]
    all_users: bool
    def __init__(self, all_users: _Optional[bool] = ...) -> None: ...

class ListSessionsResponse(_message.Message):
    __slots__ = ("sessions",)
    SESSIONS_FIELD_NUMBER: _ClassVar[int]
    sessions: _containers.RepeatedCompositeFieldContainer[Session]
    def __init__(self, sessions: _Optional[_Iterable[_Union[Session, _Mapping]]] = ...) -> None: ...

class RevokeSessionRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class RevokeSessionResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SendCommandRequest(_message.Message):
    __slots__ = ("session_id", "command")
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    command: Command
    def __init__(self, session_id: _Optional[str] = ..., command: _Optional[_Union[Command, _Mapping]] = ...) -> None: ...

class SendCommandResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

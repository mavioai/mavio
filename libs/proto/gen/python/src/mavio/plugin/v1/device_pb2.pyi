import datetime

from google.protobuf import duration_pb2 as _duration_pb2
from mavio.plugin.v1 import challenge_pb2 as _challenge_pb2
from mavio.plugin.v1 import container_pb2 as _container_pb2
from mavio.plugin.v1 import proxy_pb2 as _proxy_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class CommandKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    COMMAND_KIND_UNSPECIFIED: _ClassVar[CommandKind]
    COMMAND_KIND_PLAY: _ClassVar[CommandKind]
    COMMAND_KIND_PLAY_STATE: _ClassVar[CommandKind]
    COMMAND_KIND_SEEK: _ClassVar[CommandKind]
    COMMAND_KIND_MESSAGE: _ClassVar[CommandKind]
    COMMAND_KIND_VOLUME: _ClassVar[CommandKind]

class PlayStateCommand(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PLAY_STATE_COMMAND_UNSPECIFIED: _ClassVar[PlayStateCommand]
    PLAY_STATE_COMMAND_PAUSE: _ClassVar[PlayStateCommand]
    PLAY_STATE_COMMAND_UNPAUSE: _ClassVar[PlayStateCommand]
    PLAY_STATE_COMMAND_STOP: _ClassVar[PlayStateCommand]
    PLAY_STATE_COMMAND_NEXT: _ClassVar[PlayStateCommand]
    PLAY_STATE_COMMAND_PREVIOUS: _ClassVar[PlayStateCommand]
COMMAND_KIND_UNSPECIFIED: CommandKind
COMMAND_KIND_PLAY: CommandKind
COMMAND_KIND_PLAY_STATE: CommandKind
COMMAND_KIND_SEEK: CommandKind
COMMAND_KIND_MESSAGE: CommandKind
COMMAND_KIND_VOLUME: CommandKind
PLAY_STATE_COMMAND_UNSPECIFIED: PlayStateCommand
PLAY_STATE_COMMAND_PAUSE: PlayStateCommand
PLAY_STATE_COMMAND_UNPAUSE: PlayStateCommand
PLAY_STATE_COMMAND_STOP: PlayStateCommand
PLAY_STATE_COMMAND_NEXT: PlayStateCommand
PLAY_STATE_COMMAND_PREVIOUS: PlayStateCommand

class Device(_message.Message):
    __slots__ = ("id", "name", "product", "commands")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    PRODUCT_FIELD_NUMBER: _ClassVar[int]
    COMMANDS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    product: str
    commands: _containers.RepeatedScalarFieldContainer[CommandKind]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., product: _Optional[str] = ..., commands: _Optional[_Iterable[_Union[CommandKind, str]]] = ...) -> None: ...

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
    message: ShowMessage
    volume: Volume
    def __init__(self, play: _Optional[_Union[Play, _Mapping]] = ..., play_state: _Optional[_Union[PlayState, _Mapping]] = ..., seek: _Optional[_Union[Seek, _Mapping]] = ..., message: _Optional[_Union[ShowMessage, _Mapping]] = ..., volume: _Optional[_Union[Volume, _Mapping]] = ...) -> None: ...

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

class ShowMessage(_message.Message):
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

class SetDevicesRequest(_message.Message):
    __slots__ = ("devices",)
    DEVICES_FIELD_NUMBER: _ClassVar[int]
    devices: _containers.RepeatedCompositeFieldContainer[Device]
    def __init__(self, devices: _Optional[_Iterable[_Union[Device, _Mapping]]] = ...) -> None: ...

class SetDevicesResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SendCommandRequest(_message.Message):
    __slots__ = ("device_id", "session_id", "user_id", "user_name", "command")
    DEVICE_ID_FIELD_NUMBER: _ClassVar[int]
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    USER_NAME_FIELD_NUMBER: _ClassVar[int]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    device_id: str
    session_id: str
    user_id: str
    user_name: str
    command: Command
    def __init__(self, device_id: _Optional[str] = ..., session_id: _Optional[str] = ..., user_id: _Optional[str] = ..., user_name: _Optional[str] = ..., command: _Optional[_Union[Command, _Mapping]] = ...) -> None: ...

class SendCommandResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetServerInfoRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetServerInfoResponse(_message.Message):
    __slots__ = ("server_name", "version", "http_port", "https_port", "base_url")
    SERVER_NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    HTTP_PORT_FIELD_NUMBER: _ClassVar[int]
    HTTPS_PORT_FIELD_NUMBER: _ClassVar[int]
    BASE_URL_FIELD_NUMBER: _ClassVar[int]
    server_name: str
    version: str
    http_port: int
    https_port: int
    base_url: str
    def __init__(self, server_name: _Optional[str] = ..., version: _Optional[str] = ..., http_port: _Optional[int] = ..., https_port: _Optional[int] = ..., base_url: _Optional[str] = ...) -> None: ...

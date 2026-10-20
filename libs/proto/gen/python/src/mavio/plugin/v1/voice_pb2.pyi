import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from mavio.plugin.v1 import settings_pb2 as _settings_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class VoiceMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    VOICE_MODE_UNSPECIFIED: _ClassVar[VoiceMode]
    VOICE_MODE_CONVERSATION: _ClassVar[VoiceMode]
    VOICE_MODE_PUSH_TO_TALK: _ClassVar[VoiceMode]
    VOICE_MODE_DICTATION: _ClassVar[VoiceMode]
VOICE_MODE_UNSPECIFIED: VoiceMode
VOICE_MODE_CONVERSATION: VoiceMode
VOICE_MODE_PUSH_TO_TALK: VoiceMode
VOICE_MODE_DICTATION: VoiceMode

class ListVoiceServicesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListVoiceServicesResponse(_message.Message):
    __slots__ = ("services", "recognizers", "voices")
    SERVICES_FIELD_NUMBER: _ClassVar[int]
    RECOGNIZERS_FIELD_NUMBER: _ClassVar[int]
    VOICES_FIELD_NUMBER: _ClassVar[int]
    services: _containers.RepeatedCompositeFieldContainer[VoiceServiceInfo]
    recognizers: _containers.RepeatedCompositeFieldContainer[VoiceEngineInfo]
    voices: _containers.RepeatedCompositeFieldContainer[VoiceEngineInfo]
    def __init__(self, services: _Optional[_Iterable[_Union[VoiceServiceInfo, _Mapping]]] = ..., recognizers: _Optional[_Iterable[_Union[VoiceEngineInfo, _Mapping]]] = ..., voices: _Optional[_Iterable[_Union[VoiceEngineInfo, _Mapping]]] = ...) -> None: ...

class VoiceServiceInfo(_message.Message):
    __slots__ = ("name", "display_name", "recognizes", "settings")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    RECOGNIZES_FIELD_NUMBER: _ClassVar[int]
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    name: str
    display_name: str
    recognizes: bool
    settings: _containers.RepeatedCompositeFieldContainer[_settings_pb2.SettingField]
    def __init__(self, name: _Optional[str] = ..., display_name: _Optional[str] = ..., recognizes: _Optional[bool] = ..., settings: _Optional[_Iterable[_Union[_settings_pb2.SettingField, _Mapping]]] = ...) -> None: ...

class VoiceEngineInfo(_message.Message):
    __slots__ = ("id", "name", "service", "enabled", "languages", "hints", "ready")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    SERVICE_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    HINTS_FIELD_NUMBER: _ClassVar[int]
    READY_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    service: str
    enabled: bool
    languages: _containers.RepeatedScalarFieldContainer[str]
    hints: bool
    ready: bool
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., service: _Optional[str] = ..., enabled: _Optional[bool] = ..., languages: _Optional[_Iterable[str]] = ..., hints: _Optional[bool] = ..., ready: _Optional[bool] = ...) -> None: ...

class VoiceTurnSettings(_message.Message):
    __slots__ = ("commit_delay", "longest_pause", "idle")
    COMMIT_DELAY_FIELD_NUMBER: _ClassVar[int]
    LONGEST_PAUSE_FIELD_NUMBER: _ClassVar[int]
    IDLE_FIELD_NUMBER: _ClassVar[int]
    commit_delay: _duration_pb2.Duration
    longest_pause: _duration_pb2.Duration
    idle: _duration_pb2.Duration
    def __init__(self, commit_delay: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., longest_pause: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., idle: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class OpenVoiceSessionRequest(_message.Message):
    __slots__ = ("session_id", "language", "mode", "recognizer", "voice", "hints", "turns", "session_token", "context_json")
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    RECOGNIZER_FIELD_NUMBER: _ClassVar[int]
    VOICE_FIELD_NUMBER: _ClassVar[int]
    HINTS_FIELD_NUMBER: _ClassVar[int]
    TURNS_FIELD_NUMBER: _ClassVar[int]
    SESSION_TOKEN_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_JSON_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    language: str
    mode: VoiceMode
    recognizer: str
    voice: str
    hints: _containers.RepeatedScalarFieldContainer[str]
    turns: VoiceTurnSettings
    session_token: str
    context_json: str
    def __init__(self, session_id: _Optional[str] = ..., language: _Optional[str] = ..., mode: _Optional[_Union[VoiceMode, str]] = ..., recognizer: _Optional[str] = ..., voice: _Optional[str] = ..., hints: _Optional[_Iterable[str]] = ..., turns: _Optional[_Union[VoiceTurnSettings, _Mapping]] = ..., session_token: _Optional[str] = ..., context_json: _Optional[str] = ...) -> None: ...

class OpenVoiceSessionResponse(_message.Message):
    __slots__ = ("path",)
    PATH_FIELD_NUMBER: _ClassVar[int]
    path: str
    def __init__(self, path: _Optional[str] = ...) -> None: ...

class CloseVoiceSessionRequest(_message.Message):
    __slots__ = ("session_id",)
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    def __init__(self, session_id: _Optional[str] = ...) -> None: ...

class CloseVoiceSessionResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

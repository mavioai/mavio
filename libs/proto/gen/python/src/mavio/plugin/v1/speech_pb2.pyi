import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from mavio.plugin.v1 import settings_pb2 as _settings_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ListTranscribersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListTranscribersResponse(_message.Message):
    __slots__ = ("services", "transcribers")
    SERVICES_FIELD_NUMBER: _ClassVar[int]
    TRANSCRIBERS_FIELD_NUMBER: _ClassVar[int]
    services: _containers.RepeatedCompositeFieldContainer[TranscriptionServiceInfo]
    transcribers: _containers.RepeatedCompositeFieldContainer[TranscriberInfo]
    def __init__(self, services: _Optional[_Iterable[_Union[TranscriptionServiceInfo, _Mapping]]] = ..., transcribers: _Optional[_Iterable[_Union[TranscriberInfo, _Mapping]]] = ...) -> None: ...

class TranscriptionServiceInfo(_message.Message):
    __slots__ = ("name", "display_name", "settings")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    name: str
    display_name: str
    settings: _containers.RepeatedCompositeFieldContainer[_settings_pb2.SettingField]
    def __init__(self, name: _Optional[str] = ..., display_name: _Optional[str] = ..., settings: _Optional[_Iterable[_Union[_settings_pb2.SettingField, _Mapping]]] = ...) -> None: ...

class TranscriberInfo(_message.Message):
    __slots__ = ("id", "name", "service", "enabled", "ready", "languages", "max_duration", "translates")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    SERVICE_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    READY_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    MAX_DURATION_FIELD_NUMBER: _ClassVar[int]
    TRANSLATES_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    service: str
    enabled: bool
    ready: bool
    languages: _containers.RepeatedScalarFieldContainer[str]
    max_duration: _duration_pb2.Duration
    translates: bool
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., service: _Optional[str] = ..., enabled: _Optional[bool] = ..., ready: _Optional[bool] = ..., languages: _Optional[_Iterable[str]] = ..., max_duration: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., translates: _Optional[bool] = ...) -> None: ...

class SpeechAudio(_message.Message):
    __slots__ = ("url", "limit")
    URL_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    url: str
    limit: _duration_pb2.Duration
    def __init__(self, url: _Optional[str] = ..., limit: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class TranscribeSpeechRequest(_message.Message):
    __slots__ = ("transcriber", "audio", "language", "translate", "name")
    TRANSCRIBER_FIELD_NUMBER: _ClassVar[int]
    AUDIO_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    TRANSLATE_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    transcriber: str
    audio: SpeechAudio
    language: str
    translate: bool
    name: str
    def __init__(self, transcriber: _Optional[str] = ..., audio: _Optional[_Union[SpeechAudio, _Mapping]] = ..., language: _Optional[str] = ..., translate: _Optional[bool] = ..., name: _Optional[str] = ...) -> None: ...

class TranscribeSpeechResponse(_message.Message):
    __slots__ = ("language", "segments")
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    SEGMENTS_FIELD_NUMBER: _ClassVar[int]
    language: str
    segments: _containers.RepeatedCompositeFieldContainer[SpeechSegment]
    def __init__(self, language: _Optional[str] = ..., segments: _Optional[_Iterable[_Union[SpeechSegment, _Mapping]]] = ...) -> None: ...

class SpeechSegment(_message.Message):
    __slots__ = ("start", "end", "text")
    START_FIELD_NUMBER: _ClassVar[int]
    END_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    start: _duration_pb2.Duration
    end: _duration_pb2.Duration
    text: str
    def __init__(self, start: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., end: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., text: _Optional[str] = ...) -> None: ...

class DetectSpeechLanguageRequest(_message.Message):
    __slots__ = ("transcriber", "audio")
    TRANSCRIBER_FIELD_NUMBER: _ClassVar[int]
    AUDIO_FIELD_NUMBER: _ClassVar[int]
    transcriber: str
    audio: SpeechAudio
    def __init__(self, transcriber: _Optional[str] = ..., audio: _Optional[_Union[SpeechAudio, _Mapping]] = ...) -> None: ...

class DetectSpeechLanguageResponse(_message.Message):
    __slots__ = ("language",)
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    language: str
    def __init__(self, language: _Optional[str] = ...) -> None: ...

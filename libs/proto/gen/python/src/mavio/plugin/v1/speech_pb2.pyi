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

class SpeechFormat(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SPEECH_FORMAT_UNSPECIFIED: _ClassVar[SpeechFormat]
    SPEECH_FORMAT_OGG_OPUS: _ClassVar[SpeechFormat]
    SPEECH_FORMAT_MP3: _ClassVar[SpeechFormat]
SPEECH_FORMAT_UNSPECIFIED: SpeechFormat
SPEECH_FORMAT_OGG_OPUS: SpeechFormat
SPEECH_FORMAT_MP3: SpeechFormat

class ListRecognizersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListRecognizersResponse(_message.Message):
    __slots__ = ("recognizers", "services")
    RECOGNIZERS_FIELD_NUMBER: _ClassVar[int]
    SERVICES_FIELD_NUMBER: _ClassVar[int]
    recognizers: _containers.RepeatedCompositeFieldContainer[RecognizerInfo]
    services: _containers.RepeatedCompositeFieldContainer[RecognizerInfo]
    def __init__(self, recognizers: _Optional[_Iterable[_Union[RecognizerInfo, _Mapping]]] = ..., services: _Optional[_Iterable[_Union[RecognizerInfo, _Mapping]]] = ...) -> None: ...

class RecognizerInfo(_message.Message):
    __slots__ = ("name", "display_name", "settings", "enabled", "languages", "timestamps", "detects_language", "translates", "hints", "max_duration", "subtitles", "service")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMPS_FIELD_NUMBER: _ClassVar[int]
    DETECTS_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    TRANSLATES_FIELD_NUMBER: _ClassVar[int]
    HINTS_FIELD_NUMBER: _ClassVar[int]
    MAX_DURATION_FIELD_NUMBER: _ClassVar[int]
    SUBTITLES_FIELD_NUMBER: _ClassVar[int]
    SERVICE_FIELD_NUMBER: _ClassVar[int]
    name: str
    display_name: str
    settings: _containers.RepeatedCompositeFieldContainer[_settings_pb2.SettingField]
    enabled: bool
    languages: _containers.RepeatedScalarFieldContainer[str]
    timestamps: bool
    detects_language: bool
    translates: bool
    hints: bool
    max_duration: _duration_pb2.Duration
    subtitles: bool
    service: str
    def __init__(self, name: _Optional[str] = ..., display_name: _Optional[str] = ..., settings: _Optional[_Iterable[_Union[_settings_pb2.SettingField, _Mapping]]] = ..., enabled: _Optional[bool] = ..., languages: _Optional[_Iterable[str]] = ..., timestamps: _Optional[bool] = ..., detects_language: _Optional[bool] = ..., translates: _Optional[bool] = ..., hints: _Optional[bool] = ..., max_duration: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., subtitles: _Optional[bool] = ..., service: _Optional[str] = ...) -> None: ...

class SpeechAudio(_message.Message):
    __slots__ = ("pcm", "url", "limit")
    PCM_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    pcm: bytes
    url: str
    limit: _duration_pb2.Duration
    def __init__(self, pcm: _Optional[bytes] = ..., url: _Optional[str] = ..., limit: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class RecognizeSpeechRequest(_message.Message):
    __slots__ = ("recognizer", "audio", "language", "translate", "hints", "timestamps", "name")
    RECOGNIZER_FIELD_NUMBER: _ClassVar[int]
    AUDIO_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    TRANSLATE_FIELD_NUMBER: _ClassVar[int]
    HINTS_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMPS_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    recognizer: str
    audio: SpeechAudio
    language: str
    translate: bool
    hints: _containers.RepeatedScalarFieldContainer[str]
    timestamps: bool
    name: str
    def __init__(self, recognizer: _Optional[str] = ..., audio: _Optional[_Union[SpeechAudio, _Mapping]] = ..., language: _Optional[str] = ..., translate: _Optional[bool] = ..., hints: _Optional[_Iterable[str]] = ..., timestamps: _Optional[bool] = ..., name: _Optional[str] = ...) -> None: ...

class RecognizeSpeechResponse(_message.Message):
    __slots__ = ("text", "language", "segments")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    SEGMENTS_FIELD_NUMBER: _ClassVar[int]
    text: str
    language: str
    segments: _containers.RepeatedCompositeFieldContainer[SpeechSegment]
    def __init__(self, text: _Optional[str] = ..., language: _Optional[str] = ..., segments: _Optional[_Iterable[_Union[SpeechSegment, _Mapping]]] = ...) -> None: ...

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
    __slots__ = ("recognizer", "audio")
    RECOGNIZER_FIELD_NUMBER: _ClassVar[int]
    AUDIO_FIELD_NUMBER: _ClassVar[int]
    recognizer: str
    audio: SpeechAudio
    def __init__(self, recognizer: _Optional[str] = ..., audio: _Optional[_Union[SpeechAudio, _Mapping]] = ...) -> None: ...

class DetectSpeechLanguageResponse(_message.Message):
    __slots__ = ("language",)
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    language: str
    def __init__(self, language: _Optional[str] = ...) -> None: ...

class ListVoicesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListVoicesResponse(_message.Message):
    __slots__ = ("voices", "synthesizers", "services")
    VOICES_FIELD_NUMBER: _ClassVar[int]
    SYNTHESIZERS_FIELD_NUMBER: _ClassVar[int]
    SERVICES_FIELD_NUMBER: _ClassVar[int]
    voices: _containers.RepeatedCompositeFieldContainer[VoiceInfo]
    synthesizers: _containers.RepeatedCompositeFieldContainer[SynthesizerInfo]
    services: _containers.RepeatedCompositeFieldContainer[SynthesizerInfo]
    def __init__(self, voices: _Optional[_Iterable[_Union[VoiceInfo, _Mapping]]] = ..., synthesizers: _Optional[_Iterable[_Union[SynthesizerInfo, _Mapping]]] = ..., services: _Optional[_Iterable[_Union[SynthesizerInfo, _Mapping]]] = ...) -> None: ...

class SynthesizerInfo(_message.Message):
    __slots__ = ("name", "display_name", "settings", "enabled", "service")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    SERVICE_FIELD_NUMBER: _ClassVar[int]
    name: str
    display_name: str
    settings: _containers.RepeatedCompositeFieldContainer[_settings_pb2.SettingField]
    enabled: bool
    service: str
    def __init__(self, name: _Optional[str] = ..., display_name: _Optional[str] = ..., settings: _Optional[_Iterable[_Union[_settings_pb2.SettingField, _Mapping]]] = ..., enabled: _Optional[bool] = ..., service: _Optional[str] = ...) -> None: ...

class VoiceInfo(_message.Message):
    __slots__ = ("id", "name", "languages")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    languages: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., languages: _Optional[_Iterable[str]] = ...) -> None: ...

class SynthesizeSpeechRequest(_message.Message):
    __slots__ = ("voice", "text", "language", "format", "speed")
    VOICE_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    SPEED_FIELD_NUMBER: _ClassVar[int]
    voice: str
    text: str
    language: str
    format: SpeechFormat
    speed: float
    def __init__(self, voice: _Optional[str] = ..., text: _Optional[str] = ..., language: _Optional[str] = ..., format: _Optional[_Union[SpeechFormat, str]] = ..., speed: _Optional[float] = ...) -> None: ...

class SynthesizeSpeechResponse(_message.Message):
    __slots__ = ("audio", "content_type")
    AUDIO_FIELD_NUMBER: _ClassVar[int]
    CONTENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    audio: bytes
    content_type: str
    def __init__(self, audio: _Optional[bytes] = ..., content_type: _Optional[str] = ...) -> None: ...

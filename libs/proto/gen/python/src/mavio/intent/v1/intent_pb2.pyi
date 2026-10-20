from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class IntentMatch(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    INTENT_MATCH_UNSPECIFIED: _ClassVar[IntentMatch]
    INTENT_MATCH_FULL: _ClassVar[IntentMatch]
    INTENT_MATCH_PARTIAL: _ClassVar[IntentMatch]
INTENT_MATCH_UNSPECIFIED: IntentMatch
INTENT_MATCH_FULL: IntentMatch
INTENT_MATCH_PARTIAL: IntentMatch

class ParseIntentRequest(_message.Message):
    __slots__ = ("text", "languages", "state")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    text: str
    languages: _containers.RepeatedScalarFieldContainer[str]
    state: DialogState
    def __init__(self, text: _Optional[str] = ..., languages: _Optional[_Iterable[str]] = ..., state: _Optional[_Union[DialogState, _Mapping]] = ...) -> None: ...

class DialogState(_message.Message):
    __slots__ = ("confirmation", "results", "playing")
    CONFIRMATION_FIELD_NUMBER: _ClassVar[int]
    RESULTS_FIELD_NUMBER: _ClassVar[int]
    PLAYING_FIELD_NUMBER: _ClassVar[int]
    confirmation: bool
    results: int
    playing: bool
    def __init__(self, confirmation: _Optional[bool] = ..., results: _Optional[int] = ..., playing: _Optional[bool] = ...) -> None: ...

class ParseIntentResponse(_message.Message):
    __slots__ = ("intents",)
    INTENTS_FIELD_NUMBER: _ClassVar[int]
    intents: _containers.RepeatedCompositeFieldContainer[Intent]
    def __init__(self, intents: _Optional[_Iterable[_Union[Intent, _Mapping]]] = ...) -> None: ...

class Intent(_message.Message):
    __slots__ = ("name", "slots", "match", "confidence", "language")
    NAME_FIELD_NUMBER: _ClassVar[int]
    SLOTS_FIELD_NUMBER: _ClassVar[int]
    MATCH_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    name: str
    slots: _containers.RepeatedCompositeFieldContainer[Slot]
    match: IntentMatch
    confidence: float
    language: str
    def __init__(self, name: _Optional[str] = ..., slots: _Optional[_Iterable[_Union[Slot, _Mapping]]] = ..., match: _Optional[_Union[IntentMatch, str]] = ..., confidence: _Optional[float] = ..., language: _Optional[str] = ...) -> None: ...

class Slot(_message.Message):
    __slots__ = ("name", "text", "number", "seconds", "language", "ordinal", "keyword")
    NAME_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    NUMBER_FIELD_NUMBER: _ClassVar[int]
    SECONDS_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    ORDINAL_FIELD_NUMBER: _ClassVar[int]
    KEYWORD_FIELD_NUMBER: _ClassVar[int]
    name: str
    text: str
    number: float
    seconds: float
    language: str
    ordinal: int
    keyword: str
    def __init__(self, name: _Optional[str] = ..., text: _Optional[str] = ..., number: _Optional[float] = ..., seconds: _Optional[float] = ..., language: _Optional[str] = ..., ordinal: _Optional[int] = ..., keyword: _Optional[str] = ...) -> None: ...

class ReplyRequest(_message.Message):
    __slots__ = ("language", "key", "args")
    class ArgsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    KEY_FIELD_NUMBER: _ClassVar[int]
    ARGS_FIELD_NUMBER: _ClassVar[int]
    language: str
    key: str
    args: _containers.ScalarMap[str, str]
    def __init__(self, language: _Optional[str] = ..., key: _Optional[str] = ..., args: _Optional[_Mapping[str, str]] = ...) -> None: ...

class ReplyResponse(_message.Message):
    __slots__ = ("text", "language")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    text: str
    language: str
    def __init__(self, text: _Optional[str] = ..., language: _Optional[str] = ...) -> None: ...

class ExamplesRequest(_message.Message):
    __slots__ = ("language",)
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    language: str
    def __init__(self, language: _Optional[str] = ...) -> None: ...

class ExamplesResponse(_message.Message):
    __slots__ = ("examples", "language")
    EXAMPLES_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    examples: _containers.RepeatedScalarFieldContainer[str]
    language: str
    def __init__(self, examples: _Optional[_Iterable[str]] = ..., language: _Optional[str] = ...) -> None: ...

class LanguagesResponse(_message.Message):
    __slots__ = ("languages",)
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    languages: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, languages: _Optional[_Iterable[str]] = ...) -> None: ...

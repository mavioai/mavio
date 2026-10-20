from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Country(_message.Message):
    __slots__ = ("code", "three_letter_code", "name")
    CODE_FIELD_NUMBER: _ClassVar[int]
    THREE_LETTER_CODE_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    code: str
    three_letter_code: str
    name: str
    def __init__(self, code: _Optional[str] = ..., three_letter_code: _Optional[str] = ..., name: _Optional[str] = ...) -> None: ...

class Language(_message.Message):
    __slots__ = ("code", "three_letter_code", "name")
    CODE_FIELD_NUMBER: _ClassVar[int]
    THREE_LETTER_CODE_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    code: str
    three_letter_code: str
    name: str
    def __init__(self, code: _Optional[str] = ..., three_letter_code: _Optional[str] = ..., name: _Optional[str] = ...) -> None: ...

class Rating(_message.Message):
    __slots__ = ("name", "score")
    NAME_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    name: str
    score: int
    def __init__(self, name: _Optional[str] = ..., score: _Optional[int] = ...) -> None: ...

class ListCountriesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListCountriesResponse(_message.Message):
    __slots__ = ("countries",)
    COUNTRIES_FIELD_NUMBER: _ClassVar[int]
    countries: _containers.RepeatedCompositeFieldContainer[Country]
    def __init__(self, countries: _Optional[_Iterable[_Union[Country, _Mapping]]] = ...) -> None: ...

class ListLanguagesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListLanguagesResponse(_message.Message):
    __slots__ = ("languages",)
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    languages: _containers.RepeatedCompositeFieldContainer[Language]
    def __init__(self, languages: _Optional[_Iterable[_Union[Language, _Mapping]]] = ...) -> None: ...

class ListRatingsRequest(_message.Message):
    __slots__ = ("country",)
    COUNTRY_FIELD_NUMBER: _ClassVar[int]
    country: str
    def __init__(self, country: _Optional[str] = ...) -> None: ...

class ListRatingsResponse(_message.Message):
    __slots__ = ("ratings",)
    RATINGS_FIELD_NUMBER: _ClassVar[int]
    ratings: _containers.RepeatedCompositeFieldContainer[Rating]
    def __init__(self, ratings: _Optional[_Iterable[_Union[Rating, _Mapping]]] = ...) -> None: ...

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SceneSearchMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SCENE_SEARCH_MODE_UNSPECIFIED: _ClassVar[SceneSearchMode]
    SCENE_SEARCH_MODE_ID: _ClassVar[SceneSearchMode]
    SCENE_SEARCH_MODE_TITLE: _ClassVar[SceneSearchMode]
    SCENE_SEARCH_MODE_ID_AND_TITLE: _ClassVar[SceneSearchMode]
SCENE_SEARCH_MODE_UNSPECIFIED: SceneSearchMode
SCENE_SEARCH_MODE_ID: SceneSearchMode
SCENE_SEARCH_MODE_TITLE: SceneSearchMode
SCENE_SEARCH_MODE_ID_AND_TITLE: SceneSearchMode

class ListSceneMappingsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListSceneMappingsResponse(_message.Message):
    __slots__ = ("mappings",)
    MAPPINGS_FIELD_NUMBER: _ClassVar[int]
    mappings: _containers.RepeatedCompositeFieldContainer[SceneMapping]
    def __init__(self, mappings: _Optional[_Iterable[_Union[SceneMapping, _Mapping]]] = ...) -> None: ...

class SceneMapping(_message.Message):
    __slots__ = ("source", "series_external_ids", "title", "search_title", "season_number", "scene_season_number", "scene_origin", "search_mode", "filter_regex", "comment")
    class SeriesExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    SERIES_EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    SEARCH_TITLE_FIELD_NUMBER: _ClassVar[int]
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    SCENE_SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    SCENE_ORIGIN_FIELD_NUMBER: _ClassVar[int]
    SEARCH_MODE_FIELD_NUMBER: _ClassVar[int]
    FILTER_REGEX_FIELD_NUMBER: _ClassVar[int]
    COMMENT_FIELD_NUMBER: _ClassVar[int]
    source: str
    series_external_ids: _containers.ScalarMap[str, str]
    title: str
    search_title: str
    season_number: int
    scene_season_number: int
    scene_origin: str
    search_mode: SceneSearchMode
    filter_regex: str
    comment: str
    def __init__(self, source: _Optional[str] = ..., series_external_ids: _Optional[_Mapping[str, str]] = ..., title: _Optional[str] = ..., search_title: _Optional[str] = ..., season_number: _Optional[int] = ..., scene_season_number: _Optional[int] = ..., scene_origin: _Optional[str] = ..., search_mode: _Optional[_Union[SceneSearchMode, str]] = ..., filter_regex: _Optional[str] = ..., comment: _Optional[str] = ...) -> None: ...

class ListDailySeriesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListDailySeriesResponse(_message.Message):
    __slots__ = ("series",)
    SERIES_FIELD_NUMBER: _ClassVar[int]
    series: _containers.RepeatedCompositeFieldContainer[SeriesExternalIds]
    def __init__(self, series: _Optional[_Iterable[_Union[SeriesExternalIds, _Mapping]]] = ...) -> None: ...

class SeriesExternalIds(_message.Message):
    __slots__ = ("external_ids",)
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    external_ids: _containers.ScalarMap[str, str]
    def __init__(self, external_ids: _Optional[_Mapping[str, str]] = ...) -> None: ...

class GetSceneNumberingRequest(_message.Message):
    __slots__ = ("series_external_ids",)
    class SeriesExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    SERIES_EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    series_external_ids: _containers.ScalarMap[str, str]
    def __init__(self, series_external_ids: _Optional[_Mapping[str, str]] = ...) -> None: ...

class GetSceneNumberingResponse(_message.Message):
    __slots__ = ("found", "provider", "episodes")
    FOUND_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    EPISODES_FIELD_NUMBER: _ClassVar[int]
    found: bool
    provider: str
    episodes: _containers.RepeatedCompositeFieldContainer[SceneNumbering]
    def __init__(self, found: _Optional[bool] = ..., provider: _Optional[str] = ..., episodes: _Optional[_Iterable[_Union[SceneNumbering, _Mapping]]] = ...) -> None: ...

class SceneNumbering(_message.Message):
    __slots__ = ("season_number", "episode_number", "absolute_number", "scene_season_number", "scene_episode_number", "scene_absolute_number")
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    EPISODE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    ABSOLUTE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    SCENE_SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    SCENE_EPISODE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    SCENE_ABSOLUTE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    season_number: int
    episode_number: int
    absolute_number: int
    scene_season_number: int
    scene_episode_number: int
    scene_absolute_number: int
    def __init__(self, season_number: _Optional[int] = ..., episode_number: _Optional[int] = ..., absolute_number: _Optional[int] = ..., scene_season_number: _Optional[int] = ..., scene_episode_number: _Optional[int] = ..., scene_absolute_number: _Optional[int] = ...) -> None: ...

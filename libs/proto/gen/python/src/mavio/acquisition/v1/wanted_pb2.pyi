import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.library.v1 import item_pb2 as _item_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class MinimumAvailability(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MINIMUM_AVAILABILITY_UNSPECIFIED: _ClassVar[MinimumAvailability]
    MINIMUM_AVAILABILITY_ANNOUNCED: _ClassVar[MinimumAvailability]
    MINIMUM_AVAILABILITY_IN_CINEMAS: _ClassVar[MinimumAvailability]
    MINIMUM_AVAILABILITY_RELEASED: _ClassVar[MinimumAvailability]

class NewItemMonitor(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    NEW_ITEM_MONITOR_UNSPECIFIED: _ClassVar[NewItemMonitor]
    NEW_ITEM_MONITOR_ALL: _ClassVar[NewItemMonitor]
    NEW_ITEM_MONITOR_NONE: _ClassVar[NewItemMonitor]

class SeriesMonitor(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SERIES_MONITOR_UNSPECIFIED: _ClassVar[SeriesMonitor]
    SERIES_MONITOR_ALL: _ClassVar[SeriesMonitor]
    SERIES_MONITOR_FUTURE: _ClassVar[SeriesMonitor]
    SERIES_MONITOR_MISSING: _ClassVar[SeriesMonitor]
    SERIES_MONITOR_EXISTING: _ClassVar[SeriesMonitor]
    SERIES_MONITOR_FIRST_SEASON: _ClassVar[SeriesMonitor]
    SERIES_MONITOR_LAST_SEASON: _ClassVar[SeriesMonitor]
    SERIES_MONITOR_PILOT: _ClassVar[SeriesMonitor]
    SERIES_MONITOR_RECENT: _ClassVar[SeriesMonitor]
    SERIES_MONITOR_MONITOR_SPECIALS: _ClassVar[SeriesMonitor]
    SERIES_MONITOR_UNMONITOR_SPECIALS: _ClassVar[SeriesMonitor]
    SERIES_MONITOR_NONE: _ClassVar[SeriesMonitor]
    SERIES_MONITOR_SKIP: _ClassVar[SeriesMonitor]

class WantedFilter(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    WANTED_FILTER_UNSPECIFIED: _ClassVar[WantedFilter]
    WANTED_FILTER_MISSING: _ClassVar[WantedFilter]
    WANTED_FILTER_CUTOFF_UNMET: _ClassVar[WantedFilter]

class WantedKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    WANTED_KIND_UNSPECIFIED: _ClassVar[WantedKind]
    WANTED_KIND_MOVIE: _ClassVar[WantedKind]
    WANTED_KIND_EPISODE: _ClassVar[WantedKind]
MINIMUM_AVAILABILITY_UNSPECIFIED: MinimumAvailability
MINIMUM_AVAILABILITY_ANNOUNCED: MinimumAvailability
MINIMUM_AVAILABILITY_IN_CINEMAS: MinimumAvailability
MINIMUM_AVAILABILITY_RELEASED: MinimumAvailability
NEW_ITEM_MONITOR_UNSPECIFIED: NewItemMonitor
NEW_ITEM_MONITOR_ALL: NewItemMonitor
NEW_ITEM_MONITOR_NONE: NewItemMonitor
SERIES_MONITOR_UNSPECIFIED: SeriesMonitor
SERIES_MONITOR_ALL: SeriesMonitor
SERIES_MONITOR_FUTURE: SeriesMonitor
SERIES_MONITOR_MISSING: SeriesMonitor
SERIES_MONITOR_EXISTING: SeriesMonitor
SERIES_MONITOR_FIRST_SEASON: SeriesMonitor
SERIES_MONITOR_LAST_SEASON: SeriesMonitor
SERIES_MONITOR_PILOT: SeriesMonitor
SERIES_MONITOR_RECENT: SeriesMonitor
SERIES_MONITOR_MONITOR_SPECIALS: SeriesMonitor
SERIES_MONITOR_UNMONITOR_SPECIALS: SeriesMonitor
SERIES_MONITOR_NONE: SeriesMonitor
SERIES_MONITOR_SKIP: SeriesMonitor
WANTED_FILTER_UNSPECIFIED: WantedFilter
WANTED_FILTER_MISSING: WantedFilter
WANTED_FILTER_CUTOFF_UNMET: WantedFilter
WANTED_KIND_UNSPECIFIED: WantedKind
WANTED_KIND_MOVIE: WantedKind
WANTED_KIND_EPISODE: WantedKind

class Wanted(_message.Message):
    __slots__ = ("item_id", "monitored", "quality_profile_id", "minimum_availability", "tags", "add_time", "last_search_time", "monitor_new_items", "season_folder")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    MONITORED_FIELD_NUMBER: _ClassVar[int]
    QUALITY_PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    MINIMUM_AVAILABILITY_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    ADD_TIME_FIELD_NUMBER: _ClassVar[int]
    LAST_SEARCH_TIME_FIELD_NUMBER: _ClassVar[int]
    MONITOR_NEW_ITEMS_FIELD_NUMBER: _ClassVar[int]
    SEASON_FOLDER_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    monitored: bool
    quality_profile_id: str
    minimum_availability: MinimumAvailability
    tags: _containers.RepeatedScalarFieldContainer[str]
    add_time: _timestamp_pb2.Timestamp
    last_search_time: _timestamp_pb2.Timestamp
    monitor_new_items: NewItemMonitor
    season_folder: bool
    def __init__(self, item_id: _Optional[str] = ..., monitored: _Optional[bool] = ..., quality_profile_id: _Optional[str] = ..., minimum_availability: _Optional[_Union[MinimumAvailability, str]] = ..., tags: _Optional[_Iterable[str]] = ..., add_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., last_search_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., monitor_new_items: _Optional[_Union[NewItemMonitor, str]] = ..., season_folder: _Optional[bool] = ...) -> None: ...

class AddMovieRequest(_message.Message):
    __slots__ = ("library_id", "tmdb_id", "quality_profile_id", "minimum_availability", "tags", "search")
    LIBRARY_ID_FIELD_NUMBER: _ClassVar[int]
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    QUALITY_PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    MINIMUM_AVAILABILITY_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    SEARCH_FIELD_NUMBER: _ClassVar[int]
    library_id: str
    tmdb_id: int
    quality_profile_id: str
    minimum_availability: MinimumAvailability
    tags: _containers.RepeatedScalarFieldContainer[str]
    search: bool
    def __init__(self, library_id: _Optional[str] = ..., tmdb_id: _Optional[int] = ..., quality_profile_id: _Optional[str] = ..., minimum_availability: _Optional[_Union[MinimumAvailability, str]] = ..., tags: _Optional[_Iterable[str]] = ..., search: _Optional[bool] = ...) -> None: ...

class AddMovieResponse(_message.Message):
    __slots__ = ("item_id", "wanted")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    WANTED_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    wanted: Wanted
    def __init__(self, item_id: _Optional[str] = ..., wanted: _Optional[_Union[Wanted, _Mapping]] = ...) -> None: ...

class AddSeriesRequest(_message.Message):
    __slots__ = ("library_id", "tvdb_id", "quality_profile_id", "series_type", "season_folder", "monitor", "monitor_new_items", "tags", "search")
    LIBRARY_ID_FIELD_NUMBER: _ClassVar[int]
    TVDB_ID_FIELD_NUMBER: _ClassVar[int]
    QUALITY_PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    SERIES_TYPE_FIELD_NUMBER: _ClassVar[int]
    SEASON_FOLDER_FIELD_NUMBER: _ClassVar[int]
    MONITOR_FIELD_NUMBER: _ClassVar[int]
    MONITOR_NEW_ITEMS_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    SEARCH_FIELD_NUMBER: _ClassVar[int]
    library_id: str
    tvdb_id: int
    quality_profile_id: str
    series_type: _item_pb2.SeriesType
    season_folder: bool
    monitor: SeriesMonitor
    monitor_new_items: NewItemMonitor
    tags: _containers.RepeatedScalarFieldContainer[str]
    search: bool
    def __init__(self, library_id: _Optional[str] = ..., tvdb_id: _Optional[int] = ..., quality_profile_id: _Optional[str] = ..., series_type: _Optional[_Union[_item_pb2.SeriesType, str]] = ..., season_folder: _Optional[bool] = ..., monitor: _Optional[_Union[SeriesMonitor, str]] = ..., monitor_new_items: _Optional[_Union[NewItemMonitor, str]] = ..., tags: _Optional[_Iterable[str]] = ..., search: _Optional[bool] = ...) -> None: ...

class AddSeriesResponse(_message.Message):
    __slots__ = ("item_id", "wanted")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    WANTED_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    wanted: Wanted
    def __init__(self, item_id: _Optional[str] = ..., wanted: _Optional[_Union[Wanted, _Mapping]] = ...) -> None: ...

class SetMonitoredRequest(_message.Message):
    __slots__ = ("items", "series")
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    SERIES_FIELD_NUMBER: _ClassVar[int]
    items: MonitoredItems
    series: SeriesMonitoring
    def __init__(self, items: _Optional[_Union[MonitoredItems, _Mapping]] = ..., series: _Optional[_Union[SeriesMonitoring, _Mapping]] = ...) -> None: ...

class MonitoredItems(_message.Message):
    __slots__ = ("item_ids", "monitored")
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    MONITORED_FIELD_NUMBER: _ClassVar[int]
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    monitored: bool
    def __init__(self, item_ids: _Optional[_Iterable[str]] = ..., monitored: _Optional[bool] = ...) -> None: ...

class SeriesMonitoring(_message.Message):
    __slots__ = ("item_ids", "monitor", "monitor_new_items")
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    MONITOR_FIELD_NUMBER: _ClassVar[int]
    MONITOR_NEW_ITEMS_FIELD_NUMBER: _ClassVar[int]
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    monitor: SeriesMonitor
    monitor_new_items: NewItemMonitor
    def __init__(self, item_ids: _Optional[_Iterable[str]] = ..., monitor: _Optional[_Union[SeriesMonitor, str]] = ..., monitor_new_items: _Optional[_Union[NewItemMonitor, str]] = ...) -> None: ...

class SetMonitoredResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetWantedRequest(_message.Message):
    __slots__ = ("item_id",)
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    def __init__(self, item_id: _Optional[str] = ...) -> None: ...

class GetWantedResponse(_message.Message):
    __slots__ = ("wanted",)
    WANTED_FIELD_NUMBER: _ClassVar[int]
    wanted: Wanted
    def __init__(self, wanted: _Optional[_Union[Wanted, _Mapping]] = ...) -> None: ...

class UpdateWantedRequest(_message.Message):
    __slots__ = ("wanted",)
    WANTED_FIELD_NUMBER: _ClassVar[int]
    wanted: Wanted
    def __init__(self, wanted: _Optional[_Union[Wanted, _Mapping]] = ...) -> None: ...

class UpdateWantedResponse(_message.Message):
    __slots__ = ("wanted",)
    WANTED_FIELD_NUMBER: _ClassVar[int]
    wanted: Wanted
    def __init__(self, wanted: _Optional[_Union[Wanted, _Mapping]] = ...) -> None: ...

class ListWantedRequest(_message.Message):
    __slots__ = ("filter", "limit", "offset", "kind")
    FILTER_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    filter: WantedFilter
    limit: int
    offset: int
    kind: WantedKind
    def __init__(self, filter: _Optional[_Union[WantedFilter, str]] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ..., kind: _Optional[_Union[WantedKind, str]] = ...) -> None: ...

class WantedEpisode(_message.Message):
    __slots__ = ("item_id", "series_id", "series_name", "season_number", "episode_number", "absolute_number", "name", "air_time", "quality", "wanted")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    SERIES_ID_FIELD_NUMBER: _ClassVar[int]
    SERIES_NAME_FIELD_NUMBER: _ClassVar[int]
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    EPISODE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    ABSOLUTE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    AIR_TIME_FIELD_NUMBER: _ClassVar[int]
    QUALITY_FIELD_NUMBER: _ClassVar[int]
    WANTED_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    series_id: str
    series_name: str
    season_number: int
    episode_number: int
    absolute_number: int
    name: str
    air_time: _timestamp_pb2.Timestamp
    quality: str
    wanted: Wanted
    def __init__(self, item_id: _Optional[str] = ..., series_id: _Optional[str] = ..., series_name: _Optional[str] = ..., season_number: _Optional[int] = ..., episode_number: _Optional[int] = ..., absolute_number: _Optional[int] = ..., name: _Optional[str] = ..., air_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., quality: _Optional[str] = ..., wanted: _Optional[_Union[Wanted, _Mapping]] = ...) -> None: ...

class WantedMovie(_message.Message):
    __slots__ = ("item_id", "name", "year", "wanted", "quality", "available")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    WANTED_FIELD_NUMBER: _ClassVar[int]
    QUALITY_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    name: str
    year: int
    wanted: Wanted
    quality: str
    available: bool
    def __init__(self, item_id: _Optional[str] = ..., name: _Optional[str] = ..., year: _Optional[int] = ..., wanted: _Optional[_Union[Wanted, _Mapping]] = ..., quality: _Optional[str] = ..., available: _Optional[bool] = ...) -> None: ...

class ListWantedResponse(_message.Message):
    __slots__ = ("movies", "total", "episodes")
    MOVIES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    EPISODES_FIELD_NUMBER: _ClassVar[int]
    movies: _containers.RepeatedCompositeFieldContainer[WantedMovie]
    total: int
    episodes: _containers.RepeatedCompositeFieldContainer[WantedEpisode]
    def __init__(self, movies: _Optional[_Iterable[_Union[WantedMovie, _Mapping]]] = ..., total: _Optional[int] = ..., episodes: _Optional[_Iterable[_Union[WantedEpisode, _Mapping]]] = ...) -> None: ...

class SearchWantedRequest(_message.Message):
    __slots__ = ("item_ids", "filter", "kind")
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    FILTER_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    filter: WantedFilter
    kind: WantedKind
    def __init__(self, item_ids: _Optional[_Iterable[str]] = ..., filter: _Optional[_Union[WantedFilter, str]] = ..., kind: _Optional[_Union[WantedKind, str]] = ...) -> None: ...

class SearchWantedResponse(_message.Message):
    __slots__ = ("queued",)
    QUEUED_FIELD_NUMBER: _ClassVar[int]
    queued: int
    def __init__(self, queued: _Optional[int] = ...) -> None: ...

class SyncRSSRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SyncRSSResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

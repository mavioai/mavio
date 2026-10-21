import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.acquisition.v1 import profile_pb2 as _profile_pb2
from mavio.acquisition.v1 import wanted_pb2 as _wanted_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RequestStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    REQUEST_STATUS_UNSPECIFIED: _ClassVar[RequestStatus]
    REQUEST_STATUS_PENDING: _ClassVar[RequestStatus]
    REQUEST_STATUS_APPROVED: _ClassVar[RequestStatus]
    REQUEST_STATUS_DECLINED: _ClassVar[RequestStatus]
    REQUEST_STATUS_FAILED: _ClassVar[RequestStatus]
    REQUEST_STATUS_COMPLETED: _ClassVar[RequestStatus]

class RequestSort(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    REQUEST_SORT_UNSPECIFIED: _ClassVar[RequestSort]
    REQUEST_SORT_MODIFIED: _ClassVar[RequestSort]

class BlockedTitleSource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    BLOCKED_TITLE_SOURCE_UNSPECIFIED: _ClassVar[BlockedTitleSource]
    BLOCKED_TITLE_SOURCE_MANUAL: _ClassVar[BlockedTitleSource]
    BLOCKED_TITLE_SOURCE_TAGS: _ClassVar[BlockedTitleSource]
REQUEST_STATUS_UNSPECIFIED: RequestStatus
REQUEST_STATUS_PENDING: RequestStatus
REQUEST_STATUS_APPROVED: RequestStatus
REQUEST_STATUS_DECLINED: RequestStatus
REQUEST_STATUS_FAILED: RequestStatus
REQUEST_STATUS_COMPLETED: RequestStatus
REQUEST_SORT_UNSPECIFIED: RequestSort
REQUEST_SORT_MODIFIED: RequestSort
BLOCKED_TITLE_SOURCE_UNSPECIFIED: BlockedTitleSource
BLOCKED_TITLE_SOURCE_MANUAL: BlockedTitleSource
BLOCKED_TITLE_SOURCE_TAGS: BlockedTitleSource

class SeasonRequest(_message.Message):
    __slots__ = ("season_number", "status")
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    season_number: int
    status: RequestStatus
    def __init__(self, season_number: _Optional[int] = ..., status: _Optional[_Union[RequestStatus, str]] = ...) -> None: ...

class Request(_message.Message):
    __slots__ = ("id", "media_kind", "tmdb_id", "tvdb_id", "title", "year", "item_id", "seasons", "status", "requested_by", "modified_by", "ignore_quota", "message", "create_time", "update_time")
    ID_FIELD_NUMBER: _ClassVar[int]
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    TVDB_ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    SEASONS_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    REQUESTED_BY_FIELD_NUMBER: _ClassVar[int]
    MODIFIED_BY_FIELD_NUMBER: _ClassVar[int]
    IGNORE_QUOTA_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    id: str
    media_kind: _profile_pb2.MediaKind
    tmdb_id: int
    tvdb_id: int
    title: str
    year: int
    item_id: str
    seasons: _containers.RepeatedCompositeFieldContainer[SeasonRequest]
    status: RequestStatus
    requested_by: str
    modified_by: str
    ignore_quota: bool
    message: str
    create_time: _timestamp_pb2.Timestamp
    update_time: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., media_kind: _Optional[_Union[_profile_pb2.MediaKind, str]] = ..., tmdb_id: _Optional[int] = ..., tvdb_id: _Optional[int] = ..., title: _Optional[str] = ..., year: _Optional[int] = ..., item_id: _Optional[str] = ..., seasons: _Optional[_Iterable[_Union[SeasonRequest, _Mapping]]] = ..., status: _Optional[_Union[RequestStatus, str]] = ..., requested_by: _Optional[str] = ..., modified_by: _Optional[str] = ..., ignore_quota: _Optional[bool] = ..., message: _Optional[str] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class CreateRequestRequest(_message.Message):
    __slots__ = ("media_kind", "tmdb_id", "seasons", "all_seasons", "user_id", "ignore_quota")
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    SEASONS_FIELD_NUMBER: _ClassVar[int]
    ALL_SEASONS_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    IGNORE_QUOTA_FIELD_NUMBER: _ClassVar[int]
    media_kind: _profile_pb2.MediaKind
    tmdb_id: int
    seasons: _containers.RepeatedScalarFieldContainer[int]
    all_seasons: bool
    user_id: str
    ignore_quota: bool
    def __init__(self, media_kind: _Optional[_Union[_profile_pb2.MediaKind, str]] = ..., tmdb_id: _Optional[int] = ..., seasons: _Optional[_Iterable[int]] = ..., all_seasons: _Optional[bool] = ..., user_id: _Optional[str] = ..., ignore_quota: _Optional[bool] = ...) -> None: ...

class CreateRequestResponse(_message.Message):
    __slots__ = ("request",)
    REQUEST_FIELD_NUMBER: _ClassVar[int]
    request: Request
    def __init__(self, request: _Optional[_Union[Request, _Mapping]] = ...) -> None: ...

class ListRequestsRequest(_message.Message):
    __slots__ = ("statuses", "media_kind", "user_id", "tmdb_id", "sort", "ascending", "limit", "offset")
    STATUSES_FIELD_NUMBER: _ClassVar[int]
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    SORT_FIELD_NUMBER: _ClassVar[int]
    ASCENDING_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    statuses: _containers.RepeatedScalarFieldContainer[RequestStatus]
    media_kind: _profile_pb2.MediaKind
    user_id: str
    tmdb_id: int
    sort: RequestSort
    ascending: bool
    limit: int
    offset: int
    def __init__(self, statuses: _Optional[_Iterable[_Union[RequestStatus, str]]] = ..., media_kind: _Optional[_Union[_profile_pb2.MediaKind, str]] = ..., user_id: _Optional[str] = ..., tmdb_id: _Optional[int] = ..., sort: _Optional[_Union[RequestSort, str]] = ..., ascending: _Optional[bool] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class ListRequestsResponse(_message.Message):
    __slots__ = ("requests", "total")
    REQUESTS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    requests: _containers.RepeatedCompositeFieldContainer[Request]
    total: int
    def __init__(self, requests: _Optional[_Iterable[_Union[Request, _Mapping]]] = ..., total: _Optional[int] = ...) -> None: ...

class GetRequestRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class GetRequestResponse(_message.Message):
    __slots__ = ("request",)
    REQUEST_FIELD_NUMBER: _ClassVar[int]
    request: Request
    def __init__(self, request: _Optional[_Union[Request, _Mapping]] = ...) -> None: ...

class UpdateRequestRequest(_message.Message):
    __slots__ = ("id", "seasons")
    ID_FIELD_NUMBER: _ClassVar[int]
    SEASONS_FIELD_NUMBER: _ClassVar[int]
    id: str
    seasons: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, id: _Optional[str] = ..., seasons: _Optional[_Iterable[int]] = ...) -> None: ...

class UpdateRequestResponse(_message.Message):
    __slots__ = ("request",)
    REQUEST_FIELD_NUMBER: _ClassVar[int]
    request: Request
    def __init__(self, request: _Optional[_Union[Request, _Mapping]] = ...) -> None: ...

class DeleteRequestRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteRequestResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ApproveRequestRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class ApproveRequestResponse(_message.Message):
    __slots__ = ("request",)
    REQUEST_FIELD_NUMBER: _ClassVar[int]
    request: Request
    def __init__(self, request: _Optional[_Union[Request, _Mapping]] = ...) -> None: ...

class DeclineRequestRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeclineRequestResponse(_message.Message):
    __slots__ = ("request",)
    REQUEST_FIELD_NUMBER: _ClassVar[int]
    request: Request
    def __init__(self, request: _Optional[_Union[Request, _Mapping]] = ...) -> None: ...

class RetryRequestRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class RetryRequestResponse(_message.Message):
    __slots__ = ("request",)
    REQUEST_FIELD_NUMBER: _ClassVar[int]
    request: Request
    def __init__(self, request: _Optional[_Union[Request, _Mapping]] = ...) -> None: ...

class GetQuotaRequest(_message.Message):
    __slots__ = ("user_id",)
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    def __init__(self, user_id: _Optional[str] = ...) -> None: ...

class Quota(_message.Message):
    __slots__ = ("limit", "days", "used", "remaining", "restricted")
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    DAYS_FIELD_NUMBER: _ClassVar[int]
    USED_FIELD_NUMBER: _ClassVar[int]
    REMAINING_FIELD_NUMBER: _ClassVar[int]
    RESTRICTED_FIELD_NUMBER: _ClassVar[int]
    limit: int
    days: int
    used: int
    remaining: int
    restricted: bool
    def __init__(self, limit: _Optional[int] = ..., days: _Optional[int] = ..., used: _Optional[int] = ..., remaining: _Optional[int] = ..., restricted: _Optional[bool] = ...) -> None: ...

class GetQuotaResponse(_message.Message):
    __slots__ = ("movie", "series")
    MOVIE_FIELD_NUMBER: _ClassVar[int]
    SERIES_FIELD_NUMBER: _ClassVar[int]
    movie: Quota
    series: Quota
    def __init__(self, movie: _Optional[_Union[Quota, _Mapping]] = ..., series: _Optional[_Union[Quota, _Mapping]] = ...) -> None: ...

class BlockedTitle(_message.Message):
    __slots__ = ("media_kind", "tmdb_id", "title", "user_id", "create_time", "tags")
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    media_kind: _profile_pb2.MediaKind
    tmdb_id: int
    title: str
    user_id: str
    create_time: _timestamp_pb2.Timestamp
    tags: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, media_kind: _Optional[_Union[_profile_pb2.MediaKind, str]] = ..., tmdb_id: _Optional[int] = ..., title: _Optional[str] = ..., user_id: _Optional[str] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., tags: _Optional[_Iterable[str]] = ...) -> None: ...

class BlockTitleRequest(_message.Message):
    __slots__ = ("media_kind", "tmdb_id")
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    media_kind: _profile_pb2.MediaKind
    tmdb_id: int
    def __init__(self, media_kind: _Optional[_Union[_profile_pb2.MediaKind, str]] = ..., tmdb_id: _Optional[int] = ...) -> None: ...

class BlockTitleResponse(_message.Message):
    __slots__ = ("blocked_title",)
    BLOCKED_TITLE_FIELD_NUMBER: _ClassVar[int]
    blocked_title: BlockedTitle
    def __init__(self, blocked_title: _Optional[_Union[BlockedTitle, _Mapping]] = ...) -> None: ...

class UnblockTitleRequest(_message.Message):
    __slots__ = ("media_kind", "tmdb_id")
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    media_kind: _profile_pb2.MediaKind
    tmdb_id: int
    def __init__(self, media_kind: _Optional[_Union[_profile_pb2.MediaKind, str]] = ..., tmdb_id: _Optional[int] = ...) -> None: ...

class UnblockTitleResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListBlockedTitlesRequest(_message.Message):
    __slots__ = ("search", "limit", "offset", "source")
    SEARCH_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    search: str
    limit: int
    offset: int
    source: BlockedTitleSource
    def __init__(self, search: _Optional[str] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ..., source: _Optional[_Union[BlockedTitleSource, str]] = ...) -> None: ...

class ListBlockedTitlesResponse(_message.Message):
    __slots__ = ("blocked_titles", "total")
    BLOCKED_TITLES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    blocked_titles: _containers.RepeatedCompositeFieldContainer[BlockedTitle]
    total: int
    def __init__(self, blocked_titles: _Optional[_Iterable[_Union[BlockedTitle, _Mapping]]] = ..., total: _Optional[int] = ...) -> None: ...

class BlockCollectionRequest(_message.Message):
    __slots__ = ("tmdb_collection_id",)
    TMDB_COLLECTION_ID_FIELD_NUMBER: _ClassVar[int]
    tmdb_collection_id: int
    def __init__(self, tmdb_collection_id: _Optional[int] = ...) -> None: ...

class BlockCollectionResponse(_message.Message):
    __slots__ = ("blocked_titles",)
    BLOCKED_TITLES_FIELD_NUMBER: _ClassVar[int]
    blocked_titles: _containers.RepeatedCompositeFieldContainer[BlockedTitle]
    def __init__(self, blocked_titles: _Optional[_Iterable[_Union[BlockedTitle, _Mapping]]] = ...) -> None: ...

class UnblockCollectionRequest(_message.Message):
    __slots__ = ("tmdb_collection_id",)
    TMDB_COLLECTION_ID_FIELD_NUMBER: _ClassVar[int]
    tmdb_collection_id: int
    def __init__(self, tmdb_collection_id: _Optional[int] = ...) -> None: ...

class UnblockCollectionResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RequestSettings(_message.Message):
    __slots__ = ("default_request_movies", "default_request_series", "default_auto_approve_movies", "default_auto_approve_series", "default_report_issues", "movie_quota_limit", "movie_quota_days", "series_quota_limit", "series_quota_days", "movie_library_id", "movie_quality_profile_id", "minimum_availability", "series_library_id", "series_quality_profile_id", "season_folder", "monitor_new_items", "specials", "anime_library_id", "anime_quality_profile_id", "blocklisted_tags", "blocklisted_tags_limit")
    DEFAULT_REQUEST_MOVIES_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_REQUEST_SERIES_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_AUTO_APPROVE_MOVIES_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_AUTO_APPROVE_SERIES_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_REPORT_ISSUES_FIELD_NUMBER: _ClassVar[int]
    MOVIE_QUOTA_LIMIT_FIELD_NUMBER: _ClassVar[int]
    MOVIE_QUOTA_DAYS_FIELD_NUMBER: _ClassVar[int]
    SERIES_QUOTA_LIMIT_FIELD_NUMBER: _ClassVar[int]
    SERIES_QUOTA_DAYS_FIELD_NUMBER: _ClassVar[int]
    MOVIE_LIBRARY_ID_FIELD_NUMBER: _ClassVar[int]
    MOVIE_QUALITY_PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    MINIMUM_AVAILABILITY_FIELD_NUMBER: _ClassVar[int]
    SERIES_LIBRARY_ID_FIELD_NUMBER: _ClassVar[int]
    SERIES_QUALITY_PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    SEASON_FOLDER_FIELD_NUMBER: _ClassVar[int]
    MONITOR_NEW_ITEMS_FIELD_NUMBER: _ClassVar[int]
    SPECIALS_FIELD_NUMBER: _ClassVar[int]
    ANIME_LIBRARY_ID_FIELD_NUMBER: _ClassVar[int]
    ANIME_QUALITY_PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    BLOCKLISTED_TAGS_FIELD_NUMBER: _ClassVar[int]
    BLOCKLISTED_TAGS_LIMIT_FIELD_NUMBER: _ClassVar[int]
    default_request_movies: bool
    default_request_series: bool
    default_auto_approve_movies: bool
    default_auto_approve_series: bool
    default_report_issues: bool
    movie_quota_limit: int
    movie_quota_days: int
    series_quota_limit: int
    series_quota_days: int
    movie_library_id: str
    movie_quality_profile_id: str
    minimum_availability: _wanted_pb2.MinimumAvailability
    series_library_id: str
    series_quality_profile_id: str
    season_folder: bool
    monitor_new_items: _wanted_pb2.NewItemMonitor
    specials: bool
    anime_library_id: str
    anime_quality_profile_id: str
    blocklisted_tags: _containers.RepeatedScalarFieldContainer[str]
    blocklisted_tags_limit: int
    def __init__(self, default_request_movies: _Optional[bool] = ..., default_request_series: _Optional[bool] = ..., default_auto_approve_movies: _Optional[bool] = ..., default_auto_approve_series: _Optional[bool] = ..., default_report_issues: _Optional[bool] = ..., movie_quota_limit: _Optional[int] = ..., movie_quota_days: _Optional[int] = ..., series_quota_limit: _Optional[int] = ..., series_quota_days: _Optional[int] = ..., movie_library_id: _Optional[str] = ..., movie_quality_profile_id: _Optional[str] = ..., minimum_availability: _Optional[_Union[_wanted_pb2.MinimumAvailability, str]] = ..., series_library_id: _Optional[str] = ..., series_quality_profile_id: _Optional[str] = ..., season_folder: _Optional[bool] = ..., monitor_new_items: _Optional[_Union[_wanted_pb2.NewItemMonitor, str]] = ..., specials: _Optional[bool] = ..., anime_library_id: _Optional[str] = ..., anime_quality_profile_id: _Optional[str] = ..., blocklisted_tags: _Optional[_Iterable[str]] = ..., blocklisted_tags_limit: _Optional[int] = ...) -> None: ...

class GetRequestSettingsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetRequestSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: RequestSettings
    def __init__(self, settings: _Optional[_Union[RequestSettings, _Mapping]] = ...) -> None: ...

class UpdateRequestSettingsRequest(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: RequestSettings
    def __init__(self, settings: _Optional[_Union[RequestSettings, _Mapping]] = ...) -> None: ...

class UpdateRequestSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: RequestSettings
    def __init__(self, settings: _Optional[_Union[RequestSettings, _Mapping]] = ...) -> None: ...

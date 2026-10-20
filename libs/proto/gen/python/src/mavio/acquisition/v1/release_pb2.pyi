import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DownloadProtocol(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    DOWNLOAD_PROTOCOL_UNSPECIFIED: _ClassVar[DownloadProtocol]
    DOWNLOAD_PROTOCOL_TORRENT: _ClassVar[DownloadProtocol]
    DOWNLOAD_PROTOCOL_USENET: _ClassVar[DownloadProtocol]

class IndexerPrivacy(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    INDEXER_PRIVACY_UNSPECIFIED: _ClassVar[IndexerPrivacy]
    INDEXER_PRIVACY_PUBLIC: _ClassVar[IndexerPrivacy]
    INDEXER_PRIVACY_SEMI_PRIVATE: _ClassVar[IndexerPrivacy]
    INDEXER_PRIVACY_PRIVATE: _ClassVar[IndexerPrivacy]

class ReleaseSearchType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    RELEASE_SEARCH_TYPE_UNSPECIFIED: _ClassVar[ReleaseSearchType]
    RELEASE_SEARCH_TYPE_MOVIE: _ClassVar[ReleaseSearchType]
    RELEASE_SEARCH_TYPE_TV: _ClassVar[ReleaseSearchType]

class ReleaseKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    RELEASE_KIND_UNSPECIFIED: _ClassVar[ReleaseKind]
    RELEASE_KIND_MOVIE: _ClassVar[ReleaseKind]
    RELEASE_KIND_EPISODE: _ClassVar[ReleaseKind]
DOWNLOAD_PROTOCOL_UNSPECIFIED: DownloadProtocol
DOWNLOAD_PROTOCOL_TORRENT: DownloadProtocol
DOWNLOAD_PROTOCOL_USENET: DownloadProtocol
INDEXER_PRIVACY_UNSPECIFIED: IndexerPrivacy
INDEXER_PRIVACY_PUBLIC: IndexerPrivacy
INDEXER_PRIVACY_SEMI_PRIVATE: IndexerPrivacy
INDEXER_PRIVACY_PRIVATE: IndexerPrivacy
RELEASE_SEARCH_TYPE_UNSPECIFIED: ReleaseSearchType
RELEASE_SEARCH_TYPE_MOVIE: ReleaseSearchType
RELEASE_SEARCH_TYPE_TV: ReleaseSearchType
RELEASE_KIND_UNSPECIFIED: ReleaseKind
RELEASE_KIND_MOVIE: ReleaseKind
RELEASE_KIND_EPISODE: ReleaseKind

class IndexerRef(_message.Message):
    __slots__ = ("plugin_id", "indexer_id")
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    INDEXER_ID_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    indexer_id: str
    def __init__(self, plugin_id: _Optional[str] = ..., indexer_id: _Optional[str] = ...) -> None: ...

class Indexer(_message.Message):
    __slots__ = ("plugin_id", "id", "name", "definition_id", "protocol", "privacy", "enabled", "priority", "search_params", "tv_search_params", "movie_search_params", "disabled_until", "failure")
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DEFINITION_ID_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    PRIVACY_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    PRIORITY_FIELD_NUMBER: _ClassVar[int]
    SEARCH_PARAMS_FIELD_NUMBER: _ClassVar[int]
    TV_SEARCH_PARAMS_FIELD_NUMBER: _ClassVar[int]
    MOVIE_SEARCH_PARAMS_FIELD_NUMBER: _ClassVar[int]
    DISABLED_UNTIL_FIELD_NUMBER: _ClassVar[int]
    FAILURE_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    id: str
    name: str
    definition_id: str
    protocol: DownloadProtocol
    privacy: IndexerPrivacy
    enabled: bool
    priority: int
    search_params: _containers.RepeatedScalarFieldContainer[str]
    tv_search_params: _containers.RepeatedScalarFieldContainer[str]
    movie_search_params: _containers.RepeatedScalarFieldContainer[str]
    disabled_until: _timestamp_pb2.Timestamp
    failure: str
    def __init__(self, plugin_id: _Optional[str] = ..., id: _Optional[str] = ..., name: _Optional[str] = ..., definition_id: _Optional[str] = ..., protocol: _Optional[_Union[DownloadProtocol, str]] = ..., privacy: _Optional[_Union[IndexerPrivacy, str]] = ..., enabled: _Optional[bool] = ..., priority: _Optional[int] = ..., search_params: _Optional[_Iterable[str]] = ..., tv_search_params: _Optional[_Iterable[str]] = ..., movie_search_params: _Optional[_Iterable[str]] = ..., disabled_until: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., failure: _Optional[str] = ...) -> None: ...

class ListIndexersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListIndexersResponse(_message.Message):
    __slots__ = ("indexers", "failures")
    INDEXERS_FIELD_NUMBER: _ClassVar[int]
    FAILURES_FIELD_NUMBER: _ClassVar[int]
    indexers: _containers.RepeatedCompositeFieldContainer[Indexer]
    failures: _containers.RepeatedCompositeFieldContainer[IndexerFailure]
    def __init__(self, indexers: _Optional[_Iterable[_Union[Indexer, _Mapping]]] = ..., failures: _Optional[_Iterable[_Union[IndexerFailure, _Mapping]]] = ...) -> None: ...

class IndexerFailure(_message.Message):
    __slots__ = ("plugin_id", "indexer_id", "message", "disabled_until")
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    INDEXER_ID_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    DISABLED_UNTIL_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    indexer_id: str
    message: str
    disabled_until: _timestamp_pb2.Timestamp
    def __init__(self, plugin_id: _Optional[str] = ..., indexer_id: _Optional[str] = ..., message: _Optional[str] = ..., disabled_until: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class SearchReleasesRequest(_message.Message):
    __slots__ = ("item_id", "query", "type", "season", "episode", "categories", "indexers", "min_age", "max_age", "min_size", "max_size")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    SEASON_FIELD_NUMBER: _ClassVar[int]
    EPISODE_FIELD_NUMBER: _ClassVar[int]
    CATEGORIES_FIELD_NUMBER: _ClassVar[int]
    INDEXERS_FIELD_NUMBER: _ClassVar[int]
    MIN_AGE_FIELD_NUMBER: _ClassVar[int]
    MAX_AGE_FIELD_NUMBER: _ClassVar[int]
    MIN_SIZE_FIELD_NUMBER: _ClassVar[int]
    MAX_SIZE_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    query: str
    type: ReleaseSearchType
    season: int
    episode: str
    categories: _containers.RepeatedScalarFieldContainer[int]
    indexers: _containers.RepeatedCompositeFieldContainer[IndexerRef]
    min_age: int
    max_age: int
    min_size: int
    max_size: int
    def __init__(self, item_id: _Optional[str] = ..., query: _Optional[str] = ..., type: _Optional[_Union[ReleaseSearchType, str]] = ..., season: _Optional[int] = ..., episode: _Optional[str] = ..., categories: _Optional[_Iterable[int]] = ..., indexers: _Optional[_Iterable[_Union[IndexerRef, _Mapping]]] = ..., min_age: _Optional[int] = ..., max_age: _Optional[int] = ..., min_size: _Optional[int] = ..., max_size: _Optional[int] = ...) -> None: ...

class SearchReleasesResponse(_message.Message):
    __slots__ = ("releases", "failures")
    RELEASES_FIELD_NUMBER: _ClassVar[int]
    FAILURES_FIELD_NUMBER: _ClassVar[int]
    releases: _containers.RepeatedCompositeFieldContainer[Release]
    failures: _containers.RepeatedCompositeFieldContainer[IndexerFailure]
    def __init__(self, releases: _Optional[_Iterable[_Union[Release, _Mapping]]] = ..., failures: _Optional[_Iterable[_Union[IndexerFailure, _Mapping]]] = ...) -> None: ...

class Release(_message.Message):
    __slots__ = ("plugin_id", "indexer_id", "indexer_name", "guid", "title", "size", "download_url", "magnet_url", "info_hash", "info_url", "protocol", "publish_date", "categories", "external_ids", "grabs", "seeders", "peers", "download_volume_factor", "upload_volume_factor", "flags", "parsed", "rejections", "evaluated", "custom_formats", "custom_format_score", "episode_ids")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    INDEXER_ID_FIELD_NUMBER: _ClassVar[int]
    INDEXER_NAME_FIELD_NUMBER: _ClassVar[int]
    GUID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_URL_FIELD_NUMBER: _ClassVar[int]
    MAGNET_URL_FIELD_NUMBER: _ClassVar[int]
    INFO_HASH_FIELD_NUMBER: _ClassVar[int]
    INFO_URL_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    PUBLISH_DATE_FIELD_NUMBER: _ClassVar[int]
    CATEGORIES_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    GRABS_FIELD_NUMBER: _ClassVar[int]
    SEEDERS_FIELD_NUMBER: _ClassVar[int]
    PEERS_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_VOLUME_FACTOR_FIELD_NUMBER: _ClassVar[int]
    UPLOAD_VOLUME_FACTOR_FIELD_NUMBER: _ClassVar[int]
    FLAGS_FIELD_NUMBER: _ClassVar[int]
    PARSED_FIELD_NUMBER: _ClassVar[int]
    REJECTIONS_FIELD_NUMBER: _ClassVar[int]
    EVALUATED_FIELD_NUMBER: _ClassVar[int]
    CUSTOM_FORMATS_FIELD_NUMBER: _ClassVar[int]
    CUSTOM_FORMAT_SCORE_FIELD_NUMBER: _ClassVar[int]
    EPISODE_IDS_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    indexer_id: str
    indexer_name: str
    guid: str
    title: str
    size: int
    download_url: str
    magnet_url: str
    info_hash: str
    info_url: str
    protocol: DownloadProtocol
    publish_date: _timestamp_pb2.Timestamp
    categories: _containers.RepeatedScalarFieldContainer[int]
    external_ids: _containers.ScalarMap[str, str]
    grabs: int
    seeders: int
    peers: int
    download_volume_factor: float
    upload_volume_factor: float
    flags: _containers.RepeatedScalarFieldContainer[str]
    parsed: ParsedRelease
    rejections: _containers.RepeatedCompositeFieldContainer[Rejection]
    evaluated: bool
    custom_formats: _containers.RepeatedScalarFieldContainer[str]
    custom_format_score: int
    episode_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, plugin_id: _Optional[str] = ..., indexer_id: _Optional[str] = ..., indexer_name: _Optional[str] = ..., guid: _Optional[str] = ..., title: _Optional[str] = ..., size: _Optional[int] = ..., download_url: _Optional[str] = ..., magnet_url: _Optional[str] = ..., info_hash: _Optional[str] = ..., info_url: _Optional[str] = ..., protocol: _Optional[_Union[DownloadProtocol, str]] = ..., publish_date: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., categories: _Optional[_Iterable[int]] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., grabs: _Optional[int] = ..., seeders: _Optional[int] = ..., peers: _Optional[int] = ..., download_volume_factor: _Optional[float] = ..., upload_volume_factor: _Optional[float] = ..., flags: _Optional[_Iterable[str]] = ..., parsed: _Optional[_Union[ParsedRelease, _Mapping]] = ..., rejections: _Optional[_Iterable[_Union[Rejection, _Mapping]]] = ..., evaluated: _Optional[bool] = ..., custom_formats: _Optional[_Iterable[str]] = ..., custom_format_score: _Optional[int] = ..., episode_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class Rejection(_message.Message):
    __slots__ = ("reason", "message", "temporary")
    REASON_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    TEMPORARY_FIELD_NUMBER: _ClassVar[int]
    reason: str
    message: str
    temporary: bool
    def __init__(self, reason: _Optional[str] = ..., message: _Optional[str] = ..., temporary: _Optional[bool] = ...) -> None: ...

class ParsedRelease(_message.Message):
    __slots__ = ("kind", "titles", "year", "edition", "imdb_id", "tmdb_id", "season", "episodes", "absolute_episodes", "air_date", "full_season", "quality", "languages", "release_group")
    KIND_FIELD_NUMBER: _ClassVar[int]
    TITLES_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    EDITION_FIELD_NUMBER: _ClassVar[int]
    IMDB_ID_FIELD_NUMBER: _ClassVar[int]
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    SEASON_FIELD_NUMBER: _ClassVar[int]
    EPISODES_FIELD_NUMBER: _ClassVar[int]
    ABSOLUTE_EPISODES_FIELD_NUMBER: _ClassVar[int]
    AIR_DATE_FIELD_NUMBER: _ClassVar[int]
    FULL_SEASON_FIELD_NUMBER: _ClassVar[int]
    QUALITY_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    RELEASE_GROUP_FIELD_NUMBER: _ClassVar[int]
    kind: ReleaseKind
    titles: _containers.RepeatedScalarFieldContainer[str]
    year: int
    edition: str
    imdb_id: str
    tmdb_id: int
    season: int
    episodes: _containers.RepeatedScalarFieldContainer[int]
    absolute_episodes: _containers.RepeatedScalarFieldContainer[int]
    air_date: str
    full_season: bool
    quality: ReleaseQuality
    languages: _containers.RepeatedScalarFieldContainer[str]
    release_group: str
    def __init__(self, kind: _Optional[_Union[ReleaseKind, str]] = ..., titles: _Optional[_Iterable[str]] = ..., year: _Optional[int] = ..., edition: _Optional[str] = ..., imdb_id: _Optional[str] = ..., tmdb_id: _Optional[int] = ..., season: _Optional[int] = ..., episodes: _Optional[_Iterable[int]] = ..., absolute_episodes: _Optional[_Iterable[int]] = ..., air_date: _Optional[str] = ..., full_season: _Optional[bool] = ..., quality: _Optional[_Union[ReleaseQuality, _Mapping]] = ..., languages: _Optional[_Iterable[str]] = ..., release_group: _Optional[str] = ...) -> None: ...

class ReleaseQuality(_message.Message):
    __slots__ = ("name", "source", "resolution", "modifier", "version", "real", "repack")
    NAME_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    RESOLUTION_FIELD_NUMBER: _ClassVar[int]
    MODIFIER_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    REAL_FIELD_NUMBER: _ClassVar[int]
    REPACK_FIELD_NUMBER: _ClassVar[int]
    name: str
    source: str
    resolution: int
    modifier: str
    version: int
    real: int
    repack: bool
    def __init__(self, name: _Optional[str] = ..., source: _Optional[str] = ..., resolution: _Optional[int] = ..., modifier: _Optional[str] = ..., version: _Optional[int] = ..., real: _Optional[int] = ..., repack: _Optional[bool] = ...) -> None: ...

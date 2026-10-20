import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.plugin.v1 import settings_pb2 as _settings_pb2
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

class IndexerSearchType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    INDEXER_SEARCH_TYPE_UNSPECIFIED: _ClassVar[IndexerSearchType]
    INDEXER_SEARCH_TYPE_MOVIE: _ClassVar[IndexerSearchType]
    INDEXER_SEARCH_TYPE_TV: _ClassVar[IndexerSearchType]
DOWNLOAD_PROTOCOL_UNSPECIFIED: DownloadProtocol
DOWNLOAD_PROTOCOL_TORRENT: DownloadProtocol
DOWNLOAD_PROTOCOL_USENET: DownloadProtocol
INDEXER_PRIVACY_UNSPECIFIED: IndexerPrivacy
INDEXER_PRIVACY_PUBLIC: IndexerPrivacy
INDEXER_PRIVACY_SEMI_PRIVATE: IndexerPrivacy
INDEXER_PRIVACY_PRIVATE: IndexerPrivacy
INDEXER_SEARCH_TYPE_UNSPECIFIED: IndexerSearchType
INDEXER_SEARCH_TYPE_MOVIE: IndexerSearchType
INDEXER_SEARCH_TYPE_TV: IndexerSearchType

class ListDefinitionsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListDefinitionsResponse(_message.Message):
    __slots__ = ("definitions",)
    DEFINITIONS_FIELD_NUMBER: _ClassVar[int]
    definitions: _containers.RepeatedCompositeFieldContainer[IndexerDefinition]
    def __init__(self, definitions: _Optional[_Iterable[_Union[IndexerDefinition, _Mapping]]] = ...) -> None: ...

class IndexerDefinition(_message.Message):
    __slots__ = ("id", "name", "description", "protocol", "privacy", "language", "links", "fields")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    PRIVACY_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    LINKS_FIELD_NUMBER: _ClassVar[int]
    FIELDS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    description: str
    protocol: DownloadProtocol
    privacy: IndexerPrivacy
    language: str
    links: _containers.RepeatedScalarFieldContainer[str]
    fields: _containers.RepeatedCompositeFieldContainer[_settings_pb2.SettingField]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., description: _Optional[str] = ..., protocol: _Optional[_Union[DownloadProtocol, str]] = ..., privacy: _Optional[_Union[IndexerPrivacy, str]] = ..., language: _Optional[str] = ..., links: _Optional[_Iterable[str]] = ..., fields: _Optional[_Iterable[_Union[_settings_pb2.SettingField, _Mapping]]] = ...) -> None: ...

class ListIndexersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListIndexersResponse(_message.Message):
    __slots__ = ("indexers",)
    INDEXERS_FIELD_NUMBER: _ClassVar[int]
    indexers: _containers.RepeatedCompositeFieldContainer[Indexer]
    def __init__(self, indexers: _Optional[_Iterable[_Union[Indexer, _Mapping]]] = ...) -> None: ...

class Indexer(_message.Message):
    __slots__ = ("id", "name", "definition_id", "protocol", "privacy", "enabled", "priority", "capabilities", "disabled_until", "failure", "tags", "required_flags", "minimum_seeders", "multi_languages", "seed_ratio", "seed_time", "season_pack_seed_time", "reject_blocklisted_torrent_hashes", "anime_standard_format_search", "download_client_id")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DEFINITION_ID_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    PRIVACY_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    PRIORITY_FIELD_NUMBER: _ClassVar[int]
    CAPABILITIES_FIELD_NUMBER: _ClassVar[int]
    DISABLED_UNTIL_FIELD_NUMBER: _ClassVar[int]
    FAILURE_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    REQUIRED_FLAGS_FIELD_NUMBER: _ClassVar[int]
    MINIMUM_SEEDERS_FIELD_NUMBER: _ClassVar[int]
    MULTI_LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    SEED_RATIO_FIELD_NUMBER: _ClassVar[int]
    SEED_TIME_FIELD_NUMBER: _ClassVar[int]
    SEASON_PACK_SEED_TIME_FIELD_NUMBER: _ClassVar[int]
    REJECT_BLOCKLISTED_TORRENT_HASHES_FIELD_NUMBER: _ClassVar[int]
    ANIME_STANDARD_FORMAT_SEARCH_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    definition_id: str
    protocol: DownloadProtocol
    privacy: IndexerPrivacy
    enabled: bool
    priority: int
    capabilities: IndexerCapabilities
    disabled_until: _timestamp_pb2.Timestamp
    failure: str
    tags: _containers.RepeatedScalarFieldContainer[str]
    required_flags: _containers.RepeatedScalarFieldContainer[str]
    minimum_seeders: int
    multi_languages: _containers.RepeatedScalarFieldContainer[str]
    seed_ratio: float
    seed_time: _duration_pb2.Duration
    season_pack_seed_time: _duration_pb2.Duration
    reject_blocklisted_torrent_hashes: bool
    anime_standard_format_search: bool
    download_client_id: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., definition_id: _Optional[str] = ..., protocol: _Optional[_Union[DownloadProtocol, str]] = ..., privacy: _Optional[_Union[IndexerPrivacy, str]] = ..., enabled: _Optional[bool] = ..., priority: _Optional[int] = ..., capabilities: _Optional[_Union[IndexerCapabilities, _Mapping]] = ..., disabled_until: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., failure: _Optional[str] = ..., tags: _Optional[_Iterable[str]] = ..., required_flags: _Optional[_Iterable[str]] = ..., minimum_seeders: _Optional[int] = ..., multi_languages: _Optional[_Iterable[str]] = ..., seed_ratio: _Optional[float] = ..., seed_time: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., season_pack_seed_time: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., reject_blocklisted_torrent_hashes: _Optional[bool] = ..., anime_standard_format_search: _Optional[bool] = ..., download_client_id: _Optional[str] = ...) -> None: ...

class IndexerCapabilities(_message.Message):
    __slots__ = ("search_params", "tv_search_params", "movie_search_params", "categories", "limit_default", "limit_max", "raw_search")
    SEARCH_PARAMS_FIELD_NUMBER: _ClassVar[int]
    TV_SEARCH_PARAMS_FIELD_NUMBER: _ClassVar[int]
    MOVIE_SEARCH_PARAMS_FIELD_NUMBER: _ClassVar[int]
    CATEGORIES_FIELD_NUMBER: _ClassVar[int]
    LIMIT_DEFAULT_FIELD_NUMBER: _ClassVar[int]
    LIMIT_MAX_FIELD_NUMBER: _ClassVar[int]
    RAW_SEARCH_FIELD_NUMBER: _ClassVar[int]
    search_params: _containers.RepeatedScalarFieldContainer[str]
    tv_search_params: _containers.RepeatedScalarFieldContainer[str]
    movie_search_params: _containers.RepeatedScalarFieldContainer[str]
    categories: _containers.RepeatedCompositeFieldContainer[IndexerCategory]
    limit_default: int
    limit_max: int
    raw_search: bool
    def __init__(self, search_params: _Optional[_Iterable[str]] = ..., tv_search_params: _Optional[_Iterable[str]] = ..., movie_search_params: _Optional[_Iterable[str]] = ..., categories: _Optional[_Iterable[_Union[IndexerCategory, _Mapping]]] = ..., limit_default: _Optional[int] = ..., limit_max: _Optional[int] = ..., raw_search: _Optional[bool] = ...) -> None: ...

class IndexerCategory(_message.Message):
    __slots__ = ("id", "name", "subcategories")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    SUBCATEGORIES_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    subcategories: _containers.RepeatedCompositeFieldContainer[IndexerCategory]
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., subcategories: _Optional[_Iterable[_Union[IndexerCategory, _Mapping]]] = ...) -> None: ...

class TestIndexerRequest(_message.Message):
    __slots__ = ("indexer_id", "definition_id", "settings_json")
    INDEXER_ID_FIELD_NUMBER: _ClassVar[int]
    DEFINITION_ID_FIELD_NUMBER: _ClassVar[int]
    SETTINGS_JSON_FIELD_NUMBER: _ClassVar[int]
    indexer_id: str
    definition_id: str
    settings_json: str
    def __init__(self, indexer_id: _Optional[str] = ..., definition_id: _Optional[str] = ..., settings_json: _Optional[str] = ...) -> None: ...

class TestIndexerResponse(_message.Message):
    __slots__ = ("failure",)
    FAILURE_FIELD_NUMBER: _ClassVar[int]
    failure: str
    def __init__(self, failure: _Optional[str] = ...) -> None: ...

class SearchReleasesRequest(_message.Message):
    __slots__ = ("indexer_ids", "type", "query", "categories", "external_ids", "season", "episode", "year", "genre", "limit", "offset", "title", "min_age", "max_age", "min_size", "max_size")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    INDEXER_IDS_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    CATEGORIES_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    SEASON_FIELD_NUMBER: _ClassVar[int]
    EPISODE_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    GENRE_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    MIN_AGE_FIELD_NUMBER: _ClassVar[int]
    MAX_AGE_FIELD_NUMBER: _ClassVar[int]
    MIN_SIZE_FIELD_NUMBER: _ClassVar[int]
    MAX_SIZE_FIELD_NUMBER: _ClassVar[int]
    indexer_ids: _containers.RepeatedScalarFieldContainer[str]
    type: IndexerSearchType
    query: str
    categories: _containers.RepeatedScalarFieldContainer[int]
    external_ids: _containers.ScalarMap[str, str]
    season: int
    episode: str
    year: int
    genre: str
    limit: int
    offset: int
    title: str
    min_age: int
    max_age: int
    min_size: int
    max_size: int
    def __init__(self, indexer_ids: _Optional[_Iterable[str]] = ..., type: _Optional[_Union[IndexerSearchType, str]] = ..., query: _Optional[str] = ..., categories: _Optional[_Iterable[int]] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., season: _Optional[int] = ..., episode: _Optional[str] = ..., year: _Optional[int] = ..., genre: _Optional[str] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ..., title: _Optional[str] = ..., min_age: _Optional[int] = ..., max_age: _Optional[int] = ..., min_size: _Optional[int] = ..., max_size: _Optional[int] = ...) -> None: ...

class SearchReleasesResponse(_message.Message):
    __slots__ = ("releases", "failures")
    RELEASES_FIELD_NUMBER: _ClassVar[int]
    FAILURES_FIELD_NUMBER: _ClassVar[int]
    releases: _containers.RepeatedCompositeFieldContainer[Release]
    failures: _containers.RepeatedCompositeFieldContainer[IndexerFailure]
    def __init__(self, releases: _Optional[_Iterable[_Union[Release, _Mapping]]] = ..., failures: _Optional[_Iterable[_Union[IndexerFailure, _Mapping]]] = ...) -> None: ...

class IndexerFailure(_message.Message):
    __slots__ = ("indexer_id", "message", "disabled_until")
    INDEXER_ID_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    DISABLED_UNTIL_FIELD_NUMBER: _ClassVar[int]
    indexer_id: str
    message: str
    disabled_until: _timestamp_pb2.Timestamp
    def __init__(self, indexer_id: _Optional[str] = ..., message: _Optional[str] = ..., disabled_until: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class Release(_message.Message):
    __slots__ = ("indexer_id", "guid", "title", "description", "size", "download_url", "magnet_url", "info_hash", "info_url", "comments_url", "protocol", "publish_date", "categories", "external_ids", "year", "grabs", "files", "seeders", "peers", "download_volume_factor", "upload_volume_factor", "minimum_ratio", "minimum_seed_time", "flags", "languages", "subtitles", "genres", "poster_url", "origin", "source", "container", "codec", "resolution")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    INDEXER_ID_FIELD_NUMBER: _ClassVar[int]
    GUID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_URL_FIELD_NUMBER: _ClassVar[int]
    MAGNET_URL_FIELD_NUMBER: _ClassVar[int]
    INFO_HASH_FIELD_NUMBER: _ClassVar[int]
    INFO_URL_FIELD_NUMBER: _ClassVar[int]
    COMMENTS_URL_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    PUBLISH_DATE_FIELD_NUMBER: _ClassVar[int]
    CATEGORIES_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    GRABS_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    SEEDERS_FIELD_NUMBER: _ClassVar[int]
    PEERS_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_VOLUME_FACTOR_FIELD_NUMBER: _ClassVar[int]
    UPLOAD_VOLUME_FACTOR_FIELD_NUMBER: _ClassVar[int]
    MINIMUM_RATIO_FIELD_NUMBER: _ClassVar[int]
    MINIMUM_SEED_TIME_FIELD_NUMBER: _ClassVar[int]
    FLAGS_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    SUBTITLES_FIELD_NUMBER: _ClassVar[int]
    GENRES_FIELD_NUMBER: _ClassVar[int]
    POSTER_URL_FIELD_NUMBER: _ClassVar[int]
    ORIGIN_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    CONTAINER_FIELD_NUMBER: _ClassVar[int]
    CODEC_FIELD_NUMBER: _ClassVar[int]
    RESOLUTION_FIELD_NUMBER: _ClassVar[int]
    indexer_id: str
    guid: str
    title: str
    description: str
    size: int
    download_url: str
    magnet_url: str
    info_hash: str
    info_url: str
    comments_url: str
    protocol: DownloadProtocol
    publish_date: _timestamp_pb2.Timestamp
    categories: _containers.RepeatedScalarFieldContainer[int]
    external_ids: _containers.ScalarMap[str, str]
    year: int
    grabs: int
    files: int
    seeders: int
    peers: int
    download_volume_factor: float
    upload_volume_factor: float
    minimum_ratio: float
    minimum_seed_time: int
    flags: _containers.RepeatedScalarFieldContainer[str]
    languages: _containers.RepeatedScalarFieldContainer[str]
    subtitles: _containers.RepeatedScalarFieldContainer[str]
    genres: _containers.RepeatedScalarFieldContainer[str]
    poster_url: str
    origin: str
    source: str
    container: str
    codec: str
    resolution: str
    def __init__(self, indexer_id: _Optional[str] = ..., guid: _Optional[str] = ..., title: _Optional[str] = ..., description: _Optional[str] = ..., size: _Optional[int] = ..., download_url: _Optional[str] = ..., magnet_url: _Optional[str] = ..., info_hash: _Optional[str] = ..., info_url: _Optional[str] = ..., comments_url: _Optional[str] = ..., protocol: _Optional[_Union[DownloadProtocol, str]] = ..., publish_date: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., categories: _Optional[_Iterable[int]] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., year: _Optional[int] = ..., grabs: _Optional[int] = ..., files: _Optional[int] = ..., seeders: _Optional[int] = ..., peers: _Optional[int] = ..., download_volume_factor: _Optional[float] = ..., upload_volume_factor: _Optional[float] = ..., minimum_ratio: _Optional[float] = ..., minimum_seed_time: _Optional[int] = ..., flags: _Optional[_Iterable[str]] = ..., languages: _Optional[_Iterable[str]] = ..., subtitles: _Optional[_Iterable[str]] = ..., genres: _Optional[_Iterable[str]] = ..., poster_url: _Optional[str] = ..., origin: _Optional[str] = ..., source: _Optional[str] = ..., container: _Optional[str] = ..., codec: _Optional[str] = ..., resolution: _Optional[str] = ...) -> None: ...

class GetReleaseFileRequest(_message.Message):
    __slots__ = ("indexer_id", "download_url")
    INDEXER_ID_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_URL_FIELD_NUMBER: _ClassVar[int]
    indexer_id: str
    download_url: str
    def __init__(self, indexer_id: _Optional[str] = ..., download_url: _Optional[str] = ...) -> None: ...

class GetReleaseFileResponse(_message.Message):
    __slots__ = ("data", "content_type", "file_name", "magnet_url")
    DATA_FIELD_NUMBER: _ClassVar[int]
    CONTENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    FILE_NAME_FIELD_NUMBER: _ClassVar[int]
    MAGNET_URL_FIELD_NUMBER: _ClassVar[int]
    data: bytes
    content_type: str
    file_name: str
    magnet_url: str
    def __init__(self, data: _Optional[bytes] = ..., content_type: _Optional[str] = ..., file_name: _Optional[str] = ..., magnet_url: _Optional[str] = ...) -> None: ...

import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.plugin.v1 import indexer_pb2 as _indexer_pb2
from mavio.plugin.v1 import settings_pb2 as _settings_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DownloadKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    DOWNLOAD_KIND_UNSPECIFIED: _ClassVar[DownloadKind]
    DOWNLOAD_KIND_MOVIE: _ClassVar[DownloadKind]
    DOWNLOAD_KIND_SERIES: _ClassVar[DownloadKind]

class DownloadStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    DOWNLOAD_STATUS_UNSPECIFIED: _ClassVar[DownloadStatus]
    DOWNLOAD_STATUS_QUEUED: _ClassVar[DownloadStatus]
    DOWNLOAD_STATUS_PAUSED: _ClassVar[DownloadStatus]
    DOWNLOAD_STATUS_DOWNLOADING: _ClassVar[DownloadStatus]
    DOWNLOAD_STATUS_COMPLETED: _ClassVar[DownloadStatus]
    DOWNLOAD_STATUS_FAILED: _ClassVar[DownloadStatus]
    DOWNLOAD_STATUS_WARNING: _ClassVar[DownloadStatus]
DOWNLOAD_KIND_UNSPECIFIED: DownloadKind
DOWNLOAD_KIND_MOVIE: DownloadKind
DOWNLOAD_KIND_SERIES: DownloadKind
DOWNLOAD_STATUS_UNSPECIFIED: DownloadStatus
DOWNLOAD_STATUS_QUEUED: DownloadStatus
DOWNLOAD_STATUS_PAUSED: DownloadStatus
DOWNLOAD_STATUS_DOWNLOADING: DownloadStatus
DOWNLOAD_STATUS_COMPLETED: DownloadStatus
DOWNLOAD_STATUS_FAILED: DownloadStatus
DOWNLOAD_STATUS_WARNING: DownloadStatus

class ListDownloadClientDefinitionsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListDownloadClientDefinitionsResponse(_message.Message):
    __slots__ = ("definitions",)
    DEFINITIONS_FIELD_NUMBER: _ClassVar[int]
    definitions: _containers.RepeatedCompositeFieldContainer[DownloadClientDefinition]
    def __init__(self, definitions: _Optional[_Iterable[_Union[DownloadClientDefinition, _Mapping]]] = ...) -> None: ...

class DownloadClientDefinition(_message.Message):
    __slots__ = ("id", "name", "description", "protocol", "fields")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    FIELDS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    description: str
    protocol: _indexer_pb2.DownloadProtocol
    fields: _containers.RepeatedCompositeFieldContainer[_settings_pb2.SettingField]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., description: _Optional[str] = ..., protocol: _Optional[_Union[_indexer_pb2.DownloadProtocol, str]] = ..., fields: _Optional[_Iterable[_Union[_settings_pb2.SettingField, _Mapping]]] = ...) -> None: ...

class ListDownloadClientsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListDownloadClientsResponse(_message.Message):
    __slots__ = ("clients",)
    CLIENTS_FIELD_NUMBER: _ClassVar[int]
    clients: _containers.RepeatedCompositeFieldContainer[DownloadClient]
    def __init__(self, clients: _Optional[_Iterable[_Union[DownloadClient, _Mapping]]] = ...) -> None: ...

class DownloadClient(_message.Message):
    __slots__ = ("id", "name", "definition_id", "protocol", "enabled", "priority", "remove_completed_downloads", "remove_failed_downloads", "disabled_until", "failure", "prefers_torrent_file", "host", "tags", "container")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DEFINITION_ID_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    PRIORITY_FIELD_NUMBER: _ClassVar[int]
    REMOVE_COMPLETED_DOWNLOADS_FIELD_NUMBER: _ClassVar[int]
    REMOVE_FAILED_DOWNLOADS_FIELD_NUMBER: _ClassVar[int]
    DISABLED_UNTIL_FIELD_NUMBER: _ClassVar[int]
    FAILURE_FIELD_NUMBER: _ClassVar[int]
    PREFERS_TORRENT_FILE_FIELD_NUMBER: _ClassVar[int]
    HOST_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    CONTAINER_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    definition_id: str
    protocol: _indexer_pb2.DownloadProtocol
    enabled: bool
    priority: int
    remove_completed_downloads: bool
    remove_failed_downloads: bool
    disabled_until: _timestamp_pb2.Timestamp
    failure: str
    prefers_torrent_file: bool
    host: str
    tags: _containers.RepeatedScalarFieldContainer[str]
    container: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., definition_id: _Optional[str] = ..., protocol: _Optional[_Union[_indexer_pb2.DownloadProtocol, str]] = ..., enabled: _Optional[bool] = ..., priority: _Optional[int] = ..., remove_completed_downloads: _Optional[bool] = ..., remove_failed_downloads: _Optional[bool] = ..., disabled_until: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., failure: _Optional[str] = ..., prefers_torrent_file: _Optional[bool] = ..., host: _Optional[str] = ..., tags: _Optional[_Iterable[str]] = ..., container: _Optional[str] = ...) -> None: ...

class TestDownloadClientRequest(_message.Message):
    __slots__ = ("client_id", "definition_id", "settings_json")
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    DEFINITION_ID_FIELD_NUMBER: _ClassVar[int]
    SETTINGS_JSON_FIELD_NUMBER: _ClassVar[int]
    client_id: str
    definition_id: str
    settings_json: str
    def __init__(self, client_id: _Optional[str] = ..., definition_id: _Optional[str] = ..., settings_json: _Optional[str] = ...) -> None: ...

class TestDownloadClientResponse(_message.Message):
    __slots__ = ("failure", "warnings")
    FAILURE_FIELD_NUMBER: _ClassVar[int]
    WARNINGS_FIELD_NUMBER: _ClassVar[int]
    failure: str
    warnings: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, failure: _Optional[str] = ..., warnings: _Optional[_Iterable[str]] = ...) -> None: ...

class AddDownloadRequest(_message.Message):
    __slots__ = ("client_id", "title", "kind", "recent", "data", "file_name", "magnet_url", "info_hash", "seed_ratio", "seed_time", "full_season", "details", "blocklisted_info_hashes")
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    RECENT_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    FILE_NAME_FIELD_NUMBER: _ClassVar[int]
    MAGNET_URL_FIELD_NUMBER: _ClassVar[int]
    INFO_HASH_FIELD_NUMBER: _ClassVar[int]
    SEED_RATIO_FIELD_NUMBER: _ClassVar[int]
    SEED_TIME_FIELD_NUMBER: _ClassVar[int]
    FULL_SEASON_FIELD_NUMBER: _ClassVar[int]
    DETAILS_FIELD_NUMBER: _ClassVar[int]
    BLOCKLISTED_INFO_HASHES_FIELD_NUMBER: _ClassVar[int]
    client_id: str
    title: str
    kind: DownloadKind
    recent: bool
    data: bytes
    file_name: str
    magnet_url: str
    info_hash: str
    seed_ratio: float
    seed_time: _duration_pb2.Duration
    full_season: bool
    details: DownloadDetails
    blocklisted_info_hashes: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, client_id: _Optional[str] = ..., title: _Optional[str] = ..., kind: _Optional[_Union[DownloadKind, str]] = ..., recent: _Optional[bool] = ..., data: _Optional[bytes] = ..., file_name: _Optional[str] = ..., magnet_url: _Optional[str] = ..., info_hash: _Optional[str] = ..., seed_ratio: _Optional[float] = ..., seed_time: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., full_season: _Optional[bool] = ..., details: _Optional[_Union[DownloadDetails, _Mapping]] = ..., blocklisted_info_hashes: _Optional[_Iterable[str]] = ...) -> None: ...

class DownloadDetails(_message.Message):
    __slots__ = ("quality", "languages", "release_group", "indexer", "year", "collection", "studio", "title_slug")
    QUALITY_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    RELEASE_GROUP_FIELD_NUMBER: _ClassVar[int]
    INDEXER_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    COLLECTION_FIELD_NUMBER: _ClassVar[int]
    STUDIO_FIELD_NUMBER: _ClassVar[int]
    TITLE_SLUG_FIELD_NUMBER: _ClassVar[int]
    quality: str
    languages: _containers.RepeatedScalarFieldContainer[str]
    release_group: str
    indexer: str
    year: int
    collection: str
    studio: str
    title_slug: str
    def __init__(self, quality: _Optional[str] = ..., languages: _Optional[_Iterable[str]] = ..., release_group: _Optional[str] = ..., indexer: _Optional[str] = ..., year: _Optional[int] = ..., collection: _Optional[str] = ..., studio: _Optional[str] = ..., title_slug: _Optional[str] = ...) -> None: ...

class AddDownloadResponse(_message.Message):
    __slots__ = ("download_id",)
    DOWNLOAD_ID_FIELD_NUMBER: _ClassVar[int]
    download_id: str
    def __init__(self, download_id: _Optional[str] = ...) -> None: ...

class ListDownloadsRequest(_message.Message):
    __slots__ = ("client_ids", "include_statuses")
    CLIENT_IDS_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_STATUSES_FIELD_NUMBER: _ClassVar[int]
    client_ids: _containers.RepeatedScalarFieldContainer[str]
    include_statuses: bool
    def __init__(self, client_ids: _Optional[_Iterable[str]] = ..., include_statuses: _Optional[bool] = ...) -> None: ...

class ListDownloadsResponse(_message.Message):
    __slots__ = ("downloads", "statuses", "failures")
    DOWNLOADS_FIELD_NUMBER: _ClassVar[int]
    STATUSES_FIELD_NUMBER: _ClassVar[int]
    FAILURES_FIELD_NUMBER: _ClassVar[int]
    downloads: _containers.RepeatedCompositeFieldContainer[Download]
    statuses: _containers.RepeatedCompositeFieldContainer[DownloadClientStatus]
    failures: _containers.RepeatedCompositeFieldContainer[DownloadClientFailure]
    def __init__(self, downloads: _Optional[_Iterable[_Union[Download, _Mapping]]] = ..., statuses: _Optional[_Iterable[_Union[DownloadClientStatus, _Mapping]]] = ..., failures: _Optional[_Iterable[_Union[DownloadClientFailure, _Mapping]]] = ...) -> None: ...

class Download(_message.Message):
    __slots__ = ("client_id", "id", "title", "category", "kind", "total_size", "remaining_size", "remaining_time", "seed_ratio", "output_path", "message", "status", "encrypted", "can_move_files", "can_be_removed", "has_post_import_category")
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    CATEGORY_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    TOTAL_SIZE_FIELD_NUMBER: _ClassVar[int]
    REMAINING_SIZE_FIELD_NUMBER: _ClassVar[int]
    REMAINING_TIME_FIELD_NUMBER: _ClassVar[int]
    SEED_RATIO_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_PATH_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ENCRYPTED_FIELD_NUMBER: _ClassVar[int]
    CAN_MOVE_FILES_FIELD_NUMBER: _ClassVar[int]
    CAN_BE_REMOVED_FIELD_NUMBER: _ClassVar[int]
    HAS_POST_IMPORT_CATEGORY_FIELD_NUMBER: _ClassVar[int]
    client_id: str
    id: str
    title: str
    category: str
    kind: DownloadKind
    total_size: int
    remaining_size: int
    remaining_time: _duration_pb2.Duration
    seed_ratio: float
    output_path: str
    message: str
    status: DownloadStatus
    encrypted: bool
    can_move_files: bool
    can_be_removed: bool
    has_post_import_category: bool
    def __init__(self, client_id: _Optional[str] = ..., id: _Optional[str] = ..., title: _Optional[str] = ..., category: _Optional[str] = ..., kind: _Optional[_Union[DownloadKind, str]] = ..., total_size: _Optional[int] = ..., remaining_size: _Optional[int] = ..., remaining_time: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., seed_ratio: _Optional[float] = ..., output_path: _Optional[str] = ..., message: _Optional[str] = ..., status: _Optional[_Union[DownloadStatus, str]] = ..., encrypted: _Optional[bool] = ..., can_move_files: _Optional[bool] = ..., can_be_removed: _Optional[bool] = ..., has_post_import_category: _Optional[bool] = ...) -> None: ...

class DownloadClientStatus(_message.Message):
    __slots__ = ("client_id", "is_localhost", "sorting_mode", "removes_completed_downloads", "output_root_folders")
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    IS_LOCALHOST_FIELD_NUMBER: _ClassVar[int]
    SORTING_MODE_FIELD_NUMBER: _ClassVar[int]
    REMOVES_COMPLETED_DOWNLOADS_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_ROOT_FOLDERS_FIELD_NUMBER: _ClassVar[int]
    client_id: str
    is_localhost: bool
    sorting_mode: str
    removes_completed_downloads: bool
    output_root_folders: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, client_id: _Optional[str] = ..., is_localhost: _Optional[bool] = ..., sorting_mode: _Optional[str] = ..., removes_completed_downloads: _Optional[bool] = ..., output_root_folders: _Optional[_Iterable[str]] = ...) -> None: ...

class DownloadClientFailure(_message.Message):
    __slots__ = ("client_id", "message", "disabled_until")
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    DISABLED_UNTIL_FIELD_NUMBER: _ClassVar[int]
    client_id: str
    message: str
    disabled_until: _timestamp_pb2.Timestamp
    def __init__(self, client_id: _Optional[str] = ..., message: _Optional[str] = ..., disabled_until: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetDownloadRequest(_message.Message):
    __slots__ = ("client_id", "download_id")
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_ID_FIELD_NUMBER: _ClassVar[int]
    client_id: str
    download_id: str
    def __init__(self, client_id: _Optional[str] = ..., download_id: _Optional[str] = ...) -> None: ...

class GetDownloadResponse(_message.Message):
    __slots__ = ("download",)
    DOWNLOAD_FIELD_NUMBER: _ClassVar[int]
    download: Download
    def __init__(self, download: _Optional[_Union[Download, _Mapping]] = ...) -> None: ...

class RemoveDownloadRequest(_message.Message):
    __slots__ = ("client_id", "download_id", "delete_data")
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_ID_FIELD_NUMBER: _ClassVar[int]
    DELETE_DATA_FIELD_NUMBER: _ClassVar[int]
    client_id: str
    download_id: str
    delete_data: bool
    def __init__(self, client_id: _Optional[str] = ..., download_id: _Optional[str] = ..., delete_data: _Optional[bool] = ...) -> None: ...

class RemoveDownloadResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class MarkDownloadImportedRequest(_message.Message):
    __slots__ = ("client_id", "download_id")
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_ID_FIELD_NUMBER: _ClassVar[int]
    client_id: str
    download_id: str
    def __init__(self, client_id: _Optional[str] = ..., download_id: _Optional[str] = ...) -> None: ...

class MarkDownloadImportedResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

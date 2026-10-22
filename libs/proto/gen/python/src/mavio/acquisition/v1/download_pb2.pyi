import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.acquisition.v1 import release_pb2 as _release_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DownloadState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    DOWNLOAD_STATE_UNSPECIFIED: _ClassVar[DownloadState]
    DOWNLOAD_STATE_DOWNLOADING: _ClassVar[DownloadState]
    DOWNLOAD_STATE_IMPORT_BLOCKED: _ClassVar[DownloadState]
    DOWNLOAD_STATE_IMPORT_PENDING: _ClassVar[DownloadState]
    DOWNLOAD_STATE_IMPORTING: _ClassVar[DownloadState]
    DOWNLOAD_STATE_IMPORTED: _ClassVar[DownloadState]
    DOWNLOAD_STATE_FAILED_PENDING: _ClassVar[DownloadState]
    DOWNLOAD_STATE_FAILED: _ClassVar[DownloadState]
    DOWNLOAD_STATE_IGNORED: _ClassVar[DownloadState]

class DownloadStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    DOWNLOAD_STATUS_UNSPECIFIED: _ClassVar[DownloadStatus]
    DOWNLOAD_STATUS_QUEUED: _ClassVar[DownloadStatus]
    DOWNLOAD_STATUS_PAUSED: _ClassVar[DownloadStatus]
    DOWNLOAD_STATUS_DOWNLOADING: _ClassVar[DownloadStatus]
    DOWNLOAD_STATUS_COMPLETED: _ClassVar[DownloadStatus]
    DOWNLOAD_STATUS_FAILED: _ClassVar[DownloadStatus]
    DOWNLOAD_STATUS_WARNING: _ClassVar[DownloadStatus]

class HistoryEventType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    HISTORY_EVENT_TYPE_UNSPECIFIED: _ClassVar[HistoryEventType]
    HISTORY_EVENT_TYPE_GRABBED: _ClassVar[HistoryEventType]
    HISTORY_EVENT_TYPE_IMPORTED: _ClassVar[HistoryEventType]
    HISTORY_EVENT_TYPE_DOWNLOAD_FAILED: _ClassVar[HistoryEventType]
    HISTORY_EVENT_TYPE_FILE_DELETED: _ClassVar[HistoryEventType]
    HISTORY_EVENT_TYPE_FILE_RENAMED: _ClassVar[HistoryEventType]
    HISTORY_EVENT_TYPE_IGNORED: _ClassVar[HistoryEventType]

class ImportMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    IMPORT_MODE_UNSPECIFIED: _ClassVar[ImportMode]
    IMPORT_MODE_MOVE: _ClassVar[ImportMode]
    IMPORT_MODE_COPY: _ClassVar[ImportMode]
DOWNLOAD_STATE_UNSPECIFIED: DownloadState
DOWNLOAD_STATE_DOWNLOADING: DownloadState
DOWNLOAD_STATE_IMPORT_BLOCKED: DownloadState
DOWNLOAD_STATE_IMPORT_PENDING: DownloadState
DOWNLOAD_STATE_IMPORTING: DownloadState
DOWNLOAD_STATE_IMPORTED: DownloadState
DOWNLOAD_STATE_FAILED_PENDING: DownloadState
DOWNLOAD_STATE_FAILED: DownloadState
DOWNLOAD_STATE_IGNORED: DownloadState
DOWNLOAD_STATUS_UNSPECIFIED: DownloadStatus
DOWNLOAD_STATUS_QUEUED: DownloadStatus
DOWNLOAD_STATUS_PAUSED: DownloadStatus
DOWNLOAD_STATUS_DOWNLOADING: DownloadStatus
DOWNLOAD_STATUS_COMPLETED: DownloadStatus
DOWNLOAD_STATUS_FAILED: DownloadStatus
DOWNLOAD_STATUS_WARNING: DownloadStatus
HISTORY_EVENT_TYPE_UNSPECIFIED: HistoryEventType
HISTORY_EVENT_TYPE_GRABBED: HistoryEventType
HISTORY_EVENT_TYPE_IMPORTED: HistoryEventType
HISTORY_EVENT_TYPE_DOWNLOAD_FAILED: HistoryEventType
HISTORY_EVENT_TYPE_FILE_DELETED: HistoryEventType
HISTORY_EVENT_TYPE_FILE_RENAMED: HistoryEventType
HISTORY_EVENT_TYPE_IGNORED: HistoryEventType
IMPORT_MODE_UNSPECIFIED: ImportMode
IMPORT_MODE_MOVE: ImportMode
IMPORT_MODE_COPY: ImportMode

class DownloadClient(_message.Message):
    __slots__ = ("plugin_id", "id", "name", "definition_id", "protocol", "enabled", "priority", "disabled_until", "failure", "container")
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DEFINITION_ID_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    PRIORITY_FIELD_NUMBER: _ClassVar[int]
    DISABLED_UNTIL_FIELD_NUMBER: _ClassVar[int]
    FAILURE_FIELD_NUMBER: _ClassVar[int]
    CONTAINER_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    id: str
    name: str
    definition_id: str
    protocol: _release_pb2.DownloadProtocol
    enabled: bool
    priority: int
    disabled_until: _timestamp_pb2.Timestamp
    failure: str
    container: str
    def __init__(self, plugin_id: _Optional[str] = ..., id: _Optional[str] = ..., name: _Optional[str] = ..., definition_id: _Optional[str] = ..., protocol: _Optional[_Union[_release_pb2.DownloadProtocol, str]] = ..., enabled: _Optional[bool] = ..., priority: _Optional[int] = ..., disabled_until: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., failure: _Optional[str] = ..., container: _Optional[str] = ...) -> None: ...

class ListDownloadClientsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListDownloadClientsResponse(_message.Message):
    __slots__ = ("clients", "failures")
    class FailuresEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    CLIENTS_FIELD_NUMBER: _ClassVar[int]
    FAILURES_FIELD_NUMBER: _ClassVar[int]
    clients: _containers.RepeatedCompositeFieldContainer[DownloadClient]
    failures: _containers.ScalarMap[str, str]
    def __init__(self, clients: _Optional[_Iterable[_Union[DownloadClient, _Mapping]]] = ..., failures: _Optional[_Mapping[str, str]] = ...) -> None: ...

class GrabReleaseRequest(_message.Message):
    __slots__ = ("item_id", "release", "client_plugin_id", "client_id")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    RELEASE_FIELD_NUMBER: _ClassVar[int]
    CLIENT_PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    release: _release_pb2.Release
    client_plugin_id: str
    client_id: str
    def __init__(self, item_id: _Optional[str] = ..., release: _Optional[_Union[_release_pb2.Release, _Mapping]] = ..., client_plugin_id: _Optional[str] = ..., client_id: _Optional[str] = ...) -> None: ...

class GrabReleaseResponse(_message.Message):
    __slots__ = ("item", "client_name")
    ITEM_FIELD_NUMBER: _ClassVar[int]
    CLIENT_NAME_FIELD_NUMBER: _ClassVar[int]
    item: QueueItem
    client_name: str
    def __init__(self, item: _Optional[_Union[QueueItem, _Mapping]] = ..., client_name: _Optional[str] = ...) -> None: ...

class QueueItem(_message.Message):
    __slots__ = ("id", "item_id", "item_name", "title", "client_plugin_id", "client_id", "download_id", "indexer", "indexer_name", "protocol", "quality", "languages", "size", "state", "messages", "grab_time", "status", "remaining_size", "remaining_time")
    ID_FIELD_NUMBER: _ClassVar[int]
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    ITEM_NAME_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    CLIENT_PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_ID_FIELD_NUMBER: _ClassVar[int]
    INDEXER_FIELD_NUMBER: _ClassVar[int]
    INDEXER_NAME_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    QUALITY_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    GRAB_TIME_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    REMAINING_SIZE_FIELD_NUMBER: _ClassVar[int]
    REMAINING_TIME_FIELD_NUMBER: _ClassVar[int]
    id: str
    item_id: str
    item_name: str
    title: str
    client_plugin_id: str
    client_id: str
    download_id: str
    indexer: _release_pb2.IndexerRef
    indexer_name: str
    protocol: _release_pb2.DownloadProtocol
    quality: str
    languages: _containers.RepeatedScalarFieldContainer[str]
    size: int
    state: DownloadState
    messages: _containers.RepeatedScalarFieldContainer[str]
    grab_time: _timestamp_pb2.Timestamp
    status: DownloadStatus
    remaining_size: int
    remaining_time: _duration_pb2.Duration
    def __init__(self, id: _Optional[str] = ..., item_id: _Optional[str] = ..., item_name: _Optional[str] = ..., title: _Optional[str] = ..., client_plugin_id: _Optional[str] = ..., client_id: _Optional[str] = ..., download_id: _Optional[str] = ..., indexer: _Optional[_Union[_release_pb2.IndexerRef, _Mapping]] = ..., indexer_name: _Optional[str] = ..., protocol: _Optional[_Union[_release_pb2.DownloadProtocol, str]] = ..., quality: _Optional[str] = ..., languages: _Optional[_Iterable[str]] = ..., size: _Optional[int] = ..., state: _Optional[_Union[DownloadState, str]] = ..., messages: _Optional[_Iterable[str]] = ..., grab_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., status: _Optional[_Union[DownloadStatus, str]] = ..., remaining_size: _Optional[int] = ..., remaining_time: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class ListQueueRequest(_message.Message):
    __slots__ = ("item_id", "include_finished", "limit", "offset")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_FINISHED_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    include_finished: bool
    limit: int
    offset: int
    def __init__(self, item_id: _Optional[str] = ..., include_finished: _Optional[bool] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class ListQueueResponse(_message.Message):
    __slots__ = ("items", "total")
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[QueueItem]
    total: int
    def __init__(self, items: _Optional[_Iterable[_Union[QueueItem, _Mapping]]] = ..., total: _Optional[int] = ...) -> None: ...

class RemoveFromQueueRequest(_message.Message):
    __slots__ = ("id", "remove_from_client", "blocklist", "search")
    ID_FIELD_NUMBER: _ClassVar[int]
    REMOVE_FROM_CLIENT_FIELD_NUMBER: _ClassVar[int]
    BLOCKLIST_FIELD_NUMBER: _ClassVar[int]
    SEARCH_FIELD_NUMBER: _ClassVar[int]
    id: str
    remove_from_client: bool
    blocklist: bool
    search: bool
    def __init__(self, id: _Optional[str] = ..., remove_from_client: _Optional[bool] = ..., blocklist: _Optional[bool] = ..., search: _Optional[bool] = ...) -> None: ...

class RemoveFromQueueResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class HistoryEvent(_message.Message):
    __slots__ = ("id", "type", "item_id", "time", "source_title", "quality", "languages", "download_id", "data")
    class DataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    TIME_FIELD_NUMBER: _ClassVar[int]
    SOURCE_TITLE_FIELD_NUMBER: _ClassVar[int]
    QUALITY_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_ID_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    id: str
    type: HistoryEventType
    item_id: str
    time: _timestamp_pb2.Timestamp
    source_title: str
    quality: str
    languages: _containers.RepeatedScalarFieldContainer[str]
    download_id: str
    data: _containers.ScalarMap[str, str]
    def __init__(self, id: _Optional[str] = ..., type: _Optional[_Union[HistoryEventType, str]] = ..., item_id: _Optional[str] = ..., time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., source_title: _Optional[str] = ..., quality: _Optional[str] = ..., languages: _Optional[_Iterable[str]] = ..., download_id: _Optional[str] = ..., data: _Optional[_Mapping[str, str]] = ...) -> None: ...

class ListHistoryRequest(_message.Message):
    __slots__ = ("item_id", "types", "limit", "offset")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    TYPES_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    types: _containers.RepeatedScalarFieldContainer[HistoryEventType]
    limit: int
    offset: int
    def __init__(self, item_id: _Optional[str] = ..., types: _Optional[_Iterable[_Union[HistoryEventType, str]]] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class ListHistoryResponse(_message.Message):
    __slots__ = ("events", "total")
    EVENTS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    events: _containers.RepeatedCompositeFieldContainer[HistoryEvent]
    total: int
    def __init__(self, events: _Optional[_Iterable[_Union[HistoryEvent, _Mapping]]] = ..., total: _Optional[int] = ...) -> None: ...

class MarkFailedRequest(_message.Message):
    __slots__ = ("history_event_id",)
    HISTORY_EVENT_ID_FIELD_NUMBER: _ClassVar[int]
    history_event_id: str
    def __init__(self, history_event_id: _Optional[str] = ...) -> None: ...

class MarkFailedResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class BlocklistEntry(_message.Message):
    __slots__ = ("id", "item_id", "source_title", "quality", "languages", "time", "publish_time", "size", "protocol", "indexer", "indexer_name", "message")
    ID_FIELD_NUMBER: _ClassVar[int]
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_TITLE_FIELD_NUMBER: _ClassVar[int]
    QUALITY_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    TIME_FIELD_NUMBER: _ClassVar[int]
    PUBLISH_TIME_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    INDEXER_FIELD_NUMBER: _ClassVar[int]
    INDEXER_NAME_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    id: str
    item_id: str
    source_title: str
    quality: str
    languages: _containers.RepeatedScalarFieldContainer[str]
    time: _timestamp_pb2.Timestamp
    publish_time: _timestamp_pb2.Timestamp
    size: int
    protocol: _release_pb2.DownloadProtocol
    indexer: _release_pb2.IndexerRef
    indexer_name: str
    message: str
    def __init__(self, id: _Optional[str] = ..., item_id: _Optional[str] = ..., source_title: _Optional[str] = ..., quality: _Optional[str] = ..., languages: _Optional[_Iterable[str]] = ..., time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., publish_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., size: _Optional[int] = ..., protocol: _Optional[_Union[_release_pb2.DownloadProtocol, str]] = ..., indexer: _Optional[_Union[_release_pb2.IndexerRef, _Mapping]] = ..., indexer_name: _Optional[str] = ..., message: _Optional[str] = ...) -> None: ...

class ListBlocklistRequest(_message.Message):
    __slots__ = ("item_id", "limit", "offset")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    limit: int
    offset: int
    def __init__(self, item_id: _Optional[str] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class ListBlocklistResponse(_message.Message):
    __slots__ = ("entries", "total")
    ENTRIES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    entries: _containers.RepeatedCompositeFieldContainer[BlocklistEntry]
    total: int
    def __init__(self, entries: _Optional[_Iterable[_Union[BlocklistEntry, _Mapping]]] = ..., total: _Optional[int] = ...) -> None: ...

class DeleteBlocklistRequest(_message.Message):
    __slots__ = ("ids",)
    IDS_FIELD_NUMBER: _ClassVar[int]
    ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, ids: _Optional[_Iterable[str]] = ...) -> None: ...

class DeleteBlocklistResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ManualImportFile(_message.Message):
    __slots__ = ("path", "relative_path", "size", "quality", "revision", "rejections", "movie_id", "languages", "release_group")
    PATH_FIELD_NUMBER: _ClassVar[int]
    RELATIVE_PATH_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    QUALITY_FIELD_NUMBER: _ClassVar[int]
    REVISION_FIELD_NUMBER: _ClassVar[int]
    REJECTIONS_FIELD_NUMBER: _ClassVar[int]
    MOVIE_ID_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    RELEASE_GROUP_FIELD_NUMBER: _ClassVar[int]
    path: str
    relative_path: str
    size: int
    quality: str
    revision: int
    rejections: _containers.RepeatedScalarFieldContainer[str]
    movie_id: str
    languages: _containers.RepeatedScalarFieldContainer[str]
    release_group: str
    def __init__(self, path: _Optional[str] = ..., relative_path: _Optional[str] = ..., size: _Optional[int] = ..., quality: _Optional[str] = ..., revision: _Optional[int] = ..., rejections: _Optional[_Iterable[str]] = ..., movie_id: _Optional[str] = ..., languages: _Optional[_Iterable[str]] = ..., release_group: _Optional[str] = ...) -> None: ...

class ListManualImportRequest(_message.Message):
    __slots__ = ("id", "folder", "movie_id")
    ID_FIELD_NUMBER: _ClassVar[int]
    FOLDER_FIELD_NUMBER: _ClassVar[int]
    MOVIE_ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    folder: str
    movie_id: str
    def __init__(self, id: _Optional[str] = ..., folder: _Optional[str] = ..., movie_id: _Optional[str] = ...) -> None: ...

class ListManualImportResponse(_message.Message):
    __slots__ = ("files",)
    FILES_FIELD_NUMBER: _ClassVar[int]
    files: _containers.RepeatedCompositeFieldContainer[ManualImportFile]
    def __init__(self, files: _Optional[_Iterable[_Union[ManualImportFile, _Mapping]]] = ...) -> None: ...

class ManualImportRequest(_message.Message):
    __slots__ = ("id", "path", "quality", "folder", "movie_id", "languages", "release_group", "mode")
    ID_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    QUALITY_FIELD_NUMBER: _ClassVar[int]
    FOLDER_FIELD_NUMBER: _ClassVar[int]
    MOVIE_ID_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    RELEASE_GROUP_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    id: str
    path: str
    quality: str
    folder: str
    movie_id: str
    languages: _containers.RepeatedScalarFieldContainer[str]
    release_group: str
    mode: ImportMode
    def __init__(self, id: _Optional[str] = ..., path: _Optional[str] = ..., quality: _Optional[str] = ..., folder: _Optional[str] = ..., movie_id: _Optional[str] = ..., languages: _Optional[_Iterable[str]] = ..., release_group: _Optional[str] = ..., mode: _Optional[_Union[ImportMode, str]] = ...) -> None: ...

class ManualImportResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class DownloadClientPath(_message.Message):
    __slots__ = ("remote_path", "local_path", "root_folder", "suggested_path", "readable", "error", "link_library_paths", "container")
    REMOTE_PATH_FIELD_NUMBER: _ClassVar[int]
    LOCAL_PATH_FIELD_NUMBER: _ClassVar[int]
    ROOT_FOLDER_FIELD_NUMBER: _ClassVar[int]
    SUGGESTED_PATH_FIELD_NUMBER: _ClassVar[int]
    READABLE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    LINK_LIBRARY_PATHS_FIELD_NUMBER: _ClassVar[int]
    CONTAINER_FIELD_NUMBER: _ClassVar[int]
    remote_path: str
    local_path: str
    root_folder: bool
    suggested_path: str
    readable: bool
    error: str
    link_library_paths: _containers.RepeatedScalarFieldContainer[str]
    container: bool
    def __init__(self, remote_path: _Optional[str] = ..., local_path: _Optional[str] = ..., root_folder: _Optional[bool] = ..., suggested_path: _Optional[str] = ..., readable: _Optional[bool] = ..., error: _Optional[str] = ..., link_library_paths: _Optional[_Iterable[str]] = ..., container: _Optional[bool] = ...) -> None: ...

class GetDownloadClientPathsRequest(_message.Message):
    __slots__ = ("plugin_id", "client_id")
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    client_id: str
    def __init__(self, plugin_id: _Optional[str] = ..., client_id: _Optional[str] = ...) -> None: ...

class GetDownloadClientPathsResponse(_message.Message):
    __slots__ = ("paths", "failure")
    PATHS_FIELD_NUMBER: _ClassVar[int]
    FAILURE_FIELD_NUMBER: _ClassVar[int]
    paths: _containers.RepeatedCompositeFieldContainer[DownloadClientPath]
    failure: str
    def __init__(self, paths: _Optional[_Iterable[_Union[DownloadClientPath, _Mapping]]] = ..., failure: _Optional[str] = ...) -> None: ...

class UpdateDownloadClientPathsRequest(_message.Message):
    __slots__ = ("plugin_id", "client_id", "paths")
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    PATHS_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    client_id: str
    paths: _containers.RepeatedCompositeFieldContainer[DownloadClientPath]
    def __init__(self, plugin_id: _Optional[str] = ..., client_id: _Optional[str] = ..., paths: _Optional[_Iterable[_Union[DownloadClientPath, _Mapping]]] = ...) -> None: ...

class UpdateDownloadClientPathsResponse(_message.Message):
    __slots__ = ("paths", "failure")
    PATHS_FIELD_NUMBER: _ClassVar[int]
    FAILURE_FIELD_NUMBER: _ClassVar[int]
    paths: _containers.RepeatedCompositeFieldContainer[DownloadClientPath]
    failure: str
    def __init__(self, paths: _Optional[_Iterable[_Union[DownloadClientPath, _Mapping]]] = ..., failure: _Optional[str] = ...) -> None: ...

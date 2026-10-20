import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class LibraryKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    LIBRARY_KIND_UNSPECIFIED: _ClassVar[LibraryKind]
    LIBRARY_KIND_MOVIES: _ClassVar[LibraryKind]
    LIBRARY_KIND_SHOWS: _ClassVar[LibraryKind]
    LIBRARY_KIND_MUSIC: _ClassVar[LibraryKind]
    LIBRARY_KIND_MUSIC_VIDEOS: _ClassVar[LibraryKind]
    LIBRARY_KIND_HOME_VIDEOS: _ClassVar[LibraryKind]
    LIBRARY_KIND_BOOKS: _ClassVar[LibraryKind]
    LIBRARY_KIND_PHOTOS: _ClassVar[LibraryKind]
    LIBRARY_KIND_MIXED: _ClassVar[LibraryKind]
    LIBRARY_KIND_COLLECTIONS: _ClassVar[LibraryKind]
    LIBRARY_KIND_PLAYLISTS: _ClassVar[LibraryKind]

class ScanPhase(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SCAN_PHASE_UNSPECIFIED: _ClassVar[ScanPhase]
    SCAN_PHASE_FOLDERS: _ClassVar[ScanPhase]
    SCAN_PHASE_ITEMS: _ClassVar[ScanPhase]

class StorageClass(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    STORAGE_CLASS_UNSPECIFIED: _ClassVar[StorageClass]
    STORAGE_CLASS_LOCAL: _ClassVar[StorageClass]
    STORAGE_CLASS_NETWORK: _ClassVar[StorageClass]
    STORAGE_CLASS_CLOUD: _ClassVar[StorageClass]

class HubKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    HUB_KIND_UNSPECIFIED: _ClassVar[HubKind]
    HUB_KIND_CONTINUE_WATCHING: _ClassVar[HubKind]
    HUB_KIND_RECENTLY_ADDED: _ClassVar[HubKind]
    HUB_KIND_RECENTLY_RELEASED: _ClassVar[HubKind]
    HUB_KIND_RECENTLY_AIRED: _ClassVar[HubKind]
    HUB_KIND_TOP_RATED: _ClassVar[HubKind]
    HUB_KIND_TOP_UNPLAYED: _ClassVar[HubKind]
    HUB_KIND_START_WATCHING: _ClassVar[HubKind]
    HUB_KIND_REDISCOVER: _ClassVar[HubKind]
    HUB_KIND_BECAUSE_YOU_WATCHED: _ClassVar[HubKind]
    HUB_KIND_BECAUSE_YOU_LIKE: _ClassVar[HubKind]
    HUB_KIND_MORE_IN_GENRE: _ClassVar[HubKind]
    HUB_KIND_STARRING: _ClassVar[HubKind]
    HUB_KIND_DIRECTED_BY: _ClassVar[HubKind]
    HUB_KIND_AVAILABLE_REQUESTS: _ClassVar[HubKind]
    HUB_KIND_TRENDING: _ClassVar[HubKind]
LIBRARY_KIND_UNSPECIFIED: LibraryKind
LIBRARY_KIND_MOVIES: LibraryKind
LIBRARY_KIND_SHOWS: LibraryKind
LIBRARY_KIND_MUSIC: LibraryKind
LIBRARY_KIND_MUSIC_VIDEOS: LibraryKind
LIBRARY_KIND_HOME_VIDEOS: LibraryKind
LIBRARY_KIND_BOOKS: LibraryKind
LIBRARY_KIND_PHOTOS: LibraryKind
LIBRARY_KIND_MIXED: LibraryKind
LIBRARY_KIND_COLLECTIONS: LibraryKind
LIBRARY_KIND_PLAYLISTS: LibraryKind
SCAN_PHASE_UNSPECIFIED: ScanPhase
SCAN_PHASE_FOLDERS: ScanPhase
SCAN_PHASE_ITEMS: ScanPhase
STORAGE_CLASS_UNSPECIFIED: StorageClass
STORAGE_CLASS_LOCAL: StorageClass
STORAGE_CLASS_NETWORK: StorageClass
STORAGE_CLASS_CLOUD: StorageClass
HUB_KIND_UNSPECIFIED: HubKind
HUB_KIND_CONTINUE_WATCHING: HubKind
HUB_KIND_RECENTLY_ADDED: HubKind
HUB_KIND_RECENTLY_RELEASED: HubKind
HUB_KIND_RECENTLY_AIRED: HubKind
HUB_KIND_TOP_RATED: HubKind
HUB_KIND_TOP_UNPLAYED: HubKind
HUB_KIND_START_WATCHING: HubKind
HUB_KIND_REDISCOVER: HubKind
HUB_KIND_BECAUSE_YOU_WATCHED: HubKind
HUB_KIND_BECAUSE_YOU_LIKE: HubKind
HUB_KIND_MORE_IN_GENRE: HubKind
HUB_KIND_STARRING: HubKind
HUB_KIND_DIRECTED_BY: HubKind
HUB_KIND_AVAILABLE_REQUESTS: HubKind
HUB_KIND_TRENDING: HubKind

class Library(_message.Message):
    __slots__ = ("id", "name", "kind", "paths", "scan_interval", "preferred_language", "metadata_country", "create_time", "update_time", "save_local_metadata", "auto_collections", "extract_trickplay", "extract_chapter_images", "analyze_loudness", "download_lyrics", "provider_order", "virtual_episodes", "hubs", "storage_classes", "effective_storage_classes", "image_url", "scan_progress")
    class StorageClassesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: StorageClass
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[StorageClass, str]] = ...) -> None: ...
    class EffectiveStorageClassesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: StorageClass
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[StorageClass, str]] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    PATHS_FIELD_NUMBER: _ClassVar[int]
    SCAN_INTERVAL_FIELD_NUMBER: _ClassVar[int]
    PREFERRED_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    METADATA_COUNTRY_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    SAVE_LOCAL_METADATA_FIELD_NUMBER: _ClassVar[int]
    AUTO_COLLECTIONS_FIELD_NUMBER: _ClassVar[int]
    EXTRACT_TRICKPLAY_FIELD_NUMBER: _ClassVar[int]
    EXTRACT_CHAPTER_IMAGES_FIELD_NUMBER: _ClassVar[int]
    ANALYZE_LOUDNESS_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_LYRICS_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_ORDER_FIELD_NUMBER: _ClassVar[int]
    VIRTUAL_EPISODES_FIELD_NUMBER: _ClassVar[int]
    HUBS_FIELD_NUMBER: _ClassVar[int]
    STORAGE_CLASSES_FIELD_NUMBER: _ClassVar[int]
    EFFECTIVE_STORAGE_CLASSES_FIELD_NUMBER: _ClassVar[int]
    IMAGE_URL_FIELD_NUMBER: _ClassVar[int]
    SCAN_PROGRESS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    kind: LibraryKind
    paths: _containers.RepeatedScalarFieldContainer[str]
    scan_interval: _duration_pb2.Duration
    preferred_language: str
    metadata_country: str
    create_time: _timestamp_pb2.Timestamp
    update_time: _timestamp_pb2.Timestamp
    save_local_metadata: bool
    auto_collections: bool
    extract_trickplay: bool
    extract_chapter_images: bool
    analyze_loudness: bool
    download_lyrics: bool
    provider_order: ProviderOrder
    virtual_episodes: VirtualEpisodes
    hubs: LibraryHubs
    storage_classes: _containers.ScalarMap[str, StorageClass]
    effective_storage_classes: _containers.ScalarMap[str, StorageClass]
    image_url: str
    scan_progress: ScanProgress
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., kind: _Optional[_Union[LibraryKind, str]] = ..., paths: _Optional[_Iterable[str]] = ..., scan_interval: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., preferred_language: _Optional[str] = ..., metadata_country: _Optional[str] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., save_local_metadata: _Optional[bool] = ..., auto_collections: _Optional[bool] = ..., extract_trickplay: _Optional[bool] = ..., extract_chapter_images: _Optional[bool] = ..., analyze_loudness: _Optional[bool] = ..., download_lyrics: _Optional[bool] = ..., provider_order: _Optional[_Union[ProviderOrder, _Mapping]] = ..., virtual_episodes: _Optional[_Union[VirtualEpisodes, _Mapping]] = ..., hubs: _Optional[_Union[LibraryHubs, _Mapping]] = ..., storage_classes: _Optional[_Mapping[str, StorageClass]] = ..., effective_storage_classes: _Optional[_Mapping[str, StorageClass]] = ..., image_url: _Optional[str] = ..., scan_progress: _Optional[_Union[ScanProgress, _Mapping]] = ...) -> None: ...

class ScanProgress(_message.Message):
    __slots__ = ("library_id", "phase", "folders_done", "folders_total", "items_done", "items_total", "fraction")
    LIBRARY_ID_FIELD_NUMBER: _ClassVar[int]
    PHASE_FIELD_NUMBER: _ClassVar[int]
    FOLDERS_DONE_FIELD_NUMBER: _ClassVar[int]
    FOLDERS_TOTAL_FIELD_NUMBER: _ClassVar[int]
    ITEMS_DONE_FIELD_NUMBER: _ClassVar[int]
    ITEMS_TOTAL_FIELD_NUMBER: _ClassVar[int]
    FRACTION_FIELD_NUMBER: _ClassVar[int]
    library_id: str
    phase: ScanPhase
    folders_done: int
    folders_total: int
    items_done: int
    items_total: int
    fraction: float
    def __init__(self, library_id: _Optional[str] = ..., phase: _Optional[_Union[ScanPhase, str]] = ..., folders_done: _Optional[int] = ..., folders_total: _Optional[int] = ..., items_done: _Optional[int] = ..., items_total: _Optional[int] = ..., fraction: _Optional[float] = ...) -> None: ...

class VirtualEpisodes(_message.Message):
    __slots__ = ("missing", "unaired", "specials", "grace_days")
    MISSING_FIELD_NUMBER: _ClassVar[int]
    UNAIRED_FIELD_NUMBER: _ClassVar[int]
    SPECIALS_FIELD_NUMBER: _ClassVar[int]
    GRACE_DAYS_FIELD_NUMBER: _ClassVar[int]
    missing: bool
    unaired: bool
    specials: bool
    grace_days: int
    def __init__(self, missing: _Optional[bool] = ..., unaired: _Optional[bool] = ..., specials: _Optional[bool] = ..., grace_days: _Optional[int] = ...) -> None: ...

class LibraryHubs(_message.Message):
    __slots__ = ("home", "recommended")
    HOME_FIELD_NUMBER: _ClassVar[int]
    RECOMMENDED_FIELD_NUMBER: _ClassVar[int]
    home: HubList
    recommended: HubList
    def __init__(self, home: _Optional[_Union[HubList, _Mapping]] = ..., recommended: _Optional[_Union[HubList, _Mapping]] = ...) -> None: ...

class HubList(_message.Message):
    __slots__ = ("kinds",)
    KINDS_FIELD_NUMBER: _ClassVar[int]
    kinds: _containers.RepeatedScalarFieldContainer[HubKind]
    def __init__(self, kinds: _Optional[_Iterable[_Union[HubKind, str]]] = ...) -> None: ...

class ProviderOrder(_message.Message):
    __slots__ = ("metadata", "images", "subtitles", "lyrics", "segments", "episodes")
    METADATA_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    SUBTITLES_FIELD_NUMBER: _ClassVar[int]
    LYRICS_FIELD_NUMBER: _ClassVar[int]
    SEGMENTS_FIELD_NUMBER: _ClassVar[int]
    EPISODES_FIELD_NUMBER: _ClassVar[int]
    metadata: ProviderList
    images: ProviderList
    subtitles: ProviderList
    lyrics: ProviderList
    segments: ProviderList
    episodes: ProviderList
    def __init__(self, metadata: _Optional[_Union[ProviderList, _Mapping]] = ..., images: _Optional[_Union[ProviderList, _Mapping]] = ..., subtitles: _Optional[_Union[ProviderList, _Mapping]] = ..., lyrics: _Optional[_Union[ProviderList, _Mapping]] = ..., segments: _Optional[_Union[ProviderList, _Mapping]] = ..., episodes: _Optional[_Union[ProviderList, _Mapping]] = ...) -> None: ...

class ProviderList(_message.Message):
    __slots__ = ("plugin_ids",)
    PLUGIN_IDS_FIELD_NUMBER: _ClassVar[int]
    plugin_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, plugin_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class LibrarySpec(_message.Message):
    __slots__ = ("name", "kind", "paths", "scan_interval", "preferred_language", "metadata_country", "save_local_metadata", "auto_collections", "extract_trickplay", "extract_chapter_images", "analyze_loudness", "download_lyrics", "provider_order", "virtual_episodes", "hubs", "storage_classes")
    class StorageClassesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: StorageClass
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[StorageClass, str]] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    PATHS_FIELD_NUMBER: _ClassVar[int]
    SCAN_INTERVAL_FIELD_NUMBER: _ClassVar[int]
    PREFERRED_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    METADATA_COUNTRY_FIELD_NUMBER: _ClassVar[int]
    SAVE_LOCAL_METADATA_FIELD_NUMBER: _ClassVar[int]
    AUTO_COLLECTIONS_FIELD_NUMBER: _ClassVar[int]
    EXTRACT_TRICKPLAY_FIELD_NUMBER: _ClassVar[int]
    EXTRACT_CHAPTER_IMAGES_FIELD_NUMBER: _ClassVar[int]
    ANALYZE_LOUDNESS_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_LYRICS_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_ORDER_FIELD_NUMBER: _ClassVar[int]
    VIRTUAL_EPISODES_FIELD_NUMBER: _ClassVar[int]
    HUBS_FIELD_NUMBER: _ClassVar[int]
    STORAGE_CLASSES_FIELD_NUMBER: _ClassVar[int]
    name: str
    kind: LibraryKind
    paths: _containers.RepeatedScalarFieldContainer[str]
    scan_interval: _duration_pb2.Duration
    preferred_language: str
    metadata_country: str
    save_local_metadata: bool
    auto_collections: bool
    extract_trickplay: bool
    extract_chapter_images: bool
    analyze_loudness: bool
    download_lyrics: bool
    provider_order: ProviderOrder
    virtual_episodes: VirtualEpisodes
    hubs: LibraryHubs
    storage_classes: _containers.ScalarMap[str, StorageClass]
    def __init__(self, name: _Optional[str] = ..., kind: _Optional[_Union[LibraryKind, str]] = ..., paths: _Optional[_Iterable[str]] = ..., scan_interval: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., preferred_language: _Optional[str] = ..., metadata_country: _Optional[str] = ..., save_local_metadata: _Optional[bool] = ..., auto_collections: _Optional[bool] = ..., extract_trickplay: _Optional[bool] = ..., extract_chapter_images: _Optional[bool] = ..., analyze_loudness: _Optional[bool] = ..., download_lyrics: _Optional[bool] = ..., provider_order: _Optional[_Union[ProviderOrder, _Mapping]] = ..., virtual_episodes: _Optional[_Union[VirtualEpisodes, _Mapping]] = ..., hubs: _Optional[_Union[LibraryHubs, _Mapping]] = ..., storage_classes: _Optional[_Mapping[str, StorageClass]] = ...) -> None: ...

class ListLibrariesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListLibrariesResponse(_message.Message):
    __slots__ = ("libraries",)
    LIBRARIES_FIELD_NUMBER: _ClassVar[int]
    libraries: _containers.RepeatedCompositeFieldContainer[Library]
    def __init__(self, libraries: _Optional[_Iterable[_Union[Library, _Mapping]]] = ...) -> None: ...

class GetLibraryRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class GetLibraryResponse(_message.Message):
    __slots__ = ("library",)
    LIBRARY_FIELD_NUMBER: _ClassVar[int]
    library: Library
    def __init__(self, library: _Optional[_Union[Library, _Mapping]] = ...) -> None: ...

class CreateLibraryRequest(_message.Message):
    __slots__ = ("spec",)
    SPEC_FIELD_NUMBER: _ClassVar[int]
    spec: LibrarySpec
    def __init__(self, spec: _Optional[_Union[LibrarySpec, _Mapping]] = ...) -> None: ...

class CreateLibraryResponse(_message.Message):
    __slots__ = ("library",)
    LIBRARY_FIELD_NUMBER: _ClassVar[int]
    library: Library
    def __init__(self, library: _Optional[_Union[Library, _Mapping]] = ...) -> None: ...

class UpdateLibraryRequest(_message.Message):
    __slots__ = ("id", "spec")
    ID_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    id: str
    spec: LibrarySpec
    def __init__(self, id: _Optional[str] = ..., spec: _Optional[_Union[LibrarySpec, _Mapping]] = ...) -> None: ...

class UpdateLibraryResponse(_message.Message):
    __slots__ = ("library",)
    LIBRARY_FIELD_NUMBER: _ClassVar[int]
    library: Library
    def __init__(self, library: _Optional[_Union[Library, _Mapping]] = ...) -> None: ...

class DeleteLibraryRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteLibraryResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ScanLibraryRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class ScanLibraryResponse(_message.Message):
    __slots__ = ("job_id", "enqueued")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    ENQUEUED_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    enqueued: bool
    def __init__(self, job_id: _Optional[str] = ..., enqueued: _Optional[bool] = ...) -> None: ...

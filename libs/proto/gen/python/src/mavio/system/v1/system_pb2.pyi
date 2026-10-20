import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.acquisition.v1 import wanted_pb2 as _wanted_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class HardwareAcceleration(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    HARDWARE_ACCELERATION_UNSPECIFIED: _ClassVar[HardwareAcceleration]
    HARDWARE_ACCELERATION_AUTO: _ClassVar[HardwareAcceleration]
    HARDWARE_ACCELERATION_NONE: _ClassVar[HardwareAcceleration]
    HARDWARE_ACCELERATION_VIDEOTOOLBOX: _ClassVar[HardwareAcceleration]

class ProperDownload(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PROPER_DOWNLOAD_UNSPECIFIED: _ClassVar[ProperDownload]
    PROPER_DOWNLOAD_PREFER_AND_UPGRADE: _ClassVar[ProperDownload]
    PROPER_DOWNLOAD_DO_NOT_UPGRADE: _ClassVar[ProperDownload]
    PROPER_DOWNLOAD_DO_NOT_PREFER: _ClassVar[ProperDownload]

class ColonReplacement(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    COLON_REPLACEMENT_UNSPECIFIED: _ClassVar[ColonReplacement]
    COLON_REPLACEMENT_DELETE: _ClassVar[ColonReplacement]
    COLON_REPLACEMENT_DASH: _ClassVar[ColonReplacement]
    COLON_REPLACEMENT_SPACE_DASH: _ClassVar[ColonReplacement]
    COLON_REPLACEMENT_SPACE_DASH_SPACE: _ClassVar[ColonReplacement]
    COLON_REPLACEMENT_SMART: _ClassVar[ColonReplacement]

class MultiEpisodeStyle(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MULTI_EPISODE_STYLE_UNSPECIFIED: _ClassVar[MultiEpisodeStyle]
    MULTI_EPISODE_STYLE_EXTEND: _ClassVar[MultiEpisodeStyle]
    MULTI_EPISODE_STYLE_DUPLICATE: _ClassVar[MultiEpisodeStyle]
    MULTI_EPISODE_STYLE_REPEAT: _ClassVar[MultiEpisodeStyle]
    MULTI_EPISODE_STYLE_SCENE: _ClassVar[MultiEpisodeStyle]
    MULTI_EPISODE_STYLE_RANGE: _ClassVar[MultiEpisodeStyle]
    MULTI_EPISODE_STYLE_PREFIXED_RANGE: _ClassVar[MultiEpisodeStyle]

class EpisodeTitleRequired(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    EPISODE_TITLE_REQUIRED_UNSPECIFIED: _ClassVar[EpisodeTitleRequired]
    EPISODE_TITLE_REQUIRED_ALWAYS: _ClassVar[EpisodeTitleRequired]
    EPISODE_TITLE_REQUIRED_BULK_SEASON_RELEASES: _ClassVar[EpisodeTitleRequired]
    EPISODE_TITLE_REQUIRED_NEVER: _ClassVar[EpisodeTitleRequired]

class LogLevel(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    LOG_LEVEL_UNSPECIFIED: _ClassVar[LogLevel]
    LOG_LEVEL_DEBUG: _ClassVar[LogLevel]
    LOG_LEVEL_INFO: _ClassVar[LogLevel]
    LOG_LEVEL_WARN: _ClassVar[LogLevel]
    LOG_LEVEL_ERROR: _ClassVar[LogLevel]
HARDWARE_ACCELERATION_UNSPECIFIED: HardwareAcceleration
HARDWARE_ACCELERATION_AUTO: HardwareAcceleration
HARDWARE_ACCELERATION_NONE: HardwareAcceleration
HARDWARE_ACCELERATION_VIDEOTOOLBOX: HardwareAcceleration
PROPER_DOWNLOAD_UNSPECIFIED: ProperDownload
PROPER_DOWNLOAD_PREFER_AND_UPGRADE: ProperDownload
PROPER_DOWNLOAD_DO_NOT_UPGRADE: ProperDownload
PROPER_DOWNLOAD_DO_NOT_PREFER: ProperDownload
COLON_REPLACEMENT_UNSPECIFIED: ColonReplacement
COLON_REPLACEMENT_DELETE: ColonReplacement
COLON_REPLACEMENT_DASH: ColonReplacement
COLON_REPLACEMENT_SPACE_DASH: ColonReplacement
COLON_REPLACEMENT_SPACE_DASH_SPACE: ColonReplacement
COLON_REPLACEMENT_SMART: ColonReplacement
MULTI_EPISODE_STYLE_UNSPECIFIED: MultiEpisodeStyle
MULTI_EPISODE_STYLE_EXTEND: MultiEpisodeStyle
MULTI_EPISODE_STYLE_DUPLICATE: MultiEpisodeStyle
MULTI_EPISODE_STYLE_REPEAT: MultiEpisodeStyle
MULTI_EPISODE_STYLE_SCENE: MultiEpisodeStyle
MULTI_EPISODE_STYLE_RANGE: MultiEpisodeStyle
MULTI_EPISODE_STYLE_PREFIXED_RANGE: MultiEpisodeStyle
EPISODE_TITLE_REQUIRED_UNSPECIFIED: EpisodeTitleRequired
EPISODE_TITLE_REQUIRED_ALWAYS: EpisodeTitleRequired
EPISODE_TITLE_REQUIRED_BULK_SEASON_RELEASES: EpisodeTitleRequired
EPISODE_TITLE_REQUIRED_NEVER: EpisodeTitleRequired
LOG_LEVEL_UNSPECIFIED: LogLevel
LOG_LEVEL_DEBUG: LogLevel
LOG_LEVEL_INFO: LogLevel
LOG_LEVEL_WARN: LogLevel
LOG_LEVEL_ERROR: LogLevel

class GetHealthRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetHealthResponse(_message.Message):
    __slots__ = ("status", "version")
    class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        STATUS_UNSPECIFIED: _ClassVar[GetHealthResponse.Status]
        STATUS_SERVING: _ClassVar[GetHealthResponse.Status]
        STATUS_NOT_SERVING: _ClassVar[GetHealthResponse.Status]
    STATUS_UNSPECIFIED: GetHealthResponse.Status
    STATUS_SERVING: GetHealthResponse.Status
    STATUS_NOT_SERVING: GetHealthResponse.Status
    STATUS_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    status: GetHealthResponse.Status
    version: str
    def __init__(self, status: _Optional[_Union[GetHealthResponse.Status, str]] = ..., version: _Optional[str] = ...) -> None: ...

class GetSystemInfoRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetSystemInfoResponse(_message.Message):
    __slots__ = ("version", "os", "arch", "database", "ffmpeg_version", "start_time")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    OS_FIELD_NUMBER: _ClassVar[int]
    ARCH_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    FFMPEG_VERSION_FIELD_NUMBER: _ClassVar[int]
    START_TIME_FIELD_NUMBER: _ClassVar[int]
    version: str
    os: str
    arch: str
    database: str
    ffmpeg_version: str
    start_time: _timestamp_pb2.Timestamp
    def __init__(self, version: _Optional[str] = ..., os: _Optional[str] = ..., arch: _Optional[str] = ..., database: _Optional[str] = ..., ffmpeg_version: _Optional[str] = ..., start_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetSystemLoadRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetSystemLoadResponse(_message.Message):
    __slots__ = ("samples",)
    SAMPLES_FIELD_NUMBER: _ClassVar[int]
    samples: _containers.RepeatedCompositeFieldContainer[LoadSample]
    def __init__(self, samples: _Optional[_Iterable[_Union[LoadSample, _Mapping]]] = ...) -> None: ...

class LoadSample(_message.Message):
    __slots__ = ("time", "cpu", "memory", "disk", "network")
    TIME_FIELD_NUMBER: _ClassVar[int]
    CPU_FIELD_NUMBER: _ClassVar[int]
    MEMORY_FIELD_NUMBER: _ClassVar[int]
    DISK_FIELD_NUMBER: _ClassVar[int]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    time: _timestamp_pb2.Timestamp
    cpu: CpuLoad
    memory: MemoryLoad
    disk: DiskLoad
    network: NetworkLoad
    def __init__(self, time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., cpu: _Optional[_Union[CpuLoad, _Mapping]] = ..., memory: _Optional[_Union[MemoryLoad, _Mapping]] = ..., disk: _Optional[_Union[DiskLoad, _Mapping]] = ..., network: _Optional[_Union[NetworkLoad, _Mapping]] = ...) -> None: ...

class CpuLoad(_message.Message):
    __slots__ = ("user", "system", "server")
    USER_FIELD_NUMBER: _ClassVar[int]
    SYSTEM_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    user: float
    system: float
    server: float
    def __init__(self, user: _Optional[float] = ..., system: _Optional[float] = ..., server: _Optional[float] = ...) -> None: ...

class MemoryLoad(_message.Message):
    __slots__ = ("total_bytes", "used_bytes", "swap_total_bytes", "swap_used_bytes", "server_bytes")
    TOTAL_BYTES_FIELD_NUMBER: _ClassVar[int]
    USED_BYTES_FIELD_NUMBER: _ClassVar[int]
    SWAP_TOTAL_BYTES_FIELD_NUMBER: _ClassVar[int]
    SWAP_USED_BYTES_FIELD_NUMBER: _ClassVar[int]
    SERVER_BYTES_FIELD_NUMBER: _ClassVar[int]
    total_bytes: int
    used_bytes: int
    swap_total_bytes: int
    swap_used_bytes: int
    server_bytes: int
    def __init__(self, total_bytes: _Optional[int] = ..., used_bytes: _Optional[int] = ..., swap_total_bytes: _Optional[int] = ..., swap_used_bytes: _Optional[int] = ..., server_bytes: _Optional[int] = ...) -> None: ...

class DiskLoad(_message.Message):
    __slots__ = ("read_rate", "write_rate", "read_bytes", "written_bytes", "total_bytes", "available_bytes")
    READ_RATE_FIELD_NUMBER: _ClassVar[int]
    WRITE_RATE_FIELD_NUMBER: _ClassVar[int]
    READ_BYTES_FIELD_NUMBER: _ClassVar[int]
    WRITTEN_BYTES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_BYTES_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_BYTES_FIELD_NUMBER: _ClassVar[int]
    read_rate: float
    write_rate: float
    read_bytes: int
    written_bytes: int
    total_bytes: int
    available_bytes: int
    def __init__(self, read_rate: _Optional[float] = ..., write_rate: _Optional[float] = ..., read_bytes: _Optional[int] = ..., written_bytes: _Optional[int] = ..., total_bytes: _Optional[int] = ..., available_bytes: _Optional[int] = ...) -> None: ...

class NetworkLoad(_message.Message):
    __slots__ = ("receive_rate", "send_rate", "received_bytes", "sent_bytes")
    RECEIVE_RATE_FIELD_NUMBER: _ClassVar[int]
    SEND_RATE_FIELD_NUMBER: _ClassVar[int]
    RECEIVED_BYTES_FIELD_NUMBER: _ClassVar[int]
    SENT_BYTES_FIELD_NUMBER: _ClassVar[int]
    receive_rate: float
    send_rate: float
    received_bytes: int
    sent_bytes: int
    def __init__(self, receive_rate: _Optional[float] = ..., send_rate: _Optional[float] = ..., received_bytes: _Optional[int] = ..., sent_bytes: _Optional[int] = ...) -> None: ...

class TranscodingSettings(_message.Message):
    __slots__ = ("hardware_acceleration", "hardware_encoding", "encoder_preset", "h264_crf", "h265_crf", "threads", "tonemap_algorithm", "tonemap_range", "tonemap_desat", "tonemap_peak", "deinterlace_method", "deinterlace_double_rate", "downmix_boost", "crop_black_borders", "transcode_dir")
    HARDWARE_ACCELERATION_FIELD_NUMBER: _ClassVar[int]
    HARDWARE_ENCODING_FIELD_NUMBER: _ClassVar[int]
    ENCODER_PRESET_FIELD_NUMBER: _ClassVar[int]
    H264_CRF_FIELD_NUMBER: _ClassVar[int]
    H265_CRF_FIELD_NUMBER: _ClassVar[int]
    THREADS_FIELD_NUMBER: _ClassVar[int]
    TONEMAP_ALGORITHM_FIELD_NUMBER: _ClassVar[int]
    TONEMAP_RANGE_FIELD_NUMBER: _ClassVar[int]
    TONEMAP_DESAT_FIELD_NUMBER: _ClassVar[int]
    TONEMAP_PEAK_FIELD_NUMBER: _ClassVar[int]
    DEINTERLACE_METHOD_FIELD_NUMBER: _ClassVar[int]
    DEINTERLACE_DOUBLE_RATE_FIELD_NUMBER: _ClassVar[int]
    DOWNMIX_BOOST_FIELD_NUMBER: _ClassVar[int]
    CROP_BLACK_BORDERS_FIELD_NUMBER: _ClassVar[int]
    TRANSCODE_DIR_FIELD_NUMBER: _ClassVar[int]
    hardware_acceleration: HardwareAcceleration
    hardware_encoding: bool
    encoder_preset: str
    h264_crf: int
    h265_crf: int
    threads: int
    tonemap_algorithm: str
    tonemap_range: str
    tonemap_desat: float
    tonemap_peak: float
    deinterlace_method: str
    deinterlace_double_rate: bool
    downmix_boost: float
    crop_black_borders: bool
    transcode_dir: str
    def __init__(self, hardware_acceleration: _Optional[_Union[HardwareAcceleration, str]] = ..., hardware_encoding: _Optional[bool] = ..., encoder_preset: _Optional[str] = ..., h264_crf: _Optional[int] = ..., h265_crf: _Optional[int] = ..., threads: _Optional[int] = ..., tonemap_algorithm: _Optional[str] = ..., tonemap_range: _Optional[str] = ..., tonemap_desat: _Optional[float] = ..., tonemap_peak: _Optional[float] = ..., deinterlace_method: _Optional[str] = ..., deinterlace_double_rate: _Optional[bool] = ..., downmix_boost: _Optional[float] = ..., crop_black_borders: _Optional[bool] = ..., transcode_dir: _Optional[str] = ...) -> None: ...

class NetworkSettings(_message.Message):
    __slots__ = ("server_name", "base_url", "https_port", "certificate_path", "key_path", "local_discovery", "relay", "proxies")
    SERVER_NAME_FIELD_NUMBER: _ClassVar[int]
    BASE_URL_FIELD_NUMBER: _ClassVar[int]
    HTTPS_PORT_FIELD_NUMBER: _ClassVar[int]
    CERTIFICATE_PATH_FIELD_NUMBER: _ClassVar[int]
    KEY_PATH_FIELD_NUMBER: _ClassVar[int]
    LOCAL_DISCOVERY_FIELD_NUMBER: _ClassVar[int]
    RELAY_FIELD_NUMBER: _ClassVar[int]
    PROXIES_FIELD_NUMBER: _ClassVar[int]
    server_name: str
    base_url: str
    https_port: int
    certificate_path: str
    key_path: str
    local_discovery: bool
    relay: bool
    proxies: OutboundProxies
    def __init__(self, server_name: _Optional[str] = ..., base_url: _Optional[str] = ..., https_port: _Optional[int] = ..., certificate_path: _Optional[str] = ..., key_path: _Optional[str] = ..., local_discovery: _Optional[bool] = ..., relay: _Optional[bool] = ..., proxies: _Optional[_Union[OutboundProxies, _Mapping]] = ...) -> None: ...

class OutboundProxies(_message.Message):
    __slots__ = ("plugin_catalogs", "images", "mavio_dev", "acme")
    PLUGIN_CATALOGS_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    MAVIO_DEV_FIELD_NUMBER: _ClassVar[int]
    ACME_FIELD_NUMBER: _ClassVar[int]
    plugin_catalogs: str
    images: str
    mavio_dev: str
    acme: str
    def __init__(self, plugin_catalogs: _Optional[str] = ..., images: _Optional[str] = ..., mavio_dev: _Optional[str] = ..., acme: _Optional[str] = ...) -> None: ...

class ServerSettings(_message.Message):
    __slots__ = ("transcoding", "network", "plugin_catalogs", "update_time", "password_reset_plugin", "acquisition", "requests", "metadata", "session_idle_days")
    TRANSCODING_FIELD_NUMBER: _ClassVar[int]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_CATALOGS_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_RESET_PLUGIN_FIELD_NUMBER: _ClassVar[int]
    ACQUISITION_FIELD_NUMBER: _ClassVar[int]
    REQUESTS_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SESSION_IDLE_DAYS_FIELD_NUMBER: _ClassVar[int]
    transcoding: TranscodingSettings
    network: NetworkSettings
    plugin_catalogs: _containers.RepeatedScalarFieldContainer[str]
    update_time: _timestamp_pb2.Timestamp
    password_reset_plugin: str
    acquisition: AcquisitionSettings
    requests: RequestSettings
    metadata: MetadataSettings
    session_idle_days: int
    def __init__(self, transcoding: _Optional[_Union[TranscodingSettings, _Mapping]] = ..., network: _Optional[_Union[NetworkSettings, _Mapping]] = ..., plugin_catalogs: _Optional[_Iterable[str]] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., password_reset_plugin: _Optional[str] = ..., acquisition: _Optional[_Union[AcquisitionSettings, _Mapping]] = ..., requests: _Optional[_Union[RequestSettings, _Mapping]] = ..., metadata: _Optional[_Union[MetadataSettings, _Mapping]] = ..., session_idle_days: _Optional[int] = ...) -> None: ...

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

class MetadataSettings(_message.Message):
    __slots__ = ("preferred_language", "country")
    PREFERRED_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    COUNTRY_FIELD_NUMBER: _ClassVar[int]
    preferred_language: str
    country: str
    def __init__(self, preferred_language: _Optional[str] = ..., country: _Optional[str] = ...) -> None: ...

class AcquisitionSettings(_message.Message):
    __slots__ = ("propers", "prefer_indexer_flags", "completed_download_handling", "auto_redownload_failed", "maximum_size", "minimum_age", "retention", "allow_hardcoded_subs", "whitelisted_hardcoded_subs", "skip_free_space_check", "minimum_free_space", "availability_delay", "rss_sync_interval", "check_finished_interval", "recycle_bin", "recycle_bin_cleanup_days", "copy_using_hardlinks", "import_extra_files", "extra_file_extensions", "delete_empty_folders", "rename_movies", "replace_illegal_characters", "colon_replacement", "movie_folder_format", "standard_movie_format", "auto_redownload_failed_from_interactive_search", "rename_episodes", "standard_episode_format", "daily_episode_format", "anime_episode_format", "series_folder_format", "season_folder_format", "specials_folder_format", "multi_episode_style", "auto_unmonitor_previously_downloaded_episodes", "episode_title_required", "download_client_working_folders")
    PROPERS_FIELD_NUMBER: _ClassVar[int]
    PREFER_INDEXER_FLAGS_FIELD_NUMBER: _ClassVar[int]
    COMPLETED_DOWNLOAD_HANDLING_FIELD_NUMBER: _ClassVar[int]
    AUTO_REDOWNLOAD_FAILED_FIELD_NUMBER: _ClassVar[int]
    MAXIMUM_SIZE_FIELD_NUMBER: _ClassVar[int]
    MINIMUM_AGE_FIELD_NUMBER: _ClassVar[int]
    RETENTION_FIELD_NUMBER: _ClassVar[int]
    ALLOW_HARDCODED_SUBS_FIELD_NUMBER: _ClassVar[int]
    WHITELISTED_HARDCODED_SUBS_FIELD_NUMBER: _ClassVar[int]
    SKIP_FREE_SPACE_CHECK_FIELD_NUMBER: _ClassVar[int]
    MINIMUM_FREE_SPACE_FIELD_NUMBER: _ClassVar[int]
    AVAILABILITY_DELAY_FIELD_NUMBER: _ClassVar[int]
    RSS_SYNC_INTERVAL_FIELD_NUMBER: _ClassVar[int]
    CHECK_FINISHED_INTERVAL_FIELD_NUMBER: _ClassVar[int]
    RECYCLE_BIN_FIELD_NUMBER: _ClassVar[int]
    RECYCLE_BIN_CLEANUP_DAYS_FIELD_NUMBER: _ClassVar[int]
    COPY_USING_HARDLINKS_FIELD_NUMBER: _ClassVar[int]
    IMPORT_EXTRA_FILES_FIELD_NUMBER: _ClassVar[int]
    EXTRA_FILE_EXTENSIONS_FIELD_NUMBER: _ClassVar[int]
    DELETE_EMPTY_FOLDERS_FIELD_NUMBER: _ClassVar[int]
    RENAME_MOVIES_FIELD_NUMBER: _ClassVar[int]
    REPLACE_ILLEGAL_CHARACTERS_FIELD_NUMBER: _ClassVar[int]
    COLON_REPLACEMENT_FIELD_NUMBER: _ClassVar[int]
    MOVIE_FOLDER_FORMAT_FIELD_NUMBER: _ClassVar[int]
    STANDARD_MOVIE_FORMAT_FIELD_NUMBER: _ClassVar[int]
    AUTO_REDOWNLOAD_FAILED_FROM_INTERACTIVE_SEARCH_FIELD_NUMBER: _ClassVar[int]
    RENAME_EPISODES_FIELD_NUMBER: _ClassVar[int]
    STANDARD_EPISODE_FORMAT_FIELD_NUMBER: _ClassVar[int]
    DAILY_EPISODE_FORMAT_FIELD_NUMBER: _ClassVar[int]
    ANIME_EPISODE_FORMAT_FIELD_NUMBER: _ClassVar[int]
    SERIES_FOLDER_FORMAT_FIELD_NUMBER: _ClassVar[int]
    SEASON_FOLDER_FORMAT_FIELD_NUMBER: _ClassVar[int]
    SPECIALS_FOLDER_FORMAT_FIELD_NUMBER: _ClassVar[int]
    MULTI_EPISODE_STYLE_FIELD_NUMBER: _ClassVar[int]
    AUTO_UNMONITOR_PREVIOUSLY_DOWNLOADED_EPISODES_FIELD_NUMBER: _ClassVar[int]
    EPISODE_TITLE_REQUIRED_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_CLIENT_WORKING_FOLDERS_FIELD_NUMBER: _ClassVar[int]
    propers: ProperDownload
    prefer_indexer_flags: bool
    completed_download_handling: bool
    auto_redownload_failed: bool
    maximum_size: int
    minimum_age: int
    retention: int
    allow_hardcoded_subs: bool
    whitelisted_hardcoded_subs: _containers.RepeatedScalarFieldContainer[str]
    skip_free_space_check: bool
    minimum_free_space: int
    availability_delay: int
    rss_sync_interval: int
    check_finished_interval: int
    recycle_bin: str
    recycle_bin_cleanup_days: int
    copy_using_hardlinks: bool
    import_extra_files: bool
    extra_file_extensions: _containers.RepeatedScalarFieldContainer[str]
    delete_empty_folders: bool
    rename_movies: bool
    replace_illegal_characters: bool
    colon_replacement: ColonReplacement
    movie_folder_format: str
    standard_movie_format: str
    auto_redownload_failed_from_interactive_search: bool
    rename_episodes: bool
    standard_episode_format: str
    daily_episode_format: str
    anime_episode_format: str
    series_folder_format: str
    season_folder_format: str
    specials_folder_format: str
    multi_episode_style: MultiEpisodeStyle
    auto_unmonitor_previously_downloaded_episodes: bool
    episode_title_required: EpisodeTitleRequired
    download_client_working_folders: str
    def __init__(self, propers: _Optional[_Union[ProperDownload, str]] = ..., prefer_indexer_flags: _Optional[bool] = ..., completed_download_handling: _Optional[bool] = ..., auto_redownload_failed: _Optional[bool] = ..., maximum_size: _Optional[int] = ..., minimum_age: _Optional[int] = ..., retention: _Optional[int] = ..., allow_hardcoded_subs: _Optional[bool] = ..., whitelisted_hardcoded_subs: _Optional[_Iterable[str]] = ..., skip_free_space_check: _Optional[bool] = ..., minimum_free_space: _Optional[int] = ..., availability_delay: _Optional[int] = ..., rss_sync_interval: _Optional[int] = ..., check_finished_interval: _Optional[int] = ..., recycle_bin: _Optional[str] = ..., recycle_bin_cleanup_days: _Optional[int] = ..., copy_using_hardlinks: _Optional[bool] = ..., import_extra_files: _Optional[bool] = ..., extra_file_extensions: _Optional[_Iterable[str]] = ..., delete_empty_folders: _Optional[bool] = ..., rename_movies: _Optional[bool] = ..., replace_illegal_characters: _Optional[bool] = ..., colon_replacement: _Optional[_Union[ColonReplacement, str]] = ..., movie_folder_format: _Optional[str] = ..., standard_movie_format: _Optional[str] = ..., auto_redownload_failed_from_interactive_search: _Optional[bool] = ..., rename_episodes: _Optional[bool] = ..., standard_episode_format: _Optional[str] = ..., daily_episode_format: _Optional[str] = ..., anime_episode_format: _Optional[str] = ..., series_folder_format: _Optional[str] = ..., season_folder_format: _Optional[str] = ..., specials_folder_format: _Optional[str] = ..., multi_episode_style: _Optional[_Union[MultiEpisodeStyle, str]] = ..., auto_unmonitor_previously_downloaded_episodes: _Optional[bool] = ..., episode_title_required: _Optional[_Union[EpisodeTitleRequired, str]] = ..., download_client_working_folders: _Optional[str] = ...) -> None: ...

class GetServerSettingsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetServerSettingsResponse(_message.Message):
    __slots__ = ("settings", "available_hardware_accelerations")
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_HARDWARE_ACCELERATIONS_FIELD_NUMBER: _ClassVar[int]
    settings: ServerSettings
    available_hardware_accelerations: _containers.RepeatedScalarFieldContainer[HardwareAcceleration]
    def __init__(self, settings: _Optional[_Union[ServerSettings, _Mapping]] = ..., available_hardware_accelerations: _Optional[_Iterable[_Union[HardwareAcceleration, str]]] = ...) -> None: ...

class UpdateServerSettingsRequest(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: ServerSettings
    def __init__(self, settings: _Optional[_Union[ServerSettings, _Mapping]] = ...) -> None: ...

class UpdateServerSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: ServerSettings
    def __init__(self, settings: _Optional[_Union[ServerSettings, _Mapping]] = ...) -> None: ...

class ListDirectoryRequest(_message.Message):
    __slots__ = ("path", "include_files")
    PATH_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_FILES_FIELD_NUMBER: _ClassVar[int]
    path: str
    include_files: bool
    def __init__(self, path: _Optional[str] = ..., include_files: _Optional[bool] = ...) -> None: ...

class DirectoryEntry(_message.Message):
    __slots__ = ("name", "path", "is_dir")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    IS_DIR_FIELD_NUMBER: _ClassVar[int]
    name: str
    path: str
    is_dir: bool
    def __init__(self, name: _Optional[str] = ..., path: _Optional[str] = ..., is_dir: _Optional[bool] = ...) -> None: ...

class ListDirectoryResponse(_message.Message):
    __slots__ = ("path", "parent", "entries")
    PATH_FIELD_NUMBER: _ClassVar[int]
    PARENT_FIELD_NUMBER: _ClassVar[int]
    ENTRIES_FIELD_NUMBER: _ClassVar[int]
    path: str
    parent: str
    entries: _containers.RepeatedCompositeFieldContainer[DirectoryEntry]
    def __init__(self, path: _Optional[str] = ..., parent: _Optional[str] = ..., entries: _Optional[_Iterable[_Union[DirectoryEntry, _Mapping]]] = ...) -> None: ...

class LogRecord(_message.Message):
    __slots__ = ("time", "level", "message", "attributes")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    TIME_FIELD_NUMBER: _ClassVar[int]
    LEVEL_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    time: _timestamp_pb2.Timestamp
    level: LogLevel
    message: str
    attributes: _containers.ScalarMap[str, str]
    def __init__(self, time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., level: _Optional[_Union[LogLevel, str]] = ..., message: _Optional[str] = ..., attributes: _Optional[_Mapping[str, str]] = ...) -> None: ...

class ListLogsRequest(_message.Message):
    __slots__ = ("min_level", "max_age", "limit")
    MIN_LEVEL_FIELD_NUMBER: _ClassVar[int]
    MAX_AGE_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    min_level: LogLevel
    max_age: _duration_pb2.Duration
    limit: int
    def __init__(self, min_level: _Optional[_Union[LogLevel, str]] = ..., max_age: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., limit: _Optional[int] = ...) -> None: ...

class ListLogsResponse(_message.Message):
    __slots__ = ("records",)
    RECORDS_FIELD_NUMBER: _ClassVar[int]
    records: _containers.RepeatedCompositeFieldContainer[LogRecord]
    def __init__(self, records: _Optional[_Iterable[_Union[LogRecord, _Mapping]]] = ...) -> None: ...

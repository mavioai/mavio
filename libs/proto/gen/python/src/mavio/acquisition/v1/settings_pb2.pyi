from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

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

class AcquisitionSettings(_message.Message):
    __slots__ = ("naming", "completed_download_handling", "copy_using_hardlinks", "import_extra_files", "extra_file_extensions", "delete_empty_folders", "skip_free_space_check", "minimum_free_space", "episode_title_required", "auto_unmonitor_previously_downloaded_episodes", "download_client_working_folders", "check_finished_interval", "auto_redownload_failed", "auto_redownload_failed_from_interactive_search", "propers", "prefer_indexer_flags", "maximum_size", "minimum_age", "retention", "rss_sync_interval", "availability_delay", "allow_hardcoded_subs", "whitelisted_hardcoded_subs", "recycle_bin", "recycle_bin_cleanup_days")
    NAMING_FIELD_NUMBER: _ClassVar[int]
    COMPLETED_DOWNLOAD_HANDLING_FIELD_NUMBER: _ClassVar[int]
    COPY_USING_HARDLINKS_FIELD_NUMBER: _ClassVar[int]
    IMPORT_EXTRA_FILES_FIELD_NUMBER: _ClassVar[int]
    EXTRA_FILE_EXTENSIONS_FIELD_NUMBER: _ClassVar[int]
    DELETE_EMPTY_FOLDERS_FIELD_NUMBER: _ClassVar[int]
    SKIP_FREE_SPACE_CHECK_FIELD_NUMBER: _ClassVar[int]
    MINIMUM_FREE_SPACE_FIELD_NUMBER: _ClassVar[int]
    EPISODE_TITLE_REQUIRED_FIELD_NUMBER: _ClassVar[int]
    AUTO_UNMONITOR_PREVIOUSLY_DOWNLOADED_EPISODES_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_CLIENT_WORKING_FOLDERS_FIELD_NUMBER: _ClassVar[int]
    CHECK_FINISHED_INTERVAL_FIELD_NUMBER: _ClassVar[int]
    AUTO_REDOWNLOAD_FAILED_FIELD_NUMBER: _ClassVar[int]
    AUTO_REDOWNLOAD_FAILED_FROM_INTERACTIVE_SEARCH_FIELD_NUMBER: _ClassVar[int]
    PROPERS_FIELD_NUMBER: _ClassVar[int]
    PREFER_INDEXER_FLAGS_FIELD_NUMBER: _ClassVar[int]
    MAXIMUM_SIZE_FIELD_NUMBER: _ClassVar[int]
    MINIMUM_AGE_FIELD_NUMBER: _ClassVar[int]
    RETENTION_FIELD_NUMBER: _ClassVar[int]
    RSS_SYNC_INTERVAL_FIELD_NUMBER: _ClassVar[int]
    AVAILABILITY_DELAY_FIELD_NUMBER: _ClassVar[int]
    ALLOW_HARDCODED_SUBS_FIELD_NUMBER: _ClassVar[int]
    WHITELISTED_HARDCODED_SUBS_FIELD_NUMBER: _ClassVar[int]
    RECYCLE_BIN_FIELD_NUMBER: _ClassVar[int]
    RECYCLE_BIN_CLEANUP_DAYS_FIELD_NUMBER: _ClassVar[int]
    naming: NamingSettings
    completed_download_handling: bool
    copy_using_hardlinks: bool
    import_extra_files: bool
    extra_file_extensions: _containers.RepeatedScalarFieldContainer[str]
    delete_empty_folders: bool
    skip_free_space_check: bool
    minimum_free_space: int
    episode_title_required: EpisodeTitleRequired
    auto_unmonitor_previously_downloaded_episodes: bool
    download_client_working_folders: str
    check_finished_interval: int
    auto_redownload_failed: bool
    auto_redownload_failed_from_interactive_search: bool
    propers: ProperDownload
    prefer_indexer_flags: bool
    maximum_size: int
    minimum_age: int
    retention: int
    rss_sync_interval: int
    availability_delay: int
    allow_hardcoded_subs: bool
    whitelisted_hardcoded_subs: _containers.RepeatedScalarFieldContainer[str]
    recycle_bin: str
    recycle_bin_cleanup_days: int
    def __init__(self, naming: _Optional[_Union[NamingSettings, _Mapping]] = ..., completed_download_handling: _Optional[bool] = ..., copy_using_hardlinks: _Optional[bool] = ..., import_extra_files: _Optional[bool] = ..., extra_file_extensions: _Optional[_Iterable[str]] = ..., delete_empty_folders: _Optional[bool] = ..., skip_free_space_check: _Optional[bool] = ..., minimum_free_space: _Optional[int] = ..., episode_title_required: _Optional[_Union[EpisodeTitleRequired, str]] = ..., auto_unmonitor_previously_downloaded_episodes: _Optional[bool] = ..., download_client_working_folders: _Optional[str] = ..., check_finished_interval: _Optional[int] = ..., auto_redownload_failed: _Optional[bool] = ..., auto_redownload_failed_from_interactive_search: _Optional[bool] = ..., propers: _Optional[_Union[ProperDownload, str]] = ..., prefer_indexer_flags: _Optional[bool] = ..., maximum_size: _Optional[int] = ..., minimum_age: _Optional[int] = ..., retention: _Optional[int] = ..., rss_sync_interval: _Optional[int] = ..., availability_delay: _Optional[int] = ..., allow_hardcoded_subs: _Optional[bool] = ..., whitelisted_hardcoded_subs: _Optional[_Iterable[str]] = ..., recycle_bin: _Optional[str] = ..., recycle_bin_cleanup_days: _Optional[int] = ...) -> None: ...

class NamingSettings(_message.Message):
    __slots__ = ("rename_movies", "rename_episodes", "replace_illegal_characters", "colon_replacement", "movie_folder_format", "standard_movie_format", "standard_episode_format", "daily_episode_format", "anime_episode_format", "series_folder_format", "season_folder_format", "specials_folder_format", "multi_episode_style")
    RENAME_MOVIES_FIELD_NUMBER: _ClassVar[int]
    RENAME_EPISODES_FIELD_NUMBER: _ClassVar[int]
    REPLACE_ILLEGAL_CHARACTERS_FIELD_NUMBER: _ClassVar[int]
    COLON_REPLACEMENT_FIELD_NUMBER: _ClassVar[int]
    MOVIE_FOLDER_FORMAT_FIELD_NUMBER: _ClassVar[int]
    STANDARD_MOVIE_FORMAT_FIELD_NUMBER: _ClassVar[int]
    STANDARD_EPISODE_FORMAT_FIELD_NUMBER: _ClassVar[int]
    DAILY_EPISODE_FORMAT_FIELD_NUMBER: _ClassVar[int]
    ANIME_EPISODE_FORMAT_FIELD_NUMBER: _ClassVar[int]
    SERIES_FOLDER_FORMAT_FIELD_NUMBER: _ClassVar[int]
    SEASON_FOLDER_FORMAT_FIELD_NUMBER: _ClassVar[int]
    SPECIALS_FOLDER_FORMAT_FIELD_NUMBER: _ClassVar[int]
    MULTI_EPISODE_STYLE_FIELD_NUMBER: _ClassVar[int]
    rename_movies: bool
    rename_episodes: bool
    replace_illegal_characters: bool
    colon_replacement: ColonReplacement
    movie_folder_format: str
    standard_movie_format: str
    standard_episode_format: str
    daily_episode_format: str
    anime_episode_format: str
    series_folder_format: str
    season_folder_format: str
    specials_folder_format: str
    multi_episode_style: MultiEpisodeStyle
    def __init__(self, rename_movies: _Optional[bool] = ..., rename_episodes: _Optional[bool] = ..., replace_illegal_characters: _Optional[bool] = ..., colon_replacement: _Optional[_Union[ColonReplacement, str]] = ..., movie_folder_format: _Optional[str] = ..., standard_movie_format: _Optional[str] = ..., standard_episode_format: _Optional[str] = ..., daily_episode_format: _Optional[str] = ..., anime_episode_format: _Optional[str] = ..., series_folder_format: _Optional[str] = ..., season_folder_format: _Optional[str] = ..., specials_folder_format: _Optional[str] = ..., multi_episode_style: _Optional[_Union[MultiEpisodeStyle, str]] = ...) -> None: ...

class GetAcquisitionSettingsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetAcquisitionSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: AcquisitionSettings
    def __init__(self, settings: _Optional[_Union[AcquisitionSettings, _Mapping]] = ...) -> None: ...

class UpdateAcquisitionSettingsRequest(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: AcquisitionSettings
    def __init__(self, settings: _Optional[_Union[AcquisitionSettings, _Mapping]] = ...) -> None: ...

class UpdateAcquisitionSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: AcquisitionSettings
    def __init__(self, settings: _Optional[_Union[AcquisitionSettings, _Mapping]] = ...) -> None: ...

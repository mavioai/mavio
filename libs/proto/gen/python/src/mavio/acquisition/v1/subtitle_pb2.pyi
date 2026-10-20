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

class SubtitleHearingImpaired(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SUBTITLE_HEARING_IMPAIRED_UNSPECIFIED: _ClassVar[SubtitleHearingImpaired]
    SUBTITLE_HEARING_IMPAIRED_REQUIRED: _ClassVar[SubtitleHearingImpaired]
    SUBTITLE_HEARING_IMPAIRED_EXCLUDED: _ClassVar[SubtitleHearingImpaired]

class SubtitleFolder(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SUBTITLE_FOLDER_UNSPECIFIED: _ClassVar[SubtitleFolder]
    SUBTITLE_FOLDER_RELATIVE: _ClassVar[SubtitleFolder]
    SUBTITLE_FOLDER_METADATA: _ClassVar[SubtitleFolder]

class SubtitleItemKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SUBTITLE_ITEM_KIND_UNSPECIFIED: _ClassVar[SubtitleItemKind]
    SUBTITLE_ITEM_KIND_MOVIE: _ClassVar[SubtitleItemKind]
    SUBTITLE_ITEM_KIND_EPISODE: _ClassVar[SubtitleItemKind]

class SubtitleAction(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SUBTITLE_ACTION_UNSPECIFIED: _ClassVar[SubtitleAction]
    SUBTITLE_ACTION_DELETED: _ClassVar[SubtitleAction]
    SUBTITLE_ACTION_DOWNLOADED: _ClassVar[SubtitleAction]
    SUBTITLE_ACTION_MANUAL: _ClassVar[SubtitleAction]
    SUBTITLE_ACTION_UPGRADED: _ClassVar[SubtitleAction]
    SUBTITLE_ACTION_UPLOADED: _ClassVar[SubtitleAction]
    SUBTITLE_ACTION_SYNCED: _ClassVar[SubtitleAction]
    SUBTITLE_ACTION_EXTRACTED: _ClassVar[SubtitleAction]
    SUBTITLE_ACTION_TRANSLATED: _ClassVar[SubtitleAction]
SUBTITLE_HEARING_IMPAIRED_UNSPECIFIED: SubtitleHearingImpaired
SUBTITLE_HEARING_IMPAIRED_REQUIRED: SubtitleHearingImpaired
SUBTITLE_HEARING_IMPAIRED_EXCLUDED: SubtitleHearingImpaired
SUBTITLE_FOLDER_UNSPECIFIED: SubtitleFolder
SUBTITLE_FOLDER_RELATIVE: SubtitleFolder
SUBTITLE_FOLDER_METADATA: SubtitleFolder
SUBTITLE_ITEM_KIND_UNSPECIFIED: SubtitleItemKind
SUBTITLE_ITEM_KIND_MOVIE: SubtitleItemKind
SUBTITLE_ITEM_KIND_EPISODE: SubtitleItemKind
SUBTITLE_ACTION_UNSPECIFIED: SubtitleAction
SUBTITLE_ACTION_DELETED: SubtitleAction
SUBTITLE_ACTION_DOWNLOADED: SubtitleAction
SUBTITLE_ACTION_MANUAL: SubtitleAction
SUBTITLE_ACTION_UPGRADED: SubtitleAction
SUBTITLE_ACTION_UPLOADED: SubtitleAction
SUBTITLE_ACTION_SYNCED: SubtitleAction
SUBTITLE_ACTION_EXTRACTED: SubtitleAction
SUBTITLE_ACTION_TRANSLATED: SubtitleAction

class SubtitleLanguageSpec(_message.Message):
    __slots__ = ("language", "forced", "hearing_impaired")
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    FORCED_FIELD_NUMBER: _ClassVar[int]
    HEARING_IMPAIRED_FIELD_NUMBER: _ClassVar[int]
    language: str
    forced: bool
    hearing_impaired: bool
    def __init__(self, language: _Optional[str] = ..., forced: _Optional[bool] = ..., hearing_impaired: _Optional[bool] = ...) -> None: ...

class SubtitleProfileItem(_message.Message):
    __slots__ = ("id", "language", "forced", "hearing_impaired", "audio_exclude", "audio_only_include")
    ID_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    FORCED_FIELD_NUMBER: _ClassVar[int]
    HEARING_IMPAIRED_FIELD_NUMBER: _ClassVar[int]
    AUDIO_EXCLUDE_FIELD_NUMBER: _ClassVar[int]
    AUDIO_ONLY_INCLUDE_FIELD_NUMBER: _ClassVar[int]
    id: int
    language: str
    forced: bool
    hearing_impaired: SubtitleHearingImpaired
    audio_exclude: bool
    audio_only_include: bool
    def __init__(self, id: _Optional[int] = ..., language: _Optional[str] = ..., forced: _Optional[bool] = ..., hearing_impaired: _Optional[_Union[SubtitleHearingImpaired, str]] = ..., audio_exclude: _Optional[bool] = ..., audio_only_include: _Optional[bool] = ...) -> None: ...

class SubtitleProfile(_message.Message):
    __slots__ = ("id", "name", "items", "cutoff", "must_contain", "must_not_contain", "original_format", "tag")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    CUTOFF_FIELD_NUMBER: _ClassVar[int]
    MUST_CONTAIN_FIELD_NUMBER: _ClassVar[int]
    MUST_NOT_CONTAIN_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_FORMAT_FIELD_NUMBER: _ClassVar[int]
    TAG_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    items: _containers.RepeatedCompositeFieldContainer[SubtitleProfileItem]
    cutoff: int
    must_contain: _containers.RepeatedScalarFieldContainer[str]
    must_not_contain: _containers.RepeatedScalarFieldContainer[str]
    original_format: bool
    tag: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., items: _Optional[_Iterable[_Union[SubtitleProfileItem, _Mapping]]] = ..., cutoff: _Optional[int] = ..., must_contain: _Optional[_Iterable[str]] = ..., must_not_contain: _Optional[_Iterable[str]] = ..., original_format: _Optional[bool] = ..., tag: _Optional[str] = ...) -> None: ...

class ListSubtitleProfilesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListSubtitleProfilesResponse(_message.Message):
    __slots__ = ("profiles",)
    PROFILES_FIELD_NUMBER: _ClassVar[int]
    profiles: _containers.RepeatedCompositeFieldContainer[SubtitleProfile]
    def __init__(self, profiles: _Optional[_Iterable[_Union[SubtitleProfile, _Mapping]]] = ...) -> None: ...

class CreateSubtitleProfileRequest(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: SubtitleProfile
    def __init__(self, profile: _Optional[_Union[SubtitleProfile, _Mapping]] = ...) -> None: ...

class CreateSubtitleProfileResponse(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: SubtitleProfile
    def __init__(self, profile: _Optional[_Union[SubtitleProfile, _Mapping]] = ...) -> None: ...

class UpdateSubtitleProfileRequest(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: SubtitleProfile
    def __init__(self, profile: _Optional[_Union[SubtitleProfile, _Mapping]] = ...) -> None: ...

class UpdateSubtitleProfileResponse(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: SubtitleProfile
    def __init__(self, profile: _Optional[_Union[SubtitleProfile, _Mapping]] = ...) -> None: ...

class DeleteSubtitleProfileRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteSubtitleProfileResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class AssignSubtitleProfileRequest(_message.Message):
    __slots__ = ("target_id", "profile_id")
    TARGET_ID_FIELD_NUMBER: _ClassVar[int]
    PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    target_id: str
    profile_id: str
    def __init__(self, target_id: _Optional[str] = ..., profile_id: _Optional[str] = ...) -> None: ...

class AssignSubtitleProfileResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListSubtitleProfileAssignmentsRequest(_message.Message):
    __slots__ = ("target_ids",)
    TARGET_IDS_FIELD_NUMBER: _ClassVar[int]
    target_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, target_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class SubtitleProfileAssignment(_message.Message):
    __slots__ = ("target_id", "profile_id")
    TARGET_ID_FIELD_NUMBER: _ClassVar[int]
    PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    target_id: str
    profile_id: str
    def __init__(self, target_id: _Optional[str] = ..., profile_id: _Optional[str] = ...) -> None: ...

class ListSubtitleProfileAssignmentsResponse(_message.Message):
    __slots__ = ("assignments",)
    ASSIGNMENTS_FIELD_NUMBER: _ClassVar[int]
    assignments: _containers.RepeatedCompositeFieldContainer[SubtitleProfileAssignment]
    def __init__(self, assignments: _Optional[_Iterable[_Union[SubtitleProfileAssignment, _Mapping]]] = ...) -> None: ...

class SubtitleLanguageEquivalence(_message.Message):
    __slots__ = ("to",)
    FROM_FIELD_NUMBER: _ClassVar[int]
    TO_FIELD_NUMBER: _ClassVar[int]
    to: SubtitleLanguageSpec
    def __init__(self, to: _Optional[_Union[SubtitleLanguageSpec, _Mapping]] = ..., **kwargs) -> None: ...

class SubtitleSyncSettings(_message.Message):
    __slots__ = ("enabled", "use_threshold", "use_movie_threshold", "threshold", "movie_threshold", "force_audio", "original_language", "excluded_providers", "excluded_languages", "max_offset_seconds", "fix_framerate", "golden_section", "min_score", "max_quality_offset_seconds", "max_scale_deviation")
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    USE_THRESHOLD_FIELD_NUMBER: _ClassVar[int]
    USE_MOVIE_THRESHOLD_FIELD_NUMBER: _ClassVar[int]
    THRESHOLD_FIELD_NUMBER: _ClassVar[int]
    MOVIE_THRESHOLD_FIELD_NUMBER: _ClassVar[int]
    FORCE_AUDIO_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    EXCLUDED_PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    EXCLUDED_LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    MAX_OFFSET_SECONDS_FIELD_NUMBER: _ClassVar[int]
    FIX_FRAMERATE_FIELD_NUMBER: _ClassVar[int]
    GOLDEN_SECTION_FIELD_NUMBER: _ClassVar[int]
    MIN_SCORE_FIELD_NUMBER: _ClassVar[int]
    MAX_QUALITY_OFFSET_SECONDS_FIELD_NUMBER: _ClassVar[int]
    MAX_SCALE_DEVIATION_FIELD_NUMBER: _ClassVar[int]
    enabled: bool
    use_threshold: bool
    use_movie_threshold: bool
    threshold: int
    movie_threshold: int
    force_audio: bool
    original_language: bool
    excluded_providers: _containers.RepeatedScalarFieldContainer[str]
    excluded_languages: _containers.RepeatedScalarFieldContainer[str]
    max_offset_seconds: int
    fix_framerate: bool
    golden_section: bool
    min_score: float
    max_quality_offset_seconds: int
    max_scale_deviation: float
    def __init__(self, enabled: _Optional[bool] = ..., use_threshold: _Optional[bool] = ..., use_movie_threshold: _Optional[bool] = ..., threshold: _Optional[int] = ..., movie_threshold: _Optional[int] = ..., force_audio: _Optional[bool] = ..., original_language: _Optional[bool] = ..., excluded_providers: _Optional[_Iterable[str]] = ..., excluded_languages: _Optional[_Iterable[str]] = ..., max_offset_seconds: _Optional[int] = ..., fix_framerate: _Optional[bool] = ..., golden_section: _Optional[bool] = ..., min_score: _Optional[float] = ..., max_quality_offset_seconds: _Optional[int] = ..., max_scale_deviation: _Optional[float] = ...) -> None: ...

class SubtitleSettings(_message.Message):
    __slots__ = ("minimum_score", "minimum_score_movie", "use_scene_name", "use_embedded", "ignore_pgs", "ignore_vobsub", "ignore_ass", "utf8", "mods", "hi_extension", "single_language", "folder", "custom_folder", "upgrade", "upgrade_interval_hours", "upgrade_days", "upgrade_manual", "adaptive_searching", "adaptive_delay_days", "adaptive_delta_days", "wanted_search_interval_hours", "wanted_search_interval_movie_hours", "default_audio_language", "default_embedded_language", "skip_hashing", "language_equals", "movie_profile_tags", "series_profile_tags", "no_profile_tags", "sync", "translation")
    MINIMUM_SCORE_FIELD_NUMBER: _ClassVar[int]
    MINIMUM_SCORE_MOVIE_FIELD_NUMBER: _ClassVar[int]
    USE_SCENE_NAME_FIELD_NUMBER: _ClassVar[int]
    USE_EMBEDDED_FIELD_NUMBER: _ClassVar[int]
    IGNORE_PGS_FIELD_NUMBER: _ClassVar[int]
    IGNORE_VOBSUB_FIELD_NUMBER: _ClassVar[int]
    IGNORE_ASS_FIELD_NUMBER: _ClassVar[int]
    UTF8_FIELD_NUMBER: _ClassVar[int]
    MODS_FIELD_NUMBER: _ClassVar[int]
    HI_EXTENSION_FIELD_NUMBER: _ClassVar[int]
    SINGLE_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    FOLDER_FIELD_NUMBER: _ClassVar[int]
    CUSTOM_FOLDER_FIELD_NUMBER: _ClassVar[int]
    UPGRADE_FIELD_NUMBER: _ClassVar[int]
    UPGRADE_INTERVAL_HOURS_FIELD_NUMBER: _ClassVar[int]
    UPGRADE_DAYS_FIELD_NUMBER: _ClassVar[int]
    UPGRADE_MANUAL_FIELD_NUMBER: _ClassVar[int]
    ADAPTIVE_SEARCHING_FIELD_NUMBER: _ClassVar[int]
    ADAPTIVE_DELAY_DAYS_FIELD_NUMBER: _ClassVar[int]
    ADAPTIVE_DELTA_DAYS_FIELD_NUMBER: _ClassVar[int]
    WANTED_SEARCH_INTERVAL_HOURS_FIELD_NUMBER: _ClassVar[int]
    WANTED_SEARCH_INTERVAL_MOVIE_HOURS_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_AUDIO_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_EMBEDDED_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    SKIP_HASHING_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_EQUALS_FIELD_NUMBER: _ClassVar[int]
    MOVIE_PROFILE_TAGS_FIELD_NUMBER: _ClassVar[int]
    SERIES_PROFILE_TAGS_FIELD_NUMBER: _ClassVar[int]
    NO_PROFILE_TAGS_FIELD_NUMBER: _ClassVar[int]
    SYNC_FIELD_NUMBER: _ClassVar[int]
    TRANSLATION_FIELD_NUMBER: _ClassVar[int]
    minimum_score: int
    minimum_score_movie: int
    use_scene_name: bool
    use_embedded: bool
    ignore_pgs: bool
    ignore_vobsub: bool
    ignore_ass: bool
    utf8: bool
    mods: _containers.RepeatedScalarFieldContainer[str]
    hi_extension: str
    single_language: bool
    folder: SubtitleFolder
    custom_folder: str
    upgrade: bool
    upgrade_interval_hours: int
    upgrade_days: int
    upgrade_manual: bool
    adaptive_searching: bool
    adaptive_delay_days: int
    adaptive_delta_days: int
    wanted_search_interval_hours: int
    wanted_search_interval_movie_hours: int
    default_audio_language: str
    default_embedded_language: str
    skip_hashing: bool
    language_equals: _containers.RepeatedCompositeFieldContainer[SubtitleLanguageEquivalence]
    movie_profile_tags: bool
    series_profile_tags: bool
    no_profile_tags: _containers.RepeatedScalarFieldContainer[str]
    sync: SubtitleSyncSettings
    translation: SubtitleTranslationSettings
    def __init__(self, minimum_score: _Optional[int] = ..., minimum_score_movie: _Optional[int] = ..., use_scene_name: _Optional[bool] = ..., use_embedded: _Optional[bool] = ..., ignore_pgs: _Optional[bool] = ..., ignore_vobsub: _Optional[bool] = ..., ignore_ass: _Optional[bool] = ..., utf8: _Optional[bool] = ..., mods: _Optional[_Iterable[str]] = ..., hi_extension: _Optional[str] = ..., single_language: _Optional[bool] = ..., folder: _Optional[_Union[SubtitleFolder, str]] = ..., custom_folder: _Optional[str] = ..., upgrade: _Optional[bool] = ..., upgrade_interval_hours: _Optional[int] = ..., upgrade_days: _Optional[int] = ..., upgrade_manual: _Optional[bool] = ..., adaptive_searching: _Optional[bool] = ..., adaptive_delay_days: _Optional[int] = ..., adaptive_delta_days: _Optional[int] = ..., wanted_search_interval_hours: _Optional[int] = ..., wanted_search_interval_movie_hours: _Optional[int] = ..., default_audio_language: _Optional[str] = ..., default_embedded_language: _Optional[str] = ..., skip_hashing: _Optional[bool] = ..., language_equals: _Optional[_Iterable[_Union[SubtitleLanguageEquivalence, _Mapping]]] = ..., movie_profile_tags: _Optional[bool] = ..., series_profile_tags: _Optional[bool] = ..., no_profile_tags: _Optional[_Iterable[str]] = ..., sync: _Optional[_Union[SubtitleSyncSettings, _Mapping]] = ..., translation: _Optional[_Union[SubtitleTranslationSettings, _Mapping]] = ...) -> None: ...

class SubtitleTranslationSettings(_message.Message):
    __slots__ = ("translator", "default_score", "credit", "fallback_translators", "prompt", "user_prompt", "proofread_prompt", "proofread_user_prompt", "context_before", "context_after", "translated_context", "batch", "max_batch_size", "language_codes", "max_retries", "retry_delay_seconds", "retry_multiplier", "request_timeout_seconds", "strip_formatting", "preserve_line_breaks", "fix_overlaps", "validation", "automation")
    TRANSLATOR_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_SCORE_FIELD_NUMBER: _ClassVar[int]
    CREDIT_FIELD_NUMBER: _ClassVar[int]
    FALLBACK_TRANSLATORS_FIELD_NUMBER: _ClassVar[int]
    PROMPT_FIELD_NUMBER: _ClassVar[int]
    USER_PROMPT_FIELD_NUMBER: _ClassVar[int]
    PROOFREAD_PROMPT_FIELD_NUMBER: _ClassVar[int]
    PROOFREAD_USER_PROMPT_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_BEFORE_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_AFTER_FIELD_NUMBER: _ClassVar[int]
    TRANSLATED_CONTEXT_FIELD_NUMBER: _ClassVar[int]
    BATCH_FIELD_NUMBER: _ClassVar[int]
    MAX_BATCH_SIZE_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_CODES_FIELD_NUMBER: _ClassVar[int]
    MAX_RETRIES_FIELD_NUMBER: _ClassVar[int]
    RETRY_DELAY_SECONDS_FIELD_NUMBER: _ClassVar[int]
    RETRY_MULTIPLIER_FIELD_NUMBER: _ClassVar[int]
    REQUEST_TIMEOUT_SECONDS_FIELD_NUMBER: _ClassVar[int]
    STRIP_FORMATTING_FIELD_NUMBER: _ClassVar[int]
    PRESERVE_LINE_BREAKS_FIELD_NUMBER: _ClassVar[int]
    FIX_OVERLAPS_FIELD_NUMBER: _ClassVar[int]
    VALIDATION_FIELD_NUMBER: _ClassVar[int]
    AUTOMATION_FIELD_NUMBER: _ClassVar[int]
    translator: str
    default_score: int
    credit: bool
    fallback_translators: _containers.RepeatedScalarFieldContainer[str]
    prompt: str
    user_prompt: str
    proofread_prompt: str
    proofread_user_prompt: str
    context_before: int
    context_after: int
    translated_context: bool
    batch: bool
    max_batch_size: int
    language_codes: bool
    max_retries: int
    retry_delay_seconds: int
    retry_multiplier: int
    request_timeout_seconds: int
    strip_formatting: bool
    preserve_line_breaks: bool
    fix_overlaps: bool
    validation: SubtitleTranslationValidation
    automation: SubtitleTranslationAutomation
    def __init__(self, translator: _Optional[str] = ..., default_score: _Optional[int] = ..., credit: _Optional[bool] = ..., fallback_translators: _Optional[_Iterable[str]] = ..., prompt: _Optional[str] = ..., user_prompt: _Optional[str] = ..., proofread_prompt: _Optional[str] = ..., proofread_user_prompt: _Optional[str] = ..., context_before: _Optional[int] = ..., context_after: _Optional[int] = ..., translated_context: _Optional[bool] = ..., batch: _Optional[bool] = ..., max_batch_size: _Optional[int] = ..., language_codes: _Optional[bool] = ..., max_retries: _Optional[int] = ..., retry_delay_seconds: _Optional[int] = ..., retry_multiplier: _Optional[int] = ..., request_timeout_seconds: _Optional[int] = ..., strip_formatting: _Optional[bool] = ..., preserve_line_breaks: _Optional[bool] = ..., fix_overlaps: _Optional[bool] = ..., validation: _Optional[_Union[SubtitleTranslationValidation, _Mapping]] = ..., automation: _Optional[_Union[SubtitleTranslationAutomation, _Mapping]] = ...) -> None: ...

class SubtitleTranslationValidation(_message.Message):
    __slots__ = ("enabled", "max_file_size", "max_cue_length", "min_cue_length", "min_duration_ms", "max_duration_seconds")
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    MAX_FILE_SIZE_FIELD_NUMBER: _ClassVar[int]
    MAX_CUE_LENGTH_FIELD_NUMBER: _ClassVar[int]
    MIN_CUE_LENGTH_FIELD_NUMBER: _ClassVar[int]
    MIN_DURATION_MS_FIELD_NUMBER: _ClassVar[int]
    MAX_DURATION_SECONDS_FIELD_NUMBER: _ClassVar[int]
    enabled: bool
    max_file_size: int
    max_cue_length: int
    min_cue_length: int
    min_duration_ms: int
    max_duration_seconds: int
    def __init__(self, enabled: _Optional[bool] = ..., max_file_size: _Optional[int] = ..., max_cue_length: _Optional[int] = ..., min_cue_length: _Optional[int] = ..., min_duration_ms: _Optional[int] = ..., max_duration_seconds: _Optional[int] = ...) -> None: ...

class SubtitleTranslationAutomation(_message.Message):
    __slots__ = ("enabled", "interval_hours", "source_languages", "max_per_run", "movie_age_hours", "episode_age_hours")
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    INTERVAL_HOURS_FIELD_NUMBER: _ClassVar[int]
    SOURCE_LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    MAX_PER_RUN_FIELD_NUMBER: _ClassVar[int]
    MOVIE_AGE_HOURS_FIELD_NUMBER: _ClassVar[int]
    EPISODE_AGE_HOURS_FIELD_NUMBER: _ClassVar[int]
    enabled: bool
    interval_hours: int
    source_languages: _containers.RepeatedScalarFieldContainer[str]
    max_per_run: int
    movie_age_hours: int
    episode_age_hours: int
    def __init__(self, enabled: _Optional[bool] = ..., interval_hours: _Optional[int] = ..., source_languages: _Optional[_Iterable[str]] = ..., max_per_run: _Optional[int] = ..., movie_age_hours: _Optional[int] = ..., episode_age_hours: _Optional[int] = ...) -> None: ...

class GetSubtitleSettingsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetSubtitleSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: SubtitleSettings
    def __init__(self, settings: _Optional[_Union[SubtitleSettings, _Mapping]] = ...) -> None: ...

class UpdateSubtitleSettingsRequest(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: SubtitleSettings
    def __init__(self, settings: _Optional[_Union[SubtitleSettings, _Mapping]] = ...) -> None: ...

class UpdateSubtitleSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: SubtitleSettings
    def __init__(self, settings: _Optional[_Union[SubtitleSettings, _Mapping]] = ...) -> None: ...

class ListSubtitleProvidersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SubtitleProviderState(_message.Message):
    __slots__ = ("id", "plugin_id", "name", "display_name", "enabled", "throttled_until", "throttle_reason", "languages", "movies", "episodes")
    ID_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    THROTTLED_UNTIL_FIELD_NUMBER: _ClassVar[int]
    THROTTLE_REASON_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    MOVIES_FIELD_NUMBER: _ClassVar[int]
    EPISODES_FIELD_NUMBER: _ClassVar[int]
    id: str
    plugin_id: str
    name: str
    display_name: str
    enabled: bool
    throttled_until: _timestamp_pb2.Timestamp
    throttle_reason: str
    languages: _containers.RepeatedScalarFieldContainer[str]
    movies: bool
    episodes: bool
    def __init__(self, id: _Optional[str] = ..., plugin_id: _Optional[str] = ..., name: _Optional[str] = ..., display_name: _Optional[str] = ..., enabled: _Optional[bool] = ..., throttled_until: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., throttle_reason: _Optional[str] = ..., languages: _Optional[_Iterable[str]] = ..., movies: _Optional[bool] = ..., episodes: _Optional[bool] = ...) -> None: ...

class ListSubtitleProvidersResponse(_message.Message):
    __slots__ = ("providers", "failed_plugins")
    PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    FAILED_PLUGINS_FIELD_NUMBER: _ClassVar[int]
    providers: _containers.RepeatedCompositeFieldContainer[SubtitleProviderState]
    failed_plugins: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, providers: _Optional[_Iterable[_Union[SubtitleProviderState, _Mapping]]] = ..., failed_plugins: _Optional[_Iterable[str]] = ...) -> None: ...

class WantedSubtitles(_message.Message):
    __slots__ = ("item_id", "name", "series_id", "series_name", "season_number", "episode_number", "missing", "path")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    SERIES_ID_FIELD_NUMBER: _ClassVar[int]
    SERIES_NAME_FIELD_NUMBER: _ClassVar[int]
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    EPISODE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    MISSING_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    name: str
    series_id: str
    series_name: str
    season_number: int
    episode_number: int
    missing: _containers.RepeatedCompositeFieldContainer[SubtitleLanguageSpec]
    path: str
    def __init__(self, item_id: _Optional[str] = ..., name: _Optional[str] = ..., series_id: _Optional[str] = ..., series_name: _Optional[str] = ..., season_number: _Optional[int] = ..., episode_number: _Optional[int] = ..., missing: _Optional[_Iterable[_Union[SubtitleLanguageSpec, _Mapping]]] = ..., path: _Optional[str] = ...) -> None: ...

class ListWantedSubtitlesRequest(_message.Message):
    __slots__ = ("kind", "limit", "offset")
    KIND_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    kind: SubtitleItemKind
    limit: int
    offset: int
    def __init__(self, kind: _Optional[_Union[SubtitleItemKind, str]] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class ListWantedSubtitlesResponse(_message.Message):
    __slots__ = ("items", "total")
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[WantedSubtitles]
    total: int
    def __init__(self, items: _Optional[_Iterable[_Union[WantedSubtitles, _Mapping]]] = ..., total: _Optional[int] = ...) -> None: ...

class SearchWantedSubtitlesRequest(_message.Message):
    __slots__ = ("kind", "item_ids")
    KIND_FIELD_NUMBER: _ClassVar[int]
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    kind: SubtitleItemKind
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, kind: _Optional[_Union[SubtitleItemKind, str]] = ..., item_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class SearchWantedSubtitlesResponse(_message.Message):
    __slots__ = ("queued",)
    QUEUED_FIELD_NUMBER: _ClassVar[int]
    queued: int
    def __init__(self, queued: _Optional[int] = ...) -> None: ...

class UpgradeSubtitlesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class UpgradeSubtitlesResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ItemSubtitle(_message.Message):
    __slots__ = ("language", "path", "stream_index", "codec", "counted")
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    CODEC_FIELD_NUMBER: _ClassVar[int]
    COUNTED_FIELD_NUMBER: _ClassVar[int]
    language: SubtitleLanguageSpec
    path: str
    stream_index: int
    codec: str
    counted: bool
    def __init__(self, language: _Optional[_Union[SubtitleLanguageSpec, _Mapping]] = ..., path: _Optional[str] = ..., stream_index: _Optional[int] = ..., codec: _Optional[str] = ..., counted: _Optional[bool] = ...) -> None: ...

class GetItemSubtitlesRequest(_message.Message):
    __slots__ = ("item_id",)
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    def __init__(self, item_id: _Optional[str] = ...) -> None: ...

class GetItemSubtitlesResponse(_message.Message):
    __slots__ = ("profile", "assigned_profile_id", "subtitles", "missing")
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    ASSIGNED_PROFILE_ID_FIELD_NUMBER: _ClassVar[int]
    SUBTITLES_FIELD_NUMBER: _ClassVar[int]
    MISSING_FIELD_NUMBER: _ClassVar[int]
    profile: SubtitleProfile
    assigned_profile_id: str
    subtitles: _containers.RepeatedCompositeFieldContainer[ItemSubtitle]
    missing: _containers.RepeatedCompositeFieldContainer[SubtitleLanguageSpec]
    def __init__(self, profile: _Optional[_Union[SubtitleProfile, _Mapping]] = ..., assigned_profile_id: _Optional[str] = ..., subtitles: _Optional[_Iterable[_Union[ItemSubtitle, _Mapping]]] = ..., missing: _Optional[_Iterable[_Union[SubtitleLanguageSpec, _Mapping]]] = ...) -> None: ...

class ScoredSubtitle(_message.Message):
    __slots__ = ("provider", "subtitle_id", "language", "release_names", "score", "max_score", "percent", "matched", "not_matched", "page_link", "uploader", "format", "blocklisted")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_ID_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    RELEASE_NAMES_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    MAX_SCORE_FIELD_NUMBER: _ClassVar[int]
    PERCENT_FIELD_NUMBER: _ClassVar[int]
    MATCHED_FIELD_NUMBER: _ClassVar[int]
    NOT_MATCHED_FIELD_NUMBER: _ClassVar[int]
    PAGE_LINK_FIELD_NUMBER: _ClassVar[int]
    UPLOADER_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    BLOCKLISTED_FIELD_NUMBER: _ClassVar[int]
    provider: str
    subtitle_id: str
    language: SubtitleLanguageSpec
    release_names: _containers.RepeatedScalarFieldContainer[str]
    score: int
    max_score: int
    percent: int
    matched: _containers.RepeatedScalarFieldContainer[str]
    not_matched: _containers.RepeatedScalarFieldContainer[str]
    page_link: str
    uploader: str
    format: str
    blocklisted: bool
    def __init__(self, provider: _Optional[str] = ..., subtitle_id: _Optional[str] = ..., language: _Optional[_Union[SubtitleLanguageSpec, _Mapping]] = ..., release_names: _Optional[_Iterable[str]] = ..., score: _Optional[int] = ..., max_score: _Optional[int] = ..., percent: _Optional[int] = ..., matched: _Optional[_Iterable[str]] = ..., not_matched: _Optional[_Iterable[str]] = ..., page_link: _Optional[str] = ..., uploader: _Optional[str] = ..., format: _Optional[str] = ..., blocklisted: _Optional[bool] = ...) -> None: ...

class SearchItemSubtitlesRequest(_message.Message):
    __slots__ = ("item_id", "languages")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    languages: _containers.RepeatedCompositeFieldContainer[SubtitleLanguageSpec]
    def __init__(self, item_id: _Optional[str] = ..., languages: _Optional[_Iterable[_Union[SubtitleLanguageSpec, _Mapping]]] = ...) -> None: ...

class SearchItemSubtitlesResponse(_message.Message):
    __slots__ = ("subtitles", "failed_providers")
    SUBTITLES_FIELD_NUMBER: _ClassVar[int]
    FAILED_PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    subtitles: _containers.RepeatedCompositeFieldContainer[ScoredSubtitle]
    failed_providers: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, subtitles: _Optional[_Iterable[_Union[ScoredSubtitle, _Mapping]]] = ..., failed_providers: _Optional[_Iterable[str]] = ...) -> None: ...

class DownloadItemSubtitleRequest(_message.Message):
    __slots__ = ("item_id", "provider", "subtitle_id", "language")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_ID_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    provider: str
    subtitle_id: str
    language: SubtitleLanguageSpec
    def __init__(self, item_id: _Optional[str] = ..., provider: _Optional[str] = ..., subtitle_id: _Optional[str] = ..., language: _Optional[_Union[SubtitleLanguageSpec, _Mapping]] = ...) -> None: ...

class DownloadItemSubtitleResponse(_message.Message):
    __slots__ = ("path",)
    PATH_FIELD_NUMBER: _ClassVar[int]
    path: str
    def __init__(self, path: _Optional[str] = ...) -> None: ...

class UploadSubtitleRequest(_message.Message):
    __slots__ = ("item_id", "data", "format", "language")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    data: bytes
    format: str
    language: SubtitleLanguageSpec
    def __init__(self, item_id: _Optional[str] = ..., data: _Optional[bytes] = ..., format: _Optional[str] = ..., language: _Optional[_Union[SubtitleLanguageSpec, _Mapping]] = ...) -> None: ...

class UploadSubtitleResponse(_message.Message):
    __slots__ = ("path",)
    PATH_FIELD_NUMBER: _ClassVar[int]
    path: str
    def __init__(self, path: _Optional[str] = ...) -> None: ...

class ExtractSubtitleRequest(_message.Message):
    __slots__ = ("item_id", "stream_index", "language")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    stream_index: int
    language: SubtitleLanguageSpec
    def __init__(self, item_id: _Optional[str] = ..., stream_index: _Optional[int] = ..., language: _Optional[_Union[SubtitleLanguageSpec, _Mapping]] = ...) -> None: ...

class ExtractSubtitleResponse(_message.Message):
    __slots__ = ("path",)
    PATH_FIELD_NUMBER: _ClassVar[int]
    path: str
    def __init__(self, path: _Optional[str] = ...) -> None: ...

class DeleteSubtitleRequest(_message.Message):
    __slots__ = ("item_id", "path")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    path: str
    def __init__(self, item_id: _Optional[str] = ..., path: _Optional[str] = ...) -> None: ...

class DeleteSubtitleResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ModifySubtitleRequest(_message.Message):
    __slots__ = ("item_id", "path", "mods")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    MODS_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    path: str
    mods: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, item_id: _Optional[str] = ..., path: _Optional[str] = ..., mods: _Optional[_Iterable[str]] = ...) -> None: ...

class ModifySubtitleResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SyncSubtitleRequest(_message.Message):
    __slots__ = ("item_id", "path", "reference_path")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    REFERENCE_PATH_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    path: str
    reference_path: str
    def __init__(self, item_id: _Optional[str] = ..., path: _Optional[str] = ..., reference_path: _Optional[str] = ...) -> None: ...

class SyncSubtitleResponse(_message.Message):
    __slots__ = ("offset_ms", "ratio")
    OFFSET_MS_FIELD_NUMBER: _ClassVar[int]
    RATIO_FIELD_NUMBER: _ClassVar[int]
    offset_ms: int
    ratio: float
    def __init__(self, offset_ms: _Optional[int] = ..., ratio: _Optional[float] = ...) -> None: ...

class TranslateSubtitleRequest(_message.Message):
    __slots__ = ("item_id", "path", "stream_index", "language", "translator")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    TRANSLATOR_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    path: str
    stream_index: int
    language: SubtitleLanguageSpec
    translator: str
    def __init__(self, item_id: _Optional[str] = ..., path: _Optional[str] = ..., stream_index: _Optional[int] = ..., language: _Optional[_Union[SubtitleLanguageSpec, _Mapping]] = ..., translator: _Optional[str] = ...) -> None: ...

class TranslateSubtitleResponse(_message.Message):
    __slots__ = ("job_id",)
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    def __init__(self, job_id: _Optional[str] = ...) -> None: ...

class ProofreadSubtitleRequest(_message.Message):
    __slots__ = ("item_id", "path", "translator")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    TRANSLATOR_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    path: str
    translator: str
    def __init__(self, item_id: _Optional[str] = ..., path: _Optional[str] = ..., translator: _Optional[str] = ...) -> None: ...

class ProofreadSubtitleResponse(_message.Message):
    __slots__ = ("job_id",)
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    def __init__(self, job_id: _Optional[str] = ...) -> None: ...

class GetSubtitleTranslationStatisticsRequest(_message.Message):
    __slots__ = ("days",)
    DAYS_FIELD_NUMBER: _ClassVar[int]
    days: int
    def __init__(self, days: _Optional[int] = ...) -> None: ...

class GetSubtitleTranslationStatisticsResponse(_message.Message):
    __slots__ = ("files", "cues", "characters", "input_tokens", "output_tokens", "cost", "by_translator", "by_language_pair", "by_media", "daily")
    class ByTranslatorEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: int
        def __init__(self, key: _Optional[str] = ..., value: _Optional[int] = ...) -> None: ...
    class ByLanguagePairEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: int
        def __init__(self, key: _Optional[str] = ..., value: _Optional[int] = ...) -> None: ...
    class ByMediaEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: int
        def __init__(self, key: _Optional[str] = ..., value: _Optional[int] = ...) -> None: ...
    FILES_FIELD_NUMBER: _ClassVar[int]
    CUES_FIELD_NUMBER: _ClassVar[int]
    CHARACTERS_FIELD_NUMBER: _ClassVar[int]
    INPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    COST_FIELD_NUMBER: _ClassVar[int]
    BY_TRANSLATOR_FIELD_NUMBER: _ClassVar[int]
    BY_LANGUAGE_PAIR_FIELD_NUMBER: _ClassVar[int]
    BY_MEDIA_FIELD_NUMBER: _ClassVar[int]
    DAILY_FIELD_NUMBER: _ClassVar[int]
    files: int
    cues: int
    characters: int
    input_tokens: int
    output_tokens: int
    cost: float
    by_translator: _containers.ScalarMap[str, int]
    by_language_pair: _containers.ScalarMap[str, int]
    by_media: _containers.ScalarMap[str, int]
    daily: _containers.RepeatedCompositeFieldContainer[SubtitleTranslationDay]
    def __init__(self, files: _Optional[int] = ..., cues: _Optional[int] = ..., characters: _Optional[int] = ..., input_tokens: _Optional[int] = ..., output_tokens: _Optional[int] = ..., cost: _Optional[float] = ..., by_translator: _Optional[_Mapping[str, int]] = ..., by_language_pair: _Optional[_Mapping[str, int]] = ..., by_media: _Optional[_Mapping[str, int]] = ..., daily: _Optional[_Iterable[_Union[SubtitleTranslationDay, _Mapping]]] = ...) -> None: ...

class SubtitleTranslationDay(_message.Message):
    __slots__ = ("date", "files")
    DATE_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    date: str
    files: int
    def __init__(self, date: _Optional[str] = ..., files: _Optional[int] = ...) -> None: ...

class ListSubtitleTranslatorsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SubtitleTranslatorState(_message.Message):
    __slots__ = ("id", "plugin_id", "name", "display_name", "enabled")
    ID_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    id: str
    plugin_id: str
    name: str
    display_name: str
    enabled: bool
    def __init__(self, id: _Optional[str] = ..., plugin_id: _Optional[str] = ..., name: _Optional[str] = ..., display_name: _Optional[str] = ..., enabled: _Optional[bool] = ...) -> None: ...

class ListSubtitleTranslatorsResponse(_message.Message):
    __slots__ = ("translators", "failed_plugins")
    TRANSLATORS_FIELD_NUMBER: _ClassVar[int]
    FAILED_PLUGINS_FIELD_NUMBER: _ClassVar[int]
    translators: _containers.RepeatedCompositeFieldContainer[SubtitleTranslatorState]
    failed_plugins: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, translators: _Optional[_Iterable[_Union[SubtitleTranslatorState, _Mapping]]] = ..., failed_plugins: _Optional[_Iterable[str]] = ...) -> None: ...

class BlocklistSubtitleRequest(_message.Message):
    __slots__ = ("history_event_id",)
    HISTORY_EVENT_ID_FIELD_NUMBER: _ClassVar[int]
    history_event_id: str
    def __init__(self, history_event_id: _Optional[str] = ...) -> None: ...

class BlocklistSubtitleResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SubtitleHistoryEvent(_message.Message):
    __slots__ = ("id", "action", "item_id", "series_id", "time", "language", "provider", "subtitle_id", "score", "max_score", "path", "video_path", "description", "matched", "not_matched", "upgraded_from", "translation")
    ID_FIELD_NUMBER: _ClassVar[int]
    ACTION_FIELD_NUMBER: _ClassVar[int]
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    SERIES_ID_FIELD_NUMBER: _ClassVar[int]
    TIME_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_ID_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    MAX_SCORE_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    VIDEO_PATH_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    MATCHED_FIELD_NUMBER: _ClassVar[int]
    NOT_MATCHED_FIELD_NUMBER: _ClassVar[int]
    UPGRADED_FROM_FIELD_NUMBER: _ClassVar[int]
    TRANSLATION_FIELD_NUMBER: _ClassVar[int]
    id: str
    action: SubtitleAction
    item_id: str
    series_id: str
    time: _timestamp_pb2.Timestamp
    language: SubtitleLanguageSpec
    provider: str
    subtitle_id: str
    score: int
    max_score: int
    path: str
    video_path: str
    description: str
    matched: _containers.RepeatedScalarFieldContainer[str]
    not_matched: _containers.RepeatedScalarFieldContainer[str]
    upgraded_from: str
    translation: SubtitleTranslationRecord
    def __init__(self, id: _Optional[str] = ..., action: _Optional[_Union[SubtitleAction, str]] = ..., item_id: _Optional[str] = ..., series_id: _Optional[str] = ..., time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., language: _Optional[_Union[SubtitleLanguageSpec, _Mapping]] = ..., provider: _Optional[str] = ..., subtitle_id: _Optional[str] = ..., score: _Optional[int] = ..., max_score: _Optional[int] = ..., path: _Optional[str] = ..., video_path: _Optional[str] = ..., description: _Optional[str] = ..., matched: _Optional[_Iterable[str]] = ..., not_matched: _Optional[_Iterable[str]] = ..., upgraded_from: _Optional[str] = ..., translation: _Optional[_Union[SubtitleTranslationRecord, _Mapping]] = ...) -> None: ...

class SubtitleTranslationRecord(_message.Message):
    __slots__ = ("source_path", "source_stream_index", "source_language", "translators", "cues", "characters", "input_tokens", "output_tokens", "cost", "proofread", "revised")
    SOURCE_PATH_FIELD_NUMBER: _ClassVar[int]
    SOURCE_STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    SOURCE_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    TRANSLATORS_FIELD_NUMBER: _ClassVar[int]
    CUES_FIELD_NUMBER: _ClassVar[int]
    CHARACTERS_FIELD_NUMBER: _ClassVar[int]
    INPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    COST_FIELD_NUMBER: _ClassVar[int]
    PROOFREAD_FIELD_NUMBER: _ClassVar[int]
    REVISED_FIELD_NUMBER: _ClassVar[int]
    source_path: str
    source_stream_index: int
    source_language: str
    translators: _containers.RepeatedScalarFieldContainer[str]
    cues: int
    characters: int
    input_tokens: int
    output_tokens: int
    cost: float
    proofread: bool
    revised: int
    def __init__(self, source_path: _Optional[str] = ..., source_stream_index: _Optional[int] = ..., source_language: _Optional[str] = ..., translators: _Optional[_Iterable[str]] = ..., cues: _Optional[int] = ..., characters: _Optional[int] = ..., input_tokens: _Optional[int] = ..., output_tokens: _Optional[int] = ..., cost: _Optional[float] = ..., proofread: _Optional[bool] = ..., revised: _Optional[int] = ...) -> None: ...

class ListSubtitleHistoryRequest(_message.Message):
    __slots__ = ("item_ids", "actions", "limit", "offset")
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    ACTIONS_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    actions: _containers.RepeatedScalarFieldContainer[SubtitleAction]
    limit: int
    offset: int
    def __init__(self, item_ids: _Optional[_Iterable[str]] = ..., actions: _Optional[_Iterable[_Union[SubtitleAction, str]]] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class ListSubtitleHistoryResponse(_message.Message):
    __slots__ = ("events", "total")
    EVENTS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    events: _containers.RepeatedCompositeFieldContainer[SubtitleHistoryEvent]
    total: int
    def __init__(self, events: _Optional[_Iterable[_Union[SubtitleHistoryEvent, _Mapping]]] = ..., total: _Optional[int] = ...) -> None: ...

class SubtitleBlocklistEntry(_message.Message):
    __slots__ = ("id", "item_id", "series_id", "provider", "subtitle_id", "language", "time", "message")
    ID_FIELD_NUMBER: _ClassVar[int]
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    SERIES_ID_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_ID_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    TIME_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    id: str
    item_id: str
    series_id: str
    provider: str
    subtitle_id: str
    language: SubtitleLanguageSpec
    time: _timestamp_pb2.Timestamp
    message: str
    def __init__(self, id: _Optional[str] = ..., item_id: _Optional[str] = ..., series_id: _Optional[str] = ..., provider: _Optional[str] = ..., subtitle_id: _Optional[str] = ..., language: _Optional[_Union[SubtitleLanguageSpec, _Mapping]] = ..., time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., message: _Optional[str] = ...) -> None: ...

class ListSubtitleBlocklistRequest(_message.Message):
    __slots__ = ("item_id", "limit", "offset")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    limit: int
    offset: int
    def __init__(self, item_id: _Optional[str] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class ListSubtitleBlocklistResponse(_message.Message):
    __slots__ = ("entries", "total")
    ENTRIES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    entries: _containers.RepeatedCompositeFieldContainer[SubtitleBlocklistEntry]
    total: int
    def __init__(self, entries: _Optional[_Iterable[_Union[SubtitleBlocklistEntry, _Mapping]]] = ..., total: _Optional[int] = ...) -> None: ...

class DeleteSubtitleBlocklistRequest(_message.Message):
    __slots__ = ("ids", "all")
    IDS_FIELD_NUMBER: _ClassVar[int]
    ALL_FIELD_NUMBER: _ClassVar[int]
    ids: _containers.RepeatedScalarFieldContainer[str]
    all: bool
    def __init__(self, ids: _Optional[_Iterable[str]] = ..., all: _Optional[bool] = ...) -> None: ...

class DeleteSubtitleBlocklistResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

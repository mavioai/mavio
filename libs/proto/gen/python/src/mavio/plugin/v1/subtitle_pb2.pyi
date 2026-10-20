import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from mavio.plugin.v1 import settings_pb2 as _settings_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SubtitleFailureKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SUBTITLE_FAILURE_KIND_UNSPECIFIED: _ClassVar[SubtitleFailureKind]
    SUBTITLE_FAILURE_KIND_TOO_MANY_REQUESTS: _ClassVar[SubtitleFailureKind]
    SUBTITLE_FAILURE_KIND_DOWNLOAD_LIMIT: _ClassVar[SubtitleFailureKind]
    SUBTITLE_FAILURE_KIND_SERVICE_UNAVAILABLE: _ClassVar[SubtitleFailureKind]
    SUBTITLE_FAILURE_KIND_AUTHENTICATION: _ClassVar[SubtitleFailureKind]
    SUBTITLE_FAILURE_KIND_CONFIGURATION: _ClassVar[SubtitleFailureKind]
    SUBTITLE_FAILURE_KIND_BAD_SUBTITLE: _ClassVar[SubtitleFailureKind]
    SUBTITLE_FAILURE_KIND_OTHER: _ClassVar[SubtitleFailureKind]
SUBTITLE_FAILURE_KIND_UNSPECIFIED: SubtitleFailureKind
SUBTITLE_FAILURE_KIND_TOO_MANY_REQUESTS: SubtitleFailureKind
SUBTITLE_FAILURE_KIND_DOWNLOAD_LIMIT: SubtitleFailureKind
SUBTITLE_FAILURE_KIND_SERVICE_UNAVAILABLE: SubtitleFailureKind
SUBTITLE_FAILURE_KIND_AUTHENTICATION: SubtitleFailureKind
SUBTITLE_FAILURE_KIND_CONFIGURATION: SubtitleFailureKind
SUBTITLE_FAILURE_KIND_BAD_SUBTITLE: SubtitleFailureKind
SUBTITLE_FAILURE_KIND_OTHER: SubtitleFailureKind

class ListSubtitleProvidersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListSubtitleProvidersResponse(_message.Message):
    __slots__ = ("providers",)
    PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    providers: _containers.RepeatedCompositeFieldContainer[SubtitleProviderInfo]
    def __init__(self, providers: _Optional[_Iterable[_Union[SubtitleProviderInfo, _Mapping]]] = ...) -> None: ...

class SubtitleProviderInfo(_message.Message):
    __slots__ = ("name", "display_name", "settings", "enabled", "throttled_until", "throttle_reason", "languages", "hash_verifiable", "hearing_impaired_verifiable", "movies", "episodes")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    THROTTLED_UNTIL_FIELD_NUMBER: _ClassVar[int]
    THROTTLE_REASON_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    HASH_VERIFIABLE_FIELD_NUMBER: _ClassVar[int]
    HEARING_IMPAIRED_VERIFIABLE_FIELD_NUMBER: _ClassVar[int]
    MOVIES_FIELD_NUMBER: _ClassVar[int]
    EPISODES_FIELD_NUMBER: _ClassVar[int]
    name: str
    display_name: str
    settings: _containers.RepeatedCompositeFieldContainer[_settings_pb2.SettingField]
    enabled: bool
    throttled_until: _timestamp_pb2.Timestamp
    throttle_reason: str
    languages: _containers.RepeatedScalarFieldContainer[str]
    hash_verifiable: bool
    hearing_impaired_verifiable: bool
    movies: bool
    episodes: bool
    def __init__(self, name: _Optional[str] = ..., display_name: _Optional[str] = ..., settings: _Optional[_Iterable[_Union[_settings_pb2.SettingField, _Mapping]]] = ..., enabled: _Optional[bool] = ..., throttled_until: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., throttle_reason: _Optional[str] = ..., languages: _Optional[_Iterable[str]] = ..., hash_verifiable: _Optional[bool] = ..., hearing_impaired_verifiable: _Optional[bool] = ..., movies: _Optional[bool] = ..., episodes: _Optional[bool] = ...) -> None: ...

class SubtitleLanguage(_message.Message):
    __slots__ = ("language", "forced", "hearing_impaired")
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    FORCED_FIELD_NUMBER: _ClassVar[int]
    HEARING_IMPAIRED_FIELD_NUMBER: _ClassVar[int]
    language: str
    forced: bool
    hearing_impaired: bool
    def __init__(self, language: _Optional[str] = ..., forced: _Optional[bool] = ..., hearing_impaired: _Optional[bool] = ...) -> None: ...

class SubtitleVideo(_message.Message):
    __slots__ = ("kind", "title", "alternative_titles", "year", "external_ids", "series", "alternative_series", "series_external_ids", "season", "episode", "absolute_episode", "original_series", "special", "release_name", "source", "release_group", "resolution", "video_codec", "audio_codec", "edition", "streaming_service", "frame_rate", "file_name", "file_size", "file_hash", "audio_languages", "original_language", "audio_url", "audio_streams")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    class SeriesExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    KIND_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    ALTERNATIVE_TITLES_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    SERIES_FIELD_NUMBER: _ClassVar[int]
    ALTERNATIVE_SERIES_FIELD_NUMBER: _ClassVar[int]
    SERIES_EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    SEASON_FIELD_NUMBER: _ClassVar[int]
    EPISODE_FIELD_NUMBER: _ClassVar[int]
    ABSOLUTE_EPISODE_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_SERIES_FIELD_NUMBER: _ClassVar[int]
    SPECIAL_FIELD_NUMBER: _ClassVar[int]
    RELEASE_NAME_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    RELEASE_GROUP_FIELD_NUMBER: _ClassVar[int]
    RESOLUTION_FIELD_NUMBER: _ClassVar[int]
    VIDEO_CODEC_FIELD_NUMBER: _ClassVar[int]
    AUDIO_CODEC_FIELD_NUMBER: _ClassVar[int]
    EDITION_FIELD_NUMBER: _ClassVar[int]
    STREAMING_SERVICE_FIELD_NUMBER: _ClassVar[int]
    FRAME_RATE_FIELD_NUMBER: _ClassVar[int]
    FILE_NAME_FIELD_NUMBER: _ClassVar[int]
    FILE_SIZE_FIELD_NUMBER: _ClassVar[int]
    FILE_HASH_FIELD_NUMBER: _ClassVar[int]
    AUDIO_LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    AUDIO_URL_FIELD_NUMBER: _ClassVar[int]
    AUDIO_STREAMS_FIELD_NUMBER: _ClassVar[int]
    kind: _metadata_pb2.MediaKind
    title: str
    alternative_titles: _containers.RepeatedScalarFieldContainer[str]
    year: int
    external_ids: _containers.ScalarMap[str, str]
    series: str
    alternative_series: _containers.RepeatedScalarFieldContainer[str]
    series_external_ids: _containers.ScalarMap[str, str]
    season: int
    episode: int
    absolute_episode: int
    original_series: bool
    special: bool
    release_name: str
    source: str
    release_group: str
    resolution: str
    video_codec: str
    audio_codec: str
    edition: str
    streaming_service: str
    frame_rate: float
    file_name: str
    file_size: int
    file_hash: str
    audio_languages: _containers.RepeatedScalarFieldContainer[str]
    original_language: str
    audio_url: str
    audio_streams: _containers.RepeatedCompositeFieldContainer[SubtitleAudioStream]
    def __init__(self, kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., title: _Optional[str] = ..., alternative_titles: _Optional[_Iterable[str]] = ..., year: _Optional[int] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., series: _Optional[str] = ..., alternative_series: _Optional[_Iterable[str]] = ..., series_external_ids: _Optional[_Mapping[str, str]] = ..., season: _Optional[int] = ..., episode: _Optional[int] = ..., absolute_episode: _Optional[int] = ..., original_series: _Optional[bool] = ..., special: _Optional[bool] = ..., release_name: _Optional[str] = ..., source: _Optional[str] = ..., release_group: _Optional[str] = ..., resolution: _Optional[str] = ..., video_codec: _Optional[str] = ..., audio_codec: _Optional[str] = ..., edition: _Optional[str] = ..., streaming_service: _Optional[str] = ..., frame_rate: _Optional[float] = ..., file_name: _Optional[str] = ..., file_size: _Optional[int] = ..., file_hash: _Optional[str] = ..., audio_languages: _Optional[_Iterable[str]] = ..., original_language: _Optional[str] = ..., audio_url: _Optional[str] = ..., audio_streams: _Optional[_Iterable[_Union[SubtitleAudioStream, _Mapping]]] = ...) -> None: ...

class SubtitleAudioStream(_message.Message):
    __slots__ = ("index", "language")
    INDEX_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    index: int
    language: str
    def __init__(self, index: _Optional[int] = ..., language: _Optional[str] = ...) -> None: ...

class SearchSubtitlesRequest(_message.Message):
    __slots__ = ("video", "languages", "providers")
    VIDEO_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    video: SubtitleVideo
    languages: _containers.RepeatedCompositeFieldContainer[SubtitleLanguage]
    providers: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, video: _Optional[_Union[SubtitleVideo, _Mapping]] = ..., languages: _Optional[_Iterable[_Union[SubtitleLanguage, _Mapping]]] = ..., providers: _Optional[_Iterable[str]] = ...) -> None: ...

class SearchSubtitlesResponse(_message.Message):
    __slots__ = ("subtitles", "failures")
    SUBTITLES_FIELD_NUMBER: _ClassVar[int]
    FAILURES_FIELD_NUMBER: _ClassVar[int]
    subtitles: _containers.RepeatedCompositeFieldContainer[RemoteSubtitle]
    failures: _containers.RepeatedCompositeFieldContainer[SubtitleProviderFailure]
    def __init__(self, subtitles: _Optional[_Iterable[_Union[RemoteSubtitle, _Mapping]]] = ..., failures: _Optional[_Iterable[_Union[SubtitleProviderFailure, _Mapping]]] = ...) -> None: ...

class RemoteSubtitle(_message.Message):
    __slots__ = ("provider", "id", "language", "release_names", "matches", "hash_verifiable", "hearing_impaired_verifiable", "format", "page_link", "uploader", "comment", "download_count")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    RELEASE_NAMES_FIELD_NUMBER: _ClassVar[int]
    MATCHES_FIELD_NUMBER: _ClassVar[int]
    HASH_VERIFIABLE_FIELD_NUMBER: _ClassVar[int]
    HEARING_IMPAIRED_VERIFIABLE_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    PAGE_LINK_FIELD_NUMBER: _ClassVar[int]
    UPLOADER_FIELD_NUMBER: _ClassVar[int]
    COMMENT_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_COUNT_FIELD_NUMBER: _ClassVar[int]
    provider: str
    id: str
    language: SubtitleLanguage
    release_names: _containers.RepeatedScalarFieldContainer[str]
    matches: _containers.RepeatedScalarFieldContainer[str]
    hash_verifiable: bool
    hearing_impaired_verifiable: bool
    format: str
    page_link: str
    uploader: str
    comment: str
    download_count: int
    def __init__(self, provider: _Optional[str] = ..., id: _Optional[str] = ..., language: _Optional[_Union[SubtitleLanguage, _Mapping]] = ..., release_names: _Optional[_Iterable[str]] = ..., matches: _Optional[_Iterable[str]] = ..., hash_verifiable: _Optional[bool] = ..., hearing_impaired_verifiable: _Optional[bool] = ..., format: _Optional[str] = ..., page_link: _Optional[str] = ..., uploader: _Optional[str] = ..., comment: _Optional[str] = ..., download_count: _Optional[int] = ...) -> None: ...

class DownloadSubtitleRequest(_message.Message):
    __slots__ = ("provider", "id", "video")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    VIDEO_FIELD_NUMBER: _ClassVar[int]
    provider: str
    id: str
    video: SubtitleVideo
    def __init__(self, provider: _Optional[str] = ..., id: _Optional[str] = ..., video: _Optional[_Union[SubtitleVideo, _Mapping]] = ...) -> None: ...

class DownloadSubtitleResponse(_message.Message):
    __slots__ = ("data", "format", "language", "failure")
    DATA_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    FAILURE_FIELD_NUMBER: _ClassVar[int]
    data: bytes
    format: str
    language: SubtitleLanguage
    failure: SubtitleProviderFailure
    def __init__(self, data: _Optional[bytes] = ..., format: _Optional[str] = ..., language: _Optional[_Union[SubtitleLanguage, _Mapping]] = ..., failure: _Optional[_Union[SubtitleProviderFailure, _Mapping]] = ...) -> None: ...

class ListSubtitleTranslatorsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListSubtitleTranslatorsResponse(_message.Message):
    __slots__ = ("translators",)
    TRANSLATORS_FIELD_NUMBER: _ClassVar[int]
    translators: _containers.RepeatedCompositeFieldContainer[SubtitleTranslatorInfo]
    def __init__(self, translators: _Optional[_Iterable[_Union[SubtitleTranslatorInfo, _Mapping]]] = ...) -> None: ...

class SubtitleTranslatorInfo(_message.Message):
    __slots__ = ("name", "display_name", "settings", "enabled", "batch_size", "markup", "source_languages", "target_languages")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    BATCH_SIZE_FIELD_NUMBER: _ClassVar[int]
    MARKUP_FIELD_NUMBER: _ClassVar[int]
    SOURCE_LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    TARGET_LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    name: str
    display_name: str
    settings: _containers.RepeatedCompositeFieldContainer[_settings_pb2.SettingField]
    enabled: bool
    batch_size: int
    markup: bool
    source_languages: _containers.RepeatedScalarFieldContainer[str]
    target_languages: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, name: _Optional[str] = ..., display_name: _Optional[str] = ..., settings: _Optional[_Iterable[_Union[_settings_pb2.SettingField, _Mapping]]] = ..., enabled: _Optional[bool] = ..., batch_size: _Optional[int] = ..., markup: _Optional[bool] = ..., source_languages: _Optional[_Iterable[str]] = ..., target_languages: _Optional[_Iterable[str]] = ...) -> None: ...

class SubtitleTranslationMedia(_message.Message):
    __slots__ = ("kind", "title", "year", "external_ids", "overview", "season", "episode", "episode_title", "source_file_name", "file_name")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    KIND_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    OVERVIEW_FIELD_NUMBER: _ClassVar[int]
    SEASON_FIELD_NUMBER: _ClassVar[int]
    EPISODE_FIELD_NUMBER: _ClassVar[int]
    EPISODE_TITLE_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FILE_NAME_FIELD_NUMBER: _ClassVar[int]
    FILE_NAME_FIELD_NUMBER: _ClassVar[int]
    kind: _metadata_pb2.MediaKind
    title: str
    year: int
    external_ids: _containers.ScalarMap[str, str]
    overview: str
    season: int
    episode: int
    episode_title: str
    source_file_name: str
    file_name: str
    def __init__(self, kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., title: _Optional[str] = ..., year: _Optional[int] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., overview: _Optional[str] = ..., season: _Optional[int] = ..., episode: _Optional[int] = ..., episode_title: _Optional[str] = ..., source_file_name: _Optional[str] = ..., file_name: _Optional[str] = ...) -> None: ...

class TranslateSubtitleLinesRequest(_message.Message):
    __slots__ = ("translator", "source_language", "target_language", "lines", "media")
    TRANSLATOR_FIELD_NUMBER: _ClassVar[int]
    SOURCE_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    TARGET_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    LINES_FIELD_NUMBER: _ClassVar[int]
    MEDIA_FIELD_NUMBER: _ClassVar[int]
    translator: str
    source_language: str
    target_language: str
    lines: _containers.RepeatedScalarFieldContainer[str]
    media: SubtitleTranslationMedia
    def __init__(self, translator: _Optional[str] = ..., source_language: _Optional[str] = ..., target_language: _Optional[str] = ..., lines: _Optional[_Iterable[str]] = ..., media: _Optional[_Union[SubtitleTranslationMedia, _Mapping]] = ...) -> None: ...

class TranslateSubtitleLinesResponse(_message.Message):
    __slots__ = ("lines", "credit")
    LINES_FIELD_NUMBER: _ClassVar[int]
    CREDIT_FIELD_NUMBER: _ClassVar[int]
    lines: _containers.RepeatedScalarFieldContainer[str]
    credit: str
    def __init__(self, lines: _Optional[_Iterable[str]] = ..., credit: _Optional[str] = ...) -> None: ...

class SubtitleProviderFailure(_message.Message):
    __slots__ = ("provider", "kind", "message", "throttled_until")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    THROTTLED_UNTIL_FIELD_NUMBER: _ClassVar[int]
    provider: str
    kind: SubtitleFailureKind
    message: str
    throttled_until: _timestamp_pb2.Timestamp
    def __init__(self, provider: _Optional[str] = ..., kind: _Optional[_Union[SubtitleFailureKind, str]] = ..., message: _Optional[str] = ..., throttled_until: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

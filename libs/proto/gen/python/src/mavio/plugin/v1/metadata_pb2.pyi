import datetime

from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class MediaKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MEDIA_KIND_UNSPECIFIED: _ClassVar[MediaKind]
    MEDIA_KIND_MOVIE: _ClassVar[MediaKind]
    MEDIA_KIND_SERIES: _ClassVar[MediaKind]
    MEDIA_KIND_SEASON: _ClassVar[MediaKind]
    MEDIA_KIND_EPISODE: _ClassVar[MediaKind]
    MEDIA_KIND_MUSIC_ARTIST: _ClassVar[MediaKind]
    MEDIA_KIND_MUSIC_ALBUM: _ClassVar[MediaKind]
    MEDIA_KIND_TRACK: _ClassVar[MediaKind]
    MEDIA_KIND_MUSIC_VIDEO: _ClassVar[MediaKind]
    MEDIA_KIND_BOOK: _ClassVar[MediaKind]
    MEDIA_KIND_AUDIOBOOK: _ClassVar[MediaKind]
    MEDIA_KIND_PERSON: _ClassVar[MediaKind]
    MEDIA_KIND_VIDEO: _ClassVar[MediaKind]
    MEDIA_KIND_PHOTO_ALBUM: _ClassVar[MediaKind]
    MEDIA_KIND_PHOTO: _ClassVar[MediaKind]

class SeriesStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SERIES_STATUS_UNSPECIFIED: _ClassVar[SeriesStatus]
    SERIES_STATUS_CONTINUING: _ClassVar[SeriesStatus]
    SERIES_STATUS_ENDED: _ClassVar[SeriesStatus]
    SERIES_STATUS_UNRELEASED: _ClassVar[SeriesStatus]

class ProductionStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PRODUCTION_STATUS_UNSPECIFIED: _ClassVar[ProductionStatus]
    PRODUCTION_STATUS_RUMORED: _ClassVar[ProductionStatus]
    PRODUCTION_STATUS_PLANNED: _ClassVar[ProductionStatus]
    PRODUCTION_STATUS_IN_PRODUCTION: _ClassVar[ProductionStatus]
    PRODUCTION_STATUS_POST_PRODUCTION: _ClassVar[ProductionStatus]
    PRODUCTION_STATUS_RELEASED: _ClassVar[ProductionStatus]
    PRODUCTION_STATUS_CANCELED: _ClassVar[ProductionStatus]
    PRODUCTION_STATUS_PILOT: _ClassVar[ProductionStatus]
    PRODUCTION_STATUS_RETURNING: _ClassVar[ProductionStatus]
    PRODUCTION_STATUS_ENDED: _ClassVar[ProductionStatus]

class CreditKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CREDIT_KIND_UNSPECIFIED: _ClassVar[CreditKind]
    CREDIT_KIND_ACTOR: _ClassVar[CreditKind]
    CREDIT_KIND_GUEST_STAR: _ClassVar[CreditKind]
    CREDIT_KIND_DIRECTOR: _ClassVar[CreditKind]
    CREDIT_KIND_WRITER: _ClassVar[CreditKind]
    CREDIT_KIND_PRODUCER: _ClassVar[CreditKind]
    CREDIT_KIND_CREATOR: _ClassVar[CreditKind]
    CREDIT_KIND_COMPOSER: _ClassVar[CreditKind]
    CREDIT_KIND_CONDUCTOR: _ClassVar[CreditKind]
    CREDIT_KIND_LYRICIST: _ClassVar[CreditKind]
    CREDIT_KIND_ARTIST: _ClassVar[CreditKind]
    CREDIT_KIND_AUTHOR: _ClassVar[CreditKind]
    CREDIT_KIND_NARRATOR: _ClassVar[CreditKind]
    CREDIT_KIND_OTHER: _ClassVar[CreditKind]

class ImageKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    IMAGE_KIND_UNSPECIFIED: _ClassVar[ImageKind]
    IMAGE_KIND_PRIMARY: _ClassVar[ImageKind]
    IMAGE_KIND_BACKDROP: _ClassVar[ImageKind]
    IMAGE_KIND_LOGO: _ClassVar[ImageKind]
    IMAGE_KIND_THUMB: _ClassVar[ImageKind]
    IMAGE_KIND_BANNER: _ClassVar[ImageKind]
    IMAGE_KIND_ART: _ClassVar[ImageKind]
    IMAGE_KIND_DISC: _ClassVar[ImageKind]
    IMAGE_KIND_SCREENSHOT: _ClassVar[ImageKind]
MEDIA_KIND_UNSPECIFIED: MediaKind
MEDIA_KIND_MOVIE: MediaKind
MEDIA_KIND_SERIES: MediaKind
MEDIA_KIND_SEASON: MediaKind
MEDIA_KIND_EPISODE: MediaKind
MEDIA_KIND_MUSIC_ARTIST: MediaKind
MEDIA_KIND_MUSIC_ALBUM: MediaKind
MEDIA_KIND_TRACK: MediaKind
MEDIA_KIND_MUSIC_VIDEO: MediaKind
MEDIA_KIND_BOOK: MediaKind
MEDIA_KIND_AUDIOBOOK: MediaKind
MEDIA_KIND_PERSON: MediaKind
MEDIA_KIND_VIDEO: MediaKind
MEDIA_KIND_PHOTO_ALBUM: MediaKind
MEDIA_KIND_PHOTO: MediaKind
SERIES_STATUS_UNSPECIFIED: SeriesStatus
SERIES_STATUS_CONTINUING: SeriesStatus
SERIES_STATUS_ENDED: SeriesStatus
SERIES_STATUS_UNRELEASED: SeriesStatus
PRODUCTION_STATUS_UNSPECIFIED: ProductionStatus
PRODUCTION_STATUS_RUMORED: ProductionStatus
PRODUCTION_STATUS_PLANNED: ProductionStatus
PRODUCTION_STATUS_IN_PRODUCTION: ProductionStatus
PRODUCTION_STATUS_POST_PRODUCTION: ProductionStatus
PRODUCTION_STATUS_RELEASED: ProductionStatus
PRODUCTION_STATUS_CANCELED: ProductionStatus
PRODUCTION_STATUS_PILOT: ProductionStatus
PRODUCTION_STATUS_RETURNING: ProductionStatus
PRODUCTION_STATUS_ENDED: ProductionStatus
CREDIT_KIND_UNSPECIFIED: CreditKind
CREDIT_KIND_ACTOR: CreditKind
CREDIT_KIND_GUEST_STAR: CreditKind
CREDIT_KIND_DIRECTOR: CreditKind
CREDIT_KIND_WRITER: CreditKind
CREDIT_KIND_PRODUCER: CreditKind
CREDIT_KIND_CREATOR: CreditKind
CREDIT_KIND_COMPOSER: CreditKind
CREDIT_KIND_CONDUCTOR: CreditKind
CREDIT_KIND_LYRICIST: CreditKind
CREDIT_KIND_ARTIST: CreditKind
CREDIT_KIND_AUTHOR: CreditKind
CREDIT_KIND_NARRATOR: CreditKind
CREDIT_KIND_OTHER: CreditKind
IMAGE_KIND_UNSPECIFIED: ImageKind
IMAGE_KIND_PRIMARY: ImageKind
IMAGE_KIND_BACKDROP: ImageKind
IMAGE_KIND_LOGO: ImageKind
IMAGE_KIND_THUMB: ImageKind
IMAGE_KIND_BANNER: ImageKind
IMAGE_KIND_ART: ImageKind
IMAGE_KIND_DISC: ImageKind
IMAGE_KIND_SCREENSHOT: ImageKind

class Lookup(_message.Message):
    __slots__ = ("kind", "name", "year", "external_ids", "series_external_ids", "season_number", "episode_number", "album", "artists", "language", "country", "display_order", "episode_number_end", "all_credits")
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
    NAME_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    SERIES_EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    EPISODE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    ALBUM_FIELD_NUMBER: _ClassVar[int]
    ARTISTS_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    COUNTRY_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_ORDER_FIELD_NUMBER: _ClassVar[int]
    EPISODE_NUMBER_END_FIELD_NUMBER: _ClassVar[int]
    ALL_CREDITS_FIELD_NUMBER: _ClassVar[int]
    kind: MediaKind
    name: str
    year: int
    external_ids: _containers.ScalarMap[str, str]
    series_external_ids: _containers.ScalarMap[str, str]
    season_number: int
    episode_number: int
    album: str
    artists: _containers.RepeatedScalarFieldContainer[str]
    language: str
    country: str
    display_order: str
    episode_number_end: int
    all_credits: bool
    def __init__(self, kind: _Optional[_Union[MediaKind, str]] = ..., name: _Optional[str] = ..., year: _Optional[int] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., series_external_ids: _Optional[_Mapping[str, str]] = ..., season_number: _Optional[int] = ..., episode_number: _Optional[int] = ..., album: _Optional[str] = ..., artists: _Optional[_Iterable[str]] = ..., language: _Optional[str] = ..., country: _Optional[str] = ..., display_order: _Optional[str] = ..., episode_number_end: _Optional[int] = ..., all_credits: _Optional[bool] = ...) -> None: ...

class SearchRequest(_message.Message):
    __slots__ = ("lookup", "limit")
    LOOKUP_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    lookup: Lookup
    limit: int
    def __init__(self, lookup: _Optional[_Union[Lookup, _Mapping]] = ..., limit: _Optional[int] = ...) -> None: ...

class SearchResponse(_message.Message):
    __slots__ = ("results",)
    RESULTS_FIELD_NUMBER: _ClassVar[int]
    results: _containers.RepeatedCompositeFieldContainer[SearchResult]
    def __init__(self, results: _Optional[_Iterable[_Union[SearchResult, _Mapping]]] = ...) -> None: ...

class SearchResult(_message.Message):
    __slots__ = ("name", "year", "overview", "external_ids", "image_url", "score")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    OVERVIEW_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    IMAGE_URL_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    name: str
    year: int
    overview: str
    external_ids: _containers.ScalarMap[str, str]
    image_url: str
    score: float
    def __init__(self, name: _Optional[str] = ..., year: _Optional[int] = ..., overview: _Optional[str] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., image_url: _Optional[str] = ..., score: _Optional[float] = ...) -> None: ...

class GetMetadataRequest(_message.Message):
    __slots__ = ("lookup",)
    LOOKUP_FIELD_NUMBER: _ClassVar[int]
    lookup: Lookup
    def __init__(self, lookup: _Optional[_Union[Lookup, _Mapping]] = ...) -> None: ...

class GetMetadataResponse(_message.Message):
    __slots__ = ("found", "metadata")
    FOUND_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    found: bool
    metadata: Metadata
    def __init__(self, found: _Optional[bool] = ..., metadata: _Optional[_Union[Metadata, _Mapping]] = ...) -> None: ...

class Metadata(_message.Message):
    __slots__ = ("name", "original_title", "sort_name", "overview", "tagline", "production_year", "premiere_date", "end_date", "runtime", "official_rating", "community_rating", "critic_rating", "genres", "tags", "studios", "external_ids", "series_status", "artists", "album_artists", "people", "images", "birth_date", "death_date", "birth_place", "collection_name", "original_language", "digital_release_date", "physical_release_date", "alternative_titles", "translated_titles", "secondary_year", "production_locations", "remote_trailers", "budget", "revenue", "next_air_date", "watch_providers", "production_status", "theatrical_release_date", "production_countries", "genre_refs", "keyword_refs", "studio_refs", "vote_count")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_TITLE_FIELD_NUMBER: _ClassVar[int]
    SORT_NAME_FIELD_NUMBER: _ClassVar[int]
    OVERVIEW_FIELD_NUMBER: _ClassVar[int]
    TAGLINE_FIELD_NUMBER: _ClassVar[int]
    PRODUCTION_YEAR_FIELD_NUMBER: _ClassVar[int]
    PREMIERE_DATE_FIELD_NUMBER: _ClassVar[int]
    END_DATE_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_FIELD_NUMBER: _ClassVar[int]
    OFFICIAL_RATING_FIELD_NUMBER: _ClassVar[int]
    COMMUNITY_RATING_FIELD_NUMBER: _ClassVar[int]
    CRITIC_RATING_FIELD_NUMBER: _ClassVar[int]
    GENRES_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    STUDIOS_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    SERIES_STATUS_FIELD_NUMBER: _ClassVar[int]
    ARTISTS_FIELD_NUMBER: _ClassVar[int]
    ALBUM_ARTISTS_FIELD_NUMBER: _ClassVar[int]
    PEOPLE_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    BIRTH_DATE_FIELD_NUMBER: _ClassVar[int]
    DEATH_DATE_FIELD_NUMBER: _ClassVar[int]
    BIRTH_PLACE_FIELD_NUMBER: _ClassVar[int]
    COLLECTION_NAME_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    DIGITAL_RELEASE_DATE_FIELD_NUMBER: _ClassVar[int]
    PHYSICAL_RELEASE_DATE_FIELD_NUMBER: _ClassVar[int]
    ALTERNATIVE_TITLES_FIELD_NUMBER: _ClassVar[int]
    TRANSLATED_TITLES_FIELD_NUMBER: _ClassVar[int]
    SECONDARY_YEAR_FIELD_NUMBER: _ClassVar[int]
    PRODUCTION_LOCATIONS_FIELD_NUMBER: _ClassVar[int]
    REMOTE_TRAILERS_FIELD_NUMBER: _ClassVar[int]
    BUDGET_FIELD_NUMBER: _ClassVar[int]
    REVENUE_FIELD_NUMBER: _ClassVar[int]
    NEXT_AIR_DATE_FIELD_NUMBER: _ClassVar[int]
    WATCH_PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    PRODUCTION_STATUS_FIELD_NUMBER: _ClassVar[int]
    THEATRICAL_RELEASE_DATE_FIELD_NUMBER: _ClassVar[int]
    PRODUCTION_COUNTRIES_FIELD_NUMBER: _ClassVar[int]
    GENRE_REFS_FIELD_NUMBER: _ClassVar[int]
    KEYWORD_REFS_FIELD_NUMBER: _ClassVar[int]
    STUDIO_REFS_FIELD_NUMBER: _ClassVar[int]
    VOTE_COUNT_FIELD_NUMBER: _ClassVar[int]
    name: str
    original_title: str
    sort_name: str
    overview: str
    tagline: str
    production_year: int
    premiere_date: str
    end_date: str
    runtime: _duration_pb2.Duration
    official_rating: str
    community_rating: float
    critic_rating: float
    genres: _containers.RepeatedScalarFieldContainer[str]
    tags: _containers.RepeatedScalarFieldContainer[str]
    studios: _containers.RepeatedScalarFieldContainer[str]
    external_ids: _containers.ScalarMap[str, str]
    series_status: SeriesStatus
    artists: _containers.RepeatedScalarFieldContainer[str]
    album_artists: _containers.RepeatedScalarFieldContainer[str]
    people: _containers.RepeatedCompositeFieldContainer[PersonCredit]
    images: _containers.RepeatedCompositeFieldContainer[RemoteImage]
    birth_date: str
    death_date: str
    birth_place: str
    collection_name: str
    original_language: str
    digital_release_date: str
    physical_release_date: str
    alternative_titles: _containers.RepeatedScalarFieldContainer[str]
    translated_titles: _containers.RepeatedScalarFieldContainer[str]
    secondary_year: int
    production_locations: _containers.RepeatedScalarFieldContainer[str]
    remote_trailers: _containers.RepeatedScalarFieldContainer[str]
    budget: int
    revenue: int
    next_air_date: str
    watch_providers: _containers.RepeatedCompositeFieldContainer[WatchProvider]
    production_status: ProductionStatus
    theatrical_release_date: str
    production_countries: _containers.RepeatedScalarFieldContainer[str]
    genre_refs: _containers.RepeatedCompositeFieldContainer[Reference]
    keyword_refs: _containers.RepeatedCompositeFieldContainer[Reference]
    studio_refs: _containers.RepeatedCompositeFieldContainer[Reference]
    vote_count: int
    def __init__(self, name: _Optional[str] = ..., original_title: _Optional[str] = ..., sort_name: _Optional[str] = ..., overview: _Optional[str] = ..., tagline: _Optional[str] = ..., production_year: _Optional[int] = ..., premiere_date: _Optional[str] = ..., end_date: _Optional[str] = ..., runtime: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., official_rating: _Optional[str] = ..., community_rating: _Optional[float] = ..., critic_rating: _Optional[float] = ..., genres: _Optional[_Iterable[str]] = ..., tags: _Optional[_Iterable[str]] = ..., studios: _Optional[_Iterable[str]] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., series_status: _Optional[_Union[SeriesStatus, str]] = ..., artists: _Optional[_Iterable[str]] = ..., album_artists: _Optional[_Iterable[str]] = ..., people: _Optional[_Iterable[_Union[PersonCredit, _Mapping]]] = ..., images: _Optional[_Iterable[_Union[RemoteImage, _Mapping]]] = ..., birth_date: _Optional[str] = ..., death_date: _Optional[str] = ..., birth_place: _Optional[str] = ..., collection_name: _Optional[str] = ..., original_language: _Optional[str] = ..., digital_release_date: _Optional[str] = ..., physical_release_date: _Optional[str] = ..., alternative_titles: _Optional[_Iterable[str]] = ..., translated_titles: _Optional[_Iterable[str]] = ..., secondary_year: _Optional[int] = ..., production_locations: _Optional[_Iterable[str]] = ..., remote_trailers: _Optional[_Iterable[str]] = ..., budget: _Optional[int] = ..., revenue: _Optional[int] = ..., next_air_date: _Optional[str] = ..., watch_providers: _Optional[_Iterable[_Union[WatchProvider, _Mapping]]] = ..., production_status: _Optional[_Union[ProductionStatus, str]] = ..., theatrical_release_date: _Optional[str] = ..., production_countries: _Optional[_Iterable[str]] = ..., genre_refs: _Optional[_Iterable[_Union[Reference, _Mapping]]] = ..., keyword_refs: _Optional[_Iterable[_Union[Reference, _Mapping]]] = ..., studio_refs: _Optional[_Iterable[_Union[Reference, _Mapping]]] = ..., vote_count: _Optional[int] = ...) -> None: ...

class Reference(_message.Message):
    __slots__ = ("id", "name")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ...) -> None: ...

class WatchProvider(_message.Message):
    __slots__ = ("name", "logo_url")
    NAME_FIELD_NUMBER: _ClassVar[int]
    LOGO_URL_FIELD_NUMBER: _ClassVar[int]
    name: str
    logo_url: str
    def __init__(self, name: _Optional[str] = ..., logo_url: _Optional[str] = ...) -> None: ...

class PersonCredit(_message.Message):
    __slots__ = ("name", "kind", "role", "order", "external_ids", "image_url")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    ORDER_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    IMAGE_URL_FIELD_NUMBER: _ClassVar[int]
    name: str
    kind: CreditKind
    role: str
    order: int
    external_ids: _containers.ScalarMap[str, str]
    image_url: str
    def __init__(self, name: _Optional[str] = ..., kind: _Optional[_Union[CreditKind, str]] = ..., role: _Optional[str] = ..., order: _Optional[int] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., image_url: _Optional[str] = ...) -> None: ...

class RemoteImage(_message.Message):
    __slots__ = ("kind", "url", "width", "height", "language", "score")
    KIND_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    kind: ImageKind
    url: str
    width: int
    height: int
    language: str
    score: float
    def __init__(self, kind: _Optional[_Union[ImageKind, str]] = ..., url: _Optional[str] = ..., width: _Optional[int] = ..., height: _Optional[int] = ..., language: _Optional[str] = ..., score: _Optional[float] = ...) -> None: ...

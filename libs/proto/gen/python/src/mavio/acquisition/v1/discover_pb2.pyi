from buf.validate import validate_pb2 as _validate_pb2
from mavio.acquisition.v1 import profile_pb2 as _profile_pb2
from mavio.acquisition.v1 import request_pb2 as _request_pb2
from mavio.library.v1 import item_pb2 as _item_pb2
from mavio.plugin.v1 import discovery_pb2 as _discovery_pb2
from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from mavio.plugin.v1 import ratings_pb2 as _ratings_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class MediaStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MEDIA_STATUS_UNSPECIFIED: _ClassVar[MediaStatus]
    MEDIA_STATUS_REQUESTED: _ClassVar[MediaStatus]
    MEDIA_STATUS_WANTED: _ClassVar[MediaStatus]
    MEDIA_STATUS_PARTIALLY_AVAILABLE: _ClassVar[MediaStatus]
    MEDIA_STATUS_AVAILABLE: _ClassVar[MediaStatus]
    MEDIA_STATUS_BLOCKLISTED: _ClassVar[MediaStatus]
    MEDIA_STATUS_DELETED: _ClassVar[MediaStatus]
MEDIA_STATUS_UNSPECIFIED: MediaStatus
MEDIA_STATUS_REQUESTED: MediaStatus
MEDIA_STATUS_WANTED: MediaStatus
MEDIA_STATUS_PARTIALLY_AVAILABLE: MediaStatus
MEDIA_STATUS_AVAILABLE: MediaStatus
MEDIA_STATUS_BLOCKLISTED: MediaStatus
MEDIA_STATUS_DELETED: MediaStatus

class ListCatalogsRequest(_message.Message):
    __slots__ = ("language",)
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    language: str
    def __init__(self, language: _Optional[str] = ...) -> None: ...

class ListCatalogsResponse(_message.Message):
    __slots__ = ("catalogs",)
    CATALOGS_FIELD_NUMBER: _ClassVar[int]
    catalogs: _containers.RepeatedCompositeFieldContainer[Catalog]
    def __init__(self, catalogs: _Optional[_Iterable[_Union[Catalog, _Mapping]]] = ...) -> None: ...

class Catalog(_message.Message):
    __slots__ = ("id", "name", "kind", "media_kinds", "mixed", "filters", "sorts", "plugin_id")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    MEDIA_KINDS_FIELD_NUMBER: _ClassVar[int]
    MIXED_FIELD_NUMBER: _ClassVar[int]
    FILTERS_FIELD_NUMBER: _ClassVar[int]
    SORTS_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    kind: _discovery_pb2.CatalogKind
    media_kinds: _containers.RepeatedScalarFieldContainer[_profile_pb2.MediaKind]
    mixed: bool
    filters: _containers.RepeatedScalarFieldContainer[_discovery_pb2.DiscoverFilter]
    sorts: _containers.RepeatedScalarFieldContainer[_discovery_pb2.DiscoverSort]
    plugin_id: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., kind: _Optional[_Union[_discovery_pb2.CatalogKind, str]] = ..., media_kinds: _Optional[_Iterable[_Union[_profile_pb2.MediaKind, str]]] = ..., mixed: _Optional[bool] = ..., filters: _Optional[_Iterable[_Union[_discovery_pb2.DiscoverFilter, str]]] = ..., sorts: _Optional[_Iterable[_Union[_discovery_pb2.DiscoverSort, str]]] = ..., plugin_id: _Optional[str] = ...) -> None: ...

class DiscoverRequest(_message.Message):
    __slots__ = ("catalog", "media_kind", "tmdb_id", "genres", "studios", "networks", "keywords", "excluded_keywords", "watch_providers", "watch_region", "original_language", "released_from", "released_to", "runtime_min", "runtime_max", "rating_min", "rating_max", "votes_min", "certifications", "sort", "ascending", "language", "page", "tmdb_collection_id")
    CATALOG_FIELD_NUMBER: _ClassVar[int]
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    GENRES_FIELD_NUMBER: _ClassVar[int]
    STUDIOS_FIELD_NUMBER: _ClassVar[int]
    NETWORKS_FIELD_NUMBER: _ClassVar[int]
    KEYWORDS_FIELD_NUMBER: _ClassVar[int]
    EXCLUDED_KEYWORDS_FIELD_NUMBER: _ClassVar[int]
    WATCH_PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    WATCH_REGION_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    RELEASED_FROM_FIELD_NUMBER: _ClassVar[int]
    RELEASED_TO_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_MIN_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_MAX_FIELD_NUMBER: _ClassVar[int]
    RATING_MIN_FIELD_NUMBER: _ClassVar[int]
    RATING_MAX_FIELD_NUMBER: _ClassVar[int]
    VOTES_MIN_FIELD_NUMBER: _ClassVar[int]
    CERTIFICATIONS_FIELD_NUMBER: _ClassVar[int]
    SORT_FIELD_NUMBER: _ClassVar[int]
    ASCENDING_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    TMDB_COLLECTION_ID_FIELD_NUMBER: _ClassVar[int]
    catalog: str
    media_kind: _profile_pb2.MediaKind
    tmdb_id: int
    genres: _containers.RepeatedScalarFieldContainer[str]
    studios: _containers.RepeatedScalarFieldContainer[str]
    networks: _containers.RepeatedScalarFieldContainer[str]
    keywords: _containers.RepeatedScalarFieldContainer[str]
    excluded_keywords: _containers.RepeatedScalarFieldContainer[str]
    watch_providers: _containers.RepeatedScalarFieldContainer[str]
    watch_region: str
    original_language: str
    released_from: str
    released_to: str
    runtime_min: int
    runtime_max: int
    rating_min: float
    rating_max: float
    votes_min: int
    certifications: _containers.RepeatedScalarFieldContainer[str]
    sort: _discovery_pb2.DiscoverSort
    ascending: bool
    language: str
    page: int
    tmdb_collection_id: int
    def __init__(self, catalog: _Optional[str] = ..., media_kind: _Optional[_Union[_profile_pb2.MediaKind, str]] = ..., tmdb_id: _Optional[int] = ..., genres: _Optional[_Iterable[str]] = ..., studios: _Optional[_Iterable[str]] = ..., networks: _Optional[_Iterable[str]] = ..., keywords: _Optional[_Iterable[str]] = ..., excluded_keywords: _Optional[_Iterable[str]] = ..., watch_providers: _Optional[_Iterable[str]] = ..., watch_region: _Optional[str] = ..., original_language: _Optional[str] = ..., released_from: _Optional[str] = ..., released_to: _Optional[str] = ..., runtime_min: _Optional[int] = ..., runtime_max: _Optional[int] = ..., rating_min: _Optional[float] = ..., rating_max: _Optional[float] = ..., votes_min: _Optional[int] = ..., certifications: _Optional[_Iterable[str]] = ..., sort: _Optional[_Union[_discovery_pb2.DiscoverSort, str]] = ..., ascending: _Optional[bool] = ..., language: _Optional[str] = ..., page: _Optional[int] = ..., tmdb_collection_id: _Optional[int] = ...) -> None: ...

class DiscoverResponse(_message.Message):
    __slots__ = ("titles", "page", "total_pages", "total_results", "hidden_results")
    TITLES_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    TOTAL_PAGES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_RESULTS_FIELD_NUMBER: _ClassVar[int]
    HIDDEN_RESULTS_FIELD_NUMBER: _ClassVar[int]
    titles: _containers.RepeatedCompositeFieldContainer[DiscoveredTitle]
    page: int
    total_pages: int
    total_results: int
    hidden_results: int
    def __init__(self, titles: _Optional[_Iterable[_Union[DiscoveredTitle, _Mapping]]] = ..., page: _Optional[int] = ..., total_pages: _Optional[int] = ..., total_results: _Optional[int] = ..., hidden_results: _Optional[int] = ...) -> None: ...

class DiscoveredTitle(_message.Message):
    __slots__ = ("media_kind", "name", "original_title", "original_language", "overview", "premiere_date", "year", "external_ids", "genres", "poster_url", "backdrop_url", "community_rating", "vote_count", "popularity", "status", "item_id", "request_id", "request_status", "requestable")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_TITLE_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    OVERVIEW_FIELD_NUMBER: _ClassVar[int]
    PREMIERE_DATE_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    GENRES_FIELD_NUMBER: _ClassVar[int]
    POSTER_URL_FIELD_NUMBER: _ClassVar[int]
    BACKDROP_URL_FIELD_NUMBER: _ClassVar[int]
    COMMUNITY_RATING_FIELD_NUMBER: _ClassVar[int]
    VOTE_COUNT_FIELD_NUMBER: _ClassVar[int]
    POPULARITY_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    REQUEST_STATUS_FIELD_NUMBER: _ClassVar[int]
    REQUESTABLE_FIELD_NUMBER: _ClassVar[int]
    media_kind: _profile_pb2.MediaKind
    name: str
    original_title: str
    original_language: str
    overview: str
    premiere_date: str
    year: int
    external_ids: _containers.ScalarMap[str, str]
    genres: _containers.RepeatedScalarFieldContainer[str]
    poster_url: str
    backdrop_url: str
    community_rating: float
    vote_count: int
    popularity: float
    status: MediaStatus
    item_id: str
    request_id: str
    request_status: _request_pb2.RequestStatus
    requestable: bool
    def __init__(self, media_kind: _Optional[_Union[_profile_pb2.MediaKind, str]] = ..., name: _Optional[str] = ..., original_title: _Optional[str] = ..., original_language: _Optional[str] = ..., overview: _Optional[str] = ..., premiere_date: _Optional[str] = ..., year: _Optional[int] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., genres: _Optional[_Iterable[str]] = ..., poster_url: _Optional[str] = ..., backdrop_url: _Optional[str] = ..., community_rating: _Optional[float] = ..., vote_count: _Optional[int] = ..., popularity: _Optional[float] = ..., status: _Optional[_Union[MediaStatus, str]] = ..., item_id: _Optional[str] = ..., request_id: _Optional[str] = ..., request_status: _Optional[_Union[_request_pb2.RequestStatus, str]] = ..., requestable: _Optional[bool] = ...) -> None: ...

class ListFilterValuesRequest(_message.Message):
    __slots__ = ("catalog", "filter", "media_kind", "query", "ids", "language", "region")
    CATALOG_FIELD_NUMBER: _ClassVar[int]
    FILTER_FIELD_NUMBER: _ClassVar[int]
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    IDS_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    REGION_FIELD_NUMBER: _ClassVar[int]
    catalog: str
    filter: _discovery_pb2.DiscoverFilter
    media_kind: _profile_pb2.MediaKind
    query: str
    ids: _containers.RepeatedScalarFieldContainer[str]
    language: str
    region: str
    def __init__(self, catalog: _Optional[str] = ..., filter: _Optional[_Union[_discovery_pb2.DiscoverFilter, str]] = ..., media_kind: _Optional[_Union[_profile_pb2.MediaKind, str]] = ..., query: _Optional[str] = ..., ids: _Optional[_Iterable[str]] = ..., language: _Optional[str] = ..., region: _Optional[str] = ...) -> None: ...

class ListFilterValuesResponse(_message.Message):
    __slots__ = ("values",)
    VALUES_FIELD_NUMBER: _ClassVar[int]
    values: _containers.RepeatedCompositeFieldContainer[_discovery_pb2.FilterValue]
    def __init__(self, values: _Optional[_Iterable[_Union[_discovery_pb2.FilterValue, _Mapping]]] = ...) -> None: ...

class SearchTitlesRequest(_message.Message):
    __slots__ = ("query", "media_kind", "year", "language", "page", "plugin_id")
    QUERY_FIELD_NUMBER: _ClassVar[int]
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    query: str
    media_kind: _profile_pb2.MediaKind
    year: int
    language: str
    page: int
    plugin_id: str
    def __init__(self, query: _Optional[str] = ..., media_kind: _Optional[_Union[_profile_pb2.MediaKind, str]] = ..., year: _Optional[int] = ..., language: _Optional[str] = ..., page: _Optional[int] = ..., plugin_id: _Optional[str] = ...) -> None: ...

class SearchTitlesResponse(_message.Message):
    __slots__ = ("titles", "page", "total_pages", "total_results", "hidden_results", "plugin_id")
    TITLES_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    TOTAL_PAGES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_RESULTS_FIELD_NUMBER: _ClassVar[int]
    HIDDEN_RESULTS_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    titles: _containers.RepeatedCompositeFieldContainer[DiscoveredTitle]
    page: int
    total_pages: int
    total_results: int
    hidden_results: int
    plugin_id: str
    def __init__(self, titles: _Optional[_Iterable[_Union[DiscoveredTitle, _Mapping]]] = ..., page: _Optional[int] = ..., total_pages: _Optional[int] = ..., total_results: _Optional[int] = ..., hidden_results: _Optional[int] = ..., plugin_id: _Optional[str] = ...) -> None: ...

class GetTitleRequest(_message.Message):
    __slots__ = ("media_kind", "tmdb_id", "language")
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    media_kind: _profile_pb2.MediaKind
    tmdb_id: int
    language: str
    def __init__(self, media_kind: _Optional[_Union[_profile_pb2.MediaKind, str]] = ..., tmdb_id: _Optional[int] = ..., language: _Optional[str] = ...) -> None: ...

class GetTitleResponse(_message.Message):
    __slots__ = ("title",)
    TITLE_FIELD_NUMBER: _ClassVar[int]
    title: Title
    def __init__(self, title: _Optional[_Union[Title, _Mapping]] = ...) -> None: ...

class GetSeasonRequest(_message.Message):
    __slots__ = ("tmdb_id", "season_number", "language")
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    tmdb_id: int
    season_number: int
    language: str
    def __init__(self, tmdb_id: _Optional[int] = ..., season_number: _Optional[int] = ..., language: _Optional[str] = ...) -> None: ...

class GetSeasonResponse(_message.Message):
    __slots__ = ("episodes",)
    EPISODES_FIELD_NUMBER: _ClassVar[int]
    episodes: _containers.RepeatedCompositeFieldContainer[SeasonEpisode]
    def __init__(self, episodes: _Optional[_Iterable[_Union[SeasonEpisode, _Mapping]]] = ...) -> None: ...

class SeasonEpisode(_message.Message):
    __slots__ = ("episode_number", "name", "overview", "air_date", "still_url")
    EPISODE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    OVERVIEW_FIELD_NUMBER: _ClassVar[int]
    AIR_DATE_FIELD_NUMBER: _ClassVar[int]
    STILL_URL_FIELD_NUMBER: _ClassVar[int]
    episode_number: int
    name: str
    overview: str
    air_date: str
    still_url: str
    def __init__(self, episode_number: _Optional[int] = ..., name: _Optional[str] = ..., overview: _Optional[str] = ..., air_date: _Optional[str] = ..., still_url: _Optional[str] = ...) -> None: ...

class GetPersonRequest(_message.Message):
    __slots__ = ("tmdb_id", "language")
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    tmdb_id: int
    language: str
    def __init__(self, tmdb_id: _Optional[int] = ..., language: _Optional[str] = ...) -> None: ...

class GetPersonResponse(_message.Message):
    __slots__ = ("person",)
    PERSON_FIELD_NUMBER: _ClassVar[int]
    person: Person
    def __init__(self, person: _Optional[_Union[Person, _Mapping]] = ...) -> None: ...

class Person(_message.Message):
    __slots__ = ("name", "biography", "birthday", "deathday", "place_of_birth", "also_known_as", "known_for_department", "image_url", "external_ids", "cast", "crew", "hidden_results")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    BIOGRAPHY_FIELD_NUMBER: _ClassVar[int]
    BIRTHDAY_FIELD_NUMBER: _ClassVar[int]
    DEATHDAY_FIELD_NUMBER: _ClassVar[int]
    PLACE_OF_BIRTH_FIELD_NUMBER: _ClassVar[int]
    ALSO_KNOWN_AS_FIELD_NUMBER: _ClassVar[int]
    KNOWN_FOR_DEPARTMENT_FIELD_NUMBER: _ClassVar[int]
    IMAGE_URL_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    CAST_FIELD_NUMBER: _ClassVar[int]
    CREW_FIELD_NUMBER: _ClassVar[int]
    HIDDEN_RESULTS_FIELD_NUMBER: _ClassVar[int]
    name: str
    biography: str
    birthday: str
    deathday: str
    place_of_birth: str
    also_known_as: _containers.RepeatedScalarFieldContainer[str]
    known_for_department: str
    image_url: str
    external_ids: _containers.ScalarMap[str, str]
    cast: _containers.RepeatedCompositeFieldContainer[PersonTitle]
    crew: _containers.RepeatedCompositeFieldContainer[PersonTitle]
    hidden_results: int
    def __init__(self, name: _Optional[str] = ..., biography: _Optional[str] = ..., birthday: _Optional[str] = ..., deathday: _Optional[str] = ..., place_of_birth: _Optional[str] = ..., also_known_as: _Optional[_Iterable[str]] = ..., known_for_department: _Optional[str] = ..., image_url: _Optional[str] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., cast: _Optional[_Iterable[_Union[PersonTitle, _Mapping]]] = ..., crew: _Optional[_Iterable[_Union[PersonTitle, _Mapping]]] = ..., hidden_results: _Optional[int] = ...) -> None: ...

class PersonTitle(_message.Message):
    __slots__ = ("title", "role")
    TITLE_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    title: DiscoveredTitle
    role: str
    def __init__(self, title: _Optional[_Union[DiscoveredTitle, _Mapping]] = ..., role: _Optional[str] = ...) -> None: ...

class GetCollectionRequest(_message.Message):
    __slots__ = ("tmdb_collection_id", "language")
    TMDB_COLLECTION_ID_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    tmdb_collection_id: int
    language: str
    def __init__(self, tmdb_collection_id: _Optional[int] = ..., language: _Optional[str] = ...) -> None: ...

class GetCollectionResponse(_message.Message):
    __slots__ = ("collection",)
    COLLECTION_FIELD_NUMBER: _ClassVar[int]
    collection: Collection
    def __init__(self, collection: _Optional[_Union[Collection, _Mapping]] = ...) -> None: ...

class GetRatingsRequest(_message.Message):
    __slots__ = ("media_kind", "tmdb_id")
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    media_kind: _profile_pb2.MediaKind
    tmdb_id: int
    def __init__(self, media_kind: _Optional[_Union[_profile_pb2.MediaKind, str]] = ..., tmdb_id: _Optional[int] = ...) -> None: ...

class GetRatingsResponse(_message.Message):
    __slots__ = ("ratings",)
    RATINGS_FIELD_NUMBER: _ClassVar[int]
    ratings: _containers.RepeatedCompositeFieldContainer[_ratings_pb2.Rating]
    def __init__(self, ratings: _Optional[_Iterable[_Union[_ratings_pb2.Rating, _Mapping]]] = ...) -> None: ...

class Collection(_message.Message):
    __slots__ = ("name", "overview", "poster_url", "backdrop_url", "external_ids", "parts", "hidden_results")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    OVERVIEW_FIELD_NUMBER: _ClassVar[int]
    POSTER_URL_FIELD_NUMBER: _ClassVar[int]
    BACKDROP_URL_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    PARTS_FIELD_NUMBER: _ClassVar[int]
    HIDDEN_RESULTS_FIELD_NUMBER: _ClassVar[int]
    name: str
    overview: str
    poster_url: str
    backdrop_url: str
    external_ids: _containers.ScalarMap[str, str]
    parts: _containers.RepeatedCompositeFieldContainer[DiscoveredTitle]
    hidden_results: int
    def __init__(self, name: _Optional[str] = ..., overview: _Optional[str] = ..., poster_url: _Optional[str] = ..., backdrop_url: _Optional[str] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., parts: _Optional[_Iterable[_Union[DiscoveredTitle, _Mapping]]] = ..., hidden_results: _Optional[int] = ...) -> None: ...

class Title(_message.Message):
    __slots__ = ("media_kind", "tmdb_id", "tvdb_id", "name", "original_title", "overview", "premiere_date", "year", "official_rating", "rating_country", "genres", "community_rating", "external_ids", "poster_url", "backdrop_url", "anime", "status", "item_id", "requests", "requestable", "seasons", "original_language", "studios", "production_locations", "runtime_minutes", "budget", "revenue", "digital_release_date", "physical_release_date", "end_date", "next_air_date", "watch_providers", "tagline", "keywords", "credits", "trailer_urls", "collection_name", "production_status", "theatrical_release_date", "production_countries", "genre_refs", "keyword_refs", "studio_refs", "vote_count")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    TMDB_ID_FIELD_NUMBER: _ClassVar[int]
    TVDB_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_TITLE_FIELD_NUMBER: _ClassVar[int]
    OVERVIEW_FIELD_NUMBER: _ClassVar[int]
    PREMIERE_DATE_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    OFFICIAL_RATING_FIELD_NUMBER: _ClassVar[int]
    RATING_COUNTRY_FIELD_NUMBER: _ClassVar[int]
    GENRES_FIELD_NUMBER: _ClassVar[int]
    COMMUNITY_RATING_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    POSTER_URL_FIELD_NUMBER: _ClassVar[int]
    BACKDROP_URL_FIELD_NUMBER: _ClassVar[int]
    ANIME_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    REQUESTS_FIELD_NUMBER: _ClassVar[int]
    REQUESTABLE_FIELD_NUMBER: _ClassVar[int]
    SEASONS_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    STUDIOS_FIELD_NUMBER: _ClassVar[int]
    PRODUCTION_LOCATIONS_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_MINUTES_FIELD_NUMBER: _ClassVar[int]
    BUDGET_FIELD_NUMBER: _ClassVar[int]
    REVENUE_FIELD_NUMBER: _ClassVar[int]
    DIGITAL_RELEASE_DATE_FIELD_NUMBER: _ClassVar[int]
    PHYSICAL_RELEASE_DATE_FIELD_NUMBER: _ClassVar[int]
    END_DATE_FIELD_NUMBER: _ClassVar[int]
    NEXT_AIR_DATE_FIELD_NUMBER: _ClassVar[int]
    WATCH_PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    TAGLINE_FIELD_NUMBER: _ClassVar[int]
    KEYWORDS_FIELD_NUMBER: _ClassVar[int]
    CREDITS_FIELD_NUMBER: _ClassVar[int]
    TRAILER_URLS_FIELD_NUMBER: _ClassVar[int]
    COLLECTION_NAME_FIELD_NUMBER: _ClassVar[int]
    PRODUCTION_STATUS_FIELD_NUMBER: _ClassVar[int]
    THEATRICAL_RELEASE_DATE_FIELD_NUMBER: _ClassVar[int]
    PRODUCTION_COUNTRIES_FIELD_NUMBER: _ClassVar[int]
    GENRE_REFS_FIELD_NUMBER: _ClassVar[int]
    KEYWORD_REFS_FIELD_NUMBER: _ClassVar[int]
    STUDIO_REFS_FIELD_NUMBER: _ClassVar[int]
    VOTE_COUNT_FIELD_NUMBER: _ClassVar[int]
    media_kind: _profile_pb2.MediaKind
    tmdb_id: int
    tvdb_id: int
    name: str
    original_title: str
    overview: str
    premiere_date: str
    year: int
    official_rating: str
    rating_country: str
    genres: _containers.RepeatedScalarFieldContainer[str]
    community_rating: float
    external_ids: _containers.ScalarMap[str, str]
    poster_url: str
    backdrop_url: str
    anime: bool
    status: MediaStatus
    item_id: str
    requests: _containers.RepeatedCompositeFieldContainer[_request_pb2.Request]
    requestable: bool
    seasons: _containers.RepeatedCompositeFieldContainer[SeasonStatus]
    original_language: str
    studios: _containers.RepeatedScalarFieldContainer[str]
    production_locations: _containers.RepeatedScalarFieldContainer[str]
    runtime_minutes: int
    budget: int
    revenue: int
    digital_release_date: str
    physical_release_date: str
    end_date: str
    next_air_date: str
    watch_providers: _containers.RepeatedCompositeFieldContainer[_metadata_pb2.WatchProvider]
    tagline: str
    keywords: _containers.RepeatedScalarFieldContainer[str]
    credits: _containers.RepeatedCompositeFieldContainer[TitleCredit]
    trailer_urls: _containers.RepeatedScalarFieldContainer[str]
    collection_name: str
    production_status: _metadata_pb2.ProductionStatus
    theatrical_release_date: str
    production_countries: _containers.RepeatedScalarFieldContainer[str]
    genre_refs: _containers.RepeatedCompositeFieldContainer[_metadata_pb2.Reference]
    keyword_refs: _containers.RepeatedCompositeFieldContainer[_metadata_pb2.Reference]
    studio_refs: _containers.RepeatedCompositeFieldContainer[_metadata_pb2.Reference]
    vote_count: int
    def __init__(self, media_kind: _Optional[_Union[_profile_pb2.MediaKind, str]] = ..., tmdb_id: _Optional[int] = ..., tvdb_id: _Optional[int] = ..., name: _Optional[str] = ..., original_title: _Optional[str] = ..., overview: _Optional[str] = ..., premiere_date: _Optional[str] = ..., year: _Optional[int] = ..., official_rating: _Optional[str] = ..., rating_country: _Optional[str] = ..., genres: _Optional[_Iterable[str]] = ..., community_rating: _Optional[float] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., poster_url: _Optional[str] = ..., backdrop_url: _Optional[str] = ..., anime: _Optional[bool] = ..., status: _Optional[_Union[MediaStatus, str]] = ..., item_id: _Optional[str] = ..., requests: _Optional[_Iterable[_Union[_request_pb2.Request, _Mapping]]] = ..., requestable: _Optional[bool] = ..., seasons: _Optional[_Iterable[_Union[SeasonStatus, _Mapping]]] = ..., original_language: _Optional[str] = ..., studios: _Optional[_Iterable[str]] = ..., production_locations: _Optional[_Iterable[str]] = ..., runtime_minutes: _Optional[int] = ..., budget: _Optional[int] = ..., revenue: _Optional[int] = ..., digital_release_date: _Optional[str] = ..., physical_release_date: _Optional[str] = ..., end_date: _Optional[str] = ..., next_air_date: _Optional[str] = ..., watch_providers: _Optional[_Iterable[_Union[_metadata_pb2.WatchProvider, _Mapping]]] = ..., tagline: _Optional[str] = ..., keywords: _Optional[_Iterable[str]] = ..., credits: _Optional[_Iterable[_Union[TitleCredit, _Mapping]]] = ..., trailer_urls: _Optional[_Iterable[str]] = ..., collection_name: _Optional[str] = ..., production_status: _Optional[_Union[_metadata_pb2.ProductionStatus, str]] = ..., theatrical_release_date: _Optional[str] = ..., production_countries: _Optional[_Iterable[str]] = ..., genre_refs: _Optional[_Iterable[_Union[_metadata_pb2.Reference, _Mapping]]] = ..., keyword_refs: _Optional[_Iterable[_Union[_metadata_pb2.Reference, _Mapping]]] = ..., studio_refs: _Optional[_Iterable[_Union[_metadata_pb2.Reference, _Mapping]]] = ..., vote_count: _Optional[int] = ...) -> None: ...

class TitleCredit(_message.Message):
    __slots__ = ("name", "kind", "role", "order", "image_url", "external_ids")
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
    IMAGE_URL_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    name: str
    kind: _item_pb2.CreditKind
    role: str
    order: int
    image_url: str
    external_ids: _containers.ScalarMap[str, str]
    def __init__(self, name: _Optional[str] = ..., kind: _Optional[_Union[_item_pb2.CreditKind, str]] = ..., role: _Optional[str] = ..., order: _Optional[int] = ..., image_url: _Optional[str] = ..., external_ids: _Optional[_Mapping[str, str]] = ...) -> None: ...

class SeasonStatus(_message.Message):
    __slots__ = ("season_number", "episode_count", "premiere_date", "status", "request_id", "request_status", "requestable")
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    EPISODE_COUNT_FIELD_NUMBER: _ClassVar[int]
    PREMIERE_DATE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    REQUEST_STATUS_FIELD_NUMBER: _ClassVar[int]
    REQUESTABLE_FIELD_NUMBER: _ClassVar[int]
    season_number: int
    episode_count: int
    premiere_date: str
    status: MediaStatus
    request_id: str
    request_status: _request_pb2.RequestStatus
    requestable: bool
    def __init__(self, season_number: _Optional[int] = ..., episode_count: _Optional[int] = ..., premiere_date: _Optional[str] = ..., status: _Optional[_Union[MediaStatus, str]] = ..., request_id: _Optional[str] = ..., request_status: _Optional[_Union[_request_pb2.RequestStatus, str]] = ..., requestable: _Optional[bool] = ...) -> None: ...

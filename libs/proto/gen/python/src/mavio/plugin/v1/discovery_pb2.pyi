from buf.validate import validate_pb2 as _validate_pb2
from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class CatalogKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CATALOG_KIND_UNSPECIFIED: _ClassVar[CatalogKind]
    CATALOG_KIND_TRENDING: _ClassVar[CatalogKind]
    CATALOG_KIND_POPULAR: _ClassVar[CatalogKind]
    CATALOG_KIND_UPCOMING: _ClassVar[CatalogKind]
    CATALOG_KIND_TOP_RATED: _ClassVar[CatalogKind]
    CATALOG_KIND_RECOMMENDATIONS: _ClassVar[CatalogKind]
    CATALOG_KIND_SIMILAR: _ClassVar[CatalogKind]
    CATALOG_KIND_LIST: _ClassVar[CatalogKind]
    CATALOG_KIND_COLLECTION: _ClassVar[CatalogKind]

class DiscoverFilter(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    DISCOVER_FILTER_UNSPECIFIED: _ClassVar[DiscoverFilter]
    DISCOVER_FILTER_GENRE: _ClassVar[DiscoverFilter]
    DISCOVER_FILTER_STUDIO: _ClassVar[DiscoverFilter]
    DISCOVER_FILTER_NETWORK: _ClassVar[DiscoverFilter]
    DISCOVER_FILTER_KEYWORD: _ClassVar[DiscoverFilter]
    DISCOVER_FILTER_WATCH_PROVIDER: _ClassVar[DiscoverFilter]
    DISCOVER_FILTER_ORIGINAL_LANGUAGE: _ClassVar[DiscoverFilter]
    DISCOVER_FILTER_RELEASE_DATE: _ClassVar[DiscoverFilter]
    DISCOVER_FILTER_RUNTIME: _ClassVar[DiscoverFilter]
    DISCOVER_FILTER_RATING: _ClassVar[DiscoverFilter]
    DISCOVER_FILTER_VOTE_COUNT: _ClassVar[DiscoverFilter]
    DISCOVER_FILTER_CERTIFICATION: _ClassVar[DiscoverFilter]

class DiscoverSort(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    DISCOVER_SORT_UNSPECIFIED: _ClassVar[DiscoverSort]
    DISCOVER_SORT_POPULARITY: _ClassVar[DiscoverSort]
    DISCOVER_SORT_RELEASE_DATE: _ClassVar[DiscoverSort]
    DISCOVER_SORT_RATING: _ClassVar[DiscoverSort]
    DISCOVER_SORT_VOTE_COUNT: _ClassVar[DiscoverSort]
    DISCOVER_SORT_TITLE: _ClassVar[DiscoverSort]
CATALOG_KIND_UNSPECIFIED: CatalogKind
CATALOG_KIND_TRENDING: CatalogKind
CATALOG_KIND_POPULAR: CatalogKind
CATALOG_KIND_UPCOMING: CatalogKind
CATALOG_KIND_TOP_RATED: CatalogKind
CATALOG_KIND_RECOMMENDATIONS: CatalogKind
CATALOG_KIND_SIMILAR: CatalogKind
CATALOG_KIND_LIST: CatalogKind
CATALOG_KIND_COLLECTION: CatalogKind
DISCOVER_FILTER_UNSPECIFIED: DiscoverFilter
DISCOVER_FILTER_GENRE: DiscoverFilter
DISCOVER_FILTER_STUDIO: DiscoverFilter
DISCOVER_FILTER_NETWORK: DiscoverFilter
DISCOVER_FILTER_KEYWORD: DiscoverFilter
DISCOVER_FILTER_WATCH_PROVIDER: DiscoverFilter
DISCOVER_FILTER_ORIGINAL_LANGUAGE: DiscoverFilter
DISCOVER_FILTER_RELEASE_DATE: DiscoverFilter
DISCOVER_FILTER_RUNTIME: DiscoverFilter
DISCOVER_FILTER_RATING: DiscoverFilter
DISCOVER_FILTER_VOTE_COUNT: DiscoverFilter
DISCOVER_FILTER_CERTIFICATION: DiscoverFilter
DISCOVER_SORT_UNSPECIFIED: DiscoverSort
DISCOVER_SORT_POPULARITY: DiscoverSort
DISCOVER_SORT_RELEASE_DATE: DiscoverSort
DISCOVER_SORT_RATING: DiscoverSort
DISCOVER_SORT_VOTE_COUNT: DiscoverSort
DISCOVER_SORT_TITLE: DiscoverSort

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
    __slots__ = ("id", "name", "kind", "media_kinds", "filters", "sorts")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    MEDIA_KINDS_FIELD_NUMBER: _ClassVar[int]
    FILTERS_FIELD_NUMBER: _ClassVar[int]
    SORTS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    kind: CatalogKind
    media_kinds: _containers.RepeatedScalarFieldContainer[_metadata_pb2.MediaKind]
    filters: _containers.RepeatedScalarFieldContainer[DiscoverFilter]
    sorts: _containers.RepeatedScalarFieldContainer[DiscoverSort]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., kind: _Optional[_Union[CatalogKind, str]] = ..., media_kinds: _Optional[_Iterable[_Union[_metadata_pb2.MediaKind, str]]] = ..., filters: _Optional[_Iterable[_Union[DiscoverFilter, str]]] = ..., sorts: _Optional[_Iterable[_Union[DiscoverSort, str]]] = ...) -> None: ...

class DiscoverRequest(_message.Message):
    __slots__ = ("catalog", "media_kind", "external_ids", "genres", "studios", "networks", "keywords", "excluded_keywords", "watch_providers", "watch_region", "original_language", "released_from", "released_to", "runtime_min", "runtime_max", "rating_min", "rating_max", "votes_min", "certification_country", "certifications", "sort", "ascending", "language", "region", "include_adult", "page")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    CATALOG_FIELD_NUMBER: _ClassVar[int]
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
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
    CERTIFICATION_COUNTRY_FIELD_NUMBER: _ClassVar[int]
    CERTIFICATIONS_FIELD_NUMBER: _ClassVar[int]
    SORT_FIELD_NUMBER: _ClassVar[int]
    ASCENDING_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    REGION_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_ADULT_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    catalog: str
    media_kind: _metadata_pb2.MediaKind
    external_ids: _containers.ScalarMap[str, str]
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
    certification_country: str
    certifications: _containers.RepeatedScalarFieldContainer[str]
    sort: DiscoverSort
    ascending: bool
    language: str
    region: str
    include_adult: bool
    page: int
    def __init__(self, catalog: _Optional[str] = ..., media_kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., genres: _Optional[_Iterable[str]] = ..., studios: _Optional[_Iterable[str]] = ..., networks: _Optional[_Iterable[str]] = ..., keywords: _Optional[_Iterable[str]] = ..., excluded_keywords: _Optional[_Iterable[str]] = ..., watch_providers: _Optional[_Iterable[str]] = ..., watch_region: _Optional[str] = ..., original_language: _Optional[str] = ..., released_from: _Optional[str] = ..., released_to: _Optional[str] = ..., runtime_min: _Optional[int] = ..., runtime_max: _Optional[int] = ..., rating_min: _Optional[float] = ..., rating_max: _Optional[float] = ..., votes_min: _Optional[int] = ..., certification_country: _Optional[str] = ..., certifications: _Optional[_Iterable[str]] = ..., sort: _Optional[_Union[DiscoverSort, str]] = ..., ascending: _Optional[bool] = ..., language: _Optional[str] = ..., region: _Optional[str] = ..., include_adult: _Optional[bool] = ..., page: _Optional[int] = ...) -> None: ...

class DiscoverResponse(_message.Message):
    __slots__ = ("titles", "page", "total_pages", "total_results")
    TITLES_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    TOTAL_PAGES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_RESULTS_FIELD_NUMBER: _ClassVar[int]
    titles: _containers.RepeatedCompositeFieldContainer[DiscoveredTitle]
    page: int
    total_pages: int
    total_results: int
    def __init__(self, titles: _Optional[_Iterable[_Union[DiscoveredTitle, _Mapping]]] = ..., page: _Optional[int] = ..., total_pages: _Optional[int] = ..., total_results: _Optional[int] = ...) -> None: ...

class DiscoveredTitle(_message.Message):
    __slots__ = ("media_kind", "name", "original_title", "original_language", "overview", "premiere_date", "year", "external_ids", "genres", "poster_url", "backdrop_url", "community_rating", "vote_count", "popularity", "adult")
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
    ADULT_FIELD_NUMBER: _ClassVar[int]
    media_kind: _metadata_pb2.MediaKind
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
    adult: bool
    def __init__(self, media_kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., name: _Optional[str] = ..., original_title: _Optional[str] = ..., original_language: _Optional[str] = ..., overview: _Optional[str] = ..., premiere_date: _Optional[str] = ..., year: _Optional[int] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., genres: _Optional[_Iterable[str]] = ..., poster_url: _Optional[str] = ..., backdrop_url: _Optional[str] = ..., community_rating: _Optional[float] = ..., vote_count: _Optional[int] = ..., popularity: _Optional[float] = ..., adult: _Optional[bool] = ...) -> None: ...

class ListFilterValuesRequest(_message.Message):
    __slots__ = ("filter", "media_kind", "query", "ids", "language", "region")
    FILTER_FIELD_NUMBER: _ClassVar[int]
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    IDS_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    REGION_FIELD_NUMBER: _ClassVar[int]
    filter: DiscoverFilter
    media_kind: _metadata_pb2.MediaKind
    query: str
    ids: _containers.RepeatedScalarFieldContainer[str]
    language: str
    region: str
    def __init__(self, filter: _Optional[_Union[DiscoverFilter, str]] = ..., media_kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., query: _Optional[str] = ..., ids: _Optional[_Iterable[str]] = ..., language: _Optional[str] = ..., region: _Optional[str] = ...) -> None: ...

class ListFilterValuesResponse(_message.Message):
    __slots__ = ("values",)
    VALUES_FIELD_NUMBER: _ClassVar[int]
    values: _containers.RepeatedCompositeFieldContainer[FilterValue]
    def __init__(self, values: _Optional[_Iterable[_Union[FilterValue, _Mapping]]] = ...) -> None: ...

class FilterValue(_message.Message):
    __slots__ = ("id", "name", "image_url")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    IMAGE_URL_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    image_url: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., image_url: _Optional[str] = ...) -> None: ...

class SearchTitlesRequest(_message.Message):
    __slots__ = ("query", "media_kind", "year", "language", "include_adult", "page")
    QUERY_FIELD_NUMBER: _ClassVar[int]
    MEDIA_KIND_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_ADULT_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    query: str
    media_kind: _metadata_pb2.MediaKind
    year: int
    language: str
    include_adult: bool
    page: int
    def __init__(self, query: _Optional[str] = ..., media_kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., year: _Optional[int] = ..., language: _Optional[str] = ..., include_adult: _Optional[bool] = ..., page: _Optional[int] = ...) -> None: ...

class SearchTitlesResponse(_message.Message):
    __slots__ = ("titles", "page", "total_pages", "total_results")
    TITLES_FIELD_NUMBER: _ClassVar[int]
    PAGE_FIELD_NUMBER: _ClassVar[int]
    TOTAL_PAGES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_RESULTS_FIELD_NUMBER: _ClassVar[int]
    titles: _containers.RepeatedCompositeFieldContainer[DiscoveredTitle]
    page: int
    total_pages: int
    total_results: int
    def __init__(self, titles: _Optional[_Iterable[_Union[DiscoveredTitle, _Mapping]]] = ..., page: _Optional[int] = ..., total_pages: _Optional[int] = ..., total_results: _Optional[int] = ...) -> None: ...

class GetPersonRequest(_message.Message):
    __slots__ = ("external_ids", "language", "include_adult")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_ADULT_FIELD_NUMBER: _ClassVar[int]
    external_ids: _containers.ScalarMap[str, str]
    language: str
    include_adult: bool
    def __init__(self, external_ids: _Optional[_Mapping[str, str]] = ..., language: _Optional[str] = ..., include_adult: _Optional[bool] = ...) -> None: ...

class GetPersonResponse(_message.Message):
    __slots__ = ("found", "person")
    FOUND_FIELD_NUMBER: _ClassVar[int]
    PERSON_FIELD_NUMBER: _ClassVar[int]
    found: bool
    person: Person
    def __init__(self, found: _Optional[bool] = ..., person: _Optional[_Union[Person, _Mapping]] = ...) -> None: ...

class Person(_message.Message):
    __slots__ = ("name", "biography", "birthday", "deathday", "place_of_birth", "also_known_as", "known_for_department", "image_url", "external_ids", "cast", "crew")
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
    def __init__(self, name: _Optional[str] = ..., biography: _Optional[str] = ..., birthday: _Optional[str] = ..., deathday: _Optional[str] = ..., place_of_birth: _Optional[str] = ..., also_known_as: _Optional[_Iterable[str]] = ..., known_for_department: _Optional[str] = ..., image_url: _Optional[str] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., cast: _Optional[_Iterable[_Union[PersonTitle, _Mapping]]] = ..., crew: _Optional[_Iterable[_Union[PersonTitle, _Mapping]]] = ...) -> None: ...

class PersonTitle(_message.Message):
    __slots__ = ("title", "role")
    TITLE_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    title: DiscoveredTitle
    role: str
    def __init__(self, title: _Optional[_Union[DiscoveredTitle, _Mapping]] = ..., role: _Optional[str] = ...) -> None: ...

class GetCollectionRequest(_message.Message):
    __slots__ = ("external_ids", "language", "include_adult")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_ADULT_FIELD_NUMBER: _ClassVar[int]
    external_ids: _containers.ScalarMap[str, str]
    language: str
    include_adult: bool
    def __init__(self, external_ids: _Optional[_Mapping[str, str]] = ..., language: _Optional[str] = ..., include_adult: _Optional[bool] = ...) -> None: ...

class GetCollectionResponse(_message.Message):
    __slots__ = ("found", "collection")
    FOUND_FIELD_NUMBER: _ClassVar[int]
    COLLECTION_FIELD_NUMBER: _ClassVar[int]
    found: bool
    collection: Collection
    def __init__(self, found: _Optional[bool] = ..., collection: _Optional[_Union[Collection, _Mapping]] = ...) -> None: ...

class Collection(_message.Message):
    __slots__ = ("name", "overview", "poster_url", "backdrop_url", "external_ids", "parts")
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
    name: str
    overview: str
    poster_url: str
    backdrop_url: str
    external_ids: _containers.ScalarMap[str, str]
    parts: _containers.RepeatedCompositeFieldContainer[DiscoveredTitle]
    def __init__(self, name: _Optional[str] = ..., overview: _Optional[str] = ..., poster_url: _Optional[str] = ..., backdrop_url: _Optional[str] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., parts: _Optional[_Iterable[_Union[DiscoveredTitle, _Mapping]]] = ...) -> None: ...

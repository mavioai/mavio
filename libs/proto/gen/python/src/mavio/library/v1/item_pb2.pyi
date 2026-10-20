import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.library.v1 import media_pb2 as _media_pb2
from mavio.user.v1 import user_pb2 as _user_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ItemKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ITEM_KIND_UNSPECIFIED: _ClassVar[ItemKind]
    ITEM_KIND_MOVIE: _ClassVar[ItemKind]
    ITEM_KIND_SERIES: _ClassVar[ItemKind]
    ITEM_KIND_SEASON: _ClassVar[ItemKind]
    ITEM_KIND_EPISODE: _ClassVar[ItemKind]
    ITEM_KIND_VIDEO: _ClassVar[ItemKind]
    ITEM_KIND_MUSIC_ARTIST: _ClassVar[ItemKind]
    ITEM_KIND_MUSIC_ALBUM: _ClassVar[ItemKind]
    ITEM_KIND_TRACK: _ClassVar[ItemKind]
    ITEM_KIND_MUSIC_VIDEO: _ClassVar[ItemKind]
    ITEM_KIND_AUDIOBOOK: _ClassVar[ItemKind]
    ITEM_KIND_BOOK: _ClassVar[ItemKind]
    ITEM_KIND_PHOTO_ALBUM: _ClassVar[ItemKind]
    ITEM_KIND_PHOTO: _ClassVar[ItemKind]
    ITEM_KIND_FOLDER: _ClassVar[ItemKind]
    ITEM_KIND_COLLECTION: _ClassVar[ItemKind]
    ITEM_KIND_PLAYLIST: _ClassVar[ItemKind]

class ExtraKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    EXTRA_KIND_UNSPECIFIED: _ClassVar[ExtraKind]
    EXTRA_KIND_TRAILER: _ClassVar[ExtraKind]
    EXTRA_KIND_CLIP: _ClassVar[ExtraKind]
    EXTRA_KIND_BEHIND_THE_SCENES: _ClassVar[ExtraKind]
    EXTRA_KIND_DELETED_SCENE: _ClassVar[ExtraKind]
    EXTRA_KIND_INTERVIEW: _ClassVar[ExtraKind]
    EXTRA_KIND_SCENE: _ClassVar[ExtraKind]
    EXTRA_KIND_SAMPLE: _ClassVar[ExtraKind]
    EXTRA_KIND_FEATURETTE: _ClassVar[ExtraKind]
    EXTRA_KIND_SHORT: _ClassVar[ExtraKind]
    EXTRA_KIND_THEME_SONG: _ClassVar[ExtraKind]
    EXTRA_KIND_THEME_VIDEO: _ClassVar[ExtraKind]
    EXTRA_KIND_OTHER: _ClassVar[ExtraKind]

class SeriesStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SERIES_STATUS_UNSPECIFIED: _ClassVar[SeriesStatus]
    SERIES_STATUS_CONTINUING: _ClassVar[SeriesStatus]
    SERIES_STATUS_ENDED: _ClassVar[SeriesStatus]
    SERIES_STATUS_UNRELEASED: _ClassVar[SeriesStatus]

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

class SortField(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SORT_FIELD_UNSPECIFIED: _ClassVar[SortField]
    SORT_FIELD_NAME: _ClassVar[SortField]
    SORT_FIELD_DATE_ADDED: _ClassVar[SortField]
    SORT_FIELD_PREMIERE_DATE: _ClassVar[SortField]
    SORT_FIELD_PRODUCTION_YEAR: _ClassVar[SortField]
    SORT_FIELD_COMMUNITY_RATING: _ClassVar[SortField]
    SORT_FIELD_RUNTIME: _ClassVar[SortField]
    SORT_FIELD_INDEX: _ClassVar[SortField]
    SORT_FIELD_RANDOM: _ClassVar[SortField]
    SORT_FIELD_LAST_PLAYED: _ClassVar[SortField]
    SORT_FIELD_PLAY_COUNT: _ClassVar[SortField]
    SORT_FIELD_LIST_ORDER: _ClassVar[SortField]

class MetadataField(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    METADATA_FIELD_UNSPECIFIED: _ClassVar[MetadataField]
    METADATA_FIELD_CAST: _ClassVar[MetadataField]
    METADATA_FIELD_GENRES: _ClassVar[MetadataField]
    METADATA_FIELD_PRODUCTION_LOCATIONS: _ClassVar[MetadataField]
    METADATA_FIELD_STUDIOS: _ClassVar[MetadataField]
    METADATA_FIELD_TAGS: _ClassVar[MetadataField]
    METADATA_FIELD_NAME: _ClassVar[MetadataField]
    METADATA_FIELD_OVERVIEW: _ClassVar[MetadataField]
    METADATA_FIELD_RUNTIME: _ClassVar[MetadataField]
    METADATA_FIELD_OFFICIAL_RATING: _ClassVar[MetadataField]

class ValueKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    VALUE_KIND_UNSPECIFIED: _ClassVar[ValueKind]
    VALUE_KIND_GENRE: _ClassVar[ValueKind]
    VALUE_KIND_TAG: _ClassVar[ValueKind]
    VALUE_KIND_STUDIO: _ClassVar[ValueKind]
    VALUE_KIND_ARTIST: _ClassVar[ValueKind]
    VALUE_KIND_YEAR: _ClassVar[ValueKind]

class SeriesType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SERIES_TYPE_UNSPECIFIED: _ClassVar[SeriesType]
    SERIES_TYPE_STANDARD: _ClassVar[SeriesType]
    SERIES_TYPE_DAILY: _ClassVar[SeriesType]
    SERIES_TYPE_ANIME: _ClassVar[SeriesType]

class DisplayOrder(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    DISPLAY_ORDER_UNSPECIFIED: _ClassVar[DisplayOrder]
    DISPLAY_ORDER_AIRED: _ClassVar[DisplayOrder]
    DISPLAY_ORDER_ORIGINAL_AIR_DATE: _ClassVar[DisplayOrder]
    DISPLAY_ORDER_ABSOLUTE: _ClassVar[DisplayOrder]
    DISPLAY_ORDER_DVD: _ClassVar[DisplayOrder]
    DISPLAY_ORDER_DIGITAL: _ClassVar[DisplayOrder]
    DISPLAY_ORDER_STORY_ARC: _ClassVar[DisplayOrder]
    DISPLAY_ORDER_PRODUCTION: _ClassVar[DisplayOrder]
    DISPLAY_ORDER_TV: _ClassVar[DisplayOrder]

class SegmentKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SEGMENT_KIND_UNSPECIFIED: _ClassVar[SegmentKind]
    SEGMENT_KIND_INTRO: _ClassVar[SegmentKind]
    SEGMENT_KIND_OUTRO: _ClassVar[SegmentKind]
    SEGMENT_KIND_RECAP: _ClassVar[SegmentKind]
    SEGMENT_KIND_PREVIEW: _ClassVar[SegmentKind]
    SEGMENT_KIND_COMMERCIAL: _ClassVar[SegmentKind]
ITEM_KIND_UNSPECIFIED: ItemKind
ITEM_KIND_MOVIE: ItemKind
ITEM_KIND_SERIES: ItemKind
ITEM_KIND_SEASON: ItemKind
ITEM_KIND_EPISODE: ItemKind
ITEM_KIND_VIDEO: ItemKind
ITEM_KIND_MUSIC_ARTIST: ItemKind
ITEM_KIND_MUSIC_ALBUM: ItemKind
ITEM_KIND_TRACK: ItemKind
ITEM_KIND_MUSIC_VIDEO: ItemKind
ITEM_KIND_AUDIOBOOK: ItemKind
ITEM_KIND_BOOK: ItemKind
ITEM_KIND_PHOTO_ALBUM: ItemKind
ITEM_KIND_PHOTO: ItemKind
ITEM_KIND_FOLDER: ItemKind
ITEM_KIND_COLLECTION: ItemKind
ITEM_KIND_PLAYLIST: ItemKind
EXTRA_KIND_UNSPECIFIED: ExtraKind
EXTRA_KIND_TRAILER: ExtraKind
EXTRA_KIND_CLIP: ExtraKind
EXTRA_KIND_BEHIND_THE_SCENES: ExtraKind
EXTRA_KIND_DELETED_SCENE: ExtraKind
EXTRA_KIND_INTERVIEW: ExtraKind
EXTRA_KIND_SCENE: ExtraKind
EXTRA_KIND_SAMPLE: ExtraKind
EXTRA_KIND_FEATURETTE: ExtraKind
EXTRA_KIND_SHORT: ExtraKind
EXTRA_KIND_THEME_SONG: ExtraKind
EXTRA_KIND_THEME_VIDEO: ExtraKind
EXTRA_KIND_OTHER: ExtraKind
SERIES_STATUS_UNSPECIFIED: SeriesStatus
SERIES_STATUS_CONTINUING: SeriesStatus
SERIES_STATUS_ENDED: SeriesStatus
SERIES_STATUS_UNRELEASED: SeriesStatus
IMAGE_KIND_UNSPECIFIED: ImageKind
IMAGE_KIND_PRIMARY: ImageKind
IMAGE_KIND_BACKDROP: ImageKind
IMAGE_KIND_LOGO: ImageKind
IMAGE_KIND_THUMB: ImageKind
IMAGE_KIND_BANNER: ImageKind
IMAGE_KIND_ART: ImageKind
IMAGE_KIND_DISC: ImageKind
IMAGE_KIND_SCREENSHOT: ImageKind
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
SORT_FIELD_UNSPECIFIED: SortField
SORT_FIELD_NAME: SortField
SORT_FIELD_DATE_ADDED: SortField
SORT_FIELD_PREMIERE_DATE: SortField
SORT_FIELD_PRODUCTION_YEAR: SortField
SORT_FIELD_COMMUNITY_RATING: SortField
SORT_FIELD_RUNTIME: SortField
SORT_FIELD_INDEX: SortField
SORT_FIELD_RANDOM: SortField
SORT_FIELD_LAST_PLAYED: SortField
SORT_FIELD_PLAY_COUNT: SortField
SORT_FIELD_LIST_ORDER: SortField
METADATA_FIELD_UNSPECIFIED: MetadataField
METADATA_FIELD_CAST: MetadataField
METADATA_FIELD_GENRES: MetadataField
METADATA_FIELD_PRODUCTION_LOCATIONS: MetadataField
METADATA_FIELD_STUDIOS: MetadataField
METADATA_FIELD_TAGS: MetadataField
METADATA_FIELD_NAME: MetadataField
METADATA_FIELD_OVERVIEW: MetadataField
METADATA_FIELD_RUNTIME: MetadataField
METADATA_FIELD_OFFICIAL_RATING: MetadataField
VALUE_KIND_UNSPECIFIED: ValueKind
VALUE_KIND_GENRE: ValueKind
VALUE_KIND_TAG: ValueKind
VALUE_KIND_STUDIO: ValueKind
VALUE_KIND_ARTIST: ValueKind
VALUE_KIND_YEAR: ValueKind
SERIES_TYPE_UNSPECIFIED: SeriesType
SERIES_TYPE_STANDARD: SeriesType
SERIES_TYPE_DAILY: SeriesType
SERIES_TYPE_ANIME: SeriesType
DISPLAY_ORDER_UNSPECIFIED: DisplayOrder
DISPLAY_ORDER_AIRED: DisplayOrder
DISPLAY_ORDER_ORIGINAL_AIR_DATE: DisplayOrder
DISPLAY_ORDER_ABSOLUTE: DisplayOrder
DISPLAY_ORDER_DVD: DisplayOrder
DISPLAY_ORDER_DIGITAL: DisplayOrder
DISPLAY_ORDER_STORY_ARC: DisplayOrder
DISPLAY_ORDER_PRODUCTION: DisplayOrder
DISPLAY_ORDER_TV: DisplayOrder
SEGMENT_KIND_UNSPECIFIED: SegmentKind
SEGMENT_KIND_INTRO: SegmentKind
SEGMENT_KIND_OUTRO: SegmentKind
SEGMENT_KIND_RECAP: SegmentKind
SEGMENT_KIND_PREVIEW: SegmentKind
SEGMENT_KIND_COMMERCIAL: SegmentKind

class Item(_message.Message):
    __slots__ = ("id", "library_id", "parent_id", "kind", "name", "sort_name", "original_title", "overview", "tagline", "path", "index_number", "parent_index_number", "index_number_end", "production_year", "premiere_date", "end_date", "runtime", "official_rating", "community_rating", "critic_rating", "genres", "tags", "studios", "external_ids", "artists", "album_artists", "series_status", "extra", "owner_id", "date_added", "images", "series_id", "series_name", "season_id", "season_name", "album_id", "album_name", "locked", "locked_fields", "custom_rating", "production_locations", "collection_name", "normalization_gain", "external_urls", "virtual", "monitored", "series_type", "display_order", "absolute_number", "user_data", "original_language", "digital_release_date", "physical_release_date", "remote_trailers")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_ID_FIELD_NUMBER: _ClassVar[int]
    PARENT_ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    SORT_NAME_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_TITLE_FIELD_NUMBER: _ClassVar[int]
    OVERVIEW_FIELD_NUMBER: _ClassVar[int]
    TAGLINE_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    INDEX_NUMBER_FIELD_NUMBER: _ClassVar[int]
    PARENT_INDEX_NUMBER_FIELD_NUMBER: _ClassVar[int]
    INDEX_NUMBER_END_FIELD_NUMBER: _ClassVar[int]
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
    ARTISTS_FIELD_NUMBER: _ClassVar[int]
    ALBUM_ARTISTS_FIELD_NUMBER: _ClassVar[int]
    SERIES_STATUS_FIELD_NUMBER: _ClassVar[int]
    EXTRA_FIELD_NUMBER: _ClassVar[int]
    OWNER_ID_FIELD_NUMBER: _ClassVar[int]
    DATE_ADDED_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    SERIES_ID_FIELD_NUMBER: _ClassVar[int]
    SERIES_NAME_FIELD_NUMBER: _ClassVar[int]
    SEASON_ID_FIELD_NUMBER: _ClassVar[int]
    SEASON_NAME_FIELD_NUMBER: _ClassVar[int]
    ALBUM_ID_FIELD_NUMBER: _ClassVar[int]
    ALBUM_NAME_FIELD_NUMBER: _ClassVar[int]
    LOCKED_FIELD_NUMBER: _ClassVar[int]
    LOCKED_FIELDS_FIELD_NUMBER: _ClassVar[int]
    CUSTOM_RATING_FIELD_NUMBER: _ClassVar[int]
    PRODUCTION_LOCATIONS_FIELD_NUMBER: _ClassVar[int]
    COLLECTION_NAME_FIELD_NUMBER: _ClassVar[int]
    NORMALIZATION_GAIN_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_URLS_FIELD_NUMBER: _ClassVar[int]
    VIRTUAL_FIELD_NUMBER: _ClassVar[int]
    MONITORED_FIELD_NUMBER: _ClassVar[int]
    SERIES_TYPE_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_ORDER_FIELD_NUMBER: _ClassVar[int]
    ABSOLUTE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    USER_DATA_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    DIGITAL_RELEASE_DATE_FIELD_NUMBER: _ClassVar[int]
    PHYSICAL_RELEASE_DATE_FIELD_NUMBER: _ClassVar[int]
    REMOTE_TRAILERS_FIELD_NUMBER: _ClassVar[int]
    id: str
    library_id: str
    parent_id: str
    kind: ItemKind
    name: str
    sort_name: str
    original_title: str
    overview: str
    tagline: str
    path: str
    index_number: int
    parent_index_number: int
    index_number_end: int
    production_year: int
    premiere_date: _timestamp_pb2.Timestamp
    end_date: _timestamp_pb2.Timestamp
    runtime: _duration_pb2.Duration
    official_rating: str
    community_rating: float
    critic_rating: float
    genres: _containers.RepeatedScalarFieldContainer[str]
    tags: _containers.RepeatedScalarFieldContainer[str]
    studios: _containers.RepeatedScalarFieldContainer[str]
    external_ids: _containers.ScalarMap[str, str]
    artists: _containers.RepeatedScalarFieldContainer[str]
    album_artists: _containers.RepeatedScalarFieldContainer[str]
    series_status: SeriesStatus
    extra: ExtraKind
    owner_id: str
    date_added: _timestamp_pb2.Timestamp
    images: _containers.RepeatedCompositeFieldContainer[Image]
    series_id: str
    series_name: str
    season_id: str
    season_name: str
    album_id: str
    album_name: str
    locked: bool
    locked_fields: _containers.RepeatedScalarFieldContainer[MetadataField]
    custom_rating: str
    production_locations: _containers.RepeatedScalarFieldContainer[str]
    collection_name: str
    normalization_gain: float
    external_urls: _containers.RepeatedCompositeFieldContainer[ExternalUrl]
    virtual: bool
    monitored: bool
    series_type: SeriesType
    display_order: DisplayOrder
    absolute_number: int
    user_data: _user_pb2.UserData
    original_language: str
    digital_release_date: _timestamp_pb2.Timestamp
    physical_release_date: _timestamp_pb2.Timestamp
    remote_trailers: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[str] = ..., library_id: _Optional[str] = ..., parent_id: _Optional[str] = ..., kind: _Optional[_Union[ItemKind, str]] = ..., name: _Optional[str] = ..., sort_name: _Optional[str] = ..., original_title: _Optional[str] = ..., overview: _Optional[str] = ..., tagline: _Optional[str] = ..., path: _Optional[str] = ..., index_number: _Optional[int] = ..., parent_index_number: _Optional[int] = ..., index_number_end: _Optional[int] = ..., production_year: _Optional[int] = ..., premiere_date: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., end_date: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., runtime: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., official_rating: _Optional[str] = ..., community_rating: _Optional[float] = ..., critic_rating: _Optional[float] = ..., genres: _Optional[_Iterable[str]] = ..., tags: _Optional[_Iterable[str]] = ..., studios: _Optional[_Iterable[str]] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., artists: _Optional[_Iterable[str]] = ..., album_artists: _Optional[_Iterable[str]] = ..., series_status: _Optional[_Union[SeriesStatus, str]] = ..., extra: _Optional[_Union[ExtraKind, str]] = ..., owner_id: _Optional[str] = ..., date_added: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., images: _Optional[_Iterable[_Union[Image, _Mapping]]] = ..., series_id: _Optional[str] = ..., series_name: _Optional[str] = ..., season_id: _Optional[str] = ..., season_name: _Optional[str] = ..., album_id: _Optional[str] = ..., album_name: _Optional[str] = ..., locked: _Optional[bool] = ..., locked_fields: _Optional[_Iterable[_Union[MetadataField, str]]] = ..., custom_rating: _Optional[str] = ..., production_locations: _Optional[_Iterable[str]] = ..., collection_name: _Optional[str] = ..., normalization_gain: _Optional[float] = ..., external_urls: _Optional[_Iterable[_Union[ExternalUrl, _Mapping]]] = ..., virtual: _Optional[bool] = ..., monitored: _Optional[bool] = ..., series_type: _Optional[_Union[SeriesType, str]] = ..., display_order: _Optional[_Union[DisplayOrder, str]] = ..., absolute_number: _Optional[int] = ..., user_data: _Optional[_Union[_user_pb2.UserData, _Mapping]] = ..., original_language: _Optional[str] = ..., digital_release_date: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., physical_release_date: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., remote_trailers: _Optional[_Iterable[str]] = ...) -> None: ...

class ExternalUrl(_message.Message):
    __slots__ = ("name", "url")
    NAME_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    name: str
    url: str
    def __init__(self, name: _Optional[str] = ..., url: _Optional[str] = ...) -> None: ...

class Image(_message.Message):
    __slots__ = ("id", "kind", "index", "width", "height", "blurhash", "thumbhash", "url")
    ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    BLURHASH_FIELD_NUMBER: _ClassVar[int]
    THUMBHASH_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    id: str
    kind: ImageKind
    index: int
    width: int
    height: int
    blurhash: str
    thumbhash: bytes
    url: str
    def __init__(self, id: _Optional[str] = ..., kind: _Optional[_Union[ImageKind, str]] = ..., index: _Optional[int] = ..., width: _Optional[int] = ..., height: _Optional[int] = ..., blurhash: _Optional[str] = ..., thumbhash: _Optional[bytes] = ..., url: _Optional[str] = ...) -> None: ...

class Person(_message.Message):
    __slots__ = ("id", "name", "sort_name", "overview", "birth_date", "death_date", "birth_place", "external_ids", "images", "external_urls")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    SORT_NAME_FIELD_NUMBER: _ClassVar[int]
    OVERVIEW_FIELD_NUMBER: _ClassVar[int]
    BIRTH_DATE_FIELD_NUMBER: _ClassVar[int]
    DEATH_DATE_FIELD_NUMBER: _ClassVar[int]
    BIRTH_PLACE_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_URLS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    sort_name: str
    overview: str
    birth_date: _timestamp_pb2.Timestamp
    death_date: _timestamp_pb2.Timestamp
    birth_place: str
    external_ids: _containers.ScalarMap[str, str]
    images: _containers.RepeatedCompositeFieldContainer[Image]
    external_urls: _containers.RepeatedCompositeFieldContainer[ExternalUrl]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., sort_name: _Optional[str] = ..., overview: _Optional[str] = ..., birth_date: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., death_date: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., birth_place: _Optional[str] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., images: _Optional[_Iterable[_Union[Image, _Mapping]]] = ..., external_urls: _Optional[_Iterable[_Union[ExternalUrl, _Mapping]]] = ...) -> None: ...

class Credit(_message.Message):
    __slots__ = ("person", "kind", "role", "order")
    PERSON_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    ORDER_FIELD_NUMBER: _ClassVar[int]
    person: Person
    kind: CreditKind
    role: str
    order: int
    def __init__(self, person: _Optional[_Union[Person, _Mapping]] = ..., kind: _Optional[_Union[CreditKind, str]] = ..., role: _Optional[str] = ..., order: _Optional[int] = ...) -> None: ...

class GetItemRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class GetItemResponse(_message.Message):
    __slots__ = ("item", "media_sources", "credits", "trickplay", "segments")
    ITEM_FIELD_NUMBER: _ClassVar[int]
    MEDIA_SOURCES_FIELD_NUMBER: _ClassVar[int]
    CREDITS_FIELD_NUMBER: _ClassVar[int]
    TRICKPLAY_FIELD_NUMBER: _ClassVar[int]
    SEGMENTS_FIELD_NUMBER: _ClassVar[int]
    item: Item
    media_sources: _containers.RepeatedCompositeFieldContainer[_media_pb2.MediaSource]
    credits: _containers.RepeatedCompositeFieldContainer[Credit]
    trickplay: _containers.RepeatedCompositeFieldContainer[Trickplay]
    segments: _containers.RepeatedCompositeFieldContainer[MediaSegment]
    def __init__(self, item: _Optional[_Union[Item, _Mapping]] = ..., media_sources: _Optional[_Iterable[_Union[_media_pb2.MediaSource, _Mapping]]] = ..., credits: _Optional[_Iterable[_Union[Credit, _Mapping]]] = ..., trickplay: _Optional[_Iterable[_Union[Trickplay, _Mapping]]] = ..., segments: _Optional[_Iterable[_Union[MediaSegment, _Mapping]]] = ...) -> None: ...

class Trickplay(_message.Message):
    __slots__ = ("width", "height", "tile_width", "tile_height", "thumbnail_count", "interval", "bandwidth", "sheet_count", "url")
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    TILE_WIDTH_FIELD_NUMBER: _ClassVar[int]
    TILE_HEIGHT_FIELD_NUMBER: _ClassVar[int]
    THUMBNAIL_COUNT_FIELD_NUMBER: _ClassVar[int]
    INTERVAL_FIELD_NUMBER: _ClassVar[int]
    BANDWIDTH_FIELD_NUMBER: _ClassVar[int]
    SHEET_COUNT_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    width: int
    height: int
    tile_width: int
    tile_height: int
    thumbnail_count: int
    interval: _duration_pb2.Duration
    bandwidth: int
    sheet_count: int
    url: str
    def __init__(self, width: _Optional[int] = ..., height: _Optional[int] = ..., tile_width: _Optional[int] = ..., tile_height: _Optional[int] = ..., thumbnail_count: _Optional[int] = ..., interval: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., bandwidth: _Optional[int] = ..., sheet_count: _Optional[int] = ..., url: _Optional[str] = ...) -> None: ...

class MediaSegment(_message.Message):
    __slots__ = ("id", "kind", "start", "end")
    ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    START_FIELD_NUMBER: _ClassVar[int]
    END_FIELD_NUMBER: _ClassVar[int]
    id: str
    kind: SegmentKind
    start: _duration_pb2.Duration
    end: _duration_pb2.Duration
    def __init__(self, id: _Optional[str] = ..., kind: _Optional[_Union[SegmentKind, str]] = ..., start: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., end: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class GetLyricsRequest(_message.Message):
    __slots__ = ("item_id",)
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    def __init__(self, item_id: _Optional[str] = ...) -> None: ...

class LyricCue(_message.Message):
    __slots__ = ("position", "end_position", "start", "end")
    POSITION_FIELD_NUMBER: _ClassVar[int]
    END_POSITION_FIELD_NUMBER: _ClassVar[int]
    START_FIELD_NUMBER: _ClassVar[int]
    END_FIELD_NUMBER: _ClassVar[int]
    position: int
    end_position: int
    start: _duration_pb2.Duration
    end: _duration_pb2.Duration
    def __init__(self, position: _Optional[int] = ..., end_position: _Optional[int] = ..., start: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., end: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class LyricLine(_message.Message):
    __slots__ = ("text", "start", "cues")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    START_FIELD_NUMBER: _ClassVar[int]
    CUES_FIELD_NUMBER: _ClassVar[int]
    text: str
    start: _duration_pb2.Duration
    cues: _containers.RepeatedCompositeFieldContainer[LyricCue]
    def __init__(self, text: _Optional[str] = ..., start: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., cues: _Optional[_Iterable[_Union[LyricCue, _Mapping]]] = ...) -> None: ...

class GetLyricsResponse(_message.Message):
    __slots__ = ("artist", "album", "title", "synced", "lines")
    ARTIST_FIELD_NUMBER: _ClassVar[int]
    ALBUM_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    SYNCED_FIELD_NUMBER: _ClassVar[int]
    LINES_FIELD_NUMBER: _ClassVar[int]
    artist: str
    album: str
    title: str
    synced: bool
    lines: _containers.RepeatedCompositeFieldContainer[LyricLine]
    def __init__(self, artist: _Optional[str] = ..., album: _Optional[str] = ..., title: _Optional[str] = ..., synced: _Optional[bool] = ..., lines: _Optional[_Iterable[_Union[LyricLine, _Mapping]]] = ...) -> None: ...

class SortSpec(_message.Message):
    __slots__ = ("field", "descending")
    FIELD_FIELD_NUMBER: _ClassVar[int]
    DESCENDING_FIELD_NUMBER: _ClassVar[int]
    field: SortField
    descending: bool
    def __init__(self, field: _Optional[_Union[SortField, str]] = ..., descending: _Optional[bool] = ...) -> None: ...

class ListItemsRequest(_message.Message):
    __slots__ = ("library_ids", "parent_id", "recursive", "kinds", "include_extras", "search", "genres", "tags", "studios", "person_id", "year_from", "year_to", "played", "favorite", "resumable", "sort", "limit", "offset", "top_level", "include_virtual", "monitored", "external_ids", "owner_id")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    LIBRARY_IDS_FIELD_NUMBER: _ClassVar[int]
    PARENT_ID_FIELD_NUMBER: _ClassVar[int]
    RECURSIVE_FIELD_NUMBER: _ClassVar[int]
    KINDS_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_EXTRAS_FIELD_NUMBER: _ClassVar[int]
    SEARCH_FIELD_NUMBER: _ClassVar[int]
    GENRES_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    STUDIOS_FIELD_NUMBER: _ClassVar[int]
    PERSON_ID_FIELD_NUMBER: _ClassVar[int]
    YEAR_FROM_FIELD_NUMBER: _ClassVar[int]
    YEAR_TO_FIELD_NUMBER: _ClassVar[int]
    PLAYED_FIELD_NUMBER: _ClassVar[int]
    FAVORITE_FIELD_NUMBER: _ClassVar[int]
    RESUMABLE_FIELD_NUMBER: _ClassVar[int]
    SORT_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    TOP_LEVEL_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_VIRTUAL_FIELD_NUMBER: _ClassVar[int]
    MONITORED_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    OWNER_ID_FIELD_NUMBER: _ClassVar[int]
    library_ids: _containers.RepeatedScalarFieldContainer[str]
    parent_id: str
    recursive: bool
    kinds: _containers.RepeatedScalarFieldContainer[ItemKind]
    include_extras: bool
    search: str
    genres: _containers.RepeatedScalarFieldContainer[str]
    tags: _containers.RepeatedScalarFieldContainer[str]
    studios: _containers.RepeatedScalarFieldContainer[str]
    person_id: str
    year_from: int
    year_to: int
    played: bool
    favorite: bool
    resumable: bool
    sort: _containers.RepeatedCompositeFieldContainer[SortSpec]
    limit: int
    offset: int
    top_level: bool
    include_virtual: bool
    monitored: bool
    external_ids: _containers.ScalarMap[str, str]
    owner_id: str
    def __init__(self, library_ids: _Optional[_Iterable[str]] = ..., parent_id: _Optional[str] = ..., recursive: _Optional[bool] = ..., kinds: _Optional[_Iterable[_Union[ItemKind, str]]] = ..., include_extras: _Optional[bool] = ..., search: _Optional[str] = ..., genres: _Optional[_Iterable[str]] = ..., tags: _Optional[_Iterable[str]] = ..., studios: _Optional[_Iterable[str]] = ..., person_id: _Optional[str] = ..., year_from: _Optional[int] = ..., year_to: _Optional[int] = ..., played: _Optional[bool] = ..., favorite: _Optional[bool] = ..., resumable: _Optional[bool] = ..., sort: _Optional[_Iterable[_Union[SortSpec, _Mapping]]] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ..., top_level: _Optional[bool] = ..., include_virtual: _Optional[bool] = ..., monitored: _Optional[bool] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., owner_id: _Optional[str] = ...) -> None: ...

class ListItemsResponse(_message.Message):
    __slots__ = ("items", "total")
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[Item]
    total: int
    def __init__(self, items: _Optional[_Iterable[_Union[Item, _Mapping]]] = ..., total: _Optional[int] = ...) -> None: ...

class GetPersonRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class GetPersonResponse(_message.Message):
    __slots__ = ("person",)
    PERSON_FIELD_NUMBER: _ClassVar[int]
    person: Person
    def __init__(self, person: _Optional[_Union[Person, _Mapping]] = ...) -> None: ...

class ListValuesRequest(_message.Message):
    __slots__ = ("kind", "library_ids", "item_kinds", "search", "limit", "offset")
    KIND_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_IDS_FIELD_NUMBER: _ClassVar[int]
    ITEM_KINDS_FIELD_NUMBER: _ClassVar[int]
    SEARCH_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    kind: ValueKind
    library_ids: _containers.RepeatedScalarFieldContainer[str]
    item_kinds: _containers.RepeatedScalarFieldContainer[ItemKind]
    search: str
    limit: int
    offset: int
    def __init__(self, kind: _Optional[_Union[ValueKind, str]] = ..., library_ids: _Optional[_Iterable[str]] = ..., item_kinds: _Optional[_Iterable[_Union[ItemKind, str]]] = ..., search: _Optional[str] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class ValueCount(_message.Message):
    __slots__ = ("value", "item_count")
    VALUE_FIELD_NUMBER: _ClassVar[int]
    ITEM_COUNT_FIELD_NUMBER: _ClassVar[int]
    value: str
    item_count: int
    def __init__(self, value: _Optional[str] = ..., item_count: _Optional[int] = ...) -> None: ...

class ListValuesResponse(_message.Message):
    __slots__ = ("values",)
    VALUES_FIELD_NUMBER: _ClassVar[int]
    values: _containers.RepeatedCompositeFieldContainer[ValueCount]
    def __init__(self, values: _Optional[_Iterable[_Union[ValueCount, _Mapping]]] = ...) -> None: ...

class ListPeopleRequest(_message.Message):
    __slots__ = ("library_ids", "item_kinds", "credit_kinds", "search", "limit", "offset")
    LIBRARY_IDS_FIELD_NUMBER: _ClassVar[int]
    ITEM_KINDS_FIELD_NUMBER: _ClassVar[int]
    CREDIT_KINDS_FIELD_NUMBER: _ClassVar[int]
    SEARCH_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    library_ids: _containers.RepeatedScalarFieldContainer[str]
    item_kinds: _containers.RepeatedScalarFieldContainer[ItemKind]
    credit_kinds: _containers.RepeatedScalarFieldContainer[CreditKind]
    search: str
    limit: int
    offset: int
    def __init__(self, library_ids: _Optional[_Iterable[str]] = ..., item_kinds: _Optional[_Iterable[_Union[ItemKind, str]]] = ..., credit_kinds: _Optional[_Iterable[_Union[CreditKind, str]]] = ..., search: _Optional[str] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class PersonCount(_message.Message):
    __slots__ = ("person", "item_count")
    PERSON_FIELD_NUMBER: _ClassVar[int]
    ITEM_COUNT_FIELD_NUMBER: _ClassVar[int]
    person: Person
    item_count: int
    def __init__(self, person: _Optional[_Union[Person, _Mapping]] = ..., item_count: _Optional[int] = ...) -> None: ...

class ListPeopleResponse(_message.Message):
    __slots__ = ("people",)
    PEOPLE_FIELD_NUMBER: _ClassVar[int]
    people: _containers.RepeatedCompositeFieldContainer[PersonCount]
    def __init__(self, people: _Optional[_Iterable[_Union[PersonCount, _Mapping]]] = ...) -> None: ...

class ListLatestItemsRequest(_message.Message):
    __slots__ = ("library_ids", "kinds", "played", "ungrouped", "limit")
    LIBRARY_IDS_FIELD_NUMBER: _ClassVar[int]
    KINDS_FIELD_NUMBER: _ClassVar[int]
    PLAYED_FIELD_NUMBER: _ClassVar[int]
    UNGROUPED_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    library_ids: _containers.RepeatedScalarFieldContainer[str]
    kinds: _containers.RepeatedScalarFieldContainer[ItemKind]
    played: bool
    ungrouped: bool
    limit: int
    def __init__(self, library_ids: _Optional[_Iterable[str]] = ..., kinds: _Optional[_Iterable[_Union[ItemKind, str]]] = ..., played: _Optional[bool] = ..., ungrouped: _Optional[bool] = ..., limit: _Optional[int] = ...) -> None: ...

class ListLatestItemsResponse(_message.Message):
    __slots__ = ("items",)
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[Item]
    def __init__(self, items: _Optional[_Iterable[_Union[Item, _Mapping]]] = ...) -> None: ...

class ListSimilarItemsRequest(_message.Message):
    __slots__ = ("id", "played", "limit")
    ID_FIELD_NUMBER: _ClassVar[int]
    PLAYED_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    id: str
    played: bool
    limit: int
    def __init__(self, id: _Optional[str] = ..., played: _Optional[bool] = ..., limit: _Optional[int] = ...) -> None: ...

class ListSimilarItemsResponse(_message.Message):
    __slots__ = ("items",)
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[Item]
    def __init__(self, items: _Optional[_Iterable[_Union[Item, _Mapping]]] = ...) -> None: ...

class ListNextUpRequest(_message.Message):
    __slots__ = ("library_ids", "series_id", "since", "include_resumable", "limit", "offset")
    LIBRARY_IDS_FIELD_NUMBER: _ClassVar[int]
    SERIES_ID_FIELD_NUMBER: _ClassVar[int]
    SINCE_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_RESUMABLE_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    library_ids: _containers.RepeatedScalarFieldContainer[str]
    series_id: str
    since: _timestamp_pb2.Timestamp
    include_resumable: bool
    limit: int
    offset: int
    def __init__(self, library_ids: _Optional[_Iterable[str]] = ..., series_id: _Optional[str] = ..., since: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., include_resumable: _Optional[bool] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class ListNextUpResponse(_message.Message):
    __slots__ = ("items",)
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[Item]
    def __init__(self, items: _Optional[_Iterable[_Union[Item, _Mapping]]] = ...) -> None: ...

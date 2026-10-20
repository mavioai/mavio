import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import field_mask_pb2 as _field_mask_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.library.v1 import item_pb2 as _item_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ItemMetadata(_message.Message):
    __slots__ = ("name", "sort_name", "original_title", "overview", "tagline", "production_year", "premiere_date", "end_date", "runtime", "official_rating", "custom_rating", "community_rating", "critic_rating", "genres", "tags", "studios", "production_locations", "external_ids", "index_number", "parent_index_number", "series_status", "artists", "album_artists", "album", "locked", "locked_fields", "monitored", "series_type", "display_order", "absolute_number")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    SORT_NAME_FIELD_NUMBER: _ClassVar[int]
    ORIGINAL_TITLE_FIELD_NUMBER: _ClassVar[int]
    OVERVIEW_FIELD_NUMBER: _ClassVar[int]
    TAGLINE_FIELD_NUMBER: _ClassVar[int]
    PRODUCTION_YEAR_FIELD_NUMBER: _ClassVar[int]
    PREMIERE_DATE_FIELD_NUMBER: _ClassVar[int]
    END_DATE_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_FIELD_NUMBER: _ClassVar[int]
    OFFICIAL_RATING_FIELD_NUMBER: _ClassVar[int]
    CUSTOM_RATING_FIELD_NUMBER: _ClassVar[int]
    COMMUNITY_RATING_FIELD_NUMBER: _ClassVar[int]
    CRITIC_RATING_FIELD_NUMBER: _ClassVar[int]
    GENRES_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    STUDIOS_FIELD_NUMBER: _ClassVar[int]
    PRODUCTION_LOCATIONS_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    INDEX_NUMBER_FIELD_NUMBER: _ClassVar[int]
    PARENT_INDEX_NUMBER_FIELD_NUMBER: _ClassVar[int]
    SERIES_STATUS_FIELD_NUMBER: _ClassVar[int]
    ARTISTS_FIELD_NUMBER: _ClassVar[int]
    ALBUM_ARTISTS_FIELD_NUMBER: _ClassVar[int]
    ALBUM_FIELD_NUMBER: _ClassVar[int]
    LOCKED_FIELD_NUMBER: _ClassVar[int]
    LOCKED_FIELDS_FIELD_NUMBER: _ClassVar[int]
    MONITORED_FIELD_NUMBER: _ClassVar[int]
    SERIES_TYPE_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_ORDER_FIELD_NUMBER: _ClassVar[int]
    ABSOLUTE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    name: str
    sort_name: str
    original_title: str
    overview: str
    tagline: str
    production_year: int
    premiere_date: _timestamp_pb2.Timestamp
    end_date: _timestamp_pb2.Timestamp
    runtime: _duration_pb2.Duration
    official_rating: str
    custom_rating: str
    community_rating: float
    critic_rating: float
    genres: _containers.RepeatedScalarFieldContainer[str]
    tags: _containers.RepeatedScalarFieldContainer[str]
    studios: _containers.RepeatedScalarFieldContainer[str]
    production_locations: _containers.RepeatedScalarFieldContainer[str]
    external_ids: _containers.ScalarMap[str, str]
    index_number: int
    parent_index_number: int
    series_status: _item_pb2.SeriesStatus
    artists: _containers.RepeatedScalarFieldContainer[str]
    album_artists: _containers.RepeatedScalarFieldContainer[str]
    album: str
    locked: bool
    locked_fields: _containers.RepeatedScalarFieldContainer[_item_pb2.MetadataField]
    monitored: bool
    series_type: _item_pb2.SeriesType
    display_order: _item_pb2.DisplayOrder
    absolute_number: int
    def __init__(self, name: _Optional[str] = ..., sort_name: _Optional[str] = ..., original_title: _Optional[str] = ..., overview: _Optional[str] = ..., tagline: _Optional[str] = ..., production_year: _Optional[int] = ..., premiere_date: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., end_date: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., runtime: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., official_rating: _Optional[str] = ..., custom_rating: _Optional[str] = ..., community_rating: _Optional[float] = ..., critic_rating: _Optional[float] = ..., genres: _Optional[_Iterable[str]] = ..., tags: _Optional[_Iterable[str]] = ..., studios: _Optional[_Iterable[str]] = ..., production_locations: _Optional[_Iterable[str]] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., index_number: _Optional[int] = ..., parent_index_number: _Optional[int] = ..., series_status: _Optional[_Union[_item_pb2.SeriesStatus, str]] = ..., artists: _Optional[_Iterable[str]] = ..., album_artists: _Optional[_Iterable[str]] = ..., album: _Optional[str] = ..., locked: _Optional[bool] = ..., locked_fields: _Optional[_Iterable[_Union[_item_pb2.MetadataField, str]]] = ..., monitored: _Optional[bool] = ..., series_type: _Optional[_Union[_item_pb2.SeriesType, str]] = ..., display_order: _Optional[_Union[_item_pb2.DisplayOrder, str]] = ..., absolute_number: _Optional[int] = ...) -> None: ...

class UpdateItemRequest(_message.Message):
    __slots__ = ("id", "metadata", "update_mask")
    ID_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    UPDATE_MASK_FIELD_NUMBER: _ClassVar[int]
    id: str
    metadata: ItemMetadata
    update_mask: _field_mask_pb2.FieldMask
    def __init__(self, id: _Optional[str] = ..., metadata: _Optional[_Union[ItemMetadata, _Mapping]] = ..., update_mask: _Optional[_Union[_field_mask_pb2.FieldMask, _Mapping]] = ...) -> None: ...

class UpdateItemResponse(_message.Message):
    __slots__ = ("item",)
    ITEM_FIELD_NUMBER: _ClassVar[int]
    item: _item_pb2.Item
    def __init__(self, item: _Optional[_Union[_item_pb2.Item, _Mapping]] = ...) -> None: ...

class RefreshItemRequest(_message.Message):
    __slots__ = ("id", "replace_metadata")
    ID_FIELD_NUMBER: _ClassVar[int]
    REPLACE_METADATA_FIELD_NUMBER: _ClassVar[int]
    id: str
    replace_metadata: bool
    def __init__(self, id: _Optional[str] = ..., replace_metadata: _Optional[bool] = ...) -> None: ...

class RefreshItemResponse(_message.Message):
    __slots__ = ("item",)
    ITEM_FIELD_NUMBER: _ClassVar[int]
    item: _item_pb2.Item
    def __init__(self, item: _Optional[_Union[_item_pb2.Item, _Mapping]] = ...) -> None: ...

class SearchRemoteRequest(_message.Message):
    __slots__ = ("item_id", "name", "year", "external_ids", "provider")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    name: str
    year: int
    external_ids: _containers.ScalarMap[str, str]
    provider: str
    def __init__(self, item_id: _Optional[str] = ..., name: _Optional[str] = ..., year: _Optional[int] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., provider: _Optional[str] = ...) -> None: ...

class RemoteSearchResult(_message.Message):
    __slots__ = ("provider", "name", "year", "overview", "external_ids", "image_url", "score")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    OVERVIEW_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    IMAGE_URL_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    provider: str
    name: str
    year: int
    overview: str
    external_ids: _containers.ScalarMap[str, str]
    image_url: str
    score: float
    def __init__(self, provider: _Optional[str] = ..., name: _Optional[str] = ..., year: _Optional[int] = ..., overview: _Optional[str] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., image_url: _Optional[str] = ..., score: _Optional[float] = ...) -> None: ...

class SearchRemoteResponse(_message.Message):
    __slots__ = ("results",)
    RESULTS_FIELD_NUMBER: _ClassVar[int]
    results: _containers.RepeatedCompositeFieldContainer[RemoteSearchResult]
    def __init__(self, results: _Optional[_Iterable[_Union[RemoteSearchResult, _Mapping]]] = ...) -> None: ...

class IdentifyItemRequest(_message.Message):
    __slots__ = ("item_id", "external_ids")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    external_ids: _containers.ScalarMap[str, str]
    def __init__(self, item_id: _Optional[str] = ..., external_ids: _Optional[_Mapping[str, str]] = ...) -> None: ...

class IdentifyItemResponse(_message.Message):
    __slots__ = ("item",)
    ITEM_FIELD_NUMBER: _ClassVar[int]
    item: _item_pb2.Item
    def __init__(self, item: _Optional[_Union[_item_pb2.Item, _Mapping]] = ...) -> None: ...

class ListRemoteImagesRequest(_message.Message):
    __slots__ = ("item_id", "kind")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    kind: _item_pb2.ImageKind
    def __init__(self, item_id: _Optional[str] = ..., kind: _Optional[_Union[_item_pb2.ImageKind, str]] = ...) -> None: ...

class RemoteImage(_message.Message):
    __slots__ = ("provider", "kind", "url", "width", "height", "language", "score")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    provider: str
    kind: _item_pb2.ImageKind
    url: str
    width: int
    height: int
    language: str
    score: float
    def __init__(self, provider: _Optional[str] = ..., kind: _Optional[_Union[_item_pb2.ImageKind, str]] = ..., url: _Optional[str] = ..., width: _Optional[int] = ..., height: _Optional[int] = ..., language: _Optional[str] = ..., score: _Optional[float] = ...) -> None: ...

class ListRemoteImagesResponse(_message.Message):
    __slots__ = ("images",)
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    images: _containers.RepeatedCompositeFieldContainer[RemoteImage]
    def __init__(self, images: _Optional[_Iterable[_Union[RemoteImage, _Mapping]]] = ...) -> None: ...

class SetItemImageRequest(_message.Message):
    __slots__ = ("item_id", "kind", "url", "data")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    kind: _item_pb2.ImageKind
    url: str
    data: bytes
    def __init__(self, item_id: _Optional[str] = ..., kind: _Optional[_Union[_item_pb2.ImageKind, str]] = ..., url: _Optional[str] = ..., data: _Optional[bytes] = ...) -> None: ...

class SetItemImageResponse(_message.Message):
    __slots__ = ("image",)
    IMAGE_FIELD_NUMBER: _ClassVar[int]
    image: _item_pb2.Image
    def __init__(self, image: _Optional[_Union[_item_pb2.Image, _Mapping]] = ...) -> None: ...

class DeleteItemImageRequest(_message.Message):
    __slots__ = ("item_id", "image_id")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    IMAGE_ID_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    image_id: str
    def __init__(self, item_id: _Optional[str] = ..., image_id: _Optional[str] = ...) -> None: ...

class DeleteItemImageResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SearchRemoteLyricsRequest(_message.Message):
    __slots__ = ("item_id",)
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    def __init__(self, item_id: _Optional[str] = ...) -> None: ...

class RemoteLyrics(_message.Message):
    __slots__ = ("provider", "id", "name", "artists", "album", "duration", "synced", "score")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ARTISTS_FIELD_NUMBER: _ClassVar[int]
    ALBUM_FIELD_NUMBER: _ClassVar[int]
    DURATION_FIELD_NUMBER: _ClassVar[int]
    SYNCED_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    provider: str
    id: str
    name: str
    artists: _containers.RepeatedScalarFieldContainer[str]
    album: str
    duration: _duration_pb2.Duration
    synced: bool
    score: float
    def __init__(self, provider: _Optional[str] = ..., id: _Optional[str] = ..., name: _Optional[str] = ..., artists: _Optional[_Iterable[str]] = ..., album: _Optional[str] = ..., duration: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., synced: _Optional[bool] = ..., score: _Optional[float] = ...) -> None: ...

class SearchRemoteLyricsResponse(_message.Message):
    __slots__ = ("lyrics",)
    LYRICS_FIELD_NUMBER: _ClassVar[int]
    lyrics: _containers.RepeatedCompositeFieldContainer[RemoteLyrics]
    def __init__(self, lyrics: _Optional[_Iterable[_Union[RemoteLyrics, _Mapping]]] = ...) -> None: ...

class DownloadRemoteLyricsRequest(_message.Message):
    __slots__ = ("item_id", "provider", "id")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    provider: str
    id: str
    def __init__(self, item_id: _Optional[str] = ..., provider: _Optional[str] = ..., id: _Optional[str] = ...) -> None: ...

class DownloadRemoteLyricsResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListExternalIdKindsRequest(_message.Message):
    __slots__ = ("item_kind",)
    ITEM_KIND_FIELD_NUMBER: _ClassVar[int]
    item_kind: _item_pb2.ItemKind
    def __init__(self, item_kind: _Optional[_Union[_item_pb2.ItemKind, str]] = ...) -> None: ...

class ExternalIdKind(_message.Message):
    __slots__ = ("key", "name", "item_kinds", "persons", "plugin_id")
    KEY_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ITEM_KINDS_FIELD_NUMBER: _ClassVar[int]
    PERSONS_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    key: str
    name: str
    item_kinds: _containers.RepeatedScalarFieldContainer[_item_pb2.ItemKind]
    persons: bool
    plugin_id: str
    def __init__(self, key: _Optional[str] = ..., name: _Optional[str] = ..., item_kinds: _Optional[_Iterable[_Union[_item_pb2.ItemKind, str]]] = ..., persons: _Optional[bool] = ..., plugin_id: _Optional[str] = ...) -> None: ...

class ListExternalIdKindsResponse(_message.Message):
    __slots__ = ("kinds",)
    KINDS_FIELD_NUMBER: _ClassVar[int]
    kinds: _containers.RepeatedCompositeFieldContainer[ExternalIdKind]
    def __init__(self, kinds: _Optional[_Iterable[_Union[ExternalIdKind, _Mapping]]] = ...) -> None: ...

import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from mavio.acquisition.v1 import release_pb2 as _release_pb2
from mavio.acquisition.v1 import settings_pb2 as _settings_pb2
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

class CustomFormatSpecKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CUSTOM_FORMAT_SPEC_KIND_UNSPECIFIED: _ClassVar[CustomFormatSpecKind]
    CUSTOM_FORMAT_SPEC_KIND_RELEASE_TITLE: _ClassVar[CustomFormatSpecKind]
    CUSTOM_FORMAT_SPEC_KIND_EDITION: _ClassVar[CustomFormatSpecKind]
    CUSTOM_FORMAT_SPEC_KIND_LANGUAGE: _ClassVar[CustomFormatSpecKind]
    CUSTOM_FORMAT_SPEC_KIND_INDEXER_FLAG: _ClassVar[CustomFormatSpecKind]
    CUSTOM_FORMAT_SPEC_KIND_SOURCE: _ClassVar[CustomFormatSpecKind]
    CUSTOM_FORMAT_SPEC_KIND_RESOLUTION: _ClassVar[CustomFormatSpecKind]
    CUSTOM_FORMAT_SPEC_KIND_QUALITY_MODIFIER: _ClassVar[CustomFormatSpecKind]
    CUSTOM_FORMAT_SPEC_KIND_SIZE: _ClassVar[CustomFormatSpecKind]
    CUSTOM_FORMAT_SPEC_KIND_RELEASE_GROUP: _ClassVar[CustomFormatSpecKind]
    CUSTOM_FORMAT_SPEC_KIND_YEAR: _ClassVar[CustomFormatSpecKind]
    CUSTOM_FORMAT_SPEC_KIND_RELEASE_TYPE: _ClassVar[CustomFormatSpecKind]
MEDIA_KIND_UNSPECIFIED: MediaKind
MEDIA_KIND_MOVIE: MediaKind
MEDIA_KIND_SERIES: MediaKind
CUSTOM_FORMAT_SPEC_KIND_UNSPECIFIED: CustomFormatSpecKind
CUSTOM_FORMAT_SPEC_KIND_RELEASE_TITLE: CustomFormatSpecKind
CUSTOM_FORMAT_SPEC_KIND_EDITION: CustomFormatSpecKind
CUSTOM_FORMAT_SPEC_KIND_LANGUAGE: CustomFormatSpecKind
CUSTOM_FORMAT_SPEC_KIND_INDEXER_FLAG: CustomFormatSpecKind
CUSTOM_FORMAT_SPEC_KIND_SOURCE: CustomFormatSpecKind
CUSTOM_FORMAT_SPEC_KIND_RESOLUTION: CustomFormatSpecKind
CUSTOM_FORMAT_SPEC_KIND_QUALITY_MODIFIER: CustomFormatSpecKind
CUSTOM_FORMAT_SPEC_KIND_SIZE: CustomFormatSpecKind
CUSTOM_FORMAT_SPEC_KIND_RELEASE_GROUP: CustomFormatSpecKind
CUSTOM_FORMAT_SPEC_KIND_YEAR: CustomFormatSpecKind
CUSTOM_FORMAT_SPEC_KIND_RELEASE_TYPE: CustomFormatSpecKind

class QualityDefinition(_message.Message):
    __slots__ = ("quality", "title", "min_size", "max_size", "preferred_size")
    QUALITY_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    MIN_SIZE_FIELD_NUMBER: _ClassVar[int]
    MAX_SIZE_FIELD_NUMBER: _ClassVar[int]
    PREFERRED_SIZE_FIELD_NUMBER: _ClassVar[int]
    quality: int
    title: str
    min_size: float
    max_size: float
    preferred_size: float
    def __init__(self, quality: _Optional[int] = ..., title: _Optional[str] = ..., min_size: _Optional[float] = ..., max_size: _Optional[float] = ..., preferred_size: _Optional[float] = ...) -> None: ...

class ListQualityDefinitionsRequest(_message.Message):
    __slots__ = ("kind",)
    KIND_FIELD_NUMBER: _ClassVar[int]
    kind: MediaKind
    def __init__(self, kind: _Optional[_Union[MediaKind, str]] = ...) -> None: ...

class ListQualityDefinitionsResponse(_message.Message):
    __slots__ = ("definitions",)
    DEFINITIONS_FIELD_NUMBER: _ClassVar[int]
    definitions: _containers.RepeatedCompositeFieldContainer[QualityDefinition]
    def __init__(self, definitions: _Optional[_Iterable[_Union[QualityDefinition, _Mapping]]] = ...) -> None: ...

class UpdateQualityDefinitionsRequest(_message.Message):
    __slots__ = ("kind", "definitions")
    KIND_FIELD_NUMBER: _ClassVar[int]
    DEFINITIONS_FIELD_NUMBER: _ClassVar[int]
    kind: MediaKind
    definitions: _containers.RepeatedCompositeFieldContainer[QualityDefinition]
    def __init__(self, kind: _Optional[_Union[MediaKind, str]] = ..., definitions: _Optional[_Iterable[_Union[QualityDefinition, _Mapping]]] = ...) -> None: ...

class UpdateQualityDefinitionsResponse(_message.Message):
    __slots__ = ("definitions",)
    DEFINITIONS_FIELD_NUMBER: _ClassVar[int]
    definitions: _containers.RepeatedCompositeFieldContainer[QualityDefinition]
    def __init__(self, definitions: _Optional[_Iterable[_Union[QualityDefinition, _Mapping]]] = ...) -> None: ...

class QualityProfileItem(_message.Message):
    __slots__ = ("id", "name", "quality", "items", "allowed")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    QUALITY_FIELD_NUMBER: _ClassVar[int]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    ALLOWED_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    quality: int
    items: _containers.RepeatedCompositeFieldContainer[QualityProfileItem]
    allowed: bool
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., quality: _Optional[int] = ..., items: _Optional[_Iterable[_Union[QualityProfileItem, _Mapping]]] = ..., allowed: _Optional[bool] = ...) -> None: ...

class FormatScore(_message.Message):
    __slots__ = ("format_id", "score")
    FORMAT_ID_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    format_id: str
    score: int
    def __init__(self, format_id: _Optional[str] = ..., score: _Optional[int] = ...) -> None: ...

class QualityProfile(_message.Message):
    __slots__ = ("id", "kind", "name", "upgrade_allowed", "cutoff", "items", "min_format_score", "cutoff_format_score", "min_upgrade_format_score", "format_scores", "language")
    ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    UPGRADE_ALLOWED_FIELD_NUMBER: _ClassVar[int]
    CUTOFF_FIELD_NUMBER: _ClassVar[int]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    MIN_FORMAT_SCORE_FIELD_NUMBER: _ClassVar[int]
    CUTOFF_FORMAT_SCORE_FIELD_NUMBER: _ClassVar[int]
    MIN_UPGRADE_FORMAT_SCORE_FIELD_NUMBER: _ClassVar[int]
    FORMAT_SCORES_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    id: str
    kind: MediaKind
    name: str
    upgrade_allowed: bool
    cutoff: int
    items: _containers.RepeatedCompositeFieldContainer[QualityProfileItem]
    min_format_score: int
    cutoff_format_score: int
    min_upgrade_format_score: int
    format_scores: _containers.RepeatedCompositeFieldContainer[FormatScore]
    language: str
    def __init__(self, id: _Optional[str] = ..., kind: _Optional[_Union[MediaKind, str]] = ..., name: _Optional[str] = ..., upgrade_allowed: _Optional[bool] = ..., cutoff: _Optional[int] = ..., items: _Optional[_Iterable[_Union[QualityProfileItem, _Mapping]]] = ..., min_format_score: _Optional[int] = ..., cutoff_format_score: _Optional[int] = ..., min_upgrade_format_score: _Optional[int] = ..., format_scores: _Optional[_Iterable[_Union[FormatScore, _Mapping]]] = ..., language: _Optional[str] = ...) -> None: ...

class ListQualityProfilesRequest(_message.Message):
    __slots__ = ("kind",)
    KIND_FIELD_NUMBER: _ClassVar[int]
    kind: MediaKind
    def __init__(self, kind: _Optional[_Union[MediaKind, str]] = ...) -> None: ...

class ListQualityProfilesResponse(_message.Message):
    __slots__ = ("profiles",)
    PROFILES_FIELD_NUMBER: _ClassVar[int]
    profiles: _containers.RepeatedCompositeFieldContainer[QualityProfile]
    def __init__(self, profiles: _Optional[_Iterable[_Union[QualityProfile, _Mapping]]] = ...) -> None: ...

class CreateQualityProfileRequest(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: QualityProfile
    def __init__(self, profile: _Optional[_Union[QualityProfile, _Mapping]] = ...) -> None: ...

class CreateQualityProfileResponse(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: QualityProfile
    def __init__(self, profile: _Optional[_Union[QualityProfile, _Mapping]] = ...) -> None: ...

class UpdateQualityProfileRequest(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: QualityProfile
    def __init__(self, profile: _Optional[_Union[QualityProfile, _Mapping]] = ...) -> None: ...

class UpdateQualityProfileResponse(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: QualityProfile
    def __init__(self, profile: _Optional[_Union[QualityProfile, _Mapping]] = ...) -> None: ...

class DeleteQualityProfileRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteQualityProfileResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CustomFormatSpec(_message.Message):
    __slots__ = ("kind", "name", "negate", "required", "value", "except_language", "min", "max")
    KIND_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    NEGATE_FIELD_NUMBER: _ClassVar[int]
    REQUIRED_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    EXCEPT_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    MIN_FIELD_NUMBER: _ClassVar[int]
    MAX_FIELD_NUMBER: _ClassVar[int]
    kind: CustomFormatSpecKind
    name: str
    negate: bool
    required: bool
    value: str
    except_language: bool
    min: float
    max: float
    def __init__(self, kind: _Optional[_Union[CustomFormatSpecKind, str]] = ..., name: _Optional[str] = ..., negate: _Optional[bool] = ..., required: _Optional[bool] = ..., value: _Optional[str] = ..., except_language: _Optional[bool] = ..., min: _Optional[float] = ..., max: _Optional[float] = ...) -> None: ...

class CustomFormat(_message.Message):
    __slots__ = ("id", "name", "include_when_renaming", "specifications")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_WHEN_RENAMING_FIELD_NUMBER: _ClassVar[int]
    SPECIFICATIONS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    include_when_renaming: bool
    specifications: _containers.RepeatedCompositeFieldContainer[CustomFormatSpec]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., include_when_renaming: _Optional[bool] = ..., specifications: _Optional[_Iterable[_Union[CustomFormatSpec, _Mapping]]] = ...) -> None: ...

class ListCustomFormatsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListCustomFormatsResponse(_message.Message):
    __slots__ = ("formats",)
    FORMATS_FIELD_NUMBER: _ClassVar[int]
    formats: _containers.RepeatedCompositeFieldContainer[CustomFormat]
    def __init__(self, formats: _Optional[_Iterable[_Union[CustomFormat, _Mapping]]] = ...) -> None: ...

class CreateCustomFormatRequest(_message.Message):
    __slots__ = ("format",)
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    format: CustomFormat
    def __init__(self, format: _Optional[_Union[CustomFormat, _Mapping]] = ...) -> None: ...

class CreateCustomFormatResponse(_message.Message):
    __slots__ = ("format",)
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    format: CustomFormat
    def __init__(self, format: _Optional[_Union[CustomFormat, _Mapping]] = ...) -> None: ...

class UpdateCustomFormatRequest(_message.Message):
    __slots__ = ("format",)
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    format: CustomFormat
    def __init__(self, format: _Optional[_Union[CustomFormat, _Mapping]] = ...) -> None: ...

class UpdateCustomFormatResponse(_message.Message):
    __slots__ = ("format",)
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    format: CustomFormat
    def __init__(self, format: _Optional[_Union[CustomFormat, _Mapping]] = ...) -> None: ...

class DeleteCustomFormatRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteCustomFormatResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class DelayProfile(_message.Message):
    __slots__ = ("id", "default", "order", "enable_usenet", "enable_torrent", "preferred_protocol", "usenet_delay", "torrent_delay", "bypass_if_highest_quality", "bypass_if_above_custom_format_score", "minimum_custom_format_score", "tags")
    ID_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_FIELD_NUMBER: _ClassVar[int]
    ORDER_FIELD_NUMBER: _ClassVar[int]
    ENABLE_USENET_FIELD_NUMBER: _ClassVar[int]
    ENABLE_TORRENT_FIELD_NUMBER: _ClassVar[int]
    PREFERRED_PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    USENET_DELAY_FIELD_NUMBER: _ClassVar[int]
    TORRENT_DELAY_FIELD_NUMBER: _ClassVar[int]
    BYPASS_IF_HIGHEST_QUALITY_FIELD_NUMBER: _ClassVar[int]
    BYPASS_IF_ABOVE_CUSTOM_FORMAT_SCORE_FIELD_NUMBER: _ClassVar[int]
    MINIMUM_CUSTOM_FORMAT_SCORE_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    id: str
    default: bool
    order: int
    enable_usenet: bool
    enable_torrent: bool
    preferred_protocol: _release_pb2.DownloadProtocol
    usenet_delay: _duration_pb2.Duration
    torrent_delay: _duration_pb2.Duration
    bypass_if_highest_quality: bool
    bypass_if_above_custom_format_score: bool
    minimum_custom_format_score: int
    tags: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[str] = ..., default: _Optional[bool] = ..., order: _Optional[int] = ..., enable_usenet: _Optional[bool] = ..., enable_torrent: _Optional[bool] = ..., preferred_protocol: _Optional[_Union[_release_pb2.DownloadProtocol, str]] = ..., usenet_delay: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., torrent_delay: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., bypass_if_highest_quality: _Optional[bool] = ..., bypass_if_above_custom_format_score: _Optional[bool] = ..., minimum_custom_format_score: _Optional[int] = ..., tags: _Optional[_Iterable[str]] = ...) -> None: ...

class ListDelayProfilesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListDelayProfilesResponse(_message.Message):
    __slots__ = ("profiles",)
    PROFILES_FIELD_NUMBER: _ClassVar[int]
    profiles: _containers.RepeatedCompositeFieldContainer[DelayProfile]
    def __init__(self, profiles: _Optional[_Iterable[_Union[DelayProfile, _Mapping]]] = ...) -> None: ...

class CreateDelayProfileRequest(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: DelayProfile
    def __init__(self, profile: _Optional[_Union[DelayProfile, _Mapping]] = ...) -> None: ...

class CreateDelayProfileResponse(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: DelayProfile
    def __init__(self, profile: _Optional[_Union[DelayProfile, _Mapping]] = ...) -> None: ...

class UpdateDelayProfileRequest(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: DelayProfile
    def __init__(self, profile: _Optional[_Union[DelayProfile, _Mapping]] = ...) -> None: ...

class UpdateDelayProfileResponse(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: DelayProfile
    def __init__(self, profile: _Optional[_Union[DelayProfile, _Mapping]] = ...) -> None: ...

class DeleteDelayProfileRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteDelayProfileResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ReorderDelayProfilesRequest(_message.Message):
    __slots__ = ("ids",)
    IDS_FIELD_NUMBER: _ClassVar[int]
    ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, ids: _Optional[_Iterable[str]] = ...) -> None: ...

class ReorderDelayProfilesResponse(_message.Message):
    __slots__ = ("profiles",)
    PROFILES_FIELD_NUMBER: _ClassVar[int]
    profiles: _containers.RepeatedCompositeFieldContainer[DelayProfile]
    def __init__(self, profiles: _Optional[_Iterable[_Union[DelayProfile, _Mapping]]] = ...) -> None: ...

class ReleaseProfile(_message.Message):
    __slots__ = ("id", "name", "enabled", "required", "ignored", "indexer", "tags")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    REQUIRED_FIELD_NUMBER: _ClassVar[int]
    IGNORED_FIELD_NUMBER: _ClassVar[int]
    INDEXER_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    enabled: bool
    required: _containers.RepeatedScalarFieldContainer[str]
    ignored: _containers.RepeatedScalarFieldContainer[str]
    indexer: _release_pb2.IndexerRef
    tags: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., enabled: _Optional[bool] = ..., required: _Optional[_Iterable[str]] = ..., ignored: _Optional[_Iterable[str]] = ..., indexer: _Optional[_Union[_release_pb2.IndexerRef, _Mapping]] = ..., tags: _Optional[_Iterable[str]] = ...) -> None: ...

class ListReleaseProfilesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListReleaseProfilesResponse(_message.Message):
    __slots__ = ("profiles",)
    PROFILES_FIELD_NUMBER: _ClassVar[int]
    profiles: _containers.RepeatedCompositeFieldContainer[ReleaseProfile]
    def __init__(self, profiles: _Optional[_Iterable[_Union[ReleaseProfile, _Mapping]]] = ...) -> None: ...

class CreateReleaseProfileRequest(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: ReleaseProfile
    def __init__(self, profile: _Optional[_Union[ReleaseProfile, _Mapping]] = ...) -> None: ...

class CreateReleaseProfileResponse(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: ReleaseProfile
    def __init__(self, profile: _Optional[_Union[ReleaseProfile, _Mapping]] = ...) -> None: ...

class UpdateReleaseProfileRequest(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: ReleaseProfile
    def __init__(self, profile: _Optional[_Union[ReleaseProfile, _Mapping]] = ...) -> None: ...

class UpdateReleaseProfileResponse(_message.Message):
    __slots__ = ("profile",)
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    profile: ReleaseProfile
    def __init__(self, profile: _Optional[_Union[ReleaseProfile, _Mapping]] = ...) -> None: ...

class DeleteReleaseProfileRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteReleaseProfileResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RemotePathMapping(_message.Message):
    __slots__ = ("id", "host", "remote_path", "local_path")
    ID_FIELD_NUMBER: _ClassVar[int]
    HOST_FIELD_NUMBER: _ClassVar[int]
    REMOTE_PATH_FIELD_NUMBER: _ClassVar[int]
    LOCAL_PATH_FIELD_NUMBER: _ClassVar[int]
    id: str
    host: str
    remote_path: str
    local_path: str
    def __init__(self, id: _Optional[str] = ..., host: _Optional[str] = ..., remote_path: _Optional[str] = ..., local_path: _Optional[str] = ...) -> None: ...

class ListRemotePathMappingsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListRemotePathMappingsResponse(_message.Message):
    __slots__ = ("mappings",)
    MAPPINGS_FIELD_NUMBER: _ClassVar[int]
    mappings: _containers.RepeatedCompositeFieldContainer[RemotePathMapping]
    def __init__(self, mappings: _Optional[_Iterable[_Union[RemotePathMapping, _Mapping]]] = ...) -> None: ...

class CreateRemotePathMappingRequest(_message.Message):
    __slots__ = ("mapping",)
    MAPPING_FIELD_NUMBER: _ClassVar[int]
    mapping: RemotePathMapping
    def __init__(self, mapping: _Optional[_Union[RemotePathMapping, _Mapping]] = ...) -> None: ...

class CreateRemotePathMappingResponse(_message.Message):
    __slots__ = ("mapping",)
    MAPPING_FIELD_NUMBER: _ClassVar[int]
    mapping: RemotePathMapping
    def __init__(self, mapping: _Optional[_Union[RemotePathMapping, _Mapping]] = ...) -> None: ...

class DeleteRemotePathMappingRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteRemotePathMappingResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

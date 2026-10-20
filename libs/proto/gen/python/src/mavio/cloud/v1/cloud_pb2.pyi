import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.user.v1 import user_pb2 as _user_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class AccountKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ACCOUNT_KIND_UNSPECIFIED: _ClassVar[AccountKind]
    ACCOUNT_KIND_FULL: _ClassVar[AccountKind]
    ACCOUNT_KIND_MANAGED: _ClassVar[AccountKind]
ACCOUNT_KIND_UNSPECIFIED: AccountKind
ACCOUNT_KIND_FULL: AccountKind
ACCOUNT_KIND_MANAGED: AccountKind

class Account(_message.Message):
    __slots__ = ("id", "user_name", "display_name", "kind", "restriction")
    ID_FIELD_NUMBER: _ClassVar[int]
    USER_NAME_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    RESTRICTION_FIELD_NUMBER: _ClassVar[int]
    id: str
    user_name: str
    display_name: str
    kind: AccountKind
    restriction: _user_pb2.RestrictionProfile
    def __init__(self, id: _Optional[str] = ..., user_name: _Optional[str] = ..., display_name: _Optional[str] = ..., kind: _Optional[_Union[AccountKind, str]] = ..., restriction: _Optional[_Union[_user_pb2.RestrictionProfile, str]] = ...) -> None: ...

class Access(_message.Message):
    __slots__ = ("all_libraries", "library_ids", "max_parental_rating", "block_unrated", "allowed_labels", "blocked_labels", "allow_download")
    ALL_LIBRARIES_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_IDS_FIELD_NUMBER: _ClassVar[int]
    MAX_PARENTAL_RATING_FIELD_NUMBER: _ClassVar[int]
    BLOCK_UNRATED_FIELD_NUMBER: _ClassVar[int]
    ALLOWED_LABELS_FIELD_NUMBER: _ClassVar[int]
    BLOCKED_LABELS_FIELD_NUMBER: _ClassVar[int]
    ALLOW_DOWNLOAD_FIELD_NUMBER: _ClassVar[int]
    all_libraries: bool
    library_ids: _containers.RepeatedScalarFieldContainer[str]
    max_parental_rating: int
    block_unrated: bool
    allowed_labels: _containers.RepeatedScalarFieldContainer[str]
    blocked_labels: _containers.RepeatedScalarFieldContainer[str]
    allow_download: bool
    def __init__(self, all_libraries: _Optional[bool] = ..., library_ids: _Optional[_Iterable[str]] = ..., max_parental_rating: _Optional[int] = ..., block_unrated: _Optional[bool] = ..., allowed_labels: _Optional[_Iterable[str]] = ..., blocked_labels: _Optional[_Iterable[str]] = ..., allow_download: _Optional[bool] = ...) -> None: ...

class CreateAuthorizationRequestRequest(_message.Message):
    __slots__ = ("public_key", "name", "version", "redirect_uri", "code_challenge", "device_key", "state", "claim")
    PUBLIC_KEY_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    REDIRECT_URI_FIELD_NUMBER: _ClassVar[int]
    CODE_CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    DEVICE_KEY_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    CLAIM_FIELD_NUMBER: _ClassVar[int]
    public_key: bytes
    name: str
    version: str
    redirect_uri: str
    code_challenge: str
    device_key: bytes
    state: str
    claim: bool
    def __init__(self, public_key: _Optional[bytes] = ..., name: _Optional[str] = ..., version: _Optional[str] = ..., redirect_uri: _Optional[str] = ..., code_challenge: _Optional[str] = ..., device_key: _Optional[bytes] = ..., state: _Optional[str] = ..., claim: _Optional[bool] = ...) -> None: ...

class CreateAuthorizationRequestResponse(_message.Message):
    __slots__ = ("authorization_request", "url", "expire_time")
    AUTHORIZATION_REQUEST_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    authorization_request: str
    url: str
    expire_time: _timestamp_pb2.Timestamp
    def __init__(self, authorization_request: _Optional[str] = ..., url: _Optional[str] = ..., expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ClaimServerRequest(_message.Message):
    __slots__ = ("claim_code", "public_key", "name", "version", "authorization_request")
    CLAIM_CODE_FIELD_NUMBER: _ClassVar[int]
    PUBLIC_KEY_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    AUTHORIZATION_REQUEST_FIELD_NUMBER: _ClassVar[int]
    claim_code: str
    public_key: bytes
    name: str
    version: str
    authorization_request: str
    def __init__(self, claim_code: _Optional[str] = ..., public_key: _Optional[bytes] = ..., name: _Optional[str] = ..., version: _Optional[str] = ..., authorization_request: _Optional[str] = ...) -> None: ...

class ClaimServerResponse(_message.Message):
    __slots__ = ("administrator", "cursor")
    ADMINISTRATOR_FIELD_NUMBER: _ClassVar[int]
    CURSOR_FIELD_NUMBER: _ClassVar[int]
    administrator: Account
    cursor: int
    def __init__(self, administrator: _Optional[_Union[Account, _Mapping]] = ..., cursor: _Optional[int] = ...) -> None: ...

class ListChangesRequest(_message.Message):
    __slots__ = ("after", "page_size")
    AFTER_FIELD_NUMBER: _ClassVar[int]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    after: int
    page_size: int
    def __init__(self, after: _Optional[int] = ..., page_size: _Optional[int] = ...) -> None: ...

class ListChangesResponse(_message.Message):
    __slots__ = ("changes", "more", "cursor")
    CHANGES_FIELD_NUMBER: _ClassVar[int]
    MORE_FIELD_NUMBER: _ClassVar[int]
    CURSOR_FIELD_NUMBER: _ClassVar[int]
    changes: _containers.RepeatedCompositeFieldContainer[Change]
    more: bool
    cursor: int
    def __init__(self, changes: _Optional[_Iterable[_Union[Change, _Mapping]]] = ..., more: _Optional[bool] = ..., cursor: _Optional[int] = ...) -> None: ...

class Change(_message.Message):
    __slots__ = ("cursor", "create_time", "access_granted", "access_revoked", "account_changed", "account_revoked")
    CURSOR_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    ACCESS_GRANTED_FIELD_NUMBER: _ClassVar[int]
    ACCESS_REVOKED_FIELD_NUMBER: _ClassVar[int]
    ACCOUNT_CHANGED_FIELD_NUMBER: _ClassVar[int]
    ACCOUNT_REVOKED_FIELD_NUMBER: _ClassVar[int]
    cursor: int
    create_time: _timestamp_pb2.Timestamp
    access_granted: AccessGranted
    access_revoked: AccessRevoked
    account_changed: Account
    account_revoked: AccountRevoked
    def __init__(self, cursor: _Optional[int] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., access_granted: _Optional[_Union[AccessGranted, _Mapping]] = ..., access_revoked: _Optional[_Union[AccessRevoked, _Mapping]] = ..., account_changed: _Optional[_Union[Account, _Mapping]] = ..., account_revoked: _Optional[_Union[AccountRevoked, _Mapping]] = ...) -> None: ...

class AccessGranted(_message.Message):
    __slots__ = ("account", "access")
    ACCOUNT_FIELD_NUMBER: _ClassVar[int]
    ACCESS_FIELD_NUMBER: _ClassVar[int]
    account: Account
    access: Access
    def __init__(self, account: _Optional[_Union[Account, _Mapping]] = ..., access: _Optional[_Union[Access, _Mapping]] = ...) -> None: ...

class AccessRevoked(_message.Message):
    __slots__ = ("account_id",)
    ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    account_id: str
    def __init__(self, account_id: _Optional[str] = ...) -> None: ...

class AccountRevoked(_message.Message):
    __slots__ = ("account_id", "reason")
    class Reason(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        REASON_UNSPECIFIED: _ClassVar[AccountRevoked.Reason]
        REASON_SUSPENDED: _ClassVar[AccountRevoked.Reason]
        REASON_DELETED: _ClassVar[AccountRevoked.Reason]
        REASON_COMPROMISED: _ClassVar[AccountRevoked.Reason]
    REASON_UNSPECIFIED: AccountRevoked.Reason
    REASON_SUSPENDED: AccountRevoked.Reason
    REASON_DELETED: AccountRevoked.Reason
    REASON_COMPROMISED: AccountRevoked.Reason
    ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    account_id: str
    reason: AccountRevoked.Reason
    def __init__(self, account_id: _Optional[str] = ..., reason: _Optional[_Union[AccountRevoked.Reason, str]] = ...) -> None: ...

class Library(_message.Message):
    __slots__ = ("id", "name", "kind")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    kind: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., kind: _Optional[str] = ...) -> None: ...

class PublishLibrariesRequest(_message.Message):
    __slots__ = ("libraries",)
    LIBRARIES_FIELD_NUMBER: _ClassVar[int]
    libraries: _containers.RepeatedCompositeFieldContainer[Library]
    def __init__(self, libraries: _Optional[_Iterable[_Union[Library, _Mapping]]] = ...) -> None: ...

class PublishLibrariesResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class PublishConnectionsRequest(_message.Message):
    __slots__ = ("urls",)
    URLS_FIELD_NUMBER: _ClassVar[int]
    urls: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, urls: _Optional[_Iterable[str]] = ...) -> None: ...

class PublishConnectionsResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class Notice(_message.Message):
    __slots__ = ("cursor",)
    CURSOR_FIELD_NUMBER: _ClassVar[int]
    cursor: int
    def __init__(self, cursor: _Optional[int] = ...) -> None: ...

class SetAcmeChallengeRequest(_message.Message):
    __slots__ = ("values",)
    VALUES_FIELD_NUMBER: _ClassVar[int]
    values: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, values: _Optional[_Iterable[str]] = ...) -> None: ...

class SetAcmeChallengeResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ClearAcmeChallengeRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ClearAcmeChallengeResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CreateAcmeBindingRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CreateAcmeBindingResponse(_message.Message):
    __slots__ = ("key_id", "hmac_key")
    KEY_ID_FIELD_NUMBER: _ClassVar[int]
    HMAC_KEY_FIELD_NUMBER: _ClassVar[int]
    key_id: str
    hmac_key: bytes
    def __init__(self, key_id: _Optional[str] = ..., hmac_key: _Optional[bytes] = ...) -> None: ...

class ListRelaysRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListRelaysResponse(_message.Message):
    __slots__ = ("relays",)
    RELAYS_FIELD_NUMBER: _ClassVar[int]
    relays: _containers.RepeatedCompositeFieldContainer[Relay]
    def __init__(self, relays: _Optional[_Iterable[_Union[Relay, _Mapping]]] = ...) -> None: ...

class Relay(_message.Message):
    __slots__ = ("region", "url")
    REGION_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    region: str
    url: str
    def __init__(self, region: _Optional[str] = ..., url: _Optional[str] = ...) -> None: ...

class SetRelayRegionRequest(_message.Message):
    __slots__ = ("region",)
    REGION_FIELD_NUMBER: _ClassVar[int]
    region: str
    def __init__(self, region: _Optional[str] = ...) -> None: ...

class SetRelayRegionResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

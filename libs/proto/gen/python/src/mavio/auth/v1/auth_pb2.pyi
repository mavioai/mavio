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

class AccountMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ACCOUNT_MODE_UNSPECIFIED: _ClassVar[AccountMode]
    ACCOUNT_MODE_STANDALONE: _ClassVar[AccountMode]
    ACCOUNT_MODE_MAVIO_DEV: _ClassVar[AccountMode]
ACCOUNT_MODE_UNSPECIFIED: AccountMode
ACCOUNT_MODE_STANDALONE: AccountMode
ACCOUNT_MODE_MAVIO_DEV: AccountMode

class Device(_message.Message):
    __slots__ = ("id", "name", "client", "client_version", "public_key")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CLIENT_FIELD_NUMBER: _ClassVar[int]
    CLIENT_VERSION_FIELD_NUMBER: _ClassVar[int]
    PUBLIC_KEY_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    client: str
    client_version: str
    public_key: bytes
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., client: _Optional[str] = ..., client_version: _Optional[str] = ..., public_key: _Optional[bytes] = ...) -> None: ...

class Session(_message.Message):
    __slots__ = ("id", "device", "create_time", "last_seen_time", "current")
    ID_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    LAST_SEEN_TIME_FIELD_NUMBER: _ClassVar[int]
    CURRENT_FIELD_NUMBER: _ClassVar[int]
    id: str
    device: Device
    create_time: _timestamp_pb2.Timestamp
    last_seen_time: _timestamp_pb2.Timestamp
    current: bool
    def __init__(self, id: _Optional[str] = ..., device: _Optional[_Union[Device, _Mapping]] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., last_seen_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., current: _Optional[bool] = ...) -> None: ...

class GetAuthInfoRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetAuthInfoResponse(_message.Message):
    __slots__ = ("setup_required", "server_id", "server_name", "version", "mode", "mavio_dev_url")
    SETUP_REQUIRED_FIELD_NUMBER: _ClassVar[int]
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    SERVER_NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    MAVIO_DEV_URL_FIELD_NUMBER: _ClassVar[int]
    setup_required: bool
    server_id: str
    server_name: str
    version: str
    mode: AccountMode
    mavio_dev_url: str
    def __init__(self, setup_required: _Optional[bool] = ..., server_id: _Optional[str] = ..., server_name: _Optional[str] = ..., version: _Optional[str] = ..., mode: _Optional[_Union[AccountMode, str]] = ..., mavio_dev_url: _Optional[str] = ...) -> None: ...

class CreateFirstUserRequest(_message.Message):
    __slots__ = ("name", "password", "device")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    name: str
    password: str
    device: Device
    def __init__(self, name: _Optional[str] = ..., password: _Optional[str] = ..., device: _Optional[_Union[Device, _Mapping]] = ...) -> None: ...

class CreateFirstUserResponse(_message.Message):
    __slots__ = ("access_token", "user", "session", "access_token_expire_time")
    ACCESS_TOKEN_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    ACCESS_TOKEN_EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    access_token: str
    user: _user_pb2.User
    session: Session
    access_token_expire_time: _timestamp_pb2.Timestamp
    def __init__(self, access_token: _Optional[str] = ..., user: _Optional[_Union[_user_pb2.User, _Mapping]] = ..., session: _Optional[_Union[Session, _Mapping]] = ..., access_token_expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class LoginRequest(_message.Message):
    __slots__ = ("name", "password", "device", "totp_code")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    TOTP_CODE_FIELD_NUMBER: _ClassVar[int]
    name: str
    password: str
    device: Device
    totp_code: str
    def __init__(self, name: _Optional[str] = ..., password: _Optional[str] = ..., device: _Optional[_Union[Device, _Mapping]] = ..., totp_code: _Optional[str] = ...) -> None: ...

class LoginResponse(_message.Message):
    __slots__ = ("access_token", "user", "session", "access_token_expire_time")
    ACCESS_TOKEN_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    ACCESS_TOKEN_EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    access_token: str
    user: _user_pb2.User
    session: Session
    access_token_expire_time: _timestamp_pb2.Timestamp
    def __init__(self, access_token: _Optional[str] = ..., user: _Optional[_Union[_user_pb2.User, _Mapping]] = ..., session: _Optional[_Union[Session, _Mapping]] = ..., access_token_expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class StartPasskeySignInRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StartPasskeySignInResponse(_message.Message):
    __slots__ = ("ceremony_id", "options_json")
    CEREMONY_ID_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_JSON_FIELD_NUMBER: _ClassVar[int]
    ceremony_id: str
    options_json: str
    def __init__(self, ceremony_id: _Optional[str] = ..., options_json: _Optional[str] = ...) -> None: ...

class FinishPasskeySignInRequest(_message.Message):
    __slots__ = ("ceremony_id", "credential_json", "device")
    CEREMONY_ID_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_JSON_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    ceremony_id: str
    credential_json: str
    device: Device
    def __init__(self, ceremony_id: _Optional[str] = ..., credential_json: _Optional[str] = ..., device: _Optional[_Union[Device, _Mapping]] = ...) -> None: ...

class FinishPasskeySignInResponse(_message.Message):
    __slots__ = ("access_token", "user", "session", "access_token_expire_time")
    ACCESS_TOKEN_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    ACCESS_TOKEN_EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    access_token: str
    user: _user_pb2.User
    session: Session
    access_token_expire_time: _timestamp_pb2.Timestamp
    def __init__(self, access_token: _Optional[str] = ..., user: _Optional[_Union[_user_pb2.User, _Mapping]] = ..., session: _Optional[_Union[Session, _Mapping]] = ..., access_token_expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class StartRefreshRequest(_message.Message):
    __slots__ = ("session_id",)
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    def __init__(self, session_id: _Optional[str] = ...) -> None: ...

class StartRefreshResponse(_message.Message):
    __slots__ = ("nonce", "expire_time")
    NONCE_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    nonce: bytes
    expire_time: _timestamp_pb2.Timestamp
    def __init__(self, nonce: _Optional[bytes] = ..., expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class RefreshSessionRequest(_message.Message):
    __slots__ = ("session_id", "nonce", "signature")
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    NONCE_FIELD_NUMBER: _ClassVar[int]
    SIGNATURE_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    nonce: bytes
    signature: bytes
    def __init__(self, session_id: _Optional[str] = ..., nonce: _Optional[bytes] = ..., signature: _Optional[bytes] = ...) -> None: ...

class RefreshSessionResponse(_message.Message):
    __slots__ = ("access_token", "access_token_expire_time")
    ACCESS_TOKEN_FIELD_NUMBER: _ClassVar[int]
    ACCESS_TOKEN_EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    access_token: str
    access_token_expire_time: _timestamp_pb2.Timestamp
    def __init__(self, access_token: _Optional[str] = ..., access_token_expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class LogoutRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class LogoutResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StartQuickConnectRequest(_message.Message):
    __slots__ = ("device",)
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    device: Device
    def __init__(self, device: _Optional[_Union[Device, _Mapping]] = ...) -> None: ...

class StartQuickConnectResponse(_message.Message):
    __slots__ = ("secret", "code", "expire_time")
    SECRET_FIELD_NUMBER: _ClassVar[int]
    CODE_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    secret: str
    code: str
    expire_time: _timestamp_pb2.Timestamp
    def __init__(self, secret: _Optional[str] = ..., code: _Optional[str] = ..., expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetQuickConnectStateRequest(_message.Message):
    __slots__ = ("secret",)
    SECRET_FIELD_NUMBER: _ClassVar[int]
    secret: str
    def __init__(self, secret: _Optional[str] = ...) -> None: ...

class GetQuickConnectStateResponse(_message.Message):
    __slots__ = ("authorized", "expire_time")
    AUTHORIZED_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    authorized: bool
    expire_time: _timestamp_pb2.Timestamp
    def __init__(self, authorized: _Optional[bool] = ..., expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class AuthorizeQuickConnectRequest(_message.Message):
    __slots__ = ("code",)
    CODE_FIELD_NUMBER: _ClassVar[int]
    code: str
    def __init__(self, code: _Optional[str] = ...) -> None: ...

class AuthorizeQuickConnectResponse(_message.Message):
    __slots__ = ("device",)
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    device: Device
    def __init__(self, device: _Optional[_Union[Device, _Mapping]] = ...) -> None: ...

class LoginWithQuickConnectRequest(_message.Message):
    __slots__ = ("secret",)
    SECRET_FIELD_NUMBER: _ClassVar[int]
    secret: str
    def __init__(self, secret: _Optional[str] = ...) -> None: ...

class LoginWithQuickConnectResponse(_message.Message):
    __slots__ = ("access_token", "user", "session", "access_token_expire_time")
    ACCESS_TOKEN_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    ACCESS_TOKEN_EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    access_token: str
    user: _user_pb2.User
    session: Session
    access_token_expire_time: _timestamp_pb2.Timestamp
    def __init__(self, access_token: _Optional[str] = ..., user: _Optional[_Union[_user_pb2.User, _Mapping]] = ..., session: _Optional[_Union[Session, _Mapping]] = ..., access_token_expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ApiKey(_message.Message):
    __slots__ = ("id", "name", "user_id", "create_time", "last_use_time")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    LAST_USE_TIME_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    user_id: str
    create_time: _timestamp_pb2.Timestamp
    last_use_time: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., user_id: _Optional[str] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., last_use_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class CreateApiKeyRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class CreateApiKeyResponse(_message.Message):
    __slots__ = ("api_key", "token")
    API_KEY_FIELD_NUMBER: _ClassVar[int]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    api_key: ApiKey
    token: str
    def __init__(self, api_key: _Optional[_Union[ApiKey, _Mapping]] = ..., token: _Optional[str] = ...) -> None: ...

class ListApiKeysRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListApiKeysResponse(_message.Message):
    __slots__ = ("api_keys",)
    API_KEYS_FIELD_NUMBER: _ClassVar[int]
    api_keys: _containers.RepeatedCompositeFieldContainer[ApiKey]
    def __init__(self, api_keys: _Optional[_Iterable[_Union[ApiKey, _Mapping]]] = ...) -> None: ...

class RevokeApiKeyRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class RevokeApiKeyResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ForgotPasswordRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ForgotPasswordResponse(_message.Message):
    __slots__ = ("expire_time",)
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    expire_time: _timestamp_pb2.Timestamp
    def __init__(self, expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ResetPasswordRequest(_message.Message):
    __slots__ = ("name", "pin", "new_password")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    NEW_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    name: str
    pin: str
    new_password: str
    def __init__(self, name: _Optional[str] = ..., pin: _Optional[str] = ..., new_password: _Optional[str] = ...) -> None: ...

class ResetPasswordResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetInvitationRequest(_message.Message):
    __slots__ = ("token",)
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    token: str
    def __init__(self, token: _Optional[str] = ...) -> None: ...

class GetInvitationResponse(_message.Message):
    __slots__ = ("user_name", "expire_time")
    USER_NAME_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    user_name: str
    expire_time: _timestamp_pb2.Timestamp
    def __init__(self, user_name: _Optional[str] = ..., expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class StartInvitationPasskeyRequest(_message.Message):
    __slots__ = ("token", "name")
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    token: str
    name: str
    def __init__(self, token: _Optional[str] = ..., name: _Optional[str] = ...) -> None: ...

class StartInvitationPasskeyResponse(_message.Message):
    __slots__ = ("ceremony_id", "options_json")
    CEREMONY_ID_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_JSON_FIELD_NUMBER: _ClassVar[int]
    ceremony_id: str
    options_json: str
    def __init__(self, ceremony_id: _Optional[str] = ..., options_json: _Optional[str] = ...) -> None: ...

class AcceptInvitationRequest(_message.Message):
    __slots__ = ("token", "name", "password", "ceremony_id", "credential_json", "passkey_name", "device")
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    CEREMONY_ID_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_JSON_FIELD_NUMBER: _ClassVar[int]
    PASSKEY_NAME_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    token: str
    name: str
    password: str
    ceremony_id: str
    credential_json: str
    passkey_name: str
    device: Device
    def __init__(self, token: _Optional[str] = ..., name: _Optional[str] = ..., password: _Optional[str] = ..., ceremony_id: _Optional[str] = ..., credential_json: _Optional[str] = ..., passkey_name: _Optional[str] = ..., device: _Optional[_Union[Device, _Mapping]] = ...) -> None: ...

class AcceptInvitationResponse(_message.Message):
    __slots__ = ("access_token", "user", "session", "access_token_expire_time")
    ACCESS_TOKEN_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    ACCESS_TOKEN_EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    access_token: str
    user: _user_pb2.User
    session: Session
    access_token_expire_time: _timestamp_pb2.Timestamp
    def __init__(self, access_token: _Optional[str] = ..., user: _Optional[_Union[_user_pb2.User, _Mapping]] = ..., session: _Optional[_Union[Session, _Mapping]] = ..., access_token_expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListSwitchableUsersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListSwitchableUsersResponse(_message.Message):
    __slots__ = ("users",)
    USERS_FIELD_NUMBER: _ClassVar[int]
    users: _containers.RepeatedCompositeFieldContainer[SwitchableUser]
    def __init__(self, users: _Optional[_Iterable[_Union[SwitchableUser, _Mapping]]] = ...) -> None: ...

class SwitchableUser(_message.Message):
    __slots__ = ("id", "name", "admin", "managed", "has_pin", "current")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ADMIN_FIELD_NUMBER: _ClassVar[int]
    MANAGED_FIELD_NUMBER: _ClassVar[int]
    HAS_PIN_FIELD_NUMBER: _ClassVar[int]
    CURRENT_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    admin: bool
    managed: bool
    has_pin: bool
    current: bool
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., admin: _Optional[bool] = ..., managed: _Optional[bool] = ..., has_pin: _Optional[bool] = ..., current: _Optional[bool] = ...) -> None: ...

class SwitchUserRequest(_message.Message):
    __slots__ = ("user_id", "pin")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    pin: str
    def __init__(self, user_id: _Optional[str] = ..., pin: _Optional[str] = ...) -> None: ...

class SwitchUserResponse(_message.Message):
    __slots__ = ("user",)
    USER_FIELD_NUMBER: _ClassVar[int]
    user: _user_pb2.User
    def __init__(self, user: _Optional[_Union[_user_pb2.User, _Mapping]] = ...) -> None: ...

class StartSignInRequest(_message.Message):
    __slots__ = ("redirect_uri", "code_challenge", "device_key", "state")
    REDIRECT_URI_FIELD_NUMBER: _ClassVar[int]
    CODE_CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    DEVICE_KEY_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    redirect_uri: str
    code_challenge: str
    device_key: bytes
    state: str
    def __init__(self, redirect_uri: _Optional[str] = ..., code_challenge: _Optional[str] = ..., device_key: _Optional[bytes] = ..., state: _Optional[str] = ...) -> None: ...

class StartSignInResponse(_message.Message):
    __slots__ = ("url", "expire_time")
    URL_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    url: str
    expire_time: _timestamp_pb2.Timestamp
    def __init__(self, url: _Optional[str] = ..., expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class SignInWithAssertionRequest(_message.Message):
    __slots__ = ("assertion", "device", "signature")
    ASSERTION_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    SIGNATURE_FIELD_NUMBER: _ClassVar[int]
    assertion: str
    device: Device
    signature: bytes
    def __init__(self, assertion: _Optional[str] = ..., device: _Optional[_Union[Device, _Mapping]] = ..., signature: _Optional[bytes] = ...) -> None: ...

class SignInWithAssertionResponse(_message.Message):
    __slots__ = ("access_token", "user", "session", "access_token_expire_time")
    ACCESS_TOKEN_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    ACCESS_TOKEN_EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    access_token: str
    user: _user_pb2.User
    session: Session
    access_token_expire_time: _timestamp_pb2.Timestamp
    def __init__(self, access_token: _Optional[str] = ..., user: _Optional[_Union[_user_pb2.User, _Mapping]] = ..., session: _Optional[_Union[Session, _Mapping]] = ..., access_token_expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

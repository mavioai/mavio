import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.cloud.v1 import cloud_pb2 as _cloud_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Server(_message.Message):
    __slots__ = ("id", "public_key", "name", "version", "administrator", "access", "libraries", "claim_time", "connections", "connection_checks")
    ID_FIELD_NUMBER: _ClassVar[int]
    PUBLIC_KEY_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    ADMINISTRATOR_FIELD_NUMBER: _ClassVar[int]
    ACCESS_FIELD_NUMBER: _ClassVar[int]
    LIBRARIES_FIELD_NUMBER: _ClassVar[int]
    CLAIM_TIME_FIELD_NUMBER: _ClassVar[int]
    CONNECTIONS_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_CHECKS_FIELD_NUMBER: _ClassVar[int]
    id: str
    public_key: bytes
    name: str
    version: str
    administrator: _cloud_pb2.Account
    access: _cloud_pb2.Access
    libraries: _containers.RepeatedCompositeFieldContainer[_cloud_pb2.Library]
    claim_time: _timestamp_pb2.Timestamp
    connections: _containers.RepeatedScalarFieldContainer[str]
    connection_checks: _containers.RepeatedCompositeFieldContainer[ConnectionCheck]
    def __init__(self, id: _Optional[str] = ..., public_key: _Optional[bytes] = ..., name: _Optional[str] = ..., version: _Optional[str] = ..., administrator: _Optional[_Union[_cloud_pb2.Account, _Mapping]] = ..., access: _Optional[_Union[_cloud_pb2.Access, _Mapping]] = ..., libraries: _Optional[_Iterable[_Union[_cloud_pb2.Library, _Mapping]]] = ..., claim_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., connections: _Optional[_Iterable[str]] = ..., connection_checks: _Optional[_Iterable[_Union[ConnectionCheck, _Mapping]]] = ...) -> None: ...

class ConnectionCheck(_message.Message):
    __slots__ = ("url", "reachable", "check_time")
    URL_FIELD_NUMBER: _ClassVar[int]
    REACHABLE_FIELD_NUMBER: _ClassVar[int]
    CHECK_TIME_FIELD_NUMBER: _ClassVar[int]
    url: str
    reachable: bool
    check_time: _timestamp_pb2.Timestamp
    def __init__(self, url: _Optional[str] = ..., reachable: _Optional[bool] = ..., check_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class AccessRecord(_message.Message):
    __slots__ = ("account", "access", "invitation", "create_time", "update_time")
    ACCOUNT_FIELD_NUMBER: _ClassVar[int]
    ACCESS_FIELD_NUMBER: _ClassVar[int]
    INVITATION_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    account: _cloud_pb2.Account
    access: _cloud_pb2.Access
    invitation: bool
    create_time: _timestamp_pb2.Timestamp
    update_time: _timestamp_pb2.Timestamp
    def __init__(self, account: _Optional[_Union[_cloud_pb2.Account, _Mapping]] = ..., access: _Optional[_Union[_cloud_pb2.Access, _Mapping]] = ..., invitation: _Optional[bool] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class Invitation(_message.Message):
    __slots__ = ("server_id", "server_name", "administrator", "access", "create_time")
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    SERVER_NAME_FIELD_NUMBER: _ClassVar[int]
    ADMINISTRATOR_FIELD_NUMBER: _ClassVar[int]
    ACCESS_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    server_name: str
    administrator: _cloud_pb2.Account
    access: _cloud_pb2.Access
    create_time: _timestamp_pb2.Timestamp
    def __init__(self, server_id: _Optional[str] = ..., server_name: _Optional[str] = ..., administrator: _Optional[_Union[_cloud_pb2.Account, _Mapping]] = ..., access: _Optional[_Union[_cloud_pb2.Access, _Mapping]] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetAccountRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetAccountResponse(_message.Message):
    __slots__ = ("account", "email", "pass_expire_time", "effective_pass", "has_pin")
    ACCOUNT_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    PASS_EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    EFFECTIVE_PASS_FIELD_NUMBER: _ClassVar[int]
    HAS_PIN_FIELD_NUMBER: _ClassVar[int]
    account: _cloud_pb2.Account
    email: str
    pass_expire_time: _timestamp_pb2.Timestamp
    effective_pass: bool
    has_pin: bool
    def __init__(self, account: _Optional[_Union[_cloud_pb2.Account, _Mapping]] = ..., email: _Optional[str] = ..., pass_expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., effective_pass: _Optional[bool] = ..., has_pin: _Optional[bool] = ...) -> None: ...

class ReportCompromisedRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ReportCompromisedResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CreateClaimCodeRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CreateClaimCodeResponse(_message.Message):
    __slots__ = ("claim_code", "expire_time")
    CLAIM_CODE_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    claim_code: str
    expire_time: _timestamp_pb2.Timestamp
    def __init__(self, claim_code: _Optional[str] = ..., expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListServersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListServersResponse(_message.Message):
    __slots__ = ("servers",)
    SERVERS_FIELD_NUMBER: _ClassVar[int]
    servers: _containers.RepeatedCompositeFieldContainer[Server]
    def __init__(self, servers: _Optional[_Iterable[_Union[Server, _Mapping]]] = ...) -> None: ...

class RemoveServerRequest(_message.Message):
    __slots__ = ("server_id",)
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    def __init__(self, server_id: _Optional[str] = ...) -> None: ...

class RemoveServerResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CreateAssertionRequest(_message.Message):
    __slots__ = ("server_id", "device_key")
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    DEVICE_KEY_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    device_key: bytes
    def __init__(self, server_id: _Optional[str] = ..., device_key: _Optional[bytes] = ...) -> None: ...

class CreateAssertionResponse(_message.Message):
    __slots__ = ("assertion", "expire_time")
    ASSERTION_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    assertion: str
    expire_time: _timestamp_pb2.Timestamp
    def __init__(self, assertion: _Optional[str] = ..., expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class CreateAuthorizationCodeRequest(_message.Message):
    __slots__ = ("redirect_uri", "code_challenge", "device_key", "client_id")
    REDIRECT_URI_FIELD_NUMBER: _ClassVar[int]
    CODE_CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    DEVICE_KEY_FIELD_NUMBER: _ClassVar[int]
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    redirect_uri: str
    code_challenge: str
    device_key: bytes
    client_id: str
    def __init__(self, redirect_uri: _Optional[str] = ..., code_challenge: _Optional[str] = ..., device_key: _Optional[bytes] = ..., client_id: _Optional[str] = ...) -> None: ...

class CreateAuthorizationCodeResponse(_message.Message):
    __slots__ = ("code",)
    CODE_FIELD_NUMBER: _ClassVar[int]
    code: str
    def __init__(self, code: _Optional[str] = ...) -> None: ...

class GetAuthorizationRequestRequest(_message.Message):
    __slots__ = ("authorization_request",)
    AUTHORIZATION_REQUEST_FIELD_NUMBER: _ClassVar[int]
    authorization_request: str
    def __init__(self, authorization_request: _Optional[str] = ...) -> None: ...

class GetAuthorizationRequestResponse(_message.Message):
    __slots__ = ("server_id", "server_name", "redirect_uri", "claim", "claimed", "administrator", "expire_time")
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    SERVER_NAME_FIELD_NUMBER: _ClassVar[int]
    REDIRECT_URI_FIELD_NUMBER: _ClassVar[int]
    CLAIM_FIELD_NUMBER: _ClassVar[int]
    CLAIMED_FIELD_NUMBER: _ClassVar[int]
    ADMINISTRATOR_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    server_name: str
    redirect_uri: str
    claim: bool
    claimed: bool
    administrator: bool
    expire_time: _timestamp_pb2.Timestamp
    def __init__(self, server_id: _Optional[str] = ..., server_name: _Optional[str] = ..., redirect_uri: _Optional[str] = ..., claim: _Optional[bool] = ..., claimed: _Optional[bool] = ..., administrator: _Optional[bool] = ..., expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class AnswerAuthorizationRequestRequest(_message.Message):
    __slots__ = ("authorization_request", "approve")
    AUTHORIZATION_REQUEST_FIELD_NUMBER: _ClassVar[int]
    APPROVE_FIELD_NUMBER: _ClassVar[int]
    authorization_request: str
    approve: bool
    def __init__(self, authorization_request: _Optional[str] = ..., approve: _Optional[bool] = ...) -> None: ...

class AnswerAuthorizationRequestResponse(_message.Message):
    __slots__ = ("redirect_url",)
    REDIRECT_URL_FIELD_NUMBER: _ClassVar[int]
    redirect_url: str
    def __init__(self, redirect_url: _Optional[str] = ...) -> None: ...

class GetLinkCodeRequest(_message.Message):
    __slots__ = ("user_code",)
    USER_CODE_FIELD_NUMBER: _ClassVar[int]
    user_code: str
    def __init__(self, user_code: _Optional[str] = ...) -> None: ...

class GetLinkCodeResponse(_message.Message):
    __slots__ = ("server_id", "server_name", "client_id")
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    SERVER_NAME_FIELD_NUMBER: _ClassVar[int]
    CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    server_name: str
    client_id: str
    def __init__(self, server_id: _Optional[str] = ..., server_name: _Optional[str] = ..., client_id: _Optional[str] = ...) -> None: ...

class AnswerLinkCodeRequest(_message.Message):
    __slots__ = ("user_code", "approve")
    USER_CODE_FIELD_NUMBER: _ClassVar[int]
    APPROVE_FIELD_NUMBER: _ClassVar[int]
    user_code: str
    approve: bool
    def __init__(self, user_code: _Optional[str] = ..., approve: _Optional[bool] = ...) -> None: ...

class AnswerLinkCodeResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CreateRelayTicketRequest(_message.Message):
    __slots__ = ("server_id",)
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    def __init__(self, server_id: _Optional[str] = ...) -> None: ...

class CreateRelayTicketResponse(_message.Message):
    __slots__ = ("host", "expire_time")
    HOST_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    host: str
    expire_time: _timestamp_pb2.Timestamp
    def __init__(self, host: _Optional[str] = ..., expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListAccessRequest(_message.Message):
    __slots__ = ("server_id",)
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    def __init__(self, server_id: _Optional[str] = ...) -> None: ...

class ListAccessResponse(_message.Message):
    __slots__ = ("records",)
    RECORDS_FIELD_NUMBER: _ClassVar[int]
    records: _containers.RepeatedCompositeFieldContainer[AccessRecord]
    def __init__(self, records: _Optional[_Iterable[_Union[AccessRecord, _Mapping]]] = ...) -> None: ...

class GrantAccessRequest(_message.Message):
    __slots__ = ("server_id", "user_name", "access")
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    USER_NAME_FIELD_NUMBER: _ClassVar[int]
    ACCESS_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    user_name: str
    access: _cloud_pb2.Access
    def __init__(self, server_id: _Optional[str] = ..., user_name: _Optional[str] = ..., access: _Optional[_Union[_cloud_pb2.Access, _Mapping]] = ...) -> None: ...

class GrantAccessResponse(_message.Message):
    __slots__ = ("record",)
    RECORD_FIELD_NUMBER: _ClassVar[int]
    record: AccessRecord
    def __init__(self, record: _Optional[_Union[AccessRecord, _Mapping]] = ...) -> None: ...

class RevokeAccessRequest(_message.Message):
    __slots__ = ("server_id", "account_id")
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    account_id: str
    def __init__(self, server_id: _Optional[str] = ..., account_id: _Optional[str] = ...) -> None: ...

class RevokeAccessResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListInvitationsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListInvitationsResponse(_message.Message):
    __slots__ = ("invitations",)
    INVITATIONS_FIELD_NUMBER: _ClassVar[int]
    invitations: _containers.RepeatedCompositeFieldContainer[Invitation]
    def __init__(self, invitations: _Optional[_Iterable[_Union[Invitation, _Mapping]]] = ...) -> None: ...

class AcceptInvitationRequest(_message.Message):
    __slots__ = ("server_id",)
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    def __init__(self, server_id: _Optional[str] = ...) -> None: ...

class AcceptInvitationResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class DeclineInvitationRequest(_message.Message):
    __slots__ = ("server_id",)
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    def __init__(self, server_id: _Optional[str] = ...) -> None: ...

class DeclineInvitationResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class LeaveServerRequest(_message.Message):
    __slots__ = ("server_id",)
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    def __init__(self, server_id: _Optional[str] = ...) -> None: ...

class LeaveServerResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

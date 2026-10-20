import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.cloud.v1 import cloud_pb2 as _cloud_pb2
from mavio.user.v1 import user_pb2 as _user_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Friend(_message.Message):
    __slots__ = ("account", "state", "create_time")
    class State(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        STATE_UNSPECIFIED: _ClassVar[Friend.State]
        STATE_FRIEND: _ClassVar[Friend.State]
        STATE_RECEIVED: _ClassVar[Friend.State]
        STATE_SENT: _ClassVar[Friend.State]
    STATE_UNSPECIFIED: Friend.State
    STATE_FRIEND: Friend.State
    STATE_RECEIVED: Friend.State
    STATE_SENT: Friend.State
    ACCOUNT_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    account: _cloud_pb2.Account
    state: Friend.State
    create_time: _timestamp_pb2.Timestamp
    def __init__(self, account: _Optional[_Union[_cloud_pb2.Account, _Mapping]] = ..., state: _Optional[_Union[Friend.State, str]] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class Home(_message.Message):
    __slots__ = ("id", "name", "administrator", "members", "invitations", "frozen", "create_time")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ADMINISTRATOR_FIELD_NUMBER: _ClassVar[int]
    MEMBERS_FIELD_NUMBER: _ClassVar[int]
    INVITATIONS_FIELD_NUMBER: _ClassVar[int]
    FROZEN_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    administrator: _cloud_pb2.Account
    members: _containers.RepeatedCompositeFieldContainer[HomeMember]
    invitations: _containers.RepeatedCompositeFieldContainer[HomeInvitation]
    frozen: bool
    create_time: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., administrator: _Optional[_Union[_cloud_pb2.Account, _Mapping]] = ..., members: _Optional[_Iterable[_Union[HomeMember, _Mapping]]] = ..., invitations: _Optional[_Iterable[_Union[HomeInvitation, _Mapping]]] = ..., frozen: _Optional[bool] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class HomeMember(_message.Message):
    __slots__ = ("account", "has_pin", "servers", "join_time")
    ACCOUNT_FIELD_NUMBER: _ClassVar[int]
    HAS_PIN_FIELD_NUMBER: _ClassVar[int]
    SERVERS_FIELD_NUMBER: _ClassVar[int]
    JOIN_TIME_FIELD_NUMBER: _ClassVar[int]
    account: _cloud_pb2.Account
    has_pin: bool
    servers: _containers.RepeatedCompositeFieldContainer[ServerSummary]
    join_time: _timestamp_pb2.Timestamp
    def __init__(self, account: _Optional[_Union[_cloud_pb2.Account, _Mapping]] = ..., has_pin: _Optional[bool] = ..., servers: _Optional[_Iterable[_Union[ServerSummary, _Mapping]]] = ..., join_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ServerSummary(_message.Message):
    __slots__ = ("id", "name")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ...) -> None: ...

class HomeInvitation(_message.Message):
    __slots__ = ("home_id", "home_name", "administrator", "account", "offers", "create_time")
    HOME_ID_FIELD_NUMBER: _ClassVar[int]
    HOME_NAME_FIELD_NUMBER: _ClassVar[int]
    ADMINISTRATOR_FIELD_NUMBER: _ClassVar[int]
    ACCOUNT_FIELD_NUMBER: _ClassVar[int]
    OFFERS_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    home_id: str
    home_name: str
    administrator: _cloud_pb2.Account
    account: _cloud_pb2.Account
    offers: _containers.RepeatedCompositeFieldContainer[ServerAccess]
    create_time: _timestamp_pb2.Timestamp
    def __init__(self, home_id: _Optional[str] = ..., home_name: _Optional[str] = ..., administrator: _Optional[_Union[_cloud_pb2.Account, _Mapping]] = ..., account: _Optional[_Union[_cloud_pb2.Account, _Mapping]] = ..., offers: _Optional[_Iterable[_Union[ServerAccess, _Mapping]]] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ServerAccess(_message.Message):
    __slots__ = ("server_id", "server_name", "access")
    SERVER_ID_FIELD_NUMBER: _ClassVar[int]
    SERVER_NAME_FIELD_NUMBER: _ClassVar[int]
    ACCESS_FIELD_NUMBER: _ClassVar[int]
    server_id: str
    server_name: str
    access: _cloud_pb2.Access
    def __init__(self, server_id: _Optional[str] = ..., server_name: _Optional[str] = ..., access: _Optional[_Union[_cloud_pb2.Access, _Mapping]] = ...) -> None: ...

class ListFriendsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListFriendsResponse(_message.Message):
    __slots__ = ("friends",)
    FRIENDS_FIELD_NUMBER: _ClassVar[int]
    friends: _containers.RepeatedCompositeFieldContainer[Friend]
    def __init__(self, friends: _Optional[_Iterable[_Union[Friend, _Mapping]]] = ...) -> None: ...

class AddFriendRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class AddFriendResponse(_message.Message):
    __slots__ = ("friend",)
    FRIEND_FIELD_NUMBER: _ClassVar[int]
    friend: Friend
    def __init__(self, friend: _Optional[_Union[Friend, _Mapping]] = ...) -> None: ...

class AcceptFriendRequest(_message.Message):
    __slots__ = ("account_id",)
    ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    account_id: str
    def __init__(self, account_id: _Optional[str] = ...) -> None: ...

class AcceptFriendResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RemoveFriendRequest(_message.Message):
    __slots__ = ("account_id",)
    ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    account_id: str
    def __init__(self, account_id: _Optional[str] = ...) -> None: ...

class RemoveFriendResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetHomeRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetHomeResponse(_message.Message):
    __slots__ = ("home", "invitations")
    HOME_FIELD_NUMBER: _ClassVar[int]
    INVITATIONS_FIELD_NUMBER: _ClassVar[int]
    home: Home
    invitations: _containers.RepeatedCompositeFieldContainer[HomeInvitation]
    def __init__(self, home: _Optional[_Union[Home, _Mapping]] = ..., invitations: _Optional[_Iterable[_Union[HomeInvitation, _Mapping]]] = ...) -> None: ...

class CreateHomeRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class CreateHomeResponse(_message.Message):
    __slots__ = ("home",)
    HOME_FIELD_NUMBER: _ClassVar[int]
    home: Home
    def __init__(self, home: _Optional[_Union[Home, _Mapping]] = ...) -> None: ...

class DissolveHomeRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class DissolveHomeResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class InviteToHomeRequest(_message.Message):
    __slots__ = ("user_name", "offers")
    USER_NAME_FIELD_NUMBER: _ClassVar[int]
    OFFERS_FIELD_NUMBER: _ClassVar[int]
    user_name: str
    offers: _containers.RepeatedCompositeFieldContainer[ServerAccess]
    def __init__(self, user_name: _Optional[str] = ..., offers: _Optional[_Iterable[_Union[ServerAccess, _Mapping]]] = ...) -> None: ...

class InviteToHomeResponse(_message.Message):
    __slots__ = ("invitation",)
    INVITATION_FIELD_NUMBER: _ClassVar[int]
    invitation: HomeInvitation
    def __init__(self, invitation: _Optional[_Union[HomeInvitation, _Mapping]] = ...) -> None: ...

class CancelHomeInvitationRequest(_message.Message):
    __slots__ = ("account_id",)
    ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    account_id: str
    def __init__(self, account_id: _Optional[str] = ...) -> None: ...

class CancelHomeInvitationResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class AcceptHomeInvitationRequest(_message.Message):
    __slots__ = ("home_id",)
    HOME_ID_FIELD_NUMBER: _ClassVar[int]
    home_id: str
    def __init__(self, home_id: _Optional[str] = ...) -> None: ...

class AcceptHomeInvitationResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class DeclineHomeInvitationRequest(_message.Message):
    __slots__ = ("home_id",)
    HOME_ID_FIELD_NUMBER: _ClassVar[int]
    home_id: str
    def __init__(self, home_id: _Optional[str] = ...) -> None: ...

class DeclineHomeInvitationResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class LeaveHomeRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class LeaveHomeResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RemoveMemberRequest(_message.Message):
    __slots__ = ("account_id",)
    ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    account_id: str
    def __init__(self, account_id: _Optional[str] = ...) -> None: ...

class RemoveMemberResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CreateManagedAccountRequest(_message.Message):
    __slots__ = ("user_name", "display_name", "restriction", "pin")
    USER_NAME_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    RESTRICTION_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    user_name: str
    display_name: str
    restriction: _user_pb2.RestrictionProfile
    pin: str
    def __init__(self, user_name: _Optional[str] = ..., display_name: _Optional[str] = ..., restriction: _Optional[_Union[_user_pb2.RestrictionProfile, str]] = ..., pin: _Optional[str] = ...) -> None: ...

class CreateManagedAccountResponse(_message.Message):
    __slots__ = ("member",)
    MEMBER_FIELD_NUMBER: _ClassVar[int]
    member: HomeMember
    def __init__(self, member: _Optional[_Union[HomeMember, _Mapping]] = ...) -> None: ...

class UpdateManagedAccountRequest(_message.Message):
    __slots__ = ("account_id", "display_name", "restriction")
    ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    RESTRICTION_FIELD_NUMBER: _ClassVar[int]
    account_id: str
    display_name: str
    restriction: _user_pb2.RestrictionProfile
    def __init__(self, account_id: _Optional[str] = ..., display_name: _Optional[str] = ..., restriction: _Optional[_Union[_user_pb2.RestrictionProfile, str]] = ...) -> None: ...

class UpdateManagedAccountResponse(_message.Message):
    __slots__ = ("member",)
    MEMBER_FIELD_NUMBER: _ClassVar[int]
    member: HomeMember
    def __init__(self, member: _Optional[_Union[HomeMember, _Mapping]] = ...) -> None: ...

class ConvertManagedAccountRequest(_message.Message):
    __slots__ = ("account_id", "email", "password")
    ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    account_id: str
    email: str
    password: str
    def __init__(self, account_id: _Optional[str] = ..., email: _Optional[str] = ..., password: _Optional[str] = ...) -> None: ...

class ConvertManagedAccountResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SetPinRequest(_message.Message):
    __slots__ = ("account_id", "pin")
    ACCOUNT_ID_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    account_id: str
    pin: str
    def __init__(self, account_id: _Optional[str] = ..., pin: _Optional[str] = ...) -> None: ...

class SetPinResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

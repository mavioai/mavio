import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SubtitleMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SUBTITLE_MODE_UNSPECIFIED: _ClassVar[SubtitleMode]
    SUBTITLE_MODE_ALWAYS: _ClassVar[SubtitleMode]
    SUBTITLE_MODE_FOREIGN: _ClassVar[SubtitleMode]
    SUBTITLE_MODE_FORCED: _ClassVar[SubtitleMode]
    SUBTITLE_MODE_NONE: _ClassVar[SubtitleMode]
    SUBTITLE_MODE_SMART: _ClassVar[SubtitleMode]

class RestrictionProfile(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    RESTRICTION_PROFILE_UNSPECIFIED: _ClassVar[RestrictionProfile]
    RESTRICTION_PROFILE_YOUNGER_CHILD: _ClassVar[RestrictionProfile]
    RESTRICTION_PROFILE_OLDER_CHILD: _ClassVar[RestrictionProfile]
    RESTRICTION_PROFILE_TEEN: _ClassVar[RestrictionProfile]
SUBTITLE_MODE_UNSPECIFIED: SubtitleMode
SUBTITLE_MODE_ALWAYS: SubtitleMode
SUBTITLE_MODE_FOREIGN: SubtitleMode
SUBTITLE_MODE_FORCED: SubtitleMode
SUBTITLE_MODE_NONE: SubtitleMode
SUBTITLE_MODE_SMART: SubtitleMode
RESTRICTION_PROFILE_UNSPECIFIED: RestrictionProfile
RESTRICTION_PROFILE_YOUNGER_CHILD: RestrictionProfile
RESTRICTION_PROFILE_OLDER_CHILD: RestrictionProfile
RESTRICTION_PROFILE_TEEN: RestrictionProfile

class User(_message.Message):
    __slots__ = ("id", "name", "auth_provider", "admin", "disabled", "policy", "playback_preferences", "create_time", "last_login_time", "managed", "has_pin", "restriction", "has_totp", "passkeys")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    AUTH_PROVIDER_FIELD_NUMBER: _ClassVar[int]
    ADMIN_FIELD_NUMBER: _ClassVar[int]
    DISABLED_FIELD_NUMBER: _ClassVar[int]
    POLICY_FIELD_NUMBER: _ClassVar[int]
    PLAYBACK_PREFERENCES_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    LAST_LOGIN_TIME_FIELD_NUMBER: _ClassVar[int]
    MANAGED_FIELD_NUMBER: _ClassVar[int]
    HAS_PIN_FIELD_NUMBER: _ClassVar[int]
    RESTRICTION_FIELD_NUMBER: _ClassVar[int]
    HAS_TOTP_FIELD_NUMBER: _ClassVar[int]
    PASSKEYS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    auth_provider: str
    admin: bool
    disabled: bool
    policy: UserPolicy
    playback_preferences: PlaybackPreferences
    create_time: _timestamp_pb2.Timestamp
    last_login_time: _timestamp_pb2.Timestamp
    managed: bool
    has_pin: bool
    restriction: RestrictionProfile
    has_totp: bool
    passkeys: _containers.RepeatedCompositeFieldContainer[Passkey]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., auth_provider: _Optional[str] = ..., admin: _Optional[bool] = ..., disabled: _Optional[bool] = ..., policy: _Optional[_Union[UserPolicy, _Mapping]] = ..., playback_preferences: _Optional[_Union[PlaybackPreferences, _Mapping]] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., last_login_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., managed: _Optional[bool] = ..., has_pin: _Optional[bool] = ..., restriction: _Optional[_Union[RestrictionProfile, str]] = ..., has_totp: _Optional[bool] = ..., passkeys: _Optional[_Iterable[_Union[Passkey, _Mapping]]] = ...) -> None: ...

class Passkey(_message.Message):
    __slots__ = ("id", "name", "rp_id", "create_time", "last_use_time")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    RP_ID_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    LAST_USE_TIME_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    rp_id: str
    create_time: _timestamp_pb2.Timestamp
    last_use_time: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., rp_id: _Optional[str] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., last_use_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class UserPolicy(_message.Message):
    __slots__ = ("all_libraries", "library_ids", "max_parental_rating", "block_unrated", "allow_transcoding", "allow_download", "max_streaming_bitrate", "max_sessions", "request_movies", "request_series", "auto_approve_movies", "auto_approve_series", "report_issues", "movie_quota", "series_quota", "allowed_labels", "blocked_labels")
    ALL_LIBRARIES_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_IDS_FIELD_NUMBER: _ClassVar[int]
    MAX_PARENTAL_RATING_FIELD_NUMBER: _ClassVar[int]
    BLOCK_UNRATED_FIELD_NUMBER: _ClassVar[int]
    ALLOW_TRANSCODING_FIELD_NUMBER: _ClassVar[int]
    ALLOW_DOWNLOAD_FIELD_NUMBER: _ClassVar[int]
    MAX_STREAMING_BITRATE_FIELD_NUMBER: _ClassVar[int]
    MAX_SESSIONS_FIELD_NUMBER: _ClassVar[int]
    REQUEST_MOVIES_FIELD_NUMBER: _ClassVar[int]
    REQUEST_SERIES_FIELD_NUMBER: _ClassVar[int]
    AUTO_APPROVE_MOVIES_FIELD_NUMBER: _ClassVar[int]
    AUTO_APPROVE_SERIES_FIELD_NUMBER: _ClassVar[int]
    REPORT_ISSUES_FIELD_NUMBER: _ClassVar[int]
    MOVIE_QUOTA_FIELD_NUMBER: _ClassVar[int]
    SERIES_QUOTA_FIELD_NUMBER: _ClassVar[int]
    ALLOWED_LABELS_FIELD_NUMBER: _ClassVar[int]
    BLOCKED_LABELS_FIELD_NUMBER: _ClassVar[int]
    all_libraries: bool
    library_ids: _containers.RepeatedScalarFieldContainer[str]
    max_parental_rating: int
    block_unrated: bool
    allow_transcoding: bool
    allow_download: bool
    max_streaming_bitrate: int
    max_sessions: int
    request_movies: bool
    request_series: bool
    auto_approve_movies: bool
    auto_approve_series: bool
    report_issues: bool
    movie_quota: RequestQuota
    series_quota: RequestQuota
    allowed_labels: _containers.RepeatedScalarFieldContainer[str]
    blocked_labels: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, all_libraries: _Optional[bool] = ..., library_ids: _Optional[_Iterable[str]] = ..., max_parental_rating: _Optional[int] = ..., block_unrated: _Optional[bool] = ..., allow_transcoding: _Optional[bool] = ..., allow_download: _Optional[bool] = ..., max_streaming_bitrate: _Optional[int] = ..., max_sessions: _Optional[int] = ..., request_movies: _Optional[bool] = ..., request_series: _Optional[bool] = ..., auto_approve_movies: _Optional[bool] = ..., auto_approve_series: _Optional[bool] = ..., report_issues: _Optional[bool] = ..., movie_quota: _Optional[_Union[RequestQuota, _Mapping]] = ..., series_quota: _Optional[_Union[RequestQuota, _Mapping]] = ..., allowed_labels: _Optional[_Iterable[str]] = ..., blocked_labels: _Optional[_Iterable[str]] = ...) -> None: ...

class RequestQuota(_message.Message):
    __slots__ = ("limit", "days")
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    DAYS_FIELD_NUMBER: _ClassVar[int]
    limit: int
    days: int
    def __init__(self, limit: _Optional[int] = ..., days: _Optional[int] = ...) -> None: ...

class PlaybackPreferences(_message.Message):
    __slots__ = ("audio_languages", "subtitle_languages", "subtitle_mode", "play_default_audio_track")
    AUDIO_LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_MODE_FIELD_NUMBER: _ClassVar[int]
    PLAY_DEFAULT_AUDIO_TRACK_FIELD_NUMBER: _ClassVar[int]
    audio_languages: _containers.RepeatedScalarFieldContainer[str]
    subtitle_languages: _containers.RepeatedScalarFieldContainer[str]
    subtitle_mode: SubtitleMode
    play_default_audio_track: bool
    def __init__(self, audio_languages: _Optional[_Iterable[str]] = ..., subtitle_languages: _Optional[_Iterable[str]] = ..., subtitle_mode: _Optional[_Union[SubtitleMode, str]] = ..., play_default_audio_track: _Optional[bool] = ...) -> None: ...

class UserData(_message.Message):
    __slots__ = ("item_id", "played", "play_count", "position", "audio_stream", "subtitle_stream", "favorite", "rating", "last_played_time")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    PLAYED_FIELD_NUMBER: _ClassVar[int]
    PLAY_COUNT_FIELD_NUMBER: _ClassVar[int]
    POSITION_FIELD_NUMBER: _ClassVar[int]
    AUDIO_STREAM_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_STREAM_FIELD_NUMBER: _ClassVar[int]
    FAVORITE_FIELD_NUMBER: _ClassVar[int]
    RATING_FIELD_NUMBER: _ClassVar[int]
    LAST_PLAYED_TIME_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    played: bool
    play_count: int
    position: _duration_pb2.Duration
    audio_stream: int
    subtitle_stream: int
    favorite: bool
    rating: float
    last_played_time: _timestamp_pb2.Timestamp
    def __init__(self, item_id: _Optional[str] = ..., played: _Optional[bool] = ..., play_count: _Optional[int] = ..., position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., audio_stream: _Optional[int] = ..., subtitle_stream: _Optional[int] = ..., favorite: _Optional[bool] = ..., rating: _Optional[float] = ..., last_played_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListUsersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListUsersResponse(_message.Message):
    __slots__ = ("users",)
    USERS_FIELD_NUMBER: _ClassVar[int]
    users: _containers.RepeatedCompositeFieldContainer[User]
    def __init__(self, users: _Optional[_Iterable[_Union[User, _Mapping]]] = ...) -> None: ...

class GetUserRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class GetUserResponse(_message.Message):
    __slots__ = ("user",)
    USER_FIELD_NUMBER: _ClassVar[int]
    user: User
    def __init__(self, user: _Optional[_Union[User, _Mapping]] = ...) -> None: ...

class GetCurrentUserRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetCurrentUserResponse(_message.Message):
    __slots__ = ("user",)
    USER_FIELD_NUMBER: _ClassVar[int]
    user: User
    def __init__(self, user: _Optional[_Union[User, _Mapping]] = ...) -> None: ...

class CreateUserRequest(_message.Message):
    __slots__ = ("name", "password", "auth_provider", "admin", "policy", "playback_preferences", "managed", "restriction", "pin")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    AUTH_PROVIDER_FIELD_NUMBER: _ClassVar[int]
    ADMIN_FIELD_NUMBER: _ClassVar[int]
    POLICY_FIELD_NUMBER: _ClassVar[int]
    PLAYBACK_PREFERENCES_FIELD_NUMBER: _ClassVar[int]
    MANAGED_FIELD_NUMBER: _ClassVar[int]
    RESTRICTION_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    name: str
    password: str
    auth_provider: str
    admin: bool
    policy: UserPolicy
    playback_preferences: PlaybackPreferences
    managed: bool
    restriction: RestrictionProfile
    pin: str
    def __init__(self, name: _Optional[str] = ..., password: _Optional[str] = ..., auth_provider: _Optional[str] = ..., admin: _Optional[bool] = ..., policy: _Optional[_Union[UserPolicy, _Mapping]] = ..., playback_preferences: _Optional[_Union[PlaybackPreferences, _Mapping]] = ..., managed: _Optional[bool] = ..., restriction: _Optional[_Union[RestrictionProfile, str]] = ..., pin: _Optional[str] = ...) -> None: ...

class CreateUserResponse(_message.Message):
    __slots__ = ("user",)
    USER_FIELD_NUMBER: _ClassVar[int]
    user: User
    def __init__(self, user: _Optional[_Union[User, _Mapping]] = ...) -> None: ...

class UpdateUserRequest(_message.Message):
    __slots__ = ("id", "name", "admin", "disabled", "policy", "restriction")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ADMIN_FIELD_NUMBER: _ClassVar[int]
    DISABLED_FIELD_NUMBER: _ClassVar[int]
    POLICY_FIELD_NUMBER: _ClassVar[int]
    RESTRICTION_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    admin: bool
    disabled: bool
    policy: UserPolicy
    restriction: RestrictionProfile
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., admin: _Optional[bool] = ..., disabled: _Optional[bool] = ..., policy: _Optional[_Union[UserPolicy, _Mapping]] = ..., restriction: _Optional[_Union[RestrictionProfile, str]] = ...) -> None: ...

class UpdateUserResponse(_message.Message):
    __slots__ = ("user",)
    USER_FIELD_NUMBER: _ClassVar[int]
    user: User
    def __init__(self, user: _Optional[_Union[User, _Mapping]] = ...) -> None: ...

class DeleteUserRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteUserResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SetPasswordRequest(_message.Message):
    __slots__ = ("user_id", "current_password", "new_password")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    CURRENT_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    NEW_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    current_password: str
    new_password: str
    def __init__(self, user_id: _Optional[str] = ..., current_password: _Optional[str] = ..., new_password: _Optional[str] = ...) -> None: ...

class SetPasswordResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SetPinRequest(_message.Message):
    __slots__ = ("user_id", "pin")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    PIN_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    pin: str
    def __init__(self, user_id: _Optional[str] = ..., pin: _Optional[str] = ...) -> None: ...

class SetPinResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StartPasskeyRegistrationRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StartPasskeyRegistrationResponse(_message.Message):
    __slots__ = ("ceremony_id", "options_json")
    CEREMONY_ID_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_JSON_FIELD_NUMBER: _ClassVar[int]
    ceremony_id: str
    options_json: str
    def __init__(self, ceremony_id: _Optional[str] = ..., options_json: _Optional[str] = ...) -> None: ...

class FinishPasskeyRegistrationRequest(_message.Message):
    __slots__ = ("ceremony_id", "name", "credential_json")
    CEREMONY_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_JSON_FIELD_NUMBER: _ClassVar[int]
    ceremony_id: str
    name: str
    credential_json: str
    def __init__(self, ceremony_id: _Optional[str] = ..., name: _Optional[str] = ..., credential_json: _Optional[str] = ...) -> None: ...

class FinishPasskeyRegistrationResponse(_message.Message):
    __slots__ = ("passkey",)
    PASSKEY_FIELD_NUMBER: _ClassVar[int]
    passkey: Passkey
    def __init__(self, passkey: _Optional[_Union[Passkey, _Mapping]] = ...) -> None: ...

class DeletePasskeyRequest(_message.Message):
    __slots__ = ("user_id", "id")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    id: str
    def __init__(self, user_id: _Optional[str] = ..., id: _Optional[str] = ...) -> None: ...

class DeletePasskeyResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StartTotpRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StartTotpResponse(_message.Message):
    __slots__ = ("secret", "uri")
    SECRET_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    secret: str
    uri: str
    def __init__(self, secret: _Optional[str] = ..., uri: _Optional[str] = ...) -> None: ...

class ConfirmTotpRequest(_message.Message):
    __slots__ = ("code",)
    CODE_FIELD_NUMBER: _ClassVar[int]
    code: str
    def __init__(self, code: _Optional[str] = ...) -> None: ...

class ConfirmTotpResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class DeleteTotpRequest(_message.Message):
    __slots__ = ("user_id",)
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    def __init__(self, user_id: _Optional[str] = ...) -> None: ...

class DeleteTotpResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class UpdatePlaybackPreferencesRequest(_message.Message):
    __slots__ = ("playback_preferences",)
    PLAYBACK_PREFERENCES_FIELD_NUMBER: _ClassVar[int]
    playback_preferences: PlaybackPreferences
    def __init__(self, playback_preferences: _Optional[_Union[PlaybackPreferences, _Mapping]] = ...) -> None: ...

class UpdatePlaybackPreferencesResponse(_message.Message):
    __slots__ = ("playback_preferences",)
    PLAYBACK_PREFERENCES_FIELD_NUMBER: _ClassVar[int]
    playback_preferences: PlaybackPreferences
    def __init__(self, playback_preferences: _Optional[_Union[PlaybackPreferences, _Mapping]] = ...) -> None: ...

class GetUserDataRequest(_message.Message):
    __slots__ = ("item_ids",)
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, item_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class GetUserDataResponse(_message.Message):
    __slots__ = ("user_data",)
    USER_DATA_FIELD_NUMBER: _ClassVar[int]
    user_data: _containers.RepeatedCompositeFieldContainer[UserData]
    def __init__(self, user_data: _Optional[_Iterable[_Union[UserData, _Mapping]]] = ...) -> None: ...

class UpdateUserDataRequest(_message.Message):
    __slots__ = ("item_id", "played", "favorite", "position", "rating")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    PLAYED_FIELD_NUMBER: _ClassVar[int]
    FAVORITE_FIELD_NUMBER: _ClassVar[int]
    POSITION_FIELD_NUMBER: _ClassVar[int]
    RATING_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    played: bool
    favorite: bool
    position: _duration_pb2.Duration
    rating: float
    def __init__(self, item_id: _Optional[str] = ..., played: _Optional[bool] = ..., favorite: _Optional[bool] = ..., position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., rating: _Optional[float] = ...) -> None: ...

class UpdateUserDataResponse(_message.Message):
    __slots__ = ("user_data",)
    USER_DATA_FIELD_NUMBER: _ClassVar[int]
    user_data: UserData
    def __init__(self, user_data: _Optional[_Union[UserData, _Mapping]] = ...) -> None: ...

class Invitation(_message.Message):
    __slots__ = ("id", "user_id", "access", "max_uses", "uses", "expire_time", "create_time")
    ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    ACCESS_FIELD_NUMBER: _ClassVar[int]
    MAX_USES_FIELD_NUMBER: _ClassVar[int]
    USES_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    id: str
    user_id: str
    access: UserPolicy
    max_uses: int
    uses: int
    expire_time: _timestamp_pb2.Timestamp
    create_time: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., user_id: _Optional[str] = ..., access: _Optional[_Union[UserPolicy, _Mapping]] = ..., max_uses: _Optional[int] = ..., uses: _Optional[int] = ..., expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class CreateInvitationRequest(_message.Message):
    __slots__ = ("user_id", "access", "max_uses", "valid_for")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    ACCESS_FIELD_NUMBER: _ClassVar[int]
    MAX_USES_FIELD_NUMBER: _ClassVar[int]
    VALID_FOR_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    access: UserPolicy
    max_uses: int
    valid_for: _duration_pb2.Duration
    def __init__(self, user_id: _Optional[str] = ..., access: _Optional[_Union[UserPolicy, _Mapping]] = ..., max_uses: _Optional[int] = ..., valid_for: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class CreateInvitationResponse(_message.Message):
    __slots__ = ("invitation", "token")
    INVITATION_FIELD_NUMBER: _ClassVar[int]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    invitation: Invitation
    token: str
    def __init__(self, invitation: _Optional[_Union[Invitation, _Mapping]] = ..., token: _Optional[str] = ...) -> None: ...

class ListInvitationsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListInvitationsResponse(_message.Message):
    __slots__ = ("invitations",)
    INVITATIONS_FIELD_NUMBER: _ClassVar[int]
    invitations: _containers.RepeatedCompositeFieldContainer[Invitation]
    def __init__(self, invitations: _Optional[_Iterable[_Union[Invitation, _Mapping]]] = ...) -> None: ...

class DeleteInvitationRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteInvitationResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

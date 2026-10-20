import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class IssueType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ISSUE_TYPE_UNSPECIFIED: _ClassVar[IssueType]
    ISSUE_TYPE_VIDEO: _ClassVar[IssueType]
    ISSUE_TYPE_AUDIO: _ClassVar[IssueType]
    ISSUE_TYPE_SUBTITLES: _ClassVar[IssueType]
    ISSUE_TYPE_OTHER: _ClassVar[IssueType]

class IssueStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ISSUE_STATUS_UNSPECIFIED: _ClassVar[IssueStatus]
    ISSUE_STATUS_OPEN: _ClassVar[IssueStatus]
    ISSUE_STATUS_RESOLVED: _ClassVar[IssueStatus]

class IssueSort(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ISSUE_SORT_UNSPECIFIED: _ClassVar[IssueSort]
    ISSUE_SORT_MODIFIED: _ClassVar[IssueSort]
ISSUE_TYPE_UNSPECIFIED: IssueType
ISSUE_TYPE_VIDEO: IssueType
ISSUE_TYPE_AUDIO: IssueType
ISSUE_TYPE_SUBTITLES: IssueType
ISSUE_TYPE_OTHER: IssueType
ISSUE_STATUS_UNSPECIFIED: IssueStatus
ISSUE_STATUS_OPEN: IssueStatus
ISSUE_STATUS_RESOLVED: IssueStatus
ISSUE_SORT_UNSPECIFIED: IssueSort
ISSUE_SORT_MODIFIED: IssueSort

class Issue(_message.Message):
    __slots__ = ("id", "item_id", "type", "status", "season_number", "episode_number", "created_by", "modified_by", "comments", "create_time", "update_time")
    ID_FIELD_NUMBER: _ClassVar[int]
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    EPISODE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    MODIFIED_BY_FIELD_NUMBER: _ClassVar[int]
    COMMENTS_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    id: str
    item_id: str
    type: IssueType
    status: IssueStatus
    season_number: int
    episode_number: int
    created_by: str
    modified_by: str
    comments: _containers.RepeatedCompositeFieldContainer[IssueComment]
    create_time: _timestamp_pb2.Timestamp
    update_time: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., item_id: _Optional[str] = ..., type: _Optional[_Union[IssueType, str]] = ..., status: _Optional[_Union[IssueStatus, str]] = ..., season_number: _Optional[int] = ..., episode_number: _Optional[int] = ..., created_by: _Optional[str] = ..., modified_by: _Optional[str] = ..., comments: _Optional[_Iterable[_Union[IssueComment, _Mapping]]] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class IssueComment(_message.Message):
    __slots__ = ("id", "user_id", "message", "create_time", "update_time")
    ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    id: str
    user_id: str
    message: str
    create_time: _timestamp_pb2.Timestamp
    update_time: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., user_id: _Optional[str] = ..., message: _Optional[str] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class CreateIssueRequest(_message.Message):
    __slots__ = ("item_id", "type", "season_number", "episode_number", "message")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    EPISODE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    type: IssueType
    season_number: int
    episode_number: int
    message: str
    def __init__(self, item_id: _Optional[str] = ..., type: _Optional[_Union[IssueType, str]] = ..., season_number: _Optional[int] = ..., episode_number: _Optional[int] = ..., message: _Optional[str] = ...) -> None: ...

class CreateIssueResponse(_message.Message):
    __slots__ = ("issue",)
    ISSUE_FIELD_NUMBER: _ClassVar[int]
    issue: Issue
    def __init__(self, issue: _Optional[_Union[Issue, _Mapping]] = ...) -> None: ...

class ListIssuesRequest(_message.Message):
    __slots__ = ("statuses", "item_id", "user_id", "sort", "limit", "offset")
    STATUSES_FIELD_NUMBER: _ClassVar[int]
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    SORT_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    statuses: _containers.RepeatedScalarFieldContainer[IssueStatus]
    item_id: str
    user_id: str
    sort: IssueSort
    limit: int
    offset: int
    def __init__(self, statuses: _Optional[_Iterable[_Union[IssueStatus, str]]] = ..., item_id: _Optional[str] = ..., user_id: _Optional[str] = ..., sort: _Optional[_Union[IssueSort, str]] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class ListIssuesResponse(_message.Message):
    __slots__ = ("issues", "total")
    ISSUES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    issues: _containers.RepeatedCompositeFieldContainer[Issue]
    total: int
    def __init__(self, issues: _Optional[_Iterable[_Union[Issue, _Mapping]]] = ..., total: _Optional[int] = ...) -> None: ...

class GetIssueRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class GetIssueResponse(_message.Message):
    __slots__ = ("issue",)
    ISSUE_FIELD_NUMBER: _ClassVar[int]
    issue: Issue
    def __init__(self, issue: _Optional[_Union[Issue, _Mapping]] = ...) -> None: ...

class AddCommentRequest(_message.Message):
    __slots__ = ("id", "message")
    ID_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    id: str
    message: str
    def __init__(self, id: _Optional[str] = ..., message: _Optional[str] = ...) -> None: ...

class AddCommentResponse(_message.Message):
    __slots__ = ("issue",)
    ISSUE_FIELD_NUMBER: _ClassVar[int]
    issue: Issue
    def __init__(self, issue: _Optional[_Union[Issue, _Mapping]] = ...) -> None: ...

class ResolveIssueRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class ResolveIssueResponse(_message.Message):
    __slots__ = ("issue",)
    ISSUE_FIELD_NUMBER: _ClassVar[int]
    issue: Issue
    def __init__(self, issue: _Optional[_Union[Issue, _Mapping]] = ...) -> None: ...

class ReopenIssueRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class ReopenIssueResponse(_message.Message):
    __slots__ = ("issue",)
    ISSUE_FIELD_NUMBER: _ClassVar[int]
    issue: Issue
    def __init__(self, issue: _Optional[_Union[Issue, _Mapping]] = ...) -> None: ...

class DeleteIssueRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteIssueResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

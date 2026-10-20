from buf.validate import validate_pb2 as _validate_pb2
from mavio.plugin.v1 import proxy_pb2 as _proxy_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Challenge(_message.Message):
    __slots__ = ("url", "method", "body", "content_type", "timeout_seconds")
    URL_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    CONTENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_SECONDS_FIELD_NUMBER: _ClassVar[int]
    url: str
    method: str
    body: bytes
    content_type: str
    timeout_seconds: int
    def __init__(self, url: _Optional[str] = ..., method: _Optional[str] = ..., body: _Optional[bytes] = ..., content_type: _Optional[str] = ..., timeout_seconds: _Optional[int] = ...) -> None: ...

class Cookie(_message.Message):
    __slots__ = ("name", "value", "domain", "path")
    NAME_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    DOMAIN_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    name: str
    value: str
    domain: str
    path: str
    def __init__(self, name: _Optional[str] = ..., value: _Optional[str] = ..., domain: _Optional[str] = ..., path: _Optional[str] = ...) -> None: ...

class ChallengeSolution(_message.Message):
    __slots__ = ("cookies", "user_agent", "solver")
    COOKIES_FIELD_NUMBER: _ClassVar[int]
    USER_AGENT_FIELD_NUMBER: _ClassVar[int]
    SOLVER_FIELD_NUMBER: _ClassVar[int]
    cookies: _containers.RepeatedCompositeFieldContainer[Cookie]
    user_agent: str
    solver: str
    def __init__(self, cookies: _Optional[_Iterable[_Union[Cookie, _Mapping]]] = ..., user_agent: _Optional[str] = ..., solver: _Optional[str] = ...) -> None: ...

class SolveRequest(_message.Message):
    __slots__ = ("challenge", "proxy")
    CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    PROXY_FIELD_NUMBER: _ClassVar[int]
    challenge: Challenge
    proxy: _proxy_pb2.Proxy
    def __init__(self, challenge: _Optional[_Union[Challenge, _Mapping]] = ..., proxy: _Optional[_Union[_proxy_pb2.Proxy, _Mapping]] = ...) -> None: ...

class SolveResponse(_message.Message):
    __slots__ = ("solution",)
    SOLUTION_FIELD_NUMBER: _ClassVar[int]
    solution: ChallengeSolution
    def __init__(self, solution: _Optional[_Union[ChallengeSolution, _Mapping]] = ...) -> None: ...

class SolveChallengeRequest(_message.Message):
    __slots__ = ("challenge", "proxy_id")
    CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    PROXY_ID_FIELD_NUMBER: _ClassVar[int]
    challenge: Challenge
    proxy_id: str
    def __init__(self, challenge: _Optional[_Union[Challenge, _Mapping]] = ..., proxy_id: _Optional[str] = ...) -> None: ...

class SolveChallengeResponse(_message.Message):
    __slots__ = ("solution",)
    SOLUTION_FIELD_NUMBER: _ClassVar[int]
    solution: ChallengeSolution
    def __init__(self, solution: _Optional[_Union[ChallengeSolution, _Mapping]] = ...) -> None: ...

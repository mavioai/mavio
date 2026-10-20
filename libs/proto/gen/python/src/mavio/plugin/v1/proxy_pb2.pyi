from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ProxyKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PROXY_KIND_UNSPECIFIED: _ClassVar[ProxyKind]
    PROXY_KIND_HTTP: _ClassVar[ProxyKind]
    PROXY_KIND_SOCKS4: _ClassVar[ProxyKind]
    PROXY_KIND_SOCKS5: _ClassVar[ProxyKind]
PROXY_KIND_UNSPECIFIED: ProxyKind
PROXY_KIND_HTTP: ProxyKind
PROXY_KIND_SOCKS4: ProxyKind
PROXY_KIND_SOCKS5: ProxyKind

class Proxy(_message.Message):
    __slots__ = ("id", "name", "kind", "host", "port", "username", "password", "bypass")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    HOST_FIELD_NUMBER: _ClassVar[int]
    PORT_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    BYPASS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    kind: ProxyKind
    host: str
    port: int
    username: str
    password: str
    bypass: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., kind: _Optional[_Union[ProxyKind, str]] = ..., host: _Optional[str] = ..., port: _Optional[int] = ..., username: _Optional[str] = ..., password: _Optional[str] = ..., bypass: _Optional[_Iterable[str]] = ...) -> None: ...

class ListProxiesRequest(_message.Message):
    __slots__ = ("ids",)
    IDS_FIELD_NUMBER: _ClassVar[int]
    ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, ids: _Optional[_Iterable[str]] = ...) -> None: ...

class ListProxiesResponse(_message.Message):
    __slots__ = ("proxies",)
    PROXIES_FIELD_NUMBER: _ClassVar[int]
    proxies: _containers.RepeatedCompositeFieldContainer[Proxy]
    def __init__(self, proxies: _Optional[_Iterable[_Union[Proxy, _Mapping]]] = ...) -> None: ...

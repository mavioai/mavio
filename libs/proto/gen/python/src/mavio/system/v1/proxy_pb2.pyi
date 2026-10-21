import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
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
    __slots__ = ("id", "name", "kind", "host", "port", "username", "password", "has_password", "system", "bypass")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    HOST_FIELD_NUMBER: _ClassVar[int]
    PORT_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    HAS_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    SYSTEM_FIELD_NUMBER: _ClassVar[int]
    BYPASS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    kind: ProxyKind
    host: str
    port: int
    username: str
    password: str
    has_password: bool
    system: bool
    bypass: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., kind: _Optional[_Union[ProxyKind, str]] = ..., host: _Optional[str] = ..., port: _Optional[int] = ..., username: _Optional[str] = ..., password: _Optional[str] = ..., has_password: _Optional[bool] = ..., system: _Optional[bool] = ..., bypass: _Optional[_Iterable[str]] = ...) -> None: ...

class ListProxiesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListProxiesResponse(_message.Message):
    __slots__ = ("proxies",)
    PROXIES_FIELD_NUMBER: _ClassVar[int]
    proxies: _containers.RepeatedCompositeFieldContainer[Proxy]
    def __init__(self, proxies: _Optional[_Iterable[_Union[Proxy, _Mapping]]] = ...) -> None: ...

class CreateProxyRequest(_message.Message):
    __slots__ = ("proxy",)
    PROXY_FIELD_NUMBER: _ClassVar[int]
    proxy: Proxy
    def __init__(self, proxy: _Optional[_Union[Proxy, _Mapping]] = ...) -> None: ...

class CreateProxyResponse(_message.Message):
    __slots__ = ("proxy",)
    PROXY_FIELD_NUMBER: _ClassVar[int]
    proxy: Proxy
    def __init__(self, proxy: _Optional[_Union[Proxy, _Mapping]] = ...) -> None: ...

class UpdateProxyRequest(_message.Message):
    __slots__ = ("proxy", "clear_password")
    PROXY_FIELD_NUMBER: _ClassVar[int]
    CLEAR_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    proxy: Proxy
    clear_password: bool
    def __init__(self, proxy: _Optional[_Union[Proxy, _Mapping]] = ..., clear_password: _Optional[bool] = ...) -> None: ...

class UpdateProxyResponse(_message.Message):
    __slots__ = ("proxy",)
    PROXY_FIELD_NUMBER: _ClassVar[int]
    proxy: Proxy
    def __init__(self, proxy: _Optional[_Union[Proxy, _Mapping]] = ...) -> None: ...

class DeleteProxyRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteProxyResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class TestProxyRequest(_message.Message):
    __slots__ = ("id", "proxy", "url")
    ID_FIELD_NUMBER: _ClassVar[int]
    PROXY_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    id: str
    proxy: Proxy
    url: str
    def __init__(self, id: _Optional[str] = ..., proxy: _Optional[_Union[Proxy, _Mapping]] = ..., url: _Optional[str] = ...) -> None: ...

class TestProxyResponse(_message.Message):
    __slots__ = ("ok", "error", "status", "latency", "exit_ip", "country")
    OK_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    LATENCY_FIELD_NUMBER: _ClassVar[int]
    EXIT_IP_FIELD_NUMBER: _ClassVar[int]
    COUNTRY_FIELD_NUMBER: _ClassVar[int]
    ok: bool
    error: str
    status: int
    latency: _duration_pb2.Duration
    exit_ip: str
    country: str
    def __init__(self, ok: _Optional[bool] = ..., error: _Optional[str] = ..., status: _Optional[int] = ..., latency: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., exit_ip: _Optional[str] = ..., country: _Optional[str] = ...) -> None: ...

class OutboundProxies(_message.Message):
    __slots__ = ("plugin_catalogs", "images", "mavio_dev", "acme")
    PLUGIN_CATALOGS_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    MAVIO_DEV_FIELD_NUMBER: _ClassVar[int]
    ACME_FIELD_NUMBER: _ClassVar[int]
    plugin_catalogs: str
    images: str
    mavio_dev: str
    acme: str
    def __init__(self, plugin_catalogs: _Optional[str] = ..., images: _Optional[str] = ..., mavio_dev: _Optional[str] = ..., acme: _Optional[str] = ...) -> None: ...

class GetOutboundProxiesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetOutboundProxiesResponse(_message.Message):
    __slots__ = ("proxies",)
    PROXIES_FIELD_NUMBER: _ClassVar[int]
    proxies: OutboundProxies
    def __init__(self, proxies: _Optional[_Union[OutboundProxies, _Mapping]] = ...) -> None: ...

class UpdateOutboundProxiesRequest(_message.Message):
    __slots__ = ("proxies",)
    PROXIES_FIELD_NUMBER: _ClassVar[int]
    proxies: OutboundProxies
    def __init__(self, proxies: _Optional[_Union[OutboundProxies, _Mapping]] = ...) -> None: ...

class UpdateOutboundProxiesResponse(_message.Message):
    __slots__ = ("proxies",)
    PROXIES_FIELD_NUMBER: _ClassVar[int]
    proxies: OutboundProxies
    def __init__(self, proxies: _Optional[_Union[OutboundProxies, _Mapping]] = ...) -> None: ...

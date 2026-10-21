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

class LogLevel(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    LOG_LEVEL_UNSPECIFIED: _ClassVar[LogLevel]
    LOG_LEVEL_DEBUG: _ClassVar[LogLevel]
    LOG_LEVEL_INFO: _ClassVar[LogLevel]
    LOG_LEVEL_WARN: _ClassVar[LogLevel]
    LOG_LEVEL_ERROR: _ClassVar[LogLevel]
LOG_LEVEL_UNSPECIFIED: LogLevel
LOG_LEVEL_DEBUG: LogLevel
LOG_LEVEL_INFO: LogLevel
LOG_LEVEL_WARN: LogLevel
LOG_LEVEL_ERROR: LogLevel

class GetHealthRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetHealthResponse(_message.Message):
    __slots__ = ("status", "version")
    class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        STATUS_UNSPECIFIED: _ClassVar[GetHealthResponse.Status]
        STATUS_SERVING: _ClassVar[GetHealthResponse.Status]
        STATUS_NOT_SERVING: _ClassVar[GetHealthResponse.Status]
    STATUS_UNSPECIFIED: GetHealthResponse.Status
    STATUS_SERVING: GetHealthResponse.Status
    STATUS_NOT_SERVING: GetHealthResponse.Status
    STATUS_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    status: GetHealthResponse.Status
    version: str
    def __init__(self, status: _Optional[_Union[GetHealthResponse.Status, str]] = ..., version: _Optional[str] = ...) -> None: ...

class GetSystemInfoRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetSystemInfoResponse(_message.Message):
    __slots__ = ("version", "os", "arch", "database", "ffmpeg_version", "start_time")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    OS_FIELD_NUMBER: _ClassVar[int]
    ARCH_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    FFMPEG_VERSION_FIELD_NUMBER: _ClassVar[int]
    START_TIME_FIELD_NUMBER: _ClassVar[int]
    version: str
    os: str
    arch: str
    database: str
    ffmpeg_version: str
    start_time: _timestamp_pb2.Timestamp
    def __init__(self, version: _Optional[str] = ..., os: _Optional[str] = ..., arch: _Optional[str] = ..., database: _Optional[str] = ..., ffmpeg_version: _Optional[str] = ..., start_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetSystemLoadRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetSystemLoadResponse(_message.Message):
    __slots__ = ("samples",)
    SAMPLES_FIELD_NUMBER: _ClassVar[int]
    samples: _containers.RepeatedCompositeFieldContainer[LoadSample]
    def __init__(self, samples: _Optional[_Iterable[_Union[LoadSample, _Mapping]]] = ...) -> None: ...

class LoadSample(_message.Message):
    __slots__ = ("time", "cpu", "memory", "disk", "network")
    TIME_FIELD_NUMBER: _ClassVar[int]
    CPU_FIELD_NUMBER: _ClassVar[int]
    MEMORY_FIELD_NUMBER: _ClassVar[int]
    DISK_FIELD_NUMBER: _ClassVar[int]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    time: _timestamp_pb2.Timestamp
    cpu: CpuLoad
    memory: MemoryLoad
    disk: DiskLoad
    network: NetworkLoad
    def __init__(self, time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., cpu: _Optional[_Union[CpuLoad, _Mapping]] = ..., memory: _Optional[_Union[MemoryLoad, _Mapping]] = ..., disk: _Optional[_Union[DiskLoad, _Mapping]] = ..., network: _Optional[_Union[NetworkLoad, _Mapping]] = ...) -> None: ...

class CpuLoad(_message.Message):
    __slots__ = ("user", "system", "server")
    USER_FIELD_NUMBER: _ClassVar[int]
    SYSTEM_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    user: float
    system: float
    server: float
    def __init__(self, user: _Optional[float] = ..., system: _Optional[float] = ..., server: _Optional[float] = ...) -> None: ...

class MemoryLoad(_message.Message):
    __slots__ = ("total_bytes", "used_bytes", "swap_total_bytes", "swap_used_bytes", "server_bytes")
    TOTAL_BYTES_FIELD_NUMBER: _ClassVar[int]
    USED_BYTES_FIELD_NUMBER: _ClassVar[int]
    SWAP_TOTAL_BYTES_FIELD_NUMBER: _ClassVar[int]
    SWAP_USED_BYTES_FIELD_NUMBER: _ClassVar[int]
    SERVER_BYTES_FIELD_NUMBER: _ClassVar[int]
    total_bytes: int
    used_bytes: int
    swap_total_bytes: int
    swap_used_bytes: int
    server_bytes: int
    def __init__(self, total_bytes: _Optional[int] = ..., used_bytes: _Optional[int] = ..., swap_total_bytes: _Optional[int] = ..., swap_used_bytes: _Optional[int] = ..., server_bytes: _Optional[int] = ...) -> None: ...

class DiskLoad(_message.Message):
    __slots__ = ("read_rate", "write_rate", "read_bytes", "written_bytes", "total_bytes", "available_bytes")
    READ_RATE_FIELD_NUMBER: _ClassVar[int]
    WRITE_RATE_FIELD_NUMBER: _ClassVar[int]
    READ_BYTES_FIELD_NUMBER: _ClassVar[int]
    WRITTEN_BYTES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_BYTES_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_BYTES_FIELD_NUMBER: _ClassVar[int]
    read_rate: float
    write_rate: float
    read_bytes: int
    written_bytes: int
    total_bytes: int
    available_bytes: int
    def __init__(self, read_rate: _Optional[float] = ..., write_rate: _Optional[float] = ..., read_bytes: _Optional[int] = ..., written_bytes: _Optional[int] = ..., total_bytes: _Optional[int] = ..., available_bytes: _Optional[int] = ...) -> None: ...

class NetworkLoad(_message.Message):
    __slots__ = ("receive_rate", "send_rate", "received_bytes", "sent_bytes")
    RECEIVE_RATE_FIELD_NUMBER: _ClassVar[int]
    SEND_RATE_FIELD_NUMBER: _ClassVar[int]
    RECEIVED_BYTES_FIELD_NUMBER: _ClassVar[int]
    SENT_BYTES_FIELD_NUMBER: _ClassVar[int]
    receive_rate: float
    send_rate: float
    received_bytes: int
    sent_bytes: int
    def __init__(self, receive_rate: _Optional[float] = ..., send_rate: _Optional[float] = ..., received_bytes: _Optional[int] = ..., sent_bytes: _Optional[int] = ...) -> None: ...

class NetworkSettings(_message.Message):
    __slots__ = ("server_name", "base_url", "https_port", "certificate_path", "key_path", "local_discovery", "relay")
    SERVER_NAME_FIELD_NUMBER: _ClassVar[int]
    BASE_URL_FIELD_NUMBER: _ClassVar[int]
    HTTPS_PORT_FIELD_NUMBER: _ClassVar[int]
    CERTIFICATE_PATH_FIELD_NUMBER: _ClassVar[int]
    KEY_PATH_FIELD_NUMBER: _ClassVar[int]
    LOCAL_DISCOVERY_FIELD_NUMBER: _ClassVar[int]
    RELAY_FIELD_NUMBER: _ClassVar[int]
    server_name: str
    base_url: str
    https_port: int
    certificate_path: str
    key_path: str
    local_discovery: bool
    relay: bool
    def __init__(self, server_name: _Optional[str] = ..., base_url: _Optional[str] = ..., https_port: _Optional[int] = ..., certificate_path: _Optional[str] = ..., key_path: _Optional[str] = ..., local_discovery: _Optional[bool] = ..., relay: _Optional[bool] = ...) -> None: ...

class MetadataSettings(_message.Message):
    __slots__ = ("preferred_language", "country")
    PREFERRED_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    COUNTRY_FIELD_NUMBER: _ClassVar[int]
    preferred_language: str
    country: str
    def __init__(self, preferred_language: _Optional[str] = ..., country: _Optional[str] = ...) -> None: ...

class GeneralSettings(_message.Message):
    __slots__ = ("network", "metadata", "session_idle_days", "password_reset_plugin")
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SESSION_IDLE_DAYS_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_RESET_PLUGIN_FIELD_NUMBER: _ClassVar[int]
    network: NetworkSettings
    metadata: MetadataSettings
    session_idle_days: int
    password_reset_plugin: str
    def __init__(self, network: _Optional[_Union[NetworkSettings, _Mapping]] = ..., metadata: _Optional[_Union[MetadataSettings, _Mapping]] = ..., session_idle_days: _Optional[int] = ..., password_reset_plugin: _Optional[str] = ...) -> None: ...

class GetGeneralSettingsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetGeneralSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: GeneralSettings
    def __init__(self, settings: _Optional[_Union[GeneralSettings, _Mapping]] = ...) -> None: ...

class UpdateGeneralSettingsRequest(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: GeneralSettings
    def __init__(self, settings: _Optional[_Union[GeneralSettings, _Mapping]] = ...) -> None: ...

class UpdateGeneralSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: GeneralSettings
    def __init__(self, settings: _Optional[_Union[GeneralSettings, _Mapping]] = ...) -> None: ...

class ListDirectoryRequest(_message.Message):
    __slots__ = ("path", "include_files")
    PATH_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_FILES_FIELD_NUMBER: _ClassVar[int]
    path: str
    include_files: bool
    def __init__(self, path: _Optional[str] = ..., include_files: _Optional[bool] = ...) -> None: ...

class DirectoryEntry(_message.Message):
    __slots__ = ("name", "path", "is_dir")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    IS_DIR_FIELD_NUMBER: _ClassVar[int]
    name: str
    path: str
    is_dir: bool
    def __init__(self, name: _Optional[str] = ..., path: _Optional[str] = ..., is_dir: _Optional[bool] = ...) -> None: ...

class ListDirectoryResponse(_message.Message):
    __slots__ = ("path", "parent", "entries")
    PATH_FIELD_NUMBER: _ClassVar[int]
    PARENT_FIELD_NUMBER: _ClassVar[int]
    ENTRIES_FIELD_NUMBER: _ClassVar[int]
    path: str
    parent: str
    entries: _containers.RepeatedCompositeFieldContainer[DirectoryEntry]
    def __init__(self, path: _Optional[str] = ..., parent: _Optional[str] = ..., entries: _Optional[_Iterable[_Union[DirectoryEntry, _Mapping]]] = ...) -> None: ...

class LogRecord(_message.Message):
    __slots__ = ("time", "level", "message", "attributes")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    TIME_FIELD_NUMBER: _ClassVar[int]
    LEVEL_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    time: _timestamp_pb2.Timestamp
    level: LogLevel
    message: str
    attributes: _containers.ScalarMap[str, str]
    def __init__(self, time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., level: _Optional[_Union[LogLevel, str]] = ..., message: _Optional[str] = ..., attributes: _Optional[_Mapping[str, str]] = ...) -> None: ...

class ListLogsRequest(_message.Message):
    __slots__ = ("min_level", "max_age", "limit", "plugin_id")
    MIN_LEVEL_FIELD_NUMBER: _ClassVar[int]
    MAX_AGE_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    min_level: LogLevel
    max_age: _duration_pb2.Duration
    limit: int
    plugin_id: str
    def __init__(self, min_level: _Optional[_Union[LogLevel, str]] = ..., max_age: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., limit: _Optional[int] = ..., plugin_id: _Optional[str] = ...) -> None: ...

class ListLogsResponse(_message.Message):
    __slots__ = ("records",)
    RECORDS_FIELD_NUMBER: _ClassVar[int]
    records: _containers.RepeatedCompositeFieldContainer[LogRecord]
    def __init__(self, records: _Optional[_Iterable[_Union[LogRecord, _Mapping]]] = ...) -> None: ...

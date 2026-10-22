from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DiskKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    DISK_KIND_UNSPECIFIED: _ClassVar[DiskKind]
    DISK_KIND_SSD: _ClassVar[DiskKind]
    DISK_KIND_HDD: _ClassVar[DiskKind]
    DISK_KIND_NETWORK: _ClassVar[DiskKind]
    DISK_KIND_CLOUD: _ClassVar[DiskKind]
DISK_KIND_UNSPECIFIED: DiskKind
DISK_KIND_SSD: DiskKind
DISK_KIND_HDD: DiskKind
DISK_KIND_NETWORK: DiskKind
DISK_KIND_CLOUD: DiskKind

class ListDisksRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListDisksResponse(_message.Message):
    __slots__ = ("disks",)
    DISKS_FIELD_NUMBER: _ClassVar[int]
    disks: _containers.RepeatedCompositeFieldContainer[Disk]
    def __init__(self, disks: _Optional[_Iterable[_Union[Disk, _Mapping]]] = ...) -> None: ...

class Disk(_message.Message):
    __slots__ = ("path", "name", "file_system", "kind", "total_bytes", "used_bytes", "available_bytes")
    PATH_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    FILE_SYSTEM_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    TOTAL_BYTES_FIELD_NUMBER: _ClassVar[int]
    USED_BYTES_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_BYTES_FIELD_NUMBER: _ClassVar[int]
    path: str
    name: str
    file_system: str
    kind: DiskKind
    total_bytes: int
    used_bytes: int
    available_bytes: int
    def __init__(self, path: _Optional[str] = ..., name: _Optional[str] = ..., file_system: _Optional[str] = ..., kind: _Optional[_Union[DiskKind, str]] = ..., total_bytes: _Optional[int] = ..., used_bytes: _Optional[int] = ..., available_bytes: _Optional[int] = ...) -> None: ...

class GetStatusRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetStatusResponse(_message.Message):
    __slots__ = ("available", "version", "managed", "error")
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    MANAGED_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    available: bool
    version: str
    managed: bool
    error: str
    def __init__(self, available: _Optional[bool] = ..., version: _Optional[str] = ..., managed: _Optional[bool] = ..., error: _Optional[str] = ...) -> None: ...

class ListProvidersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListProvidersResponse(_message.Message):
    __slots__ = ("providers",)
    PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    providers: _containers.RepeatedCompositeFieldContainer[Provider]
    def __init__(self, providers: _Optional[_Iterable[_Union[Provider, _Mapping]]] = ...) -> None: ...

class Provider(_message.Message):
    __slots__ = ("name", "description", "options")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    name: str
    description: str
    options: _containers.RepeatedCompositeFieldContainer[Option]
    def __init__(self, name: _Optional[str] = ..., description: _Optional[str] = ..., options: _Optional[_Iterable[_Union[Option, _Mapping]]] = ...) -> None: ...

class Option(_message.Message):
    __slots__ = ("name", "help", "provider", "default_value", "type", "required", "password", "advanced", "sensitive", "exclusive", "examples")
    NAME_FIELD_NUMBER: _ClassVar[int]
    HELP_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_VALUE_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    REQUIRED_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    ADVANCED_FIELD_NUMBER: _ClassVar[int]
    SENSITIVE_FIELD_NUMBER: _ClassVar[int]
    EXCLUSIVE_FIELD_NUMBER: _ClassVar[int]
    EXAMPLES_FIELD_NUMBER: _ClassVar[int]
    name: str
    help: str
    provider: str
    default_value: str
    type: str
    required: bool
    password: bool
    advanced: bool
    sensitive: bool
    exclusive: bool
    examples: _containers.RepeatedCompositeFieldContainer[Example]
    def __init__(self, name: _Optional[str] = ..., help: _Optional[str] = ..., provider: _Optional[str] = ..., default_value: _Optional[str] = ..., type: _Optional[str] = ..., required: _Optional[bool] = ..., password: _Optional[bool] = ..., advanced: _Optional[bool] = ..., sensitive: _Optional[bool] = ..., exclusive: _Optional[bool] = ..., examples: _Optional[_Iterable[_Union[Example, _Mapping]]] = ...) -> None: ...

class Example(_message.Message):
    __slots__ = ("value", "help", "provider")
    VALUE_FIELD_NUMBER: _ClassVar[int]
    HELP_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    value: str
    help: str
    provider: str
    def __init__(self, value: _Optional[str] = ..., help: _Optional[str] = ..., provider: _Optional[str] = ...) -> None: ...

class ListRemotesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListRemotesResponse(_message.Message):
    __slots__ = ("remotes",)
    REMOTES_FIELD_NUMBER: _ClassVar[int]
    remotes: _containers.RepeatedCompositeFieldContainer[Remote]
    def __init__(self, remotes: _Optional[_Iterable[_Union[Remote, _Mapping]]] = ...) -> None: ...

class Remote(_message.Message):
    __slots__ = ("name", "type", "parameters")
    class ParametersEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    name: str
    type: str
    parameters: _containers.ScalarMap[str, str]
    def __init__(self, name: _Optional[str] = ..., type: _Optional[str] = ..., parameters: _Optional[_Mapping[str, str]] = ...) -> None: ...

class ConfigQuestion(_message.Message):
    __slots__ = ("state", "option", "error")
    STATE_FIELD_NUMBER: _ClassVar[int]
    OPTION_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    state: str
    option: Option
    error: str
    def __init__(self, state: _Optional[str] = ..., option: _Optional[_Union[Option, _Mapping]] = ..., error: _Optional[str] = ...) -> None: ...

class CreateRemoteRequest(_message.Message):
    __slots__ = ("name", "type", "parameters")
    class ParametersEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    name: str
    type: str
    parameters: _containers.ScalarMap[str, str]
    def __init__(self, name: _Optional[str] = ..., type: _Optional[str] = ..., parameters: _Optional[_Mapping[str, str]] = ...) -> None: ...

class CreateRemoteResponse(_message.Message):
    __slots__ = ("remote", "question")
    REMOTE_FIELD_NUMBER: _ClassVar[int]
    QUESTION_FIELD_NUMBER: _ClassVar[int]
    remote: Remote
    question: ConfigQuestion
    def __init__(self, remote: _Optional[_Union[Remote, _Mapping]] = ..., question: _Optional[_Union[ConfigQuestion, _Mapping]] = ...) -> None: ...

class UpdateRemoteRequest(_message.Message):
    __slots__ = ("name", "parameters")
    class ParametersEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    name: str
    parameters: _containers.ScalarMap[str, str]
    def __init__(self, name: _Optional[str] = ..., parameters: _Optional[_Mapping[str, str]] = ...) -> None: ...

class UpdateRemoteResponse(_message.Message):
    __slots__ = ("remote", "question")
    REMOTE_FIELD_NUMBER: _ClassVar[int]
    QUESTION_FIELD_NUMBER: _ClassVar[int]
    remote: Remote
    question: ConfigQuestion
    def __init__(self, remote: _Optional[_Union[Remote, _Mapping]] = ..., question: _Optional[_Union[ConfigQuestion, _Mapping]] = ...) -> None: ...

class ContinueRemoteRequest(_message.Message):
    __slots__ = ("name", "state", "answer")
    NAME_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    ANSWER_FIELD_NUMBER: _ClassVar[int]
    name: str
    state: str
    answer: str
    def __init__(self, name: _Optional[str] = ..., state: _Optional[str] = ..., answer: _Optional[str] = ...) -> None: ...

class ContinueRemoteResponse(_message.Message):
    __slots__ = ("remote", "question")
    REMOTE_FIELD_NUMBER: _ClassVar[int]
    QUESTION_FIELD_NUMBER: _ClassVar[int]
    remote: Remote
    question: ConfigQuestion
    def __init__(self, remote: _Optional[_Union[Remote, _Mapping]] = ..., question: _Optional[_Union[ConfigQuestion, _Mapping]] = ...) -> None: ...

class DeleteRemoteRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class DeleteRemoteResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListFoldersRequest(_message.Message):
    __slots__ = ("path",)
    PATH_FIELD_NUMBER: _ClassVar[int]
    path: str
    def __init__(self, path: _Optional[str] = ...) -> None: ...

class ListFoldersResponse(_message.Message):
    __slots__ = ("folders",)
    FOLDERS_FIELD_NUMBER: _ClassVar[int]
    folders: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, folders: _Optional[_Iterable[str]] = ...) -> None: ...

class GetRemoteUsageRequest(_message.Message):
    __slots__ = ("remote",)
    REMOTE_FIELD_NUMBER: _ClassVar[int]
    remote: str
    def __init__(self, remote: _Optional[str] = ...) -> None: ...

class GetRemoteUsageResponse(_message.Message):
    __slots__ = ("total_bytes", "used_bytes", "free_bytes")
    TOTAL_BYTES_FIELD_NUMBER: _ClassVar[int]
    USED_BYTES_FIELD_NUMBER: _ClassVar[int]
    FREE_BYTES_FIELD_NUMBER: _ClassVar[int]
    total_bytes: int
    used_bytes: int
    free_bytes: int
    def __init__(self, total_bytes: _Optional[int] = ..., used_bytes: _Optional[int] = ..., free_bytes: _Optional[int] = ...) -> None: ...

class OfflineDownloads(_message.Message):
    __slots__ = ("enabled", "priority", "folder", "tags", "remove_completed_downloads", "remove_failed_downloads")
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    PRIORITY_FIELD_NUMBER: _ClassVar[int]
    FOLDER_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    REMOVE_COMPLETED_DOWNLOADS_FIELD_NUMBER: _ClassVar[int]
    REMOVE_FAILED_DOWNLOADS_FIELD_NUMBER: _ClassVar[int]
    enabled: bool
    priority: int
    folder: str
    tags: _containers.RepeatedScalarFieldContainer[str]
    remove_completed_downloads: bool
    remove_failed_downloads: bool
    def __init__(self, enabled: _Optional[bool] = ..., priority: _Optional[int] = ..., folder: _Optional[str] = ..., tags: _Optional[_Iterable[str]] = ..., remove_completed_downloads: _Optional[bool] = ..., remove_failed_downloads: _Optional[bool] = ...) -> None: ...

class GetOfflineDownloadsRequest(_message.Message):
    __slots__ = ("remote",)
    REMOTE_FIELD_NUMBER: _ClassVar[int]
    remote: str
    def __init__(self, remote: _Optional[str] = ...) -> None: ...

class GetOfflineDownloadsResponse(_message.Message):
    __slots__ = ("supported", "settings")
    SUPPORTED_FIELD_NUMBER: _ClassVar[int]
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    supported: bool
    settings: OfflineDownloads
    def __init__(self, supported: _Optional[bool] = ..., settings: _Optional[_Union[OfflineDownloads, _Mapping]] = ...) -> None: ...

class UpdateOfflineDownloadsRequest(_message.Message):
    __slots__ = ("remote", "settings")
    REMOTE_FIELD_NUMBER: _ClassVar[int]
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    remote: str
    settings: OfflineDownloads
    def __init__(self, remote: _Optional[str] = ..., settings: _Optional[_Union[OfflineDownloads, _Mapping]] = ...) -> None: ...

class UpdateOfflineDownloadsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: OfflineDownloads
    def __init__(self, settings: _Optional[_Union[OfflineDownloads, _Mapping]] = ...) -> None: ...

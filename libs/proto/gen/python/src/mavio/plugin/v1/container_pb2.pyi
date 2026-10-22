from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GpuSupport(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    GPU_SUPPORT_UNSPECIFIED: _ClassVar[GpuSupport]
    GPU_SUPPORT_OPTIONAL: _ClassVar[GpuSupport]
    GPU_SUPPORT_REQUIRED: _ClassVar[GpuSupport]

class ContainerMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CONTAINER_MODE_UNSPECIFIED: _ClassVar[ContainerMode]
    CONTAINER_MODE_MANAGED: _ClassVar[ContainerMode]
    CONTAINER_MODE_EXTERNAL: _ClassVar[ContainerMode]

class ContainerState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CONTAINER_STATE_UNSPECIFIED: _ClassVar[ContainerState]
    CONTAINER_STATE_PULLING: _ClassVar[ContainerState]
    CONTAINER_STATE_STARTING: _ClassVar[ContainerState]
    CONTAINER_STATE_READY: _ClassVar[ContainerState]
    CONTAINER_STATE_UNHEALTHY: _ClassVar[ContainerState]
    CONTAINER_STATE_UNAVAILABLE: _ClassVar[ContainerState]
GPU_SUPPORT_UNSPECIFIED: GpuSupport
GPU_SUPPORT_OPTIONAL: GpuSupport
GPU_SUPPORT_REQUIRED: GpuSupport
CONTAINER_MODE_UNSPECIFIED: ContainerMode
CONTAINER_MODE_MANAGED: ContainerMode
CONTAINER_MODE_EXTERNAL: ContainerMode
CONTAINER_STATE_UNSPECIFIED: ContainerState
CONTAINER_STATE_PULLING: ContainerState
CONTAINER_STATE_STARTING: ContainerState
CONTAINER_STATE_READY: ContainerState
CONTAINER_STATE_UNHEALTHY: ContainerState
CONTAINER_STATE_UNAVAILABLE: ContainerState

class ContainerDependency(_message.Message):
    __slots__ = ("name", "description", "image", "port", "health_path", "command", "env", "volumes", "gpu", "gpu_image", "memory_mib", "cpus", "required", "host_access", "direct", "no_proxy_ipv4_only", "folders", "peer_port", "entrypoint")
    class EnvEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    class VolumesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    IMAGE_FIELD_NUMBER: _ClassVar[int]
    PORT_FIELD_NUMBER: _ClassVar[int]
    HEALTH_PATH_FIELD_NUMBER: _ClassVar[int]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    ENV_FIELD_NUMBER: _ClassVar[int]
    VOLUMES_FIELD_NUMBER: _ClassVar[int]
    GPU_FIELD_NUMBER: _ClassVar[int]
    GPU_IMAGE_FIELD_NUMBER: _ClassVar[int]
    MEMORY_MIB_FIELD_NUMBER: _ClassVar[int]
    CPUS_FIELD_NUMBER: _ClassVar[int]
    REQUIRED_FIELD_NUMBER: _ClassVar[int]
    HOST_ACCESS_FIELD_NUMBER: _ClassVar[int]
    DIRECT_FIELD_NUMBER: _ClassVar[int]
    NO_PROXY_IPV4_ONLY_FIELD_NUMBER: _ClassVar[int]
    FOLDERS_FIELD_NUMBER: _ClassVar[int]
    PEER_PORT_FIELD_NUMBER: _ClassVar[int]
    ENTRYPOINT_FIELD_NUMBER: _ClassVar[int]
    name: str
    description: str
    image: str
    port: int
    health_path: str
    command: _containers.RepeatedScalarFieldContainer[str]
    env: _containers.ScalarMap[str, str]
    volumes: _containers.ScalarMap[str, str]
    gpu: GpuSupport
    gpu_image: str
    memory_mib: int
    cpus: float
    required: bool
    host_access: bool
    direct: bool
    no_proxy_ipv4_only: bool
    folders: _containers.RepeatedCompositeFieldContainer[ContainerFolder]
    peer_port: int
    entrypoint: str
    def __init__(self, name: _Optional[str] = ..., description: _Optional[str] = ..., image: _Optional[str] = ..., port: _Optional[int] = ..., health_path: _Optional[str] = ..., command: _Optional[_Iterable[str]] = ..., env: _Optional[_Mapping[str, str]] = ..., volumes: _Optional[_Mapping[str, str]] = ..., gpu: _Optional[_Union[GpuSupport, str]] = ..., gpu_image: _Optional[str] = ..., memory_mib: _Optional[int] = ..., cpus: _Optional[float] = ..., required: _Optional[bool] = ..., host_access: _Optional[bool] = ..., direct: _Optional[bool] = ..., no_proxy_ipv4_only: _Optional[bool] = ..., folders: _Optional[_Iterable[_Union[ContainerFolder, _Mapping]]] = ..., peer_port: _Optional[int] = ..., entrypoint: _Optional[str] = ...) -> None: ...

class ContainerFolder(_message.Message):
    __slots__ = ("name", "path", "description")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    name: str
    path: str
    description: str
    def __init__(self, name: _Optional[str] = ..., path: _Optional[str] = ..., description: _Optional[str] = ...) -> None: ...

class ContainerStatus(_message.Message):
    __slots__ = ("name", "mode", "address", "state", "error", "progress", "queued", "secrets")
    class SecretsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    ADDRESS_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    PROGRESS_FIELD_NUMBER: _ClassVar[int]
    QUEUED_FIELD_NUMBER: _ClassVar[int]
    SECRETS_FIELD_NUMBER: _ClassVar[int]
    name: str
    mode: ContainerMode
    address: str
    state: ContainerState
    error: str
    progress: float
    queued: bool
    secrets: _containers.ScalarMap[str, str]
    def __init__(self, name: _Optional[str] = ..., mode: _Optional[_Union[ContainerMode, str]] = ..., address: _Optional[str] = ..., state: _Optional[_Union[ContainerState, str]] = ..., error: _Optional[str] = ..., progress: _Optional[float] = ..., queued: _Optional[bool] = ..., secrets: _Optional[_Mapping[str, str]] = ...) -> None: ...

class ListContainersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListContainersResponse(_message.Message):
    __slots__ = ("containers",)
    CONTAINERS_FIELD_NUMBER: _ClassVar[int]
    containers: _containers.RepeatedCompositeFieldContainer[ContainerStatus]
    def __init__(self, containers: _Optional[_Iterable[_Union[ContainerStatus, _Mapping]]] = ...) -> None: ...

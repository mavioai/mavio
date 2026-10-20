import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.plugin.v1 import container_pb2 as _container_pb2
from mavio.plugin.v1 import plugin_pb2 as _plugin_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class PluginState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PLUGIN_STATE_UNSPECIFIED: _ClassVar[PluginState]
    PLUGIN_STATE_READY: _ClassVar[PluginState]
    PLUGIN_STATE_UNCONFIGURED: _ClassVar[PluginState]
    PLUGIN_STATE_FAILED: _ClassVar[PluginState]
    PLUGIN_STATE_DISABLED: _ClassVar[PluginState]
PLUGIN_STATE_UNSPECIFIED: PluginState
PLUGIN_STATE_READY: PluginState
PLUGIN_STATE_UNCONFIGURED: PluginState
PLUGIN_STATE_FAILED: PluginState
PLUGIN_STATE_DISABLED: PluginState

class Plugin(_message.Message):
    __slots__ = ("manifest", "state", "error", "proxy", "dev_folder")
    MANIFEST_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    PROXY_FIELD_NUMBER: _ClassVar[int]
    DEV_FOLDER_FIELD_NUMBER: _ClassVar[int]
    manifest: _plugin_pb2.Manifest
    state: PluginState
    error: str
    proxy: str
    dev_folder: str
    def __init__(self, manifest: _Optional[_Union[_plugin_pb2.Manifest, _Mapping]] = ..., state: _Optional[_Union[PluginState, str]] = ..., error: _Optional[str] = ..., proxy: _Optional[str] = ..., dev_folder: _Optional[str] = ...) -> None: ...

class ListPluginsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListPluginsResponse(_message.Message):
    __slots__ = ("plugins",)
    PLUGINS_FIELD_NUMBER: _ClassVar[int]
    plugins: _containers.RepeatedCompositeFieldContainer[Plugin]
    def __init__(self, plugins: _Optional[_Iterable[_Union[Plugin, _Mapping]]] = ...) -> None: ...

class GetPluginConfigRequest(_message.Message):
    __slots__ = ("plugin_id",)
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    def __init__(self, plugin_id: _Optional[str] = ...) -> None: ...

class GetPluginConfigResponse(_message.Message):
    __slots__ = ("config_json", "update_time")
    CONFIG_JSON_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    config_json: str
    update_time: _timestamp_pb2.Timestamp
    def __init__(self, config_json: _Optional[str] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class SetPluginConfigRequest(_message.Message):
    __slots__ = ("plugin_id", "config_json")
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    CONFIG_JSON_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    config_json: str
    def __init__(self, plugin_id: _Optional[str] = ..., config_json: _Optional[str] = ...) -> None: ...

class SetPluginConfigResponse(_message.Message):
    __slots__ = ("plugin", "config_json")
    PLUGIN_FIELD_NUMBER: _ClassVar[int]
    CONFIG_JSON_FIELD_NUMBER: _ClassVar[int]
    plugin: Plugin
    config_json: str
    def __init__(self, plugin: _Optional[_Union[Plugin, _Mapping]] = ..., config_json: _Optional[str] = ...) -> None: ...

class ListPluginConfigOptionsRequest(_message.Message):
    __slots__ = ("plugin_id", "source")
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    source: str
    def __init__(self, plugin_id: _Optional[str] = ..., source: _Optional[str] = ...) -> None: ...

class ListPluginConfigOptionsResponse(_message.Message):
    __slots__ = ("options",)
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    options: _containers.RepeatedCompositeFieldContainer[PluginConfigOption]
    def __init__(self, options: _Optional[_Iterable[_Union[PluginConfigOption, _Mapping]]] = ...) -> None: ...

class PluginConfigOption(_message.Message):
    __slots__ = ("value", "name", "description", "tags", "settings_schema")
    VALUE_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    SETTINGS_SCHEMA_FIELD_NUMBER: _ClassVar[int]
    value: str
    name: str
    description: str
    tags: _containers.RepeatedScalarFieldContainer[str]
    settings_schema: str
    def __init__(self, value: _Optional[str] = ..., name: _Optional[str] = ..., description: _Optional[str] = ..., tags: _Optional[_Iterable[str]] = ..., settings_schema: _Optional[str] = ...) -> None: ...

class CatalogPlugin(_message.Message):
    __slots__ = ("id", "name", "description", "author", "homepage", "catalog_url", "versions", "installed_version", "category")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    AUTHOR_FIELD_NUMBER: _ClassVar[int]
    HOMEPAGE_FIELD_NUMBER: _ClassVar[int]
    CATALOG_URL_FIELD_NUMBER: _ClassVar[int]
    VERSIONS_FIELD_NUMBER: _ClassVar[int]
    INSTALLED_VERSION_FIELD_NUMBER: _ClassVar[int]
    CATEGORY_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    description: str
    author: str
    homepage: str
    catalog_url: str
    versions: _containers.RepeatedCompositeFieldContainer[CatalogVersion]
    installed_version: str
    category: _plugin_pb2.Category
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., description: _Optional[str] = ..., author: _Optional[str] = ..., homepage: _Optional[str] = ..., catalog_url: _Optional[str] = ..., versions: _Optional[_Iterable[_Union[CatalogVersion, _Mapping]]] = ..., installed_version: _Optional[str] = ..., category: _Optional[_Union[_plugin_pb2.Category, str]] = ...) -> None: ...

class CatalogVersion(_message.Message):
    __slots__ = ("version", "changelog", "release_time")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    CHANGELOG_FIELD_NUMBER: _ClassVar[int]
    RELEASE_TIME_FIELD_NUMBER: _ClassVar[int]
    version: str
    changelog: str
    release_time: _timestamp_pb2.Timestamp
    def __init__(self, version: _Optional[str] = ..., changelog: _Optional[str] = ..., release_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListCatalogPluginsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListCatalogPluginsResponse(_message.Message):
    __slots__ = ("plugins",)
    PLUGINS_FIELD_NUMBER: _ClassVar[int]
    plugins: _containers.RepeatedCompositeFieldContainer[CatalogPlugin]
    def __init__(self, plugins: _Optional[_Iterable[_Union[CatalogPlugin, _Mapping]]] = ...) -> None: ...

class InstallPluginRequest(_message.Message):
    __slots__ = ("plugin_id", "version")
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    version: str
    def __init__(self, plugin_id: _Optional[str] = ..., version: _Optional[str] = ...) -> None: ...

class InstallPluginResponse(_message.Message):
    __slots__ = ("plugin",)
    PLUGIN_FIELD_NUMBER: _ClassVar[int]
    plugin: Plugin
    def __init__(self, plugin: _Optional[_Union[Plugin, _Mapping]] = ...) -> None: ...

class UninstallPluginRequest(_message.Message):
    __slots__ = ("plugin_id",)
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    def __init__(self, plugin_id: _Optional[str] = ...) -> None: ...

class UninstallPluginResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SetPluginEnabledRequest(_message.Message):
    __slots__ = ("plugin_id", "enabled")
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    enabled: bool
    def __init__(self, plugin_id: _Optional[str] = ..., enabled: _Optional[bool] = ...) -> None: ...

class SetPluginEnabledResponse(_message.Message):
    __slots__ = ("plugin",)
    PLUGIN_FIELD_NUMBER: _ClassVar[int]
    plugin: Plugin
    def __init__(self, plugin: _Optional[_Union[Plugin, _Mapping]] = ...) -> None: ...

class RestartPluginRequest(_message.Message):
    __slots__ = ("plugin_id",)
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    def __init__(self, plugin_id: _Optional[str] = ...) -> None: ...

class RestartPluginResponse(_message.Message):
    __slots__ = ("plugin",)
    PLUGIN_FIELD_NUMBER: _ClassVar[int]
    plugin: Plugin
    def __init__(self, plugin: _Optional[_Union[Plugin, _Mapping]] = ...) -> None: ...

class ContainerRuntime(_message.Message):
    __slots__ = ("available", "reason", "endpoint", "engine", "version", "api_version", "rootless", "in_container", "on_host", "gpu")
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    ENDPOINT_FIELD_NUMBER: _ClassVar[int]
    ENGINE_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    API_VERSION_FIELD_NUMBER: _ClassVar[int]
    ROOTLESS_FIELD_NUMBER: _ClassVar[int]
    IN_CONTAINER_FIELD_NUMBER: _ClassVar[int]
    ON_HOST_FIELD_NUMBER: _ClassVar[int]
    GPU_FIELD_NUMBER: _ClassVar[int]
    available: bool
    reason: str
    endpoint: str
    engine: str
    version: str
    api_version: str
    rootless: bool
    in_container: bool
    on_host: bool
    gpu: bool
    def __init__(self, available: _Optional[bool] = ..., reason: _Optional[str] = ..., endpoint: _Optional[str] = ..., engine: _Optional[str] = ..., version: _Optional[str] = ..., api_version: _Optional[str] = ..., rootless: _Optional[bool] = ..., in_container: _Optional[bool] = ..., on_host: _Optional[bool] = ..., gpu: _Optional[bool] = ...) -> None: ...

class GetContainerRuntimeRequest(_message.Message):
    __slots__ = ("detect",)
    DETECT_FIELD_NUMBER: _ClassVar[int]
    detect: bool
    def __init__(self, detect: _Optional[bool] = ...) -> None: ...

class GetContainerRuntimeResponse(_message.Message):
    __slots__ = ("runtime",)
    RUNTIME_FIELD_NUMBER: _ClassVar[int]
    runtime: ContainerRuntime
    def __init__(self, runtime: _Optional[_Union[ContainerRuntime, _Mapping]] = ...) -> None: ...

class PluginContainer(_message.Message):
    __slots__ = ("dependency", "status", "use_gpu", "commands", "gpu_commands", "plugin_id", "proxy", "command")
    DEPENDENCY_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    USE_GPU_FIELD_NUMBER: _ClassVar[int]
    COMMANDS_FIELD_NUMBER: _ClassVar[int]
    GPU_COMMANDS_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    PROXY_FIELD_NUMBER: _ClassVar[int]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    dependency: _container_pb2.ContainerDependency
    status: _container_pb2.ContainerStatus
    use_gpu: bool
    commands: ContainerCommands
    gpu_commands: ContainerCommands
    plugin_id: str
    proxy: str
    command: str
    def __init__(self, dependency: _Optional[_Union[_container_pb2.ContainerDependency, _Mapping]] = ..., status: _Optional[_Union[_container_pb2.ContainerStatus, _Mapping]] = ..., use_gpu: _Optional[bool] = ..., commands: _Optional[_Union[ContainerCommands, _Mapping]] = ..., gpu_commands: _Optional[_Union[ContainerCommands, _Mapping]] = ..., plugin_id: _Optional[str] = ..., proxy: _Optional[str] = ..., command: _Optional[str] = ...) -> None: ...

class ContainerCommands(_message.Message):
    __slots__ = ("docker_run", "compose", "address")
    DOCKER_RUN_FIELD_NUMBER: _ClassVar[int]
    COMPOSE_FIELD_NUMBER: _ClassVar[int]
    ADDRESS_FIELD_NUMBER: _ClassVar[int]
    docker_run: str
    compose: str
    address: str
    def __init__(self, docker_run: _Optional[str] = ..., compose: _Optional[str] = ..., address: _Optional[str] = ...) -> None: ...

class ListPluginContainersRequest(_message.Message):
    __slots__ = ("plugin_id",)
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    def __init__(self, plugin_id: _Optional[str] = ...) -> None: ...

class ListPluginContainersResponse(_message.Message):
    __slots__ = ("containers",)
    CONTAINERS_FIELD_NUMBER: _ClassVar[int]
    containers: _containers.RepeatedCompositeFieldContainer[PluginContainer]
    def __init__(self, containers: _Optional[_Iterable[_Union[PluginContainer, _Mapping]]] = ...) -> None: ...

class ListContainersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListContainersResponse(_message.Message):
    __slots__ = ("containers",)
    CONTAINERS_FIELD_NUMBER: _ClassVar[int]
    containers: _containers.RepeatedCompositeFieldContainer[PluginContainer]
    def __init__(self, containers: _Optional[_Iterable[_Union[PluginContainer, _Mapping]]] = ...) -> None: ...

class SetPluginContainerRequest(_message.Message):
    __slots__ = ("plugin_id", "name", "mode", "address", "use_gpu", "proxy")
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    ADDRESS_FIELD_NUMBER: _ClassVar[int]
    USE_GPU_FIELD_NUMBER: _ClassVar[int]
    PROXY_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    name: str
    mode: _container_pb2.ContainerMode
    address: str
    use_gpu: bool
    proxy: str
    def __init__(self, plugin_id: _Optional[str] = ..., name: _Optional[str] = ..., mode: _Optional[_Union[_container_pb2.ContainerMode, str]] = ..., address: _Optional[str] = ..., use_gpu: _Optional[bool] = ..., proxy: _Optional[str] = ...) -> None: ...

class SetPluginContainerResponse(_message.Message):
    __slots__ = ("container",)
    CONTAINER_FIELD_NUMBER: _ClassVar[int]
    container: PluginContainer
    def __init__(self, container: _Optional[_Union[PluginContainer, _Mapping]] = ...) -> None: ...

class SetPluginProxyRequest(_message.Message):
    __slots__ = ("plugin_id", "proxy")
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    PROXY_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    proxy: str
    def __init__(self, plugin_id: _Optional[str] = ..., proxy: _Optional[str] = ...) -> None: ...

class SetPluginProxyResponse(_message.Message):
    __slots__ = ("plugin",)
    PLUGIN_FIELD_NUMBER: _ClassVar[int]
    plugin: Plugin
    def __init__(self, plugin: _Optional[_Union[Plugin, _Mapping]] = ...) -> None: ...

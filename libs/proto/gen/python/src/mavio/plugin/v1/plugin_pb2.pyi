from mavio.plugin.v1 import container_pb2 as _container_pb2
from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from mavio.plugin.v1 import task_pb2 as _task_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Runtime(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    RUNTIME_UNSPECIFIED: _ClassVar[Runtime]
    RUNTIME_WASM: _ClassVar[Runtime]
    RUNTIME_PROCESS: _ClassVar[Runtime]

class Capability(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CAPABILITY_UNSPECIFIED: _ClassVar[Capability]
    CAPABILITY_METADATA_PROVIDER: _ClassVar[Capability]
    CAPABILITY_AUTH_PROVIDER: _ClassVar[Capability]
    CAPABILITY_NOTIFIER: _ClassVar[Capability]
    CAPABILITY_SUBTITLE_PROVIDER: _ClassVar[Capability]
    CAPABILITY_SEGMENT_PROVIDER: _ClassVar[Capability]
    CAPABILITY_TASK_RUNNER: _ClassVar[Capability]
    CAPABILITY_EVENT_CONSUMER: _ClassVar[Capability]
    CAPABILITY_HTTP_HANDLER: _ClassVar[Capability]
    CAPABILITY_DEVICE_CONTROLLER: _ClassVar[Capability]
    CAPABILITY_IMAGE_PROVIDER: _ClassVar[Capability]
    CAPABILITY_LOCAL_METADATA: _ClassVar[Capability]
    CAPABILITY_METADATA_SAVER: _ClassVar[Capability]
    CAPABILITY_METADATA_PROCESSOR: _ClassVar[Capability]
    CAPABILITY_LYRICS_PROVIDER: _ClassVar[Capability]
    CAPABILITY_RESOLVER: _ClassVar[Capability]
    CAPABILITY_INTRO_PROVIDER: _ClassVar[Capability]
    CAPABILITY_IMAGE_GENERATOR: _ClassVar[Capability]
    CAPABILITY_MEDIA_SOURCE_PROVIDER: _ClassVar[Capability]
    CAPABILITY_PASSWORD_RESET: _ClassVar[Capability]
    CAPABILITY_EPISODE_LIST_PROVIDER: _ClassVar[Capability]
    CAPABILITY_DISCOVERY_PROVIDER: _ClassVar[Capability]
    CAPABILITY_SCENE_MAPPING_PROVIDER: _ClassVar[Capability]
    CAPABILITY_INDEXER_PROVIDER: _ClassVar[Capability]
    CAPABILITY_DOWNLOAD_CLIENT_PROVIDER: _ClassVar[Capability]
    CAPABILITY_SUBTITLE_TRANSLATOR: _ClassVar[Capability]
    CAPABILITY_AI_GATEWAY: _ClassVar[Capability]
    CAPABILITY_AGENT_SESSION: _ClassVar[Capability]
    CAPABILITY_RATINGS_PROVIDER: _ClassVar[Capability]
    CAPABILITY_SPEECH_TO_TEXT: _ClassVar[Capability]
    CAPABILITY_TEXT_TO_SPEECH: _ClassVar[Capability]
    CAPABILITY_CHALLENGE_SOLVER: _ClassVar[Capability]
    CAPABILITY_VOICE_SESSION: _ClassVar[Capability]

class Category(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CATEGORY_UNSPECIFIED: _ClassVar[Category]
    CATEGORY_METADATA: _ClassVar[Category]
    CATEGORY_SUBTITLES: _ClassVar[Category]
    CATEGORY_DOWNLOADS: _ClassVar[Category]
    CATEGORY_AUTHENTICATION: _ClassVar[Category]
    CATEGORY_NOTIFICATIONS: _ClassVar[Category]
    CATEGORY_DEVICES: _ClassVar[Category]
    CATEGORY_AI: _ClassVar[Category]
    CATEGORY_SPEECH: _ClassVar[Category]
RUNTIME_UNSPECIFIED: Runtime
RUNTIME_WASM: Runtime
RUNTIME_PROCESS: Runtime
CAPABILITY_UNSPECIFIED: Capability
CAPABILITY_METADATA_PROVIDER: Capability
CAPABILITY_AUTH_PROVIDER: Capability
CAPABILITY_NOTIFIER: Capability
CAPABILITY_SUBTITLE_PROVIDER: Capability
CAPABILITY_SEGMENT_PROVIDER: Capability
CAPABILITY_TASK_RUNNER: Capability
CAPABILITY_EVENT_CONSUMER: Capability
CAPABILITY_HTTP_HANDLER: Capability
CAPABILITY_DEVICE_CONTROLLER: Capability
CAPABILITY_IMAGE_PROVIDER: Capability
CAPABILITY_LOCAL_METADATA: Capability
CAPABILITY_METADATA_SAVER: Capability
CAPABILITY_METADATA_PROCESSOR: Capability
CAPABILITY_LYRICS_PROVIDER: Capability
CAPABILITY_RESOLVER: Capability
CAPABILITY_INTRO_PROVIDER: Capability
CAPABILITY_IMAGE_GENERATOR: Capability
CAPABILITY_MEDIA_SOURCE_PROVIDER: Capability
CAPABILITY_PASSWORD_RESET: Capability
CAPABILITY_EPISODE_LIST_PROVIDER: Capability
CAPABILITY_DISCOVERY_PROVIDER: Capability
CAPABILITY_SCENE_MAPPING_PROVIDER: Capability
CAPABILITY_INDEXER_PROVIDER: Capability
CAPABILITY_DOWNLOAD_CLIENT_PROVIDER: Capability
CAPABILITY_SUBTITLE_TRANSLATOR: Capability
CAPABILITY_AI_GATEWAY: Capability
CAPABILITY_AGENT_SESSION: Capability
CAPABILITY_RATINGS_PROVIDER: Capability
CAPABILITY_SPEECH_TO_TEXT: Capability
CAPABILITY_TEXT_TO_SPEECH: Capability
CAPABILITY_CHALLENGE_SOLVER: Capability
CAPABILITY_VOICE_SESSION: Capability
CATEGORY_UNSPECIFIED: Category
CATEGORY_METADATA: Category
CATEGORY_SUBTITLES: Category
CATEGORY_DOWNLOADS: Category
CATEGORY_AUTHENTICATION: Category
CATEGORY_NOTIFICATIONS: Category
CATEGORY_DEVICES: Category
CATEGORY_AI: Category
CATEGORY_SPEECH: Category

class Manifest(_message.Message):
    __slots__ = ("id", "name", "version", "description", "author", "homepage", "license", "runtime", "capabilities", "permissions", "config_schema", "api_version", "tasks", "config_page", "external_id_kinds", "local_metadata_files", "category", "containers")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    AUTHOR_FIELD_NUMBER: _ClassVar[int]
    HOMEPAGE_FIELD_NUMBER: _ClassVar[int]
    LICENSE_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_FIELD_NUMBER: _ClassVar[int]
    CAPABILITIES_FIELD_NUMBER: _ClassVar[int]
    PERMISSIONS_FIELD_NUMBER: _ClassVar[int]
    CONFIG_SCHEMA_FIELD_NUMBER: _ClassVar[int]
    API_VERSION_FIELD_NUMBER: _ClassVar[int]
    TASKS_FIELD_NUMBER: _ClassVar[int]
    CONFIG_PAGE_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_ID_KINDS_FIELD_NUMBER: _ClassVar[int]
    LOCAL_METADATA_FILES_FIELD_NUMBER: _ClassVar[int]
    CATEGORY_FIELD_NUMBER: _ClassVar[int]
    CONTAINERS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    version: str
    description: str
    author: str
    homepage: str
    license: str
    runtime: Runtime
    capabilities: _containers.RepeatedScalarFieldContainer[Capability]
    permissions: Permissions
    config_schema: str
    api_version: str
    tasks: _containers.RepeatedCompositeFieldContainer[_task_pb2.Task]
    config_page: str
    external_id_kinds: _containers.RepeatedCompositeFieldContainer[ExternalIdKind]
    local_metadata_files: _containers.RepeatedScalarFieldContainer[str]
    category: Category
    containers: _containers.RepeatedCompositeFieldContainer[_container_pb2.ContainerDependency]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., version: _Optional[str] = ..., description: _Optional[str] = ..., author: _Optional[str] = ..., homepage: _Optional[str] = ..., license: _Optional[str] = ..., runtime: _Optional[_Union[Runtime, str]] = ..., capabilities: _Optional[_Iterable[_Union[Capability, str]]] = ..., permissions: _Optional[_Union[Permissions, _Mapping]] = ..., config_schema: _Optional[str] = ..., api_version: _Optional[str] = ..., tasks: _Optional[_Iterable[_Union[_task_pb2.Task, _Mapping]]] = ..., config_page: _Optional[str] = ..., external_id_kinds: _Optional[_Iterable[_Union[ExternalIdKind, _Mapping]]] = ..., local_metadata_files: _Optional[_Iterable[str]] = ..., category: _Optional[_Union[Category, str]] = ..., containers: _Optional[_Iterable[_Union[_container_pb2.ContainerDependency, _Mapping]]] = ...) -> None: ...

class ExternalIdKind(_message.Message):
    __slots__ = ("key", "name", "media_kinds", "url_template")
    KEY_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    MEDIA_KINDS_FIELD_NUMBER: _ClassVar[int]
    URL_TEMPLATE_FIELD_NUMBER: _ClassVar[int]
    key: str
    name: str
    media_kinds: _containers.RepeatedScalarFieldContainer[_metadata_pb2.MediaKind]
    url_template: str
    def __init__(self, key: _Optional[str] = ..., name: _Optional[str] = ..., media_kinds: _Optional[_Iterable[_Union[_metadata_pb2.MediaKind, str]]] = ..., url_template: _Optional[str] = ...) -> None: ...

class Permissions(_message.Message):
    __slots__ = ("http_hosts", "read_paths", "api", "act_as_users", "events")
    HTTP_HOSTS_FIELD_NUMBER: _ClassVar[int]
    READ_PATHS_FIELD_NUMBER: _ClassVar[int]
    API_FIELD_NUMBER: _ClassVar[int]
    ACT_AS_USERS_FIELD_NUMBER: _ClassVar[int]
    EVENTS_FIELD_NUMBER: _ClassVar[int]
    http_hosts: _containers.RepeatedScalarFieldContainer[str]
    read_paths: _containers.RepeatedScalarFieldContainer[str]
    api: _containers.RepeatedScalarFieldContainer[str]
    act_as_users: bool
    events: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, http_hosts: _Optional[_Iterable[str]] = ..., read_paths: _Optional[_Iterable[str]] = ..., api: _Optional[_Iterable[str]] = ..., act_as_users: _Optional[bool] = ..., events: _Optional[_Iterable[str]] = ...) -> None: ...

class DescribeRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class DescribeResponse(_message.Message):
    __slots__ = ("manifest",)
    MANIFEST_FIELD_NUMBER: _ClassVar[int]
    manifest: Manifest
    def __init__(self, manifest: _Optional[_Union[Manifest, _Mapping]] = ...) -> None: ...

class ConfigureRequest(_message.Message):
    __slots__ = ("config_json",)
    CONFIG_JSON_FIELD_NUMBER: _ClassVar[int]
    config_json: str
    def __init__(self, config_json: _Optional[str] = ...) -> None: ...

class ConfigureResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class HealthRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class HealthResponse(_message.Message):
    __slots__ = ("healthy", "message")
    HEALTHY_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    healthy: bool
    message: str
    def __init__(self, healthy: _Optional[bool] = ..., message: _Optional[str] = ...) -> None: ...

class ShutdownRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ShutdownResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

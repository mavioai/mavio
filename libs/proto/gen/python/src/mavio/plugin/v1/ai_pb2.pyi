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

class AiCredential(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    AI_CREDENTIAL_UNSPECIFIED: _ClassVar[AiCredential]
    AI_CREDENTIAL_API_KEY: _ClassVar[AiCredential]
    AI_CREDENTIAL_OAUTH: _ClassVar[AiCredential]

class AiLoginState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    AI_LOGIN_STATE_UNSPECIFIED: _ClassVar[AiLoginState]
    AI_LOGIN_STATE_RUNNING: _ClassVar[AiLoginState]
    AI_LOGIN_STATE_SUCCEEDED: _ClassVar[AiLoginState]
    AI_LOGIN_STATE_FAILED: _ClassVar[AiLoginState]
    AI_LOGIN_STATE_CANCELED: _ClassVar[AiLoginState]

class AiLoginPromptKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    AI_LOGIN_PROMPT_KIND_UNSPECIFIED: _ClassVar[AiLoginPromptKind]
    AI_LOGIN_PROMPT_KIND_TEXT: _ClassVar[AiLoginPromptKind]
    AI_LOGIN_PROMPT_KIND_SECRET: _ClassVar[AiLoginPromptKind]
    AI_LOGIN_PROMPT_KIND_SELECT: _ClassVar[AiLoginPromptKind]
    AI_LOGIN_PROMPT_KIND_MANUAL_CODE: _ClassVar[AiLoginPromptKind]

class AiRole(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    AI_ROLE_UNSPECIFIED: _ClassVar[AiRole]
    AI_ROLE_USER: _ClassVar[AiRole]
    AI_ROLE_ASSISTANT: _ClassVar[AiRole]

class AiThinking(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    AI_THINKING_UNSPECIFIED: _ClassVar[AiThinking]
    AI_THINKING_OFF: _ClassVar[AiThinking]
    AI_THINKING_MINIMAL: _ClassVar[AiThinking]
    AI_THINKING_LOW: _ClassVar[AiThinking]
    AI_THINKING_MEDIUM: _ClassVar[AiThinking]
    AI_THINKING_HIGH: _ClassVar[AiThinking]
AI_CREDENTIAL_UNSPECIFIED: AiCredential
AI_CREDENTIAL_API_KEY: AiCredential
AI_CREDENTIAL_OAUTH: AiCredential
AI_LOGIN_STATE_UNSPECIFIED: AiLoginState
AI_LOGIN_STATE_RUNNING: AiLoginState
AI_LOGIN_STATE_SUCCEEDED: AiLoginState
AI_LOGIN_STATE_FAILED: AiLoginState
AI_LOGIN_STATE_CANCELED: AiLoginState
AI_LOGIN_PROMPT_KIND_UNSPECIFIED: AiLoginPromptKind
AI_LOGIN_PROMPT_KIND_TEXT: AiLoginPromptKind
AI_LOGIN_PROMPT_KIND_SECRET: AiLoginPromptKind
AI_LOGIN_PROMPT_KIND_SELECT: AiLoginPromptKind
AI_LOGIN_PROMPT_KIND_MANUAL_CODE: AiLoginPromptKind
AI_ROLE_UNSPECIFIED: AiRole
AI_ROLE_USER: AiRole
AI_ROLE_ASSISTANT: AiRole
AI_THINKING_UNSPECIFIED: AiThinking
AI_THINKING_OFF: AiThinking
AI_THINKING_MINIMAL: AiThinking
AI_THINKING_LOW: AiThinking
AI_THINKING_MEDIUM: AiThinking
AI_THINKING_HIGH: AiThinking

class ListAiProvidersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListAiProvidersResponse(_message.Message):
    __slots__ = ("providers",)
    PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    providers: _containers.RepeatedCompositeFieldContainer[AiProvider]
    def __init__(self, providers: _Optional[_Iterable[_Union[AiProvider, _Mapping]]] = ...) -> None: ...

class AiProvider(_message.Message):
    __slots__ = ("id", "name", "api_key", "oauth", "subscription", "oauth_label", "credential", "available", "source", "custom")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    API_KEY_FIELD_NUMBER: _ClassVar[int]
    OAUTH_FIELD_NUMBER: _ClassVar[int]
    SUBSCRIPTION_FIELD_NUMBER: _ClassVar[int]
    OAUTH_LABEL_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    CUSTOM_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    api_key: bool
    oauth: bool
    subscription: bool
    oauth_label: str
    credential: AiCredential
    available: bool
    source: str
    custom: bool
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., api_key: _Optional[bool] = ..., oauth: _Optional[bool] = ..., subscription: _Optional[bool] = ..., oauth_label: _Optional[str] = ..., credential: _Optional[_Union[AiCredential, str]] = ..., available: _Optional[bool] = ..., source: _Optional[str] = ..., custom: _Optional[bool] = ...) -> None: ...

class ListAiModelsRequest(_message.Message):
    __slots__ = ("provider", "available")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_FIELD_NUMBER: _ClassVar[int]
    provider: str
    available: bool
    def __init__(self, provider: _Optional[str] = ..., available: _Optional[bool] = ...) -> None: ...

class ListAiModelsResponse(_message.Message):
    __slots__ = ("models",)
    MODELS_FIELD_NUMBER: _ClassVar[int]
    models: _containers.RepeatedCompositeFieldContainer[AiModel]
    def __init__(self, models: _Optional[_Iterable[_Union[AiModel, _Mapping]]] = ...) -> None: ...

class AiModel(_message.Message):
    __slots__ = ("provider", "id", "name", "reasoning", "images", "context_window", "max_output_tokens", "cost")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    REASONING_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_WINDOW_FIELD_NUMBER: _ClassVar[int]
    MAX_OUTPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    COST_FIELD_NUMBER: _ClassVar[int]
    provider: str
    id: str
    name: str
    reasoning: bool
    images: bool
    context_window: int
    max_output_tokens: int
    cost: AiModelCost
    def __init__(self, provider: _Optional[str] = ..., id: _Optional[str] = ..., name: _Optional[str] = ..., reasoning: _Optional[bool] = ..., images: _Optional[bool] = ..., context_window: _Optional[int] = ..., max_output_tokens: _Optional[int] = ..., cost: _Optional[_Union[AiModelCost, _Mapping]] = ...) -> None: ...

class AiModelCost(_message.Message):
    __slots__ = ("input", "output", "cache_read", "cache_write")
    INPUT_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_FIELD_NUMBER: _ClassVar[int]
    CACHE_READ_FIELD_NUMBER: _ClassVar[int]
    CACHE_WRITE_FIELD_NUMBER: _ClassVar[int]
    input: float
    output: float
    cache_read: float
    cache_write: float
    def __init__(self, input: _Optional[float] = ..., output: _Optional[float] = ..., cache_read: _Optional[float] = ..., cache_write: _Optional[float] = ...) -> None: ...

class SetAiApiKeyRequest(_message.Message):
    __slots__ = ("provider", "key")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    KEY_FIELD_NUMBER: _ClassVar[int]
    provider: str
    key: str
    def __init__(self, provider: _Optional[str] = ..., key: _Optional[str] = ...) -> None: ...

class SetAiApiKeyResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StartAiLoginRequest(_message.Message):
    __slots__ = ("provider",)
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    provider: str
    def __init__(self, provider: _Optional[str] = ...) -> None: ...

class StartAiLoginResponse(_message.Message):
    __slots__ = ("login_id",)
    LOGIN_ID_FIELD_NUMBER: _ClassVar[int]
    login_id: str
    def __init__(self, login_id: _Optional[str] = ...) -> None: ...

class GetAiLoginRequest(_message.Message):
    __slots__ = ("login_id",)
    LOGIN_ID_FIELD_NUMBER: _ClassVar[int]
    login_id: str
    def __init__(self, login_id: _Optional[str] = ...) -> None: ...

class GetAiLoginResponse(_message.Message):
    __slots__ = ("state", "events", "prompt", "error")
    STATE_FIELD_NUMBER: _ClassVar[int]
    EVENTS_FIELD_NUMBER: _ClassVar[int]
    PROMPT_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    state: AiLoginState
    events: _containers.RepeatedCompositeFieldContainer[AiLoginEvent]
    prompt: AiLoginPrompt
    error: str
    def __init__(self, state: _Optional[_Union[AiLoginState, str]] = ..., events: _Optional[_Iterable[_Union[AiLoginEvent, _Mapping]]] = ..., prompt: _Optional[_Union[AiLoginPrompt, _Mapping]] = ..., error: _Optional[str] = ...) -> None: ...

class AiLoginEvent(_message.Message):
    __slots__ = ("info", "url", "device_code", "progress")
    INFO_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    DEVICE_CODE_FIELD_NUMBER: _ClassVar[int]
    PROGRESS_FIELD_NUMBER: _ClassVar[int]
    info: AiLoginInfo
    url: AiLoginUrl
    device_code: AiLoginDeviceCode
    progress: str
    def __init__(self, info: _Optional[_Union[AiLoginInfo, _Mapping]] = ..., url: _Optional[_Union[AiLoginUrl, _Mapping]] = ..., device_code: _Optional[_Union[AiLoginDeviceCode, _Mapping]] = ..., progress: _Optional[str] = ...) -> None: ...

class AiLoginInfo(_message.Message):
    __slots__ = ("message", "links")
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    LINKS_FIELD_NUMBER: _ClassVar[int]
    message: str
    links: _containers.RepeatedCompositeFieldContainer[AiLoginLink]
    def __init__(self, message: _Optional[str] = ..., links: _Optional[_Iterable[_Union[AiLoginLink, _Mapping]]] = ...) -> None: ...

class AiLoginLink(_message.Message):
    __slots__ = ("url", "label")
    URL_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    url: str
    label: str
    def __init__(self, url: _Optional[str] = ..., label: _Optional[str] = ...) -> None: ...

class AiLoginUrl(_message.Message):
    __slots__ = ("url", "instructions")
    URL_FIELD_NUMBER: _ClassVar[int]
    INSTRUCTIONS_FIELD_NUMBER: _ClassVar[int]
    url: str
    instructions: str
    def __init__(self, url: _Optional[str] = ..., instructions: _Optional[str] = ...) -> None: ...

class AiLoginDeviceCode(_message.Message):
    __slots__ = ("user_code", "verification_uri", "expires_in_seconds")
    USER_CODE_FIELD_NUMBER: _ClassVar[int]
    VERIFICATION_URI_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_IN_SECONDS_FIELD_NUMBER: _ClassVar[int]
    user_code: str
    verification_uri: str
    expires_in_seconds: int
    def __init__(self, user_code: _Optional[str] = ..., verification_uri: _Optional[str] = ..., expires_in_seconds: _Optional[int] = ...) -> None: ...

class AiLoginPrompt(_message.Message):
    __slots__ = ("id", "kind", "message", "placeholder", "options")
    ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    PLACEHOLDER_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    id: str
    kind: AiLoginPromptKind
    message: str
    placeholder: str
    options: _containers.RepeatedCompositeFieldContainer[AiLoginOption]
    def __init__(self, id: _Optional[str] = ..., kind: _Optional[_Union[AiLoginPromptKind, str]] = ..., message: _Optional[str] = ..., placeholder: _Optional[str] = ..., options: _Optional[_Iterable[_Union[AiLoginOption, _Mapping]]] = ...) -> None: ...

class AiLoginOption(_message.Message):
    __slots__ = ("id", "label", "description")
    ID_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    id: str
    label: str
    description: str
    def __init__(self, id: _Optional[str] = ..., label: _Optional[str] = ..., description: _Optional[str] = ...) -> None: ...

class WatchAiLoginRequest(_message.Message):
    __slots__ = ("login_id",)
    LOGIN_ID_FIELD_NUMBER: _ClassVar[int]
    login_id: str
    def __init__(self, login_id: _Optional[str] = ...) -> None: ...

class WatchAiLoginResponse(_message.Message):
    __slots__ = ("state", "events", "prompt", "error")
    STATE_FIELD_NUMBER: _ClassVar[int]
    EVENTS_FIELD_NUMBER: _ClassVar[int]
    PROMPT_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    state: AiLoginState
    events: _containers.RepeatedCompositeFieldContainer[AiLoginEvent]
    prompt: AiLoginPrompt
    error: str
    def __init__(self, state: _Optional[_Union[AiLoginState, str]] = ..., events: _Optional[_Iterable[_Union[AiLoginEvent, _Mapping]]] = ..., prompt: _Optional[_Union[AiLoginPrompt, _Mapping]] = ..., error: _Optional[str] = ...) -> None: ...

class AnswerAiLoginRequest(_message.Message):
    __slots__ = ("login_id", "prompt_id", "value")
    LOGIN_ID_FIELD_NUMBER: _ClassVar[int]
    PROMPT_ID_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    login_id: str
    prompt_id: str
    value: str
    def __init__(self, login_id: _Optional[str] = ..., prompt_id: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...

class AnswerAiLoginResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CancelAiLoginRequest(_message.Message):
    __slots__ = ("login_id",)
    LOGIN_ID_FIELD_NUMBER: _ClassVar[int]
    login_id: str
    def __init__(self, login_id: _Optional[str] = ...) -> None: ...

class CancelAiLoginResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class LogoutAiRequest(_message.Message):
    __slots__ = ("provider",)
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    provider: str
    def __init__(self, provider: _Optional[str] = ...) -> None: ...

class LogoutAiResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CompleteAiRequest(_message.Message):
    __slots__ = ("provider", "model", "system", "messages", "output_schema", "thinking", "max_tokens", "temperature")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    SYSTEM_FIELD_NUMBER: _ClassVar[int]
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_SCHEMA_FIELD_NUMBER: _ClassVar[int]
    THINKING_FIELD_NUMBER: _ClassVar[int]
    MAX_TOKENS_FIELD_NUMBER: _ClassVar[int]
    TEMPERATURE_FIELD_NUMBER: _ClassVar[int]
    provider: str
    model: str
    system: str
    messages: _containers.RepeatedCompositeFieldContainer[AiMessage]
    output_schema: str
    thinking: AiThinking
    max_tokens: int
    temperature: float
    def __init__(self, provider: _Optional[str] = ..., model: _Optional[str] = ..., system: _Optional[str] = ..., messages: _Optional[_Iterable[_Union[AiMessage, _Mapping]]] = ..., output_schema: _Optional[str] = ..., thinking: _Optional[_Union[AiThinking, str]] = ..., max_tokens: _Optional[int] = ..., temperature: _Optional[float] = ...) -> None: ...

class AiMessage(_message.Message):
    __slots__ = ("role", "text")
    ROLE_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    role: AiRole
    text: str
    def __init__(self, role: _Optional[_Union[AiRole, str]] = ..., text: _Optional[str] = ...) -> None: ...

class CompleteAiResponse(_message.Message):
    __slots__ = ("text", "output_json", "usage", "stop_reason")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_JSON_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    STOP_REASON_FIELD_NUMBER: _ClassVar[int]
    text: str
    output_json: str
    usage: AiUsage
    stop_reason: str
    def __init__(self, text: _Optional[str] = ..., output_json: _Optional[str] = ..., usage: _Optional[_Union[AiUsage, _Mapping]] = ..., stop_reason: _Optional[str] = ...) -> None: ...

class StreamAiRequest(_message.Message):
    __slots__ = ("completion",)
    COMPLETION_FIELD_NUMBER: _ClassVar[int]
    completion: CompleteAiRequest
    def __init__(self, completion: _Optional[_Union[CompleteAiRequest, _Mapping]] = ...) -> None: ...

class StreamAiResponse(_message.Message):
    __slots__ = ("text", "thinking", "done")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    THINKING_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    text: str
    thinking: str
    done: CompleteAiResponse
    def __init__(self, text: _Optional[str] = ..., thinking: _Optional[str] = ..., done: _Optional[_Union[CompleteAiResponse, _Mapping]] = ...) -> None: ...

class AiUsage(_message.Message):
    __slots__ = ("input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens", "cost")
    INPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    CACHE_READ_TOKENS_FIELD_NUMBER: _ClassVar[int]
    CACHE_WRITE_TOKENS_FIELD_NUMBER: _ClassVar[int]
    COST_FIELD_NUMBER: _ClassVar[int]
    input_tokens: int
    output_tokens: int
    cache_read_tokens: int
    cache_write_tokens: int
    cost: float
    def __init__(self, input_tokens: _Optional[int] = ..., output_tokens: _Optional[int] = ..., cache_read_tokens: _Optional[int] = ..., cache_write_tokens: _Optional[int] = ..., cost: _Optional[float] = ...) -> None: ...

class AgentSession(_message.Message):
    __slots__ = ("id", "title", "provider", "model", "created", "updated", "usage", "owner", "labels", "ended")
    class LabelsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    CREATED_FIELD_NUMBER: _ClassVar[int]
    UPDATED_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    OWNER_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    ENDED_FIELD_NUMBER: _ClassVar[int]
    id: str
    title: str
    provider: str
    model: str
    created: _timestamp_pb2.Timestamp
    updated: _timestamp_pb2.Timestamp
    usage: AiUsage
    owner: str
    labels: _containers.ScalarMap[str, str]
    ended: bool
    def __init__(self, id: _Optional[str] = ..., title: _Optional[str] = ..., provider: _Optional[str] = ..., model: _Optional[str] = ..., created: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., updated: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., usage: _Optional[_Union[AiUsage, _Mapping]] = ..., owner: _Optional[str] = ..., labels: _Optional[_Mapping[str, str]] = ..., ended: _Optional[bool] = ...) -> None: ...

class AgentTool(_message.Message):
    __slots__ = ("name", "description", "parameters_json", "result_json")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_JSON_FIELD_NUMBER: _ClassVar[int]
    RESULT_JSON_FIELD_NUMBER: _ClassVar[int]
    name: str
    description: str
    parameters_json: str
    result_json: str
    def __init__(self, name: _Optional[str] = ..., description: _Optional[str] = ..., parameters_json: _Optional[str] = ..., result_json: _Optional[str] = ...) -> None: ...

class AgentToolCall(_message.Message):
    __slots__ = ("id", "session_id", "owner", "tool", "arguments_json", "create_time")
    ID_FIELD_NUMBER: _ClassVar[int]
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    OWNER_FIELD_NUMBER: _ClassVar[int]
    TOOL_FIELD_NUMBER: _ClassVar[int]
    ARGUMENTS_JSON_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    id: str
    session_id: str
    owner: str
    tool: str
    arguments_json: str
    create_time: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., session_id: _Optional[str] = ..., owner: _Optional[str] = ..., tool: _Optional[str] = ..., arguments_json: _Optional[str] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class AgentMemory(_message.Message):
    __slots__ = ("id", "text", "create_time", "update_time", "use_time", "session_id")
    ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    USE_TIME_FIELD_NUMBER: _ClassVar[int]
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    text: str
    create_time: _timestamp_pb2.Timestamp
    update_time: _timestamp_pb2.Timestamp
    use_time: _timestamp_pb2.Timestamp
    session_id: str
    def __init__(self, id: _Optional[str] = ..., text: _Optional[str] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., use_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., session_id: _Optional[str] = ...) -> None: ...

class AgentMessage(_message.Message):
    __slots__ = ("role", "text", "time")
    ROLE_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    TIME_FIELD_NUMBER: _ClassVar[int]
    role: AiRole
    text: str
    time: _timestamp_pb2.Timestamp
    def __init__(self, role: _Optional[_Union[AiRole, str]] = ..., text: _Optional[str] = ..., time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class CreateAgentSessionRequest(_message.Message):
    __slots__ = ("provider", "model", "instructions", "title", "tools", "code_mode", "owner", "memory", "labels")
    class LabelsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    INSTRUCTIONS_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    TOOLS_FIELD_NUMBER: _ClassVar[int]
    CODE_MODE_FIELD_NUMBER: _ClassVar[int]
    OWNER_FIELD_NUMBER: _ClassVar[int]
    MEMORY_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    provider: str
    model: str
    instructions: str
    title: str
    tools: _containers.RepeatedCompositeFieldContainer[AgentTool]
    code_mode: bool
    owner: str
    memory: bool
    labels: _containers.ScalarMap[str, str]
    def __init__(self, provider: _Optional[str] = ..., model: _Optional[str] = ..., instructions: _Optional[str] = ..., title: _Optional[str] = ..., tools: _Optional[_Iterable[_Union[AgentTool, _Mapping]]] = ..., code_mode: _Optional[bool] = ..., owner: _Optional[str] = ..., memory: _Optional[bool] = ..., labels: _Optional[_Mapping[str, str]] = ...) -> None: ...

class CreateAgentSessionResponse(_message.Message):
    __slots__ = ("session",)
    SESSION_FIELD_NUMBER: _ClassVar[int]
    session: AgentSession
    def __init__(self, session: _Optional[_Union[AgentSession, _Mapping]] = ...) -> None: ...

class ListAgentSessionsRequest(_message.Message):
    __slots__ = ("owner",)
    OWNER_FIELD_NUMBER: _ClassVar[int]
    owner: str
    def __init__(self, owner: _Optional[str] = ...) -> None: ...

class ListAgentSessionsResponse(_message.Message):
    __slots__ = ("sessions",)
    SESSIONS_FIELD_NUMBER: _ClassVar[int]
    sessions: _containers.RepeatedCompositeFieldContainer[AgentSession]
    def __init__(self, sessions: _Optional[_Iterable[_Union[AgentSession, _Mapping]]] = ...) -> None: ...

class GetAgentSessionRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class GetAgentSessionResponse(_message.Message):
    __slots__ = ("session", "messages")
    SESSION_FIELD_NUMBER: _ClassVar[int]
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    session: AgentSession
    messages: _containers.RepeatedCompositeFieldContainer[AgentMessage]
    def __init__(self, session: _Optional[_Union[AgentSession, _Mapping]] = ..., messages: _Optional[_Iterable[_Union[AgentMessage, _Mapping]]] = ...) -> None: ...

class SendAgentMessageRequest(_message.Message):
    __slots__ = ("session_id", "text", "request_id")
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    text: str
    request_id: str
    def __init__(self, session_id: _Optional[str] = ..., text: _Optional[str] = ..., request_id: _Optional[str] = ...) -> None: ...

class SendAgentMessageResponse(_message.Message):
    __slots__ = ("answer", "usage")
    ANSWER_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    answer: AgentMessage
    usage: AiUsage
    def __init__(self, answer: _Optional[_Union[AgentMessage, _Mapping]] = ..., usage: _Optional[_Union[AiUsage, _Mapping]] = ...) -> None: ...

class StreamAgentMessageRequest(_message.Message):
    __slots__ = ("message",)
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    message: SendAgentMessageRequest
    def __init__(self, message: _Optional[_Union[SendAgentMessageRequest, _Mapping]] = ...) -> None: ...

class StreamAgentMessageResponse(_message.Message):
    __slots__ = ("text", "done")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    text: str
    done: SendAgentMessageResponse
    def __init__(self, text: _Optional[str] = ..., done: _Optional[_Union[SendAgentMessageResponse, _Mapping]] = ...) -> None: ...

class DeleteAgentSessionRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteAgentSessionResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class AddAgentMessagesRequest(_message.Message):
    __slots__ = ("session_id", "messages", "request_id")
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    messages: _containers.RepeatedCompositeFieldContainer[AgentMessage]
    request_id: str
    def __init__(self, session_id: _Optional[str] = ..., messages: _Optional[_Iterable[_Union[AgentMessage, _Mapping]]] = ..., request_id: _Optional[str] = ...) -> None: ...

class AddAgentMessagesResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class EndAgentSessionRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class EndAgentSessionResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class WatchAgentToolCallsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class WatchAgentToolCallsResponse(_message.Message):
    __slots__ = ("call",)
    CALL_FIELD_NUMBER: _ClassVar[int]
    call: AgentToolCall
    def __init__(self, call: _Optional[_Union[AgentToolCall, _Mapping]] = ...) -> None: ...

class SubmitAgentToolResultRequest(_message.Message):
    __slots__ = ("call_id", "text", "result_json", "failed")
    CALL_ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    RESULT_JSON_FIELD_NUMBER: _ClassVar[int]
    FAILED_FIELD_NUMBER: _ClassVar[int]
    call_id: str
    text: str
    result_json: str
    failed: bool
    def __init__(self, call_id: _Optional[str] = ..., text: _Optional[str] = ..., result_json: _Optional[str] = ..., failed: _Optional[bool] = ...) -> None: ...

class SubmitAgentToolResultResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListAgentMemoriesRequest(_message.Message):
    __slots__ = ("owner",)
    OWNER_FIELD_NUMBER: _ClassVar[int]
    owner: str
    def __init__(self, owner: _Optional[str] = ...) -> None: ...

class ListAgentMemoriesResponse(_message.Message):
    __slots__ = ("memories",)
    MEMORIES_FIELD_NUMBER: _ClassVar[int]
    memories: _containers.RepeatedCompositeFieldContainer[AgentMemory]
    def __init__(self, memories: _Optional[_Iterable[_Union[AgentMemory, _Mapping]]] = ...) -> None: ...

class PutAgentMemoryRequest(_message.Message):
    __slots__ = ("owner", "id", "text")
    OWNER_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    owner: str
    id: str
    text: str
    def __init__(self, owner: _Optional[str] = ..., id: _Optional[str] = ..., text: _Optional[str] = ...) -> None: ...

class PutAgentMemoryResponse(_message.Message):
    __slots__ = ("memory",)
    MEMORY_FIELD_NUMBER: _ClassVar[int]
    memory: AgentMemory
    def __init__(self, memory: _Optional[_Union[AgentMemory, _Mapping]] = ...) -> None: ...

class DeleteAgentMemoriesRequest(_message.Message):
    __slots__ = ("owner", "ids")
    OWNER_FIELD_NUMBER: _ClassVar[int]
    IDS_FIELD_NUMBER: _ClassVar[int]
    owner: str
    ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, owner: _Optional[str] = ..., ids: _Optional[_Iterable[str]] = ...) -> None: ...

class DeleteAgentMemoriesResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

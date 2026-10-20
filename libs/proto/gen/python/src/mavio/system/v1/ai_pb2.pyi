from buf.validate import validate_pb2 as _validate_pb2
from mavio.plugin.v1 import ai_pb2 as _ai_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ListAiProvidersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListAiProvidersResponse(_message.Message):
    __slots__ = ("plugin_id", "providers")
    PLUGIN_ID_FIELD_NUMBER: _ClassVar[int]
    PROVIDERS_FIELD_NUMBER: _ClassVar[int]
    plugin_id: str
    providers: _containers.RepeatedCompositeFieldContainer[_ai_pb2.AiProvider]
    def __init__(self, plugin_id: _Optional[str] = ..., providers: _Optional[_Iterable[_Union[_ai_pb2.AiProvider, _Mapping]]] = ...) -> None: ...

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
    models: _containers.RepeatedCompositeFieldContainer[_ai_pb2.AiModel]
    def __init__(self, models: _Optional[_Iterable[_Union[_ai_pb2.AiModel, _Mapping]]] = ...) -> None: ...

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
    state: _ai_pb2.AiLoginState
    events: _containers.RepeatedCompositeFieldContainer[_ai_pb2.AiLoginEvent]
    prompt: _ai_pb2.AiLoginPrompt
    error: str
    def __init__(self, state: _Optional[_Union[_ai_pb2.AiLoginState, str]] = ..., events: _Optional[_Iterable[_Union[_ai_pb2.AiLoginEvent, _Mapping]]] = ..., prompt: _Optional[_Union[_ai_pb2.AiLoginPrompt, _Mapping]] = ..., error: _Optional[str] = ...) -> None: ...

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
    state: _ai_pb2.AiLoginState
    events: _containers.RepeatedCompositeFieldContainer[_ai_pb2.AiLoginEvent]
    prompt: _ai_pb2.AiLoginPrompt
    error: str
    def __init__(self, state: _Optional[_Union[_ai_pb2.AiLoginState, str]] = ..., events: _Optional[_Iterable[_Union[_ai_pb2.AiLoginEvent, _Mapping]]] = ..., prompt: _Optional[_Union[_ai_pb2.AiLoginPrompt, _Mapping]] = ..., error: _Optional[str] = ...) -> None: ...

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

class CreateAiConversationRequest(_message.Message):
    __slots__ = ("provider", "model", "instructions", "title")
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    INSTRUCTIONS_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    provider: str
    model: str
    instructions: str
    title: str
    def __init__(self, provider: _Optional[str] = ..., model: _Optional[str] = ..., instructions: _Optional[str] = ..., title: _Optional[str] = ...) -> None: ...

class CreateAiConversationResponse(_message.Message):
    __slots__ = ("conversation",)
    CONVERSATION_FIELD_NUMBER: _ClassVar[int]
    conversation: _ai_pb2.AgentSession
    def __init__(self, conversation: _Optional[_Union[_ai_pb2.AgentSession, _Mapping]] = ...) -> None: ...

class ListAiConversationsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListAiConversationsResponse(_message.Message):
    __slots__ = ("conversations",)
    CONVERSATIONS_FIELD_NUMBER: _ClassVar[int]
    conversations: _containers.RepeatedCompositeFieldContainer[_ai_pb2.AgentSession]
    def __init__(self, conversations: _Optional[_Iterable[_Union[_ai_pb2.AgentSession, _Mapping]]] = ...) -> None: ...

class GetAiConversationRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class GetAiConversationResponse(_message.Message):
    __slots__ = ("conversation", "messages")
    CONVERSATION_FIELD_NUMBER: _ClassVar[int]
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    conversation: _ai_pb2.AgentSession
    messages: _containers.RepeatedCompositeFieldContainer[_ai_pb2.AgentMessage]
    def __init__(self, conversation: _Optional[_Union[_ai_pb2.AgentSession, _Mapping]] = ..., messages: _Optional[_Iterable[_Union[_ai_pb2.AgentMessage, _Mapping]]] = ...) -> None: ...

class SendAiMessageRequest(_message.Message):
    __slots__ = ("conversation_id", "text", "request_id")
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    text: str
    request_id: str
    def __init__(self, conversation_id: _Optional[str] = ..., text: _Optional[str] = ..., request_id: _Optional[str] = ...) -> None: ...

class SendAiMessageResponse(_message.Message):
    __slots__ = ("text", "done")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    text: str
    done: _ai_pb2.SendAgentMessageResponse
    def __init__(self, text: _Optional[str] = ..., done: _Optional[_Union[_ai_pb2.SendAgentMessageResponse, _Mapping]] = ...) -> None: ...

class DeleteAiConversationRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteAiConversationResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from mavio.intent.v1 import intent_pb2 as _intent_pb2
from mavio.library.v1 import item_pb2 as _item_pb2
from mavio.plugin.v1 import ai_pb2 as _ai_pb2
from mavio.plugin.v1 import voice_pb2 as _voice_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class AssistantAccess(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ASSISTANT_ACCESS_UNSPECIFIED: _ClassVar[AssistantAccess]
    ASSISTANT_ACCESS_ALL: _ClassVar[AssistantAccess]
    ASSISTANT_ACCESS_ADMINISTRATORS: _ClassVar[AssistantAccess]
    ASSISTANT_ACCESS_SELECTED: _ClassVar[AssistantAccess]

class ControlCommand(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CONTROL_COMMAND_UNSPECIFIED: _ClassVar[ControlCommand]
    CONTROL_COMMAND_PAUSE: _ClassVar[ControlCommand]
    CONTROL_COMMAND_RESUME: _ClassVar[ControlCommand]
    CONTROL_COMMAND_STOP: _ClassVar[ControlCommand]
    CONTROL_COMMAND_SEEK: _ClassVar[ControlCommand]
    CONTROL_COMMAND_SEEK_BY: _ClassVar[ControlCommand]
    CONTROL_COMMAND_SKIP: _ClassVar[ControlCommand]
    CONTROL_COMMAND_NEXT: _ClassVar[ControlCommand]
    CONTROL_COMMAND_PREVIOUS: _ClassVar[ControlCommand]
    CONTROL_COMMAND_SPEED: _ClassVar[ControlCommand]
    CONTROL_COMMAND_VOLUME: _ClassVar[ControlCommand]
    CONTROL_COMMAND_MUTE: _ClassVar[ControlCommand]
    CONTROL_COMMAND_UNMUTE: _ClassVar[ControlCommand]
    CONTROL_COMMAND_FULLSCREEN: _ClassVar[ControlCommand]

class NavigateTarget(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    NAVIGATE_TARGET_UNSPECIFIED: _ClassVar[NavigateTarget]
    NAVIGATE_TARGET_HOME: _ClassVar[NavigateTarget]
    NAVIGATE_TARGET_BACK: _ClassVar[NavigateTarget]
    NAVIGATE_TARGET_ITEM: _ClassVar[NavigateTarget]
    NAVIGATE_TARGET_LIBRARY: _ClassVar[NavigateTarget]
    NAVIGATE_TARGET_SEARCH: _ClassVar[NavigateTarget]
    NAVIGATE_TARGET_SETTINGS: _ClassVar[NavigateTarget]
ASSISTANT_ACCESS_UNSPECIFIED: AssistantAccess
ASSISTANT_ACCESS_ALL: AssistantAccess
ASSISTANT_ACCESS_ADMINISTRATORS: AssistantAccess
ASSISTANT_ACCESS_SELECTED: AssistantAccess
CONTROL_COMMAND_UNSPECIFIED: ControlCommand
CONTROL_COMMAND_PAUSE: ControlCommand
CONTROL_COMMAND_RESUME: ControlCommand
CONTROL_COMMAND_STOP: ControlCommand
CONTROL_COMMAND_SEEK: ControlCommand
CONTROL_COMMAND_SEEK_BY: ControlCommand
CONTROL_COMMAND_SKIP: ControlCommand
CONTROL_COMMAND_NEXT: ControlCommand
CONTROL_COMMAND_PREVIOUS: ControlCommand
CONTROL_COMMAND_SPEED: ControlCommand
CONTROL_COMMAND_VOLUME: ControlCommand
CONTROL_COMMAND_MUTE: ControlCommand
CONTROL_COMMAND_UNMUTE: ControlCommand
CONTROL_COMMAND_FULLSCREEN: ControlCommand
NAVIGATE_TARGET_UNSPECIFIED: NavigateTarget
NAVIGATE_TARGET_HOME: NavigateTarget
NAVIGATE_TARGET_BACK: NavigateTarget
NAVIGATE_TARGET_ITEM: NavigateTarget
NAVIGATE_TARGET_LIBRARY: NavigateTarget
NAVIGATE_TARGET_SEARCH: NavigateTarget
NAVIGATE_TARGET_SETTINGS: NavigateTarget

class AssistantSettings(_message.Message):
    __slots__ = ("enabled", "model", "code_mode", "ai_access", "ai_user_ids", "idle_minutes", "retention_days", "memory", "hints", "speech", "longest_pause", "commit_delay", "session_idle")
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    CODE_MODE_FIELD_NUMBER: _ClassVar[int]
    AI_ACCESS_FIELD_NUMBER: _ClassVar[int]
    AI_USER_IDS_FIELD_NUMBER: _ClassVar[int]
    IDLE_MINUTES_FIELD_NUMBER: _ClassVar[int]
    RETENTION_DAYS_FIELD_NUMBER: _ClassVar[int]
    MEMORY_FIELD_NUMBER: _ClassVar[int]
    HINTS_FIELD_NUMBER: _ClassVar[int]
    SPEECH_FIELD_NUMBER: _ClassVar[int]
    LONGEST_PAUSE_FIELD_NUMBER: _ClassVar[int]
    COMMIT_DELAY_FIELD_NUMBER: _ClassVar[int]
    SESSION_IDLE_FIELD_NUMBER: _ClassVar[int]
    enabled: bool
    model: str
    code_mode: bool
    ai_access: AssistantAccess
    ai_user_ids: _containers.RepeatedScalarFieldContainer[str]
    idle_minutes: int
    retention_days: int
    memory: bool
    hints: bool
    speech: _containers.RepeatedCompositeFieldContainer[AssistantSpeech]
    longest_pause: _duration_pb2.Duration
    commit_delay: _duration_pb2.Duration
    session_idle: _duration_pb2.Duration
    def __init__(self, enabled: _Optional[bool] = ..., model: _Optional[str] = ..., code_mode: _Optional[bool] = ..., ai_access: _Optional[_Union[AssistantAccess, str]] = ..., ai_user_ids: _Optional[_Iterable[str]] = ..., idle_minutes: _Optional[int] = ..., retention_days: _Optional[int] = ..., memory: _Optional[bool] = ..., hints: _Optional[bool] = ..., speech: _Optional[_Iterable[_Union[AssistantSpeech, _Mapping]]] = ..., longest_pause: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., commit_delay: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., session_idle: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class AssistantSpeech(_message.Message):
    __slots__ = ("language", "recognizer", "voice")
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    RECOGNIZER_FIELD_NUMBER: _ClassVar[int]
    VOICE_FIELD_NUMBER: _ClassVar[int]
    language: str
    recognizer: str
    voice: str
    def __init__(self, language: _Optional[str] = ..., recognizer: _Optional[str] = ..., voice: _Optional[str] = ...) -> None: ...

class GetAssistantRequest(_message.Message):
    __slots__ = ("language",)
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    language: str
    def __init__(self, language: _Optional[str] = ...) -> None: ...

class GetAssistantResponse(_message.Message):
    __slots__ = ("enabled", "ai", "memory", "voice", "examples", "languages")
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    AI_FIELD_NUMBER: _ClassVar[int]
    MEMORY_FIELD_NUMBER: _ClassVar[int]
    VOICE_FIELD_NUMBER: _ClassVar[int]
    EXAMPLES_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    enabled: bool
    ai: bool
    memory: bool
    voice: bool
    examples: _containers.RepeatedScalarFieldContainer[str]
    languages: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, enabled: _Optional[bool] = ..., ai: _Optional[bool] = ..., memory: _Optional[bool] = ..., voice: _Optional[bool] = ..., examples: _Optional[_Iterable[str]] = ..., languages: _Optional[_Iterable[str]] = ...) -> None: ...

class VoiceEngine(_message.Message):
    __slots__ = ("id", "name", "languages", "ready")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    READY_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    languages: _containers.RepeatedScalarFieldContainer[str]
    ready: bool
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., languages: _Optional[_Iterable[str]] = ..., ready: _Optional[bool] = ...) -> None: ...

class AskRequest(_message.Message):
    __slots__ = ("text", "intent", "context", "spoken", "answer", "choice", "failed_actions", "run_id")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    INTENT_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    SPOKEN_FIELD_NUMBER: _ClassVar[int]
    ANSWER_FIELD_NUMBER: _ClassVar[int]
    CHOICE_FIELD_NUMBER: _ClassVar[int]
    FAILED_ACTIONS_FIELD_NUMBER: _ClassVar[int]
    RUN_ID_FIELD_NUMBER: _ClassVar[int]
    text: str
    intent: _intent_pb2.Intent
    context: AskContext
    spoken: bool
    answer: ConfirmationAnswer
    choice: str
    failed_actions: _containers.RepeatedScalarFieldContainer[str]
    run_id: str
    def __init__(self, text: _Optional[str] = ..., intent: _Optional[_Union[_intent_pb2.Intent, _Mapping]] = ..., context: _Optional[_Union[AskContext, _Mapping]] = ..., spoken: _Optional[bool] = ..., answer: _Optional[_Union[ConfirmationAnswer, _Mapping]] = ..., choice: _Optional[str] = ..., failed_actions: _Optional[_Iterable[str]] = ..., run_id: _Optional[str] = ...) -> None: ...

class SteerAskRequest(_message.Message):
    __slots__ = ("run_id", "text")
    RUN_ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    run_id: str
    text: str
    def __init__(self, run_id: _Optional[str] = ..., text: _Optional[str] = ...) -> None: ...

class SteerAskResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class AbortAskRequest(_message.Message):
    __slots__ = ("run_id",)
    RUN_ID_FIELD_NUMBER: _ClassVar[int]
    run_id: str
    def __init__(self, run_id: _Optional[str] = ...) -> None: ...

class AbortAskResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class AskContext(_message.Message):
    __slots__ = ("languages", "playing", "screen", "item_id", "library_id", "selected_item_ids", "time_zone")
    LANGUAGES_FIELD_NUMBER: _ClassVar[int]
    PLAYING_FIELD_NUMBER: _ClassVar[int]
    SCREEN_FIELD_NUMBER: _ClassVar[int]
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_ID_FIELD_NUMBER: _ClassVar[int]
    SELECTED_ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    TIME_ZONE_FIELD_NUMBER: _ClassVar[int]
    languages: _containers.RepeatedScalarFieldContainer[str]
    playing: AskPlaying
    screen: str
    item_id: str
    library_id: str
    selected_item_ids: _containers.RepeatedScalarFieldContainer[str]
    time_zone: str
    def __init__(self, languages: _Optional[_Iterable[str]] = ..., playing: _Optional[_Union[AskPlaying, _Mapping]] = ..., screen: _Optional[str] = ..., item_id: _Optional[str] = ..., library_id: _Optional[str] = ..., selected_item_ids: _Optional[_Iterable[str]] = ..., time_zone: _Optional[str] = ...) -> None: ...

class AskPlaying(_message.Message):
    __slots__ = ("item_id", "position", "paused", "speed", "audio_stream_index", "subtitle_stream_index", "volume", "muted")
    ITEM_ID_FIELD_NUMBER: _ClassVar[int]
    POSITION_FIELD_NUMBER: _ClassVar[int]
    PAUSED_FIELD_NUMBER: _ClassVar[int]
    SPEED_FIELD_NUMBER: _ClassVar[int]
    AUDIO_STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    VOLUME_FIELD_NUMBER: _ClassVar[int]
    MUTED_FIELD_NUMBER: _ClassVar[int]
    item_id: str
    position: _duration_pb2.Duration
    paused: bool
    speed: float
    audio_stream_index: int
    subtitle_stream_index: int
    volume: int
    muted: bool
    def __init__(self, item_id: _Optional[str] = ..., position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., paused: _Optional[bool] = ..., speed: _Optional[float] = ..., audio_stream_index: _Optional[int] = ..., subtitle_stream_index: _Optional[int] = ..., volume: _Optional[int] = ..., muted: _Optional[bool] = ...) -> None: ...

class ConfirmationAnswer(_message.Message):
    __slots__ = ("id", "confirmed")
    ID_FIELD_NUMBER: _ClassVar[int]
    CONFIRMED_FIELD_NUMBER: _ClassVar[int]
    id: str
    confirmed: bool
    def __init__(self, id: _Optional[str] = ..., confirmed: _Optional[bool] = ...) -> None: ...

class AskResponse(_message.Message):
    __slots__ = ("understood", "tool", "action", "confirmation", "answer", "done")
    UNDERSTOOD_FIELD_NUMBER: _ClassVar[int]
    TOOL_FIELD_NUMBER: _ClassVar[int]
    ACTION_FIELD_NUMBER: _ClassVar[int]
    CONFIRMATION_FIELD_NUMBER: _ClassVar[int]
    ANSWER_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    understood: Understood
    tool: ToolActivity
    action: ClientAction
    confirmation: Confirmation
    answer: str
    done: Done
    def __init__(self, understood: _Optional[_Union[Understood, _Mapping]] = ..., tool: _Optional[_Union[ToolActivity, _Mapping]] = ..., action: _Optional[_Union[ClientAction, _Mapping]] = ..., confirmation: _Optional[_Union[Confirmation, _Mapping]] = ..., answer: _Optional[str] = ..., done: _Optional[_Union[Done, _Mapping]] = ...) -> None: ...

class Understood(_message.Message):
    __slots__ = ("text", "intent", "ai")
    TEXT_FIELD_NUMBER: _ClassVar[int]
    INTENT_FIELD_NUMBER: _ClassVar[int]
    AI_FIELD_NUMBER: _ClassVar[int]
    text: str
    intent: str
    ai: bool
    def __init__(self, text: _Optional[str] = ..., intent: _Optional[str] = ..., ai: _Optional[bool] = ...) -> None: ...

class ToolActivity(_message.Message):
    __slots__ = ("id", "tool", "summary", "done", "failed")
    ID_FIELD_NUMBER: _ClassVar[int]
    TOOL_FIELD_NUMBER: _ClassVar[int]
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    FAILED_FIELD_NUMBER: _ClassVar[int]
    id: str
    tool: str
    summary: str
    done: bool
    failed: bool
    def __init__(self, id: _Optional[str] = ..., tool: _Optional[str] = ..., summary: _Optional[str] = ..., done: _Optional[bool] = ..., failed: _Optional[bool] = ...) -> None: ...

class ClientAction(_message.Message):
    __slots__ = ("id", "play", "control", "streams", "navigate", "show_items")
    ID_FIELD_NUMBER: _ClassVar[int]
    PLAY_FIELD_NUMBER: _ClassVar[int]
    CONTROL_FIELD_NUMBER: _ClassVar[int]
    STREAMS_FIELD_NUMBER: _ClassVar[int]
    NAVIGATE_FIELD_NUMBER: _ClassVar[int]
    SHOW_ITEMS_FIELD_NUMBER: _ClassVar[int]
    id: str
    play: PlayAction
    control: ControlAction
    streams: StreamsAction
    navigate: NavigateAction
    show_items: ShowItemsAction
    def __init__(self, id: _Optional[str] = ..., play: _Optional[_Union[PlayAction, _Mapping]] = ..., control: _Optional[_Union[ControlAction, _Mapping]] = ..., streams: _Optional[_Union[StreamsAction, _Mapping]] = ..., navigate: _Optional[_Union[NavigateAction, _Mapping]] = ..., show_items: _Optional[_Union[ShowItemsAction, _Mapping]] = ...) -> None: ...

class PlayAction(_message.Message):
    __slots__ = ("item_ids", "start_index", "start_position", "audio_stream_index", "subtitle_stream_index")
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    START_INDEX_FIELD_NUMBER: _ClassVar[int]
    START_POSITION_FIELD_NUMBER: _ClassVar[int]
    AUDIO_STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    start_index: int
    start_position: _duration_pb2.Duration
    audio_stream_index: int
    subtitle_stream_index: int
    def __init__(self, item_ids: _Optional[_Iterable[str]] = ..., start_index: _Optional[int] = ..., start_position: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., audio_stream_index: _Optional[int] = ..., subtitle_stream_index: _Optional[int] = ...) -> None: ...

class ControlAction(_message.Message):
    __slots__ = ("command", "value", "segment")
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    SEGMENT_FIELD_NUMBER: _ClassVar[int]
    command: ControlCommand
    value: float
    segment: str
    def __init__(self, command: _Optional[_Union[ControlCommand, str]] = ..., value: _Optional[float] = ..., segment: _Optional[str] = ...) -> None: ...

class StreamsAction(_message.Message):
    __slots__ = ("audio_stream_index", "subtitle_stream_index", "subtitle_offset")
    AUDIO_STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_STREAM_INDEX_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_OFFSET_FIELD_NUMBER: _ClassVar[int]
    audio_stream_index: int
    subtitle_stream_index: int
    subtitle_offset: float
    def __init__(self, audio_stream_index: _Optional[int] = ..., subtitle_stream_index: _Optional[int] = ..., subtitle_offset: _Optional[float] = ...) -> None: ...

class NavigateAction(_message.Message):
    __slots__ = ("target", "id", "query")
    TARGET_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    target: NavigateTarget
    id: str
    query: str
    def __init__(self, target: _Optional[_Union[NavigateTarget, str]] = ..., id: _Optional[str] = ..., query: _Optional[str] = ...) -> None: ...

class ShowItemsAction(_message.Message):
    __slots__ = ("items", "choose")
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    CHOOSE_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[_item_pb2.Item]
    choose: bool
    def __init__(self, items: _Optional[_Iterable[_Union[_item_pb2.Item, _Mapping]]] = ..., choose: _Optional[bool] = ...) -> None: ...

class Confirmation(_message.Message):
    __slots__ = ("id", "text")
    ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    id: str
    text: str
    def __init__(self, id: _Optional[str] = ..., text: _Optional[str] = ...) -> None: ...

class Done(_message.Message):
    __slots__ = ("answer", "language", "conversation_id", "confirmation", "results", "not_understood", "cancelled")
    ANSWER_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    CONFIRMATION_FIELD_NUMBER: _ClassVar[int]
    RESULTS_FIELD_NUMBER: _ClassVar[int]
    NOT_UNDERSTOOD_FIELD_NUMBER: _ClassVar[int]
    CANCELLED_FIELD_NUMBER: _ClassVar[int]
    answer: str
    language: str
    conversation_id: str
    confirmation: bool
    results: int
    not_understood: bool
    cancelled: bool
    def __init__(self, answer: _Optional[str] = ..., language: _Optional[str] = ..., conversation_id: _Optional[str] = ..., confirmation: _Optional[bool] = ..., results: _Optional[int] = ..., not_understood: _Optional[bool] = ..., cancelled: _Optional[bool] = ...) -> None: ...

class StartVoiceSessionRequest(_message.Message):
    __slots__ = ("language", "mode", "context")
    LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    language: str
    mode: _voice_pb2.VoiceMode
    context: AskContext
    def __init__(self, language: _Optional[str] = ..., mode: _Optional[_Union[_voice_pb2.VoiceMode, str]] = ..., context: _Optional[_Union[AskContext, _Mapping]] = ...) -> None: ...

class StartVoiceSessionResponse(_message.Message):
    __slots__ = ("url", "session_id", "expire_time")
    URL_FIELD_NUMBER: _ClassVar[int]
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    url: str
    session_id: str
    expire_time: _timestamp_pb2.Timestamp
    def __init__(self, url: _Optional[str] = ..., session_id: _Optional[str] = ..., expire_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class AssistantConversation(_message.Message):
    __slots__ = ("id", "title", "device_name", "create_time", "update_time", "usage")
    ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    DEVICE_NAME_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    id: str
    title: str
    device_name: str
    create_time: _timestamp_pb2.Timestamp
    update_time: _timestamp_pb2.Timestamp
    usage: _ai_pb2.AiUsage
    def __init__(self, id: _Optional[str] = ..., title: _Optional[str] = ..., device_name: _Optional[str] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., usage: _Optional[_Union[_ai_pb2.AiUsage, _Mapping]] = ...) -> None: ...

class AssistantMessage(_message.Message):
    __slots__ = ("user", "text", "time", "speech_url")
    USER_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    TIME_FIELD_NUMBER: _ClassVar[int]
    SPEECH_URL_FIELD_NUMBER: _ClassVar[int]
    user: bool
    text: str
    time: _timestamp_pb2.Timestamp
    speech_url: str
    def __init__(self, user: _Optional[bool] = ..., text: _Optional[str] = ..., time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., speech_url: _Optional[str] = ...) -> None: ...

class ListAssistantConversationsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListAssistantConversationsResponse(_message.Message):
    __slots__ = ("conversations",)
    CONVERSATIONS_FIELD_NUMBER: _ClassVar[int]
    conversations: _containers.RepeatedCompositeFieldContainer[AssistantConversation]
    def __init__(self, conversations: _Optional[_Iterable[_Union[AssistantConversation, _Mapping]]] = ...) -> None: ...

class GetAssistantConversationRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class GetAssistantConversationResponse(_message.Message):
    __slots__ = ("conversation", "messages")
    CONVERSATION_FIELD_NUMBER: _ClassVar[int]
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    conversation: AssistantConversation
    messages: _containers.RepeatedCompositeFieldContainer[AssistantMessage]
    def __init__(self, conversation: _Optional[_Union[AssistantConversation, _Mapping]] = ..., messages: _Optional[_Iterable[_Union[AssistantMessage, _Mapping]]] = ...) -> None: ...

class DeleteAssistantConversationRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteAssistantConversationResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class AssistantMemory(_message.Message):
    __slots__ = ("id", "text", "create_time", "update_time", "use_time")
    ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    USE_TIME_FIELD_NUMBER: _ClassVar[int]
    id: str
    text: str
    create_time: _timestamp_pb2.Timestamp
    update_time: _timestamp_pb2.Timestamp
    use_time: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., text: _Optional[str] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., update_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., use_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListAssistantMemoriesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListAssistantMemoriesResponse(_message.Message):
    __slots__ = ("memories",)
    MEMORIES_FIELD_NUMBER: _ClassVar[int]
    memories: _containers.RepeatedCompositeFieldContainer[AssistantMemory]
    def __init__(self, memories: _Optional[_Iterable[_Union[AssistantMemory, _Mapping]]] = ...) -> None: ...

class UpdateAssistantMemoryRequest(_message.Message):
    __slots__ = ("id", "text")
    ID_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    id: str
    text: str
    def __init__(self, id: _Optional[str] = ..., text: _Optional[str] = ...) -> None: ...

class UpdateAssistantMemoryResponse(_message.Message):
    __slots__ = ("memory",)
    MEMORY_FIELD_NUMBER: _ClassVar[int]
    memory: AssistantMemory
    def __init__(self, memory: _Optional[_Union[AssistantMemory, _Mapping]] = ...) -> None: ...

class DeleteAssistantMemoriesRequest(_message.Message):
    __slots__ = ("ids",)
    IDS_FIELD_NUMBER: _ClassVar[int]
    ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, ids: _Optional[_Iterable[str]] = ...) -> None: ...

class DeleteAssistantMemoriesResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetAssistantSettingsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetAssistantSettingsResponse(_message.Message):
    __slots__ = ("settings", "recognizers", "voices")
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    RECOGNIZERS_FIELD_NUMBER: _ClassVar[int]
    VOICES_FIELD_NUMBER: _ClassVar[int]
    settings: AssistantSettings
    recognizers: _containers.RepeatedCompositeFieldContainer[VoiceEngine]
    voices: _containers.RepeatedCompositeFieldContainer[VoiceEngine]
    def __init__(self, settings: _Optional[_Union[AssistantSettings, _Mapping]] = ..., recognizers: _Optional[_Iterable[_Union[VoiceEngine, _Mapping]]] = ..., voices: _Optional[_Iterable[_Union[VoiceEngine, _Mapping]]] = ...) -> None: ...

class UpdateAssistantSettingsRequest(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: AssistantSettings
    def __init__(self, settings: _Optional[_Union[AssistantSettings, _Mapping]] = ...) -> None: ...

class UpdateAssistantSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: AssistantSettings
    def __init__(self, settings: _Optional[_Union[AssistantSettings, _Mapping]] = ...) -> None: ...

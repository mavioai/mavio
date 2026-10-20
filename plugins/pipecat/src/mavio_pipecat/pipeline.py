"""A voice session's pipeline on a WebSocket (docs/assistant.md §8.2)."""

from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from typing import Any

from fastapi import WebSocket
from loguru import logger
from pipecat.audio.turn.smart_turn.base_smart_turn import SmartTurnParams
from pipecat.audio.turn.smart_turn.local_smart_turn_v3 import LocalSmartTurnAnalyzerV3
from pipecat.audio.vad.silero import SileroVADAnalyzer
from pipecat.frames.frames import (
    Frame,
    InputAudioRawFrame,
    InputTransportMessageFrame,
    OutputTransportMessageUrgentFrame,
    ProposedUserStartedSpeakingFrame,
    ProposedUserStoppedSpeakingFrame,
)
from pipecat.pipeline.pipeline import Pipeline
from pipecat.pipeline.worker import PipelineParams, PipelineWorker
from pipecat.processors.aggregators.llm_context import LLMContext
from pipecat.processors.aggregators.llm_response_universal import (
    LLMContextAggregatorPair,
    LLMUserAggregatorParams,
)
from pipecat.processors.frame_processor import FrameDirection, FrameProcessor
from pipecat.transports.websocket.fastapi import (
    FastAPIWebsocketParams,
    FastAPIWebsocketTransport,
)
from pipecat.turns.user_start import ExternalUserTurnStartStrategy
from pipecat.turns.user_stop import ExternalUserTurnStopStrategy, TurnAnalyzerUserTurnStopStrategy
from pipecat.turns.user_turn_strategies import UserTurnStrategies
from pipecat.workers.runner import WorkerRunner

from mavio.plugin.v1 import voice_pb2
from mavio_pipecat.ask import Asker
from mavio_pipecat.assistant import COMMIT_DELAY, MavioAssistant
from mavio_pipecat.events import SessionEvents
from mavio_pipecat.serializer import INPUT_SAMPLE_RATE, OUTPUT_SAMPLE_RATE, MavioSerializer

# The defaults of the turn settings (docs/assistant.md §10).
LONGEST_PAUSE = 3.0
SESSION_IDLE = 8.0


@dataclass
class Services:
    """A session's recognizer and voice."""

    stt: FrameProcessor
    tts: FrameProcessor


@dataclass
class Turns:
    """How a session takes turns (docs/assistant.md §8.3)."""

    mode: int = voice_pb2.VOICE_MODE_CONVERSATION
    commit_delay: float = COMMIT_DELAY
    longest_pause: float = LONGEST_PAUSE
    idle: float = SESSION_IDLE
    # The client's AskContext, which its context events replace.
    context: dict[str, Any] = field(default_factory=lambda: {})

    @classmethod
    def of(cls, request: voice_pb2.OpenVoiceSessionRequest, context: dict[str, Any]) -> Turns:
        t = request.turns

        def seconds(name: str, default: float) -> float:
            return (
                t.__getattribute__(name).ToTimedelta().total_seconds()
                if t.HasField(name)
                else default
            )

        return cls(
            mode=request.mode or voice_pb2.VOICE_MODE_CONVERSATION,
            commit_delay=seconds("commit_delay", COMMIT_DELAY),
            longest_pause=seconds("longest_pause", LONGEST_PAUSE),
            idle=seconds("idle", SESSION_IDLE),
            context=context,
        )


class PushToTalk(FrameProcessor):
    """Listens only while the client holds to talk: the microphone's audio
    is silenced between ptt_end and ptt_start, which propose the start and
    the end of the user's turn."""

    def __init__(self, **kwargs: Any):
        super().__init__(**kwargs)
        self._held = False

    async def process_frame(self, frame: Frame, direction: FrameDirection) -> None:
        await super().process_frame(frame, direction)
        if isinstance(frame, InputAudioRawFrame) and not self._held:
            # Silence rather than nothing, so that the VAD hears the turn end.
            frame = InputAudioRawFrame(
                audio=bytes(len(frame.audio)),
                sample_rate=frame.sample_rate,
                num_channels=frame.num_channels,
            )
        elif isinstance(frame, InputTransportMessageFrame):
            match frame.message.get("type"):
                case "ptt_start" if not self._held:
                    self._held = True
                    await self.push_frame(ProposedUserStartedSpeakingFrame())
                case "ptt_end" if self._held:
                    self._held = False
                    await self.push_frame(ProposedUserStoppedSpeakingFrame())
                case _:
                    pass
        await self.push_frame(frame, direction)


def _strategies(turns: Turns) -> UserTurnStrategies:
    if turns.mode == voice_pb2.VOICE_MODE_PUSH_TO_TALK:
        return UserTurnStrategies(
            start=[ExternalUserTurnStartStrategy()], stop=[ExternalUserTurnStopStrategy()]
        )
    # VAD or a transcript starts the user's turn; Smart Turn ends it, or
    # the longest pause does.
    analyzer = LocalSmartTurnAnalyzerV3(params=SmartTurnParams(stop_secs=turns.longest_pause))
    return UserTurnStrategies(stop=[TurnAnalyzerUserTurnStopStrategy(turn_analyzer=analyzer)])


async def run_session(
    websocket: WebSocket,
    ask: Asker,
    services: Services,
    turns: Turns,
    *,
    session_id: str = "",
    on_idle: Callable[[], Awaitable[None]] | None = None,
) -> None:
    """Runs a session on an accepted WebSocket until the client leaves or
    the user stays silent for the idle time after an answer, when on_idle
    is called."""
    transport = FastAPIWebsocketTransport(
        websocket=websocket,
        params=FastAPIWebsocketParams(
            audio_in_enabled=True,
            audio_in_sample_rate=INPUT_SAMPLE_RATE,
            audio_out_enabled=True,
            audio_out_sample_rate=OUTPUT_SAMPLE_RATE,
            add_wav_header=False,
            serializer=MavioSerializer(),
        ),
    )
    dictation = turns.mode == voice_pb2.VOICE_MODE_DICTATION
    user_aggregator, assistant_aggregator = LLMContextAggregatorPair(
        LLMContext(),
        user_params=LLMUserAggregatorParams(
            vad_analyzer=SileroVADAnalyzer(),
            user_turn_strategies=_strategies(turns),
            user_idle_timeout=0 if dictation else turns.idle,
        ),
    )
    processors: list[FrameProcessor] = [transport.input()]
    if turns.mode == voice_pb2.VOICE_MODE_PUSH_TO_TALK:
        processors.append(PushToTalk())
    processors += [services.stt, SessionEvents(), user_aggregator]
    if not dictation:
        processors += [
            MavioAssistant(ask, commit_delay=turns.commit_delay, context=turns.context),
            services.tts,
        ]
    processors += [transport.output(), assistant_aggregator]
    worker = PipelineWorker(
        Pipeline(processors),
        params=PipelineParams(
            audio_in_sample_rate=INPUT_SAMPLE_RATE,
            audio_out_sample_rate=OUTPUT_SAMPLE_RATE,
            enable_metrics=True,
        ),
        # The session speaks its own protocol, not RTVI.
        enable_rtvi=False,
    )
    runner = WorkerRunner(handle_sigint=False)
    await runner.add_workers(worker)

    @transport.event_handler("on_client_connected")
    async def on_client_connected(_transport: object, _client: object) -> None:
        ready: dict[str, Any] = {
            "type": "ready",
            "input_sample_rate": INPUT_SAMPLE_RATE,
            "output_sample_rate": OUTPUT_SAMPLE_RATE,
        }
        if session_id:
            ready["session_id"] = session_id
        await worker.queue_frame(OutputTransportMessageUrgentFrame(message=ready))

    @transport.event_handler("on_client_disconnected")
    async def on_client_disconnected(_transport: object, _client: object) -> None:
        logger.info("voice session: the client left")
        await runner.cancel()

    @user_aggregator.event_handler("on_user_turn_idle")
    async def on_user_turn_idle(_aggregator: object) -> None:
        logger.info("voice session: the user stayed silent")
        if on_idle is not None:
            await on_idle()
        await runner.cancel()

    await runner.run()

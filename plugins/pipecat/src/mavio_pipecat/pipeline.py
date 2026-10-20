"""A voice session's pipeline on a WebSocket."""

import re
from collections.abc import Callable
from dataclasses import dataclass, field

from fastapi import WebSocket
from loguru import logger
from pipecat.audio.vad.silero import SileroVADAnalyzer
from pipecat.frames.frames import Frame, OutputTransportMessageUrgentFrame, TTSUpdateSettingsFrame
from pipecat.pipeline.pipeline import Pipeline
from pipecat.pipeline.worker import PipelineParams, PipelineWorker
from pipecat.processors.aggregators.llm_context import LLMContext
from pipecat.processors.aggregators.llm_response_universal import (
    LLMContextAggregatorPair,
    LLMUserAggregatorParams,
)
from pipecat.processors.frame_processor import FrameProcessor
from pipecat.transcriptions.language import Language
from pipecat.transports.websocket.fastapi import (
    FastAPIWebsocketParams,
    FastAPIWebsocketTransport,
)
from pipecat.workers.runner import WorkerRunner

from mavio_pipecat.ask import Asker
from mavio_pipecat.assistant import MavioAssistant
from mavio_pipecat.events import SessionEvents
from mavio_pipecat.serializer import INPUT_SAMPLE_RATE, OUTPUT_SAMPLE_RATE, MavioSerializer

_HAN = re.compile(r"[㐀-鿿]")


@dataclass
class Speech:
    """The session's speech services: how it recognizes and how it speaks."""

    # "whisper" (faster-whisper here) or "openai" (an OpenAI-compatible
    # service).
    stt: str = "whisper"
    whisper_model: str = "deepdml/faster-whisper-large-v3-turbo-ct2"
    # "kokoro" (here) or "openai".
    tts: str = "kokoro"
    # Kokoro's voice by the language of the turn, "en" or "zh".
    kokoro_voices: dict[str, str] = field(
        default_factory=lambda: {"en": "af_heart", "zh": "zf_xiaobei"}
    )
    openai_base_url: str = "https://api.openai.com/v1"
    openai_api_key: str = ""
    openai_stt_model: str = "gpt-4o-transcribe"
    openai_tts_model: str = "gpt-4o-mini-tts"
    openai_voice: str = "alloy"


def turn_language(text: str) -> str:
    """The prototype's guess of a turn's language, for the voice: Chinese
    when it has Han characters, else English."""
    return "zh" if _HAN.search(text) else "en"


def _stt(speech: Speech) -> FrameProcessor:
    match speech.stt:
        case "whisper":
            from pipecat.services.whisper.stt import WhisperSTTService

            # No language: Whisper detects it in each turn.
            return WhisperSTTService(
                settings=WhisperSTTService.Settings(model=speech.whisper_model, language=None)
            )
        case "openai":
            from pipecat.services.openai.stt import OpenAISTTService

            return OpenAISTTService(
                api_key=speech.openai_api_key,
                base_url=speech.openai_base_url,
                settings=OpenAISTTService.Settings(model=speech.openai_stt_model, language=None),
            )
        case _:
            raise ValueError(f"unknown STT service {speech.stt!r}")


def _tts(speech: Speech) -> tuple[FrameProcessor, Callable[[str], Frame | None]]:
    """The TTS service, and the function giving the settings frame for the
    language of a turn."""
    match speech.tts:
        case "kokoro":
            from pipecat.services.kokoro.tts import KokoroTTSService

            def settings(text: str) -> Frame | None:
                language = turn_language(text)
                return TTSUpdateSettingsFrame(
                    delta=KokoroTTSService.Settings(
                        voice=speech.kokoro_voices[language],
                        language=Language.ZH if language == "zh" else Language.EN,
                    )
                )

            tts = KokoroTTSService(
                settings=KokoroTTSService.Settings(voice=speech.kokoro_voices["en"])
            )
            return tts, settings
        case "openai":
            from pipecat.services.openai.tts import OpenAITTSService

            tts = OpenAITTSService(
                api_key=speech.openai_api_key,
                base_url=speech.openai_base_url,
                settings=OpenAITTSService.Settings(
                    model=speech.openai_tts_model, voice=speech.openai_voice
                ),
            )
            return tts, lambda _text: None
        case _:
            raise ValueError(f"unknown TTS service {speech.tts!r}")


@dataclass
class Services:
    """The services of a session, and the function giving the frame that
    sets the TTS service for the language of a turn."""

    stt: FrameProcessor
    tts: FrameProcessor
    speech_settings: Callable[[str], Frame | None]

    @classmethod
    def of(cls, speech: Speech) -> Services:
        tts, speech_settings = _tts(speech)
        return cls(_stt(speech), tts, speech_settings)


async def run_session(websocket: WebSocket, ask: Asker, services: Services) -> None:
    """Runs a session on an accepted WebSocket until the client leaves."""
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
    context = LLMContext()
    # The default turn strategies: VAD or a transcript starts the user's
    # turn, Smart Turn v3 ends it.
    user_aggregator, assistant_aggregator = LLMContextAggregatorPair(
        context, user_params=LLMUserAggregatorParams(vad_analyzer=SileroVADAnalyzer())
    )
    pipeline = Pipeline(
        [
            transport.input(),
            services.stt,
            SessionEvents(),
            user_aggregator,
            MavioAssistant(ask, speech_settings=services.speech_settings),
            services.tts,
            transport.output(),
            assistant_aggregator,
        ]
    )
    worker = PipelineWorker(
        pipeline,
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
        await worker.queue_frame(OutputTransportMessageUrgentFrame(message={"type": "ready"}))

    @transport.event_handler("on_client_disconnected")
    async def on_client_disconnected(_transport: object, _client: object) -> None:
        logger.info("voice session: the client left")
        await runner.cancel()

    await runner.run()

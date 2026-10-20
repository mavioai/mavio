# faster-whisper has no type stubs.
# pyright: reportMissingTypeStubs=false
"""The recognizer of voice sessions on faster-whisper, called directly
rather than through Pipecat's Whisper service: it loads the model from the
plugin's data folder once for every session, prompts it with the
session's hints and reports the language it detected in each turn."""

import asyncio
from collections.abc import AsyncGenerator, Callable
from typing import TYPE_CHECKING

import numpy as np
from pipecat.frames.frames import ErrorFrame, Frame, TranscriptionFrame
from pipecat.services.stt_service import SegmentedSTTService
from pipecat.transcriptions.language import Language
from pipecat.utils.time import time_now_iso8601

from mavio_pipecat.languages import whisper_code

if TYPE_CHECKING:
    from faster_whisper import WhisperModel

# Segments Whisper deems more likely silence than this are dropped.
_NO_SPEECH = 0.6


def language_of(code: str) -> Language | None:
    try:
        return Language(code)
    except ValueError:
        return None


class WhisperRecognizer(SegmentedSTTService):
    """Transcribes each utterance the VAD delimits with a Whisper model.

    load returns the model, loading it the first time: it blocks, and is
    called in a worker thread. language is the user's, BCP 47, empty for
    Whisper to detect it in each turn; prompt is the phrases it is likely to
    hear.
    """

    def __init__(
        self,
        load: Callable[[], WhisperModel],
        *,
        language: str = "",
        prompt: str | None = None,
        **kwargs: object,
    ):
        super().__init__(**kwargs)  # pyright: ignore[reportArgumentType]
        self._load = load
        self._language = whisper_code(language) if language else None
        self._prompt = prompt

    @property
    def wants_wav_segments(self) -> bool:
        return False

    def can_generate_metrics(self) -> bool:
        return True

    async def run_stt(self, audio: bytes) -> AsyncGenerator[Frame]:
        samples = np.frombuffer(audio, dtype=np.int16).astype(np.float32) / 32768.0

        def transcribe() -> tuple[str, str]:
            model = self._load()
            segments, info = model.transcribe(
                samples,
                language=self._language,
                initial_prompt=self._prompt,
                condition_on_previous_text=False,
            )
            text = " ".join(s.text.strip() for s in segments if s.no_speech_prob < _NO_SPEECH)
            return text.strip(), info.language

        await self.start_processing_metrics()
        try:
            text, detected = await asyncio.to_thread(transcribe)
        except Exception as e:
            yield ErrorFrame(f"Whisper: {e}")
            return
        finally:
            await self.stop_processing_metrics()
        if text:
            yield TranscriptionFrame(text, self._user_id, time_now_iso8601(), language_of(detected))

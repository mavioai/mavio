# faster-whisper has no type stubs.
# pyright: reportMissingTypeStubs=false
"""Subtitle transcription (SPEECH_TO_TEXT, docs/assistant.md §8.9): the
transcribers of the plugin's settings, on faster-whisper called directly
in a worker process with a bounded number of threads, or on an
OpenAI-compatible transcription API answering verbose_json; one video at a
time, so that voice sessions keep their latency."""

import asyncio
import io
import os
import tempfile
import wave
from collections.abc import Callable
from concurrent.futures import ProcessPoolExecutor
from dataclasses import dataclass
from multiprocessing import get_context
from pathlib import Path
from typing import TYPE_CHECKING, Any, cast

import httpx
from connectrpc.code import Code
from connectrpc.errors import ConnectError
from connectrpc.request import RequestContext
from google.protobuf.duration_pb2 import Duration

from mavio.plugin.v1 import speech_pb2
from mavio_pipecat.config import Config, Engine
from mavio_pipecat.languages import tag_of, whisper_code
from mavio_pipecat.models import Models
from mavio_pipecat.services import Catalog

if TYPE_CHECKING:
    from faster_whisper import WhisperModel

# The audio's rate and sample size.
_RATE = 16000
_BYTES = 2
# The audio an OpenAI-compatible service is sent at once, under its 25 MB.
_CHUNK = 10 * 60 * _RATE * _BYTES
# What language detection hears.
_DETECT_SECONDS = 30

# The worker process's models, by path and device.
_loaded: dict[tuple[str, str], WhisperModel] = {}


@dataclass(frozen=True)
class Segment:
    start: float
    end: float
    text: str


def _whisper_model(path: str, device: str) -> WhisperModel:
    from faster_whisper import WhisperModel

    model = _loaded.get((path, device))
    if model is None:
        # Half the CPUs, leaving the rest to voice sessions and the server.
        threads = max(1, (os.cpu_count() or 2) // 2)
        model = WhisperModel(path, device=device, cpu_threads=threads)
        _loaded[(path, device)] = model
    return model


def _samples(pcm_path: str) -> Any:
    import numpy as np

    return np.fromfile(pcm_path, dtype=np.int16).astype(np.float32) / 32768.0


def whisper_transcribe(
    pcm_path: str, path: str, device: str, language: str | None, translate: bool
) -> tuple[str, list[Segment]]:
    """Transcribes 16 kHz PCM in a file, in the worker process."""
    model = _whisper_model(path, device)
    segments, info = model.transcribe(
        _samples(pcm_path),
        language=language,
        task="translate" if translate else "transcribe",
        vad_filter=True,
        word_timestamps=True,
    )
    out = [Segment(s.start, s.end, s.text.strip()) for s in segments]
    return info.language, out


def whisper_detect(pcm_path: str, path: str, device: str) -> str:
    """Detects the language of 16 kHz PCM in a file, in the worker process."""
    model = _whisper_model(path, device)
    language, _probability, _all = model.detect_language(_samples(pcm_path))
    return language


def _duration(seconds: float) -> Duration:
    d = Duration()
    d.FromNanoseconds(round(seconds * 1e9))
    return d


def _wav(pcm: bytes) -> bytes:
    out = io.BytesIO()
    with wave.open(out, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(_BYTES)
        w.setframerate(_RATE)
        w.writeframes(pcm)
    return out.getvalue()


class Transcription:
    """SpeechToTextService over the configured transcribers.

    host reads the audio from the host API; models holds Whisper's
    models; executor makes the worker process, and cloud the HTTP client
    of the cloud services.
    """

    def __init__(
        self,
        host: httpx.AsyncClient,
        models: Models,
        config: Callable[[], Config],
        *,
        executor: Callable[[], ProcessPoolExecutor] | None = None,
        cloud: Callable[[], httpx.AsyncClient] | None = None,
        tmp: Path | None = None,
    ):
        self._host = host
        self._models = models
        self._config = config
        self._catalog = Catalog()
        self._executor_of = executor or (
            lambda: ProcessPoolExecutor(max_workers=1, mp_context=get_context("spawn"))
        )
        self._executor: ProcessPoolExecutor | None = None
        self._cloud = cloud or (lambda: httpx.AsyncClient(timeout=httpx.Timeout(30, read=600)))
        self._tmp = tmp
        # One video at a time.
        self._lock = asyncio.Lock()

    def close(self) -> None:
        if self._executor is not None:
            self._executor.shutdown(cancel_futures=True)

    def _ready(self, engine: Engine) -> bool:
        service = self._catalog.transcription.get(engine.service)
        return service is not None and self._models.ready(
            service.model(service.values(engine.settings))
        )

    async def list_transcribers(
        self, request: speech_pb2.ListTranscribersRequest, ctx: RequestContext[Any, Any]
    ) -> speech_pb2.ListTranscribersResponse:
        services = self._catalog.transcription.values()
        return speech_pb2.ListTranscribersResponse(
            services=[
                speech_pb2.TranscriptionServiceInfo(
                    name=s.name,
                    display_name=s.display_name,
                    settings=[f.proto() for f in s.settings],
                )
                for s in services
            ],
            transcribers=[
                speech_pb2.TranscriberInfo(
                    id=e.id,
                    name=e.name or self._catalog.transcription[e.service].display_name,
                    service=e.service,
                    enabled=e.enabled,
                    ready=await asyncio.to_thread(self._ready, e),
                    translates=True,
                )
                for e in self._config().transcribers
            ],
        )

    def _engine(self, transcriber: str) -> tuple[Engine, dict[str, Any]]:
        for e in self._config().transcribers:
            if e.id == transcriber and e.enabled:
                service = self._catalog.transcription[e.service]
                return e, service.values(e.settings)
        raise ConnectError(Code.NOT_FOUND, f"no transcriber {transcriber!r}")

    async def _audio(self, audio: speech_pb2.SpeechAudio) -> bytes:
        try:
            r = await self._host.get(audio.url, timeout=httpx.Timeout(30, read=None))
            r.raise_for_status()
        except httpx.HTTPError as e:
            raise ConnectError(Code.UNAVAILABLE, f"read the audio: {e}") from e
        pcm = r.content
        if audio.HasField("limit"):
            pcm = pcm[: int(audio.limit.ToTimedelta().total_seconds() * _RATE) * _BYTES]
        return pcm

    async def _in_worker(self, pcm: bytes, fn: Callable[..., Any], *args: Any) -> Any:
        """Runs fn in the worker process with the audio's file, then args."""
        if self._executor is None:
            self._executor = self._executor_of()
        with tempfile.NamedTemporaryFile(dir=self._tmp, suffix=".pcm", delete=False) as f:
            f.write(pcm)
        try:
            loop = asyncio.get_running_loop()
            return await loop.run_in_executor(self._executor, fn, f.name, *args)
        finally:
            await asyncio.to_thread(Path(f.name).unlink, missing_ok=True)

    async def transcribe_speech(
        self, request: speech_pb2.TranscribeSpeechRequest, ctx: RequestContext[Any, Any]
    ) -> speech_pb2.TranscribeSpeechResponse:
        engine, settings = self._engine(request.transcriber)
        language = whisper_code(request.language) if request.language else None
        async with self._lock:
            pcm = await self._audio(request.audio)
            if engine.service == "whisper":
                path = await asyncio.to_thread(self._whisper_path, settings["model"])
                detected, segments = await self._in_worker(
                    pcm,
                    whisper_transcribe,
                    path,
                    settings["device"],
                    language,
                    request.translate,
                )
            else:
                detected, segments = await self._openai(settings, pcm, language, request.translate)
        return speech_pb2.TranscribeSpeechResponse(
            language=tag_of(detected),
            segments=[
                speech_pb2.SpeechSegment(
                    start=_duration(s.start), end=_duration(s.end), text=s.text
                )
                for s in segments
                if s.text
            ],
        )

    async def detect_speech_language(
        self, request: speech_pb2.DetectSpeechLanguageRequest, ctx: RequestContext[Any, Any]
    ) -> speech_pb2.DetectSpeechLanguageResponse:
        engine, settings = self._engine(request.transcriber)
        async with self._lock:
            pcm = (await self._audio(request.audio))[: _DETECT_SECONDS * _RATE * _BYTES]
            if engine.service == "whisper":
                path = await asyncio.to_thread(self._whisper_path, settings["model"])
                detected = await self._in_worker(pcm, whisper_detect, path, settings["device"])
            else:
                detected, _ = await self._openai(settings, pcm, None, False)
        return speech_pb2.DetectSpeechLanguageResponse(language=tag_of(detected))

    def _whisper_path(self, model: str) -> str:
        from faster_whisper.utils import download_model

        try:
            return cast(str, download_model(model, local_files_only=True))
        except Exception as e:
            raise ConnectError(
                Code.FAILED_PRECONDITION, f"the Whisper model {model} is not downloaded"
            ) from e

    async def _openai(
        self, settings: dict[str, Any], pcm: bytes, language: str | None, translate: bool
    ) -> tuple[str, list[Segment]]:
        """Transcribes with an OpenAI-compatible service, ten minutes at a
        time, asking for verbose_json's segments."""
        base = str(settings["base_url"]).rstrip("/")
        url = base + ("/audio/translations" if translate else "/audio/transcriptions")
        headers = (
            {"Authorization": f"Bearer {settings['api_key']}"} if settings.get("api_key") else {}
        )
        detected = ""
        out: list[Segment] = []
        async with self._cloud() as http:
            for offset in range(0, max(len(pcm), 1), _CHUNK):
                data: dict[str, str] = {
                    "model": settings["model"],
                    "response_format": "verbose_json",
                    "timestamp_granularities[]": "segment",
                }
                if language and not translate:
                    data["language"] = language
                try:
                    r = await http.post(
                        url,
                        headers=headers,
                        data=data,
                        files={
                            "file": ("audio.wav", _wav(pcm[offset : offset + _CHUNK]), "audio/wav")
                        },
                    )
                except httpx.HTTPError as e:
                    raise ConnectError(Code.UNAVAILABLE, f"transcription: {e}") from e
                if r.status_code != 200:
                    code = {
                        401: Code.UNAUTHENTICATED,
                        403: Code.PERMISSION_DENIED,
                        429: Code.RESOURCE_EXHAUSTED,
                    }.get(r.status_code, Code.UNAVAILABLE)
                    raise ConnectError(code, f"transcription: {r.status_code} {r.text[:200]}")
                body = cast(dict[str, Any], r.json())
                detected = detected or str(body.get("language", ""))
                start = offset / (_RATE * _BYTES)
                for s in cast(list[dict[str, Any]], body.get("segments") or []):
                    out.append(
                        Segment(
                            start + float(s["start"]),
                            start + float(s["end"]),
                            str(s.get("text", "")).strip(),
                        )
                    )
                if not body.get("segments") and body.get("text"):
                    end = start + len(pcm[offset : offset + _CHUNK]) / (_RATE * _BYTES)
                    out.append(Segment(start, end, str(body["text"]).strip()))
        return detected, out

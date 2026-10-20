"""Subtitle transcription (SPEECH_TO_TEXT, docs/assistant.md §8.9): the
transcribers of the plugin's settings, on Whisper in its speaches
container or another OpenAI-compatible transcription API, answering
verbose_json; one video at a time, so that voice sessions keep their
latency."""

import asyncio
import io
import wave
from collections.abc import Callable
from dataclasses import dataclass
from typing import Any, cast

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
from mavio_pipecat.speaches import SpeachesError

# The audio's rate and sample size.
_RATE = 16000
_BYTES = 2
# The audio a service is sent at once, under OpenAI's 25 MB.
_CHUNK = 10 * 60 * _RATE * _BYTES
# What language detection hears.
_DETECT_SECONDS = 30


@dataclass(frozen=True)
class Segment:
    start: float
    end: float
    text: str


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
    models and its container; cloud makes the HTTP client of the services.
    """

    def __init__(
        self,
        host: httpx.AsyncClient,
        models: Models,
        config: Callable[[], Config],
        *,
        cloud: Callable[[], httpx.AsyncClient] | None = None,
    ):
        self._host = host
        self._models = models
        self._config = config
        self._catalog = Catalog()
        self._cloud = cloud or (lambda: httpx.AsyncClient(timeout=httpx.Timeout(30, read=600)))
        # One video at a time.
        self._lock = asyncio.Lock()

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

    async def transcribe_speech(
        self, request: speech_pb2.TranscribeSpeechRequest, ctx: RequestContext[Any, Any]
    ) -> speech_pb2.TranscribeSpeechResponse:
        engine, settings = self._engine(request.transcriber)
        language = whisper_code(request.language) if request.language else None
        async with self._lock:
            pcm = await self._audio(request.audio)
            detected, segments = await self._openai(
                await self._settings(engine, settings), pcm, language, request.translate
            )
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
            detected, _ = await self._openai(
                await self._settings(engine, settings), pcm, None, False
            )
        return speech_pb2.DetectSpeechLanguageResponse(language=tag_of(detected))

    async def _settings(self, engine: Engine, settings: dict[str, Any]) -> dict[str, Any]:
        """The OpenAI-compatible settings of an engine: Whisper's are its
        container's, once its model is downloaded."""
        if engine.service != "whisper":
            return settings
        speaches = self._models.speaches
        try:
            base_url = speaches.base_url
        except SpeachesError as e:
            raise ConnectError(Code.UNAVAILABLE, str(e)) from e
        if not await asyncio.to_thread(speaches.has, settings["model"]):
            raise ConnectError(
                Code.FAILED_PRECONDITION, f"the Whisper model {settings['model']} is not downloaded"
            )
        return {"base_url": base_url, "model": settings["model"], "vad_filter": True}

    async def _openai(
        self, settings: dict[str, Any], pcm: bytes, language: str | None, translate: bool
    ) -> tuple[str, list[Segment]]:
        """Transcribes with an OpenAI-compatible service, ten minutes at a
        time, asking for verbose_json's segments; speaches also takes
        vad_filter, skipping what is not speech."""
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
                if settings.get("vad_filter"):
                    data["vad_filter"] = "true"
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

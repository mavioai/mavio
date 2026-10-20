"""The plugin: voice sessions (VOICE_SESSION), which the server opens and
relays, and subtitle transcription (SPEECH_TO_TEXT), on Pipecat
(docs/plugins.md §8.16).

The server opens a session with OpenVoiceSession and connects to its
WebSocket, under /_mavio/voice/ on the plugin's socket, relaying the
client's; the session asks the assistant through the host API with the
session's token.
"""

import asyncio
import contextlib
import json
import sys
from collections.abc import Callable
from pathlib import Path
from typing import Any, cast

import httpx
from connectrpc.code import Code
from connectrpc.errors import ConnectError
from connectrpc.request import RequestContext
from fastapi import FastAPI, WebSocket
from loguru import logger

from mavio.plugin.v1 import task_pb2, voice_pb2
from mavio.plugin.v1.speech_connect import SpeechToTextServiceASGIApplication
from mavio.plugin.v1.task_connect import TaskRunnerServiceASGIApplication
from mavio.plugin.v1.voice_connect import VoiceSessionServiceASGIApplication
from mavio_pipecat.ask import AskClient, Asker
from mavio_pipecat.config import Config, Engine, parse_config
from mavio_pipecat.models import ModelRef, Models, Progress
from mavio_pipecat.pipeline import Services, Turns, run_session
from mavio_pipecat.services import Catalog, Service, Session
from mavio_pipecat.speaches import Speaches
from mavio_pipecat.transcribe import Transcription
from mavio_plugin import (
    ASGIApp,
    data_dir,
    host_client,
    load_manifest,
    report_task_progress,
    serve,
    start_task,
)

# The prefix of the sessions' WebSockets on the plugin's socket.
VOICE_PREFIX = "/_mavio/voice/"
# The header of the session token on the host API.
VOICE_SESSION_HEADER = "Mavio-Voice-Session"
# The task downloading the models.
DOWNLOAD_TASK = "download-models"


def speaks(languages: list[str], language: str) -> bool:
    """Whether an engine of languages takes a BCP 47 language: any when it
    lists none, else one of the same primary language."""
    if not languages or not language:
        return True
    base = language.split("-")[0].lower()
    return any(lang.split("-")[0].lower() == base for lang in languages)


class Speech:
    """The configured engines, the services they are made from and their
    models."""

    def __init__(self, models: Models, catalog: Catalog | None = None):
        self.models = models
        self.catalog = catalog or Catalog()
        self.config = Config()

    def _service(self, engine: Engine, kind: dict[str, Service]) -> Service:
        return kind[engine.service]

    def engine_models(self, engine: Engine, kind: dict[str, Service]) -> tuple[ModelRef, ...]:
        service = self._service(engine, kind)
        return service.models(service.values(engine.settings))

    def info(self, engine: Engine, kind: dict[str, Service]) -> voice_pb2.VoiceEngineInfo:
        service = self._service(engine, kind)
        values = service.values(engine.settings)
        return voice_pb2.VoiceEngineInfo(
            id=engine.id,
            name=engine.name or service.display_name,
            service=engine.service,
            enabled=engine.enabled,
            languages=service.languages(values),
            hints=service.hints,
            ready=all(self.models.ready(m) for m in service.models(values)),
        )

    def pick(
        self, engines: tuple[Engine, ...], kind: dict[str, Service], id: str, language: str
    ) -> tuple[Engine, Service]:
        """The engine of an ID, else the first enabled and ready one
        speaking the language."""
        for e in engines:
            if not e.enabled or (id and e.id != id):
                continue
            info = self.info(e, kind)
            if info.ready and (id or speaks(list(info.languages), language)):
                return e, self._service(e, kind)
        what = "recognizer" if kind is self.catalog.recognition else "voice"
        raise ConnectError(
            Code.FAILED_PRECONDITION,
            f"no {what} {id!r} is ready" if id else f"no {what} is ready for {language!r}",
        )

    def missing(self) -> list[ModelRef]:
        """The models of the enabled engines not downloaded."""
        c, cat = self.config, self.catalog
        refs = [
            *(
                m
                for e in c.recognizers
                if e.enabled
                for m in self.engine_models(e, cat.recognition)
            ),
            *(m for e in c.voices if e.enabled for m in self.engine_models(e, cat.synthesis)),
            *(
                m
                for e in c.transcribers
                if e.enabled
                for m in self.engine_models(e, cat.transcription)
            ),
        ]
        return [r for r in dict.fromkeys(refs) if not self.models.ready(r)]


class Sessions:
    """The sessions the server opened, and their WebSockets.

    asker makes the client asking the assistant with a session token;
    services makes the recognizer and voice of an opened session, by
    default from the configured engines.
    """

    def __init__(
        self,
        asker: Callable[[str], Asker],
        speech: Speech,
        services: Callable[[voice_pb2.OpenVoiceSessionRequest], Services] | None = None,
    ):
        self._asker = asker
        self.speech = speech
        self._services = services or self.services_of
        self._opened: dict[str, voice_pb2.OpenVoiceSessionRequest] = {}
        self._running: dict[str, asyncio.Task[None]] = {}

    def services_of(self, request: voice_pb2.OpenVoiceSessionRequest) -> Services:
        s, cat = self.speech, self.speech.catalog
        language = request.language
        recognizer, stt = s.pick(
            s.config.recognizers, cat.recognition, request.recognizer, language
        )
        voice, tts = s.pick(s.config.voices, cat.synthesis, request.voice, language)
        session = Session(language=language, hints=tuple(request.hints) if stt.hints else ())
        assert stt.make is not None and tts.make is not None
        return Services(
            stt.make(stt.values(recognizer.settings), session, s.models),
            tts.make(tts.values(voice.settings), session, s.models),
        )

    # VoiceSessionService.

    async def list_voice_services(
        self,
        request: voice_pb2.ListVoiceServicesRequest,
        ctx: RequestContext[Any, Any],
    ) -> voice_pb2.ListVoiceServicesResponse:
        s, cat = self.speech, self.speech.catalog

        def services(
            kind: dict[str, Service], recognizes: bool
        ) -> list[voice_pb2.VoiceServiceInfo]:
            return [
                voice_pb2.VoiceServiceInfo(
                    name=v.name,
                    display_name=v.display_name,
                    recognizes=recognizes,
                    settings=[f.proto() for f in v.settings],
                )
                for v in kind.values()
            ]

        def engines() -> tuple[list[voice_pb2.VoiceEngineInfo], list[voice_pb2.VoiceEngineInfo]]:
            return (
                [s.info(e, cat.recognition) for e in s.config.recognizers],
                [s.info(e, cat.synthesis) for e in s.config.voices],
            )

        recognizers, voices = await asyncio.to_thread(engines)
        return voice_pb2.ListVoiceServicesResponse(
            services=services(cat.recognition, True) + services(cat.synthesis, False),
            recognizers=recognizers,
            voices=voices,
        )

    async def open_voice_session(
        self,
        request: voice_pb2.OpenVoiceSessionRequest,
        ctx: RequestContext[Any, Any],
    ) -> voice_pb2.OpenVoiceSessionResponse:
        if not request.session_id or not request.session_token:
            raise ConnectError(Code.INVALID_ARGUMENT, "a session needs its ID and token")
        self._opened[request.session_id] = request
        return voice_pb2.OpenVoiceSessionResponse(path=VOICE_PREFIX + request.session_id)

    async def close_voice_session(
        self,
        request: voice_pb2.CloseVoiceSessionRequest,
        ctx: RequestContext[Any, Any],
    ) -> voice_pb2.CloseVoiceSessionResponse:
        self._opened.pop(request.session_id, None)
        task = self._running.get(request.session_id)
        if task is not None:
            task.cancel()
        return voice_pb2.CloseVoiceSessionResponse()

    # The sessions' WebSockets.

    async def connect(self, websocket: WebSocket, session_id: str) -> None:
        """Runs an opened session on its WebSocket, once."""
        opened = self._opened.get(session_id)
        if opened is None or session_id in self._running:
            await websocket.close(code=1008)
            return
        try:
            services = await asyncio.to_thread(self._services, opened)
        except ConnectError as e:
            logger.warning(f"voice session {session_id}: {e.message}")
            await websocket.accept()
            await websocket.send_text(
                json.dumps({"type": "error", "code": "unavailable", "message": e.message})
            )
            await websocket.close(code=1011)
            return
        await websocket.accept()
        ask = self._asker(opened.session_token)
        context: dict[str, Any] = {}
        if opened.context_json:
            with contextlib.suppress(json.JSONDecodeError):
                context = json.loads(opened.context_json)
        task = asyncio.current_task()
        assert task is not None
        self._running[session_id] = task
        try:
            await run_session(
                websocket, ask, services, Turns.of(opened, context), session_id=session_id
            )
        except asyncio.CancelledError:
            logger.info(f"voice session {session_id}: closed")
        finally:
            self._running.pop(session_id, None)
            self._opened.pop(session_id, None)
            await ask.aclose()


class Tasks:
    """TaskRunnerService: the models' download."""

    def __init__(self, speech: Speech, host: httpx.AsyncClient | None = None):
        self._speech = speech
        self._host = host

    async def download(self, progress: Progress | None = None) -> list[str]:
        missing = await asyncio.to_thread(self._speech.missing)
        return await self._speech.models.download(missing, progress)

    async def _report(self, progress: float, status: str) -> None:
        if self._host is not None and not await report_task_progress(
            self._host, DOWNLOAD_TASK, progress, status
        ):
            logger.debug("the host took no progress of the download")

    async def run_task(
        self, request: task_pb2.RunTaskRequest, ctx: RequestContext[Any, Any]
    ) -> task_pb2.RunTaskResponse:
        if request.task_id != DOWNLOAD_TASK:
            raise ConnectError(Code.NOT_FOUND, f"no task {request.task_id!r}")
        done = await self.download(self._report)
        # The summary names the models, untranslated; nothing downloaded
        # needs none.
        return task_pb2.RunTaskResponse(message=", ".join(done))


def create_app(sessions: Sessions) -> FastAPI:
    """The sessions' WebSockets, mounted under VOICE_PREFIX."""
    app = FastAPI()

    @app.websocket(VOICE_PREFIX + "{session_id}")
    async def voice(websocket: WebSocket, session_id: str) -> None:
        await sessions.connect(websocket, session_id)

    return app


async def run(manifest_path: Path) -> None:
    root = Path(data_dir() or ".")
    host = host_client()
    speaches = Speaches(host)
    models = Models(root / "models", speaches)
    speech = Speech(models)

    def asker(token: str) -> Asker:
        return AskClient(http=host, headers={VOICE_SESSION_HEADER: token})

    sessions = Sessions(asker, speech)
    tasks = Tasks(speech, host)
    transcription = Transcription(host, models, lambda: speech.config)

    async def configure(config_json: str) -> None:
        try:
            speech.config = parse_config(config_json, speech.catalog)
        except (ValueError, TypeError) as e:
            raise ConnectError(Code.INVALID_ARGUMENT, f"configuration: {e}") from e
        # The server configures the plugin again when Whisper's container
        # starts or stops.
        await speaches.refresh()
        # The models of new engines are downloaded at once, by a run of the
        # task the host queues once the plugin is ready, which shows in its
        # tasks; a run retries what failed.
        if await asyncio.to_thread(speech.missing) and not await start_task(host, DOWNLOAD_TASK):
            logger.warning("the host queued no download of the models")

    try:
        await serve(
            load_manifest(manifest_path),
            [
                VoiceSessionServiceASGIApplication(sessions),
                SpeechToTextServiceASGIApplication(transcription),
                TaskRunnerServiceASGIApplication(tasks),
            ],
            configure=configure,
            mounts={VOICE_PREFIX: cast(ASGIApp, create_app(sessions))},
        )
    finally:
        await host.aclose()


def main() -> None:
    # The host starts the plugin in its folder, beside its manifest.
    manifest = Path(sys.argv[1] if len(sys.argv) > 1 else "manifest.json")
    logger.remove()
    logger.add(sys.stderr, level="INFO")
    asyncio.run(run(manifest))


if __name__ == "__main__":
    main()

"""The plugin: voice sessions on Pipecat (VOICE_SESSION), which the server
opens and relays (docs/plugins.md §8.16).

The server opens a session with OpenVoiceSession and connects to its
WebSocket, under /_mavio/voice/ on the plugin's socket, relaying the
client's; the session asks the assistant through the host API with the
session's token. The prototype configures one recognizer and one voice
from its settings.
"""

import asyncio
import contextlib
import json
import sys
from collections.abc import Callable
from dataclasses import fields
from pathlib import Path
from typing import Any, cast

from connectrpc.code import Code
from connectrpc.errors import ConnectError
from connectrpc.request import RequestContext
from fastapi import FastAPI, WebSocket
from loguru import logger

from mavio.plugin.v1 import voice_pb2
from mavio.plugin.v1.voice_connect import VoiceSessionServiceASGIApplication
from mavio_pipecat.ask import AskClient, Asker
from mavio_pipecat.pipeline import Services, Speech, run_session
from mavio_plugin import ASGIApp, host_client, load_manifest, serve

# The prefix of the sessions' WebSockets on the plugin's socket.
VOICE_PREFIX = "/_mavio/voice/"
# The header of the session token on the host API.
VOICE_SESSION_HEADER = "Mavio-Voice-Session"

_SERVICES = [
    voice_pb2.VoiceServiceInfo(name="whisper", display_name="Whisper", recognizes=True),
    voice_pb2.VoiceServiceInfo(
        name="openai-stt", display_name="OpenAI-compatible transcription", recognizes=True
    ),
    voice_pb2.VoiceServiceInfo(name="kokoro", display_name="Kokoro"),
    voice_pb2.VoiceServiceInfo(name="openai-tts", display_name="OpenAI-compatible speech"),
]
# The services of the settings' stt and tts.
_RECOGNIZERS = {"whisper": "whisper", "openai": "openai-stt"}
_VOICES = {"kokoro": "kokoro", "openai": "openai-tts"}


def speech_of(config_json: str) -> Speech:
    """The prototype's settings: Speech's fields, by name."""
    config = cast(dict[str, Any], json.loads(config_json or "{}"))
    names = {f.name for f in fields(Speech) if f.type is str}
    given = {k: v for k, v in config.items() if k in names and isinstance(v, str)}
    speech = Speech(**cast(dict[str, Any], given))
    if speech.stt not in _RECOGNIZERS or speech.tts not in _VOICES:
        raise ValueError(f"unknown services {speech.stt!r}, {speech.tts!r}")
    return speech


class Sessions:
    """The sessions the server opened, and their WebSockets.

    asker makes the client asking the assistant with a session token;
    services makes the services of each session.
    """

    def __init__(
        self,
        asker: Callable[[str], Asker],
        services: Callable[[], Services],
    ):
        self._asker = asker
        self._services = services
        self.speech = Speech()
        self._opened: dict[str, voice_pb2.OpenVoiceSessionRequest] = {}
        self._running: dict[str, asyncio.Task[None]] = {}

    # VoiceSessionService.

    async def list_voice_services(
        self,
        request: voice_pb2.ListVoiceServicesRequest,
        ctx: RequestContext[
            voice_pb2.ListVoiceServicesRequest, voice_pb2.ListVoiceServicesResponse
        ],
    ) -> voice_pb2.ListVoiceServicesResponse:
        s = self.speech
        return voice_pb2.ListVoiceServicesResponse(
            services=_SERVICES,
            recognizers=[
                voice_pb2.VoiceEngineInfo(
                    id=s.stt, name=s.stt, service=_RECOGNIZERS[s.stt], enabled=True, ready=True
                )
            ],
            voices=[
                voice_pb2.VoiceEngineInfo(
                    id=s.tts, name=s.tts, service=_VOICES[s.tts], enabled=True, ready=True
                )
            ],
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
        ctx: RequestContext[
            voice_pb2.CloseVoiceSessionRequest, voice_pb2.CloseVoiceSessionResponse
        ],
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
        await websocket.accept()
        ask = self._asker(opened.session_token)
        context: dict[str, Any] | None = None
        if opened.context_json:
            with contextlib.suppress(json.JSONDecodeError):
                context = json.loads(opened.context_json)
        commit_delay = (
            opened.turns.commit_delay.ToTimedelta().total_seconds()
            if opened.turns.HasField("commit_delay")
            else None
        )
        task = asyncio.current_task()
        assert task is not None
        self._running[session_id] = task
        try:
            kwargs: dict[str, Any] = {"session_id": session_id, "context": context}
            if commit_delay is not None:
                kwargs["commit_delay"] = commit_delay
            await run_session(websocket, ask, self._services(), **kwargs)
        except asyncio.CancelledError:
            logger.info(f"voice session {session_id}: closed")
        finally:
            self._running.pop(session_id, None)
            self._opened.pop(session_id, None)
            await ask.aclose()


def create_app(sessions: Sessions) -> FastAPI:
    """The sessions' WebSockets, mounted under VOICE_PREFIX."""
    app = FastAPI()

    @app.websocket(VOICE_PREFIX + "{session_id}")
    async def voice(websocket: WebSocket, session_id: str) -> None:
        await sessions.connect(websocket, session_id)

    return app


async def run(manifest_path: Path) -> None:
    host = host_client()

    def asker(token: str) -> Asker:
        return AskClient(http=host, headers={VOICE_SESSION_HEADER: token})

    sessions = Sessions(asker, lambda: Services.of(sessions.speech))

    async def configure(config_json: str) -> None:
        try:
            sessions.speech = speech_of(config_json)
        except (ValueError, TypeError) as e:
            raise ConnectError(Code.INVALID_ARGUMENT, f"configuration: {e}") from e

    try:
        await serve(
            load_manifest(manifest_path),
            [VoiceSessionServiceASGIApplication(sessions)],
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

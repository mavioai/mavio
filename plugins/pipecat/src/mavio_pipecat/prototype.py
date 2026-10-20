"""The prototype's server: a test page and the session's WebSocket, run
on their own, without the plugin host or the server's relay.

    uv run mavio-pipecat-prototype --server http://localhost:8688

The user's access token comes from MAVIO_TOKEN; an OpenAI-compatible
service's key from OPENAI_API_KEY.
"""

import argparse
import os
from collections.abc import Callable
from contextlib import asynccontextmanager
from importlib.resources import files

import uvicorn
from fastapi import FastAPI, WebSocket
from fastapi.responses import HTMLResponse, Response

from mavio_pipecat.ask import AskClient, Asker
from mavio_pipecat.pipeline import Services, Speech, run_session

_STATIC = files("mavio_pipecat") / "static"


def create_app(ask: Asker, services: Callable[[], Services]) -> FastAPI:
    """The app; services makes the services of each session."""

    @asynccontextmanager
    async def lifespan(_app: FastAPI):
        yield
        await ask.aclose()

    app = FastAPI(lifespan=lifespan)

    @app.get("/")
    async def page() -> HTMLResponse:
        return HTMLResponse((_STATIC / "index.html").read_text())

    @app.get("/capture.js")
    async def capture() -> Response:
        return Response((_STATIC / "capture.js").read_text(), media_type="text/javascript")

    @app.websocket("/voice")
    async def voice(websocket: WebSocket) -> None:
        await websocket.accept()
        await run_session(websocket, ask, services())

    return app


def main() -> None:
    parser = argparse.ArgumentParser(description="The voice session prototype.")
    parser.add_argument("--host", default="localhost")
    parser.add_argument("--port", type=int, default=8690)
    parser.add_argument("--server", default="http://localhost:8688", help="the Mavio server")
    parser.add_argument("--stt", choices=["whisper", "openai"], default="whisper")
    parser.add_argument("--tts", choices=["kokoro", "openai"], default="kokoro")
    parser.add_argument("--whisper-model", default=Speech.whisper_model)
    parser.add_argument("--openai-base-url", default=Speech.openai_base_url)
    parser.add_argument("--openai-stt-model", default=Speech.openai_stt_model)
    parser.add_argument("--openai-tts-model", default=Speech.openai_tts_model)
    parser.add_argument("--openai-voice", default=Speech.openai_voice)
    args = parser.parse_args()
    token = os.environ.get("MAVIO_TOKEN", "")
    if not token:
        parser.error("set MAVIO_TOKEN to a user's access token")
    speech = Speech(
        stt=args.stt,
        whisper_model=args.whisper_model,
        tts=args.tts,
        openai_base_url=args.openai_base_url,
        openai_api_key=os.environ.get("OPENAI_API_KEY", ""),
        openai_stt_model=args.openai_stt_model,
        openai_tts_model=args.openai_tts_model,
        openai_voice=args.openai_voice,
    )
    app = create_app(AskClient(args.server, token=token), lambda: Services.of(speech))
    uvicorn.run(app, host=args.host, port=args.port)


if __name__ == "__main__":
    main()

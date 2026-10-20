import json
from collections.abc import AsyncGenerator, AsyncIterator
from typing import Any

from fastapi.testclient import TestClient
from pipecat.frames.frames import Frame, TTSAudioRawFrame
from pipecat.processors.filters.identity_filter import IdentityFilter
from pipecat.services.tts_service import TTSService

from mavio_pipecat.pipeline import Services
from mavio_pipecat.prototype import create_app


class FakeAsk:
    def __init__(self):
        self.requests: list[dict[str, Any]] = []

    async def ask(self, request: dict[str, Any]) -> AsyncIterator[dict[str, Any]]:
        self.requests.append(request)
        yield {"understood": {"text": request["text"], "intent": "greet"}}
        yield {"done": {"answer": "Hello.", "language": "en"}}

    async def aclose(self):
        pass


class FakeTTS(TTSService):
    """Speaks 100 ms of silence for each text."""

    def __init__(self):
        super().__init__(sample_rate=24000)

    async def run_tts(self, text: str, context_id: str) -> AsyncGenerator[Frame | None]:
        yield TTSAudioRawFrame(b"\x00\x00" * 2400, self.sample_rate, 1, context_id=context_id)


def test_typed_request_is_answered_with_speech():
    ask = FakeAsk()
    app = create_app(ask, lambda: Services(IdentityFilter(), FakeTTS(), lambda _text: None))
    with TestClient(app) as client, client.websocket_connect("/voice") as ws:
        assert json.loads(ws.receive_text()) == {"type": "ready"}
        ws.send_text(json.dumps({"type": "context", "context": {"screen": "home"}}))
        ws.send_text(json.dumps({"type": "text", "text": "hi"}))
        events: list[str] = []
        audio = 0
        while audio == 0 or "bot_stopped" not in events:
            message = ws.receive()
            if message.get("bytes") is not None:
                audio += len(message["bytes"])
            else:
                events.append(json.loads(message["text"])["type"])
            assert len(events) < 50, f"got = {events}, want an end"
        for want in ["run", "intent", "answer", "end", "bot_started"]:
            assert want in events, f"got = {events}, want {want}"
    assert ask.requests == [{"text": "hi", "spoken": True, "context": {"screen": "home"}}]

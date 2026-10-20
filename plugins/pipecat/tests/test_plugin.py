import json
from typing import Any, cast

import pytest
from fastapi.testclient import TestClient
from google.protobuf.duration_pb2 import Duration
from pipecat.processors.filters.identity_filter import IdentityFilter
from starlette.websockets import WebSocketDisconnect
from test_session import FakeAsk, FakeTTS

from mavio.plugin.v1 import voice_pb2
from mavio_pipecat.ask import Asker
from mavio_pipecat.pipeline import Services
from mavio_pipecat.plugin import VOICE_PREFIX, Sessions, create_app, speech_of

_CTX = cast(Any, None)


def test_speech_of():
    speech = speech_of('{"stt": "openai", "openai_api_key": "k", "kokoro_voices": "x"}')
    assert (speech.stt, speech.tts, speech.openai_api_key) == ("openai", "kokoro", "k")
    assert speech.kokoro_voices["en"] == "af_heart"
    with pytest.raises(ValueError):
        speech_of('{"tts": "piper"}')


def _sessions() -> tuple[Sessions, dict[str, FakeAsk]]:
    asks: dict[str, FakeAsk] = {}

    def asker(token: str) -> Asker:
        asks[token] = FakeAsk()
        return asks[token]

    sessions = Sessions(asker, lambda: Services(IdentityFilter(), FakeTTS(), lambda _text: None))
    return sessions, asks


async def _open(sessions: Sessions, session_id: str) -> str:
    response = await sessions.open_voice_session(
        voice_pb2.OpenVoiceSessionRequest(
            session_id=session_id,
            session_token="token-" + session_id,
            context_json='{"screen": "home"}',
            turns=voice_pb2.VoiceTurnSettings(commit_delay=Duration(nanos=100_000_000)),
        ),
        _CTX,
    )
    return response.path


async def test_session_asks_with_its_token():
    sessions, asks = _sessions()
    path = await _open(sessions, "session1")
    assert path == VOICE_PREFIX + "session1"
    with TestClient(create_app(sessions)) as client, client.websocket_connect(path) as ws:
        ready = json.loads(ws.receive_text())
        assert ready["type"] == "ready" and ready["session_id"] == "session1"
        ws.send_text(json.dumps({"type": "text", "text": "hi"}))
        while json.loads(ws.receive_text())["type"] != "end":
            pass
    # The session's context is the server's until the client sends one.
    assert asks["token-session1"].requests == [
        {"text": "hi", "spoken": True, "context": {"screen": "home"}}
    ]


async def test_unopened_session_is_refused():
    sessions, _ = _sessions()
    with (
        TestClient(create_app(sessions)) as client,
        pytest.raises(WebSocketDisconnect),
        client.websocket_connect(VOICE_PREFIX + "nobody") as ws,
    ):
        ws.receive_text()


async def test_closed_session_is_refused():
    sessions, _ = _sessions()
    path = await _open(sessions, "session2")
    await sessions.close_voice_session(
        voice_pb2.CloseVoiceSessionRequest(session_id="session2"), _CTX
    )
    with (
        TestClient(create_app(sessions)) as client,
        pytest.raises(WebSocketDisconnect),
        client.websocket_connect(path) as ws,
    ):
        ws.receive_text()


async def test_lists_the_configured_services():
    sessions, _ = _sessions()
    sessions.speech = speech_of('{"stt": "openai", "tts": "kokoro"}')
    response = await sessions.list_voice_services(voice_pb2.ListVoiceServicesRequest(), _CTX)
    assert [r.service for r in response.recognizers] == ["openai-stt"]
    assert [v.service for v in response.voices] == ["kokoro"]
    names = {s.name for s in response.services}
    assert {r.service for r in response.recognizers} <= names

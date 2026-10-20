import json
from pathlib import Path
from typing import Any, cast

import pytest
from connectrpc.errors import ConnectError
from fakes import FakeAsk, FakeTTS
from fastapi.testclient import TestClient
from google.protobuf.duration_pb2 import Duration
from pipecat.frames.frames import (
    InputAudioRawFrame,
    InputTransportMessageFrame,
    ProposedUserStartedSpeakingFrame,
    ProposedUserStoppedSpeakingFrame,
)
from pipecat.processors.filters.identity_filter import IdentityFilter
from pipecat.tests.utils import run_test
from starlette.websockets import WebSocketDisconnect

from mavio.plugin.v1 import task_pb2, voice_pb2
from mavio_pipecat.ask import Asker
from mavio_pipecat.config import parse_config
from mavio_pipecat.models import Models
from mavio_pipecat.pipeline import PushToTalk, Services, Turns
from mavio_pipecat.plugin import VOICE_PREFIX, Sessions, Speech, Tasks, create_app, speaks

_CTX = cast(Any, None)

_CONFIG = json.dumps(
    {
        "recognizers": [
            {"id": "r-off", "service": "openai", "name": "Off", "enabled": False},
            {"id": "r1", "service": "openai", "name": "OpenAI", "settings": {"api_key": "k"}},
            {"id": "r2", "service": "whisper", "name": "Whisper"},
        ],
        "voices": [
            {
                "id": "v-zh",
                "service": "kokoro",
                "name": "小北",
                "settings": {"voice": "zf_xiaobei"},
            },
            {"id": "v-en", "service": "openai", "name": "Alloy"},
        ],
    }
)


def _speech(tmp_path: Path) -> Speech:
    speech = Speech(Models(tmp_path))
    speech.config = parse_config(_CONFIG)
    return speech


def _sessions(tmp_path: Path) -> tuple[Sessions, dict[str, FakeAsk]]:
    asks: dict[str, FakeAsk] = {}

    def asker(token: str) -> Asker:
        asks[token] = FakeAsk()
        return asks[token]

    sessions = Sessions(
        asker, _speech(tmp_path), services=lambda _request: Services(IdentityFilter(), FakeTTS())
    )
    return sessions, asks


async def _open(sessions: Sessions, session_id: str, **kwargs: Any) -> str:
    response = await sessions.open_voice_session(
        voice_pb2.OpenVoiceSessionRequest(
            session_id=session_id,
            session_token="token-" + session_id,
            context_json='{"screen": "home"}',
            turns=voice_pb2.VoiceTurnSettings(commit_delay=Duration(nanos=100_000_000)),
            **kwargs,
        ),
        _CTX,
    )
    return response.path


def test_speaks():
    cases: list[tuple[list[str], str, bool]] = [
        ([], "zh", True),
        (["zh"], "", True),
        (["zh"], "zh-Hans-CN", True),
        (["en-US"], "en-GB", True),
        (["en-US"], "zh", False),
    ]
    for languages, language, want in cases:
        got = speaks(languages, language)
        assert got == want, f"speaks({languages}, {language!r}) = {got}, want = {want}"


async def test_session_asks_with_its_token(tmp_path: Path):
    sessions, asks = _sessions(tmp_path)
    path = await _open(sessions, "session1")
    assert path == VOICE_PREFIX + "session1"
    with TestClient(create_app(sessions)) as client, client.websocket_connect(path) as ws:
        ready = json.loads(ws.receive_text())
        assert ready["type"] == "ready" and ready["session_id"] == "session1"
        ws.send_text(json.dumps({"type": "text", "text": "hi"}))
        while json.loads(ws.receive_text())["type"] != "answer":
            pass
    # The session's context is the server's until the client sends one.
    assert asks["token-session1"].requests == [
        {"text": "hi", "spoken": True, "context": {"screen": "home"}}
    ]


async def test_dictation_asks_nothing(tmp_path: Path):
    sessions, asks = _sessions(tmp_path)
    path = await _open(sessions, "session3", mode=voice_pb2.VOICE_MODE_DICTATION)
    with TestClient(create_app(sessions)) as client, client.websocket_connect(path) as ws:
        assert json.loads(ws.receive_text())["type"] == "ready"
        ws.send_text(json.dumps({"type": "text", "text": "hi"}))
        ws.send_bytes(bytes(640))
    assert asks["token-session3"].requests == []


async def test_unopened_session_is_refused(tmp_path: Path):
    sessions, _ = _sessions(tmp_path)
    with (
        TestClient(create_app(sessions)) as client,
        pytest.raises(WebSocketDisconnect),
        client.websocket_connect(VOICE_PREFIX + "nobody") as ws,
    ):
        ws.receive_text()


async def test_closed_session_is_refused(tmp_path: Path):
    sessions, _ = _sessions(tmp_path)
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


async def test_session_without_a_ready_voice_fails(tmp_path: Path):
    sessions = Sessions(lambda _token: FakeAsk(), _speech(tmp_path))
    # Kokoro's model is not downloaded, and the only other voice speaks
    # any language: Chinese takes it.
    request = voice_pb2.OpenVoiceSessionRequest(language="zh", voice="v-zh")
    with pytest.raises(ConnectError):
        sessions.services_of(request)
    path = await _open(sessions, "session4", voice="v-zh")
    with TestClient(create_app(sessions)) as client, client.websocket_connect(path) as ws:
        error = json.loads(ws.receive_text())
        assert error["type"] == "error", f"got = {error}"


async def test_lists_the_configured_engines(tmp_path: Path):
    sessions, _ = _sessions(tmp_path)
    response = await sessions.list_voice_services(voice_pb2.ListVoiceServicesRequest(), _CTX)
    got = [(r.id, r.service, r.enabled, r.ready, r.hints) for r in response.recognizers]
    want = [
        ("r-off", "openai", False, True, True),
        ("r1", "openai", True, True, True),
        # Its model is not downloaded.
        ("r2", "whisper", True, False, True),
    ]
    assert got == want, f"recognizers = {got}, want = {want}"
    voices = [(v.id, list(v.languages), v.ready) for v in response.voices]
    assert voices == [("v-zh", ["zh"], False), ("v-en", [], True)], f"voices = {voices}"
    recognizes = {(s.name, s.recognizes) for s in response.services}
    assert ("whisper", True) in recognizes and ("kokoro", False) in recognizes
    openai = next(s for s in response.services if s.name == "openai" and s.recognizes)
    secret = [f.name for f in openai.settings if f.secret]
    assert secret == ["api_key"], f"secret settings = {secret}"


async def test_tasks_download_what_is_missing(tmp_path: Path):
    speech = _speech(tmp_path)
    missing = [str(r) for r in speech.missing()]
    assert missing == ["whisper large-v3-turbo", "kokoro"], f"missing = {missing}"
    with pytest.raises(ConnectError):
        await Tasks(speech).run_task(task_pb2.RunTaskRequest(task_id="nothing"), _CTX)


def test_turns_of_the_request():
    request = voice_pb2.OpenVoiceSessionRequest(
        mode=voice_pb2.VOICE_MODE_PUSH_TO_TALK,
        turns=voice_pb2.VoiceTurnSettings(
            longest_pause=Duration(seconds=2), idle=Duration(seconds=20)
        ),
    )
    t = Turns.of(request, {})
    got = (t.mode, t.commit_delay, t.longest_pause, t.idle)
    want = (voice_pb2.VOICE_MODE_PUSH_TO_TALK, 0.7, 2.0, 20.0)
    assert got == want, f"turns = {got}, want = {want}"
    assert (
        Turns.of(voice_pb2.OpenVoiceSessionRequest(), {}).mode == voice_pb2.VOICE_MODE_CONVERSATION
    )


async def test_push_to_talk_hears_only_while_held():
    audio = InputAudioRawFrame(audio=b"\x01\x02" * 4, sample_rate=16000, num_channels=1)
    down, _ = await run_test(
        PushToTalk(),
        frames_to_send=[
            audio,
            InputTransportMessageFrame(message={"type": "ptt_start"}),
            audio,
            InputTransportMessageFrame(message={"type": "ptt_end"}),
            audio,
        ],
    )
    heard = [f.audio for f in down if isinstance(f, InputAudioRawFrame)]
    assert heard == [bytes(8), audio.audio, bytes(8)], f"heard = {heard}"
    proposed = [
        type(f)
        for f in down
        if isinstance(f, (ProposedUserStartedSpeakingFrame, ProposedUserStoppedSpeakingFrame))
    ]
    assert proposed == [ProposedUserStartedSpeakingFrame, ProposedUserStoppedSpeakingFrame]

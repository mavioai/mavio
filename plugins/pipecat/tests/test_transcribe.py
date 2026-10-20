import json
from pathlib import Path
from typing import Any, cast

import httpx
import pytest
from connectrpc.errors import ConnectError
from fakes import FakeSpeaches

from mavio.plugin.v1 import speech_pb2
from mavio_pipecat.config import parse_config
from mavio_pipecat.models import Models
from mavio_pipecat.transcribe import Transcription

_CTX = cast(Any, None)
_TINY = "Systran/faster-whisper-tiny"
# Two seconds of audio.
_PCM = bytes(2 * 16000 * 2)

_CONFIG = json.dumps(
    {
        "transcribers": [
            {"id": "w", "service": "whisper", "name": "Whisper", "settings": {"model": _TINY}},
            {"id": "o", "service": "openai", "name": "OpenAI", "settings": {"api_key": "k"}},
            {"id": "off", "service": "openai", "name": "Off", "enabled": False},
        ]
    }
)


def _host(requests: list[httpx.Request]) -> httpx.AsyncClient:
    def serve(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        return httpx.Response(200, content=_PCM)

    return httpx.AsyncClient(base_url="http://host", transport=httpx.MockTransport(serve))


def _transcription(
    tmp_path: Path, cloud: httpx.MockTransport | None = None, speaches: FakeSpeaches | None = None
) -> Transcription:
    config = parse_config(_CONFIG)
    return Transcription(
        _host([]),
        Models(tmp_path, speaches),
        lambda: config,
        cloud=(lambda: httpx.AsyncClient(transport=cloud)) if cloud else None,
    )


def _request(transcriber: str, **kwargs: Any) -> speech_pb2.TranscribeSpeechRequest:
    return speech_pb2.TranscribeSpeechRequest(
        transcriber=transcriber, audio=speech_pb2.SpeechAudio(url="/speech/audio/t"), **kwargs
    )


async def test_lists_the_transcribers(tmp_path: Path):
    t = _transcription(tmp_path)
    response = await t.list_transcribers(speech_pb2.ListTranscribersRequest(), _CTX)
    got = [(x.id, x.service, x.enabled, x.ready) for x in response.transcribers]
    want = [
        ("w", "whisper", True, False),
        ("o", "openai", True, True),
        ("off", "openai", False, True),
    ]
    assert got == want, f"transcribers = {got}, want = {want}"
    assert [s.name for s in response.services] == ["whisper", "openai"]


async def test_openai_asks_for_verbose_json(tmp_path: Path):
    asked: list[httpx.Request] = []

    def serve(request: httpx.Request) -> httpx.Response:
        asked.append(request)
        return httpx.Response(
            200,
            json={
                "language": "chinese",
                "segments": [
                    {"start": 0.5, "end": 1.25, "text": " 你好 "},
                    {"start": 1.5, "end": 2, "text": ""},
                ],
            },
        )

    t = _transcription(tmp_path, httpx.MockTransport(serve))
    response = await t.transcribe_speech(_request("o", language="zh-Hans"), _CTX)
    assert response.language == "zh"
    got = [
        (s.start.ToTimedelta().total_seconds(), s.end.ToTimedelta().total_seconds(), s.text)
        for s in response.segments
    ]
    assert got == [(0.5, 1.25, "你好")], f"segments = {got}"
    body = asked[0].content
    assert asked[0].url.path == "/v1/audio/transcriptions"
    assert asked[0].headers["authorization"] == "Bearer k"
    for want in [b"verbose_json", b'name="language"\r\n\r\nzh', b"RIFF"]:
        assert want in body, f"the request lacks {want!r}"


async def test_openai_failures_carry_codes(tmp_path: Path):
    t = _transcription(
        tmp_path, httpx.MockTransport(lambda _r: httpx.Response(401, text="bad key"))
    )
    with pytest.raises(ConnectError) as e:
        await t.transcribe_speech(_request("o", translate=True), _CTX)
    assert e.value.code.name == "UNAUTHENTICATED"
    with pytest.raises(ConnectError) as e:
        await t.transcribe_speech(_request("off"), _CTX)
    assert e.value.code.name == "NOT_FOUND"


async def test_whisper_asks_its_container(tmp_path: Path):
    asked: list[httpx.Request] = []

    def serve(request: httpx.Request) -> httpx.Response:
        asked.append(request)
        language = "japanese" if b'name="language"' not in request.content else "english"
        return httpx.Response(
            200,
            json={"language": language, "segments": [{"start": 0, "end": 1.5, "text": " Hello "}]},
        )

    t = _transcription(tmp_path, httpx.MockTransport(serve), FakeSpeaches(_TINY))
    response = await t.transcribe_speech(_request("w", language="en-US"), _CTX)
    assert (response.language, [s.text for s in response.segments]) == ("en", ["Hello"])
    assert str(asked[0].url) == "http://whisper:8000/v1/audio/transcriptions"
    assert "authorization" not in asked[0].headers
    for want in [_TINY.encode(), b'name="vad_filter"\r\n\r\ntrue', b'name="language"\r\n\r\nen']:
        assert want in asked[0].content, f"the request lacks {want!r}"
    detected = await t.detect_speech_language(
        speech_pb2.DetectSpeechLanguageRequest(
            transcriber="w", audio=speech_pb2.SpeechAudio(url="/speech/audio/t")
        ),
        _CTX,
    )
    assert detected.language == "ja"
    await t.transcribe_speech(_request("w", translate=True), _CTX)
    assert asked[-1].url.path == "/v1/audio/translations"


async def test_whisper_without_its_model_or_container_fails(tmp_path: Path):
    t = _transcription(tmp_path, speaches=FakeSpeaches())
    with pytest.raises(ConnectError) as e:
        await t.transcribe_speech(_request("w"), _CTX)
    assert e.value.code.name == "FAILED_PRECONDITION"
    t = _transcription(tmp_path, speaches=FakeSpeaches(_TINY, address=""))
    with pytest.raises(ConnectError) as e:
        await t.transcribe_speech(_request("w"), _CTX)
    assert e.value.code.name == "UNAVAILABLE"

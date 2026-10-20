import json
import struct
from collections.abc import Callable
from typing import Any

import httpx
import pytest

from mavio_pipecat.ask import AskClient, AskError


def envelope(message: dict[str, Any], flags: int = 0) -> bytes:
    body = json.dumps(message).encode()
    return struct.pack(">BI", flags, len(body)) + body


Handler = Callable[[httpx.Request], httpx.Response]


class Chunks(httpx.AsyncByteStream):
    """A body in pieces of seven bytes, which cut the envelopes."""

    def __init__(self, body: bytes):
        self._body = body

    async def __aiter__(self):
        for i in range(0, len(self._body), 7):
            yield self._body[i : i + 7]


def client(handler: Handler) -> AskClient:
    return AskClient(
        "http://mavio", "token", httpx.AsyncClient(transport=httpx.MockTransport(handler))
    )


async def test_ask_streams_responses():
    requests: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        body = envelope({"answer": "Hel"}) + envelope({"answer": "lo"}) + envelope({}, 2)
        return httpx.Response(200, stream=Chunks(body))

    got = [m async for m in client(handler).ask({"text": "hi", "spoken": True})]
    assert got == [{"answer": "Hel"}, {"answer": "lo"}], f"got = {got}"
    request = requests[0]
    assert request.url.path == "/mavio.assistant.v1.AssistantService/Ask"
    assert request.headers["authorization"] == "Bearer token"
    assert request.headers["content-type"] == "application/connect+json"
    assert json.loads(request.content[5:]) == {"text": "hi", "spoken": True}


async def test_ask_errors():
    cases: dict[str, Handler] = {
        "end stream error": lambda _r: httpx.Response(
            200, content=envelope({"error": {"code": "permission_denied", "message": "no"}}, 2)
        ),
        "status": lambda _r: httpx.Response(
            401, json={"code": "unauthenticated", "message": "token"}
        ),
        "no end": lambda _r: httpx.Response(200, content=envelope({"answer": "a"})),
    }
    want = {
        "end stream error": "permission_denied",
        "status": "unauthenticated",
        "no end": "unavailable",
    }
    for name, handler in cases.items():
        with pytest.raises(AskError) as e:
            async for _ in client(handler).ask({"text": "hi"}):
                pass
        assert e.value.code == want[name], f"{name}: got = {e.value.code}, want = {want[name]}"

"""A client of the server's AssistantService.Ask over Connect's JSON
streaming protocol."""

import json
import struct
from collections.abc import AsyncIterator
from typing import Any, Protocol

import httpx

_ASK_PATH = "/mavio.assistant.v1.AssistantService/Ask"
# Connect's envelope: a flags byte and a big-endian length.
_ENVELOPE = struct.Struct(">BI")
_END_STREAM = 0x02


class Asker(Protocol):
    """What asks the server: AskClient, or a fake in tests."""

    def ask(self, request: dict[str, Any]) -> AsyncIterator[dict[str, Any]]: ...

    async def aclose(self) -> None: ...


class AskError(Exception):
    """Ask failed, with the Connect error's code and message."""

    def __init__(self, code: str, message: str):
        super().__init__(f"{code}: {message}" if message else code)
        self.code = code


class AskClient:
    """Calls Ask: as a user, with the user's access token as a bearer, run
    on its own; or through the host API with an http client of it and the
    session token in Mavio-Voice-Session, as the plugin
    (docs/assistant.md §8.4).
    """

    def __init__(
        self,
        base_url: str = "",
        token: str | None = None,
        http: httpx.AsyncClient | None = None,
        headers: dict[str, str] | None = None,
    ):
        """base_url empty, the http client's base URL is the server's."""
        self._url = base_url.rstrip("/") + _ASK_PATH
        self._headers = dict(headers or {})
        if token is not None:
            self._headers["Authorization"] = f"Bearer {token}"
        # A client given is the caller's to close.
        self._owned = http is None
        self._http = http or httpx.AsyncClient(timeout=httpx.Timeout(10, read=None))

    async def aclose(self) -> None:
        if self._owned:
            await self._http.aclose()

    async def ask(self, request: dict[str, Any]) -> AsyncIterator[dict[str, Any]]:
        """Sends an AskRequest, in its JSON form, and yields the
        AskResponses as they come."""
        body = json.dumps(request).encode()
        headers = {
            "Content-Type": "application/connect+json",
            "Connect-Protocol-Version": "1",
            **self._headers,
        }
        content = _ENVELOPE.pack(0, len(body)) + body
        async with self._http.stream("POST", self._url, content=content, headers=headers) as r:
            if r.status_code != 200:
                raw = await r.aread()
                error: dict[str, Any] = {}
                try:
                    error = json.loads(raw)
                except json.JSONDecodeError:
                    pass
                raise AskError(str(error.get("code", r.status_code)), error.get("message", ""))
            buffer = bytearray()
            async for chunk in r.aiter_bytes():
                buffer += chunk
                while len(buffer) >= _ENVELOPE.size:
                    flags, size = _ENVELOPE.unpack_from(buffer)
                    end = _ENVELOPE.size + size
                    if len(buffer) < end:
                        break
                    message = json.loads(buffer[_ENVELOPE.size : end] or b"{}")
                    del buffer[:end]
                    if flags & _END_STREAM:
                        error = message.get("error")
                        if error:
                            raise AskError(error.get("code", "unknown"), error.get("message", ""))
                        return
                    yield message
        raise AskError("unavailable", "the stream ended without its end")

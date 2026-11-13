import asyncio
import io
import json
import os
import shutil
import tempfile
from collections.abc import AsyncIterator
from pathlib import Path

import httpx
import pytest

from mavio.plugin.v1 import plugin_pb2
from mavio_plugin import ENV_SOCKET, ENV_TOKEN, serve
from mavio_plugin.serve import Receive, Scope, Send, loopback

TOKEN = "secret-token"


async def _echo_path(scope: Scope, receive: Receive, send: Send) -> None:
    """An ASGI application answering with its path and headers."""
    body = json.dumps(
        {
            "path": scope["path"],
            "headers": sorted(k.decode() for k, _ in scope["headers"]),
        }
    ).encode()
    await send({"type": "http.response.start", "status": 200, "headers": []})
    await send({"type": "http.response.body", "body": body})


class _Running:
    def __init__(self, task: asyncio.Task[None], stdout: io.StringIO, client: httpx.AsyncClient):
        self.task = task
        self.stdout = stdout
        self.client = client
        self.configured: list[str] = []


@pytest.fixture
async def plugin(monkeypatch: pytest.MonkeyPatch) -> AsyncIterator[_Running]:
    # Unix socket paths are short: macOS's temporary folder is too deep.
    # On Windows the plugin serves on loopback TCP instead.
    folder = Path(tempfile.mkdtemp(prefix="mp-", dir=None if loopback() else "/tmp"))
    sock = str(folder / "plugin.sock")
    monkeypatch.setenv(ENV_SOCKET, sock)
    monkeypatch.setenv(ENV_TOKEN, TOKEN)
    # As the server sets it for a plugin reaching sites through its gateway.
    monkeypatch.setenv("NO_PROXY", "localhost,::1,10.0.0.0/8,fc00::/7,.local")
    monkeypatch.setenv("HTTPS_PROXY", "http://127.0.0.1:1")
    stdout = io.StringIO()
    configured: list[str] = []

    async def configure(config_json: str) -> None:
        configured.append(config_json)

    manifest = plugin_pb2.Manifest(id="org.example.py", version="1.0.0")
    task = asyncio.create_task(
        serve(
            manifest,
            configure=configure,
            http=_echo_path,
            mounts={"/_mavio/voice/": _echo_path},
            stdout=stdout,
        )
    )
    for _ in range(200):
        if stdout.getvalue():
            break
        await asyncio.sleep(0.01)
    address = json.loads(stdout.getvalue()).get("address")
    if address:
        client = httpx.AsyncClient(base_url=f"http://{address}", trust_env=False)
    else:
        client = httpx.AsyncClient(
            transport=httpx.AsyncHTTPTransport(uds=sock), base_url="http://plugin"
        )
    running = _Running(task, stdout, client)
    running.configured = configured
    try:
        yield running
    finally:
        await client.aclose()
        if not task.done():
            task.cancel()
            with pytest.raises(asyncio.CancelledError):
                await task
        shutil.rmtree(folder, ignore_errors=True)


def _auth(token: str = TOKEN) -> dict[str, str]:
    return {"Authorization": f"Bearer {token}"}


async def test_handshake(plugin: _Running) -> None:
    line = json.loads(plugin.stdout.getvalue())
    if loopback():
        assert line.pop("address").startswith("127.0.0.1:")
    assert line == {"mavio_plugin": 1, "plugin_id": "org.example.py"}


async def test_plugin_service(plugin: _Running) -> None:
    r = await plugin.client.post(
        "/mavio.plugin.v1.PluginService/Describe", json={}, headers=_auth()
    )
    assert r.status_code == 200, r.text
    assert r.json()["manifest"]["id"] == "org.example.py"
    r = await plugin.client.post(
        "/mavio.plugin.v1.PluginService/Configure",
        json={"configJson": '{"a":1}'},
        headers=_auth(),
    )
    assert r.status_code == 200, r.text
    assert plugin.configured == ['{"a":1}']
    r = await plugin.client.post("/mavio.plugin.v1.PluginService/Health", json={}, headers=_auth())
    assert r.json() == {"healthy": True}


async def test_token(plugin: _Running) -> None:
    for headers in ({}, _auth("wrong")):
        r = await plugin.client.post(
            "/mavio.plugin.v1.PluginService/Describe", json={}, headers=headers
        )
        assert r.status_code == 401
    r = await plugin.client.get("/_mavio/voice/abc", headers=_auth("wrong"))
    assert r.status_code == 401
    r = await plugin.client.get("/_mavio/http/x", headers={"mavio-plugin-token": "wrong"})
    assert r.status_code == 401


async def test_routes(plugin: _Running) -> None:
    r = await plugin.client.get("/_mavio/http/a/b", headers={"mavio-plugin-token": TOKEN})
    assert r.status_code == 200
    # The prefix and the token are removed.
    assert r.json()["path"] == "/a/b"
    assert "mavio-plugin-token" not in r.json()["headers"]
    r = await plugin.client.get("/_mavio/voice/abc", headers=_auth())
    assert r.json()["path"] == "/_mavio/voice/abc"
    r = await plugin.client.get("/elsewhere", headers=_auth())
    assert r.status_code == 404


async def test_shutdown(plugin: _Running) -> None:
    r = await plugin.client.post(
        "/mavio.plugin.v1.PluginService/Shutdown", json={}, headers=_auth()
    )
    assert r.status_code == 200
    await asyncio.wait_for(plugin.task, 10)


async def test_no_proxy_without_ipv6_blocks(plugin: _Running) -> None:
    got = os.environ["NO_PROXY"]
    want = "localhost,::1,10.0.0.0/8,.local"
    assert got == want, f"NO_PROXY = {got!r}, want = {want!r}"
    # httpx makes a client from the environment.
    httpx.Client().close()

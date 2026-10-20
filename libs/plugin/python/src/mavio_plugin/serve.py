"""Serving the plugin to the host."""

import asyncio
import contextlib
import hmac
import json
import os
import signal
import socket
import sys
import threading
from collections.abc import Awaitable, Callable, Mapping, MutableMapping, Sequence
from pathlib import Path
from typing import Any, TextIO, cast

from connectrpc.request import RequestContext
from connectrpc.server import ConnectASGIApplication
from google.protobuf import json_format
from hypercorn.asyncio import serve as hypercorn_serve  # pyright: ignore[reportUnknownVariableType]
from hypercorn.config import Config

from mavio.plugin.v1 import plugin_pb2
from mavio.plugin.v1.plugin_connect import PluginServiceASGIApplication

# Environment variables the host sets for the plugin.
ENV_SOCKET = "MAVIO_PLUGIN_SOCKET"
ENV_TOKEN = "MAVIO_PLUGIN_TOKEN"
ENV_HOST_SOCKET = "MAVIO_HOST_SOCKET"
ENV_HOST_ADDRESS = "MAVIO_HOST_ADDRESS"
ENV_DATA = "MAVIO_PLUGIN_DATA"
ENV_HOST_OS = "MAVIO_HOST_OS"
ENV_DEVICE_ID = "MAVIO_DEVICE_ID"
# "stdin" when stdin is a pipe the host holds open while it lives: the
# plugin stops when it ends, however the host ended.
ENV_LIFELINE = "MAVIO_PLUGIN_LIFELINE"

# The handshake protocol version.
PROTOCOL_VERSION = 1
# The base URL of the host API.
HOST_URL = "http://mavio.host"
# The prefix of requests to the plugin's HTTP routes.
HTTP_PREFIX = "/_mavio/http"
_TOKEN_HEADER = b"mavio-plugin-token"

type Scope = MutableMapping[str, Any]
type Message = MutableMapping[str, Any]
type Receive = Callable[[], Awaitable[Message]]
type Send = Callable[[Message], Awaitable[None]]
type ASGIApp = Callable[[Scope, Receive, Send], Awaitable[None]]


def loopback() -> bool:
    """Whether the plugin serves on loopback TCP instead of the socket, as
    on Windows."""
    return sys.platform == "win32"


def data_dir() -> str:
    """The plugin's writable data folder."""
    return os.environ.get(ENV_DATA, "")


def host_os() -> str:
    """The host's operating system and its version, such as "ubuntu 22.04"."""
    return os.environ.get(ENV_HOST_OS, "")


def device_id() -> str:
    """A UUID standing for the host's machine, the same across restarts,
    upgrades and reinstalls; empty outside a Mavio host."""
    return os.environ.get(ENV_DEVICE_ID, "")


def load_manifest(path: str | Path) -> plugin_pb2.Manifest:
    """Reads a manifest.json."""
    return json_format.Parse(Path(path).read_text(), plugin_pb2.Manifest())


class _PluginService:
    """PluginService: Describe returns the manifest, Configure hands the
    configuration to the plugin, Health asks it, Shutdown stops serving."""

    def __init__(
        self,
        manifest: plugin_pb2.Manifest,
        configure: Callable[[str], Awaitable[None]] | None,
        health: Callable[[], Awaitable[str | None]] | None,
        stop: asyncio.Event,
    ):
        self._manifest = manifest
        self._configure = configure
        self._health = health
        self._stop = stop

    async def describe(
        self,
        request: plugin_pb2.DescribeRequest,
        ctx: RequestContext[plugin_pb2.DescribeRequest, plugin_pb2.DescribeResponse],
    ) -> plugin_pb2.DescribeResponse:
        return plugin_pb2.DescribeResponse(manifest=self._manifest)

    async def configure(
        self,
        request: plugin_pb2.ConfigureRequest,
        ctx: RequestContext[plugin_pb2.ConfigureRequest, plugin_pb2.ConfigureResponse],
    ) -> plugin_pb2.ConfigureResponse:
        if self._configure is not None:
            await self._configure(request.config_json)
        return plugin_pb2.ConfigureResponse()

    async def health(
        self,
        request: plugin_pb2.HealthRequest,
        ctx: RequestContext[plugin_pb2.HealthRequest, plugin_pb2.HealthResponse],
    ) -> plugin_pb2.HealthResponse:
        problem = await self._health() if self._health is not None else None
        if problem is None:
            return plugin_pb2.HealthResponse(healthy=True)
        return plugin_pb2.HealthResponse(healthy=False, message=problem)

    async def shutdown(
        self,
        request: plugin_pb2.ShutdownRequest,
        ctx: RequestContext[plugin_pb2.ShutdownRequest, plugin_pb2.ShutdownResponse],
    ) -> plugin_pb2.ShutdownResponse:
        # Hypercorn finishes this response before it stops.
        self._stop.set()
        return plugin_pb2.ShutdownResponse()


def _header(scope: Scope, name: bytes) -> bytes | None:
    for key, value in cast(list[tuple[bytes, bytes]], scope.get("headers", [])):
        if key.lower() == name:
            return value
    return None


def _same(got: bytes | None, want: bytes) -> bool:
    return got is not None and hmac.compare_digest(got, want)


async def _deny(scope: Scope, send: Send, status: int) -> None:
    if scope["type"] == "websocket":
        # Closed before it is accepted, a WebSocket is answered 403.
        await send({"type": "websocket.close", "code": 1008})
        return
    await send({"type": "http.response.start", "status": status, "headers": []})
    await send({"type": "http.response.body", "body": b""})


async def _lifespan(receive: Receive, send: Send) -> None:
    while True:
        message = await receive()
        match message["type"]:
            case "lifespan.startup":
                await send({"type": "lifespan.startup.complete"})
            case "lifespan.shutdown":
                await send({"type": "lifespan.shutdown.complete"})
                return
            case _:
                pass


class Router:
    """The plugin's ASGI application: requests under HTTP_PREFIX, with the
    token in mavio-plugin-token, go to the HTTP routes without the prefix;
    others need the token as a bearer and go to the mounted application of
    their path's prefix, else to the Connect service of their path."""

    def __init__(
        self,
        token: str,
        services: Sequence[ConnectASGIApplication[Any]],
        http: ASGIApp | None = None,
        mounts: Mapping[str, ASGIApp] | None = None,
    ):
        self._token = token.encode()
        self._services = [(s.path + "/", cast(ASGIApp, s)) for s in services]
        self._http = http
        self._mounts = list((mounts or {}).items())

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] == "lifespan":
            await _lifespan(receive, send)
            return
        path: str = scope["path"]
        if path.startswith(HTTP_PREFIX + "/"):
            if self._http is None:
                await _deny(scope, send, 404)
                return
            if not _same(_header(scope, _TOKEN_HEADER), self._token):
                await _deny(scope, send, 401)
                return
            scope = dict(scope)
            scope["path"] = path[len(HTTP_PREFIX) :]
            scope["raw_path"] = scope["path"].encode()
            scope["headers"] = [
                (k, v)
                for k, v in cast(list[tuple[bytes, bytes]], scope["headers"])
                if k.lower() != _TOKEN_HEADER
            ]
            await self._http(scope, receive, send)
            return
        if not _same(_header(scope, b"authorization"), b"Bearer " + self._token):
            await _deny(scope, send, 401)
            return
        for prefix, app in [*self._mounts, *self._services]:
            if path.startswith(prefix):
                await app(scope, receive, send)
                return
        await _deny(scope, send, 404)


def _listen(path: str) -> tuple[socket.socket, str | None]:
    """The plugin's listening socket, and its loopback address when it is
    not the socket at path."""
    if loopback():
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.bind(("127.0.0.1", 0))
        host, port = sock.getsockname()
        address = f"{host}:{port}"
    else:
        with contextlib.suppress(FileNotFoundError):
            os.remove(path)
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.bind(path)
        address = None
    sock.listen(128)
    return sock, address


def _watch_lifeline(stdin: TextIO, stop: asyncio.Event) -> None:
    """Stops the plugin when stdin ends, read in a thread on every
    platform."""
    loop = asyncio.get_running_loop()

    def read() -> None:
        with contextlib.suppress(OSError, ValueError):
            while stdin.buffer.read(4096):
                pass
        loop.call_soon_threadsafe(stop.set)

    threading.Thread(target=read, name="lifeline", daemon=True).start()


async def serve(
    manifest: plugin_pb2.Manifest,
    services: Sequence[ConnectASGIApplication[Any]] = (),
    *,
    configure: Callable[[str], Awaitable[None]] | None = None,
    health: Callable[[], Awaitable[str | None]] | None = None,
    http: ASGIApp | None = None,
    mounts: Mapping[str, ASGIApp] | None = None,
    stdout: TextIO | None = None,
    stdin: TextIO | None = None,
) -> None:
    """Runs the plugin until the host shuts it down.

    services are the plugin's Connect services; PluginService is served
    from the manifest, configure receiving the configuration's JSON and
    health returning a problem, or None when the plugin is healthy. http
    serves the plugin's HTTP routes, and mounts serve the paths starting
    with their keys, such as "/_mavio/voice/".
    """
    path = os.environ.get(ENV_SOCKET, "")
    token = os.environ.get(ENV_TOKEN, "")
    if not path or not token:
        raise RuntimeError(
            f"{ENV_SOCKET} and {ENV_TOKEN} must be set; plugins are started by the Mavio host"
        )
    stop = asyncio.Event()
    plugin = PluginServiceASGIApplication(_PluginService(manifest, configure, health, stop))
    app = Router(token, [plugin, *services], http, mounts)

    sock, address = _listen(path)
    config = Config()
    config.bind = [f"fd://{sock.detach()}"]
    config.accesslog = None
    # Sessions, such as voice sessions' WebSockets, end with the plugin.
    config.graceful_timeout = 5

    loop = asyncio.get_running_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        with contextlib.suppress(NotImplementedError, RuntimeError):
            loop.add_signal_handler(sig, stop.set)
    if os.environ.get(ENV_LIFELINE) == "stdin":
        _watch_lifeline(stdin or sys.stdin, stop)

    handshake: dict[str, Any] = {"mavio_plugin": PROTOCOL_VERSION, "plugin_id": manifest.id}
    if address:
        handshake["address"] = address
    out = stdout or sys.stdout
    out.write(json.dumps(handshake) + "\n")
    out.flush()
    try:
        await hypercorn_serve(cast(Any, app), config, shutdown_trigger=stop.wait)
    finally:
        for sig in (signal.SIGTERM, signal.SIGINT):
            with contextlib.suppress(NotImplementedError, RuntimeError):
                loop.remove_signal_handler(sig)

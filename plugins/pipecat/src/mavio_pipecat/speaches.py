"""Whisper on speaches (docs/plugins.md §8.16): the plugin's container
dependency "whisper", which the server runs on a container engine with a
GPU when it gives one, or an administrator elsewhere. The recognizers and
transcribers of the Whisper service call its OpenAI-compatible API; its
models, in its own volume, are downloaded through its models API."""

from typing import Any, cast
from urllib.parse import quote

import httpx
from google.protobuf import json_format
from loguru import logger

from mavio.plugin.v1 import container_pb2

# The container dependency's name in the manifest, the same as the
# service's, which the settings' x-mavio-requires-container follows.
CONTAINER = "whisper"
_LIST_CONTAINERS = "/mavio.plugin.v1.HostService/ListContainers"


class SpeachesError(Exception):
    """speaches cannot be reached, or failed."""


class Speaches:
    """The address of the whisper container, as the host last told it, and
    its models.

    host is the host API's client; the server configures the plugin again
    when the address changes, and refresh then reads it.
    """

    def __init__(self, host: httpx.AsyncClient | None = None, address: str = ""):
        self._host = host
        self.address = address.rstrip("/")
        self.state = ""

    @property
    def base_url(self) -> str:
        """The base URL of its OpenAI-compatible API."""
        if not self.address:
            raise SpeachesError(f"Whisper's container is not running ({self.state or 'unset'})")
        return self.address + "/v1"

    async def refresh(self) -> None:
        """Reads the container's address from the host."""
        if self._host is None:
            return
        try:
            r = await self._host.post(
                _LIST_CONTAINERS, json={}, headers={"Content-Type": "application/json"}
            )
            r.raise_for_status()
        except httpx.HTTPError as e:
            logger.warning(f"list container dependencies: {e}")
            return
        response = container_pb2.ListContainersResponse()
        json_format.ParseDict(cast(dict[str, Any], r.json()), response, ignore_unknown_fields=True)
        for c in response.containers:
            if c.name == CONTAINER:
                self.address = c.address.rstrip("/")
                state = container_pb2.ContainerState.Name(c.state)
                self.state = state.removeprefix("CONTAINER_STATE_").lower()
                if c.error:
                    self.state += ": " + c.error
                return
        self.address, self.state = "", ""

    def _model_url(self, model: str) -> str:
        return f"{self.base_url}/models/{quote(model, safe='/')}"

    def has(self, model: str) -> bool:
        """Whether a model is downloaded: blocking, for a worker thread."""
        if not self.address:
            return False
        try:
            return httpx.get(self._model_url(model), timeout=5).status_code == 200
        except httpx.HTTPError:
            return False

    async def download(self, model: str) -> None:
        """Downloads a model into the container's volume, which takes as
        long as the model's size."""
        try:
            async with httpx.AsyncClient(timeout=httpx.Timeout(30, read=None)) as http:
                r = await http.post(self._model_url(model))
        except httpx.HTTPError as e:
            raise SpeachesError(f"download {model}: {e}") from e
        if r.status_code not in (200, 201):
            raise SpeachesError(f"download {model}: {r.status_code} {r.text[:200]}")

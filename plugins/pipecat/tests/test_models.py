import json
from pathlib import Path

import httpx
import pytest
from fakes import FakeSpeaches

from mavio_pipecat.models import (
    Models,
    kokoro_model,
    piper_model,
    piper_voice_url,
    whisper_model,
)
from mavio_pipecat.speaches import Speaches, SpeachesError

_TINY = "Systran/faster-whisper-tiny"


def _kokoro(models: Models) -> None:
    files = models.kokoro()
    files.model.parent.mkdir(parents=True)
    files.model.write_bytes(b"x")
    files.voices.write_bytes(b"x")


def test_ready(tmp_path: Path):
    models = Models(tmp_path)
    assert models.ready(None)
    assert not models.ready(kokoro_model())
    assert not models.ready(whisper_model(_TINY)), "ready without its container"
    files = models.kokoro()
    files.model.parent.mkdir(parents=True)
    files.model.write_bytes(b"x")
    assert not models.ready(kokoro_model()), "ready with one of its two files"
    files.voices.write_bytes(b"x")
    assert models.ready(kokoro_model())
    assert Models(tmp_path, FakeSpeaches(_TINY)).ready(whisper_model(_TINY))


def test_piper_voices(tmp_path: Path):
    models = Models(tmp_path)
    voice = "zh_CN-huayan-medium"
    assert not models.ready(piper_model(voice))
    model = models.piper(voice)
    model.parent.mkdir(parents=True)
    model.write_bytes(b"x")
    assert not models.ready(piper_model(voice)), "ready without its configuration"
    model.with_suffix(".onnx.json").write_text("{}")
    assert models.ready(piper_model(voice))
    got = piper_voice_url("en_US-hfc_female-medium")
    want = (
        "https://huggingface.co/rhasspy/piper-voices/resolve/main/"
        "en/en_US/hfc_female/medium/en_US-hfc_female-medium"
    )
    assert got == want, f"URL = {got}, want = {want}"
    with pytest.raises(ValueError):
        piper_voice_url("lessac")


async def test_download_skips_what_is_ready(tmp_path: Path):
    speaches = FakeSpeaches(_TINY)
    models = Models(tmp_path, speaches)
    _kokoro(models)
    assert await models.download([kokoro_model(), kokoro_model(), whisper_model(_TINY)]) == []
    assert speaches.downloaded == []


async def test_download_asks_the_container(tmp_path: Path):
    speaches = FakeSpeaches()
    models = Models(tmp_path, speaches)
    _kokoro(models)
    got = await models.download([whisper_model(_TINY), kokoro_model()])
    assert got == [f"whisper {_TINY}"], f"downloaded = {got}"
    assert speaches.downloaded == [_TINY]


async def test_download_reports_progress(tmp_path: Path):
    models = Models(tmp_path, FakeSpeaches())
    _kokoro(models)
    got: list[float] = []

    async def progress(f: float) -> None:
        got.append(f)

    base = "Systran/faster-whisper-base"
    await models.download([whisper_model(_TINY), kokoro_model(), whisper_model(base)], progress)
    assert got == [0, 0.5], f"progress = {got}, want = [0, 0.5]"


async def test_a_failed_download_leaves_the_others(tmp_path: Path):
    models = Models(tmp_path, FakeSpeaches(fail=True))
    _kokoro(models)
    with pytest.raises(ExceptionGroup) as e:
        await models.download([whisper_model(_TINY), whisper_model("Systran/faster-whisper-base")])
    assert len(e.value.exceptions) == 2


async def test_refresh_reads_the_address():
    asked: list[httpx.Request] = []
    containers: list[dict[str, object]] = [
        {"name": "other", "address": "http://other:1"},
        {
            "name": "speaches",
            "address": "http://127.0.0.1:49153/",
            "state": "CONTAINER_STATE_READY",
        },
    ]

    def serve(request: httpx.Request) -> httpx.Response:
        asked.append(request)
        return httpx.Response(200, json={"containers": containers})

    host = httpx.AsyncClient(base_url="http://host", transport=httpx.MockTransport(serve))
    speaches = Speaches(host)
    with pytest.raises(SpeachesError):
        _ = speaches.base_url
    await speaches.refresh()
    assert asked[0].url.path == "/mavio.plugin.v1.HostService/ListContainers"
    assert json.loads(asked[0].content) == {}
    assert (speaches.base_url, speaches.state) == ("http://127.0.0.1:49153/v1", "ready")
    containers[1] = {"name": "speaches", "state": "CONTAINER_STATE_UNAVAILABLE"}
    await speaches.refresh()
    assert not speaches.address and not speaches.has(_TINY)
    with pytest.raises(SpeachesError, match="unavailable"):
        _ = speaches.base_url

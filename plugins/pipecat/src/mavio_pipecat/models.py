# faster-whisper has no type stubs.
# pyright: reportMissingTypeStubs=false
"""The local models of the bundled services, in the plugin's data folder
(docs/plugins.md §8.16): Whisper's from Hugging Face, in its cache there
(HF_HOME), and Kokoro's from its releases. They are downloaded by the
plugin's download-models task, through the proxy the administrator gives
the plugin, which the environment's HTTPS_PROXY names; an engine whose
model is missing is not ready."""

import asyncio
import threading
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING, cast

import httpx
from loguru import logger

if TYPE_CHECKING:
    from faster_whisper import WhisperModel

_KOKORO_RELEASE = "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/"
_KOKORO_FILES = ("kokoro-v1.0.onnx", "voices-v1.0.bin")


@dataclass(frozen=True)
class ModelRef:
    """A local model: Whisper's of a size, or Kokoro's."""

    kind: str
    name: str = ""

    def __str__(self) -> str:
        return f"{self.kind} {self.name}".strip()


def whisper_model(name: str) -> ModelRef:
    return ModelRef("whisper", name)


def kokoro_model() -> ModelRef:
    return ModelRef("kokoro")


@dataclass(frozen=True)
class KokoroFiles:
    model: Path
    voices: Path


class Models:
    """The models in a folder, and those loaded, shared by every session."""

    def __init__(self, root: Path):
        self.root = root
        self._lock = threading.Lock()
        self._whisper: dict[tuple[str, str], WhisperModel] = {}
        self._downloading: dict[ModelRef, asyncio.Task[None]] = {}

    # Whisper's are in Hugging Face's cache, which HF_HOME places in the
    # folder.

    def _whisper_path(self, name: str, *, download: bool) -> str:
        from faster_whisper.utils import download_model

        return cast(str, download_model(name, local_files_only=not download))

    def ready(self, ref: ModelRef | None) -> bool:
        """Whether a model is downloaded; no model is."""
        if ref is None:
            return True
        match ref.kind:
            case "whisper":
                try:
                    self._whisper_path(ref.name, download=False)
                except Exception:
                    return False
                return True
            case "kokoro":
                files = self.kokoro()
                return files.model.is_file() and files.voices.is_file()
            case _:
                return False

    def kokoro(self) -> KokoroFiles:
        folder = self.root / "kokoro"
        return KokoroFiles(folder / _KOKORO_FILES[0], folder / _KOKORO_FILES[1])

    def whisper(self, name: str, device: str) -> Callable[[], WhisperModel]:
        """A function loading Whisper's model of a size on a device, once:
        blocking, for a worker thread."""

        def load() -> WhisperModel:
            from faster_whisper import WhisperModel

            with self._lock:
                model = self._whisper.get((name, device))
                if model is None:
                    path = self._whisper_path(name, download=False)
                    model = WhisperModel(path, device=device)
                    self._whisper[(name, device)] = model
                return model

        return load

    async def download(self, refs: list[ModelRef]) -> list[str]:
        """Downloads the models missing, one at a time, returning those it
        downloaded; a download already running is waited for."""
        done: list[str] = []
        for ref in dict.fromkeys(refs):
            if self.ready(ref):
                continue
            task = self._downloading.get(ref)
            if task is None:
                task = asyncio.create_task(self._download(ref))
                self._downloading[ref] = task
                task.add_done_callback(lambda _t, ref=ref: self._downloading.pop(ref, None))
            await asyncio.shield(task)
            done.append(str(ref))
        return done

    async def _download(self, ref: ModelRef) -> None:
        logger.info(f"downloading the {ref} model")
        match ref.kind:
            case "whisper":
                await asyncio.to_thread(self._whisper_path, ref.name, download=True)
            case "kokoro":
                for name in _KOKORO_FILES:
                    await _fetch(_KOKORO_RELEASE + name, self.root / "kokoro" / name)
            case _:
                raise ValueError(f"unknown model {ref}")
        logger.info(f"downloaded the {ref} model")


async def _fetch(url: str, dest: Path) -> None:
    """Downloads a file whole or not at all, logging its progress."""
    if await asyncio.to_thread(dest.is_file):
        return
    await asyncio.to_thread(dest.parent.mkdir, parents=True, exist_ok=True)
    part = dest.with_name(dest.name + ".part")
    async with (
        httpx.AsyncClient(follow_redirects=True, timeout=httpx.Timeout(30, read=300)) as http,
        http.stream("GET", url) as r,
    ):
        r.raise_for_status()
        size = int(r.headers.get("content-length", 0))
        got, logged = 0, 0
        with part.open("wb") as f:
            async for chunk in r.aiter_bytes(1 << 20):
                f.write(chunk)
                got += len(chunk)
                if size and got * 10 // size > logged:
                    logged = got * 10 // size
                    logger.info(f"{dest.name}: {logged * 10}% of {size >> 20} MB")
    await asyncio.to_thread(part.rename, dest)

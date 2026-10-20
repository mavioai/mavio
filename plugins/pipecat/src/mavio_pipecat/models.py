"""The local models of the bundled services (docs/plugins.md §8.16):
Whisper's in the volume of its container, speaches, which downloads them
from Hugging Face, and Kokoro's and Piper's in the plugin's data
folder, from their releases. They are downloaded by the plugin's download-models task,
Kokoro's through the proxy the administrator gives the plugin, which the
environment's HTTPS_PROXY names; an engine whose model is missing is not
ready."""

import asyncio
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from pathlib import Path

import httpx
from loguru import logger

from mavio_pipecat.speaches import Speaches

# Progress is told the fraction of a download done, 0 to 1, and what it
# downloads, such as "kokoro · kokoro-v1.0.onnx · 93/310 MB".
type Progress = Callable[[float, str], Awaitable[None]]

_KOKORO_RELEASE = "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/"
_KOKORO_FILES = ("kokoro-v1.0.onnx", "voices-v1.0.bin")
_PIPER_VOICES = "https://huggingface.co/rhasspy/piper-voices/resolve/main/"


@dataclass(frozen=True)
class ModelRef:
    """A local model: Whisper's of an ID, Kokoro's, or Piper's of a voice."""

    kind: str
    name: str = ""

    def __str__(self) -> str:
        return f"{self.kind} {self.name}".strip()


def whisper_model(name: str) -> ModelRef:
    return ModelRef("whisper", name)


def kokoro_model() -> ModelRef:
    return ModelRef("kokoro")


def piper_model(voice: str) -> ModelRef:
    return ModelRef("piper", voice)


def piper_voice_url(voice: str) -> str:
    """The base URL of a Piper voice's files, by its name, such as
    "en_US-lessac-medium": its language, name and quality."""
    parts = voice.split("-")
    if len(parts) < 3 or "_" not in parts[0] or any(not p for p in parts):
        raise ValueError(f"not a Piper voice: {voice!r}")
    lang, name, quality = parts[0], "-".join(parts[1:-1]), parts[-1]
    return f"{_PIPER_VOICES}{lang.split('_')[0]}/{lang}/{name}/{quality}/{voice}"


@dataclass(frozen=True)
class KokoroFiles:
    model: Path
    voices: Path


class Models:
    """The models in a folder and in speaches, shared by every session."""

    def __init__(self, root: Path, speaches: Speaches | None = None):
        self.root = root
        self.speaches = speaches or Speaches()
        self._downloading: dict[ModelRef, asyncio.Task[None]] = {}

    def ready(self, ref: ModelRef | None) -> bool:
        """Whether a model is downloaded, blocking; no model is. One
        downloading is not, though speaches lists a Whisper model as soon as
        its download starts."""
        if ref is None:
            return True
        if ref in self._downloading:
            return False
        match ref.kind:
            case "whisper":
                return self.speaches.has(ref.name)
            case "kokoro":
                files = self.kokoro()
                return files.model.is_file() and files.voices.is_file()
            case "piper":
                model = self.piper(ref.name)
                return model.is_file() and model.with_suffix(".onnx.json").is_file()
            case _:
                return False

    def kokoro(self) -> KokoroFiles:
        folder = self.root / "kokoro"
        return KokoroFiles(folder / _KOKORO_FILES[0], folder / _KOKORO_FILES[1])

    def piper(self, voice: str) -> Path:
        """The model of a Piper voice, its configuration beside it."""
        return self.root / "piper" / (voice + ".onnx")

    async def download(self, refs: list[ModelRef], progress: Progress | None = None) -> list[str]:
        """Downloads the models missing, one at a time, returning those it
        downloaded; a download already running is waited for. One failing
        leaves the others downloaded, and fails after them.

        progress is told the fraction done, 0 to 1, by the models done and
        the bytes of the one downloading where its size is known (Kokoro's
        and Piper's; speaches tells none of a Whisper model), and the model
        and file downloading."""
        done: list[str] = []
        failed: list[Exception] = []
        todo = [ref for ref in dict.fromkeys(refs) if not await asyncio.to_thread(self.ready, ref)]
        report = progress or _ignore
        for i, ref in enumerate(todo):
            await report(i / len(todo), str(ref))

            async def part(f: float, status: str, i: int = i, ref: ModelRef = ref) -> None:
                await report((i + f) / len(todo), f"{ref} · {status}" if status else str(ref))

            task = self._downloading.get(ref)
            if task is None:
                task = asyncio.create_task(self._download(ref, part))
                self._downloading[ref] = task
                task.add_done_callback(lambda _t, ref=ref: self._downloading.pop(ref, None))
            try:
                await asyncio.shield(task)
            except Exception as e:
                failed.append(e)
                continue
            done.append(str(ref))
        if failed:
            raise ExceptionGroup("downloading the models", failed)
        return done

    async def _download(self, ref: ModelRef, progress: Progress) -> None:
        logger.info(f"downloading the {ref} model")
        match ref.kind:
            case "whisper":
                await self.speaches.download(ref.name)
            case "kokoro":
                n = len(_KOKORO_FILES)
                for i, name in enumerate(_KOKORO_FILES):

                    async def part(f: float, status: str, i: int = i) -> None:
                        await progress((i + f) / n, status)

                    await _fetch(_KOKORO_RELEASE + name, self.root / "kokoro" / name, part)
            case "piper":
                url, model = piper_voice_url(ref.name), self.piper(ref.name)
                # The configuration is small; the model, tens of MB, tells
                # the progress.
                await _fetch(url + ".onnx.json", model.with_suffix(".onnx.json"))
                await _fetch(url + ".onnx", model, progress)
            case _:
                raise ValueError(f"unknown model {ref}")
        logger.info(f"downloaded the {ref} model")


async def _ignore(_: float, __: str) -> None:
    pass


async def _fetch(url: str, dest: Path, progress: Progress = _ignore) -> None:
    """Downloads a file whole or not at all, logging and reporting its
    progress by the tenth, with the file's name and the MB downloaded."""
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
                    await progress(logged / 10, f"{dest.name} · {got >> 20}/{size >> 20} MB")
    await asyncio.to_thread(part.rename, dest)

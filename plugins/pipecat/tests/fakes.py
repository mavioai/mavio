"""Fakes of the assistant, of a voice and of Whisper's container."""

import asyncio
from collections.abc import AsyncGenerator
from typing import Any

from pipecat.frames.frames import Frame, TTSAudioRawFrame
from pipecat.services.tts_service import TTSService

from mavio_pipecat.ask import AskError
from mavio_pipecat.speaches import Speaches, SpeachesError


class FakeAsk:
    """Answers each request with the responses of its script, or with a
    greeting when it has none left; a script item that is a float sleeps
    that long."""

    def __init__(self, *scripts: list[Any]):
        self.scripts = list(scripts)
        self.requests: list[dict[str, Any]] = []
        self.steers: list[tuple[str, str]] = []
        self.aborts: list[str] = []
        # The runs in progress, which steer and abort.
        self.running: set[str] = set()

    async def ask(self, request: dict[str, Any]):
        self.requests.append(request)
        script = (
            self.scripts.pop(0)
            if self.scripts
            else [
                {"understood": {"text": request["text"], "intent": "greet"}},
                {"done": {"answer": "Hello.", "language": "en", "conversationId": "c1"}},
            ]
        )
        run_id = request.get("runId", "")
        self.running.add(run_id)
        try:
            for item in script:
                if isinstance(item, float):
                    await asyncio.sleep(item)
                else:
                    yield item
        finally:
            self.running.discard(run_id)

    async def steer(self, run_id: str, text: str):
        if run_id not in self.running:
            raise AskError("failed_precondition", f"run {run_id} has ended")
        self.steers.append((run_id, text))

    async def abort(self, run_id: str):
        self.aborts.append(run_id)

    async def aclose(self):
        pass


class FakeTTS(TTSService):
    """Speaks 100 ms of silence for each text."""

    def __init__(self):
        super().__init__(sample_rate=24000)

    async def run_tts(self, text: str, context_id: str) -> AsyncGenerator[Frame | None]:
        yield TTSAudioRawFrame(b"\x00\x00" * 2400, self.sample_rate, 1, context_id=context_id)


class FakeSpeaches(Speaches):
    """Whisper's container at an address, holding the models given and
    downloading others at once; failing them when fail is set."""

    def __init__(self, *models: str, address: str = "http://whisper:8000", fail: bool = False):
        super().__init__(address=address)
        self.models = set(models)
        self.downloaded: list[str] = []
        self.fail = fail

    def has(self, model: str) -> bool:
        return bool(self.address) and model in self.models

    async def download(self, model: str) -> None:
        if self.fail:
            raise SpeachesError(f"download {model}: 503")
        self.downloaded.append(model)
        self.models.add(model)

"""Fakes of the assistant and of a voice."""

import asyncio
from collections.abc import AsyncGenerator
from typing import Any

from pipecat.frames.frames import Frame, TTSAudioRawFrame
from pipecat.services.tts_service import TTSService


class FakeAsk:
    """Answers each request with the responses of its script, or with a
    greeting when it has none left; a script item that is a float sleeps
    that long."""

    def __init__(self, *scripts: list[Any]):
        self.scripts = list(scripts)
        self.requests: list[dict[str, Any]] = []

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
        for item in script:
            if isinstance(item, float):
                await asyncio.sleep(item)
            else:
                yield item

    async def aclose(self):
        pass


class FakeTTS(TTSService):
    """Speaks 100 ms of silence for each text."""

    def __init__(self):
        super().__init__(sample_rate=24000)

    async def run_tts(self, text: str, context_id: str) -> AsyncGenerator[Frame | None]:
        yield TTSAudioRawFrame(b"\x00\x00" * 2400, self.sample_rate, 1, context_id=context_id)

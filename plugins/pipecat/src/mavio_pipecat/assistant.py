"""The processor that takes the user's turns to the server's Ask, in place
of a Pipecat LLM service."""

import asyncio
import time
from collections.abc import Callable
from typing import Any

import httpx
from loguru import logger
from pipecat.frames.frames import (
    Frame,
    InputTransportMessageFrame,
    InterruptionFrame,
    LLMContextFrame,
    LLMFullResponseEndFrame,
    LLMFullResponseStartFrame,
    LLMTextFrame,
    OutputTransportMessageUrgentFrame,
)
from pipecat.processors.aggregators.llm_context import LLMContext
from pipecat.processors.frame_processor import FrameDirection, FrameProcessor

from mavio_pipecat.ask import Asker, AskError

# How long the effects of an answer, its client actions, wait after the turn
# ended: the user who goes on speaking within it steers the request, and the
# effects are dropped (docs/assistant.md).
COMMIT_DELAY = 0.7


def trailing_user_text(context: LLMContext) -> str:
    """The user's messages since the last answer, joined: a turn that
    follows one not yet answered steers it, and the request is asked again
    with both."""
    texts: list[str] = []
    for message in reversed(context.get_messages()):
        if not isinstance(message, dict):
            continue
        role = message.get("role")
        if role == "assistant":
            break
        if role != "user":
            continue
        content = message.get("content")
        if isinstance(content, list):
            content = " ".join(
                part.get("text", "") for part in content if part.get("type") == "text"
            )
        if isinstance(content, str) and content.strip():
            texts.append(content.strip())
    return " ".join(reversed(texts))


class MavioAssistant(FrameProcessor):
    """Asks the server with each turn of the user and speaks the answer.

    An LLMContextFrame from the user aggregator starts a request with the
    turn; one that comes while a request runs cancels it and asks again with
    both turns (the prototype's steer). An interruption, the user speaking
    over the answer, cancels the request (abort). The answer's deltas go on
    as LLM text for the TTS service, and the request's events go to the
    client as transport messages.
    """

    def __init__(
        self,
        ask: Asker,
        *,
        commit_delay: float = COMMIT_DELAY,
        speech_settings: Callable[[str], Frame | None] | None = None,
        **kwargs: Any,
    ):
        """speech_settings gives the frame that sets the TTS service for
        the language of a turn, such as its voice; None leaves it."""
        super().__init__(**kwargs)
        self._ask = ask
        self._commit_delay = commit_delay
        self._speech_settings = speech_settings
        self._context: dict[str, Any] = {}
        self._run: asyncio.Task[None] | None = None
        self._speaking = False

    async def process_frame(self, frame: Frame, direction: FrameDirection) -> None:
        await super().process_frame(frame, direction)
        if isinstance(frame, LLMContextFrame):
            if frame.speculation:
                return
            text = trailing_user_text(frame.context)
            await self._cancel(end_response=True)
            if text:
                self._run = self.create_task(self._request(text, time.monotonic()), "ask")
            return
        if isinstance(frame, InterruptionFrame):
            await self._cancel(end_response=False)
        elif isinstance(frame, InputTransportMessageFrame):
            await self._client_message(frame.message)
        await self.push_frame(frame, direction)

    async def cleanup(self) -> None:
        await self._cancel(end_response=False)
        await super().cleanup()

    async def _client_message(self, message: dict[str, Any]) -> None:
        match message.get("type"):
            case "context":
                context = message.get("context")
                if isinstance(context, dict):
                    self._context = context
            case "text":
                text = message.get("text")
                if isinstance(text, str) and text.strip():
                    await self._cancel(end_response=True)
                    self._run = self.create_task(self._request(text.strip(), time.monotonic()))
            case _:
                pass

    async def _cancel(self, *, end_response: bool) -> None:
        run, self._run = self._run, None
        if run is not None and not run.done():
            await self.cancel_task(run)
        if self._speaking:
            self._speaking = False
            if end_response:
                await self.push_frame(LLMFullResponseEndFrame())

    async def _event(self, event: dict[str, Any]) -> None:
        await self.push_frame(OutputTransportMessageUrgentFrame(message=event))

    async def _request(self, text: str, turn_end: float) -> None:
        await self._event({"type": "run", "text": text})
        request: dict[str, Any] = {"text": text, "spoken": True}
        if self._context:
            request["context"] = self._context
        if self._speech_settings is not None:
            settings = self._speech_settings(text)
            if settings is not None:
                await self.push_frame(settings)
        answered = False
        try:
            async for response in self._ask.ask(request):
                if "understood" in response:
                    await self._event({"type": "intent", **response["understood"]})
                elif "tool" in response:
                    await self._event({"type": "tool", **response["tool"]})
                elif "action" in response:
                    wait = turn_end + self._commit_delay - time.monotonic()
                    if wait > 0:
                        await asyncio.sleep(wait)
                    await self._event({"type": "action", "action": response["action"]})
                elif "confirmation" in response:
                    await self._event({"type": "confirm", **response["confirmation"]})
                elif "answer" in response:
                    answered = True
                    await self._say(response["answer"])
                elif "done" in response:
                    done = response["done"]
                    if not answered and done.get("answer"):
                        await self._say(done["answer"])
                    await self._event({"type": "end", **done})
        except (AskError, httpx.HTTPError) as e:
            logger.warning(f"voice session: Ask failed: {e}")
            code = e.code if isinstance(e, AskError) else "unavailable"
            await self._event({"type": "error", "code": code, "message": str(e)})
        # A cancelled request does not get here: _cancel ends its response.
        if self._speaking:
            self._speaking = False
            await self.push_frame(LLMFullResponseEndFrame())

    async def _say(self, text: str) -> None:
        if not self._speaking:
            self._speaking = True
            await self.push_frame(LLMFullResponseStartFrame())
        await self._event({"type": "answer", "text": text})
        await self.push_frame(LLMTextFrame(text))

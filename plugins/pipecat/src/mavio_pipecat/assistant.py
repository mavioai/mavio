"""The processor that takes the user's turns to the server's Ask, in place
of a Pipecat LLM service."""

import asyncio
import itertools
import time
from typing import Any

import httpx
from loguru import logger
from pipecat.frames.frames import (
    BotStartedSpeakingFrame,
    BotStoppedSpeakingFrame,
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
# How long the end of a run waits for more of its answer's speech once the
# voice is silent.
SPEECH_START_TIMEOUT = 3.0


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
    client as transport messages, each run's with its ID.

    A run's end waits until its answer has been spoken, so that what
    relays the session knows the speech that came before it is the run's
    (docs/assistant.md §8.8); one cancelled ends at once.
    """

    def __init__(
        self,
        ask: Asker,
        *,
        commit_delay: float = COMMIT_DELAY,
        context: dict[str, Any] | None = None,
        **kwargs: Any,
    ):
        """context is the client's AskContext, until a context event
        replaces it."""
        super().__init__(**kwargs)
        self._ask = ask
        self._commit_delay = commit_delay
        self._context: dict[str, Any] = context or {}
        self._ids = itertools.count(1)
        self._run: asyncio.Task[None] | None = None
        self._run_id = ""
        self._speaking = False
        # The end of a run whose answer is being spoken, and whether the
        # voice speaks.
        self._pending: dict[str, Any] | None = None
        self._bot_speaking = False
        self._pending_timeout: asyncio.Task[None] | None = None

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
        elif isinstance(frame, BotStartedSpeakingFrame):
            self._bot_speaking = True
        elif isinstance(frame, BotStoppedSpeakingFrame):
            self._bot_speaking = False
            await self._end_pending()
        await self.push_frame(frame, direction)

    async def cleanup(self) -> None:
        await self._cancel(end_response=False)
        await super().cleanup()

    async def _end_pending(self) -> None:
        """Sends the end of the run whose answer was spoken."""
        pending, self._pending = self._pending, None
        timeout, self._pending_timeout = self._pending_timeout, None
        if timeout is not None and timeout is not asyncio.current_task():
            await self.cancel_task(timeout)
        if pending is not None:
            await self._event(pending)

    async def _end_unspoken(self) -> None:
        """Ends the pending run when the voice says no more of it."""
        await asyncio.sleep(SPEECH_START_TIMEOUT)
        if not self._bot_speaking:
            await self._end_pending()

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
        # A run that answered keeps its end, sent before the next run's
        # events: its speech was cut, not its answer.
        await self._end_pending()
        run, self._run = self._run, None
        if run is not None and not run.done():
            await self.cancel_task(run)
            await self._event({"type": "end", "id": self._run_id, "status": "cancelled"})
        if self._speaking:
            self._speaking = False
            if end_response:
                await self.push_frame(LLMFullResponseEndFrame())

    async def _event(self, event: dict[str, Any]) -> None:
        await self.push_frame(OutputTransportMessageUrgentFrame(message=event))

    async def _request(self, text: str, turn_end: float) -> None:
        run_id = self._run_id = str(next(self._ids))
        await self._event({"type": "run", "id": run_id, "text": text})
        request: dict[str, Any] = {"text": text, "spoken": True}
        if self._context:
            request["context"] = self._context
        answered = False
        end: dict[str, Any] = {"type": "end", "id": run_id, "status": "failed"}
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
                    await self._say(run_id, response["answer"])
                elif "done" in response:
                    done = response["done"]
                    if not answered and done.get("answer"):
                        await self._say(run_id, done["answer"])
                    end = {
                        "type": "end",
                        "id": run_id,
                        "status": "completed",
                        "conversation_id": done.get("conversationId", ""),
                        "answer": done.get("answer", ""),
                        "language": done.get("language", ""),
                    }
        except (AskError, httpx.HTTPError) as e:
            logger.warning(f"voice session: Ask failed: {e}")
            code = e.code if isinstance(e, AskError) else "unavailable"
            await self._event({"type": "error", "code": code, "message": str(e)})
        # A cancelled request does not get here: _cancel ends it.
        self._run = None
        if not self._speaking:
            await self._event(end)
            return
        self._speaking = False
        self._pending = end
        self._pending_timeout = self.create_task(self._end_unspoken(), "end")
        await self.push_frame(LLMFullResponseEndFrame())

    async def _say(self, run_id: str, text: str) -> None:
        if not self._speaking:
            self._speaking = True
            await self.push_frame(LLMFullResponseStartFrame())
        await self._event({"type": "answer", "id": run_id, "text": text})
        await self.push_frame(LLMTextFrame(text))

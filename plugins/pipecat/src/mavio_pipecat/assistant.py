"""The processor that takes the user's turns to the server's Ask, in place
of a Pipecat LLM service."""

import asyncio
import contextlib
import itertools
import uuid
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
    UserStartedSpeakingFrame,
    UserStoppedSpeakingFrame,
)
from pipecat.processors.aggregators.llm_context import LLMContext
from pipecat.processors.frame_processor import FrameDirection, FrameProcessor

from mavio_pipecat.ask import Asker, AskError

# How long the end of a run waits for more of its answer's speech once the
# voice is silent.
SPEECH_START_TIMEOUT = 3.0
# How long an answer held while the user spoke waits, once the turn ends,
# for the turn's text, before it is spoken: a turn with no words has none.
HELD_ANSWER_WAIT = 0.5


def user_text(context: LLMContext, start: int = 0) -> str:
    """The user's messages from the context's message start on, joined:
    those not yet asked. An answer not spoken never enters the context, so
    what was asked is counted, not read from the answers."""
    texts: list[str] = []
    for message in context.get_messages()[start:]:
        if not isinstance(message, dict) or message.get("role") != "user":
            continue
        content = message.get("content")
        if isinstance(content, list):
            content = " ".join(
                part.get("text", "") for part in content if part.get("type") == "text"
            )
        if isinstance(content, str) and content.strip():
            texts.append(content.strip())
    return " ".join(texts)


class MavioAssistant(FrameProcessor):
    """Asks the server with each turn of the user and speaks the answer.

    An LLMContextFrame from the user aggregator starts a run with the turn;
    one that comes while a run has not answered yet steers it (SteerAsk):
    the user paused rather than ended, and the server drops the effects the
    run held and carries out the joined request. An answer that comes while
    the user is speaking again is held, not spoken over them: when the turn
    ends with words, the answer is dropped, unheard, and the turn starts the
    next run with what the user went on to say; a turn without words lets
    it be spoken. An interruption, the user
    speaking over the answer, aborts the run (AbortAsk), and the turn that
    interrupted starts the next. The answer's deltas go on as LLM text for
    the TTS service, and the run's events go to the client as transport
    messages, each with the run's ID (docs/assistant.md §8.4).

    A run's end waits until its answer has been spoken, so that what
    relays the session knows the speech that came before it is the run's
    (docs/assistant.md §8.8); one cancelled ends at once.
    """

    def __init__(
        self,
        ask: Asker,
        *,
        context: dict[str, Any] | None = None,
        **kwargs: Any,
    ):
        """context is the client's AskContext, until a context event
        replaces it."""
        super().__init__(**kwargs)
        self._ask = ask
        self._context: dict[str, Any] = context or {}
        self._ids = itertools.count(1)
        # Run IDs are unique among the device's: the session's runs are
        # named apart from another session's.
        self._prefix = uuid.uuid4().hex[:12]
        self._run: asyncio.Task[None] | None = None
        self._run_id = ""
        self._run_text = ""
        # The run in progress has answered: a turn no longer steers it.
        self._answered = False
        self._speaking = False
        # The end of a run whose answer is being spoken, and whether the
        # voice speaks.
        self._pending: dict[str, Any] | None = None
        self._bot_speaking = False
        self._pending_timeout: asyncio.Task[None] | None = None
        # A turn of the user opened while the run had not answered: its
        # answer and end are held until the turn ends.
        self._holding = False
        self._held: list[tuple[str, str]] = []
        self._held_end: dict[str, Any] | None = None
        self._release: asyncio.Task[None] | None = None
        # How many of the context's messages have been asked or steered with.
        self._asked = 0

    async def process_frame(self, frame: Frame, direction: FrameDirection) -> None:
        await super().process_frame(frame, direction)
        if isinstance(frame, LLMContextFrame):
            if frame.speculation:
                return
            text = user_text(frame.context, self._asked)
            self._asked = len(frame.context.get_messages())
            if self._holding:
                await self._stop_release()
                self._holding = False
                if self._held or self._held_end is not None:
                    # Answered while the user went on: the answer is dropped
                    # unheard, and what they went on to say is asked.
                    await self._drop_held()
                    if text:
                        self._run = self.create_task(self._request(text), "ask")
                    return
            if await self._steer(text):
                return
            await self._cancel(end_response=True, interrupt=True)
            if text:
                self._run = self.create_task(self._request(text), "ask")
            return
        if isinstance(frame, UserStartedSpeakingFrame):
            if self._run is not None and not self._run.done() and not self._answered:
                self._holding = True
        elif isinstance(frame, UserStoppedSpeakingFrame):
            if self._holding and self._release is None:
                self._release = self.create_task(self._release_later(), "release")
        elif isinstance(frame, InterruptionFrame):
            # The user speaking before the answer may only pause: the turn
            # steers the run when it ends.
            if self._answered or self._run is None:
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
        await self._stop_release()
        await self._cancel(end_response=False)
        await super().cleanup()

    async def _stop_release(self) -> None:
        release, self._release = self._release, None
        if release is not None and release is not asyncio.current_task():
            await self.cancel_task(release)

    async def _release_later(self) -> None:
        """Speaks the held answer when the turn that held it ended with
        no words."""
        await asyncio.sleep(HELD_ANSWER_WAIT)
        self._release = None
        self._holding = False
        held, self._held = self._held, []
        for run_id, text in held:
            await self._say(run_id, text)
        end, self._held_end = self._held_end, None
        if end is not None:
            await self._finish(end)

    async def _drop_held(self) -> None:
        """Ends the run whose answer was held, cancelled: it was not heard."""
        self._held, self._held_end = [], None
        run, self._run = self._run, None
        if run is not None and not run.done():
            with contextlib.suppress(AskError, httpx.HTTPError):
                await self._ask.abort(self._wire_id(self._run_id))
            await self.cancel_task(run)
        await self._event({"type": "end", "id": self._run_id, "status": "cancelled"})

    def _wire_id(self, run_id: str) -> str:
        return f"{self._prefix}-{run_id}"

    async def _steer(self, text: str) -> bool:
        """Steers the run in progress with a turn; false when there is
        none to steer, or it ended meanwhile."""
        run = self._run
        if run is None or run.done() or self._answered or not text:
            return False
        try:
            await self._ask.steer(self._wire_id(self._run_id), text)
        except (AskError, httpx.HTTPError) as e:
            logger.info(f"voice session: steer: {e}")
            return False
        self._run_text = f"{self._run_text} {text}"
        await self._event({"type": "run", "id": self._run_id, "text": self._run_text})
        return True

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
                    await self._cancel(end_response=True, interrupt=True)
                    self._run = self.create_task(self._request(text.strip()))
            case _:
                pass

    async def _cancel(self, *, end_response: bool, interrupt: bool = False) -> None:
        """Ends the run in progress. interrupt is set when another run
        replaces it: speech of its answer still playing is interrupted, so
        that the next run's speech does not follow it, nor is kept as that
        run's."""
        if interrupt and (self._speaking or self._bot_speaking or self._pending is not None):
            await self.broadcast_interruption()
        # A run that answered keeps its end, sent before the next run's
        # events: its speech was cut, not its answer.
        await self._end_pending()
        run, self._run = self._run, None
        if run is not None and not run.done():
            # The server aborts the model's run too; what was done stays.
            with contextlib.suppress(AskError, httpx.HTTPError):
                await self._ask.abort(self._wire_id(self._run_id))
            await self.cancel_task(run)
            await self._event({"type": "end", "id": self._run_id, "status": "cancelled"})
        if self._speaking:
            self._speaking = False
            if end_response:
                await self.push_frame(LLMFullResponseEndFrame())

    async def _event(self, event: dict[str, Any]) -> None:
        await self.push_frame(OutputTransportMessageUrgentFrame(message=event))

    async def _request(self, text: str) -> None:
        run_id = self._run_id = str(next(self._ids))
        self._run_text, self._answered = text, False
        await self._event({"type": "run", "id": run_id, "text": text})
        request: dict[str, Any] = {"text": text, "spoken": True, "runId": self._wire_id(run_id)}
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
                    # The server held it for the commit delay.
                    await self._event({"type": "action", "action": response["action"]})
                elif "confirmation" in response:
                    await self._event({"type": "confirm", **response["confirmation"]})
                elif "answer" in response:
                    answered = True
                    await self._say(run_id, response["answer"])
                elif "done" in response:
                    self._answered = True
                    done = response["done"]
                    if not answered and done.get("answer"):
                        await self._say(run_id, done["answer"])
                    end = {
                        "type": "end",
                        "id": run_id,
                        "status": "cancelled" if done.get("cancelled") else "completed",
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
        if self._holding:
            self._held_end = end
            return
        await self._finish(end)

    async def _finish(self, end: dict[str, Any]) -> None:
        """Sends a run's end, once its answer has been spoken."""
        if not self._speaking:
            await self._event(end)
            return
        self._speaking = False
        self._pending = end
        self._pending_timeout = self.create_task(self._end_unspoken(), "end")
        await self.push_frame(LLMFullResponseEndFrame())

    async def _say(self, run_id: str, text: str) -> None:
        self._answered = True
        if self._holding:
            self._held.append((run_id, text))
            return
        if not self._speaking:
            self._speaking = True
            await self.push_frame(LLMFullResponseStartFrame())
        await self._event({"type": "answer", "id": run_id, "text": text})
        await self.push_frame(LLMTextFrame(text))

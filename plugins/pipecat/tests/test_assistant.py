import asyncio
from collections.abc import Sequence
from typing import Any, cast

from pipecat.frames.frames import (
    Frame,
    InterruptionFrame,
    LLMContextFrame,
    LLMFullResponseEndFrame,
    LLMFullResponseStartFrame,
    LLMTextFrame,
    OutputTransportMessageUrgentFrame,
)
from pipecat.processors.aggregators.llm_context import LLMContext, LLMContextMessage
from pipecat.tests.utils import SleepFrame, run_test

from mavio_pipecat.assistant import MavioAssistant, trailing_user_text


class FakeAsk:
    """Answers each request with the responses of its script; a script
    item that is a float sleeps that long."""

    def __init__(self, *scripts: list[Any]):
        self.scripts = list(scripts)
        self.requests: list[dict[str, Any]] = []

    async def ask(self, request: dict[str, Any]):
        self.requests.append(request)
        for item in self.scripts.pop(0):
            if isinstance(item, float):
                await asyncio.sleep(item)
            else:
                yield item

    async def aclose(self):
        pass


def context(*messages: tuple[str, str]) -> LLMContextFrame:
    return LLMContextFrame(
        LLMContext([cast(LLMContextMessage, {"role": r, "content": c}) for r, c in messages])
    )


def events(frames: Sequence[Frame]) -> list[str]:
    return [f.message["type"] for f in frames if isinstance(f, OutputTransportMessageUrgentFrame)]


def speech(frames: Sequence[Frame]) -> list[str]:
    out: list[str] = []
    for f in frames:
        if isinstance(f, LLMTextFrame):
            out.append(f.text)
        elif isinstance(f, LLMFullResponseStartFrame):
            out.append("start")
        elif isinstance(f, LLMFullResponseEndFrame):
            out.append("end")
    return out


def test_trailing_user_text():
    cases = [
        ([("user", "hi")], "hi"),
        (
            [("user", "a"), ("assistant", "b"), ("user", "play"), ("user", "the matrix")],
            "play the matrix",
        ),
        ([("user", "a"), ("assistant", "b")], ""),
    ]
    for messages, want in cases:
        got = trailing_user_text(context(*messages).context)
        assert got == want, f"trailing_user_text({messages}) = {got!r}, want = {want!r}"


async def test_answer_is_spoken():
    ask = FakeAsk(
        [
            {"understood": {"text": "hi", "ai": True}},
            {"answer": "Hel"},
            {"answer": "lo"},
            {"done": {"answer": "Hello", "language": "en"}},
        ]
    )
    down, _ = await run_test(
        MavioAssistant(ask, commit_delay=0),
        frames_to_send=[context(("user", "hi")), SleepFrame(0.2)],
    )
    assert ask.requests == [{"text": "hi", "spoken": True}], f"got = {ask.requests}"
    assert events(down) == ["run", "intent", "answer", "answer", "end"], f"got = {events(down)}"
    assert speech(down) == ["start", "Hel", "lo", "end"], f"got = {speech(down)}"


async def test_fast_path_answer_is_in_done():
    ask = FakeAsk([{"understood": {"intent": "pause"}}, {"done": {"answer": "Paused."}}])
    down, _ = await run_test(
        MavioAssistant(ask, commit_delay=0),
        frames_to_send=[context(("user", "pause")), SleepFrame(0.2)],
    )
    assert speech(down) == ["start", "Paused.", "end"], f"got = {speech(down)}"


async def test_next_turn_steers():
    ask = FakeAsk(
        [{"understood": {"text": "play"}}, 5.0, {"done": {"answer": "never"}}],
        [{"done": {"answer": "Playing The Matrix."}}],
    )
    down, _ = await run_test(
        MavioAssistant(ask, commit_delay=0),
        frames_to_send=[
            context(("user", "play")),
            SleepFrame(0.1),
            context(("user", "play"), ("user", "the matrix")),
            SleepFrame(0.2),
        ],
    )
    texts = [r["text"] for r in ask.requests]
    assert texts == ["play", "play the matrix"], f"got = {texts}"
    assert speech(down) == ["start", "Playing The Matrix.", "end"], f"got = {speech(down)}"


async def test_interruption_aborts_and_drops_held_actions():
    ask = FakeAsk(
        [
            {"answer": "Playing"},
            {"action": {"id": "a1", "play": {"itemIds": ["x"]}}},
            {"done": {"answer": "Playing"}},
        ]
    )
    down, _ = await run_test(
        MavioAssistant(ask, commit_delay=0.5),
        frames_to_send=[
            context(("user", "play x")),
            SleepFrame(0.1),
            InterruptionFrame(),
            SleepFrame(0.6),
        ],
    )
    assert events(down) == ["run", "answer"], f"got = {events(down)}"
    # The TTS service and the assistant aggregator end the interrupted
    # response themselves.
    assert speech(down) == ["start", "Playing"], f"got = {speech(down)}"


async def test_actions_wait_for_the_commit_delay():
    ask = FakeAsk(
        [{"action": {"id": "a1", "control": {"command": "CONTROL_COMMAND_PAUSE"}}}, {"done": {}}]
    )
    down, _ = await run_test(
        MavioAssistant(ask, commit_delay=0.3),
        frames_to_send=[context(("user", "pause")), SleepFrame(0.1)],
    )
    assert events(down) == ["run"], f"before the delay: got = {events(down)}"

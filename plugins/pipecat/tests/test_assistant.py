from collections.abc import Sequence
from typing import Any, cast

from fakes import FakeAsk
from pipecat.frames.frames import (
    BotStartedSpeakingFrame,
    BotStoppedSpeakingFrame,
    Frame,
    InterruptionFrame,
    LLMContextFrame,
    LLMFullResponseEndFrame,
    LLMFullResponseStartFrame,
    LLMTextFrame,
    OutputTransportMessageUrgentFrame,
)
from pipecat.processors.aggregators.llm_context import LLMContext, LLMContextMessage
from pipecat.processors.frame_processor import FrameDirection
from pipecat.tests.utils import SleepFrame, run_test

from mavio_pipecat.assistant import MavioAssistant, trailing_user_text


def context(*messages: tuple[str, str]) -> LLMContextFrame:
    return LLMContextFrame(
        LLMContext([cast(LLMContextMessage, {"role": r, "content": c}) for r, c in messages])
    )


def messages(frames: Sequence[Frame]) -> list[dict[str, Any]]:
    return [f.message for f in frames if isinstance(f, OutputTransportMessageUrgentFrame)]


def events(frames: Sequence[Frame]) -> list[str]:
    return [m["type"] for m in messages(frames)]


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
            {"done": {"answer": "Hello", "language": "en", "conversationId": "c1"}},
        ]
    )
    assistant = MavioAssistant(ask, commit_delay=0)
    down, _ = await run_test(
        assistant,
        frames_to_send=[context(("user", "hi")), SleepFrame(0.2)],
    )
    assert ask.requests == [{"text": "hi", "spoken": True}], f"got = {ask.requests}"
    # The run's end waits for its speech, or for the session to end.
    want = ["run", "intent", "answer", "answer", "end"]
    assert events(down) == want, f"got = {events(down)}"
    assert speech(down) == ["start", "Hel", "lo", "end"], f"got = {speech(down)}"
    ids = {m.get("id") for m in messages(down) if m["type"] in ("run", "answer")}
    assert ids == {"1"}, f"run IDs = {ids}, want = {{'1'}}"


async def test_end_follows_the_speech():
    ask = FakeAsk([{"answer": "Hello"}, {"done": {"answer": "Hello", "conversationId": "c1"}}])
    down, _ = await run_test(
        MavioAssistant(ask, commit_delay=0),
        frames_to_send=[
            context(("user", "hi")),
            SleepFrame(0.1),
            BotStartedSpeakingFrame(),
            BotStoppedSpeakingFrame(),
        ],
        frames_to_send_direction=FrameDirection.DOWNSTREAM,
    )
    end = messages(down)[-1]
    want = {
        "type": "end",
        "id": "1",
        "status": "completed",
        "conversation_id": "c1",
        "answer": "Hello",
        "language": "",
    }
    assert end == want, f"end = {end}, want = {want}"


async def test_unspoken_end_is_sent():
    ask = FakeAsk([{"done": {"conversationId": "c1"}}])
    down, _ = await run_test(
        MavioAssistant(ask, commit_delay=0),
        frames_to_send=[context(("user", "hi")), SleepFrame(0.2)],
    )
    end = messages(down)[-1]
    assert (end["type"], end["status"]) == ("end", "completed"), f"got = {end}"


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
    ends = [(m["id"], m["status"]) for m in messages(down) if m["type"] == "end"]
    assert ends[0] == ("1", "cancelled"), f"ends = {ends}, want the first run cancelled"


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
    assert events(down) == ["run", "answer", "end"], f"got = {events(down)}"
    assert messages(down)[-1]["status"] == "cancelled"
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
    # The session ended before the delay: the run is cancelled, its
    # action dropped.
    assert events(down) == ["run", "end"], f"before the delay: got = {events(down)}"
    assert messages(down)[-1]["status"] == "cancelled"

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
    UserStartedSpeakingFrame,
    UserStoppedSpeakingFrame,
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
    assistant = MavioAssistant(ask)
    down, _ = await run_test(
        assistant,
        frames_to_send=[context(("user", "hi")), SleepFrame(0.2)],
    )
    run_id = ask.requests[0].pop("runId")
    assert ask.requests == [{"text": "hi", "spoken": True}], f"got = {ask.requests}"
    assert run_id.endswith("-1"), f"run ID = {run_id}"
    # The run's end waits for its speech, or for the session to end.
    want = ["run", "intent", "answer", "answer", "end"]
    assert events(down) == want, f"got = {events(down)}"
    assert speech(down) == ["start", "Hel", "lo", "end"], f"got = {speech(down)}"
    ids = {m.get("id") for m in messages(down) if m["type"] in ("run", "answer")}
    assert ids == {"1"}, f"run IDs = {ids}, want = {{'1'}}"


async def test_end_follows_the_speech():
    ask = FakeAsk([{"answer": "Hello"}, {"done": {"answer": "Hello", "conversationId": "c1"}}])
    down, _ = await run_test(
        MavioAssistant(ask),
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
        MavioAssistant(ask),
        frames_to_send=[context(("user", "hi")), SleepFrame(0.2)],
    )
    end = messages(down)[-1]
    assert (end["type"], end["status"]) == ("end", "completed"), f"got = {end}"


async def test_fast_path_answer_is_in_done():
    ask = FakeAsk([{"understood": {"intent": "pause"}}, {"done": {"answer": "Paused."}}])
    down, _ = await run_test(
        MavioAssistant(ask),
        frames_to_send=[context(("user", "pause")), SleepFrame(0.2)],
    )
    assert speech(down) == ["start", "Paused.", "end"], f"got = {speech(down)}"


async def test_next_turn_steers():
    ask = FakeAsk(
        [{"understood": {"text": "play"}}, 0.3, {"done": {"answer": "Playing The Matrix."}}]
    )
    down, _ = await run_test(
        MavioAssistant(ask),
        frames_to_send=[
            context(("user", "play")),
            SleepFrame(0.1),
            # The user speaking again before the answer has only paused.
            InterruptionFrame(),
            context(("user", "play"), ("user", "the matrix")),
            SleepFrame(0.4),
        ],
    )
    run_id = ask.requests[0]["runId"]
    assert [r["text"] for r in ask.requests] == ["play"], f"requests = {ask.requests}"
    assert ask.steers == [(run_id, "the matrix")], f"steers = {ask.steers}"
    assert ask.aborts == [], f"aborts = {ask.aborts}"
    runs = [(m["id"], m["text"]) for m in messages(down) if m["type"] == "run"]
    assert runs == [("1", "play"), ("1", "play the matrix")], f"runs = {runs}"
    assert speech(down) == ["start", "Playing The Matrix.", "end"], f"got = {speech(down)}"


async def test_turn_after_the_run_ended_starts_another():
    ask = FakeAsk([{"done": {"answer": "Paused."}}], [{"done": {"answer": "Resumed."}}])
    await run_test(
        MavioAssistant(ask),
        frames_to_send=[
            context(("user", "pause")),
            SleepFrame(0.1),
            context(("user", "pause"), ("assistant", "Paused."), ("user", "resume")),
            SleepFrame(0.1),
        ],
    )
    assert [r["text"] for r in ask.requests] == ["pause", "resume"], f"got = {ask.requests}"
    assert ask.steers == []
    assert ask.requests[0]["runId"] != ask.requests[1]["runId"]


async def test_interruption_aborts_the_answer():
    ask = FakeAsk([{"answer": "Playing"}, 1.0, {"done": {"answer": "Playing"}}])
    down, _ = await run_test(
        MavioAssistant(ask),
        frames_to_send=[
            context(("user", "play x")),
            SleepFrame(0.1),
            InterruptionFrame(),
            SleepFrame(0.1),
        ],
    )
    assert events(down) == ["run", "answer", "end"], f"got = {events(down)}"
    assert messages(down)[-1]["status"] == "cancelled"
    assert ask.aborts == [ask.requests[0]["runId"]], f"aborts = {ask.aborts}"
    # The TTS service and the assistant aggregator end the interrupted
    # response themselves.
    assert speech(down) == ["start", "Playing"], f"got = {speech(down)}"


async def test_aborted_run_ends_cancelled():
    ask = FakeAsk([{"done": {"cancelled": True}}])
    down, _ = await run_test(
        MavioAssistant(ask),
        frames_to_send=[context(("user", "pause")), SleepFrame(0.1)],
    )
    end = messages(down)[-1]
    assert (end["type"], end["status"]) == ("end", "cancelled"), f"got = {end}"


async def test_answer_while_the_user_goes_on_is_dropped():
    # The answer comes while the user speaks again: it is not spoken over
    # them, and the turn asks what they went on to say.
    ask = FakeAsk(
        [
            0.1,
            {"answer": "Ha, thanks!"},
            {"done": {"answer": "Ha, thanks!", "conversationId": "c1"}},
        ],
        [{"answer": "I can't check the weather."}, {"done": {"conversationId": "c1"}}],
    )
    down, _ = await run_test(
        MavioAssistant(ask),
        frames_to_send=[
            context(("user", "you're fun")),
            SleepFrame(0.05),
            UserStartedSpeakingFrame(),
            InterruptionFrame(),
            SleepFrame(0.2),
            UserStoppedSpeakingFrame(),
            context(("user", "you're fun"), ("user", "how's the weather")),
            SleepFrame(0.2),
        ],
    )
    assert [r["text"] for r in ask.requests] == ["you're fun", "how's the weather"], (
        f"requests = {ask.requests}"
    )
    assert ask.steers == [], f"steers = {ask.steers}"
    assert speech(down) == ["start", "I can't check the weather.", "end"], f"got = {speech(down)}"
    got = [
        (m["type"], m.get("id"), m.get("status"))
        for m in messages(down)
        if m["type"] in ("run", "answer", "end")
    ]
    want = [
        ("run", "1", None),
        ("end", "1", "cancelled"),
        ("run", "2", None),
        ("answer", "2", None),
        ("end", "2", "completed"),
    ]
    assert got == want, f"events = {got}, want = {want}"


async def test_answer_held_through_a_wordless_turn_is_spoken():
    ask = FakeAsk([0.1, {"answer": "Hello"}, {"done": {"answer": "Hello", "conversationId": "c1"}}])
    down, _ = await run_test(
        MavioAssistant(ask),
        frames_to_send=[
            context(("user", "hi")),
            SleepFrame(0.05),
            UserStartedSpeakingFrame(),
            InterruptionFrame(),
            SleepFrame(0.2),
            UserStoppedSpeakingFrame(),
            SleepFrame(0.8),
        ],
    )
    assert speech(down) == ["start", "Hello", "end"], f"got = {speech(down)}"
    assert events(down) == ["run", "answer", "end"], f"got = {events(down)}"
    assert messages(down)[-1]["status"] == "completed"


async def test_a_run_replacing_a_spoken_answer_interrupts_it():
    ask = FakeAsk(
        [{"answer": "Paused."}, {"done": {"answer": "Paused."}}], [{"done": {"answer": "Resumed."}}]
    )
    down, _ = await run_test(
        MavioAssistant(ask),
        frames_to_send=[
            context(("user", "pause")),
            SleepFrame(0.1),
            BotStartedSpeakingFrame(),
            context(("user", "pause"), ("assistant", "Paused."), ("user", "resume")),
            SleepFrame(0.1),
        ],
    )
    kinds = [
        type(f).__name__
        for f in down
        if isinstance(f, (InterruptionFrame, OutputTransportMessageUrgentFrame))
    ]
    first_run2 = next(
        i
        for i, f in enumerate(down)
        if isinstance(f, OutputTransportMessageUrgentFrame)
        and f.message == {"type": "run", "id": "2", "text": "resume"}
    )
    interrupt = next(i for i, f in enumerate(down) if isinstance(f, InterruptionFrame))
    assert interrupt < first_run2, f"frames = {kinds}"

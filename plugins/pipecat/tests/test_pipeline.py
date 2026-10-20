from typing import Any

from pipecat.frames.frames import (
    BotStartedSpeakingFrame,
    BotStoppedSpeakingFrame,
    Frame,
    InterimTranscriptionFrame,
    TranscriptionFrame,
    VADUserStartedSpeakingFrame,
)
from pipecat.turns.user_start.base_user_turn_start_strategy import BaseUserTurnStartStrategy

from mavio_pipecat.pipeline import BargeIn, QuietVADStart, words


def test_words():
    cases = [
        ("", 0),
        ("mm", 1),
        ("stop it", 2),
        ("嗯", 1),
        ("停下", 2),
        ("停下 please", 3),
        ("止めて", 3),
        ("다음 에피소드", 2),
    ]
    for text, want in cases:
        assert words(text) == want, f"words({text!r}) = {words(text)}, want = {want}"


async def _started(strategy: BaseUserTurnStartStrategy, frames: list[Frame]) -> list[str]:
    """What the strategy does with the frames: the user's turn started, or
    the transcript dropped."""
    did: list[str] = []

    async def started(_strategy: Any, _params: Any) -> None:
        did.append("started")

    async def reset(_strategy: Any) -> None:
        did.append("reset")

    strategy.add_event_handler("on_user_turn_started", started)
    strategy.add_event_handler("on_reset_aggregation", reset)
    for f in frames:
        await strategy.process_frame(f)
    return did


def _said(text: str, interim: bool = False) -> Frame:
    if interim:
        return InterimTranscriptionFrame(text, "user", "")
    return TranscriptionFrame(text, "user", "")


async def test_vad_starts_turns_only_while_the_assistant_is_silent():
    vad = VADUserStartedSpeakingFrame()
    got = await _started(
        QuietVADStart(),
        [vad, BotStartedSpeakingFrame(), vad, BotStoppedSpeakingFrame(), vad],
    )
    assert got == ["started", "started"], f"got = {got}"


async def test_barge_in_takes_two_words_while_the_assistant_speaks():
    got = await _started(BargeIn(), [_said("嗯")])
    assert got == ["started"], f"while silent, got = {got}"
    got = await _started(
        BargeIn(),
        [BotStartedSpeakingFrame(), _said("mm"), _said("嗯"), _said("停下", interim=True)],
    )
    assert got == ["reset", "reset", "started"], f"while speaking, got = {got}"

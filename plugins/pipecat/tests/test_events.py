from collections.abc import Sequence
from typing import Any

from pipecat.frames.frames import (
    BotStartedSpeakingFrame,
    Frame,
    InterimTranscriptionFrame,
    OutputTransportMessageUrgentFrame,
    TranscriptionFrame,
    UserStoppedSpeakingFrame,
)
from pipecat.processors.frame_processor import FrameDirection
from pipecat.tests.utils import run_test
from pipecat.transcriptions.language import Language

from mavio_pipecat.events import SessionEvents


def messages(frames: Sequence[Frame]) -> list[dict[str, Any]]:
    return [f.message for f in frames if isinstance(f, OutputTransportMessageUrgentFrame)]


async def test_transcripts():
    down, _ = await run_test(
        SessionEvents(),
        frames_to_send=[
            InterimTranscriptionFrame("pla", "u", "t"),
            TranscriptionFrame("播放", "u", "t", language=Language.ZH),
        ],
    )
    want = [
        {"type": "transcript", "text": "pla", "final": False},
        {"type": "transcript", "text": "播放", "final": True, "language": "zh"},
    ]
    assert messages(down) == want, f"got = {messages(down)}, want = {want}"


async def test_turns_and_speech_coming_upstream():
    down, up = await run_test(
        SessionEvents(),
        frames_to_send=[UserStoppedSpeakingFrame(), BotStartedSpeakingFrame()],
        frames_to_send_direction=FrameDirection.UPSTREAM,
    )
    want = [{"type": "user_stopped"}, {"type": "bot_started"}]
    assert messages(down) == want, f"got = {messages(down)}, want = {want}"
    assert [type(f) for f in up] == [UserStoppedSpeakingFrame, BotStartedSpeakingFrame]

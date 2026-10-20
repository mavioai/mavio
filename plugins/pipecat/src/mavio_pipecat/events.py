"""The processor that tells the client what the session hears and says."""

from typing import Any

from pipecat.frames.frames import (
    BotStartedSpeakingFrame,
    BotStoppedSpeakingFrame,
    Frame,
    InterimTranscriptionFrame,
    OutputTransportMessageUrgentFrame,
    TranscriptionFrame,
    UserStartedSpeakingFrame,
    UserStoppedSpeakingFrame,
)
from pipecat.processors.frame_processor import FrameDirection, FrameProcessor


class SessionEvents(FrameProcessor):
    """Turns the pipeline's frames into the protocol's events: the user's
    turns and transcripts, and the bot's speech.

    It sits between the STT service and the user aggregator, where each of
    these frames passes once: transcripts downstream from the STT service,
    the user's turns upstream from the aggregator, the bot's speech upstream
    from the output transport.
    """

    async def process_frame(self, frame: Frame, direction: FrameDirection) -> None:
        await super().process_frame(frame, direction)
        await self.push_frame(frame, direction)
        event = _event(frame)
        if event is not None:
            await self.push_frame(OutputTransportMessageUrgentFrame(message=event))


def _event(frame: Frame) -> dict[str, Any] | None:
    match frame:
        case UserStartedSpeakingFrame():
            return {"type": "user_started"}
        case UserStoppedSpeakingFrame():
            return {"type": "user_stopped"}
        case TranscriptionFrame() | InterimTranscriptionFrame():
            event: dict[str, Any] = {
                "type": "transcript",
                "text": frame.text,
                "final": isinstance(frame, TranscriptionFrame),
            }
            if frame.language:
                event["language"] = str(frame.language)
            return event
        case BotStartedSpeakingFrame():
            return {"type": "bot_started"}
        case BotStoppedSpeakingFrame():
            return {"type": "bot_stopped"}
        case _:
            return None

"""The session protocol's messages on the WebSocket, as Pipecat frames."""

import json
from typing import Any, cast

from loguru import logger
from pipecat.frames.frames import (
    Frame,
    InputAudioRawFrame,
    InputTransportMessageFrame,
    InterruptionFrame,
    OutputAudioRawFrame,
    OutputTransportMessageFrame,
    OutputTransportMessageUrgentFrame,
)
from pipecat.serializers.base_serializer import FrameSerializer

# The rate of the microphone's PCM the client sends.
INPUT_SAMPLE_RATE = 16000
# The rate of the answer's PCM the client plays.
OUTPUT_SAMPLE_RATE = 24000


class MavioSerializer(FrameSerializer):
    """Binary messages are mono 16-bit little-endian PCM, 16 kHz from the
    client and 24 kHz to it; text messages are the protocol's JSON events,
    each with its "type".

    Events from the client become InputTransportMessageFrames; events to it
    are the transport message frames the pipeline's processors push, and
    an interruption, which tells the client to drop the speech it has not
    played yet.
    """

    async def serialize(self, frame: Frame) -> str | bytes | None:
        if isinstance(frame, OutputAudioRawFrame):
            return frame.audio
        if isinstance(frame, InterruptionFrame):
            return json.dumps({"type": "interrupt"})
        if isinstance(frame, (OutputTransportMessageFrame, OutputTransportMessageUrgentFrame)):
            if self.should_ignore_frame(frame):
                return None
            return json.dumps(frame.message, ensure_ascii=False)
        return None

    async def deserialize(self, data: str | bytes) -> Frame | None:
        if isinstance(data, bytes):
            return InputAudioRawFrame(audio=data, sample_rate=INPUT_SAMPLE_RATE, num_channels=1)
        try:
            message: object = json.loads(data)
        except json.JSONDecodeError:
            logger.warning("voice session: a text message is not JSON")
            return None
        if not isinstance(message, dict) or not isinstance(
            cast(dict[str, Any], message).get("type"), str
        ):
            logger.warning("voice session: a message has no type")
            return None
        return InputTransportMessageFrame(message=message)

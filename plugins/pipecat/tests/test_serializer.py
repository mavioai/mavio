import json

from pipecat.frames.frames import (
    InputAudioRawFrame,
    InputTransportMessageFrame,
    InterruptionFrame,
    OutputAudioRawFrame,
    OutputTransportMessageUrgentFrame,
    TextFrame,
)

from mavio_pipecat.serializer import MavioSerializer


async def test_deserialize():
    s = MavioSerializer()
    frame = await s.deserialize(b"\x01\x00" * 320)
    assert isinstance(frame, InputAudioRawFrame), f"got = {frame!r}, want = InputAudioRawFrame"
    assert (frame.sample_rate, frame.num_channels, len(frame.audio)) == (16000, 1, 640)

    frame = await s.deserialize('{"type": "context", "context": {"screen": "home"}}')
    assert isinstance(frame, InputTransportMessageFrame)
    assert frame.message == {"type": "context", "context": {"screen": "home"}}

    for bad in ["not json", "[1]", '{"text": "no type"}']:
        got = await s.deserialize(bad)
        assert got is None, f"deserialize({bad!r}) = {got!r}, want = None"


async def test_serialize():
    s = MavioSerializer()
    audio = OutputAudioRawFrame(audio=b"\x00\x01" * 10, sample_rate=24000, num_channels=1)
    assert await s.serialize(audio) == audio.audio
    assert json.loads(await s.serialize(InterruptionFrame()) or "") == {"type": "interrupt"}
    message = OutputTransportMessageUrgentFrame(message={"type": "answer", "text": "好"})
    assert await s.serialize(message) == '{"type": "answer", "text": "好"}'
    assert await s.serialize(TextFrame("x")) is None

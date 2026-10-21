from pathlib import Path

import pytest
from fakes import FakeSpeaches
from pipecat.services.tts_service import TTSService

from mavio_pipecat.models import Models
from mavio_pipecat.services import SYNTHESIS, Session
from mavio_pipecat.spoken import spoken_text


@pytest.mark.parametrize(
    ("text", "want"),
    [
        ("好的", "好的"),
        ("好的😊", "好的"),
        ("😂", ""),
        ("👂 ", " "),
        ("Sure 👍🏽 here it is.", "Sure here it is."),
        ("Family 👨‍👩‍👧 night", "Family night"),
        ("❤️ it ✅", " it "),
        ("Press 1️⃣ now", "Press now"),
        ("🇯🇵 Tokyo", " Tokyo"),
        ("Season 2, 10:30 — 50%", "Season 2, 10:30 — 50%"),
        ("《星际穿越》", "《星际穿越》"),
    ],
)
def test_spoken_text(text: str, want: str):
    got = spoken_text(text)
    assert got == want, f"spoken_text({text!r}) = {got!r}, want = {want!r}"


async def test_tts_says_nothing_of_emoji_only_text(tmp_path: Path):
    """A TTS service the plugin makes sends no text of emoji only to be
    synthesized, and the rest of a text without its emoji."""
    service = next(s for s in SYNTHESIS if s.name == "openai")
    assert service.make is not None
    tts = service.make(
        service.values({"api_key": "k", "voice": "v"}),
        Session(language="zh-CN"),
        Models(tmp_path, FakeSpeaches()),
    )
    assert isinstance(tts, TTSService)
    for text, want in (("😂", None), ("好的😊", "好的")):
        got = await tts._filter_text(text)  # pyright: ignore[reportPrivateUsage]
        assert got == want, f"_filter_text({text!r}) = {got!r}, want = {want!r}"

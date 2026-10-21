"""What of an answer's text the voice speaks: its emoji and pictographs are
left out, since no TTS service pronounces them, and a sentence of nothing
else is not sent at all."""

import re

from pipecat.utils.text.base_text_filter import BaseTextFilter

_EMOJI = re.compile(
    "[#*0-9]️?⃣"  # keycaps
    "|["
    "\U0001f000-\U0001faff"  # pictographs, emoticons, flags, skin tones
    "⌀-⏿"  # technical symbols: ⌚ ⏰
    "☀-➿"  # symbols and dingbats: ☀ ✅ ❤
    "⬀-⯿"  # stars and arrows: ⭐ ⬆
    "︎️‍"  # presentation selectors and joiners
    "\U000e0020-\U000e007f"  # tags of subdivision flags
    "]+"
)
_SPACES = re.compile(r"[ \t]{2,}")


def spoken_text(text: str) -> str:
    """The text with its emoji left out; spaces they leave doubled are
    joined, and nothing but whitespace remains of a text of emoji only."""
    if not _EMOJI.search(text):
        return text
    return _SPACES.sub(" ", _EMOJI.sub("", text))


class SpokenTextFilter(BaseTextFilter):
    """The TTS services' filter of the text they synthesize: spoken_text.
    A TTS service skips text the filters leave nothing but whitespace of."""

    async def filter(self, text: str) -> str:
        return spoken_text(text)

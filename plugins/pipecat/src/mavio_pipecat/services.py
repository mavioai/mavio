"""The recognition, synthesis and transcription services the plugin
bundles (docs/assistant.md §8.7, §8.9): their settings, which the
administrator fills for each recognizer, voice and transcriber, and how
each makes its Pipecat service."""

import json
from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Any

from pipecat.processors.frame_processor import FrameProcessor
from pipecat.transcriptions.language import Language

from mavio.plugin.v1 import settings_pb2
from mavio_pipecat.models import ModelRef, Models, kokoro_model, piper_model, whisper_model
from mavio_pipecat.speaches import CONTAINER

# The longest list of hints a service is given.
MAX_HINTS = 100


@dataclass(frozen=True)
class Setting:
    """A setting of a service, as the administrator enters it."""

    name: str
    label: str
    help: str = ""
    # "text", "number", "select" or "checkbox".
    type: str = "text"
    default: Any = None
    options: tuple[tuple[Any, str], ...] = ()
    secret: bool = False
    advanced: bool = False

    def proto(self) -> settings_pb2.SettingField:
        types = {
            "text": settings_pb2.SETTING_FIELD_TYPE_TEXT,
            "number": settings_pb2.SETTING_FIELD_TYPE_NUMBER,
            "select": settings_pb2.SETTING_FIELD_TYPE_SELECT,
            "checkbox": settings_pb2.SETTING_FIELD_TYPE_CHECKBOX,
        }
        return settings_pb2.SettingField(
            name=self.name,
            label=self.label,
            help_text=self.help,
            type=types[self.type],
            default_json="" if self.default is None else json.dumps(self.default),
            options=[
                settings_pb2.SettingFieldOption(value_json=json.dumps(v), label=label)
                for v, label in self.options
            ],
            advanced=self.advanced,
            secret=self.secret,
        )


@dataclass(frozen=True)
class Session:
    """What a session's services are made for: the user's language, BCP
    47, empty when the recognizer detects it, and the phrases it is likely
    to hear."""

    language: str = ""
    hints: tuple[str, ...] = ()


@dataclass(frozen=True)
class Service:
    """A service recognizers, voices or transcribers are configured from.

    make makes a session's Pipecat service of an engine's settings; model
    tells the local model an engine's settings need, if any; languages
    tells the languages an engine speaks or recognizes, empty for any.
    """

    name: str
    display_name: str
    settings: tuple[Setting, ...] = ()
    hints: bool = False
    make: Callable[[dict[str, Any], Session, Models], FrameProcessor] | None = None
    model: Callable[[dict[str, Any]], ModelRef | None] = lambda _settings: None
    languages: Callable[[dict[str, Any]], list[str]] = lambda _settings: []

    def values(self, settings: dict[str, Any]) -> dict[str, Any]:
        """The settings with the defaults of those not given."""
        out = {s.name: s.default for s in self.settings if s.default is not None}
        out.update({k: v for k, v in settings.items() if v not in (None, "")})
        return out


def pipecat_language(tag: str) -> Language | None:
    """Pipecat's language of a BCP 47 tag, or of its primary language;
    None for none Pipecat knows."""
    for t in (tag, tag.split("-")[0]):
        try:
            return Language(t)
        except ValueError:
            continue
    return None


def _prompt(session: Session) -> str | None:
    hints = session.hints[:MAX_HINTS]
    return ", ".join(hints) if hints else None


def _api_key(label: str = "API key") -> Setting:
    return Setting("api_key", label, secret=True)


_GOOGLE_SETTINGS = (
    Setting(
        "credentials",
        "Service account key",
        "The JSON key of a Google Cloud service account",
        secret=True,
    ),
    Setting("location", "Location", default="global", advanced=True),
)


# speaches' Whisper models, by their Hugging Face IDs.
_WHISPER_MODELS = (
    ("deepdml/faster-whisper-large-v3-turbo-ct2", "large-v3-turbo (1.6 GB)"),
    ("Systran/faster-whisper-large-v3", "large-v3 (3 GB)"),
    ("Systran/faster-whisper-medium", "medium (1.5 GB)"),
    ("Systran/faster-whisper-small", "small (480 MB)"),
    ("Systran/faster-whisper-base", "base (145 MB)"),
    ("Systran/faster-whisper-tiny", "tiny (75 MB)"),
)

WHISPER_SETTINGS = (
    Setting(
        "model",
        "Model",
        type="select",
        default=_WHISPER_MODELS[0][0],
        options=_WHISPER_MODELS,
    ),
)

OPENAI_SETTINGS = (
    Setting(
        "base_url",
        "Base URL",
        "The API's base URL, such as that of speaches or Groq",
        default="https://api.openai.com/v1",
    ),
    _api_key(),
)


def _whisper_stt(settings: dict[str, Any], session: Session, models: Models) -> FrameProcessor:
    """Whisper on speaches, through its OpenAI-compatible API."""
    return _openai_stt(
        {"base_url": models.speaches.base_url, "model": settings["model"]}, session, models
    )


def _openai_stt(settings: dict[str, Any], session: Session, _models: Models) -> FrameProcessor:
    from pipecat.services.openai.stt import OpenAISTTService

    return OpenAISTTService(
        # A client needs a key, which speaches takes any of.
        api_key=settings.get("api_key") or "none",
        base_url=settings["base_url"],
        settings=OpenAISTTService.Settings(
            model=settings["model"],
            language=pipecat_language(session.language),
            prompt=_prompt(session),
        ),
    )


def _deepgram_stt(settings: dict[str, Any], session: Session, _models: Models) -> FrameProcessor:
    from pipecat.services.deepgram.stt import DeepgramSTTService

    language = pipecat_language(session.language)
    return DeepgramSTTService(
        api_key=settings.get("api_key", ""),
        settings=DeepgramSTTService.Settings(
            model=settings["model"],
            language=language if language is not None else "multi",
            keyterm=list(session.hints[:MAX_HINTS]),
        ),
    )


def _cartesia_stt(settings: dict[str, Any], session: Session, _models: Models) -> FrameProcessor:
    from pipecat.services.cartesia.stt import CartesiaSTTService

    model = settings["model"]
    # Only the ink-2 and ink-preview models take key terms.
    hints = model.startswith(("ink-2", "ink-preview")) and session.hints
    return CartesiaSTTService(
        api_key=settings.get("api_key", ""),
        settings=CartesiaSTTService.Settings(
            model=model,
            language=pipecat_language(session.language),
            keyterm=list(session.hints[:MAX_HINTS]) if hints else None,
        ),
    )


def _elevenlabs_stt(settings: dict[str, Any], session: Session, _models: Models) -> FrameProcessor:
    from pipecat.services.elevenlabs.stt import ElevenLabsRealtimeSTTService

    return ElevenLabsRealtimeSTTService(
        api_key=settings.get("api_key", ""),
        settings=ElevenLabsRealtimeSTTService.Settings(
            model=settings["model"], language=pipecat_language(session.language)
        ),
    )


def _assemblyai_stt(settings: dict[str, Any], session: Session, _models: Models) -> FrameProcessor:
    from pipecat.services.assemblyai.stt import AssemblyAISTTService

    return AssemblyAISTTService(
        api_key=settings.get("api_key", ""),
        settings=AssemblyAISTTService.Settings(
            model=settings["model"],
            language=pipecat_language(session.language),
            keyterms_prompt=list(session.hints[:MAX_HINTS]) or None,
        ),
    )


def _soniox_stt(settings: dict[str, Any], session: Session, _models: Models) -> FrameProcessor:
    from pipecat.services.soniox.stt import SonioxContextObject, SonioxSTTService

    language = pipecat_language(session.language)
    hints = list(session.hints[:MAX_HINTS])
    return SonioxSTTService(
        api_key=settings.get("api_key", ""),
        settings=SonioxSTTService.Settings(
            model=settings["model"],
            language_hints=[language] if language else None,
            context=SonioxContextObject(terms=hints) if hints else None,
        ),
    )


def _speechmatics_stt(
    settings: dict[str, Any], session: Session, _models: Models
) -> FrameProcessor:
    from pipecat.services.speechmatics.stt import SpeechmaticsSTTService

    return SpeechmaticsSTTService(
        api_key=settings.get("api_key", ""),
        settings=SpeechmaticsSTTService.Settings(
            language=pipecat_language(session.language) or Language.EN,
            additional_vocab=[
                SpeechmaticsSTTService.AdditionalVocabEntry(content=h)
                for h in session.hints[:MAX_HINTS]
            ],
        ),
    )


def _google_stt(settings: dict[str, Any], session: Session, _models: Models) -> FrameProcessor:
    from pipecat.services.google.stt import GoogleSTTService

    language = pipecat_language(session.language)
    hints = session.hints[:MAX_HINTS]
    return GoogleSTTService(
        credentials=settings.get("credentials", ""),
        location=settings["location"],
        settings=GoogleSTTService.Settings(
            model=settings["model"],
            languages=[language] if language else [Language.EN_US],
            adaptation={"phrase_sets": [{"phrases": [{"value": h} for h in hints]}]}
            if hints
            else None,
        ),
    )


RECOGNITION = (
    Service(
        CONTAINER,
        "Speaches",
        WHISPER_SETTINGS,
        hints=True,
        make=_whisper_stt,
        model=lambda s: whisper_model(s["model"]),
    ),
    Service(
        "openai",
        "OpenAI-compatible",
        (*OPENAI_SETTINGS, Setting("model", "Model", default="gpt-4o-transcribe")),
        hints=True,
        make=_openai_stt,
    ),
    Service(
        "deepgram",
        "Deepgram",
        (_api_key(), Setting("model", "Model", default="nova-3")),
        hints=True,
        make=_deepgram_stt,
    ),
    Service(
        "cartesia",
        "Cartesia",
        (
            _api_key(),
            Setting(
                "model",
                "Model",
                "Only the ink-2 models take the users' hints",
                default="ink-whisper",
            ),
        ),
        hints=True,
        make=_cartesia_stt,
    ),
    Service(
        "elevenlabs",
        "ElevenLabs",
        (_api_key(), Setting("model", "Model", default="scribe_v2_realtime")),
        make=_elevenlabs_stt,
    ),
    Service(
        "assemblyai",
        "AssemblyAI",
        (_api_key(), Setting("model", "Model", default="universal-3-6-pro")),
        hints=True,
        make=_assemblyai_stt,
    ),
    Service(
        "soniox",
        "Soniox",
        (_api_key(), Setting("model", "Model", default="stt-rt-v5")),
        hints=True,
        make=_soniox_stt,
    ),
    Service(
        "speechmatics",
        "Speechmatics",
        (_api_key(),),
        hints=True,
        make=_speechmatics_stt,
    ),
    Service(
        "google",
        "Google Cloud",
        (*_GOOGLE_SETTINGS, Setting("model", "Model", default="latest_long")),
        hints=True,
        make=_google_stt,
    ),
)

# Kokoro's voices are named after their language: "af_heart" is American
# English, "zf_xiaobei" Mandarin.
_KOKORO_LANGUAGES = {
    "a": "en-US",
    "b": "en-GB",
    "e": "es",
    "f": "fr",
    "h": "hi",
    "i": "it",
    "j": "ja",
    "p": "pt-BR",
    "z": "zh",
}


def kokoro_language(voice: str) -> str:
    """The language a Kokoro voice speaks, by its name; empty when unknown."""
    return _KOKORO_LANGUAGES.get(voice[:1], "")


def _kokoro_tts(settings: dict[str, Any], session: Session, models: Models) -> FrameProcessor:
    from pipecat.services.kokoro.tts import KokoroTTSService

    files = models.kokoro()
    language = pipecat_language(kokoro_language(settings["voice"]) or session.language)
    return KokoroTTSService(
        model_path=str(files.model),
        voices_path=str(files.voices),
        settings=KokoroTTSService.Settings(
            voice=settings["voice"], language=language or Language.EN
        ),
    )


def _openai_tts(settings: dict[str, Any], session: Session, _models: Models) -> FrameProcessor:
    from pipecat.services.openai.tts import OpenAITTSService

    return OpenAITTSService(
        api_key=settings.get("api_key", ""),
        base_url=settings["base_url"],
        settings=OpenAITTSService.Settings(model=settings["model"], voice=settings["voice"]),
    )


def _deepgram_tts(settings: dict[str, Any], session: Session, _models: Models) -> FrameProcessor:
    from pipecat.services.deepgram.tts import DeepgramTTSService

    return DeepgramTTSService(
        api_key=settings.get("api_key", ""),
        settings=DeepgramTTSService.Settings(voice=settings["voice"]),
    )


def _cartesia_tts(settings: dict[str, Any], session: Session, _models: Models) -> FrameProcessor:
    from pipecat.services.cartesia.tts import CartesiaTTSService

    return CartesiaTTSService(
        api_key=settings.get("api_key", ""),
        settings=CartesiaTTSService.Settings(
            model=settings["model"],
            voice=settings.get("voice", ""),
            language=pipecat_language(session.language),
        ),
    )


def _elevenlabs_tts(settings: dict[str, Any], session: Session, _models: Models) -> FrameProcessor:
    from pipecat.services.elevenlabs.tts import ElevenLabsTTSService

    return ElevenLabsTTSService(
        api_key=settings.get("api_key", ""),
        settings=ElevenLabsTTSService.Settings(
            model=settings["model"],
            voice=settings.get("voice", ""),
            language=pipecat_language(session.language),
        ),
    )


def piper_language(voice: str) -> str:
    """The language a Piper voice speaks, by its name: "zh_CN-huayan-medium"
    speaks zh-CN; empty when it names none."""
    lang = voice.split("-")[0]
    return lang.replace("_", "-") if "_" in lang else ""


def _piper_tts(settings: dict[str, Any], session: Session, models: Models) -> FrameProcessor:
    from pipecat.services.piper.tts import PiperTTSService

    voice = settings["voice"]
    return PiperTTSService(
        download_dir=models.piper(voice).parent,
        settings=PiperTTSService.Settings(voice=voice),
    )


def _soniox_tts(settings: dict[str, Any], session: Session, _models: Models) -> FrameProcessor:
    from pipecat.services.soniox.tts import SonioxTTSService

    return SonioxTTSService(
        api_key=settings.get("api_key", ""),
        settings=SonioxTTSService.Settings(
            model=settings["model"],
            voice=settings["voice"],
            language=pipecat_language(session.language),
        ),
    )


def google_language(voice: str) -> str:
    """The language a Google voice speaks, by its name: "cmn-CN-Chirp3-HD-
    Aoede" speaks Mandarin, zh-CN; empty when it names none."""
    parts = voice.split("-")
    if len(parts) <= 2:
        return ""
    return ("zh" if parts[0] == "cmn" else parts[0]) + "-" + parts[1]


def _google_tts(settings: dict[str, Any], session: Session, _models: Models) -> FrameProcessor:
    from pipecat.services.google.tts import GoogleTTSService

    voice = settings["voice"]
    language = pipecat_language(google_language(voice)) or pipecat_language(session.language)
    return GoogleTTSService(
        credentials=settings.get("credentials", ""),
        location=settings["location"],
        settings=GoogleTTSService.Settings(voice=voice, language=language or Language.EN_US),
    )


SYNTHESIS = (
    Service(
        "kokoro",
        "Kokoro",
        (
            Setting(
                "voice",
                "Voice",
                "A Kokoro voice, named after its language: "
                "af_heart, bf_emma, zf_xiaobei, jf_alpha…",
                default="af_heart",
            ),
        ),
        make=_kokoro_tts,
        model=lambda _s: kokoro_model(),
        languages=lambda s: [kokoro_language(s["voice"])] if kokoro_language(s["voice"]) else [],
    ),
    Service(
        "openai",
        "OpenAI-compatible",
        (
            *OPENAI_SETTINGS,
            Setting("model", "Model", default="gpt-4o-mini-tts"),
            Setting("voice", "Voice", default="alloy"),
        ),
        make=_openai_tts,
    ),
    Service(
        "deepgram",
        "Deepgram",
        (_api_key(), Setting("voice", "Voice", default="aura-2-helena-en")),
        make=_deepgram_tts,
    ),
    Service(
        "cartesia",
        "Cartesia",
        (
            _api_key(),
            Setting("model", "Model", default="sonic-3"),
            Setting("voice", "Voice", "The voice's ID"),
        ),
        make=_cartesia_tts,
    ),
    Service(
        "elevenlabs",
        "ElevenLabs",
        (
            _api_key(),
            Setting("model", "Model", default="eleven_flash_v2_5"),
            Setting("voice", "Voice", "The voice's ID"),
        ),
        make=_elevenlabs_tts,
    ),
    Service(
        "piper",
        "Piper",
        (
            Setting(
                "voice",
                "Voice",
                "A Piper voice, named after its language: "
                "en_US-lessac-medium, zh_CN-huayan-medium, de_DE-thorsten-medium…",
                default="en_US-lessac-medium",
            ),
        ),
        make=_piper_tts,
        model=lambda s: piper_model(s["voice"]),
        languages=lambda s: [piper_language(s["voice"])] if piper_language(s["voice"]) else [],
    ),
    Service(
        "soniox",
        "Soniox",
        (
            _api_key(),
            Setting("model", "Model", default="tts-rt-v2"),
            Setting("voice", "Voice", default="Bryce"),
        ),
        make=_soniox_tts,
    ),
    Service(
        "google",
        "Google Cloud",
        (
            *_GOOGLE_SETTINGS,
            Setting(
                "voice",
                "Voice",
                "A Chirp 3 HD voice, named after its language: "
                "en-US-Chirp3-HD-Charon, cmn-CN-Chirp3-HD-Aoede…",
                default="en-US-Chirp3-HD-Charon",
            ),
        ),
        make=_google_tts,
        languages=lambda s: [google_language(s["voice"])] if google_language(s["voice"]) else [],
    ),
)

TRANSCRIPTION = (
    Service(
        CONTAINER,
        "Speaches",
        WHISPER_SETTINGS,
        model=lambda s: whisper_model(s["model"]),
    ),
    Service(
        "openai",
        "OpenAI-compatible",
        (*OPENAI_SETTINGS, Setting("model", "Model", default="whisper-1")),
    ),
)


@dataclass
class Catalog:
    """The services by kind and name."""

    recognition: dict[str, Service] = field(
        default_factory=lambda: {s.name: s for s in RECOGNITION}
    )
    synthesis: dict[str, Service] = field(default_factory=lambda: {s.name: s for s in SYNTHESIS})
    transcription: dict[str, Service] = field(
        default_factory=lambda: {s.name: s for s in TRANSCRIPTION}
    )

import json
from pathlib import Path
from typing import Any, cast

from fakes import FakeSpeaches, google_credentials
from pipecat.services.openai.stt import OpenAISTTService
from pipecat.services.soniox.stt import SonioxContextObject
from pipecat.services.stt_service import STTService
from pipecat.services.tts_service import TTSService
from pipecat.transcriptions.language import Language

from mavio.plugin.v1 import settings_pb2
from mavio_pipecat.models import Models
from mavio_pipecat.services import (
    RECOGNITION,
    SYNTHESIS,
    TRANSCRIPTION,
    Session,
    Setting,
    google_language,
    kokoro_language,
    pipecat_language,
    piper_language,
)


def test_every_service_makes_its_pipecat_service(tmp_path: Path):
    """Each service is made of its defaults and a key, without reaching
    it; Kokoro and Piper, whose models are not downloaded, are not made."""
    models = Models(tmp_path, FakeSpeaches())
    session = Session(language="zh-CN", hints=("星际穿越", "Inception"))
    keys = {"api_key": "k", "credentials": google_credentials()}
    for service in RECOGNITION:
        assert service.make is not None
        stt = service.make(service.values(keys), session, models)
        assert isinstance(stt, STTService), f"{service.name} made {stt!r}"
    for service in SYNTHESIS:
        assert service.make is not None
        if not models.ready(service.model(service.values({}))):
            assert service.name in ("kokoro", "piper"), f"{service.name} needs a model"
            continue
        tts = service.make(service.values({**keys, "voice": "v"}), session, models)
        assert isinstance(tts, TTSService), f"{service.name} made {tts!r}"
    names = [s.name for s in TRANSCRIPTION]
    assert names == ["speaches", "openai"], f"transcription services = {names}"


def test_whisper_takes_the_language_and_hints(tmp_path: Path):
    service = RECOGNITION[0]
    assert service.name == "speaches" and service.make is not None
    stt = service.make(
        service.values({}),
        Session(language="zh-Hans", hints=("星际穿越",)),
        Models(tmp_path, FakeSpeaches()),
    )
    assert isinstance(stt, OpenAISTTService)
    settings = stt._settings  # pyright: ignore[reportPrivateUsage]
    got = (str(stt._client.base_url), settings.model, settings.language, settings.prompt)  # pyright: ignore[reportPrivateUsage]
    want = (
        "http://speaches:8000/v1/",
        "deepdml/faster-whisper-large-v3-turbo-ct2",
        Language.ZH,
        "星际穿越",
    )
    assert got == want, f"Whisper = {got}, want = {want}"


def test_values_fill_the_defaults():
    service = RECOGNITION[1]
    got = service.values({"api_key": "k", "model": ""})
    want = {"base_url": "https://api.openai.com/v1", "api_key": "k", "model": "gpt-4o-transcribe"}
    assert got == want, f"values = {got}, want = {want}"


def test_languages():
    assert pipecat_language("zh-CN") == Language.ZH_CN
    assert pipecat_language("zh-Hans") == Language.ZH
    assert pipecat_language("") is None
    assert [kokoro_language(v) for v in ["af_heart", "zf_xiaobei", "xx"]] == ["en-US", "zh", ""]
    got = [piper_language(v) for v in ["zh_CN-huayan-medium", "en_GB-alan-low", "v"]]
    assert got == ["zh-CN", "en-GB", ""], f"Piper's = {got}"
    got = [google_language(v) for v in ["cmn-CN-Chirp3-HD-Aoede", "en-US-Chirp3-HD-Charon", "v"]]
    assert got == ["zh-CN", "en-US", ""], f"Google's = {got}"


def test_hints_go_to_the_services_that_take_them(tmp_path: Path):
    models = Models(tmp_path)
    session = Session(language="en-US", hints=("Severance",))
    services = {s.name: s for s in RECOGNITION}
    got: dict[str, Any] = {}
    for name, field, settings in [
        ("assemblyai", "keyterms_prompt", {}),
        ("cartesia", "keyterm", {"model": "ink-2"}),
        ("soniox", "context", {}),
        ("google", "adaptation", {"credentials": google_credentials()}),
    ]:
        s = services[name]
        assert s.make is not None and s.hints, f"{name} takes no hints"
        stt = cast(Any, s.make(s.values({"api_key": "k", **settings}), session, models))
        got[name] = getattr(stt._settings, field)
    # Google's is made a SpeechAdaptation.
    got["google"] = [p.value for p in got["google"].phrase_sets[0].inline_phrase_set.phrases]
    want = {
        "assemblyai": ["Severance"],
        "cartesia": ["Severance"],
        "soniox": SonioxContextObject(terms=["Severance"]),
        "google": ["Severance"],
    }
    assert got == want, f"hints = {got}, want = {want}"
    s = services["cartesia"]
    assert s.make is not None
    stt = cast(Any, s.make(s.values({"api_key": "k"}), session, models))
    assert stt._settings.keyterm is None, "ink-whisper given key terms"


def test_setting_proto():
    s = Setting(
        "model", "Model", type="select", default="tiny", options=(("tiny", "Tiny"),), secret=True
    )
    p = s.proto()
    assert p.type == settings_pb2.SETTING_FIELD_TYPE_SELECT
    assert (json.loads(p.default_json), p.secret) == ("tiny", True)
    assert [(json.loads(o.value_json), o.label) for o in p.options] == [("tiny", "Tiny")]


def test_leaves_out_what_is_not_packaged(tmp_path: Path):
    """resampy, numba and sympy are excluded from the plugin's
    dependencies, and faster-whisper is not one: nothing imports them."""
    import importlib
    import sys

    for module in ("mavio_pipecat.pipeline", "mavio_pipecat.plugin"):
        importlib.import_module(module)
    models = Models(tmp_path, FakeSpeaches())
    for service in RECOGNITION:
        assert service.make is not None
        service.make(
            service.values({"api_key": "k", "credentials": google_credentials()}), Session(), models
        )
    imported = [
        m for m in ("resampy", "numba", "llvmlite", "sympy", "faster_whisper") if m in sys.modules
    ]
    assert imported == [], f"imported = {imported}"

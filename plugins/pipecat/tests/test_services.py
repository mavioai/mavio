import json
from pathlib import Path

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
    kokoro_language,
    pipecat_language,
)
from mavio_pipecat.whisper import WhisperRecognizer


def test_every_service_makes_its_pipecat_service(tmp_path: Path):
    """Each service is made of its defaults and a key, without reaching
    it; Kokoro, whose model is not downloaded, is not made."""
    models = Models(tmp_path)
    session = Session(language="zh-CN", hints=("星际穿越", "Inception"))
    for service in RECOGNITION:
        assert service.make is not None
        stt = service.make(service.values({"api_key": "k"}), session, models)
        assert isinstance(stt, STTService), f"{service.name} made {stt!r}"
    for service in SYNTHESIS:
        assert service.make is not None
        if not models.ready(service.model(service.values({}))):
            assert service.name == "kokoro", f"{service.name} needs a model"
            continue
        tts = service.make(service.values({"api_key": "k", "voice": "v"}), session, models)
        assert isinstance(tts, TTSService), f"{service.name} made {tts!r}"
    names = [s.name for s in TRANSCRIPTION]
    assert names == ["whisper", "openai"], f"transcription services = {names}"


def test_whisper_takes_the_language_and_hints(tmp_path: Path):
    service = RECOGNITION[0]
    assert service.name == "whisper" and service.make is not None
    stt = service.make(
        service.values({}), Session(language="zh-Hans", hints=("星际穿越",)), Models(tmp_path)
    )
    assert isinstance(stt, WhisperRecognizer)
    assert (stt._language, stt._prompt) == ("zh", "星际穿越")  # pyright: ignore[reportPrivateUsage]


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


def test_setting_proto():
    s = Setting(
        "model", "Model", type="select", default="tiny", options=(("tiny", "Tiny"),), secret=True
    )
    p = s.proto()
    assert p.type == settings_pb2.SETTING_FIELD_TYPE_SELECT
    assert (json.loads(p.default_json), p.secret) == ("tiny", True)
    assert [(json.loads(o.value_json), o.label) for o in p.options] == [("tiny", "Tiny")]

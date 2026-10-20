from pathlib import Path

from mavio_pipecat.models import Models, kokoro_model, whisper_model


def test_ready(tmp_path: Path):
    models = Models(tmp_path)
    assert models.ready(None)
    assert not models.ready(kokoro_model())
    assert not models.ready(whisper_model("tiny"))
    files = models.kokoro()
    files.model.parent.mkdir(parents=True)
    files.model.write_bytes(b"x")
    assert not models.ready(kokoro_model()), "ready with one of its two files"
    files.voices.write_bytes(b"x")
    assert models.ready(kokoro_model())


async def test_download_skips_what_is_ready(tmp_path: Path):
    models = Models(tmp_path)
    files = models.kokoro()
    files.model.parent.mkdir(parents=True)
    files.model.write_bytes(b"x")
    files.voices.write_bytes(b"x")
    assert await models.download([kokoro_model(), kokoro_model()]) == []

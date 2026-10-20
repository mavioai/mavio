import pytest

from mavio_pipecat.config import Engine, parse_config


def test_parse_config():
    c = parse_config(
        '{"recognizers": [{"id": "a", "service": "deepgram", "name": "DG",'
        ' "settings": {"api_key": "k"}}],'
        ' "transcribers": [{"id": "b", "service": "speaches", "enabled": false}]}'
    )
    assert c.recognizers == (Engine("a", "deepgram", "DG", True, {"api_key": "k"}),)
    assert c.voices == ()
    assert c.transcribers == (Engine("b", "speaches", "", False, {}),)
    assert parse_config("").recognizers == ()


def test_parse_config_refuses():
    for bad in [
        "[]",
        '{"voices": {}}',
        '{"voices": [1]}',
        # A recognition service is not a voice.
        '{"voices": [{"id": "a", "service": "speaches"}]}',
        '{"transcribers": [{"id": "a", "service": "deepgram"}]}',
        '{"recognizers": [{"id": "a", "service": "openai", "settings": []}]}',
    ]:
        with pytest.raises(ValueError):
            parse_config(bad)

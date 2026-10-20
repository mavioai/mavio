"""The plugin's settings (docs/plugins.md §8.16): the recognizers, voices
and transcribers the administrator configured, each from a service the
plugin bundles, with an ID the server generates."""

import json
from dataclasses import dataclass, field
from typing import Any, cast

from mavio_pipecat.services import Catalog, Service


@dataclass(frozen=True)
class Engine:
    """A recognizer, voice or transcriber: a service with its settings."""

    id: str
    service: str
    name: str = ""
    enabled: bool = True
    settings: dict[str, Any] = field(default_factory=lambda: {})


@dataclass(frozen=True)
class Config:
    recognizers: tuple[Engine, ...] = ()
    voices: tuple[Engine, ...] = ()
    transcribers: tuple[Engine, ...] = ()


def _engines(raw: object, services: dict[str, Service], kind: str) -> tuple[Engine, ...]:
    if raw is None:
        return ()
    if not isinstance(raw, list):
        raise ValueError(f"{kind} is not a list")
    out: list[Engine] = []
    for item in cast(list[object], raw):
        if not isinstance(item, dict):
            raise ValueError(f"a {kind} entry is not an object")
        e = cast(dict[str, Any], item)
        service = e.get("service")
        if service not in services:
            raise ValueError(f"{kind}: unknown service {service!r}")
        settings: object = e.get("settings")
        if settings is None:
            settings = {}
        if not isinstance(settings, dict):
            raise ValueError(f"{kind}: the settings of {service} are not an object")
        out.append(
            Engine(
                id=str(e.get("id", "")),
                service=service,
                name=str(e.get("name", "")),
                enabled=bool(e.get("enabled", True)),
                settings=cast(dict[str, Any], settings),
            )
        )
    return tuple(out)


def parse_config(config_json: str, catalog: Catalog | None = None) -> Config:
    """The settings of their JSON; fails on a service the plugin does not
    bundle."""
    catalog = catalog or Catalog()
    raw = json.loads(config_json or "{}")
    if not isinstance(raw, dict):
        raise ValueError("the configuration is not an object")
    c = cast(dict[str, Any], raw)
    return Config(
        recognizers=_engines(c.get("recognizers"), catalog.recognition, "recognizers"),
        voices=_engines(c.get("voices"), catalog.synthesis, "voices"),
        transcribers=_engines(c.get("transcribers"), catalog.transcription, "transcribers"),
    )

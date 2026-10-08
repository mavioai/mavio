# fixtures

> English | [简体中文](README.zh-CN.md)

Deterministically generates test media with `ffmpeg -f lavfi` (`testsrc2`,
`sine`, …): multiple containers and codecs, HDR10 / Dolby Vision metadata,
multiple audio and subtitle tracks, interlaced sources, odd durations.

Generated files go to `.fixtures/` (git-ignored) and are never committed.
Tests that need them must skip with a clear message when they are missing.

Status: not implemented yet (P0).

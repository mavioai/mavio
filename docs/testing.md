# Mavio Testing Strategy

> English | [简体中文](testing.zh-CN.md)

> Related: [Architecture](architecture.md) · [Development](development.md) · [Roadmap](roadmap.md) · [AGENTS.md](../AGENTS.md)

---

## 1. Test Layers

| Layer | Scope | Where | Runs in |
| :--- | :--- | :--- | :--- |
| Unit tests | Pure logic; no network, no real ffmpeg, no hardware. Table-driven, many ported from Jellyfin | `*_test.go` next to the code, data in `testdata/` | Every CI run, all platforms |
| Conformance tests | The same repository test suite on SQLite (in-memory / temp file) and PostgreSQL (testcontainers-go); both must pass | `libs/store` | Every CI run (Linux) |
| Integration tests | Real ffmpeg / ffprobe, generated test media, real hardware encoders; call `t.Skip` with a reason when prerequisites are missing | Next to the code, behind prerequisites | CI where prerequisites exist; hardware runners for vendor paths |
| Smoke tests | Wire the libraries completed so far together inside the server (see [Roadmap §1](roadmap.md#1-principles)) | `apps/server/internal/smoke` | Every CI run |

Concurrency logic involving timeouts, timers or idle reaping is tested deterministically with `testing/synctest`.

---

## 2. Porting Test Cases from Jellyfin

### 2.1 Method
1. `tools/testport` extracts parameterized cases such as `[InlineData]`, `[MemberData]` and `TheoryData` from Jellyfin's C# tests, writes them out as JSON / YAML, and places them in the corresponding library's `testdata/` directory. It is a one-off migration tool; the output is reviewed by a human afterwards.
2. Non-parameterized tests (logic assertions) are translated by hand into Go table-driven tests.
3. Test assets (NFO samples, ffprobe JSON, subtitle files, keyframe data) are copied into the corresponding library's `testdata/`, named by purpose (e.g. `testdata/nfo/`, `testdata/probe/`). Every case file or asset records its origin in its metadata (file path and test name in the Jellyfin repository); no directory in the repository is named after Jellyfin.
4. Cases that depend on .NET-specific behavior, or that conflict with Mavio's design decisions, are marked `skip` in testdata with a reason; **silently dropping cases is not allowed**.
5. Once porting is complete, nothing depends on a Jellyfin runtime; new cases are written directly in Mavio.

### 2.2 Mapping
| Jellyfin tests | Assets and focus | Mavio target |
| :--- | :--- | :--- |
| `tests/Jellyfin.Naming.Tests` (TV / Video / Music / AudioBook / Book / ExternalFiles, ~575 parameterized cases) | Series / seasons / multi-episode / absolute numbering / date-based episodes; multiple versions, stacks, extras, 3D, date cleaning; external files | `libs/naming` |
| `tests/Jellyfin.Server.Implementations.Tests/Library` | Movie, series, season and audio resolvers; ignore rules (`.ignore`); sorting; media stream selection (`MediaStreamSelectorTests`) | `libs/library` (resolver chain, ignore rules); `libs/media/decision` (stream selection) |
| `tests/Jellyfin.XbmcMetadata.Tests` (Parsers + Test Data) | NFO samples for movies, series, seasons, episodes, music albums, artists, music videos | `libs/metadata/nfo` |
| `tests/Jellyfin.Providers.Tests` (Tmdb / MediaInfo / Lyrics / Music / TV / ExternalId) | TMDB utilities and cast handling, missing-episode detection, lyrics parsing, external IDs | `plugins/scraper-tmdb`, `libs/metadata` |
| `tests/Jellyfin.MediaEncoding.Tests/Probing` (ffprobe JSON samples) | Probe result normalization: missing bitrates, interlacing, duration precision and other edge cases | `libs/media/probe` |
| `tests/Jellyfin.MediaEncoding.Tests/Subtitles` | SRT / ASS / SSA parsing and encoding | `libs/subtitle` |
| `tests/Jellyfin.MediaEncoding.Tests` (EncoderValidator, ApplePlatformHelper) | ffmpeg version parsing, encoder capability parsing, Apple platform detection | `libs/media/hwaccel` |
| `tests/Jellyfin.Controller.Tests/MediaEncoding` (`EncodingHelper*Tests`) | Transcode argument derivation, audio bitstream filters, Dolby Vision handling, audio encoder inference | `libs/media/planner` |
| `tests/Jellyfin.Model.Tests/Dlna` (`StreamBuilderTests` + 19 `DeviceProfile-*.json`) | Direct play / remux / transcode decisions; profiles converted into `ClientCapabilities` | `libs/media/decision` |
| `tests/Jellyfin.MediaEncoding.Keyframes.Tests` | Keyframe extraction samples | `libs/media/keyframes` |
| `tests/Jellyfin.MediaEncoding.Hls.Tests` | Keyframe-based dynamic HLS playlist generation | `libs/streaming` |
| `tests/Jellyfin.Drawing.Skia.Tests` | Resize dimension calculation, sharpening, SVG security validation | `libs/imaging` |
| `tests/Jellyfin.Server.Implementations.Tests/Trickplay` | Trickplay generation parameters | `libs/imaging` / `libs/media` |
| `tests/Jellyfin.Extensions.Tests`, `Jellyfin.Common.Tests` | Generic string and path utilities | Merged into the relevant libraries as needed |
| `Jellyfin.Api.Tests`, `Jellyfin.Server.Integration.Tests` | HTTP API behavior | Not ported for now (to be evaluated together with the shim later) |

---

## 3. Test Media

`tools/fixtures` deterministically generates test media with `ffmpeg -f lavfi` (`testsrc2`, `sine`): a catalog of short clips covering multiple containers and codecs (H.264, HEVC Main10, MPEG-2, VP9, AV1, AAC, AC-3, MP2, Opus, FLAC), HDR10 signaling and metadata, interlacing, multiple audio and subtitle tracks with languages and dispositions, chapters, music tags and non-integer frame rates and durations.

* `pnpm nx run fixtures:generate` writes the catalog to `.fixtures/` at the repository root (git-ignored; override with `MAVIO_FIXTURES`). Binary media files are never committed.
* Generation is incremental: `.fixtures/manifest.json` records a key per fixture (a hash of the ffmpeg version, arguments and auxiliary files), and only missing or changed fixtures are regenerated.
* Fixtures whose encoders the local ffmpeg build lacks are skipped with a warning.
* Tests get a fixture path with `fixtures.Require(t, "<name>")` from `github.com/mavioai/mavio/tools/fixtures`, which skips the test when the fixture has not been generated.
* `go run ./cmd/fixtures -list` (in `tools/fixtures`) lists the catalog.


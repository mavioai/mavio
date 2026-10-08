# Mavio Testing Strategy

> English | [简体中文](testing.zh-CN.md)

> Related: [Architecture](architecture.md) · [Domain Model](domain.md) · [Development](development.md) · [Roadmap](roadmap.md) · [AGENTS.md](../AGENTS.md)

---

## 1. Test Layers

| Layer | Scope | Where | Runs in |
| :--- | :--- | :--- | :--- |
| Unit tests | Pure logic; no network, no real ffmpeg, no hardware. Table-driven, many ported from Jellyfin | `*_test.go` next to the code, data in `testdata/` | Every CI run, all platforms |
| Conformance tests | The same repository test suite on SQLite (temporary file) and PostgreSQL (a testcontainers-go container, or the server in `MAVIO_TEST_POSTGRES_DSN`; skipped without Docker or with `-short`); both must pass | `libs/store` | Every CI run (PostgreSQL on Linux) |
| Integration tests | Real ffmpeg / ffprobe, generated test media, real hardware encoders; call `t.Skip` with a reason when prerequisites are missing | Next to the code, behind prerequisites | CI where prerequisites exist; vendor paths only on the hardware available (maintainers' machines, GitHub-hosted runners) |
| Smoke tests | Wire the libraries completed so far together inside the server (see [Roadmap §1](roadmap.md#1-principles)) | `apps/server/internal/smoke` | Every CI run |

Concurrency logic involving timeouts, timers or idle reaping is tested deterministically with `testing/synctest`.

---

## 2. Porting Test Cases from Jellyfin

### 2.1 Method
`tools/testport` performs the extraction; `tools/testport/mapping.json` maps Jellyfin test projects and test data directories to Mavio targets (the table in §2.2).

1. **Parameterized cases**: `[InlineData]` cases, and `[MemberData]` cases backed by `TheoryData.Add(…)`, `yield return` or collection initializers, are written as one JSON file per C# test class under the target's `testdata/cases/`, named in snake_case without the `Tests` suffix (e.g. `TV/EpisodeNumberTests.cs` → `testdata/cases/tv/episode_number.json`). Arguments are keyed by parameter name, with declared defaults filled in. Constants become JSON values; enum members and other symbols become `{"$symbol": …}`, flag combinations `{"$flags": […]}`, constructor calls `{"$new": …}`, and anything that is not a constant keeps its source text as `{"$expr": …}`.
2. **Provenance**: every case file records the Jellyfin repository, commit and source file; every case records its method, line and stable ID (`Method/n`).
3. **Jellyfin's own skips**: cases skipped in Jellyfin (`Skip = …`) and `[InlineData]` lines that Jellyfin commented out (typically `// TODO:` / `// FIXME:` known failures) are kept with a `skip` reason.
4. **Generated files are never edited by hand.** Cases that depend on .NET-specific behavior or conflict with Mavio's design are skipped in the Go test by case ID, with a reason; **silently dropping cases is not allowed**.
5. **Non-parameterized tests** (`[Fact]`) are listed in each case file under `facts` and translated by hand into Go tests; `[MemberData]` that is computed at runtime and `[ClassData]` are listed under `unsupported` with a reason and ported by hand.
6. **Test assets** (NFO samples, ffprobe JSON, subtitle files, keyframe data, device profiles) are copied into the target's `testdata/`, in subdirectories named by purpose (e.g. `testdata/nfo/`, `testdata/probe/`), with a `SOURCES.json` recording each file's origin. No directory in the repository is named after Jellyfin.
7. Run it per target as each library is implemented: `go run ./cmd/testport -jellyfin <jellyfin checkout> -only libs/naming` (in `tools/testport`). Once a library is ported, new cases are written directly in Mavio.

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
| `tests/Jellyfin.Controller.Tests/Entities/BaseItemTests.cs` | Sort names (`GetSortName`, `ModifySortChunks`) | `libs/store` |
| `tests/Jellyfin.Extensions.Tests/StringExtensionsTests.cs` | Diacritics removal used by search keys | `libs/store` |
| `tests/Jellyfin.Extensions.Tests`, `Jellyfin.Common.Tests` (other classes) | Generic string and path utilities | Merged into the relevant libraries as needed |
| `Jellyfin.Api.Tests`, `Jellyfin.Server.Integration.Tests` | HTTP API behavior | Not ported for now (to be evaluated together with the shim later) |

---

## 3. Test Media

`tools/fixtures` deterministically generates test media with `ffmpeg -f lavfi` (`testsrc2`, `sine`): a catalog of short clips covering multiple containers and codecs (H.264, HEVC Main10, MPEG-2, VP9, AV1, AAC, AC-3, MP2, Opus, FLAC), HDR10 signaling and metadata, interlacing, multiple audio and subtitle tracks with languages and dispositions, chapters, music tags and non-integer frame rates and durations.

* `pnpm nx run fixtures:media` writes the catalog to `.fixtures/` at the repository root (git-ignored; override with `MAVIO_FIXTURES`). Binary media files are never committed.
* Generation is incremental: `.fixtures/manifest.json` records a key per fixture (a hash of the ffmpeg version, arguments and auxiliary files), and only missing or changed fixtures are regenerated.
* Fixtures whose encoders the local ffmpeg build lacks are skipped with a warning.
* Tests get a fixture path with `fixtures.Require(t, "<name>")` from `github.com/mavioai/mavio/tools/fixtures`, which skips the test when the fixture has not been generated.
* `go run ./cmd/fixtures -list` (in `tools/fixtures`) lists the catalog.


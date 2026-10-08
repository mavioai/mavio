# Mavio Roadmap

> English | [简体中文](roadmap.zh-CN.md)

> Related: [Architecture](architecture.md) · [Development](development.md) · [Testing](testing.md) · [AGENTS.md](../AGENTS.md)

---

## 1. Principles

* **Bottom-up**: build the low-level libraries with no business dependencies first, then assemble layer by layer; each layer depends only on completed layers below it.
* **Definition of done**: when a phase's libraries are completed they must satisfy both:
  * **Correctness gate**: all ported test cases pass (or are explicitly marked skip with a reason), see [Testing §2](testing.md#2-porting-test-cases-from-jellyfin);
  * **Performance gate**: benchmarks meet the budgets listed in this document.
* **Performance budgets**: the numbers below are initial budgets, calibrated once P0 has the benchmark infrastructure in place and measurements from the reference hardware: Intel N100 (amd64) and RK3588 (arm64).
* **Integration smoke tests**: starting from P1, the end of each phase adds an integration test in `apps/server/internal/smoke` that wires the completed libraries together. It is not an MVP; it only exposes interface drift early, reducing the bottom-up approach's risk of "discovering problems only at final integration".

---

## 2. Phase Overview

```mermaid
flowchart TD
    P0["P0 Engineering foundation<br/>Nx / go.work / buf / mise / CI / benchmark infra / test media generation / testport"]
    P1["P1 Contracts, storage and plugin runtimes<br/>libs/proto, libs/core, libs/store, libs/plugin"]
    P2["P2 Pure computation libraries<br/>libs/naming, libs/subtitle, libs/metadata, libs/imaging"]
    P3["P3 Media pipeline<br/>libs/media: probe, keyframes, hwaccel, decision, planner, supervisor"]
    P4["P4 Scanning and first plugin<br/>libs/library, plugins/scraper-tmdb"]
    P5["P5 Streaming and API<br/>libs/streaming, Connect services"]
    P6["P6 Server assembly and distribution<br/>apps/server, single binary + jellyfin-ffmpeg, container images"]
    P7["Later<br/>libs/client, libs/ui, apps/web, apps/desktop, apps/mobile, Jellyfin shim evaluation"]

    P0 --> P1
    P1 --> P2
    P1 --> P3
    P2 --> P4
    P3 --> P5
    P4 --> P5
    P5 --> P6
    P6 --> P7
```

| Phase | Theme | Status |
| :--- | :--- | :--- |
| P0 | Engineering foundation | 🚧 In progress |
| P1 | Contracts, storage and plugin runtimes | Not started |
| P2 | Pure computation libraries | Not started |
| P3 | Media pipeline | Not started |
| P4 | Scanning and first plugin | Not started |
| P5 | Streaming and API | Not started |
| P6 | Server assembly and distribution | Not started |
| P7 | Clients and ecosystem | Not started |

---

## 3. Phase Details

### P0 Engineering Foundation
**Scope**: repository skeleton, mise, Nx, go.work, buf, golangci-lint + depguard; CI matrix (linux amd64/arm64, macOS, Windows); benchstat and continuous benchmarking; `tools/fixtures`, `tools/testport`.

**Correctness gate**: `nx affected` works; `tidy-check` (`GOWORK=off`) passes.

**Performance gate**: benchmark results are compared and shown on PRs.

**Progress**:
- [x] Go module skeleton and `go.work`; Nx with the local plugin `tools/nx-go` (infers projects and the dependency graph)
- [x] Toolchain versions pinned with mise
- [x] buf and the first contract `mavio.system.v1.SystemService`; `apps/server` serves a Connect health check
- [x] golangci-lint and depguard dependency-direction rules
- [x] CI workflow: affected-project checks, generated-code check, `buf breaking`, multi-platform test matrix (not yet run on GitHub)
- [ ] `tools/fixtures`: test media generation
- [ ] `tools/testport`: Jellyfin test case extraction
- [ ] `tools/bench`: benchmark orchestration, benchstat comparison and continuous benchmarking

### P1 Contracts, Storage and Plugin Runtimes
**Scope**: first version of the `proto` contracts; `core` domain model; `store` (ent + Atlas + sqlc, both dialects) and the job queue; `plugin` with both runtimes and the handshake.

**Correctness gate**: repository conformance tests pass on both SQLite and PostgreSQL; a crashing plugin does not affect the host and is restarted automatically.

**Performance gate**:
- Browse / filter query p99 on a 100k-item library: SQLite < 20ms, PostgreSQL < 30ms
- Bulk upsert ≥ 5k rows/s
- Empty WASM plugin call < 50µs, empty UDS call < 300µs

### P2 Pure Computation Libraries
**Scope**: `naming`, `subtitle`, `metadata` (NFO), `imaging`.

**Correctness gate**: all corresponding ported cases pass.

**Performance gate**:
- `naming` single parse < 10µs
- 1080p JPEG → 400px WebP on one core < 40ms (amd64)
- Bounded peak memory for image processing

### P3 Media Pipeline
**Scope**: `probe`, `keyframes`, `hwaccel`, `decision`, `planner`, `supervisor`.

**Correctness gate**: all ported StreamBuilder and EncodingHelper cases pass; real transcode smoke tests pass on each vendor path on reference hardware.

**Performance gate**:
- Decision + planning < 1ms per call
- Overhead from ffmpeg start to first segment < 50ms
- Zero leaked ffmpeg processes after sessions end

### P4 Scanning and First Plugin
**Scope**: `library` scanner (full reconciliation, see [Architecture §9](architecture.md#9-library-scanning-and-change-detection-libslibrary)), resolver chain, job scheduling; `plugins/scraper-tmdb` (WASM).

**Correctness gate**: ported library resolver cases pass; scan results are identical on both databases; reconciliation converges to the correct state for additions / deletions / renames / content changes / temporarily unavailable mount points.

**Performance gate**:
- Cold scan of 10k files (excluding remote scraping) < 60s
- No-change reconciliation < 2s (local SSD)
- RSS during scans < 150MB

### P5 Streaming and API
**Scope**: `streaming` (CMAF HLS, on-demand segmenting, seeking, segment cache); Connect services (library, playback, user, system).

**Correctness gate**: ported HLS cases pass; playback verified on real hls.js / AVPlayer / Media3 clients.

**Performance gate**:
- Remux first-segment TTFB < 300ms
- First segment after seek < 1s

### P6 Server Assembly and Distribution
**Scope**: `apps/server` assembly; `CGO_ENABLED=0` cross-compilation; container images bundling jellyfin-ffmpeg.

**Correctness gate**: end-to-end smoke test: scan → scrape → playback decision → HLS playback.

**Performance gate**:
- Idle RSS < 80MB with a 10k-item library
- Cold start < 500ms

### P7 Clients and Ecosystem
**Scope**: `libs/client`, `libs/ui`; `apps/web`, `apps/desktop`, `apps/mobile`; evaluation of a Jellyfin API compatibility shim. Concrete gates will be defined after P6.

---

## 4. Risks and Open Items

| Item | Description | When |
| :--- | :--- | :--- |
| Platforms on the wazero interpreter | SQLite and WASM codec performance degrades on armv7 / riscv64 | Measure in P1; enable pure-Go fallback build tags for those platforms if needed |
| Cost of dual-dialect sqlc | Hot queries need two copies of the SQL | Keep the ≤ 30-query budget; rely on conformance tests |
| Pure-Go image resizing performance | If the P2 budget is missed | Resize large images with ffmpeg ([Architecture §6.2](architecture.md#62-performance-strategy)) |
| SVG rasterization | Pure-Go options are incomplete | Evaluate a WASM build of resvg in P2, or only serve SVGs as-is |
| Hardware test coverage | Needs real Intel, NVIDIA, AMD, Apple and Rockchip hardware | Set up self-hosted CI runners or a hardware test pool before P3 |
| Go modules split too finely | Friction in dependency upgrades and tidying | Keep watching; merge modules when needed |

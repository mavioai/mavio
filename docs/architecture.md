# Mavio Architecture Blueprint

> English | [简体中文](architecture.zh-CN.md)

> Related: [Domain Model](domain.md) · [Development](development.md) · [Roadmap](roadmap.md) · [Testing](testing.md) · [AGENTS.md](../AGENTS.md)

> Mavio is a next-generation self-hosted media server with a Go server at its core. It draws on the domain knowledge, FFmpeg pipeline and test assets that Jellyfin has accumulated over many years, but it does not aim to be compatible with the Jellyfin ecosystem; every technology choice is the most modern and best-suited option available today.

---

## 1. Positioning and Principles

### 1.1 Goals
* **Lightweight and efficient**: low resident memory, fast cold starts, high scan throughput and low first-segment latency follow from the modern stack — a single native Go binary with no runtime VM, WASM sandboxing instead of heavyweight isolation, and ffmpeg hardware pipelines.
* **Single-binary delivery**: the server is a single `CGO_ENABLED=0` Go binary; its only external runtime dependency is `jellyfin-ffmpeg`.
* **Strongly typed contracts**: Protobuf + Connect-RPC is used for all internal and external APIs; future Web / Expo / Desktop clients generate their SDKs directly from the contracts.
* **Plugin isolation**: plugins never enter the host's address space. Two runtimes — a WASM (wazero) sandbox and a child process (UDS) — share one set of contracts.
* **Multiple databases**: SQLite and PostgreSQL are both first-class.

### 1.2 Non-goals (current phase)
* No compatibility with the Jellyfin client ecosystem and no Jellyfin data migration. A Jellyfin API compatibility shim will be evaluated separately later.
* No UI yet (Web / Mobile / Desktop), but the directory layout and contracts already reserve room for it.

### 1.3 Engineering Principles
1. **Bottom-up**: build the low-level libraries with no business dependencies first; each library passes its tests when completed, before the next layer is assembled on top.
2. **No CGO**: any dependency that introduces CGO needs a separate justification. When native capabilities are needed, prefer an *ffmpeg child process* or a *WASM module running on wazero*.
3. **Libraries are usable on their own**: every Go library is an independent module that can be `go test`ed / `go build`t without Nx; Nx only orchestrates and caches.
4. **Contract first**: `libs/proto` is the single source of truth for every cross-process and cross-language boundary.

### 1.4 License
Mavio is licensed under **GPL-3.0** (`GPL-3.0-only`). GPL-3.0 is compatible with Apache-2.0, so core dependencies such as wazero, ent, connect-go, Atlas and OpenTelemetry can be used. Third-party dependencies must have GPL-3.0-compatible licenses (MIT, BSD, ISC, Apache-2.0, MPL-2.0, LGPL, GPL-3.0); GPL-2.0-only is not allowed.

---

## 2. Technology Choices

| Area | Choice | Notes |
| :--- | :--- | :--- |
| Server language | **Go (latest stable, currently 1.27)** | Uses `iter.Seq` pipelines, `log/slog`, `os.Root` (media path sandboxing), `testing.B.Loop`, `testing/synctest`, the go.mod `tool` directive |
| Contracts and RPC | **Protobuf + buf + Connect** (`connect-go`; `protobuf-es` v2 / `connect-es` v2 on the TS side) | `buf lint` + `buf breaking` guard contract evolution; `protovalidate` for declarative validation |
| HTTP | Standard library `net/http` (1.22+ routing patterns) | Connect handlers mount directly; media streams use plain HTTP (Range / HLS) |
| ORM / queries | **ent + sqlc + Atlas** | See §5 |
| Databases | **SQLite + PostgreSQL** | SQLite driver `ncruces/go-sqlite3` (WASM + wazero, pure Go); PostgreSQL driver `pgx/v5` (through `database/sql`) |
| Audio / video | **jellyfin-ffmpeg** (external process) | Reuses its hardware driver patches, tone-mapping filters and low-power encoding support |
| Imaging | **Pure Go + wazero WASM codecs + ffmpeg** | See §6 |
| Streaming | **HLS + fMP4 (CMAF)**; direct play over HTTP Range | Natively supported by hls.js / Media3 / AVPlayer |
| Plugins | **wazero WASM + child processes (UDS everywhere)** | See §7 |
| File name rules | **Jellyfin's naming rules on `dlclark/regexp2`** | The rules use .NET regular expressions (lookaround, atomic groups), which `regexp2` runs unchanged; each match has a one-second timeout |
| Concurrency | `errgroup.SetLimit` + `iter.Seq` | |
| Change detection | **Full reconciliation scans** (scheduled, on startup, manual / API-triggered), pruned by directory mtime | See §9 |
| Background jobs | In-house database job queue (leases + retries) | Works on both SQLite and PostgreSQL |
| Observability | `log/slog` + OpenTelemetry (traces / metrics) | |

Engineering tooling — Nx, mise, buf, golangci-lint, CI — is described in [Development](development.md).

### 2.1 wazero as the Shared Native Capability Layer
wazero hosts three kinds of workloads: SQLite (ncruces), image codecs (§6) and WASM plugins (§7). All native C library capabilities are thus obtained through WASM, and the whole server stays pure Go and freely cross-compilable.

Note: wazero's compiler backend supports only amd64 / arm64; other architectures (e.g. armv7, riscv64) fall back to the interpreter. For those architectures a build tag switches to `modernc.org/sqlite` and pure-Go codec fallbacks, and the documentation marks them as degraded platforms.

---

## 3. Repository Structure

How the monorepo is built and worked on (Nx, Go module conventions, code generation, CI) is described in [Development](development.md).

### 3.1 Directory Structure
Libraries under `libs/` are named directly by domain; a single library can be both a Go module and an npm package (for example `libs/proto`).

```text
mavio/
├── go.work                      # Aggregates all Go modules
├── nx.json
├── package.json
├── pnpm-workspace.yaml
├── mise.toml
│
├── apps/
│   ├── server/                  # [Go] Server: cmd/mavio, dependency wiring, config, Connect service implementations
│   ├── web/                     # (later) React + Vite + TanStack Router / Query
│   ├── mobile/                  # (later) Expo
│   └── desktop/                 # (later) Tauri 2, reusing web; can embed the server as a sidecar
│
├── libs/
│   ├── proto/                   # [Go + TS] Contracts: *.proto sources, buf config, generated Go/TS code
│   ├── plugin/                  # [Go] Plugin SDK and host runtimes (wasm + child process)
│   ├── core/                    # [Go] Domain model and repository ports (see domain.md); no infrastructure dependencies
│   ├── store/                   # [Go] ent schema, sqlc queries, Atlas migrations, repository implementations
│   ├── naming/                  # [Go] File / directory name parsing
│   ├── metadata/                # [Go] NFO read/write, external IDs, metadata merge strategy
│   ├── subtitle/                # [Go] SRT / ASS / SSA / WebVTT subtitle parsing, conversion and character set detection
│   ├── imaging/                 # [Go] Image processing, collages, blurhash / thumbhash
│   ├── media/                   # [Go] Probing, keyframes, hardware acceleration, playback decisions, transcode planning, ffmpeg supervision
│   ├── library/                 # [Go] Scanner, resolver chain, job scheduling
│   ├── streaming/               # [Go] HLS (CMAF) playlists, on-demand segmenting, segment cache
│   ├── client/                  # (later) [TS] connect-es client wrapper and Query hooks
│   └── ui/                      # (later) [TS] Cross-platform components and design tokens
│
├── plugins/
│   ├── scraper-tmdb/            # [Go → wasm] TMDB metadata scraper
│   ├── scraper-musicbrainz/     # [Go → wasm] Music metadata scraper
│   ├── notifier-webhook/        # [Go → wasm] Webhook notifications
│   └── auth-ldap/               # [Go → child process] LDAP authentication (needs native long-lived TCP connections)
│
├── tools/
│   ├── nx-go/                   # Local Nx plugin: infers projects from go.mod and builds the dependency graph
│   ├── fixtures/                # Deterministic test media generation with ffmpeg lavfi (HDR10, interlacing, multiple audio and subtitle tracks)
│   └── testport/                # One-off tool that extracts cases from Jellyfin's C# tests into testdata
│
└── docs/
```

### 3.2 Dependency Direction
```text
apps/server ──▶ streaming ──▶ media ──▶ core
     │             │            ▲
     │             ▼            │
     ├──────▶ library ──▶ naming / metadata / subtitle / imaging ──▶ core
     │             │
     ├──────▶ store ──▶ core
     └──────▶ plugin ──▶ proto

plugins/* ──▶ plugin ──▶ proto
```
* `core` depends on no other library; `naming` and `subtitle` are pure-function libraries.
* `media` depends only on `core` and knows nothing about databases or plugins.
* Only `apps/server` does wiring. The dependency direction is enforced in CI by depguard rules.

---

## 4. Contract Design (libs/proto)

```text
libs/proto/
├── buf.yaml  buf.gen.yaml
├── mavio/
│   ├── library/v1/             # LibraryService, ItemService: libraries, items, media sources and streams, people, scans
│   ├── auth/v1/                # AuthService: first run, sign-in and sign-out, signed-in devices
│   ├── user/v1/                # UserService, UserDataService: accounts, policies, preferences, per-item state
│   ├── system/v1/              # SystemService: health, server information
│   ├── playback/v1/            # PlaybackService: client capabilities, playback decisions, progress reporting
│   └── plugin/v1/              # Plugin contract: PluginService (manifest, configuration, lifecycle),
│                               #   MetadataProviderService, AuthProviderService, NotifierService
├── gen/go/                     # Generated protobuf-go + connect-go (Go module)
└── gen/ts/                     # (later) Generated protobuf-es + connect-es (npm package)
```

* Client capabilities are expressed with Mavio's own `ClientCapabilities` (containers, codecs, levels, HDR capabilities, subtitle delivery methods, bandwidth), in place of Jellyfin's `DeviceProfile`. The Go type lives in `libs/media/decision`, which stays free of `libs/proto`; the `playback/v1` message mirrors it and `apps/server` converts between them. When porting the StreamBuilder tests, Jellyfin profiles are converted into this type.
* Media streams (direct play, HLS playlists and segments, images) do not go through Connect but over plain HTTP, to benefit from Range, caching and CDN semantics.
* API messages mirror the domain model ([Domain Model](domain.md)) but never expose file system internals such as image paths or password hashes.
* `plugin/v1` is published to plugin authors and is self-contained: it defines its own lean messages (`Lookup`, `Metadata`, `PersonCredit`, `RemoteImage`, …) instead of importing `library/v1`, so the two contracts can evolve independently.

---

## 5. Storage Layer (libs/store): ent + sqlc, Two Databases

### 5.1 Responsibilities
| ent | sqlc |
| :--- | :--- |
| **Single source of truth for the schema** (`internal/ent/schema`) | Dialect-specific SQL that ent cannot express portably: the job queue (`INSERT … ON CONFLICT … WHERE` deduplication, atomic leasing with `FOR UPDATE SKIP LOCKED` on PostgreSQL) |
| CRUD, batch upserts (`CreateBulk` + `ON CONFLICT`), relationship traversal | |
| Dynamic item queries: optional filters, recursive descendants (a recursive CTE both dialects support), user-data joins, sorting, paging | |

Item queries are built with ent rather than sqlc because their filters and sort orders combine dynamically; static SQL with optional parameters would defeat the query planner. sqlc queries are written once per dialect, so they stay limited to the cases above (budget ≤ 30 queries).

### 5.2 Generation Pipeline
```text
ent schema ──go generate──────────────────────▶ internal/ent (generated client)
     │
     └──migrategen -dialect sqlite / postgres──▶ migrations/<dialect>/*.sql + atlas.sum
                                                        │
                                   sqlc generate ◀──────┘ (reads migrations as schema)
                                        │
                                        ▼
                         internal/sqlcsqlite, internal/sqlcpg
```
* `internal/cmd/migrategen` writes versioned migrations: it replays the existing migration directory on an empty development database (in-memory SQLite, or a throwaway PostgreSQL container), diffs it against the ent schema with Atlas, and writes the difference as a new migration in Atlas format. SQLite migrations use standard double-quoted identifiers, which sqlc's parser requires.
* The two migration directories are versioned separately and reviewed by a human before committing; merged migrations are never edited.
* `migrategen` drops removed columns and indexes, and skips index changes that only reflect how PostgreSQL normalizes a partial index's predicate.
* The server applies pending migrations on startup (`store.Open`): each file runs in its own transaction and is recorded in `schema_migrations`. SQLite migrations that rebuild tables run with foreign-key enforcement turned off on their connection (the pragma has no effect inside a transaction) and are checked with `PRAGMA foreign_key_check` before committing.
* Derived search and sort keys carry a version (`keysVersion`); when it changes, `store.Open` recomputes the keys of all existing rows once and records the version in `schema_migrations`.

### 5.3 Dual-dialect Practices
* **Shared transactions**: `Store.InTx` begins a `*sql.Tx` and binds both an ent client (through a driver whose nested transactions are no-ops) and the sqlc queries to it, so repositories can be combined freely within one transaction.
* **Unified types**: `core.ID` implements `sql.Scanner` / `driver.Valuer` and is stored as `uuid` (PostgreSQL) or text (SQLite); sqlc `overrides` map it and the nullable columns to the same Go types in both packages, so the PostgreSQL adapter converts structs directly.
* **Multi-valued attributes** (genres, tags, studios, artists, album artists) live in an `item_values` table, so "matches any of" filters are portable and indexable.
* **Search and sorting** use keys computed in Go and matched with `LIKE`. `search_key` (items, people) and `value_key` (item values) hold Jellyfin's clean form of the name (`cleanValue`: no diacritics, lower case, punctuation as spaces, half-width), used for matching, relevance and grouping; `original_key` holds the lower-cased original title, matched by raw search patterns. `sort_key` holds the Jellyfin sort form of the sort name (articles and punctuation removed, numbers zero-padded, non-Latin text transliterated with go-unidecode); it orders names and matches the sort form of search terms. See domain model §9.2.
* **Case-insensitive uniqueness** (user names) uses a lower-cased `name_key` column with a unique index.
* **Times** are stored with microsecond precision (`timestamptz` on PostgreSQL, integer microseconds on SQLite) and returned in UTC.
* **Repository ports**: `store` exposes only the interfaces defined in `core`; ent and sqlc types never leave the package.
* **Conformance tests**: the same repository test suite runs on SQLite (temporary file) and PostgreSQL (a testcontainers-go container started by `libs/store/pgtest`, or `MAVIO_TEST_POSTGRES_DSN`); a change passes only if both pass.

### 5.4 SQLite Runtime Settings
WAL mode, `synchronous=NORMAL`, `busy_timeout`, foreign keys on. A single-connection writer pool runs writes and transactions with `_txlock=immediate` (the write lock is taken when a transaction begins, avoiding upgrade deadlocks); a separate pool serves reads. Time values use `_timefmt=unixepoch_micro`. `PRAGMA optimize` runs on close.

---

## 6. Image Processing (libs/imaging)

### 6.1 Requirements and Implementation
Requirements are derived from Jellyfin's `src/Jellyfin.Drawing.Skia`, `MediaBrowser.Controller/Drawing/ImageProcessingOptions.cs`, and the Trickplay and Photos modules.

| Capability | Mavio implementation |
| :--- | :--- |
| Decode JPEG / PNG / GIF / BMP | Go standard library + `x/image` |
| Decode WebP | `gen2brain/vpx` (a pure-Go port of libwebp) |
| Decode AVIF / HEIC / JPEG XL | `gen2brain/avif`, `heic`, `jpegxl` (compiled to WASM, running on wazero) |
| Encode WebP / JPEG / PNG | `gen2brain/vpx` for WebP, standard-library JPEG and PNG |
| Encode AVIF (optional) | `gen2brain/avif` |
| Resize / crop / fill (Width, Height, Max*, Fill*) | `x/image/draw` CatmullRom / ApproxBiLinear; large images are first downsampled by powers of two |
| EXIF auto-orientation | Lightweight EXIF parsing (only Orientation and a few other tags) |
| Sharpen, blur, background color, foreground layer | Hand-written convolution / Gaussian blur (parallelizable in tiles) + `image/draw` compositing |
| Unplayed count / played percentage badges | Rendered by clients |
| Library collages, splash screens (with text) | `image/draw` compositing + `go-text/typesetting` text layout, with a bundled Noto font subset covering CJK |
| Trickplay thumbnail sheets | ffmpeg: the `fps` + `scale` + `tile` filters output the sheet in one pass, with hardware decoding available |
| Video screenshots, chapter images | ffmpeg |
| Blurhash / Thumbhash | Pure Go (`bbrks/go-blurhash`, `go.n16f.net/thumbhash`): blurhash on a 32px thumbnail with Jellyfin's component counts, thumbhash on a 100px one |
| SVG | Checked for external references (`CheckSVG`: href, CSS `url()` and `@import`, nested data URIs, external or exploding entities) and served to clients as-is; SVGs are not rasterized |
| Reading image dimensions | `image.DecodeConfig` |

### 6.2 Performance Strategy
A media server's image workload is "poster-scale": individual images are small, results are cacheable, and the access pattern is "process once, hit many times". Performance comes mainly from the architecture:
1. **Derived-image cache**: a content-addressed cache keyed by (source content hash, processing parameters), with `singleflight` collapsing concurrent requests for the same image.
2. **Pre-generation during scans**: common sizes are generated during scans, so request paths mostly just read from the cache.
3. **All video-derived images go to ffmpeg**: trickplay, screenshots and chapter images are produced entirely inside ffmpeg, with hardware decoding available.
4. **Fallback**: if pure-Go resizing proves too slow for large images, they are resized with ffmpeg's `scale` / `zscale` instead.

---

## 7. Plugin System (libs/plugin)

### 7.1 Two Runtimes, One Contract
| | WASM plugins (wazero) | Child-process plugins (UDS) |
| :--- | :--- | :--- |
| Use cases | Lightweight request–response plugins: metadata scrapers, lyrics / subtitle providers, notifiers, naming rule extensions | Plugins needing native network protocols (LDAP), long-lived connections, heavy computation, native libraries or their own HTTP server |
| Distribution | A single `.wasm` file, built once for all platforms | One binary per platform |
| Isolation | Memory sandbox; capabilities are granted entirely by the host | OS process isolation; can be combined with cgroups / rlimits |
| Call overhead | In-process function call + one memory copy | One UDS round trip |
| Authoring languages | Go (`GOOS=wasip1` + `go:wasmexport`), TinyGo, Rust, etc. | Any language that can implement a Connect / gRPC service |

Both runtimes implement the same set of services defined in `libs/proto/mavio/plugin/v1` (`MetadataProvider`, `AuthProvider`, `Notifier`, …). The host calls them through a single `plugin.Host` interface and does not care which runtime a plugin runs in.

### 7.2 WASM Runtime
* **Calls are Connect requests**: the host's Connect clients use an HTTP transport that, instead of a network connection, writes the request (path, headers, body) as an envelope into guest memory and calls the module export `mavio_call(ptr, len) -> (ptr << 32 | len)`; `mavio_alloc` / `mavio_free` manage the shared buffers. Inside the guest, `guest/wasm` hands the request to the standard Connect handlers the plugin registered and returns the response envelope (status, headers, body). Plugin code is therefore identical for both runtimes.
* **Build mode**: plugins are WASI reactors (`GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared`); the host runs `_initialize` once per instance.
* **Host functions** (module `mavio`):
  * `http_fetch(ptr, len) -> (handle << 32 | len)` performs an HTTP request on the plugin's behalf if the manifest's `http_hosts` allow the host, and `http_read(handle, ptr)` copies the result into guest memory. The two-step form avoids calling back into the guest from a host function, which Go's wasip1 runtime does not support. The guest SDK wraps them in an ordinary `*http.Client` (`guest.HTTPClient()`).
  * Plugins log to stderr (and stdout), which the host forwards to its logger.
  * Configuration arrives through the `Configure` RPC rather than a host function.
* **File system**: nothing is mounted by default; directories listed in the manifest's `read_paths` are mounted read-only at the same paths.
* **Instances**: each plugin has its own wazero runtime and a pool of module instances (the number of concurrent calls). An instance whose call traps, panics, exits or exceeds the call timeout is discarded and replaced, so a failing call never affects the host or later calls. A configuration accepted by the plugin is replayed on every instance before its next call.
* **Resource limits**: memory page cap per instance; per-call timeouts (`WithCloseOnContextDone`); the compilation cache (`CompilationCache`) is persisted to disk, since compiling a module takes far longer than instantiating it.
* **Constraint**: Go under `wasip1` has no sockets, so all network access goes through `http_fetch`.

### 7.3 Child-process Runtime
* **Unix Domain Sockets everywhere**: natively supported on Linux / macOS, and on Windows via `AF_UNIX` since Windows 10 1803. The socket lives in a private directory (mode 0700) under a short base path, as socket paths are limited to about 100 bytes.
* **Handshake**: the host passes the socket path and a one-time token through `MAVIO_PLUGIN_SOCKET` / `MAVIO_PLUGIN_TOKEN`. Once listening, the plugin prints one JSON line on stdout (`{"mavio_plugin":1,"plugin_id":…}`); the host checks the protocol version and plugin ID. Every request carries the token as `Authorization: Bearer …`.
* **Transport**: Connect over the socket (HTTP/2 h2c), reusing the handlers / clients generated from `libs/proto`.
* **Supervision**: stdout and stderr are forwarded to the host's logger; periodic `Health` RPCs (three consecutive failures kill the process); after an exit the supervisor restarts the plugin with exponential backoff (1 s doubling up to one minute), holding calls until it is back.
* **Shutdown**: `Close` sends the `Shutdown` RPC (after which the guest SDK exits), then SIGTERM, then SIGKILL. On Linux the kernel also kills the plugin if the host dies (`Pdeathsig`).
* **Permissions**: process plugins have the operating system's network and file access; `http_hosts` and `read_paths` are enforced by the WASM runtime only, so capabilities that need neither should ship as WASM plugins.
* **Hot plugging**: installing, upgrading and uninstalling plugins never requires restarting the host.

### 7.4 Plugin SDK Structure
```text
libs/plugin/
├── manifest/        # manifest.json loading and validation, host permissions, configuration JSON Schema validation
├── host/            # Host side: Open (either runtime, checked against Describe), Configure, the Plugin interface
│   ├── wasm/        # wazero runtime, instance pool, http_fetch / http_read host functions
│   └── process/     # Child process, socket handshake, token, supervision and restart
├── guest/           # Plugin side: Handle (register Connect handlers), HTTPClient
│   ├── wasm/        # //go:build wasip1: module exports, HTTP through the host
│   └── process/     # Serve: listen on the socket, authenticate, handshake, exit after Shutdown
├── internal/        # abi (WASM envelopes), proc (process protocol), clients, testplugin
└── schema/          # (later) JSON Schema generation from Go structs for configuration forms
```
A plugin registers its handlers in `init` with `guest.Handle(pluginv1connect.New…ServiceHandler(impl))`; a `wasip1` build imports `guest/wasm`, a native build calls `process.Serve`. `internal/testplugin` is a complete example built for both runtimes.

### 7.5 Plugins in the Server
* **Plugin folder**: the server starts every folder of its plugin folder (`-plugin-dir`) that holds a `manifest.json`, through `libs/plugin/host`. A folder that cannot be read, a duplicate plugin ID or a plugin that fails to start is reported as failed and left out; the server runs on without it.
* **Configuration**: an administrator's configuration is stored by plugin ID ([Domain Model](domain.md) §10) and set through `SystemService.SetPluginConfig`, which validates it against the manifest's `config_schema`, delivers it with the `Configure` RPC and stores it once the plugin accepts it, without a restart. At startup each plugin gets its stored configuration again.
* **States**: a plugin is ready when it runs with its configuration or its schema accepts an empty one; unconfigured while it waits for a configuration its schema requires; failed when it could not start or rejected its stored configuration. `SystemService.ListPlugins` reports them, with the manifest.
* **Metadata providers**: every started metadata plugin is a provider of library refreshes; while it is not ready it knows nothing, so a plugin configured later takes part in the next refresh.

---

## 8. Media Pipeline (libs/media) and Hardware Acceleration

### 8.1 Subpackages
```text
libs/media/
├── probe/        # Runs ffprobe (JSON output) and normalizes it into core MediaInfo (video, audio, subtitle streams, HDR10 / HDR10+ / DV metadata)
├── keyframes/    # Keyframe extraction (packet-level keyframe flags via ffprobe), used to cut HLS segments on keyframe boundaries
├── hwaccel/      # Hardware capability detection: -hwaccels / -encoders listings + trial encodes, producing Capabilities
├── decision/     # Playback decision: MediaInfo × ClientCapabilities → DirectPlay / Remux / Transcode (per stream)
├── planner/      # Transcode planning: decision × hardware capabilities → filter graph IR → ffmpeg arguments
└── supervisor/   # ffmpeg process supervision: context cancellation, idle reaping, throttling, segment output tracking, progress parsing
```

### 8.2 Design Points
* **`decision` and `planner` are pure functions**: they read no files and start no processes, so they can be verified with table-driven ported tests.
* **Filter graph intermediate representation (IR)**: `planner` first produces a structured filter graph (nodes for decode / hwupload / scale / tonemap / overlay / encode, each carrying its pixel format and hardware context) and then serializes it into ffmpeg arguments. Tests can assert on the IR instead of on brittle command-line strings.
* **Split by hardware vendor**: inside `planner`, Intel, NVIDIA, AMD, Apple, Rockchip, V4L2 and software encoding each have their own strategy, avoiding single-file bloat like Jellyfin's `EncodingHelper.cs` (8,000+ lines). Porting reorganizes the structure around behavior and tests.
* **`supervisor`**: one goroutine group per transcode session; ffmpeg is terminated when the client disconnects or after an idle timeout; throttled according to client playback progress; timeout tests are written deterministically with `testing/synctest`.

### 8.3 Hardware Acceleration Support Matrix
| Platform | Decode / encode | Tone mapping | Subtitle burn-in | Notes |
| :--- | :--- | :--- | :--- | :--- |
| **Intel** QSV / VA-API | Fully hardware, zero-copy | `vpp_qsv` / `tonemap_vaapi` / OpenCL | `overlay_qsv` / `overlay_vaapi` | The main platform for NAS and mini PCs; top priority |
| **NVIDIA** NVENC / NVDEC | CUDA memory pipeline | `tonemap_cuda` | `overlay_cuda` | |
| **AMD** AMF / VA-API | AMF (D3D11) on Windows, VA-API (radeonsi) on Linux | OpenCL / Vulkan | `overlay_vaapi` / OpenCL | |
| **Apple** VideoToolbox | Hardware H.264 / HEVC | `tonemap_videotoolbox` (Metal) | `overlay_videotoolbox` | Running natively on macOS |
| **Rockchip** RKMPP | Hardware decode and encode | RGA / OpenCL | RGA overlay | ARM NAS and dev boards (RK3588, etc.) |
| **V4L2** M2M | Decode / encode | — | — | Raspberry Pi and similar; limited capabilities, mostly remux |
| **Software** | libx264 / libx265 / SVT-AV1 | `zscale` + `tonemapx` | `subtitles` / `overlay` | Fallback |

Supported operating systems: Linux (first-class, including container deployments with `/dev/dri` and `/dev/nvidia*` device mapping), Windows 10/11, macOS (Apple Silicon / Intel).

---

## 9. Library Scanning and Change Detection (libs/library)

Mavio discovers library changes with **full reconciliation scans**, using the same mechanism for local disks and network file systems (SMB / NFS):

* **Triggers**: on server startup, on a per-library schedule, manually by an administrator, and through the API (e.g. a callback when a download client finishes).
* **Directory mtime pruning**: the database records each directory's mtime and inode (file ID on Windows); directories where neither has changed skip `readdir`. Adding, removing or renaming a file updates the parent directory's mtime, so pruning cannot miss those changes. Content changes that do not touch directory entries are detected by comparing each file's size + mtime, so files in non-pruned directories are still stat'ed; when absolute certainty is required, a library can be configured to periodically run a full reconciliation with pruning disabled.
* **Resolution one folder at a time**: the resolver maps a folder and its entries, given its place in the library (library kind, and whether it lies in a series, season or artist), to the item the folder is (a movie in its own folder, a disc rip, a series, a season, an album, an audiobook), the items made of its files, and the subfolders to resolve next with their place. It follows Jellyfin's naming and resolver rules; extras are found from the owner's folder and its extras folders. Because a folder's result depends only on its listing and its place, folders that did not change need not be resolved again.
* **Reconciliation reads only metadata**: it walks directories and compares file stats; only new or changed files are sent to ffprobe and scraping.
* **Concurrency**: library roots and directory subtrees are walked concurrently (`errgroup.SetLimit`), with lower concurrency on network file systems.
* **Jobs**: scans, probes and metadata refreshes are durable jobs run by workers that lease them (`library.scan`, `media.probe`, `media.keyframes`, `item.refresh`, `image.placeholders`). A scan queues a probe for each new or changed media file. A probe queues a metadata refresh and, for video, a low-priority extraction of the keyframes HLS needs to cut copied video. A refresh applies providers' metadata and then the local NFO file, leaving locked fields alone. Artwork beside the media is found by Jellyfin's local image names (`poster`, `folder`, `<file>-poster`, `fanart-1`, `season01-poster`, `<episode>-thumb`, …; in folders shared by several videos only names prefixed with the file's) and comes before images an NFO file names; each kind of image comes from the most trusted source that has one. Images found again keep their ID, size and placeholders, and a placeholders job then measures new images and computes their blurhash and thumbhash. Scheduled scans enqueue the next one after the library's scan interval; requested scans run beside them, and scans of one library never overlap.
* **Consistency**: each scan records a "scan generation" in the database; after a scan completes, items not seen in that generation are marked missing (soft-deleted first, purged after a grace period), so a temporarily unavailable mount point cannot wipe out a whole library.

---

## 10. HLS Streaming (libs/streaming)

* **Playlists up front**: a playback's media playlist lists every segment of the whole media source before anything is transcoded (VOD), so clients can seek anywhere. Encoded video and audio are divided into segments of equal length, with keyframes forced at the boundaries; copied video can only be cut at its keyframes, so its segments are cut at the first keyframe at or after each multiple of the segment length, using the keyframes stored with the media source. The master playlist announces the output's RFC 6381 codec strings, resolution, frame rate and dynamic range (`VIDEO-RANGE`), with kept Dolby Vision or HDR10+ as `SUPPLEMENTAL-CODECS`.
* **Segments on demand**: one ffmpeg run per stream writes segments ahead of the client. A request for a segment that is not written yet waits for the run, or restarts ffmpeg at that segment when it lies before where the run started or more than 24 seconds beyond what it has written. Written segments are kept for the playback session and served again, so seeking back costs nothing. Requests to one stream are served one at a time.
* **Restarts that line up**: ffmpeg's HLS muxer spaces its cuts from where a run starts, which would misplace copied video's cuts after a seek. Copied video is therefore written one group of pictures per file, numbered by its keyframe, and a segment is served as its consecutive files; a run starting anywhere writes the same files. Restarts read copied video from the middle of the segment's first group of pictures, because ffmpeg moves seeks back by a few frames for streams with reordered frames. Encoded runs start exactly at a segment boundary, so their cuts line up by themselves.
* **One timeline**: segments carry the source's timestamps from where it starts, continued from the start position after a seek (`-output_ts_offset`), as ffmpeg also times subtitles extracted for delivery as files. They may be negative (`-avoid_negative_ts disabled`, with `frag_discont` in fMP4): video with reordered frames and audio with encoder priming are not shifted later, so the first frame plays at zero, in sync with the subtitles.
* **Safe replacement**: ffmpeg writes every file under a temporary name and renames it when complete (`-hls_flags temp_file`): a file under its final name is complete, and a later run replacing it never truncates what a client is reading. Each run writes its own initialization segment; the first one is kept and served.

---

## 11. API and Authentication (apps/server)

* **Assembly**: `apps/server/internal/server` wires storage, plugins, playback, images, the library worker and the handler tree, and runs them until shutdown; `cmd/mavio` only parses flags into its configuration, and the end-to-end test runs the same assembly.
* **One handler tree**: `apps/server/internal/httpserver` mounts the Connect services (implemented in `internal/rpc`) and the plain HTTP media endpoints on one `http.ServeMux`. Every Connect request is checked against its protovalidate rules before it reaches the service; violations are `invalid_argument`.
* **Accounts**: passwords are hashed with argon2id (19 MiB, 2 passes, 1 lane) into a PHC string; a hash with other parameters is still accepted and replaced at the next successful sign-in. Signing in with an unknown name costs the same as a wrong password. Users that authenticate through a plugin have no password hash.
* **First run**: while no user exists, `AuthService.CreateFirstUser` creates an administrator without credentials and signs it in; afterwards it fails with `failed_precondition`. `AuthService.GetAuthInfo` tells clients whether this step is pending.
* **Sessions**: signing in issues an access token of 32 random bytes (base64url) for one device; the server stores only its SHA-256 hash as an `AuthSession` ([Domain Model](domain.md)). Clients send it as `Authorization: Bearer <token>`. Tokens do not expire; they end when the client signs out, the user revokes the session or deletes the account, or the same device signs in again. A session's last activity is recorded at most once a minute.
* **Authorization**: an interceptor resolves the token to the user and session and puts them in the request context; unknown tokens and disabled users are `unauthenticated`. Only `GetAuthInfo`, `CreateFirstUser`, `Login` and the `SystemService` health check are public. Services check administrator rights and library access themselves, from the user in the context (`permission_denied`). Changes that would leave no enabled administrator fail (`failed_precondition`), and a password change ends the user's other sessions. Folder and file paths of libraries, items and media sources are shown to administrators only.
* **Media URLs**: players cannot always attach headers to media requests (AVPlayer, `<video>`), so media endpoints accept no bearer tokens. A playback started through the authenticated `PlaybackService` gets an unguessable ID, and its media is served under `/media/{playback}/`: the file itself for direct play (`stream.<ext>`, with range requests, opened within its library folder), else `master.m3u8`, `main.m3u8`, `init.mp4` and numbered segments from `libs/streaming`. The URLs stop working when the client stops the playback, after five minutes without progress reports or media requests (the last reported position is kept), or once the sign-in session that started it is gone, which is checked every 30 seconds.
* **Playbacks** (`internal/playback`): the decision considers the user's remembered or preferred streams, their library and rating limits, transcoding permission and bitrate cap, and the user's concurrent playback limit. Remuxes and transcodes are delivered as HLS only; video of a source whose keyframes are not extracted yet is transcoded for that playback, and an urgent keyframe job is queued; a decision for a progressive remux is made again for the client's HLS profile, and a playback copying the video is reported as a direct stream. Progress reports and stopping record the position with `UserData.RecordPosition` ([Domain Model](domain.md)), counting a completed play once per playback. Without ffmpeg, media is only played directly.
* **Images**: artwork is served at `/images/{id}`, with Jellyfin's size parameters (`width`, `height`, `maxWidth`, `maxHeight`, `fillWidth`, `fillHeight`, `quality`); as with media URLs, the ID that clients learn from access-filtered API responses is the only credential. Local images are read within their item's library folder; provider images are downloaded once into the cache directory. Without size parameters the original is served; renderings from `imaging.Process` are in the requested `format` (`jpg`, `png`, `webp`), or WebP for clients whose `Accept` header takes it, else JPEG (PNG with transparency). They are cached under the source's content hash and the parameters, and concurrent requests for one rendering share it. SVGs are served as they are once `CheckSVG` passes.
* **Development player**: with `-dev`, the server also serves `/dev/player`, a single page that signs in, lists videos and plays them through `PlaybackService`, with hls.js or Safari's own HLS player (`?engine=native|hlsjs`) and an optional streaming bitrate cap that makes videos transcode (`?bitrate=<bits per second>`), for checking playback on real browsers before the clients exist. `-dev-library <dir>` adds the `Movies` and `Shows` folders of the sample library `pnpm nx run fixtures:dev-library` generates.
* **Subtitles**: text subtitles delivered as files are served at `/media/{playback}/subtitles/{index}.{format}`, converted by `libs/subtitle` from an external file in the library or from the embedded stream, which ffmpeg extracts once per playback (ASS keeps its styling, other text becomes SRT). For HLS clients that take subtitles in the manifest, each text subtitle is a rendition of the master playlist: a WebVTT playlist with its language as a BCP 47 tag, segmented along the video: each segment, `subtitles/{index}-{segment}.vtt`, holds the cues shown during it, timed on the source's timeline as the video is, and the last also those past the end of the media. Image subtitles (PGS, VobSub) are only burned in; external files the server cannot write, or extract without ffmpeg, are dropped.

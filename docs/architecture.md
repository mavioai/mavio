# Mavio Architecture Blueprint

> English | [简体中文](architecture.zh-CN.md)

> Related: [Clients](clients.md) · [Domain Model](domain.md) · [Development](development.md) · [Roadmap](roadmap.md) · [Testing](testing.md) · [Optimization](optimization.md) · [AGENTS.md](../AGENTS.md)

> Mavio is a next-generation self-hosted media server with a Go server at its core. It draws on the domain knowledge, FFmpeg pipeline and test assets that Jellyfin has accumulated over many years, but it does not aim to be compatible with the Jellyfin ecosystem; every technology choice is the most modern and best-suited option available today.

---

## 1. Positioning and Principles

### 1.1 Goals
* **Lightweight and efficient**: low resident memory, fast cold starts, high scan throughput and low first-segment latency follow from the modern stack — a single native Go binary with no runtime VM, WASM sandboxing instead of heavyweight isolation, and ffmpeg hardware pipelines.
* **Single-binary delivery**: the server is a single `CGO_ENABLED=0` Go binary; its only required external runtime dependency is `jellyfin-ffmpeg`; libraries on cloud storage also need `rclone` (§9.2).
* **Strongly typed contracts**: Protobuf + Connect-RPC is used for all internal and external APIs; the future clients ([Clients](clients.md)) generate their SDKs directly from the contracts.
* **Plugin isolation**: plugins never enter the host's address space. Two runtimes — a WASM (wazero) sandbox and a child process (UDS) — share one set of contracts.
* **Multiple databases**: SQLite and PostgreSQL are both first-class.
* **Private by default**: no telemetry, playback analytics or device tracking; all data stays on the host. Without metadata plugins the server works from local NFO files and images and makes no outbound metadata requests; the plugins that reach the services acquisition uses need no keys of the administrator (§13.2).

### 1.2 Non-goals (current phase)
* No compatibility with the Jellyfin client ecosystem and no Jellyfin data migration. A Jellyfin API compatibility shim is evaluated in [Jellyfin Compatibility](jellyfin-compat.md) and decided in P21.
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
| API reference | **protoc-gen-connect-openapi** + **Scalar** | One OpenAPI 3.1 document generated from the contracts; Connect gRPC reflection (`grpcreflect`) for tools such as `buf curl` and grpcurl |
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
| Command line and settings | **Cobra + Viper** | Flags, `MAVIO_*` environment variables and `config.toml` in the Mavio home; see §11 |

Engineering tooling — Nx, mise, buf, golangci-lint, CI — is described in [Development](development.md).

### 2.1 wazero as the Shared Native Capability Layer
wazero hosts three kinds of workloads: SQLite (ncruces), image codecs (§6) and WASM plugins (§7). All native C library capabilities are thus obtained through WASM, and the whole server stays pure Go and freely cross-compilable.

Note: wazero's compiler backend supports only amd64 / arm64; other architectures (e.g. armv7, riscv64) fall back to the interpreter, where SQLite and the WASM codecs run much slower. Releases therefore target amd64 and arm64 (§12); pure-Go fallbacks for other architectures are an open item in the [roadmap's risks](roadmap.md#4-risks-and-open-items).

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
│   ├── web/                     # [TS] Web UI, embedded in the server (see clients.md)
│   ├── ios/ tvos/ macos/        # [Swift] iPhone and iPad, Apple TV, Mac (see clients.md)
│   ├── android/ android-tv/     # [Kotlin] Phones and tablets, Android TV (see clients.md)
│   ├── desktop/                 # (later) [Go] macOS, Windows and Linux: MyGo, libmpv (see clients.md)
│   ├── website/                 # (P28) [TS] mavio.dev on Cloudflare Workers (see mavio-dev.md)
│   ├── relay/                   # (P28) [Go] Relay: TLS passthrough by SNI (see mavio-dev.md)
│   └── dns/                     # (P28) [Go] Name servers of mavio.site (see mavio-dev.md)
│
├── libs/
│   ├── proto/                   # [Go + TS] Contracts: *.proto sources, buf config, generated Go/TS code
│   ├── plugin/                  # [Go + TS] Plugin SDK and host runtimes (wasm + child process); TS SDK for process plugins
│   ├── core/                    # [Go] Domain model and repository ports (see domain.md); no infrastructure dependencies
│   ├── store/                   # [Go] ent schema, sqlc queries, Atlas migrations, repository implementations
│   ├── naming/                  # [Go] File / directory name parsing
│   ├── metadata/                # [Go] NFO read/write, external IDs, rating systems, metadata merge strategy
│   ├── subtitle/                # [Go] SRT / ASS / SSA / WebVTT subtitle parsing, conversion and character set detection; Subzero's mods, language and hearing-impaired detection; ftfy's text fixes
│   ├── imaging/                 # [Go] Image processing, collages, blurhash / thumbhash
│   ├── media/                   # [Go] Probing, keyframes, hardware acceleration, playback decisions, transcode planning, ffmpeg supervision
│   ├── library/                 # [Go] Scanner, resolver chain, job scheduling
│   ├── streaming/               # [Go] HLS (CMAF) playlists, on-demand segmenting, segment cache
│   ├── release/                 # [Go] Release title parsing: numbering, quality, languages, edition, group
│   ├── acquire/                 # [Go] Acquisition engine: profiles, decisions, searches, queue, import (movie, series)
│   ├── rclone/                  # [Go] rclone rcd: rc client, storage on remotes, media proxy, rcd supervision
│   ├── client-ts/               # [TS] connect-es client and Query hooks of the web UI (see clients.md)
│   ├── client-rust/             # [Rust] Shared core of the native clients (see clients.md)
│   ├── apple/                   # [Swift] Code shared by the Apple apps (see clients.md)
│   ├── android/                 # [Kotlin] Code shared by the Android apps (see clients.md)
│   └── design/                  # Design tokens and icons for every client (see clients.md)
│
├── plugins/
│   ├── scraper-tmdb/            # [Go → wasm] TMDB metadata scraper
│   ├── scraper-musicbrainz/     # [Go → wasm] Music metadata scraper
│   ├── scraper-theaudiodb/      # [Go → wasm] Music biographies and artwork
│   ├── scraper-fanart/          # [Go → wasm] fanart.tv artwork
│   ├── scraper-openlibrary/     # [Go → wasm] Open Library book metadata
│   ├── subtitles/               # [Go → child process] Subtitle providers: OpenSubtitles.com and Bazarr's other sites
│   ├── scraper-servarr/         # [Go → wasm] Servarr's metadata services: SkyHook, Radarr metadata, scene mappings, XEM
│   ├── indexers/                # [Go → child process] Indexers: Newznab, Torznab, Cardigann, Prowlarr's sites
│   ├── download-clients/        # [Go → child process] Download clients: qBittorrent, Transmission, SABnzbd, NZBGet, …
│   ├── agent/                   # [TS → child process, Bun] AI gateway (pi-ai) and agent sessions (pi-durable)
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
     ├──────▶ acquire ──▶ release / naming ──▶ core
     ├──────▶ store ──▶ core
     ├──────▶ rclone ──▶ core
     └──────▶ plugin ──▶ proto

plugins/* ──▶ plugin ──▶ proto
```
* `core` depends on no other library; `naming`, `subtitle` and `release` are pure-function libraries.
* First-party plugins may also use the pure-function libraries: the subtitles plugin reads release names with `release`.
* `acquire`, like `library`, reaches storage, scans and plugins only through ports in `core`, which `apps/server` implements.
* `media` depends only on `core` and knows nothing about databases or plugins.
* `rclone` depends only on `core`: it implements `core.Storage` over an rcd's HTTP API and links none of rclone's code.
* Only `apps/server` does wiring. The dependency direction is enforced in CI by depguard rules.

---

## 4. Contract Design (libs/proto)

```text
libs/proto/
├── buf.yaml  buf.gen.yaml
├── mavio/
│   ├── library/v1/             # LibraryService, ItemService: libraries, items, media sources and streams, people, scans;
│                               #   MetadataService: editing, identifying, images, subtitle downloads
│   ├── auth/v1/                # AuthService: first run, sign-in and sign-out, signed-in devices, API keys
│   ├── user/v1/                # UserService, UserDataService: accounts, policies, preferences, per-item state
│   ├── system/v1/              # SystemService: health, server information, settings, plugins, folders, logs;
│                               #   TaskService, ActivityService, BackupService, LocalizationService, AiService
│   ├── playback/v1/            # PlaybackService: client capabilities, playback decisions, progress reporting
│   └── plugin/v1/              # Plugin contract: PluginService (manifest, configuration, lifecycle),
│                               #   MetadataProviderService, SubtitleProviderService, MediaSegmentProviderService,
│                               #   AuthProviderService, NotifierService
├── gen/go/                     # Generated protobuf-go + connect-go (Go module)
└── gen/ts/                     # Generated protobuf-es + connect-es (npm package)
```

* Client capabilities are expressed with Mavio's own `ClientCapabilities` (containers, codecs, levels, HDR capabilities, subtitle delivery methods, bandwidth), in place of Jellyfin's `DeviceProfile`. The Go type lives in `libs/media/decision`, which stays free of `libs/proto`; the `playback/v1` message mirrors it and `apps/server` converts between them. When porting the StreamBuilder tests, Jellyfin profiles are converted into this type.
* Media streams (direct play, HLS playlists and segments, images) do not go through Connect but over plain HTTP, to benefit from Range, caching and CDN semantics. Uploads of chosen images, at most 20 MiB, are a field of `MetadataService.SetItemImage`.
* API messages mirror the domain model ([Domain Model](domain.md)) but never expose file system internals such as image paths or password hashes.
* `plugin/v1` is published to plugin authors and is self-contained: it defines its own lean messages (`Lookup`, `Metadata`, `PersonCredit`, `RemoteImage`, …) instead of importing `library/v1`, so the two contracts can evolve independently.

---

## 5. Storage Layer (libs/store): ent + sqlc, Two Databases

### 5.1 Responsibilities
| ent | sqlc |
| :--- | :--- |
| **Single source of truth for the schema** (`internal/ent/schema`) | Dialect-specific SQL that ent cannot express portably: the job queue (`INSERT … ON CONFLICT … WHERE` deduplication of pending jobs, atomic leasing that never runs two jobs of one unique key at once, with `FOR UPDATE SKIP LOCKED` on PostgreSQL) |
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
| Collages of libraries, collections and playlists | `image/draw` compositing of up to four images (`Collage`), each covering its cell; without text, since clients label their cards |
| Splash screens (with text) | `image/draw` compositing + `go-text/typesetting` text layout, with a bundled Noto font subset covering CJK |
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

This chapter covers the runtimes and the plugins the server calls; [Plugins](plugins.md) covers what plugins reach in turn — the host API, events, HTTP routes, tasks, remote devices and the further provider types — and the DLNA plugin.

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
* **Calls are Connect requests**: the host's Connect clients use an HTTP transport that, instead of a network connection, writes the request (path, headers, body, and for HTTP routes the method and query) as an envelope into guest memory and calls the module export `mavio_call(ptr, len) -> (ptr << 32 | len)`; `mavio_alloc` / `mavio_free` manage the shared buffers. Inside the guest, `guest/wasm` hands the request to the standard Connect handlers the plugin registered and returns the response envelope (status, headers, body). Plugin code is therefore identical for both runtimes.
* **Build mode**: plugins are WASI reactors (`GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared`); the host runs `_initialize` once per instance.
* **Host functions** (module `mavio`):
  * `http_fetch(ptr, len) -> (handle << 32 | len)` performs an HTTP request on the plugin's behalf if the manifest's `http_hosts` allow the host, and `http_read(handle, ptr)` copies the result into guest memory. The two-step form avoids calling back into the guest from a host function, which Go's wasip1 runtime does not support. The guest SDK wraps them in an ordinary `*http.Client` (`guest.HTTPClient()`). Requests to `mavio.host`, the host API ([Plugins §3](plugins.md#3-host-api)), are served in process by the server's handler tree instead (`guest.HostClient()`).
  * Plugins log to stderr (and stdout), which the host forwards to its logger.
  * Configuration arrives through the `Configure` RPC rather than a host function.
* **File system**: directories listed in the manifest's `read_paths` are mounted read-only at the same paths, and the plugin's data folder writable at `/data`; nothing else is.
* **Instances**: each plugin has its own wazero runtime and a pool of module instances (the number of concurrent calls). An instance whose call traps, panics, exits or exceeds the call timeout is discarded and replaced, so a failing call never affects the host or later calls. A configuration accepted by the plugin is replayed on every instance before its next call.
* **Resource limits**: memory page cap per instance; per-call timeouts (`WithCloseOnContextDone`); the compilation cache (`CompilationCache`) is persisted to disk, since compiling a module takes far longer than instantiating it.
* **Constraint**: Go under `wasip1` has no sockets, so all network access goes through `http_fetch`.

### 7.3 Child-process Runtime
* **Unix Domain Sockets everywhere**: natively supported on Linux / macOS, and on Windows via `AF_UNIX` since Windows 10 1803. The socket lives in a private directory (mode 0700) under a short base path, as socket paths are limited to about 100 bytes.
* **Handshake**: the host passes the socket path and a one-time token through `MAVIO_PLUGIN_SOCKET` / `MAVIO_PLUGIN_TOKEN`. Once listening, the plugin prints one JSON line on stdout (`{"mavio_plugin":1,"plugin_id":…}`); the host checks the protocol version and plugin ID. Every request carries the token as `Authorization: Bearer …`. The host API is served on a second socket in the same folder, `MAVIO_HOST_SOCKET`, which requires the same token.
* **Loopback TCP**: runtimes without Unix sockets on Windows, such as Node's and Bun's, serve on a loopback TCP address instead and name it in the handshake's `address` (`127.0.0.1:port`), which the host dials after checking that it is a loopback address; on Windows the host also serves the host API on a loopback address, `MAVIO_HOST_ADDRESS`. Requests carry the token there as on the sockets.
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
├── guest/           # Plugin side: Handle (register Connect handlers), HTTPClient, HostClient
│   ├── wasm/        # //go:build wasip1: module exports, HTTP through the host
│   └── process/     # Serve: listen on the socket, authenticate, handshake, exit after Shutdown
├── internal/        # abi (WASM envelopes), proc (process protocol), clients, testplugin
├── schema/          # (later) JSON Schema generation from Go structs for configuration forms
└── ts/              # @mavio/plugin: serve, hostTransport, dataDir, hostOS for TypeScript process plugins
```
A plugin registers its handlers in `init` with `guest.Handle(pluginv1connect.New…ServiceHandler(impl))`; a `wasip1` build imports `guest/wasm`, a native build calls `process.Serve`. `internal/testplugin` is a complete example built for both runtimes. Process plugins in TypeScript (Node or Bun) use `@mavio/plugin`: `serve` registers their Connect services from `@mavio/proto`, listens on the socket (on Windows on loopback TCP), authenticates, prints the handshake and returns after `Shutdown`; `hostTransport` is a Connect transport to the host API.

### 7.5 Plugins in the Server
* **Plugin folder**: the server starts every folder of its plugin folder (`--plugin-dir`) that holds a `manifest.json`, through `libs/plugin/host`. A folder that cannot be read, a duplicate plugin ID or a plugin that fails to start is reported as failed and left out; the server runs on without it. Before that, the plugins of the seed folder (`--plugin-seed-dir`, such as those a container image ships) are copied into the plugin folder, each once: a marker file in the plugin folder records each plugin seeded, so one uninstalled or upgraded stays so, and a plugin the folder already holds is not copied.
* **Configuration**: an administrator's configuration is stored by plugin ID ([Domain Model](domain.md) §10) and set through `SystemService.SetPluginConfig`, which validates it against the manifest's `config_schema`, delivers it with the `Configure` RPC and stores it once the plugin accepts it, without a restart. At startup each plugin gets its stored configuration again.
* **States**: a plugin is ready when it runs with its configuration or its schema accepts an empty one; unconfigured while it waits for a configuration its schema requires; failed when it could not start or rejected its stored configuration. `SystemService.ListPlugins` reports them, with the manifest.
* **Metadata providers**: every started metadata plugin is a provider of library refreshes; while it is not ready it knows nothing, so a plugin configured later takes part in the next refresh. Providers that only have images, such as fanart.tv, find them by the IDs earlier providers found, as later providers see them.
* **Hot plugging**: `SystemService.InstallPlugin` installs a plugin from a catalog, or upgrades or downgrades it, and `UninstallPlugin` removes it, while the server serves. An installation downloads the version's zip package, checks its SHA-256 against the catalog, unpacks it into a hidden folder of the plugin folder and checks its manifest; the running version is stopped, its folder set aside, the new one moved in and started with the stored configuration; if it fails to start, the previous version comes back. Uninstalling stops the plugin and deletes its folder and configuration. Capabilities are looked up when used, so refreshes, searches and notifications reach a plugin as soon as it is ready.
* **Catalogs**: a catalog is a JSON document at an http or https URL of the server settings, listing plugins with their versions: version, API version, runtime (with os and arch for process plugins), package URL relative to the catalog, SHA-256, changelog and release time. `ListCatalogPlugins` offers the versions the server can run, newest first, with the version installed; the first catalog listing a plugin wins, and an unreachable catalog is skipped.
* **Host API**: each plugin's host API requests reach the handler tree with the plugin's grant (`auth.PluginHandler`); the authentication interceptor checks them against the manifest's `permissions.api` and resolves `Mavio-User` ([Plugins §3](plugins.md#3-host-api)).
* **Authentication and notifications**: users whose `AuthProvider` names a plugin sign in when the plugin's `Authenticate` accepts their name and password. Every activity of the activity log goes to the notification plugins as an `Event`. Event-consuming plugins get the events their manifest names, logged or not, in batches through a queue of their own ([Plugins §4](plugins.md#4-events)).
* **Tasks and data folders**: plugins' tasks run as `plugin.task` jobs on a worker of their own and appear in `TaskService`; each plugin has a data folder in `--plugin-data-dir` ([Plugins §6](plugins.md#6-tasks-and-data-folder)).
* **Segment providers**: every started segment plugin is asked for the segments of probed movies and episodes, with their IDs, duration and chapters.
* **Subtitle providers**: every started subtitle plugin is searched by `MetadataService.SearchSubtitles` with the video's OpenSubtitles hash (`subtitle.FileHash`), name and IDs, and finds nothing while it is not ready.

---

## 8. Media Pipeline (libs/media) and Hardware Acceleration

### 8.1 Subpackages
```text
libs/media/
├── probe/        # Runs ffprobe (JSON output) and normalizes it into core MediaInfo (video, audio, subtitle streams, HDR10 / HDR10+ / DV metadata)
├── keyframes/    # Keyframe extraction (Matroska Cues and MP4 indexes, else packet-level keyframe flags via ffprobe), used to cut HLS segments on keyframe boundaries
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
* **Black borders**: a low-priority job (`media.borders`, `libs/media/borders`) samples five frames of each probed video with ffmpeg as 8-bit luma and measures them with `libs/imaging`'s detector (sampled rows and columns, 8-bit and 10-bit P010 planes); borders count only where every non-black sample has them, at least 2% of the frame and rounded to even pixels, and are kept on the video stream (`core.MediaStream.Crop`). When a transcode re-encodes the video, unrotated and without burned-in subtitles, the planner crops them before scaling (server setting `CropBlackBorders`), so letterboxed films spend no bitrate on black bars; copied video is never cropped. Trickplay sheets and chapter images crop them too.
* **Dolby Vision Profile 7**: a separate enhancement-layer track is recognized only from explicit signaling, a Dolby Vision configuration with an enhancement layer and no base layer (`core.MediaStream.IsDolbyVisionEnhancement`), never from track order, resolution or names. Stream selection never picks such a track while the file has another video track, so playback, transcodes and copies use the Base Layer. A single-track Profile 7 stream goes to clients declaring Dolby Vision with an enhancement layer; for clients taking HDR10 only, the planner removes the Dolby Vision metadata and keeps the HDR10-compatible Base Layer, tone mapping it when it re-encodes for SDR clients (`libs/media/planner/hdr.go`, Jellyfin's rules).
* **Stereo downmix**: the planner carries Jellyfin's stereo downmix algorithms (`libs/media/planner/audio.go`: Dave750, night mode dialogue, RFC 7845, AC-4 and ITU-R BS.775 with the center channel at full level, 7.1 folded to 5.1 first); without one, ffmpeg's downmix applies. Downmixed audio is amplified by the server's downmix boost (`DownmixBoost`, 2 by default, as in Jellyfin), which keeps dialogue audible on stereo devices.

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
* **Resolution one folder at a time**: the resolver maps a folder and its entries, given its place in the library (library kind, and whether it lies in a series, season or artist), to the item the folder is (a movie in its own folder, a disc rip, a series, a season, an album, an audiobook), the items made of its files, and the subfolders to resolve next with their place. It follows Jellyfin's naming and resolver rules; extras are found from the owner's folder and its extras folders. Resolver plugins (`library.FolderResolver`, [plugins.md](plugins.md) §8.6) first leave entries out and may claim a folder before the built-in resolver; a resolver failing makes the folder unreadable, keeping its items. Because a folder's result depends only on its listing and its place, folders that did not change need not be resolved again.
* **Reconciliation reads only metadata**: it walks directories and compares file stats; only new or changed files are sent to ffprobe and scraping.
* **Concurrency**: library roots and directory subtrees are walked concurrently (`errgroup.SetLimit`), with lower concurrency on network file systems.
* **Jobs**: scans, probes and metadata refreshes are durable jobs run by workers that lease them (`library.scan`, `media.probe`, `media.keyframes`, `item.refresh`, `image.placeholders`). A scan queues a probe for each new or changed media file, and a metadata refresh for each new item without media, such as a series, season or album; it attaches the subtitle files beside each video as its external streams (§9.1). A probe of a video also queues a low-priority detection of its black borders (`media.borders`), which transcodes crop. Then, as the library asks, it queues the media extras (`libs/media/thumbnails`, after border detection): the trickplay sheets (`media.trickplay`: one ffmpeg pass of the fps, crop, scale and tile filters over a keyframe a thumbnail, read through a Matroska file's index or else decoded alone from the file, 320 pixels wide, every 10 seconds, 10 × 10 to a sheet, written aside in the metadata folder and swapped in; [Optimization §2.5](optimization.md#25-trickplay-from-keyframes)), chapter images (`media.chapters`: a frame of each chapter, 640 pixels wide, the first chapter's a little after the start), the media segments of movies and episodes from segment provider plugins (`media.segments`; where two providers' segments of a kind overlap, the first's), and for tracks their loudness (`media.loudness`: ffmpeg's `ebur128` integrated loudness, an album's following once all its tracks have theirs). Extras wait for the quiet gate, and a file whose images or measurement failed is not read again for a day (`FailNotes`, in memory, at most 4,096 notes). `library.extras` queues them for every item of a library. A probe queues a metadata refresh; for video it reads the keyframes HLS needs to cut copied video from the container's index, and for a file without one queues a low-priority extraction that reads every packet, where the storage class allows ([Optimization §2.4](optimization.md#24-keyframes-from-container-indexes)). A refresh applies providers' metadata, then local metadata plugins' (`library.LocalReader`), then the local NFO file, then has metadata processor plugins adjust the result (`library.Processor`), leaving locked fields alone, and scores the resulting rating (`metadata.RatingScore`). Image generator plugins (`library.ImageGenerator`) then make images of the kinds no source gave, kept in the metadata folder. Artwork beside the media is found by Jellyfin's local image names (`poster`, `folder`, `<file>-poster`, `fanart-1`, `season01-poster`, `<episode>-thumb`, …; in folders shared by several videos only names prefixed with the file's) and comes before images an NFO file names; a photo is its own primary image, which the image route serves and renders; each kind of image comes from the most trusted source that has one. Images found again keep their ID, size and placeholders, and a placeholders job then measures new images and computes their blurhash and thumbhash. Artwork an administrator chose for an item of a library that does not save local metadata is kept in the metadata folder (`--metadata-dir`, `<id[:2]>/<id>/<kind>.<ext>`) and comes before all other artwork. In libraries saving local metadata, a refresh writes the item's NFO file (`metadata.WriteNFO`, Kodi's format with Jellyfin's lock elements; over the NFO file found, else `<file>.nfo` beside a movie or video and Jellyfin's names otherwise) and the metadata saver plugins' files (`library.Saver`), and saves its provider images under the local image names, replacing files of the same name in other formats; seasons, tracks and photos keep theirs remotely. Scheduled scans enqueue the next one after the library's scan interval; requested scans run beside them, and scans of one library never overlap. A folder scan (`library.scan_folder`), which imports queue for the folder they wrote to, reconciles that folder and those below it, with the folders above it from the library folder down, and marks nothing missing, leaving that to the library's next scan.
* **Storage awareness**: scans and probes serialize reads of hard disks and remote volumes, wait while clients stream, and leave out files still being written until they settle ([Optimization §1](optimization.md)).
* **Storage port**: everything that reads or writes library files (scans, local metadata and artwork, sidecars, lyrics, subtitle downloads, NFO and image writes, direct play, imports and the recycle bin) goes through `core.Storage`, opened for a library path by `library.Storages`: local folders over `os.Root`, rclone remotes through `Storages.Remote`. Media tools take a `library.MediaFile`, whose URL ffmpeg and ffprobe read (a `file:` URL locally) and whose OS path, for local files only, allows prefetching and reading container indexes directly. Imports move and link local files on the file system, have a remote move or copy files within itself (`core.StorageTransfers`), and otherwise copy through the server, deleting the source after a move.
* **Consistency**: each scan records a "scan generation" in the database; after a scan completes, items not seen in that generation are marked missing (soft-deleted first, purged after a grace period), so a temporarily unavailable mount point cannot wipe out a whole library.

### 9.1 Subtitle Sidecars and Scoring

Subtitle file names vary widely, Chinese ones especially. `libs/subtitle` normalizes language aliases and scores subtitle files against a video.

Aliases normalized to `zh`:

```text
"zh", "chi", "zho", "chs", "cht", "sc", "tc", "gb", "big5",
"zh-hans", "zh-hant", "zh-cn", "zh-tw", "zh-hk",
"简体", "繁體", "繁体", "简中", "繁中", "简", "繁", "中文", "双语" ──▶ "zh"
```

Scoring:

| Match | Score | Example |
| :--- | :--- | :--- |
| Same name as the video | `+1000` | `Movie.mkv` and `Movie.srt` |
| Video's name and a delimiter as prefix | `+500` | `Movie.mkv` and `Movie.zh.srt` |
| A preferred language | `+200` | the user's preferred subtitle language |
| Another language | `-100` | |
| Forced | `-150` | `.forced`: few lines of dialogue |
| Hearing impaired | `-50` | `.sdh`, `.cc`, `.hi` |
| Format | `+20` ASS/SSA, `+10` SRT | richer styling first |

* **Sidecar discovery** (`libs/library`): scans attach the subtitle files beside a video as its external streams: files whose name is the video's, or starts with it and a delimiter (a stem score of at least 500). Their language, title and default, forced and hearing-impaired flags come from the file name, with Chinese variants recognized (`zh-Hans`, `zh-Hant`, `chi`) and three-letter codes taken only for languages that also have a two-letter one, so that flags such as `sdh` stay flags. They are ordered by the score without a user's languages and numbered after the embedded streams; files added or removed later update the streams without probing the video again, and probing keeps them. Subtitle files are also found in the subtitle folder the settings name: the `relative` folder within the video's folder, whose changes mark the video's folder changed, and the item's `metadata` folder, read when the scan saves the video's folder. Playback and the subtitle service open sidecars by the path the scan recorded, also outside the library.
* **Playback selection** (`libs/media/decision`): a user's preferred subtitle language matches streams by the normalized aliases, so a preference for Chinese finds simplified, traditional and regional variants, and hearing-impaired streams rank lower.

### 9.2 rclone Remotes (libs/rclone)

A library path `rclone:<remote>:<path>` lies on a remote of rclone, reached through an `rclone rcd` over its HTTP API (the rc API); the server links none of rclone's code, and an rcd may run on another machine.

* **Managed rcd**: by default the server runs `rclone rcd` itself (`rclone` names the executable; without one, remotes are unavailable and the rest of the server works). It listens on a Unix socket in a private folder, with a random user and password made at each start; readiness is `rc/noopauth`; when it exits it is restarted after a delay that doubles up to a minute and resets after a minute of running; on shutdown it is asked to quit (`core/quit`), then terminated, then killed. Its log lines go to the server's log at their levels.
* **External rcd**: `rclone-url`, with `rclone-user` and `rclone-pass`, reaches an rcd the server does not run, over HTTPS, a Unix socket, or HTTP on the loopback interface only, so its credentials never cross a network in clear.
* **Configuration**: the managed rcd keeps its remotes in `<home>/rclone/rclone.conf`. When the server creates the file it encrypts it, with a random password kept in `rclone.pass` beside it (mode 0600) and passed to the rcd as `RCLONE_CONFIG_PASS`; a file the server did not create is read as it is. `mavio.storage.v1.StorageService` (administrators only) lists the providers and their options, from which clients build their forms (`config/providers`), creates, updates and deletes remotes, answers the questions of rclone's non-interactive configuration, OAuth included (`config/create`, `config/update` with `continue`), and lists folders for choosing library paths; listings leave out passwords, sensitive options and tokens.
* **Storage**: `rclone.Remotes` opens a `core.Storage` for a remote's folder: listings and stats (`operations/list`, `operations/stat`, with the backend's original IDs as file IDs), uploads (`operations/uploadfile`, streamed as multipart), folders, deletes, and moves and copies within the rcd (`operations/movefile`, `operations/copyfile`, server-side where the backend allows; capabilities from `operations/fsinfo`, cached for a minute). Long operations run as rc jobs (`_async`) and are polled; a job the rcd no longer knows, after it restarted, fails with `rclone.ErrJobLost`. Running transfers are recorded in the storage job ledger (`core.StorageJob`), so that a transfer asked for again after the server restarted, such as an import retried, waits for the job still running instead of starting another, and one that finished is not run again; at startup the ledger drops the jobs that ended or that the rcd forgot. Remotes have no settable modification times; callers skip them.
* **Media bytes**: reads are HTTP range requests to the rcd's `--rc-serve`. ffmpeg and ffprobe read through a proxy the server runs on the loopback interface: each media URL carries a random token (kept twelve hours after its last use) that the proxy maps to the object, adding the rcd's credentials, so they never appear on ffmpeg's command line; the proxy streams responses and forwards range and conditional headers. Every folder of a remote names its objects from the remote's root, so that they share one file system in the rcd, which keeps it an hour. A read keeps the version (ETag) of its first response and asks for it with every later request, failing with `rclone.ErrChanged` when the file changed. Folders of the `cloud` storage class are read through a block cache on local disk (`rclone.BlockCache`, in `<cache>/rclone/blocks`, sized by `rclone-cache-size`), which both `Storage.Open` and the proxy read, ahead as the read's `core.ReadIntent` asks; playbacks fetch ahead into it from their keyframes' file offsets ([Optimization §3](optimization.md#3-reading-remote-storage-through-rclone)).
* **Offline downloads**: cloud drives whose rclone backend downloads links itself (mavioai/rclone's `operations/addurl`, `listoffline` and `deleteoffline`, such as PikPak and 115) are download clients built into the server (`downloads.Offline`, provider `dev.mavio.rclone-offline`), configured through `StorageService.ListOfflineClients` and `SetOfflineClients`. A client is a folder of a remote; magnet links of movies are downloaded into its `movies` folder and of series into `series`, and its downloads are known by their torrent's info hash. The ledger keeps each download's task, title, category and last seen status, which tells the server's downloads apart from the drive's others and keeps those the drive stops listing. A completed download's output path is on the remote, so imports into libraries on the same remote move the files within it.
* **Imports from remotes**: imports read download folders on remotes through their storages, probe their files through the media proxy, have the remote move or copy them (`core.StorageTransfers`), and delete emptied download folders with `operations/purge`.

---

## 10. HLS Streaming (libs/streaming)

* **Playlists up front**: a playback's media playlist lists every segment of the whole media source before anything is transcoded (VOD), so clients can seek anywhere. Encoded video and audio are divided into segments of equal length, with keyframes forced at the boundaries; copied video can only be cut at its keyframes, so its segments are cut at the first keyframe at or after each multiple of the segment length, using the keyframes stored with the media source. The master playlist announces the output's RFC 6381 codec strings, resolution, frame rate and dynamic range (`VIDEO-RANGE`), with kept Dolby Vision or HDR10+ as `SUPPLEMENTAL-CODECS`.
* **Segments on demand**: one ffmpeg run per stream writes segments ahead of the client. A request for a segment that is not written yet waits for the run, or restarts ffmpeg at that segment when it lies before where the run started or more than 24 seconds beyond what it has written. Written segments are kept for the playback session and served again, so seeking back costs nothing. Requests to one stream are served one at a time.
* **Restarts that line up**: ffmpeg's HLS muxer spaces its cuts from where a run starts, which would misplace copied video's cuts after a seek. Copied video is therefore written one group of pictures per file, numbered by its keyframe, and a segment is served as its consecutive files; a run starting anywhere writes the same files. Restarts read copied video from the middle of the segment's first group of pictures, because ffmpeg moves seeks back by a few frames for streams with reordered frames. Encoded runs start exactly at a segment boundary, so their cuts line up by themselves.
* **One timeline**: segments carry the source's timestamps from where it starts, continued from the start position after a seek (`-output_ts_offset`), as ffmpeg also times subtitles extracted for delivery as files. The video's may be negative (`-avoid_negative_ts disabled`, with `frag_discont` in fMP4): video with reordered frames is not shifted later, so the first frame plays at zero, in sync with the subtitles. Audio in fMP4 that starts before zero, as encoder priming and the priming of AAC in MP4 sources do, is shifted to zero (the `setts` bitstream filter), as a fragment's decode time (`tfdt`) is unsigned and Media3 rejects a negative one; it plays that much late, at most a few tens of milliseconds.
* **Safe replacement**: ffmpeg writes every file under a temporary name and renames it when complete (`-hls_flags temp_file`): a file under its final name is complete, and a later run replacing it never truncates what a client is reading. Each run writes its own initialization segment; the first one is kept and served.

---

## 11. API and Authentication (apps/server)

* **Assembly**: `apps/server/internal/server` wires storage, plugins, playback, images, the library worker and the handler tree, and runs them until shutdown; `cmd/mavio` only resolves its configuration, and the end-to-end test runs the same assembly.
* **Home and settings**: everything the server keeps lives in its home, `~/.mavio` unless `--home` or `MAVIO_HOME` names another: `config.toml` (optional), the SQLite database `mavio.db`, the server's identity key `identity.key` (§11, Sessions), `plugins/`, `plugin-data/` (the plugins' data folders), `metadata/` (chosen artwork), `backups/` and `cache/` (downloaded and resized images, compiled plugins, running transcodes in `cache/transcodes/`, and the block cache of rclone remotes in `cache/rclone/blocks/`), each created when missing. Conventions need no configuration; the settings that do exist (`addr`, `database`, `identity-key`, `ffmpeg`, `ffprobe`, `cache-dir`, `transcode-dir`, `plugin-dir`, `plugin-seed-dir`, `plugin-data-dir`, `metadata-dir`, `backup-dir`, `discovery-addr`, `rclone`, `rclone-config`, `rclone-url`, `rclone-user`, `rclone-pass`, `rclone-cache-size`, `mavio-dev-url`, `claim-token`, `dev`, `dev-library`) are read with Viper from, in order, the command's flag (`--cache-dir`), an environment variable (`MAVIO_CACHE_DIR`) and `config.toml` (`cache-dir = "/srv/cache"`), and default to the layout above; the transcode folder follows the cache folder. Paths in `config.toml` must be absolute, and a relative one stops the server; relative paths in flags and variables are relative to the working directory; an empty `plugin-dir`, `backup-dir` or `discovery-addr` turns plugins, backups or discovery off, and a `rclone-cache-size` of 0 the block cache. Unknown keys in `config.toml` stop the server. `mavio version` prints the version, `mavio admin reset-password` recovers the administrator of a standalone server ([Accounts §4.5](accounts.md#45-recovery)) and `mavio admin unclaim` unclaims a server in mavio.dev mode ([Accounts §3.2](accounts.md#32-servers-and-claiming)), and `--restore <backup>` restores a backup into the empty database before serving.
* **One handler tree**: `apps/server/internal/httpserver` mounts the Connect services (implemented in `internal/rpc`) and the plain HTTP media endpoints on one `http.ServeMux`. Every Connect request is checked against its protovalidate rules before it reaches the service; violations are `invalid_argument`.
* **Cross-origin requests** (`httpserver.cors`): every route answers requests from any origin (CORS, preflights and private network requests included), so that a web UI served by one server signs in to others ([Clients §5.4](clients.md#54-several-servers)). Their credentials are bearer tokens and signed media URLs, never cookies, so another origin gains nothing a request of its own would not.
* **Accounts**: passwords are hashed with argon2id (19 MiB, 2 passes, 1 lane) into a PHC string; a hash with other parameters is still accepted and replaced at the next successful sign-in. Signing in with an unknown name costs the same as a wrong password. Users that authenticate through a plugin have no password hash. Passwords, PINs, TOTP secrets and passkeys are `Credential`s of their own ([Domain Model](domain.md) §7). A user may set up TOTP (`UserService.StartTotp` and `ConfirmTotp`, with a code of the new secret); `Login` then needs a code of the current period or one next to it, used once, and fails as `failed_precondition` when the right password comes without one. Passkeys are created by signed-in users (`StartPasskeyRegistration`, `FinishPasskeyRegistration`) and sign in alone, naming their user (`AuthService.StartPasskeySignIn`, `FinishPasskeySignIn`); their ceremonies are kept in memory for five minutes and used once, and the relying party is the host of the request's `Origin` ([Accounts §8](accounts.md#8-server-implementation)). Managed users have neither, and users of authentication plugins have no passkeys. Failed sign-ins are throttled per user name and per address, in memory: three are free, then each locks the name and the address out twice as long as the one before, from a second up to 15 minutes, and a sign-in while locked out fails as `resource_exhausted`; a successful one clears its name, and a name or address is forgotten an hour after its last failure. Phase P28 builds the account design of [Accounts](accounts.md): the server's two modes, device-bound sessions and library access records.
* **Quick Connect**: a new device can be signed in from a signed-in one, as in Jellyfin. It starts a request without credentials and shows its six-digit code; a signed-in user enters the code, which signs the device in as that user; the device, polling the request with its secret (32 random bytes, never shown), then claims its access token once. Requests are kept in memory and expire after ten minutes, or one minute after they were authorized. Codes are unique among pending requests.
* **First run**: while the server's account mode is unset ([Accounts §2](accounts.md#2-server-modes)), `AuthService.CreateFirstUser` sets the server up in standalone mode: it stores the mode, creates the administrator and signs it in, in one transaction; afterwards it fails with `failed_precondition`. `AuthService.GetAuthInfo` tells clients whether setup is pending and which mode the server is in, and names the server: its ID, stored in the database, by which a client signed in to several servers recognizes one it reaches at another address, its name and its version.
* **Sessions** ([Accounts §5.2–§5.3](accounts.md#5-keys-and-tokens)): a device signs in with its Ed25519 public key, and the server keeps the session bound to it as an `AuthSession` ([Domain Model](domain.md)). Signing in gives an access token valid for an hour, which clients send as `Authorization: Bearer <token>`: the session's ID and expiry, authenticated with HMAC-SHA256 under a key derived with HKDF from the server's Ed25519 identity key. The identity key is a PEM file in the server's home (`identity-key`), created when missing with mode 0600, so that a copy of the database cannot make tokens; nothing is stored per token, so every device or browser tab of a session refreshes on its own. Before the token expires, or when a call fails as `unauthenticated` because it has, the device asks `StartRefresh` for a nonce (32 random bytes, kept in memory for two minutes, one per session, used once) and sends its signature to `RefreshSession`: Ed25519 over `mavio-session-refresh-v1`, a zero byte, the session ID's 16 bytes and the nonce. A refresh fails, and the device signs in again, when the signature or nonce is wrong, the user is disabled, or the session was unused for longer than the server settings' `SessionIdleDays`; the daily cleanup deletes such sessions. Sessions also end when the client signs out, the user revokes the session or deletes the account, or the same device signs in again. A session's last activity is recorded at most once a minute.
* **Authorization**: an interceptor resolves an access token to the session and its user, or an API key's token, looked up by its SHA-256 hash, to the administrator who created it, and puts them in the request context; unknown and expired tokens and disabled users are `unauthenticated`. Only `GetAuthInfo`, `CreateFirstUser`, `Login`, `StartRefresh`, `RefreshSession`, the unauthenticated steps of Quick Connect, of password resets and of accepting invitations, and the `SystemService` health check are public. Services check administrator rights and library access themselves, from the user in the context (`permission_denied`). The server has exactly one administrator, created at setup: creating another, changing who it is, and disabling or deleting it fail (`failed_precondition`). The administrator creates members and managed users, sets managed users' PINs and restriction profiles, and makes a managed user a member by giving it a password; users set their own PINs (`UserService.SetPin`). A password change ends the user's other sessions.
* **Invitations and switching** ([Accounts §4.3–§4.4](accounts.md#43-joining-and-signing-in)): `UserService.CreateInvitation` returns an invitation's token once and stores its SHA-256 hash. `AuthService.GetInvitation` describes a usable invitation, `StartInvitationPasskey` begins a passkey ceremony for the member it creates, under an ID chosen then and bound to the invitation, and `AcceptInvitation` counts the use, creates or converts the member with its password or passkey and signs the device in, in one transaction; unknown, used up and expired invitations are alike `not_found`. `AuthService.ListSwitchableUsers` and `SwitchUser` need a session that may switch: one signed in as the administrator, or one switched from it (`AuthSession.SwitcherID`). Switching moves the session to the target user, after checking its PIN: a missing PIN is `failed_precondition`, a wrong one `permission_denied`, and wrong PINs are throttled per user as sign-ins are, `resource_exhausted` while locked out. Folder and file paths of libraries, items and media sources are shown to administrators only.
* **Claiming and the link to mavio.dev** ([Accounts §3.2](accounts.md#32-servers-and-claiming), [mavio.dev §2–§3](mavio-dev.md#2-website-and-api-appswebsite)): the server's ID is the first 20 bytes of its identity public key's SHA-256 in lowercase base32, shown by `GetAuthInfo` and local discovery. `internal/cloud` calls mavio.dev's `mavio.cloud.v1.ServerService` at `mavio-dev-url` with JWTs signed by the identity key. `AuthService.StartSignIn`, which needs no sign-in, pushes the web UI's authorization request, with its redirect URI, PKCE challenge, device key and state, to mavio.dev and returns the page of mavio.dev where the account approves it; while the server is unclaimed, the request claims it and the server keeps its ID until it expires. `SignInWithAssertion` first asks mavio.dev for the outcome of the requests it keeps, newest first, and an approved one claims the server. A standalone server answers `failed_precondition` and an unreachable mavio.dev `unavailable`. `claim-token` claims an unclaimed server with a claim code when it starts. Either way mavio.dev answers with the administrator's account, and one transaction stores the claim in the account settings and makes that account's user the administrator. A server that needs setting up opens `http://localhost:<port>/setup` in the desktop's default browser when it starts (`open-browser`). The link publishes the URLs the server is reached at, best first: its names under `mavio.site` over HTTPS once it has its certificate, then its addresses over HTTP ([mavio.dev §4.3](mavio-dev.md#43-connecting)). While the server is claimed, a link holds its WebSocket to mavio.dev (`golang.org/x/net/websocket`), reconnecting with exponential backoff and jitter, and fetches the changes after its cursor when it connects, when a notice names a later cursor and every six hours; each page of changes is applied in the transaction that advances the cursor. The link publishes the libraries when it connects and checks every minute whether they changed. `AuthService.SignInWithAssertion` signs in with a sign-in assertion ([Accounts §5.5](accounts.md#55-sign-in-assertions)) checked against mavio.dev's cached key set; an invalid assertion, another device or an account without access is `unauthenticated`. In mavio.dev mode the RPCs of the server's own credentials and users answer `failed_precondition`, and `GetAuthInfo` gives the mavio.dev URL.
* **Media URLs**: players cannot always attach headers to media requests (AVPlayer, `<video>`), so media and images are fetched through URLs signed for the caller's session (`auth.MediaURLs`, [Accounts §5.4](accounts.md#54-media-urls)): `/signed/{token}/{path}` serves `/{path}` once the token is checked, and the paths without the prefix take bearer tokens, or the host API's plugins. The Connect services sign the URLs they return through the request's context. A playback started through the authenticated `PlaybackService` gets an unguessable ID, and its media is served under `/media/{playback}/`: the file itself for direct play (`stream.<ext>`, with range requests, opened within its library folder), else `master.m3u8`, `main.m3u8`, `init.mp4` and numbered segments from `libs/streaming`. The URLs stop working when the client stops the playback, after five minutes without progress reports or media requests (the last reported position is kept), or once the sign-in session that started it is gone, which is checked every 30 seconds.
* **Playbacks** (`internal/playback`): the decision considers the user's remembered or preferred streams, their library and rating limits, transcoding permission and bitrate cap, and the user's concurrent playback limit. Remuxes and transcodes are delivered as HLS only; video of a source whose keyframes are not extracted yet is transcoded for that playback, and an urgent keyframe job is queued; a decision for a progressive remux is made again for the client's HLS profile, and a playback copying the video is reported as a direct stream. Progress reports and stopping record the position with `UserData.RecordPosition` ([Domain Model](domain.md)), counting a completed play once per playback. Without ffmpeg, media is only played directly.
* **Events** (`internal/events`, `EventService`): every signed-in device can hold one event stream, a Connect server stream that starts with the device's session and sends a heartbeat every 30 seconds; a newer stream of the device replaces the older, and a stream more than 256 events behind is dropped so its client reconnects. The server keeps the streams in memory. `events.Observe` wraps the store, so that what is written reaches the streams once its transaction commits: library and item changes, gathered for five seconds and then sent to each device as the items and libraries its user may see; a user's item state to that user's devices; library scans starting and ending to administrators. Plugin configuration changes and session changes are sent by the services that make them.
* **Sessions** (`SessionService`): a device is online while its event stream is open. Sessions list the user's signed-in devices, or every user's for administrators, with what each plays (`playback.Manager.NowPlaying`: item, position and pause as last reported). Devices learn that a session of their user came online, went offline, or started, stopped, paused or resumed a playback. A command (play items, pause, resume, stop, next, previous, seek, show a message) goes to an online device of the caller, or of anyone for administrators, through its event stream; the device carries it out and reports its playback as usual.
* **SyncPlay** (`internal/syncplay`, `SyncPlayService`): online devices watch together in groups, kept in memory, following Jellyfin's SyncPlay. A group is idle, waiting, paused or playing. Setting a queue, seeking, stepping through the queue and a member buffering or joining make it wait; once no member buffers it plays, or stays paused if it was. It starts playing after twice the highest member ping, at least 500 ms, so that every member receives the command in time. A member reporting ready more than 500 ms from the group's position is told to seek; the time a report gives is ignored when it is more than two seconds from the server's. Every change sends each member the group (state, members, queue, and the position it is at at a given time) with, when the member has to act, a command timed in server time; clients estimate their clock's offset with `GetTime`. A device is in one group at most, leaves it when it goes offline, and a group without members ends. Members may join only groups whose queue they may play, and a queue must be playable by every member.
* **Browsing** (`internal/browse`): the latest items and next up are built from item queries in the user's libraries and rating. Latest items are the leaf items (movies, episodes, tracks, …) by date added; grouped, as in Jellyfin's current server, an album stands for its new tracks and a series for its episodes added within a day of its newest one: their season when they are all in one season of a series with several, the series when it has one season or they span several, the newest episode when it came alone, and that episode too when the user may not see the container. Next up walks the user's episodes by last played, takes each series once in that order and asks `NextEpisode` ([Domain Model](domain.md)) for its next episode until the page is full, so the series played most recently come first. Item responses name an episode's series and season and a track's album.
* **Home** (`internal/home`, `HomeService`): the hubs of Home and of a library's Recommended tab ([Clients §6.3.1](clients.md#631-home)), each a page of items with its kind and, for a seeded hub, its seed, which clients name in their own language. They are built from item queries in the user's libraries and rating: continue watching (resumable items and next up, by last activity), recently added (as latest items), recently released and aired, top rated and top unplayed, start watching (series not begun), rediscover (items last played over a year ago, series over half a year ago), and the requests that became available in the last 30 days. The seeded hubs follow Jellyfin's movie recommendations — because you watched, because you like, more in genre, starring, directed by — with seeds drawn from the user's plays of the last 90 days, or their favorites, by a hash of the user, the hub and the day. Trending lists the titles of the discovery providers' first trending catalog that the server lacks. Administrators set for each library which hubs appear on Home and on its tab (`Library.Hubs`, [Domain Model](domain.md) §4.1); users pin and order libraries and hide hubs in their display preferences of client `mavio`, view `home`.
* **Similar items** (`ItemService.ListSimilarItems`): the items of the same kind scored as Jellyfin's local similar items provider scores movies, by the genres (10), tags (5), studios (5), directors (50) and actors (15) shared with the item (`ItemQuery.SimilarTo`, [Domain Model](domain.md) §9.1), within the user's libraries and rating; episodes and seasons stand for their series. They fill an item's More Like This and the because you watched hubs.
* **Collections and playlists** (`CollectionService`, `PlaylistService`): administrators curate collections, users their own playlists, in the curated libraries the server creates when first needed. Every user sees the playlists library, with their own playlists only; the collections library follows the library policy. `ItemService.ListItems` with a collection or playlist as parent lists its items once each, in its order unless sorted otherwise, as the user's policy allows; `ListPlaylistEntries` lists a playlist's entries with their IDs. Adding a series, season, album or collection to a playlist adds its playable items in order. `LibraryService` does not create, change or scan curated libraries, and the startup scans skip them.
* **Metadata management** (`MetadataService`, administrators only): `UpdateItem` sets the fields an update mask names, locks included, and saves the item as `library.Refresher.Update`; `RefreshItem` refreshes an item at once, and with `replace_metadata` forgets what providers gave before, except external IDs and locked fields, and ignores the item's NFO file (in libraries saving local metadata also the artwork beside it, which the providers' replaces). `SearchRemote` asks the metadata plugins that search what an item may be, by its name and year or those given, and `IdentifyItem` replaces the item's external IDs with a result's and refreshes it that way. `ListRemoteImages` lists the providers' images of an item; `SetItemImage` downloads one or takes an upload and makes it the first image of its kind, beside the media or in the metadata folder; `DeleteItemImage` removes an image and its file. `SearchSubtitles` and `DownloadSubtitle` find subtitles through the subtitle plugins and save them beside the video as `<file>.<language>[.sdh][.forced].<format>`, converted to UTF-8, where they become the video's subtitle streams at once. A change to an item of a library saving local metadata rewrites its NFO file, and a movie of a library with automatic collections joins its collection.
* **Settings** (`internal/settings`, `SystemService.GetServerSettings` and `UpdateServerSettings`): the server settings ([Domain Model](domain.md) §12) are stored in the database and applied without a restart. Each part of the server registers how it applies them; a change goes to every part, and when one rejects it, as when a certificate cannot be read, those that took it get the previous settings back and nothing is stored. Transcoding settings apply to the playbacks that start afterwards, through a new planner and transcode folder in `playback.Manager`; `GetServerSettings` also lists the hardware accelerations the server's ffmpeg supports.
* **Network** (`internal/network`): the handler tree is served at the root and under the base URL, whose bare path redirects to it with a slash. HTTPS runs on its own port with the configured certificate, started, restarted or stopped as the settings change; without one, a claimed server serves its `*.<ID>.mavio.site` certificate from `internal/certs` once it has one ([mavio.dev §4.2](mavio-dev.md#42-certificates)), and with it, unless the settings turn relaying off, `internal/relay` keeps a connection to mavio.dev's nearest relay and serves the same handler over TLS on the connections the relay hands it ([mavio.dev §5.2](mavio-dev.md#52-servers-connections)). Local discovery answers the UDP request `who is MavioServer?` on `--discovery-addr` (7359 by default) with the server's ID, name, version and address on the interface the request came in on.
* **Tasks and jobs** (`TaskService`): the recurring tasks are each library's scan, the plugins' tasks, the acquisition jobs (RSS sync, the queue check and the recycle bin cleanup, on the intervals of the acquisition settings, and the daily refresh of wanted movies) and the daily job cleanup; `ListTasks` gives each its interval, last finished run, next run and whether it runs, `RunTask` queues a run now, and `ListJobs` pages through the job queue.
* **API keys** (`AuthService.CreateApiKey`, `ListApiKeys`, `RevokeApiKey`): the interceptor takes an API key's token where a session's is expected ([Domain Model](domain.md) §12.2); its requests have no session.
* **Activity and logs** (`internal/activity`, `ActivityService`; `internal/logs`, `SystemService.ListLogs`): activities are stored and queued for the notification plugins, and purged after 90 days. The log also publishes the events it leaves out (item, user data, task and scan events, from `events.Observe` and the hub) to the event consumers; the plugin manager keeps a queue per consumer. The activity log is created before the hub and the plugins, and reaches the plugins once they started. The server keeps its last 2,000 log records of level info and above in memory for `ListLogs`, while logging as before.
* **Folders** (`SystemService.ListDirectory`): administrators browse the server's folders, or the roots (the drives on Windows), to choose library folders; hidden entries are left out.
* **Localization** (`LocalizationService`): countries, languages with an ISO 639-1 code and the ratings of a country's rating system, from Jellyfin's data in `libs/metadata` (`Countries`, `Languages`, `ParentalRatings`).
* **AI providers** (`AiService`, administrators only): the providers, models, API keys and OAuth logins of the plugin with the `AI_GATEWAY` capability ([Plugins §8.15](plugins.md#815-ai-gateway-and-agent-sessions)), forwarded to it; the server keeps no credential. Logins run in the plugin and are followed through `WatchAiLogin`, which streams each change until the login ends, or read with `GetAiLogin`; the plugin's unauthenticated failures, which concern the provider, become failed preconditions.
* **Backups** (`internal/backup`, `BackupService`): a backup is a zip file in `--backup-dir` with `manifest.json`, the database rows (`store.Dump`: each table as JSON lines of typed values, from one snapshot), the metadata folder, the plugin folder and the plugins' data folders. Administrators download it at `/system/backups/{name}` with their bearer token. `-restore <file>` restores one into the empty database of a new server before it starts (`store.Restore`: same dialect, a schema with every migration the backup's had; rows are inserted in foreign-key order, items after their parents, and derived keys are computed again) and puts the folders back.
* **Downloads and progressive streams**: a playback asking to `download` needs the user's download permission and is decided in the static context: the file itself when the client plays it, served as an attachment, else a progressive remux or transcode to the client's static profile, never HLS. Streaming clients without an HLS profile get progressive streams too. ffmpeg writes a progressive stream for each request straight into the response (fragmented MP4, Matroska or MPEG-TS), so it cannot be seeked.
* **Media extras**: `ItemService.GetItem` lists an item's trickplay sheets and media segments; sheets are served at `/images/trickplay/{item}/{width}/{sheet}.jpg` and chapter images at `/images/chapters/{item}/{source}/{index}`, with the image parameters, from the metadata folder, at the signed URLs `GetItem` returns. Items carry the normalization gain of their measured loudness. `ItemService.GetLyrics` reads the lyric file beside a track (`.elrc`, `.lrc` or `.txt`, in a legacy character set if need be) with `metadata.ParseLyrics`: LRC lines with their times and ID tags, enhanced LRC words as cues in UTF-16 code units, as Jellyfin's parsers read them. A playback lists the files attached to its source, such as the fonts of ASS subtitles that clients render; ffmpeg extracts each once at `/media/{playback}/attachments/{index}`.
* **Images**: artwork is served at `/images/{id}`, with Jellyfin's size parameters (`width`, `height`, `maxWidth`, `maxHeight`, `fillWidth`, `fillHeight`, `quality`); clients fetch it at the signed URL of the image in API responses. Local images are read within their item's library folder or the metadata folder; provider images are downloaded once into the cache directory. Without size parameters the original is served; renderings from `imaging.Process` are in the requested `format` (`jpg`, `png`, `webp`), or WebP for clients whose `Accept` header takes it, else JPEG (PNG with transparency). They are cached under the source's content hash and the parameters, and concurrent requests for one rendering share it. SVGs are served as they are once `CheckSVG` passes.
* **Collages**: `/images/collages/{id}` composes, with the same parameters, the image of a library (its latest items' posters side by side at 16:9), a collection (its items' posters in a 2 × 2 grid at 2:3) or a playlist (the posters of its items' series and albums, or its items', in a square grid), as Jellyfin's dynamic images do; libraries carry the signed URLs of their collages. A collage is cached under the images it shows, so it follows changes; with no image to show it is not found.
* **Development player**: with `--dev`, the server also serves `/dev/player`, a single page that signs in, lists videos and plays them through `PlaybackService`, with hls.js or Safari's own HLS player (`?engine=native|hlsjs`) and an optional streaming bitrate cap that makes videos transcode (`?bitrate=<bits per second>`), for checking playback on real browsers before the clients exist. `-dev-library <dir>` adds the `Movies` and `Shows` folders of the sample library `pnpm nx run fixtures:dev-library` generates.
* **API reference and reflection** (`internal/apidocs`): `/api/spec.json` serves the OpenAPI document of `libs/proto/openapi` (the Connect services the server serves, JSON only, and the plain HTTP media routes of `openapi/base.yaml`), with the server's version and the public procedures marked as needing no token. `/api` renders it with Scalar, loaded from the jsdelivr CDN at a pinned version, against the address the page was opened at, base URL included. The server also answers gRPC reflection (v1 and v1alpha) for every service. All three need no token; they describe the API but grant nothing.
* **Subtitles**: text subtitles delivered as files are served at `/media/{playback}/subtitles/{index}.{format}`, converted by `libs/subtitle` from an external file in the library or from the embedded stream, which ffmpeg extracts with every other embedded text stream of the file once while it is unchanged, into the item's metadata folder (ASS keeps its styling, other text becomes SRT). On cloud storage embedded streams are not extracted: they reach such clients as HLS renditions that the transcode writes as it reads the video ([Optimization §2.6](optimization.md#26-embedded-subtitles)). For HLS clients that take subtitles in the manifest, each text subtitle is a rendition of the master playlist: a WebVTT playlist with its language as a BCP 47 tag, segmented along the video: each segment, `subtitles/{index}-{segment}.vtt`, holds the cues shown during it, timed on the source's timeline as the video is, and the last also those past the end of the media. Image subtitles (PGS, VobSub) are only burned in; external files the server cannot write, or extract without ffmpeg, are dropped.

---

## 12. Distribution

* **Server binaries**: `pnpm nx run server:dist` cross-compiles `cmd/mavio` with `CGO_ENABLED=0` and `-trimpath` for Linux, macOS and Windows on amd64 and arm64 into `apps/server/dist/`, stamping `$VERSION` into `internal/buildinfo` when set; every binary is static.
* **Container image** (`apps/server/Dockerfile`, built from the repository root): the server is cross-compiled on the build machine's platform, so linux/amd64 and linux/arm64 images build without emulation of Go. The image bundles jellyfin-ffmpeg's portable build in `/usr/lib/jellyfin-ffmpeg`, the version `mise.toml` pins, checked against GitHub's SHA-256 digest of the release asset, on Debian slim with CA certificates and time zone data, and Mavio's fork of rclone in `/usr/local/bin/rclone`, the Linux build of the release `mise.toml` pins, checked against its SHA-256 digest, which the server runs as its rcd (§9.2) with its config in `/config/rclone/rclone.conf`.
* **Image layout**: the server runs as `mavio` (uid and gid 1000) on port 8686, answering discovery on UDP port 7359, with its home in `/config` (`config.toml`, the SQLite database, plugin folder, chosen artwork and backups), images, compiled plugins and transcodes in `/cache`, and media libraries mounted under `/media`. The image sets these and the jellyfin-ffmpeg binaries through `MAVIO_*` environment variables, which flags after the image name and `-e` variables override. The DLNA plugin is built with the server and seeded from `/usr/lib/mavio/plugins`; its SSDP on UDP port 1900 reaches the container only with `--network host`. The indexers, download clients, subtitles and agent plugins are seeded alike, the agent plugin compiled by Bun for the image's platform; with no indexers configured the first only fetches the Cardigann definitions daily, and with no clients, providers or credentials configured the others do nothing. A health check calls `SystemService.GetHealth`.

---

## 13. Acquisition (libs/acquire)

Mavio finds, downloads and imports the movies and series its users want, as Seerr, Radarr, Sonarr and Prowlarr do beside Jellyfin, in one server: one catalog of items, one set of users, one configuration. The work is phases P15 to P20 of the [Roadmap](roadmap.md).

### 13.1 Parts
| Part | Where | Counterpart |
| :--- | :--- | :--- |
| Discovery | `DiscoverService`; lists from `DISCOVERY_PROVIDER` plugins (`scraper-tmdb`, `scraper-servarr`) | Seerr's discovery |
| Requests and issues | `RequestService`, `IssueService`; rules in `apps/server/internal/requests` | Seerr's requests and issues |
| Acquisition engine | `libs/acquire`, packages `movie` and `series`, `organize` naming files; services in `apps/server` | Radarr, Sonarr |
| Wanted items | `WantedService`: movies added by TMDB ID and series by TVDB ID, searches, RSS sync and scene mapping updates as jobs of `apps/server/internal/acquisition` | Radarr's movies, Sonarr's series, their searches |
| Profiles | `ProfileService`: quality definitions and profiles, custom formats, delay and release profiles, remote path mappings | Radarr's and Sonarr's settings |
| Release parsing | `libs/release` | Radarr's and Sonarr's parsers |
| Servarr's metadata | `plugins/scraper-servarr` (WASM) | Radarr's and Sonarr's metadata sources, scene mappings |
| Indexers | `plugins/indexers` (process), `INDEXER_PROVIDER`; searched through `ReleaseService` | Prowlarr |
| Download clients | `plugins/download-clients` (process), `DOWNLOAD_CLIENT_PROVIDER`; reached through `DownloadService` | Radarr's and Sonarr's download clients |

* **State in the server, networks in plugins.** Wanted items, profiles, decisions, the queue, history, imports and requests are the server's, next to the items and users they concern. What talks to indexers and download clients, and changes with them, is in plugins, upgraded from catalogs without a server release.
* **One engine.** Movies and series share quality profiles, custom formats, decisions, the queue and imports; the `movie` and `series` packages differ in how they search, monitor and match releases to items. The shared model is the root package `acquire`: Radarr's quality IDs and the qualities' definitions (Radarr's defaults for movies, Sonarr's for series), quality profiles with their groups, cutoffs and format scores, custom formats with every specification of Radarr and Sonarr's release type, delay profiles and Sonarr's release profiles. Conditions on sources use Radarr's sources and modifiers for series too, Sonarr's raw sources being Radarr's Raw-HD and remux modifiers.
* **Plugins do not call each other.** The server gathers what the provider plugins give, as it does with subtitle providers, and calls the indexers and download clients itself.

### 13.2 Data Sources and Numbering
* **Servarr's services**, without keys, are the default sources: Radarr's metadata service (`api.radarr.video`) for movies and lists, Sonarr's SkyHook (`skyhook.sonarr.tv`) for series and episodes, Sonarr's services (`services.sonarr.tv`) for scene mappings and daily series, XEM (`thexem.info`), and Prowlarr's indexer definitions (`indexers.prowlarr.com`). Requests to a Servarr service carry the User-Agent of the application whose service it is, as that application builds it (`Sonarr/<version> (<os> <os version>)`).
* **TMDB**: `scraper-tmdb` uses an administrator's own API key when one is set, else Radarr's TMDB read access token; when TMDB rejects that token, it falls back to Mavio's own API key, which the plugin ships.
* **Numbering on disk follows TVDB**, as Sonarr's does: a series' episodes come from SkyHook in TVDB order with absolute numbers and air dates, files are named by those numbers (anime also by absolute number), and requests list seasons in that order. A series has a type, standard, daily or anime, which decides how releases are matched.
* **TMDB's metadata** is attached through the series' display order, Jellyfin's `DisplayOrder` over TMDB's episode groups, and TMDB's episode TVDB IDs where they exist.
* **Releases named otherwise** are matched through Sonarr's scene mappings and XEM; daily series by air date. The scene numbering XEM gives a wanted series is kept on its episodes (`EpisodeState`), extrapolated to the episodes it leaves out as Sonarr's `XemService` extrapolates it, when the series is added and with the scene mappings every 3 hours.

### 13.3 Flow
1. **Request**: a user requests a movie or seasons; approved by an administrator or automatically within the user's quota, and only within the user's rating limit. The request makes the movie or episodes wanted, monitored with a quality profile.
2. **Search**: a wanted item is searched at once; RSS sync and searches for wanted items and upgrades run as jobs on their intervals.
3. **Releases**: `ReleaseService` asks the indexers for a movie or episodes by their IDs, numbers and categories; `libs/release` parses each release, which is matched to items by IDs, titles and scene mappings.
4. **Decision**: each release goes through the specifications of Radarr's and Sonarr's decision engines (quality allowed, size, custom format score, language, already queued, upgrade allowed, delay, blocklist, age, seeders) and the accepted ones are ranked.
5. **Download**: the best release goes to a download client through `DownloadService`, with the category of its kind; the queue tracks it by polling the clients.
6. **Import**: a completed download, with remote path mappings applied, is matched to its items again and verified with ffprobe; its files are hard linked into the library when they share a volume, else copied or moved, under the library's naming template, holding the volume's slot of `VolumeLedger` and yielding to playback through `QuietGate`. The folder imported to is scanned, rather than the library, and the wanted items become items with media. A download the import specifications block, or a folder that is not a download, can be imported by hand through `DownloadService` (`ListManualImport`, `ManualImport`), choosing the file, the movie, its quality, languages and release group, and whether it is moved or copied.
7. **Upgrades and failures**: a better release replaces a file until the profile's cutoff, the replaced file going to a recycle folder; a failed or mismatched download is blocklisted and searched again.
8. **Notification**: the request becomes available and its requester is notified through the activity log.

### 13.4 Indexers (`plugins/indexers`)
A first-party process plugin porting Prowlarr: Newznab and Torznab, the Cardigann engine with definitions fetched from `indexers.prowlarr.com` at run time, and every site Prowlarr implements in code; capabilities and categories, rate limits and backoff after failures, indexer proxies (HTTP, SOCKS, FlareSolverr). Its configuration lists the configured indexers; the definitions and the fields each takes come from the plugin. `IndexerProviderService` lists the indexers with their capabilities, searches them (movie, series or text, with IDs, season and episode, categories), reads their RSS feeds and fetches a release's torrent file, magnet link or NZB.

### 13.5 Download Clients (`plugins/download-clients`)
A first-party process plugin with every download client Radarr and Sonarr support, such as qBittorrent, Transmission, Deluge, rTorrent, Aria2, SABnzbd, NZBGet and a watch folder. Its configuration lists the configured clients, each with a category for movies and one for series. `DownloadClientProviderService` adds a release (torrent file, magnet link or NZB, in the category and with the priority of its kind, and with the seeding limits of its indexer), lists downloads with their state, progress and output path, marks them imported and removes them. Remote path mappings are the server's. Cloud drives' offline downloads are download clients built into the server (§9.2).

### 13.6 Storage
Wanted state, quality definitions and profiles, custom formats, delay and release profiles, the queue, history, the blocklist and requests are in `libs/store`, on both databases, behind ports in `core`.

### 13.7 Discovery, Requests and Issues
The package `apps/server/internal/requests` carries Seerr's rules out; `DiscoverService`, `RequestService` and `IssueService` wrap it, and `server.go` wires it to the acquisition runtime, the metadata providers, the episode listers, the discovery providers and the activity log.
* **Permissions** are booleans of the user policy: request movies, request series, auto-approve each, report issues; and quotas of movies, or of seasons of series, in a number of days, each user's own or the server's. Administrators are Seerr's request, issue and user managers: they approve, decline and retry requests, request for others, ignore quotas, and are unlimited. The server's request settings give new users their permissions and say where approved requests go: a movie library and quality profile with a minimum availability, a show library and quality profile with season folders and the monitoring of new seasons, a library and quality profile of their own for anime series, and whether specials may be requested.
* **Titles** are looked up by TMDB ID through the metadata providers in the country and language of the server's metadata settings, and cached for hours: their name, year, rating, overview, genres, tags, images and, for series, TVDB ID. A series tagged anime is added as an anime series, with the anime library and quality profile when set. The episode listers give a series' seasons.
* **The blocklist**: administrators blocklist movies and series by TMDB ID, as Seerr's, or the movies of a collection, which a discovery provider's collection catalog lists; a blocklisted title is marked blocklisted, above every other status, and requests of it are refused. The weekly blocklisted tags job (`requests.blocklisted_tags`), as Seerr's, takes the titles blocklisted by tags off the blocklist, then blocklists the movies and series of each blocklisted tag, read from its provider's popular catalog filtering by keyword in every order up to the settings' limit of pages (or in its own order when there are fewer pages), with a pause of 250ms after each query; titles blocklisted by hand stay untagged, and keywords the provider does not know are dropped from the settings. The titles are read before the blocklist is changed in one transaction.
* **Creating a request** checks, in Seerr's order: the blocklist, the permission, the rating limit (the title's rating scored over that country's system; unrated passes unless the policy blocks unrated items), the quota (declined requests and those ignoring quotas do not count), then duplicates: a movie requested, wanted or available is refused; of a series, the seasons other active requests ask for and those wanted or with files are left out, and the remaining seasons must fit the quota. The request is then pending, or approved at once when the user's requests of that kind are. A user lock and a title lock serialize the checks and the write, so concurrent requests neither exceed a quota nor duplicate a title.
* **Approval** makes the title wanted: a movie through `AddMovie` with a search; a series through `AddSeries` monitoring nothing, then its requested seasons monitored and searched for. A title the libraries have already is made wanted, its seasons added. A failure marks the request failed with the error, for an administrator to retry.
* **Availability**: the acquisition runtime's import hook, and a recurring job every ten minutes for files found by scans, complete approved and failed requests whose movie has its file, or whose seasons' aired episodes all have files, season by season. The job is listed among the tasks and can be run at once.
* **Discovery** merges the providers' catalogs as `<plugin>/<catalog>` and marks each title of a page as available, partially available, wanted, requested, blocklisted, deleted (a completed request's files are gone) or nothing, with its item, its active request and whether the user may request it. `SearchTitles` finds titles by name through the first provider that searches, marked alike, and `GetTitle` details a title with the status of each season. The user's rating limit applies: catalogs filtering by certification are asked only for the certifications within it when the caller asks for certifications or the policy blocks unrated titles, so that unrated titles are otherwise kept; the other titles are looked up and those above it left out. Adult titles are never listed. A page counts the titles it leaves out, and its total is the provider's less them.
* **Issues** report a video, audio, subtitle or other problem with an item the user can access, optionally of a season and episode, with a thread of comments. Their creators and administrators comment, resolve and reopen them; administrators, or the creator of an issue without replies, delete them.
* **Notifications** are activities whose user is the one concerned: `request.pending` (with administrators' notifiers in mind), `request.approved`, `request.auto_approved`, `request.declined`, `request.failed`, `request.available`, `issue.created`, `issue.commented`, `issue.resolved` and `issue.reopened`, with the request or issue in their attributes. Notifier plugins deliver them, `ActivityService.ListMyActivities` lists a user's own, and a `NotificationReceived` event reaches the sessions of the user concerned as they are recorded, and administrators' for requests pending or failed and issues reported to them.

---

## 14. Subtitle Acquisition (libs/acquire/subtitles)

Mavio finds the subtitles its movies and episodes lack, as Bazarr does beside Sonarr and Radarr, in the server: what Bazarr syncs from Sonarr and Radarr are the server's own items, and its index of subtitles is the streams scans and probes record. The work is phase P26 of the [Roadmap](roadmap.md).

### 14.1 Parts
| Part | Where | Counterpart in Bazarr |
| :--- | :--- | :--- |
| Subtitle profiles, history, blocklist, failed attempts | `libs/core`, `libs/store` | Language profiles, `table_history*`, `table_blacklist*`, `failedAttempts` |
| Rules: wanted subtitles, scores, the choice of the best, upgrades, adaptive searching | `libs/acquire/subtitles` (pure functions) | `list_missing_subtitles`, `subliminal_patch/score.py`, `download_best_subtitles`, `upgrade.py`, `adaptive_searching.py` |
| Runtime and API | `apps/server/internal/subtitles`, `SubtitleService` | `subtitles/wanted`, `upgrade`, `manual`, the scheduler's jobs |
| Providers, translators | `plugins/subtitles` (process), `SUBTITLE_PROVIDER`, `SUBTITLE_TRANSLATOR` | `subliminal_patch/providers`, `pitcher.py`, the throttling of `get_providers.py`, `subtitles/tools/translate` |
| Mods, encodings, language detection | `libs/subtitle` | subzero, character set detection, `guess_language` |
| Synchronization | `libs/media/subsync`, speech detected by Silero VAD run in Go (`libs/media/vad`) | ffsubsync with its Silero detector |

* **The server's own data stands in for Bazarr's sync.** Movies, series and episodes are the server's items; the release a file came from is the grab its import recorded (Bazarr's `sceneName` and history refiner), else the file's name; release names are parsed by `libs/release`, in place of guessit; codecs, resolution and audio languages come from the stored probe. Bazarr's Sonarr and Radarr sync, path mappings, Plex and Jellyfin refreshes, Apprise, scheduler, backup and announcements have their counterparts in imports and scans, the item's streams, the activity log with notifier plugins, `TaskService` and P11.
* **Rules apart from networks**, as acquisition (§13): profiles, decisions, history and the blocklist are the server's; what talks to subtitle sites, and changes with them, is in the plugin, upgraded from the catalog without a server release.
* **Mods and detection** (`libs/subtitle`): `Subtitle.ApplyMods` applies Subzero's mods to the parsed cues by their Bazarr identifiers (`common`, `remove_tags`, `remove_HI`, `OCR_fixes`, `color(name=…)`, `emoji`, `reverse_rtl`, `fix_uppercase`, `shift_offset(…)`, `change_FPS(…)`, `two_point_fit(…)`) as Subzero does: whole-file mods before or after the line mods, line mods line by line in Subzero's order, the last occurrence of an exclusive mod, time offsets added up, mods for other languages and the upper-case fix of subtitles not mostly upper case left out, cues left empty dropped; it returns the mods used as Subzero reports them. SSA and ASS files keep, as pysubs2 keeps them for Subzero, their script info, styles, comments, other sections and each event's other fields: written in their own format, only the timing and text of the cues that changed are rewritten, and dropped cues removed. The mods' regular expressions are Subzero's, run by regexp2. `DetectHearingImpaired` applies Bazarr's expressions to a file's text, parentheses not counting in Arabic and Persian. `DetectLanguage` guesses the language of a sidecar without one as guess_language does, from Unicode blocks and trigram models, without spelling dictionaries; Chinese is told apart by the character sets of hanzidentifier, `zh-Hans` or `zh-Hant`, and `zh` when only characters shared by both scripts appear. The OCR fix dictionaries (Subtitle Edit's lists with Subzero's additions), the emoji, the public suffix list by which the common fixes leave domain names alone, guess_language's models and CC-CEDICT's character sets are generated from their sources by `internal/cmd/subzerogen`, which records each origin and license. Tests compare the results with those of Bazarr's own code (`testdata/mods/golden.py`). `FixText` repairs text as ftfy's `fix_text` with Bazarr's options, which Bazarr applies to every subtitle it saves: HTML entities decoded unless the text holds a `<`, mojibake undone by ftfy's badness heuristic, C1 controls, Latin ligatures, line breaks, surrogates, terminal escapes and control characters fixed, and NFC normalization, curly quotes and full-width characters kept; its character tables are generated by `internal/cmd/ftfygen` from ftfy as Bazarr vendors it and CPython's codecs, and tests compare it with ftfy (`testdata/fixtext/golden.py`).

### 14.2 Flow
1. **Wanted**: when a movie or episode file is imported, found by a scan or probed, or its subtitle profile changes, the subtitles the profile wants are compared with the file's subtitle streams, external and, where the settings count them, embedded, as Bazarr's `list_missing_subtitles`: languages excluded when the audio already has them or wanted only when it has them, the cutoff, hearing-impaired subtitles standing in for normal ones unless the profile excludes them, and Bazarr's language equivalences. What each movie and episode is missing is kept (`core.SubtitleWanted`, one row per item), so that the wanted lists and searches read no item's streams: searches, downloads, uploads, deletions and extractions update their item's row, scans update the rows of items whose subtitle files alone changed (`library.Scanner.OnSidecars`), and a refresh of every row (`subtitles.wanted_refresh`) runs at startup and after profiles, their assignments or the settings change; rows of missing items are left out of the lists until their files are found again.
2. **Search**: wanted subtitles are searched once a new or changed file of a movie or episode, imported or found by a scan, is probed and its item refreshed (`library.Jobs.Probed`), so that the search knows its streams and its IDs, and by the wanted search jobs of movies and episodes; after the first failure, adaptive searching searches a language only once per interval after a delay, as Bazarr's `is_search_active`.
3. **Query**: the video is described by its item's names, alternative titles, IDs, numbering (season, episode, absolute number), year, the release name and its parsed source, release group, resolution, codecs, edition and streaming service, the frame rate and the OpenSubtitles hash; the library's subtitle providers are asked in its order ([Plugins §8.11](plugins.md#811-provider-order-per-library)), for every wanted language at once.
4. **Scores**: each candidate carries the properties its provider found matching; the server scores them with Bazarr's weights (episodes out of 360, the hash 359; movies out of 180, the hash 179; a hash counting only when the other properties agree, IMDb and TVDB IDs standing for what they imply), and the best one at or above the minimum score is taken, honoring the profile's hearing-impaired mode, its words that must and must not appear, and for episodes a matching series, season and episode.
5. **Saving**: the subtitle is decoded to UTF-8, modified by the mods the settings name, repaired by `subtitle.FixText` (saved as UTF-8 when that changes it), flagged hearing-impaired when its text shows it, and written through `core.Storage` beside the video, in a folder of the video's folder (the `relative` setting, a single folder name such as `Subs`) or in the item's folder within the metadata folder (the `metadata` setting, `<id[:2]>/<id>/subtitles/`, which takes the place of Bazarr's absolute folder and suits libraries that cannot or should not be written to, such as cloud storage) as `<video>.<language>[.forced|.hi|.cc|.sdh].<ext>`, aside and renamed; it becomes an external stream of the item without probing the video again.
6. **History and notification**: the download is recorded with its provider, score and matches, and a `subtitle.downloaded` activity is logged; a subtitle that fails to decode or that an administrator rejects is blocklisted, and another is searched for.
7. **Upgrades**: the upgrade job searches again, for the days the settings give, the subtitles downloaded below their maximum score, accepting only better ones, as Bazarr's `upgrade_subtitles`; the replaced file is deleted and the history links the two.
8. **By hand**: administrators search a video's subtitles with their scores and matches, download, upload, delete, modify and synchronize them, and extract embedded ones where §14.3 allows.

### 14.3 Storage Classes
Subtitle acquisition reads media under the policy of [Optimization §2.3](optimization.md#23-policy-by-storage-class):
* The OpenSubtitles hash, two range reads, is computed once per file and kept until the file changes, so that repeated searches read nothing.
* The probe's media information is used; nothing is probed again for subtitles.
* Embedded text subtitles are extracted to sidecars only by hand and only on local and network storage, from the extraction cache of playback ([Optimization §2.6](optimization.md#26-embedded-subtitles)), filled by its single ffmpeg run when empty; on cloud storage they are never extracted.
* Synchronization against the audio reads the whole audio track, so it runs only on local and network storage; on cloud storage a subtitle is synchronized only by hand against another subtitle file of the video, whose cues stand for the speech, which reads no video on any storage.
* Language detection and mods read only the subtitle files.
* Whisper's audio (§14.5) is served only on local and network storage, and embedded subtitles are translated only there; on cloud storage Whisper finds nothing and only subtitle files are translated.

### 14.4 Subtitle Providers (`plugins/subtitles`)
A first-party process plugin porting Bazarr's providers: subtitle sites behind cookies, logins and Cloudflare (through the indexers' kinds of proxies and FlareSolverr), captchas solved by Anti-Captcha, CaptchaAI or DeathByCaptcha with the administrator's account, archives (zip, RAR, 7z) and self-hosted providers at addresses a WASM manifest cannot list. Its configuration enables providers with their settings; `SubtitleProviderService` lists them, searches them for a video in several languages, returning candidates with their release names, flags and matched properties, and downloads one. Providers failing are throttled for Bazarr's durations by kind of failure (too many requests, download limit, service unavailable, authentication), some until midnight in their site's time zone, counted failures only after five in two minutes; their state is kept in the plugin's data folder. OpenSubtitles.com is one of its providers.

### 14.5 Translation and Transcription
* **Translation** ports Lingarr. An administrator translates an external subtitle file of a movie or episode, or an embedded text subtitle stream on local and network storage (from the extraction cache of playback), into a language with its forced and hearing-impaired flags, as a `subtitles.translate` job. Translators are AI models, `ai/<provider>/<model>`, completed through the plugin with the `AI_GATEWAY` capability ([Plugins §8.15](plugins.md#815-ai-gateway-and-agent-sessions)), which holds every credential, and the translators of `SUBTITLE_TRANSLATOR` plugins ([Plugins §8.14](plugins.md#814-subtitle-translators)), `<plugin>/<translator>`. The chain is the translator named, else the settings' translator, else the first enabled plugin translator, followed by the settings' fallbacks; it is ordered by how well each translator's languages match the source and target (exact, alias, script, a parent such as the base language, a variant; never a sibling region, so pt-PT does not stand for pt-BR), and a chunk of cues one translator fails goes to the next.
  * AI models translate a cue at a time with Lingarr's prompts, the cues before and after as context (with their translations when the settings say so), or in batches answered as JSON against an output schema. Rate-limited and unavailable requests, and batches answered with malformed JSON, are retried with a growing delay, each attempt within the request timeout. A cue answered with nothing keeps its text and is given as context untranslated. Plugin translators get batches of their size, as SubRip when they keep its markup, else as plain text. Cues are translated without their markup and SSA vector drawings (fansub signs), cues with nothing else left out; lines are joined or kept as the settings say, stacked cues are translated once, and answers are split back into lines of at most 42 characters.
  * The translation keeps the subtitle's format (SubRip, ASS, SSA, WebVTT) when the profile keeps original formats and formatting is not stripped, else is SubRip. Timing and styles are kept; overlapping cues are fixed and the translation validated (size, cue count, lengths, durations, order, control characters) when the settings say so. When the settings say so a SubRip translation is credited by a cue before its first (`# Subtitles translated with … #`), as Bazarr's `add_translator_info`. The translation is saved as downloads are (§14.2) and recorded in the history as `translated`, scored at the settings' default percentage of the maximum score, with its record: source, translators, cues, characters, tokens and cost.
  * **Proofreading** (`subtitles.proofread`) has an AI model, the one named or else the first that translated the file, compare each translated cue with its source and replace it with its correction. Cues pair with the source's by order when the file has as many (its credit aside), else by exact timing, else with the source cues overlapping at least half of the shorter, so a file synchronized or edited since its translation is still proofread; the record notes the cues revised.
  * **Automation** (the `subtitles.translate_wanted` task, every interval when enabled) translates what movies and episodes lack, taking them in turns from where the last run stopped, up to the settings' maximum per run, once their files are old enough, from a subtitle in the first source language they have: an external file before an embedded text stream, never a forced one, and one not for the hearing impaired before one that is. A translation pending or running for the same item and language is not queued again.
  * **Statistics** (`GetSubtitleTranslationStatistics`) add up the records: files, cues, characters, tokens and cost, by translator, language pair and media type, and the files of each of the last days.
* **Transcription** is Whisper's, a provider of `plugins/subtitles` as in Bazarr: it transcribes the audio in its own language or translates it to English, never to other languages and never through translators. The audio's language comes from the audio streams' tags; Whisper detects it from the first 30 seconds of the first stream when no stream is tagged or a tag is ambiguous (`und`, `mul`, `zxx` and the like); with several streams, the one in the subtitle's language is transcribed. The plugin streams the audio from the host API ([Plugins §8.13](plugins.md#813-subtitle-providers)) to a whisper-asr-webservice server, which answers with SubRip.

# Mavio Roadmap

> English | [简体中文](roadmap.zh-CN.md)

> Related: [Architecture](architecture.md) · [Domain Model](domain.md) · [Development](development.md) · [Testing](testing.md) · [Optimization](optimization.md) · [AGENTS.md](../AGENTS.md)

---

## 1. Principles

* **Bottom-up**: build the low-level libraries with no business dependencies first, then assemble layer by layer; each layer depends only on completed layers below it.
* **Definition of done**: a phase is complete when its libraries pass their tests, including all ported test cases (or cases explicitly marked skip with a reason), see [Testing §2](testing.md#2-porting-test-cases).
* **Server first**: P7 to P12 complete the server, by comparing it with Jellyfin's server (its API controllers and providers); P13 and P14 bring plugins level with Jellyfin's and add DLNA; P15 to P20 add acquisition, what Seerr, Radarr, Sonarr and Prowlarr do beside Jellyfin ([Architecture §13](architecture.md#13-acquisition-libsacquire)); the clients follow in P21, whose foundation is built alongside P19 and P20 on the browsing and playback API already finished; P22 verifies playback end to end on real devices; P23 packages and publishes the releases.
* **Integration smoke tests**: starting from P1, the end of each phase adds an integration test in `apps/server/internal/smoke` that wires the completed libraries together. It is not an MVP; it only exposes interface drift early, reducing the bottom-up approach's risk of "discovering problems only at final integration".

---

## 2. Phase Overview

```mermaid
flowchart TD
    P0["P0 Engineering foundation<br/>Nx / go.work / buf / mise / CI / test media generation / testport"]
    P1["P1 Contracts, storage and plugin runtimes<br/>libs/proto, libs/core, libs/store, libs/plugin"]
    P2["P2 Pure computation libraries<br/>libs/naming, libs/subtitle, libs/metadata, libs/imaging"]
    P3["P3 Media pipeline<br/>libs/media: probe, keyframes, hwaccel, decision, planner, supervisor"]
    P4["P4 Scanning and first plugin<br/>libs/library, plugins/scraper-tmdb"]
    P5["P5 Streaming and API<br/>libs/streaming, Connect services"]
    P6["P6 Server assembly and distribution<br/>apps/server, single binary + jellyfin-ffmpeg, container images"]
    P7["P7 Browsing<br/>views, latest, next up, value lists, collections and playlists, collages"]
    P8["P8 Live events and sessions<br/>event stream, sessions across devices, remote control, SyncPlay, Quick Connect"]
    P9["P9 Storage and performance optimization<br/>device detection, anti-thrashing, quiet gate, prefetching, subtitle scoring, DV P7"]
    P10["P10 Metadata management<br/>editing, identification, images, NFO writing, more providers"]
    P11["P11 Administration and operations<br/>server settings, jobs, plugin hot plugging and catalog, backup, networking"]
    P12["P12 Media extras<br/>trickplay, chapter images, media segments, lyrics, fonts, normalization, offline downloads"]
    P13["P13 Plugin platform<br/>host API, events, HTTP routes, tasks, remote devices, more provider types"]
    P14["P14 DLNA<br/>plugins/dlna: media server, Play To, device profiles"]
    P15["P15 Acquisition foundation<br/>wanted items, display orders, series types, libs/release"]
    P16["P16 Acquisition data sources<br/>plugins/scraper-servarr, discovery in scraper-tmdb"]
    P17["P17 Indexers<br/>plugins/indexers, ReleaseService"]
    P18["P18 Movie acquisition<br/>libs/acquire, plugins/download-clients"]
    P19["P19 Series acquisition<br/>season packs, anime, daily series, scene mappings"]
    P20["P20 Discovery and requests<br/>DiscoverService, RequestService"]
    P21["P21 Clients and ecosystem<br/>apps/web, Apple, Android and desktop apps, libs/client-rust, Jellyfin shim decision"]
    P22["P22 Real-device acceptance<br/>end-to-end playback on real televisions and clients"]
    P23["P23 Packaging and release<br/>libs/proto and libs/plugin tags, server binaries and container images"]
    P24["P24 Storage I/O efficiency<br/>storage classes, container indexes, keyframe trickplay, subtitle caching"]
    P25["P25 Storage backends and rclone<br/>storage port, libs/rclone, rcd, offline downloads"]
    P26["P26 Subtitle acquisition<br/>libs/acquire/subtitles, plugins/subtitles, porting Bazarr"]

    P0 --> P1
    P1 --> P2
    P1 --> P3
    P2 --> P4
    P3 --> P5
    P4 --> P5
    P5 --> P6
    P6 --> P7
    P7 --> P8
    P8 --> P9
    P8 --> P10
    P8 --> P11
    P8 --> P12
    P9 --> P13
    P10 --> P13
    P11 --> P13
    P12 --> P13
    P13 --> P14
    P14 --> P15
    P15 --> P16
    P15 --> P17
    P16 --> P18
    P17 --> P18
    P18 --> P19
    P19 --> P20
    P20 --> P21
    P14 --> P22
    P21 --> P22
    P22 --> P23
    P9 --> P24
    P12 --> P24
    P24 --> P25
    P18 --> P25
    P19 --> P26
    P24 --> P26
```

| Phase | Theme | Status |
| :--- | :--- | :--- |
| P0 | Engineering foundation | ✅ Done |
| P1 | Contracts, storage and plugin runtimes | ✅ Done |
| P2 | Pure computation libraries | ✅ Done |
| P3 | Media pipeline | ✅ Done |
| P4 | Scanning and first plugin | ✅ Done |
| P5 | Streaming and API | ✅ Done |
| P6 | Server assembly and distribution | In progress |
| P7 | Browsing | ✅ Done |
| P8 | Live events and sessions | ✅ Done |
| P9 | Storage and performance optimization | ✅ Done |
| P10 | Metadata management | ✅ Done |
| P11 | Administration and operations | ✅ Done |
| P12 | Media extras | ✅ Done |
| P13 | Plugin platform | ✅ Done |
| P14 | DLNA | ✅ Done |
| P15 | Acquisition foundation | ✅ Done |
| P16 | Acquisition data sources | In progress |
| P17 | Indexers | ✅ Done |
| P18 | Movie acquisition | ✅ Done |
| P19 | Series acquisition | ✅ Done |
| P20 | Discovery and requests | ✅ Done |
| P21 | Clients and ecosystem | In progress |
| P22 | Real-device acceptance | Not started |
| P23 | Packaging and release | Not started |
| P24 | Storage I/O efficiency | In progress |
| P25 | Storage backends and rclone | In progress |
| P26 | Subtitle acquisition | Not started |

---

## 3. Phase Details

### P0 Engineering Foundation
**Scope**: repository skeleton, mise, Nx, go.work, buf, golangci-lint + depguard; CI matrix (linux amd64/arm64, macOS, Windows); `tools/fixtures`, `tools/testport`.

**Done when**: `nx affected` works; `tidy-check` (`GOWORK=off`) passes.

**Progress**:
- [x] Go module skeleton and `go.work`; Nx with the local plugin `tools/nx-go` (infers projects and the dependency graph)
- [x] Toolchain versions pinned with mise
- [x] buf and the first contract `mavio.system.v1.SystemService`; `apps/server` serves a Connect health check
- [x] golangci-lint and depguard dependency-direction rules
- [x] CI workflow: affected-project checks, generated-code check, `buf breaking`, multi-platform test matrix
- [x] `tools/fixtures`: test media generation
- [x] `tools/testport`: Jellyfin test case extraction

### P1 Contracts, Storage and Plugin Runtimes
**Scope**: first version of the `proto` contracts; `core` domain model; `store` (ent + Atlas + sqlc, both dialects) and the job queue; `plugin` with both runtimes and the handshake.

**Done when**: repository conformance tests pass on both SQLite and PostgreSQL; a crashing plugin does not affect the host and is restarted automatically.

**Progress**:
- [x] `libs/core`: domain model and repository ports ([Domain Model](domain.md))
- [x] `libs/proto`: first version of the contracts (library, user, system, plugin)
- [x] `libs/store`: SQLite and PostgreSQL implementation of the ports, including the job queue
- [x] `libs/plugin`: WASM and child-process runtimes
- [x] Smoke test `apps/server/internal/smoke`: a queued metadata refresh fetched from a plugin (both runtimes), stored with its credits and found by search

### P2 Pure Computation Libraries
**Scope**: `naming`, `subtitle`, `metadata` (NFO), `imaging`.

**Done when**: all corresponding ported cases pass.

**Progress**:
- [x] `libs/naming`: Jellyfin's naming rules for movies, episodes, seasons, series, stacks, versions, extras, music, audiobooks, books and external files; all ported cases pass or are skipped with a reason
- [x] `libs/subtitle`: SRT / SSA / ASS / WebVTT parsing, conversion to SRT / SSA / ASS / WebVTT / TTML / JSON, time-window filtering and character set detection
- [x] `libs/metadata`: NFO reading for movies, videos, music videos, series, seasons, episodes (including multi-episode files), albums and artists; provider IDs in URLs; movie NFO locations. Writing NFO files is left to the phase that saves metadata.
- [x] `libs/imaging`: Jellyfin's size rules, resizing with sharpening on downscale, image formats and SVG safety checks. WebP encoding and placeholders (blurhash / thumbhash) come with the image API in P5, collages with browsing in P7.
- [x] Smoke test `apps/server/internal/smoke`: an episode's files are named, read from NFO, stored with their credits and found again; its subtitle is converted to WebVTT and its poster resized

### P3 Media Pipeline
**Scope**: `probe`, `keyframes`, `hwaccel`, `decision`, `planner`, `supervisor`.

**Done when**: all ported StreamBuilder and EncodingHelper cases pass; real transcode smoke tests pass on the hardware available: the maintainers' own machines and the GitHub-hosted CI runners. Vendor paths without such hardware are covered by the ported argument-derivation cases only.

**Progress**:
- [x] `probe`: ffprobe output normalized as Jellyfin does, including Dolby Vision, HDR10+ and range types; all ported cases pass
- [x] `keyframes`: keyframe times from ffprobe packet flags
- [x] `hwaccel`: version checks, codec, filter and option listings, VideoToolbox trial encodes, VA-API driver checks
- [x] `decision`: StreamBuilder with `ClientCapabilities` in place of `DeviceProfile`, and stream selection; all 307 StreamBuilder cases pass
- [x] `planner`: filter graph IR, CMAF HLS and progressive commands; software and VideoToolbox strategies; all ported EncodingHelper cases pass
- [x] `supervisor`: lifecycle, idle reaping, throttling, progress, served segments
- [x] Smoke test `apps/server/internal/smoke`: an HDR10 HEVC file is probed, direct played by a capable client and transcoded to H.264 HLS from a seek position for a web client, under supervision
- [x] Real transcodes on the GitHub-hosted runners (CI job `media`, Linux and macOS)

### P4 Scanning and First Plugin
**Scope**: `library` scanner (full reconciliation, see [Architecture §9](architecture.md#9-library-scanning-and-change-detection-libslibrary)), resolver chain, job scheduling; `plugins/scraper-tmdb` (WASM).

**Done when**: ported library resolver cases pass; scan results are identical on both databases; reconciliation converges to the correct state for additions / deletions / renames / content changes / temporarily unavailable mount points.

**Progress**:
- [x] Resolver chain, one folder at a time: movies, shows, music, books, home videos and photos, extras, `.ignore` files; ported resolver cases pass
- [x] Scanner: scan generations, missing items kept and purged after a grace period, folders pruned by modification time and file ID, unreadable folders keep their items
- [x] Jobs: scans, probes and metadata refreshes on the job queue, with leases and scheduled scans
- [x] `plugins/scraper-tmdb`: movies, series, seasons and episodes from TMDB; ported `TmdbUtils` cases pass, missing-episode cases skipped until virtual episodes are in the domain model
- [x] Adapters in `apps/server`: metadata plugins as providers, ffprobe as prober
- [x] Smoke test `apps/server/internal/smoke`: a film and a shows library are scanned as jobs with a WASM plugin as provider; additions, deletions, renames, rewritten files and an unavailable library folder converge to the same state on SQLite and PostgreSQL

### P5 Streaming and API
**Scope**: `streaming` (CMAF HLS, on-demand segmenting, seeking, segment cache); Connect services (library, playback, user, system).

**Done when**: ported HLS cases pass; playback verified on real hls.js and AVFoundation clients. Media3 is verified in P21, with the Android client.

**Progress**:
- [x] `libs/streaming`: playlists of the whole media source, RFC 6381 codec strings, segments generated on demand with restarts on seek, segments of copied video joined from one file per group of pictures; ported HLS cases pass
- [x] Authentication: first administrator, sign-in per device with hashed access tokens, argon2id passwords, `mavio.auth.v1.AuthService`, bearer-token and request-validation interceptors
- [x] Playback: `mavio.playback.v1.PlaybackService` and media endpoints for direct play and HLS remuxes and transcodes, remembered and preferred streams, resume positions and played state; verified end to end with real ffmpeg
- [x] Subtitle delivery: text subtitles as converted files, from external files or extracted with ffmpeg, and as HLS renditions
- [x] Library, item and user Connect services; the server runs the library jobs
- [x] Image endpoint: local and provider artwork, resized with Jellyfin's size rules and cached
- [x] WebP encoding (`gen2brain/vpx`) negotiated with clients; blurhash / thumbhash placeholders computed in the library jobs
- [x] Playback verified with the development player on hls.js in Chromium: direct stream, transcodes, seeking with restarts, an HLS subtitle rendition in sync, progress and stop
- [x] Playback verified with the development player on Safari's native HLS, which is AVFoundation's: direct stream, transcodes, seeking with restarts, an HLS subtitle rendition in sync. HLS subtitle renditions are WebVTT segmented along the video without `X-TIMESTAMP-MAP`; if a player misplaces them, add one derived from the video's timestamps

### P6 Server Assembly and Distribution
**Scope**: `apps/server` assembly; `CGO_ENABLED=0` cross-compilation; container images bundling jellyfin-ffmpeg's portable build, the one `mise.toml` pins. Publishing them is part of P23.

**Done when**: end-to-end smoke test: scan → scrape → playback decision → HLS playback.

**Progress**:
- [x] Assembly in `apps/server/internal/server`, which `cmd/mavio` runs with its flags
- [x] Plugins started from the plugin folder (`--plugin-dir`); their configurations stored, validated against the manifest's schema and applied through `SystemService` without a restart; metadata plugins take part in library refreshes once ready
- [x] End-to-end test of the assembled server through its API with real ffmpeg and a WASM metadata plugin: an administrator configures the plugin, a library is scanned, its film scraped and played as an HLS direct stream
- [x] `CGO_ENABLED=0` cross-compilation to Linux, macOS and Windows on amd64 and arm64 (`server:dist`), checked in CI
- [x] Container images for linux/amd64 and linux/arm64 bundling jellyfin-ffmpeg's portable build, built and run in CI

### P7 Browsing
**Scope**: what a client lists and opens.

- The user's library views; latest items per library; continue watching and next up for series
- Series, seasons and episodes; albums and artists
- Genres, studios, people and years with item counts, which `libs/store` already queries
- Collections and playlists: create, edit, reorder and delete; the domain model already has both
- Library and collection collages
- Display preferences per user and client, kept by the server so they follow the user across devices
- Inherited parental ratings

**Done when**: an end-to-end test through the API browses a scanned film and shows library through the views, latest items, next up and genres, builds a playlist, and does not show the episodes of a series rated above the user's limit.

**Progress**:
- [x] Inherited parental ratings: an unrated item takes its nearest rated ancestor's score, derived by the store on every write and backfilled for existing databases; rating filters and access checks use it
- [x] Rating scores: metadata refreshes map content ratings to scores with Jellyfin's rating systems (`metadata.RatingScore`, its ported cases pass); unrated is nil rather than zero, in items and in policies' maximum ratings
- [x] Genres, tags, studios, artists, years and people with item counts (`ItemService.ListValues`, `ListPeople`), limited to the items the user may access
- [x] Latest items (`ListLatestItems`) grouped into series, seasons and albums as in Jellyfin, and next up (`ListNextUp`) with specials in aired order; item responses name an episode's series and season and a track's album. Continue watching is `ListItems` with `resumable`, sorted by last played
- [x] Collections (`CollectionService`, administrators) and playlists (`PlaylistService`, each user their own, with entries that can repeat items and move) in curated libraries the server creates; their items listed through `ListItems` with them as parent
- [x] Display preferences per user, client and view (`DisplayPreferencesService`), with names and values the client chooses
- [x] Collages of libraries, collections and playlists (`/images/collages/{id}`, `imaging.Collage`), drawn without text
- [x] Scans refresh the metadata of new items without media (series, seasons, albums), which only media files' probes did before
- [x] End-to-end test of the assembled server through its API (`internal/server`, `TestBrowsing`): films and shows scanned with their NFO files and browsed through the views, latest items, next up and genres; a user limited to PG builds a playlist and sees no episode of a TV-14 series

### P8 Live Events and Sessions
**Scope**: what a client learns without asking, and what one device does to another.

- Events: a stream per signed-in device carrying library changes, played state and progress from the user's other devices, job progress and plugin states
- Sessions: what is playing on each device; remote control of one device from another (play, pause, seek, stop, messages)
- Synchronized playback (SyncPlay): groups sharing one play state, waiting for members that buffer
- Signing in a new device from a signed-in one (Quick Connect)

**Done when**: an end-to-end test through the API: a device receives a library change after a scan and another device's progress, and controls that device's playback; two devices in a SyncPlay group stay at the same position. Timing logic is tested with `testing/synctest`.

**Progress**:
- [x] Event streams (`EventService`): library changes, item states, sessions, commands, scans and plugins, each device getting what its user may see; the store is observed so that writes reach the streams once committed
- [x] Sessions (`SessionService`): devices online while streaming, what each plays, remote commands
- [x] Quick Connect (`AuthService`): a new device signed in by entering its code on a signed-in one
- [x] SyncPlay (`SyncPlayService`): groups of devices playing one queue at one position, waiting for buffering members, as in Jellyfin; timing tested with `testing/synctest`
- [x] End-to-end test of the assembled server through its API (`internal/server`, `TestLiveEvents`): a TV learns of the film a scan found and of a phone's playback and state, pauses the phone, and both are told to start a SyncPlay group at the same time and position

### P9 Storage and Performance Optimization
**Scope**: end-to-end I/O latency mitigation and speculative warming ([Optimization](optimization.md)); language/dialect-aware subtitle scoring; high-fidelity video pipeline enhancements; zero-fuzz release verification.

- Multi-dimensional storage medium and protocol identification: native detection of local SSDs, rotational HDDs, remote network shares (SMB/NFS), and cloud mounts across Linux, Windows, and macOS
- Anti-thrashing physical drive serialization (`VolumeLedger`): single-flight queues for rotational media, eliminating head contention during scans and probes
- Foreground streaming quiet gate (`QuietGate`): automatic yielding and throttling of background library scans and disk maintenance while client playback sessions are active
- In-progress write detection (`GrowthPolicy`): write-window and size-stability checks to avoid scanning partially downloaded or actively growing files
- Speculative index and metadata prefetching (`PrefetchHeadTail`): targeted warming of container headers and index atoms via `posix_fadvise` and sequential fallback
- Subtitle scoring and dialect normalization: language scoring matrix, Chinese variant normalization (Traditional/Simplified/regional dialects), and hearing-impaired downranking
- High-fidelity video pipeline adaptations: Dolby Vision Profile 7 EL extraction and fallback tone-mapping; black border detection (`cropdetect`)
- Security and release invariants: pinned dependency verification and fail-closed privacy checks

**Done when**: cross-platform unit and integration tests pass for storage detection, concurrency serialization, quiet gates, and prefetching on Linux, Windows, and macOS; rotational disks remain sequential during scans; playback startup cleanly interrupts background I/O; subtitle scoring correctly matches Chinese regional dialects.

**Progress**:
- [x] Multi-dimensional device and protocol identification: `libs/library/storage` with native Linux (`sysfs`/`st_dev`), macOS (`statfs`/`fsid`), and Windows (`GetDriveType`/`IOCTL_STORAGE_QUERY_PROPERTY`) implementations
- [x] Anti-thrashing volume serialization: `VolumeLedger`, one first-come-first-served slot per volume
- [x] Foreground streaming quiet gate: `QuietGate`, a quiet deadline that foreground activity extends and background reads wait out
- [x] In-progress write detection: `GrowthPolicy` leaves out files with download suffixes or modified within the write window and defers them; `Scanner.RunDeferred` rescans a library once its deferred files settle or go away
- [x] Speculative prefetch scheduler: `PrefetchHeadTail` with `posix_fadvise(WILLNEED)` on Linux and sequential chunk warming fallback
- [x] Integration of `libs/library/storage` into the scanner and probes: devices detected once per folder (`storage.Detector`); only drives with a seek penalty and remote or cloud volumes are serialized, the walker reading one folder at a time on them and each folder listing and probe holding the device; a probe of several versions on one disk no longer waits for itself
- [x] `QuietGate` and prefetching in playback: starting a playback and every media request extend the quiet window, as khuaplayer's foreground storage gate does, instead of holding background I/O for whole playbacks
- [x] Subtitle scoring and Chinese alias normalization: sidecar subtitle files attached to their videos by scans, with languages from their names (Chinese variants included) and ordered by the scoring model; playback selection matches preferred languages by the normalized aliases
- [x] Dolby Vision Profile 7: separate enhancement-layer tracks, recognized only from their configuration, are never chosen as the video; HDR10 clients get the Base Layer without Dolby Vision metadata
- [x] Black border cropping: borders found by sampling frames (`libs/media/borders`, job `media.borders`), kept on the video stream and cropped by the planner when it re-encodes
- [x] Pinned dependencies: jellyfin-ffmpeg's SHA-256 per platform in `mise.toml`, checked against the Dockerfile's by a test
- [x] Keeping volumes awake: uncached heartbeat reads (`VolumeHeartbeat`) by `storage.Keeper` for the volumes of playbacks in progress and of present users' continue-watching media (`internal/warming`), and a wake when a client opens an item
- [x] Resume latency: playback starts at the keyframe or segment start up to 5 seconds before the requested position (`StartPlaybackResponse.start_position`), HLS starts ffmpeg there when the playback starts, and the media around it is read ahead
- [x] Implementation index of every storage mechanism, with its departures from Jellyfin ([Optimization §1.9](optimization.md))

### P10 Metadata Management
**Scope**: correcting and completing what the library scans find.

- Edit an item and lock its fields; identify an item again against the providers; refresh one item
- List a provider's images; choose, upload and delete images
- Write NFO files and images next to the media (`libs/metadata` only reads NFO files today)
- Providers as plugins: music and books (MusicBrainz, TheAudioDB), more artwork (fanart.tv), subtitle downloads
- Movie collections created from the providers' collections

**Done when**: an end-to-end test through the API: an administrator identifies a misidentified film again and edits it, the locked fields survive a refresh, and the chosen poster is written next to the file and read back by a new scan.

**Progress**:
- [x] `MetadataService`: edit items with an update mask, locks included; refresh one item, optionally replacing its metadata; search the providers and identify an item again
- [x] Images: list the providers' images; choose one by URL or upload it; delete images with their files; chosen artwork beside the media or in the metadata folder (`--metadata-dir`)
- [x] Local metadata: `metadata.WriteNFO`, which `ParseNFO` reads back; libraries saving local metadata write NFO files and save provider images under Jellyfin's local image names
- [x] Provider plugins: `scraper-musicbrainz` (artists, albums, tracks, Cover Art Archive), `scraper-theaudiodb`, `scraper-openlibrary` (books and audiobooks), `scraper-fanart`; subtitle provider contract (`SubtitleProviderService`) with `subtitles-opensubtitles`, downloads saved beside the video
- [x] Automatic movie collections from the providers' collections (`AutoCollections`)
- [x] End-to-end test (`apps/server/internal/server/metadata_test.go`)

### P11 Administration and Operations
**Scope**: running a server without restarting it or editing its flags.

- Server settings stored in the database and set through `SystemService`, taking over from the command-line flags what an administrator changes at run time; first the transcoding settings (hardware acceleration chosen by the administrator, encoder presets, tone mapping, transcode folder)
- Browsing the server's folders to choose library paths
- Jobs: list the scheduled jobs with their last runs, run one now
- Plugin hot plugging: installing, upgrading and uninstalling plugins without restarting the server, as [Architecture §7.3](architecture.md#73-child-process-runtime) states. Today the plugin folder is read at startup only, and only configuration changes apply without a restart
- A plugin catalog to install and upgrade from; authentication and notification plugins, whose contracts (`AuthProvider`, `Notifier`) already exist, used by the server
- API keys for integrations, activity log, server logs, backup and restore, localization data (countries, languages, rating systems)
- Networking: HTTPS with configured certificates, a base URL behind reverse proxies, discovery of servers on the local network

**Done when**: an end-to-end test through the API: an administrator changes a transcoding setting and the next transcode uses it; a plugin is installed from a catalog, upgraded and uninstalled while the server keeps serving; a backup is restored into a new server.

**Progress**:
- [x] Server settings in the database, applied at once and rolled back when a part rejects them: transcoding (hardware acceleration, encoder preset and quality, tone mapping, deinterlacing, downmix, cropping, transcode folder), network, plugin catalogs
- [x] Folder browsing; tasks with their last and next runs, run now; the job queue; daily cleanup of finished jobs
- [x] Plugin hot plugging from catalogs: install, upgrade with rollback, uninstall; authentication and notification plugins used by the server
- [x] API keys; activity log sent to notification plugins; recent server logs
- [x] Backup and restore (`store.Dump` and `store.Restore` on both databases, artwork and plugins)
- [x] Localization data: countries, languages, rating systems; the ported Jellyfin localization cases pass, translations excepted
- [x] Networking: base URL, HTTPS with configured certificates, local discovery
- [x] End-to-end test (`apps/server/internal/server/admin_test.go`)

### P12 Media Extras
**Scope**: media features beyond playing a stream.

- Trickplay thumbnail sheets ([Architecture §6.1](architecture.md#61-requirements-and-implementation)) and chapter images
- Media segments (intros, credits) from segment provider plugins
- Lyrics; attached fonts for ASS subtitles rendered by clients; audio normalization
- Progressive transcoding and offline downloads (§4)

**Done when**: tests with real ffmpeg produce trickplay sheets and chapter images, and a progressive transcode is downloaded and played; ported cases pass where Jellyfin has them.

**Progress**:
- [x] Trickplay sheets and chapter images (`libs/media/thumbnails`, jobs `media.trickplay` and `media.chapters`), cropped by the borders found, with a failure circuit breaker; served from the metadata folder
- [x] Media segments from segment provider plugins (`MediaSegmentProviderService`, job `media.segments`)
- [x] Lyrics (`metadata.ParseLyrics`; Jellyfin's ELRC case passes) through `ItemService.GetLyrics`; font attachments served to clients; loudness of tracks and albums (`media.loudness`) as normalization gains
- [x] Progressive remuxes and transcodes; downloads, the file itself or a progressive transcode
- [x] End-to-end test with real ffmpeg (`apps/server/internal/server/extras_test.go`)

### P13 Plugin Platform
**Scope**: what Jellyfin's plugins can do, apart from channels and live TV, for Mavio's plugins in both runtimes ([Plugins](plugins.md)): the host API with scopes and acting as users; events to consumers and every activity to notifiers; HTTP routes; tasks and a data folder; remote devices; image, lyrics, local metadata, saver, processor, resolver, intro, image generator, media source and password reset providers; external ID declarations; provider order per library.

**Done when**: the test plugin, built for both runtimes, exercises every capability through an assembled server in an end-to-end test; the contracts and SDK are documented.

**Progress**:
- [x] Host API: scopes, acting as users, transport for both runtimes
- [x] Data folder; tasks
- [x] Events: the event types of [Plugins §4](plugins.md#4-events), event consumers, notifiers
- [x] HTTP routes
- [x] Remote devices; acting as devices
- [x] Image, lyrics, local metadata, saver and processor providers; external IDs
- [x] Resolvers, intro providers, image generators, media source providers, password reset
- [x] Provider order per library
- [x] End-to-end test (`apps/server/internal/server/capabilities_test.go` with real ffmpeg, `plugins_test.go`), in both runtimes

### P14 DLNA
**Scope**: `plugins/dlna`, a first-party process plugin ([Plugins §10](plugins.md#10-dlna-pluginsdlna)): media server (SSDP, ContentDirectory, ConnectionManager, media with DLNA headers), Play To (renderer discovery, AVTransport and RenderingControl), device profiles; built for every server platform, packaged for a catalog and bundled in the container image.

**Done when**: an end-to-end test with a simulated control point and renderer on the loopback interface discovers the server, browses a library, plays a file with range requests, and drives a renderer through play, pause, seek and stop with progress reported. Playback on a real television is verified in P22.

**Progress**:
- [x] SSDP and device description
- [x] ContentDirectory and ConnectionManager
- [x] Media with DLNA headers and device profiles
- [x] Play To
- [x] Builds, catalog packages and container image (seeded plugin folder)
- [x] End-to-end test (`apps/server/internal/server/dlna_test.go`)

### P15 Acquisition Foundation
**Scope**: what the server needs before it can want media it does not have ([Architecture §13](architecture.md#13-acquisition-libsacquire)).

- Wanted items: movies and episodes known from metadata but without files, kept apart from the missing items of §4.7 of the [Domain Model](domain.md); monitored state per movie, series, season and episode
- Series: TVDB ID, series type (standard, daily, anime) and display order, Jellyfin's `DisplayOrder` with TMDB episode groups
- `libs/release`: release title parsing (title, year, season and episode, absolute and daily numbers, quality, source, resolution, codecs, languages, edition, release group), with Radarr's and Sonarr's parser rules in their .NET syntax on `regexp2`
- `tools/testport` reads NUnit test cases, so that Radarr's and Sonarr's parser cases are ported as Jellyfin's are

**Done when**: the ported parser cases pass or are skipped with a reason; the missing-episode cases of `scraper-tmdb` skipped in P4 pass; a series' episodes are listed in its display order.

**Progress**:
- [x] Virtual and monitored items ([Domain Model §4.8](domain.md#48-virtual-items)); episode list providers (`EPISODE_LIST_PROVIDER`, [Plugins §8.12](plugins.md#812-acquisition-providers)); the missing-episode cases pass in `libs/library`
- [x] Series types, display orders and absolute numbers; `scraper-tmdb` numbers seasons and episodes by the series' TMDB episode group
- [x] `libs/release` with Radarr's and Sonarr's parser cases
- [x] `tools/testport` reads NUnit test cases
- [x] Monitoring a series or changing its display order refreshes it at once; end-to-end test (`apps/server/internal/server/episodes_test.go`)

### P16 Acquisition Data Sources
**Scope**: the metadata acquisition needs, without keys by default.

- `plugins/scraper-servarr` (WASM): Radarr's movie metadata and lists, Sonarr's SkyHook series and episodes in TVDB order, scene mappings, daily series, XEM
- `scraper-tmdb`: Radarr's TMDB read access token unless an administrator sets a key, and Mavio's own key when TMDB rejects the token; `DISCOVERY_PROVIDER` (trending, popular, upcoming, by genre, studio, network, keyword and watch provider, recommendations and similar titles); episode groups

**Done when**: with no key configured, a movie and a series are identified and refreshed, also when TMDB rejects Radarr's token, the series' wanted episodes are numbered as TVDB numbers them, and TMDB's episode metadata is attached to them by their TVDB IDs, also where TMDB numbers them otherwise.

**Progress**:
- [x] `scraper-tmdb` without a key: Radarr's token, Mavio's key once TMDB rejects it (built in from `MAVIO_TMDB_KEY`)
- [ ] Mavio's own TMDB API key registered and set as `MAVIO_TMDB_KEY` in release builds (needs the project's TMDB account)
- [x] `DISCOVERY_PROVIDER` in `scraper-tmdb`
- [x] `plugins/scraper-servarr`: Radarr's movie metadata and lists, SkyHook's series and episodes, scene mappings, daily series, XEM
- [x] End-to-end test (`apps/server/internal/server/sources_test.go`)

### P17 Indexers
**Scope**: `plugins/indexers`, a first-party process plugin porting Prowlarr: Newznab, Torznab, the Cardigann engine with the definitions of `indexers.prowlarr.com`, every site Prowlarr implements in code, categories and capabilities, rate limits and backoff, indexer proxies (HTTP, SOCKS, FlareSolverr). The capability `INDEXER_PROVIDER`; `ReleaseService` searches the configured indexers and parses what they return with `libs/release`.

**Done when**: Prowlarr's ported cases pass; recorded responses of Newznab, Torznab, Cardigann definitions and the sites implemented in code are searched and parsed into releases through the assembled server.

**Progress**:
- [x] The `INDEXER_PROVIDER` contract (`mavio/plugin/v1/indexer.proto`) and its host client
- [x] Prowlarr's cases and recorded responses ported into `plugins/indexers/testdata`
- [x] Number, size and date parsing (`internal/parse`)
- [x] Categories, capabilities and search criteria (`internal/indexer`)
- [x] Newznab and Torznab (`internal/sites/newznab`, `internal/sites/torznab`)
  - [x] The age and size limits of searches (`minage`, `maxage`, `minsize`, `maxsize`): queries carry them, Newznab and Torznab send them, and the plugin drops releases outside them; the interactive `SearchReleases` takes them, as Prowlarr's searches do
  - [x] Capabilities kept across restarts in the plugin's data folder, read again only when the indexer's URL or API key changes or after a week
- [x] The Cardigann engine (`internal/sites/cardigann`): definitions read from a directory and its `Custom` directory, templates, filters, HTML, XML and JSON selectors, logins, searches and downloads
  - [x] Fetching and updating the definitions of `indexers.prowlarr.com`, which the plugin's wiring does
  - [ ] Captchas solved by hand: an image captcha fails automatic logins, as no UI shows it yet
  - [x] Cookies of the `cookie` setting replace those responses set, as Prowlarr's GetCookies does
- [x] The sites Prowlarr implements in code, but those it marks obsolete
  - [x] Gazelle (`internal/sites/gazelle`): AlphaRatio, BrokenStones, DesiGaane, DICMusic, GreatPosterWall, Secret Cinema; with an API key, Orpheus and Redacted
  - [x] AvistaZ (`internal/sites/avistaz`): AnimeZ, AvistaZ, CinemaZ, ExoticaZ, PrivateHD
  - [x] SpeedApp (`internal/sites/speedapp`): RetroFlix, SpeedApp.io
  - [x] BroadcasTheNet (`internal/sites/broadcasthenet`), with the API's 150 calls an hour as its definition's default limits
    - [x] The origin, source, container, codec and resolution of its releases, as Prowlarr's TorrentInfo keeps them
  - [x] FileList (`internal/sites/filelist`)
  - [x] HDBits (`internal/sites/hdbits`)
  - [x] PassThePopcorn (`internal/sites/passthepopcorn`)
  - [x] GazelleGames (`internal/sites/gazellegames`), asking for the passkey on testing or on the first download
    - [x] Its size limits (`sizesmall`, `sizeslarge`), sent from the queries' `minsize` and `maxsize`
  - [x] IPTorrents (`internal/sites/iptorrents`), scraping its pages with the cookie and User-Agent of a browser
  - [x] BitHDTV, HD-Torrents, ImmortalSeed, Libble, NorBits and PiXELHD (`internal/sites/<name>`), scraping their pages
  - [x] Headphones, Knaben, NzbIndex, RuTracker, TorrentPotato and TorrentsCSV (`internal/sites/<name>`)
    - [x] Downloaded NZBs checked before they are sent on, as Prowlarr's NzbValidationService does, for every Usenet indexer
    - [ ] RuTracker's login captcha, which fails automatic logins as no UI shows it yet
  - [x] BeyondHD, M-Team TP, MyAnonamouse, Nebulance, Shazbat and TorrentDay (`internal/sites/<name>`)
    - [x] MyAnonamouse's size limits (`tor[minSize]`, `tor[maxSize]`), sent from the queries' `minsize` and `maxsize`
    - [x] MyAnonamouse's cookies cleared before a test, as Prowlarr does
  - [x] PreToMe, RevolutionTT, SceneHD, SpeedCD, TorrentBytes, TorrentSyndikat, Uniotaku and XSpeeds (`internal/sites/<name>`)
  - [x] AnimeBytes, Anidex, Anidub, BakaBT, SubsPlease and Toloka.to (`internal/sites/<name>`)
    - [x] AnimeBytes' results kept for three minutes, as Prowlarr caches them
    - [x] Anidub's resolution, read from each torrent's quality block (Prowlarr reads it from the wrong element and finds none)
  - [x] The API tokens of AvistaZ and SpeedApp sites kept across restarts in the plugin's data folder, as Prowlarr keeps them in the settings
  - [x] The seeding settings of torrent indexers: the minimum seeders the decisions check, and the seed ratio, seed time and season pack seed time download clients are given with grabs, as Radarr's and Sonarr's SeedConfigProvider
- [x] Rate limits, backoff and indexer proxies (HTTP, SOCKS, FlareSolverr)
- [x] `ReleaseService` (`mavio/acquisition/v1/release.proto`, `apps/server/internal/releases`): lists the indexers of every ready provider; searches them for a movie, season or episode of the library tier by tier as Radarr's and Sonarr's FetchReleases do (the IDs each indexer takes, then its titles), or for text; parses the releases with `libs/release`
  - [x] Seasons of anime searched episode by episode, each aired episode by its absolute number, as Sonarr's SearchAnimeSeason
  - [x] Newznab's `title` parameter, sent with the raw titles to indexers taking it, as Sonarr does
  - [x] Scene titles and mappings (XEM) of series searched besides their titles, with P19
  - [x] Indexers and their capabilities kept by the server for an hour, until their plugin restarts or is configured again, so that searches need not list the indexers first
- [x] End-to-end test (`apps/server/internal/server/releases_test.go`): Newznab, Torznab, a Cardigann definition fetched as `indexers.prowlarr.com` serves them and NzbIndex, searched through the assembled server

### P18 Movie Acquisition
**Scope**: `libs/acquire` with its `movie` package, porting Radarr; `plugins/download-clients`, a first-party process plugin with every download client Radarr and Sonarr support.

- Quality definitions and profiles, a profile per monitored movie, series and season; custom formats, delay and release profiles
- Decisions: the specifications of Radarr's decision engine; ranking
- RSS sync and searches as jobs; the queue of tracked downloads; remote path mappings
- Import: completed downloads verified with ffprobe, hard linked or moved into the library under a naming template, a scan of their folder; upgrades until the cutoff, replaced files to a recycle folder, blocklist and a new search when a download fails
- History

**Done when**: an end-to-end test with a simulated indexer and download client on the loopback interface: a wanted movie is found, sent, tracked, imported and played, then upgraded by a better release; a failed download is blocklisted and replaced.

**Progress**:
- [x] The `DOWNLOAD_CLIENT_PROVIDER` contract (`mavio/plugin/v1/download_client.proto`) and its host client
- [x] `plugins/download-clients`: the download clients of Radarr and Sonarr, with their cases ported (`internal/clients/<name>`: Aria2, Torrent and Usenet Blackhole, Deluge, Download Station, Flood, Freebox, Hadouken, NZBGet, NzbVortex, Pneumatic, qBittorrent, rqbit, rTorrent, SABnzbd, Transmission, uTorrent, Vuze)
  - [x] Seed limits given with grabs kept in the plugin's data folder (`seeds.json`) across restarts, as Radarr keeps its seed configuration
  - [x] Download client tags (`tags`), so that a movie or series only uses the clients that share one of its tags, else those without tags
  - [x] Flood's additional tags (title slug, quality, languages, release group, year, indexer, studio or network, collection), from the details grabs carry
  - [x] Sonarr's refusal of full seasons by Pneumatic, which grabs tell
  - [x] Torrents whose hash the release did not give refused when their hash is blocklisted, for indexers set to reject them and releases not from an interactive search, as Radarr's TorrentClientBase.EnsureReleaseIsNotBlocklisted; the hash the client learnt is recorded with the grab
  - [x] Helpers the clients share instead of each keeping its own: remote path joining, .NET version parsing, JSON-RPC and XML-RPC, URL base validation, a free-form tag field, "release rejected" and "release blocked" errors, warnings from tests, NZB validity checks, a configurable history limit (`history_limit`), IDs kept in the case clients give them, and `::1` counted as localhost
- [x] `libs/acquire`: qualities and their definitions, quality profiles, custom formats, delay profiles, release profiles
- [x] Decisions: the specifications of Radarr's decision engine and its ranking, with its cases ported (`libs/acquire`)
  - [x] Indexers found by name when their ID finds none, as Radarr's AggregateLanguages
  - [x] Unknown and any languages kept apart (`release.Any`), as Radarr's Language.Unknown and Language.Any: profiles wanting any language accept every release, those wanting Unknown only releases of an unknown language
  - [x] Tags of indexers, which the indexer tag specification checks
- [x] Domain and storage: profiles, custom formats, the queue of tracked downloads, history, the blocklist, remote path mappings, naming (`libs/core/acquisition.go`, `libs/store`)
  - [x] The format scores of quality profiles checked against the custom formats that exist, as Radarr's QualityProfileController: no score of a missing format or two of one, a minimum score that can be reached; deleting the last format resets the profiles' format scores, as Radarr's QualityProfileService
- [x] Adding a wanted movie to a library by its TMDB ID, with a quality profile and minimum availability (`WantedService`, `apps/server/internal/acquisition`)
  - [x] Digital and physical release dates of movies (from TMDB's release dates, or Radarr's metadata), which minimum availability uses as Radarr's Movie.IsAvailable; a movie without them is released 90 days after its premiere
  - [x] The original language of movies, which profiles wanting "Original" take and releases without languages are in; profiles accept any language for movies whose original language is unknown
  - [x] A movie whose identification fails is kept under its placeholder name ("TMDB <id>") and refreshed by the daily refresh of wanted movies, which refreshes those due for it as Radarr's RefreshMovieService and ShouldRefreshMovie: never refreshed, refreshed more than 180 days ago, or at least 12 hours ago and not yet released or physically released in the last 30 days
- [x] Movie searches and RSS sync as jobs; grabs through `DownloadService`; decisions in `SearchReleases`
  - [x] Releases matched to movies by their alternative titles, translations and secondary years, as Radarr's MovieService.FindByTitle and TryGetMovieBySearchCriteria
  - [x] Tags, required flags, minimum seeders and MULTi languages of indexers, which the indexer plugin reports from its configuration
  - [x] Download client tags and round robin among the clients of the first priority, as Radarr's DownloadClientProvider.GetDownloadClient
  - [x] An indexer's own download client (`download_client_id`), which its releases are always sent to, as Radarr's IndexerDefinition.DownloadClientId; a release whose indexer's client is missing waits pending
  - [x] When every client of a protocol is left out after failing, a grab still tries one, as Radarr's DownloadClientProvider; only releases kept pending until a client is back wait for one
  - [x] Failed releases grabbed from an interactive search searched for again only with `AutoRedownloadFailedFromInteractiveSearch`, as Radarr's RedownloadFailedDownloadService
  - [x] An RSS sync asked for while another is running runs again after it, rather than being dropped (one asked for while another is pending is answered by that one): the job queue deduplicates only pending jobs and never leases two jobs of one unique key at once
- [x] The queue: tracked downloads polled from the clients
  - [x] The downloads of a client that fails to answer, or is left out after failing, keep their last state rather than being flagged
- [x] Import, upgrades and the recycle folder
  - [x] Qualities of imported files aggregated from their names, their download folder's, the release's and their media info, whose video size decides the resolution, as Radarr's AggregateQuality; the MediaInfo tokens of file names written from the file read at import
  - [x] Subtitles imported with a movie named by their language, tags and copy number, matched by the video's name, the movie's title and year, or taken when the video is the download's only one, as Radarr's SubtitleService; the extras of a file an upgrade replaces recycled with it
    - [x] Extra files tracked in the database (`core.ExtraFile`, `extra_files`), as Radarr's and Sonarr's SubtitleFile and OtherExtraFile tables, for movies and episodes: imports record them with their language and tags, and upgrades recycle those recorded rather than those found by name beside the old file
  - [x] Manual import of the files of a tracked movie download (`ListManualImport`, `ManualImport` of `DownloadService`): its video files listed with the quality an import gives them and the rejections, and the chosen one imported with the chosen quality whatever they are, as Radarr's ManualImportService
    - [x] Manual import of folders and files that are not tracked downloads, their files matched to wanted movies by name or rejected as "Unknown Movie", of files for another movie than the download's, and with chosen languages, release group and import mode (move or copy), as Radarr's ManualImportService and ImportMode
  - [x] Free space checked on the volume of the movie folder an import writes to, or of its nearest existing parent, rather than the library folder, as Radarr's FreeSpaceSpecification
  - [x] Download folders an import moved the movie out of deleted when only samples and small RAR files are left, as Radarr's DownloadedMovieImportService.ShouldDeleteFolder; folders of libraries and remote path mappings are kept
  - [x] A scan of the folder a movie was imported to only (`library.scan_folder`), rather than of its library
- [x] Failed downloads: the blocklist and a new search
- [x] The acquisition jobs (RSS sync, the queue check, the recycle bin cleanup) listed and run on demand among the tasks of `TaskService`
- [x] End-to-end test (`apps/server/internal/server/acquisition_test.go`): a Newznab indexer and SABnzbd simulated on the loopback interface, through the indexer and download client plugins; a wanted movie is searched, grabbed, tracked, imported under Radarr's naming and played, upgraded by a release found by RSS sync with the old file in the recycle bin, and a failed download is blocklisted and replaced by a new search
  - [x] A torrent client (`torrent_acquisition_test.go`): a Torznab indexer and qBittorrent simulated on the loopback interface; the magnet link sent with the indexer's seeding limits, the seeding download imported by a hard link and an upgrade, with hard links off, by a copy, and a torrent removed with its files once paused at its limits
  - [x] Remote path mappings in the end-to-end test: qBittorrent's folders of another machine mapped to the server's

### P19 Series Acquisition
**Scope**: `libs/acquire/series`, porting Sonarr: episode, season and series searches, season packs, multi-episode files, anime with absolute numbers, daily series by air date, scene mappings and XEM, monitoring of new seasons and episodes.

**Done when**: the end-to-end test of P18 for a series: wanted episodes are found singly and as a season pack, an anime and a daily series are matched through their numbering, and each episode becomes playable once imported.

**Progress**:
- [x] Decisions: the specifications of Sonarr's decision engine and its ranking, with its cases ported (`libs/acquire/series`)
  - [x] Indexers found by name when their ID finds none, as Sonarr's AggregateLanguages
- [x] Episode file names, series and season folders named as Sonarr names them (`libs/acquire/organize`)
- [x] Wanted series and episodes: series added by TVDB ID, with a quality profile, monitoring of new seasons and episodes (`WantedService.AddSeries`, `apps/server/internal/acquisition`)
  - [x] Seasons and episodes scans find on disk monitored as Sonarr's GetMonitoredStatus: an episode as its season, but not an episode 0 outside season 1; a season as the series' new seasons, but not the specials
  - [x] Sonarr's `AutoUnmonitorPreviouslyDownloadedEpisodes`: episodes of wanted series whose files a scan finds missing are unmonitored, and RSS syncs skip them until then
  - [x] Each episode's own last search time (`EpisodeState.LastSearchedAt`), set when an indexer was searched for it and ordering the bulk searches of missing episodes
- [x] Episode, season and series searches and RSS sync as jobs; releases matched to episodes through scene mappings, XEM, absolute numbers and air dates (`libs/acquire/series`, job `acquisition.episode_search`, scene mappings refreshed by `acquisition.scene_mappings`)
  - [x] Scene numbering kept on episodes (`EpisodeState`), extrapolated and marked unverified as Sonarr's `XemService` does, refreshed when a series is added and with the scene mappings
  - [x] Anime searched by season and episode, and anime season packs by season, of indexers set to (`anime_standard_format_search`, Sonarr's `AnimeStandardFormatSearch`); scene mappings searched by both IDs and title
  - [x] Interactive searches of a whole series: each season but the specials searched as Sonarr's series search does, anime episode by episode, and the releases decided on as by RSS
  - [x] The series and movie engines log with the server's logger rather than the default one
- [x] Import of episodes, season packs and multi-episode files; upgrades
  - [x] Series naming settings: whether episodes are renamed, the episode file formats, the series, season and specials folder formats and the multi-episode style, with Sonarr's defaults when empty
  - [x] Sonarr's import specifications for episode titles (with the `EpisodeTitleRequired` setting), unpacking, unverified scene numbering, absolute numbers and matching folders, and its scene name calculation
    - [x] The download client working folders as a setting (`DownloadClientWorkingFolders`), with Sonarr's default `_UNPACK_|_FAILED_`
  - [x] The qualities of episode files aggregated as Sonarr's AggregateQuality: from the names of the file, the download's folder, the download and the grabbed release, and the frame size of the file's video
    - [x] The source the title tag of a file's container tells, as Sonarr's AugmentQualityFromMediaInfo reads it: probes keep that tag apart from those of the streams (`ContainerTitle`), which movie imports read too, as Radarr's
  - [x] Episode imports as movie imports where Sonarr behaves as Radarr: free space checked for each file on the volume of the series folder, as Sonarr's FreeSpaceSpecification; subtitles named by their language, tags and copy number and matched by the video's name or the season and episodes theirs hold, as Sonarr's SubtitleService; the extras of replaced files recycled; download folders emptied by moves deleted, as Sonarr's ShouldDeleteFolder; a scan of the series folder only
  - [x] The history of the episodes a multi-episode file covers kept by the file's item, on import and on scans
- [x] End-to-end test (`apps/server/internal/server/series_acquisition_test.go`): a season pack and a single episode of a standard series, an anime release named by a scene mapping title and an absolute number, and a daily series' release named by air date are found, imported under Sonarr's naming and played
  - [x] RSS syncs of series, multi-episode files and upgrades of episodes in the end-to-end test (`series_flows_test.go`): a release of two episodes grabbed by an RSS sync and imported as one file, upgraded by another
  - [x] XEM's scene numbering in the end-to-end test (`series_flows_test.go`): the indexer searched by the scene's numbers, and the release named by them imported as TVDB numbers the episode
  - [x] Failed episode downloads, the blocklist and the searches replacing them in the end-to-end test (`series_flows_test.go`)
  - [x] `RunTask` answers with the queued job when it does not queue another

### P20 Discovery and Requests
**Scope**: porting Seerr. `DiscoverService` lists what the discovery providers offer, marked as available, wanted or requested; `RequestService`: requests of movies and seasons, approval by administrators or automatic, quotas per user, users' limits on ratings applied to what they may request, notifications of the requester; reporting problems with an item.

**Done when**: an end-to-end test: a user limited to PG finds a film and a series through discovery, cannot request one rated above the limit, requests the others; an administrator approves; they are acquired and the user is notified that they are available.

**Progress**:
- [x] Domain and storage: requests with their seasons, issues and their comments, users' request permissions and quotas, the server's request settings (`libs/core/request.go`, `libs/store`)
- [x] `DiscoverService`: the catalogs of the discovery providers, their titles marked as available, wanted or requested, and the user's rating limit applied (`apps/server/internal/requests`)
  - [x] Text search in discovery (`DiscoverService.SearchTitles`), through the `SearchTitles` of discovery providers; scraper-tmdb searches as Seerr does, with its `tmdb:`, `imdb:`, `tvdb:` and `year:` prefixes
  - [x] Totals of pages that leave titles out: `hidden_results` counts the titles left out of a page, `total_results` is the provider's total less them, exact on a single page and otherwise an upper bound
  - [x] Unrated titles kept by certification filters unless the user's policy blocks them: certifications go to the provider only when the caller asks for some or blocks unrated titles; otherwise the server rates the titles itself
  - [x] Details of a title with the status of each of its seasons (`DiscoverService.GetTitle`), as Seerr's movie and series details
- [x] `RequestService`: requests of movies and seasons checked as Seerr checks them (permissions, rating limit, quotas, duplicates), approved automatically or by an administrator, acquired through `WantedService`, completed when their items become available (`requests.availability`)
  - [x] A quality profile and library of their own for anime requested (`RequestSettings.AnimeQualityProfileID`, `AnimeLibraryID`), added as anime series, as Seerr's anime settings
  - [x] Seerr's blocklisted and deleted statuses of titles: the blocklist (`BlockTitle`, `UnblockTitle`, `ListBlockedTitles`, `core.BlockedTitle`) refuses requests of its titles; a title whose completed request's files are gone is deleted and may be requested again
    - [x] Seerr's blocklisted tags, blocklisting the titles of keywords automatically (`BlocklistedTags`, `BlocklistedTagsLimit`, the weekly `requests.blocklisted_tags` job, titles tagged with `BlockedTitle.Tags`), and blocklisting a collection (`BlockCollection`, `UnblockCollection`, through collection catalogs of discovery providers, which `scraper-tmdb` gives)
  - [x] The country and language of a server-wide metadata setting (`ServerSettings.Metadata`) for ratings and names, rather than the destination library's
  - [x] The availability job listed among the tasks (`TaskService`, "Sync request availability")
- [x] `IssueService`: problems with an item reported, commented on, resolved and reopened
- [x] Notifications: activities of the user concerned, read through `ActivityService.ListMyActivities` and delivered by notifier plugins
  - [x] An event pushed to the sessions of the user concerned, and of administrators for the requests and issues awaiting them (`NotificationReceived`)
- [x] End-to-end test (`apps/server/internal/server/requests_test.go`): a user limited to PG discovers a film and a series through scraper-tmdb, is refused an R-rated film, requests the others; an administrator approves; they are grabbed, imported and played, and the user is notified that they are available; an issue the user reports is resolved
  - [x] Declined requests, exhausted quotas and retried failed requests in the end-to-end test (`request_flows_test.go`)
  - [x] Changed seasons of pending requests and requests of every season in the end-to-end test (`request_flows_test.go`)
  - [x] Unrated titles hidden from users whose policy blocks them in the end-to-end test (`request_flows_test.go`): left out of catalogs, refused by `GetTitle` and requests, and asked of TMDB by certification

### P21 Clients and Ecosystem
**Scope**: the [clients](clients.md), built on the finished server API: the web UI embedded in the server (`apps/web`, served at `/`), the Apple apps for iPhone and iPad, Apple TV and the Mac (`apps/ios`, `apps/tvos`, `apps/macos`), the Android apps for phones and tablets and for Android TV (`apps/android`, `apps/android-tv`), the desktop app on MyGo for macOS, Windows and Linux (`apps/desktop`), and the libraries they share (`libs/client-ts`, `libs/client-rust`, `libs/apple`, `libs/android`, `libs/design`); the decision on a Jellyfin API compatibility shim ([evaluation](jellyfin-compat.md)). Its foundation does not wait for P20: the toolchains and build targets, the TypeScript and Rust code generated from `libs/proto`, `libs/design`, the prototypes the open items of §4 name for clients, and the skeletons of the apps and libraries. Only the discovery and request screens wait for P20. The foundation ends with a slice of every app, which proves the shared core, the design system and the platform's players together before any client is completed: it signs in to two servers, shows the merged Home, opens an item's details, and plays a source directly and one the server sends as HLS, its progress reported and resumed. The web UI and the iPhone and iPad app went past their slices. Which client is completed first is then chosen; the others follow it.

**Done when**: every client signs in to two servers at once and shows one Home merging their hubs, with a title both servers have shown once; browses a library, searches both servers, opens an item and plays it — direct play, and a transcode where the client needs one — with audio and subtitle selection, its progress reported and resumed on another client; requests a title from Trending, which is approved, acquired and then shown as available; the native apps start offline on cached content, and those with downloads play a downloaded item offline. Each client's screens pass their snapshot tests, and `libs/design/parity.md` has no feature left planned.

**Progress**:
- [x] Toolchains and build targets: Rust in `libs/client-rust` with `tools/nx-cargo`; Go, Node.js and pnpm updated
- [x] TypeScript and Rust generated from `libs/proto` (`@mavio/proto`, `mavio-api`)
- [x] `libs/design`: tokens, icons, strings, component specifications and the feature parity table, compiled for every client
- [x] Server: `HomeService` with the hubs of Home and of the library tabs (`apps/server/internal/home`), chosen per library by `Library.Hubs`
  - [ ] A hub of the collections an administrator promoted to Home
  - [ ] Several seeds per seeded hub kind, as Plex shows several Because You Watched rows
- [x] Server: `ItemService.ListSimilarItems`, scored by `ItemQuery.SimilarTo` (`libs/store/items.go`)
  - [ ] The discovery providers' similar titles that the libraries have, after the local ones, as Jellyfin's remote similar items providers add them
- [x] Server: cross-origin requests on every route (`apps/server/internal/httpserver/cors.go`)
- [x] Server: the server's ID, name and version in `AuthService.GetAuthInfo` and the discovery reply, by which clients recognize a server at any address
- [x] Server: `ListItems` by external IDs (`ItemQuery.ExternalIDs`), by which clients find a title's versions on their other servers
- [x] Server: the caller's user data on the items listed for them (`Item.user_data`), the progress, played marks and last activity by which clients draw items and merge Continue Watching across servers
- [ ] Prototypes:
  - [x] Rust on tvOS: on `nightly-2026-10-10` with `-Zbuild-std`, `mavio-api` and its `events` example build for `aarch64-apple-tvos` and `aarch64-apple-tvos-sim`, the example runs in the tvOS simulator, and TLS (rustls with aws-lc-rs) builds
  - [x] connect-rust streams with `EventService`: `libs/client-rust/api/examples/events.rs` signs in and prints the device's events, on macOS and in the tvOS simulator
  - [x] AetherEngine 7.34.0 on macOS, iOS and tvOS: on the Mac an HDR10 Matroska source plays on AVPlayer through the engine's remux, decoded by VideoToolbox, and raises the display's EDR headroom; Picture in Picture starts on its `AVPlayerLayer` and, for VP9, AV1 and MPEG-2 on its software path, on its `AVSampleBufferDisplayLayer`; an ASS track arrives as raw event lines with its script header. The iPad and Apple TV simulators play the same sources on the software path, and the iPad one enters Picture in Picture on the sample buffer display layer when the app goes to the background
    - [ ] On the user's devices: the picture inside Picture in Picture on an iPhone or iPad, HDR10 and display criteria on an Apple TV, and a Dolby Vision source
  - [x] MyGo's video element on macOS: `Window.NewVideoView` and `ui.Video` on the branch `v0.4.5-mavio` of MyGo's fork `mavioai/mygo`, with MyGo's unit and end-to-end tests; libmpv for macOS from mavio-deps (`mpv-macos-0.41.0.g88eb9f2-1`) with Mavio's patch embedding it in the view `wid` names. A prototype loading libmpv with purego plays SDR H.264 and HDR10 HEVC in a MyGo window, decoded by VideoToolbox with `gpu-next` on MoltenVK, the HDR10 one output as PQ, and MyGo's controls over the video take the clicks; mpv opens no window of its own
    - [ ] Resizing the window and moving it to another screen during playback, and HDR10 on an HDR display
    - [ ] The video element contributed to MyGo upstream, and the mpv patch offered to mpv, so that the fork and the patch can go
  - [ ] MyGo's video element on Windows with HDR10, when a Windows machine is available, before the desktop app ships there
  - [x] The core on Android: in an instrumented test of `libs/android/core` (`ClientTest`, given the server and account as runner arguments), the Kotlin binding signs in, goes online on the event stream, receives events and delivers Home, in an Android 17 emulator with 16 KB pages, the 64-bit libraries being 16 KB aligned. Reaching a server on the local network needs the runtime permission `ACCESS_LOCAL_NETWORK`
  - [x] Media3 playing the server's fMP4 HLS: Media3 1.11.1 rejects the negative `tfdt` that audio starting before zero was written with, so the server now shifts such audio to zero. In an instrumented test of `libs/android/player` (`HlsTest`, given a playback's master playlist as a runner argument), ExoPlayer plays a remux with transcoded AAC audio across segments, seeks to where the server restarts the transcode and plays to the end. Media3 refuses cleartext HTTP unless the app allows it
  - [x] Media3's FFmpeg decoder extension, which has no published artifact for the current Media3: `android-lib:ffmpeg` builds FFmpeg's audio decoders as LGPL and links Media3's JNI layer with them, at the Media3 version of the version catalog, in about four minutes (cached in CI). In an instrumented test of `libs/android/player` (`FfmpegAudioTest`, given files as a runner argument), ExoPlayer decodes DTS and TrueHD with it on the emulator
  - [x] libmpv on Android: built from source with mpv-android's build scripts and JNI layer at pinned versions (`android-lib:mpv`, about an hour for the three ABIs, cached in CI), FFmpeg and mpv as GPL-3.0. In an instrumented test of `libs/android/player` (`MpvPlayerTest`, given a Matroska file as a runner argument), `MpvPlayer`, a Media3 `SimpleBasePlayer`, renders into a `SurfaceView` with OpenGL ES and, behind a `MediaSession`, the controller starts it at an offset, sees it ready and playing, pauses, seeks and reaches the end; releasing it is safe. On the emulator MediaCodec stalls libmpv, so the test decodes in software
    - [ ] On the user's devices: MediaCodec decoding (`mediacodec`, with `mediacodec-copy` as the fallback) and styled ASS subtitles drawn over the picture
    - [ ] HTTPS sources: libmpv's TLS (mbedtls) needs a CA file, as mpv-android ships one (`tls-ca-file`)
  - [x] libass on Apple platforms: `apple:ass` builds libass 0.17.5 with FreeType, FriBidi and HarfBuzz as a static XCFramework for macOS, iOS and tvOS and their simulators, with Core Text as the font provider, in about four minutes (cached in CI); the apps link it. `ASSRenderer` in `MavioPlayer` takes the script header, fonts as memory fonts and the event lines of AetherEngine's cues, and renders a frame's subtitles into a `CGImage`; its tests render an event in its style and the lines of a cue on the Mac
  - [x] The Go binding on macOS: `mavio-capi` exports the C ABI (`mavio_open`, `mavio_call`, `mavio_close`) with the messages of `mavio.capi.v1` for opening the core, its version, probing, signing in, the event streams and the servers, and the Go module `libs/client-rust/capi` loads it with purego without cgo. Its tests (`capi:test`, `CGO_ENABLED=0`) receive answers and errors through the one callback on the core's threads, 200 calls at once among them, and, given a server and account in the environment, sign in, receive the token to keep and follow the event stream to Connected
    - [ ] Linux and Windows: the same tests on their runners (`syscall` on Windows), unverified until CI runs them
  - [x] Snapshot testing on every platform: the screens and their fixed data in `libs/design/snapshots/screens.json`; Playwright 1.63 on the web with the browser in its container image, drawing sign-in and Home at the phone, tablet and desktop sizes; swift-snapshot-testing 1.19 on the iOS simulator, drawing Home with `MavioUI` on the iPhone and iPad; Roborazzi 1.76 with Robolectric 4.17 on the JVM, drawing the wordmark on a phone and a TV; `design:gallery` placing them side by side, and the CI jobs comparing them
    - [ ] The desktop app's snapshot tests with MyGo's tester, with its skeleton
    - [ ] The Android apps' screens, with their slice
    - [ ] References recorded again on the CI runners if their drawing differs from the Mac's (the Android tests run on linux amd64)
- [ ] Skeletons of the apps and libraries
  - [x] `libs/client-rust`: `mavio-core`, and `mavio-uniffi`, whose `uniffi-bindgen` generates the Swift module `MavioCore` and the Kotlin package `dev.mavio.core`; the C ABI and its messages (`capi`, `proto`) with the calls of the Go binding prototype; the rest comes with the desktop app, their only user
  - [x] Web: `apps/web` (React, Vite, TanStack Router and Query, Tailwind CSS on the design tokens) and `libs/client-ts`; the server embeds the build (`apps/server/internal/webui`) and serves it at `/`, its container image building it first
  - [x] Apple: `libs/apple` (`MavioKit` over the core's XCFramework, `MavioPlayer` with AetherEngine, `MavioUI` over `MavioDesign`, the Swift package of `libs/design`) and the apps `apps/ios`, `apps/tvos` and `apps/macos`, each showing the core's version on the simulator and the Mac
  - [x] Android: `libs/android` (`core` with the core's Kotlin API and libraries, `player` with Media3, `ui` with the design tokens, icons and strings) and the apps `apps/android` and `apps/android-tv`, which build and pass Android lint
  - [ ] Desktop: `apps/desktop` with the core's C ABI (`capi`, `proto`), on macOS first, after MyGo's video element there; libmpv for macOS pinned in `tools/deps`, the app bundle finding MoltenVK beside it (`@executable_path/../Frameworks`); libmpv for Windows and Linux built in mavio-deps
  - [x] Prebuilt dependencies: libass, libmpv and Media3's FFmpeg extension downloaded from mavio-deps' releases (`libass-0.17.5-2`, `mpv-0.41.0.g88eb9f2-1`, `media3-ffmpeg-1.11.1-1`) and verified by their checksums instead of built (`tools/deps`)
- [x] `libs/client-rust`: servers, their discovery and sign-in (password, first user, Quick Connect), the event streams with backoff, the offline store, playback reporting with offline replay, downloads, screen state, the merging of several servers (Home, Search, items' copies on other servers); the Swift and Kotlin bindings
  - [ ] The rest of the core's calls in the C ABI and the Go binding (`capi`, `proto`): Quick Connect, Home, libraries, details, search, playback and downloads, with the desktop app
  - [ ] Remote control commands and SyncPlay over the event stream, with the players of the apps that take them
- [x] Web UI (`apps/web`, `libs/client-ts`): sign-in (password, first user, Quick Connect), several servers and their order, the merged Home, libraries with their Recommended and All tabs, hubs, search across servers, details with versions and copies on other servers, the player (direct play and HLS, audio and subtitle tracks, external players), authorizing devices; refreshed by the event streams
  - [ ] Discovery and request screens, with P20
  - [ ] Administration screens: libraries, users, devices, plugins, tasks, settings
  - [ ] Person pages, collections and playlists
  - [ ] Playing the next episode at the end of one
  - [ ] Styled ASS subtitles with JASSUB
  - [ ] The query cache kept offline
  - [ ] Storybook of the components
  - [ ] Snapshot tests of the screens beyond sign-in and Home
- [x] iPhone and iPad app (`apps/ios`, `libs/apple`): sign-in (password, first user, Quick Connect, servers found nearby), several servers with tokens in the Keychain, the merged Home, libraries with their Recommended and Library tabs, sorting and filters, hubs, search across servers, details with versions and copies on other servers, the player (AVPlayer and AetherEngine chosen by the capabilities the app declares, HLS from the server, audio and subtitle tracks, playback speed, AirPlay, Picture in Picture on AVPlayer), downloads with a background `URLSession` and offline playback with replayed progress, authorizing devices; a tab bar on the iPhone and a sidebar on the iPad
  - [ ] Discovery and request screens, with P20
  - [ ] Picture in Picture while AetherEngine plays
  - [ ] Styled ASS subtitles with libass
  - [ ] Now Playing and the remote commands (`MPNowPlayingInfoCenter`, `MPRemoteCommandCenter`)
  - [ ] Playing the next episode at the end of one
  - [ ] Gestures: hold for double speed, swipe for brightness and volume
  - [ ] External subtitle files while the server sends HLS
  - [ ] A redrawn frame when AetherEngine seeks while paused, and scrub thumbnails
  - [ ] The multicast entitlement, so that servers on the network are found on a device and not only in the simulator
  - [ ] Snapshot tests of the screens beyond Home
- [ ] Slices: the Android phone and tablet app, the Apple TV and Android TV apps, and the Mac app
- [ ] Android phone and tablet app (`apps/android`, `libs/android`)
- [ ] Apple TV and Android TV apps
- [ ] Mac app
- [ ] Desktop app (macOS, Windows and Linux)
- [ ] The decision on a Jellyfin API compatibility shim

### P22 Real-Device Acceptance
**Scope**: end-to-end playback on real devices, after the clients exist: DLNA media server and Play To on real televisions, with the profiles adjusted to what they do.

**Done when**: a real television browses and plays a library through the media server, and is driven through Play To with progress reported.

**Progress**:
- [ ] DLNA on a real television

### P23 Packaging and Release
**Scope**: the first releases ([Development §7](development.md#7-releases-and-versioning)): `libs/proto` and `libs/plugin` tagged in that order; the server's release binaries and container images published; the official plugin catalog published with the DLNA plugin.

**Done when**: a plugin outside the repository builds against the tagged `libs/proto` and `libs/plugin` with `GOWORK=off` and no `replace`; the release binaries and images of a version are published and the images start.

**Progress**:
- [ ] Tag `libs/proto` and `libs/plugin` releases
- [ ] Publish the server's release binaries and container images
- [ ] Publish the official plugin catalog

### P24 Storage I/O Efficiency
**Scope**: what the server reads of media on its own, by the cost of reading it ([Optimization §2](optimization.md#2-storage-access-by-storage-class)): storage classes of library paths, keyframes from container indexes, trickplay from keyframes, embedded subtitles cached once on local storage and read while playing on cloud storage.

**Done when**: on a library classed cloud, scans, probes, keyframes, trickplay and playback with embedded subtitles read only a small fraction of each file, asserted by byte counts in tests; on local storage, embedded text subtitles are extracted once per file for every playback after.

**Progress**:
- [x] Storage classes: `core.StorageClass` set per library path or detected from the device, returned with the effective classes by `LibraryService`
- [x] Detection: FUSE mounts by their subtype on Linux, loopback NFS and SMB and macFUSE on macOS, WinFsp volumes on Windows
- [x] Storage policy: whole-file reads, samples, speculative reads and folder pruning by class (`library.PolicyOf`)
- [ ] Setting storage classes in the web UI, with its library administration
- [x] Keyframes from Matroska Cues and MP4 sample tables, `mfra` and `sidx`, extracted with the probe; ffprobe only without an index and off cloud storage
- [x] Trickplay from keyframes: Matroska sampled through its index, on every storage class; other files decode only their keyframes, falling back to every frame, off cloud storage; ffmpeg on one thread at a low priority
- [x] Embedded text subtitles extracted once into a persistent cache on local storage; on cloud storage, delivered by the stream that plays the video and never extracted whole
- [ ] Subtitle search and download, which writes sidecar subtitles beside the video

### P25 Storage Backends and rclone
**Scope**: libraries on storage the server reaches through a storage port rather than the file system: the local implementation over `os.Root`, and rclone's remotes through an `rclone rcd` the server runs or reaches ([Architecture §9](architecture.md)).

**Done when**: a library on an rclone remote scans, probes, plays and imports downloads through `rclone rcd`, tested against rclone's memory backend; remotes are configured through the API; offline downloads of cloud drives act as a download client.

**Progress**:
- [x] `core.Storage` port and its local implementation; scanning, local metadata, sidecars, writes, playback and imports through it
- [x] `libs/rclone`: rc client and storage over `rclone rcd`, media bytes through a loopback proxy
- [x] Managed and external `rclone rcd`; remote configuration API (`mavio.storage.v1`)
- [x] Offline downloads as a built-in download client, with their jobs recorded in the database
- [x] Distribution of the mavioai/rclone binary: release builds for Linux, macOS and Windows on amd64 and arm64 by the fork's CI, pinned in `mise.toml` and bundled into the container image
- [ ] Web UI: remotes in the server settings, rclone folders in the library form's folder picker (`StorageService.ListFolders`), and the offline download clients in the download client settings
- [ ] Manual imports from folders on remotes in the web UI's folder picker
- [ ] Reads of rclone remotes ([Optimization §3](optimization.md#3-reading-remote-storage-through-rclone)), in this order:
  - [x] Every backend, through the rcd and Mavio: in mavioai/rclone, the object cache with its version checks, the storage's statuses and seeking through `--rc-serve`, stream limits set by options, and the fixes of webdav and http (ranges answered with 200) and crypt (the nonce); in Mavio, one `Fs` per remote, the version pinned on every read and the block cache in `libs/rclone` with read-ahead by intent
  - [x] The block cache: read-ahead from the source's keyframes when a playback starts or seeks, and through the next segments' byte ranges for HLS; eviction of the ranges of open items furthest behind their readers
  - [x] The object cache: invalidation by the `ChangeNotify` of the remotes in it, and `If-Match` with the storage's ETag in `Object.Open` of the backends that read by path (s3, webdav, azureblob, swift, oracleobjectstorage and qingstor; googlecloudstorage and b2 already read one version)
  - [x] The backends of §3.2: `FindByListing`, link caches with lifetimes estimated where the storage states none, default stream limits estimated for the drives that throttle downloads, and the fixes of single backends (filelu's ranges, shade's first request and the "Also" column)
  - [ ] Measurements of the fork's drives and the common consumer drives (§3.5), replacing the estimated link lifetimes and stream limits and finding the remotes to read sequentially; a link cache for protondrive once its SDK can read from a kept link
- [ ] Later: remote plugins (plugin processes on other machines), direct links of cloud drives, incremental scans by change notification

### P26 Subtitle Acquisition
**Scope**: porting Bazarr ([Architecture §14](architecture.md#14-subtitle-acquisition-libsacquiresubtitles)): the subtitles movies and episodes lack in the languages of their subtitle profiles are searched, scored, downloaded, processed and upgraded; `plugins/subtitles`, a first-party process plugin with the providers Bazarr's settings offer.

- Subtitle profiles (Bazarr's language profiles), assigned to libraries, movies and series and by tag; subtitle history, blocklist and failed attempts
- `libs/acquire/subtitles`: Bazarr's wanted subtitles, scores, choice of the best subtitle, upgrades and adaptive searching
- Searches of wanted subtitles and upgrades as jobs, a search when a file is imported or found; manual search, download, upload and deletion; sidecars named as Bazarr names them
- `SUBTITLE_PROVIDER` extended; `plugins/subtitles` with Bazarr's providers, their throttling and the anti-captcha services; `subtitles-opensubtitles` merged into it
- Subzero's mods, encodings and language detection in `libs/subtitle`; synchronization as ffsubsync does it
- Storage classes respected ([Architecture §14.3](architecture.md#143-storage-classes)): embedded subtitles never extracted and audio never read on cloud storage

**Done when**: Bazarr's ported cases pass or are skipped with a reason; an end-to-end test with subtitle providers simulated on the loopback interface: an imported movie and episode get the subtitles their profiles want, one is upgraded by a better subtitle, one failing to decode is blocklisted and replaced, and one out of sync is synchronized.

**Progress**:
- [x] Domain and storage: subtitle profiles and their assignment, subtitle history, blocklist and failed attempts, the subtitle settings (`libs/core`, `libs/store`)
- [x] `tools/testport` reads pytest cases; Bazarr's score, mod and provider cases ported
- [x] `libs/acquire/subtitles`: wanted subtitles, scores, choice, upgrades, adaptive searching, language equivalences, ban lists and profiles by tag, as pure functions; release names guessed by `libs/release` and matched as subliminal's `guess_matches`
  - [x] The fields guessit gives Bazarr's scores that `libs/release` lacked, added to it with guessit's patterns: streaming service, audio and video codecs (editions come from Radarr's rules); the same values as guessit for all but 4 of the 940 release names of the ported cases
- [x] `SUBTITLE_PROVIDER` extended: provider listing, video descriptions and several languages in queries, matched properties in results, kinds of failures
- [x] `plugins/subtitles`: the plugin, archives (zip, RAR, 7z), throttling, proxies and FlareSolverr, anti-captcha services (Anti-Captcha, CaptchaAI, DeathByCaptcha); OpenSubtitles.com moved in, `subtitles-opensubtitles` removed
  - [x] The plugin: provider settings, searches of every enabled provider at once, Bazarr's throttling by kind of failure kept in its data folder, OpenSubtitles.com ported from Bazarr's provider (titles found by name without an IMDb ID, forced and translated subtitles filtered, its retries and failures); `subtitles-opensubtitles` removed
  - [x] Archives: zip, RAR and 7z, and subtitle files served bare, told apart by their content; the subtitle picked as Bazarr's `get_subtitle_from_archive`, by episode title (difflib's ratio), number (read as guessit reads file names) or the first for a movie; Bazarr's archive cases ported
  - [x] Network: one proxy for every provider (HTTP, SOCKS4, SOCKS5) with Bazarr's list of hosts reached directly; Cloudflare's challenges solved with FlareSolverr, whose cookies and user agent are reused per host; captchas solved by Anti-Captcha, CaptchaAI or DeathByCaptcha (reCAPTCHA v2 and images), as Bazarr's pitchers
  - [x] JSON APIs with accounts: SubDL, SubSource, Jimaku, Assrt, BetaSeries, Titlovi, LegendasNet, SubX, Subs.ro, HDBits, Ktuvit, SubsDump, Subsarr
  - [x] JSON APIs without accounts and other protocols: Gestdown, AnimeTosho (both), Tsukihime, Subtis, RegieLive, Bayflix, Vladoonmooo, Subsynchro, Wizdom, BSPlayer, NapiProjekt, Napisy24; AniDB and the anime lists for anime IDs
  - [x] Sites scraped without accounts: AnimeKalesi, AnimeSubInfo, GreekSubs, GreekSubtitles, Hosszupuska, Nekur, PrijevodiOnline, SousTitres.eu, SubClub, Subf2m, Subs4Free, SubsCenter, SubsSabbz, SubsUnacs, SubtitleCat, Subtitrari Noi, Subtitriid, SubtitulamosTV, SuperSubtitles, Titrari, TVSubtitles, YavkaNet, YIFY Subtitles
  - [x] Sites with logins, cookies or captchas: Addic7ed, AvistaZ, CinemaZ, Karagarga, LegendasDivx, Pipocas, Subs4Series, Titulky, TurkceAltyazi.org, XSubs, Zimuku
  - [ ] Later: Jimaku's anime IDs through `internal/anidb`; one release name parser for the providers in place of the guessit approximations of `scrape` and `archivemix` (other properties than season, episode, year and title); the sanitizing and release group helpers of `scrape` and `sitekit` merged, and tests of `sitekit`; provider caches kept in the data folder across restarts; Tsukihime's streaming service guess; GreekSubtitles' encoding hint; random user agents where Bazarr picks them
- [ ] `SubtitleService` and the jobs: subtitle profiles, their assignments and the subtitle settings, wanted subtitles, wanted searches, upgrades, a search at import and scan, manual search, download, upload, deletion, history, blocklist; `subtitle.downloaded` activities; synchronization after downloads under the settings' score thresholds and by hand, against the audio on local and network storage and against another subtitle file on every storage class
  - [x] `apps/server/internal/subtitles` and `SubtitleService`: profiles, assignments and settings, wanted subtitles found anew from the profiles and the streams, wanted searches and upgrades as recurring jobs, a search once a new or changed file is probed and its item refreshed, imports included, manual search, download, upload, deletion, mods and blocklisting by hand, history and blocklist; unusable subtitles blocklisted, failures to save not; `subtitle.*` activities; synchronization against the audio after downloads under the thresholds and by hand, on local and network storage only; the NapiProjekt hash of local and network files; subtitles saved beside their videos, in a folder of the video's folder or in one absolute folder, where scans find them too
  - [ ] Synchronization against another subtitle file
  - [ ] Wanted subtitles kept in a table, should listing them anew be too slow for large libraries
- [x] OpenSubtitles hashes kept per file until it changes: on its media source, which scans replace when the file changes
- [ ] Embedded subtitles: counted as present where the settings say so; extracted to sidecars by hand on local and network storage from the playback cache, never on cloud storage
  - [x] Counted as present where the settings say so, PGS, VobSub and ASS ignored as they say
  - [ ] Extracted to sidecars by hand on local and network storage from the playback cache
- [ ] Subzero's mods (common, tags, HI removal, OCR fixes with its dictionaries, colors, emoji, RTL, upper case, offsets, frame rates, two-point fit) and UTF-8 in `libs/subtitle`; hearing-impaired detection from text; language detection of sidecars without a language, Chinese scripts told apart
  - [x] Subzero's mods (`Subtitle.ApplyMods`): every mod by its Bazarr identifier, in Subzero's order and with its rules (exclusive mods, merged offsets, mods per language, the upper-case check, cues dropped when empty), Subzero's expressions run by regexp2; the OCR fix dictionaries, emoji, public suffixes, guess_language's models and the CC-CEDICT character sets generated by `internal/cmd/subzerogen`; results the same as Bazarr's own code on 25 scenarios (`testdata/mods/golden.py`)
  - [x] UTF-8: `ToUTF8` converts subtitles in legacy character sets
  - [x] Hearing-impaired detection from text (`DetectHearingImpaired`), as Bazarr's `parse_for_hi_regex`
  - [x] Language detection (`DetectLanguage`), as guess_language without spelling dictionaries; Chinese told apart by the character sets of hanzidentifier (`zh-Hans`, `zh-Hant`, `zh` when only shared characters appear; `DetectChineseScript`)
  - [x] Applied to downloads by the settings' mods and to sidecars by hand (`ModifySubtitle`); mods the settings name are checked
  - [ ] ftfy's text fixes, which Bazarr applies to every subtitle it saves (mojibake, HTML entities, control characters)
  - [ ] Styles and comments of ASS and SSA files kept through the mods, which work on the cues
- [x] Silero VAD (`libs/media/vad`): the 16 kHz model's inference in Go, its weights read from Silero's ONNX file by `internal/cmd/silerogen` and embedded, probabilities within 1e-5 of onnxruntime's on generated noise and on Silero's test recording (the `speech.wav` fixture); a minute of audio takes 0.16 s on ten cores of an M1 Max, 0.5 s on one
- [x] Synchronization (`libs/media/subsync`, ffsubsync's algorithm over Silero's speech): the speech of cues and of audio read by ffmpeg, alignment by FFT cross-correlation with the frame rate ratios, the golden-section search and the inferred ratio, ffsubsync's quality gate; offsets, ratios and scores the same as ffsubsync's on generated subtitle pairs (`testdata/golden.py`); two hours of subtitles aligned in 0.25 s
- [ ] End-to-end test (`apps/server/internal/server/subtitles_test.go`)
  - [x] A scanned film gets the French subtitles its library's profile wants from the subtitles plugin against a fake Subsarr on the loopback interface; deletion, upload, mods by hand and the settings' checks through `SubtitleService`
  - [ ] An imported movie and episode, an upgrade, a subtitle failing to decode blocklisted and replaced, and a subtitle out of sync with speech synchronized
- [ ] Later: translation (Google, Gemini, Lingarr) and Whisper transcription, as provider types of their own

---

## 4. Risks and Open Items

| Item | Description | When |
| :--- | :--- | :--- |
| Platforms on the wazero interpreter | SQLite and WASM codec performance degrades on armv7 / riscv64 | Measure in P1; enable pure-Go fallback build tags for those platforms if needed |
| Cost of dual-dialect sqlc | Hot queries need two copies of the SQL | Keep the ≤ 30-query budget; rely on conformance tests |
| Pure-Go image resizing performance | Large images may resize too slowly | Resize large images with ffmpeg ([Architecture §6.2](architecture.md#62-performance-strategy)) |
| SVG rasterization | Pure-Go options are incomplete | Decided in P2: SVGs are checked and served as-is, not rasterized |
| Hardware test coverage | Only the maintainers' machines and GitHub-hosted runners are available; other vendors' encoders are untested on real hardware | Real transcode tests run where the hardware exists and skip elsewhere; other vendor paths rely on the ported EncodingHelper cases |
| Go modules split too finely | Friction in dependency upgrades and tidying | Keep watching; merge modules when needed |
| Image subtitles | PGS and VobSub are only burned in, which forces a video transcode | When a client renders PGS itself (P21); then serve the stream as `.sup` |
| Servarr's services | Acquisition uses the services Radarr, Sonarr and Prowlarr use, and Radarr's TMDB token, without keys of its own; their operators may change or withdraw them | Requests carry the User-Agent of the application whose service it is; an administrator can set their own TMDB key, Mavio's own key stands in when Radarr's token is rejected, and `scraper-servarr` follows the services in its releases |
| Series numbering | Releases and files follow TVDB's numbering, TMDB's metadata may number the same episodes otherwise, anime in particular | Display orders with TMDB episode groups and episode TVDB IDs in P15 and P16; scene mappings and XEM in P19 |
| Indexer churn | Sites change their pages and APIs | Cardigann definitions are fetched at run time; the indexers plugin is upgraded from the catalog without a server release |
| Subtitle site churn | Most of Bazarr's providers scrape sites that change their pages, block bots or close | The providers are in `plugins/subtitles`, upgraded from the catalog without a server release; recorded responses keep their parsers tested |
| Speech detection for synchronization | Silero VAD, a neural network, costs about 0.7 million multiply-adds every 32 ms of audio in pure Go, about a minute of one core for a two-hour film; ffsubsync's offsets come from WebRTC's VAD by default | The convolutions run in parallel and only the LSTM in order, as a low-priority job yielding to playback; offsets compared with ffsubsync's on recorded samples in P26 |
| Live TV, DVR and channels | Large parts of Jellyfin that Mavio has neither adopted nor ruled out; the plugin platform leaves out their provider types | Decide before P21 |
| Client licensing and app stores | The clients would be GPL-3.0 like the rest of the repository, whose terms conflict with the App Store's; LGPL builds of FFmpeg (AetherEngine's frameworks on Apple platforms) also oblige letting users relink them, and the Android apps' libmpv, built as GPL-3.0, binds them to GPL-3.0 | Decide before the first App Store release: a separate license for the clients, a contributor agreement, or no App Store distribution |
| AetherEngine | The second engine of the Apple apps is a young package, mostly by one maintainer, releasing often in 7.x | Prototype first in P21: EDR output of an HDR10 Matroska source, Dolby Vision, styled ASS subtitles and Picture in Picture on macOS, iOS and tvOS; pin the version and follow its releases |
| libmpv on Android | libmpv is the second engine of the Android apps; its OpenGL ES rendering may be slow on the weak GPUs of Android TV devices, many of them 32-bit, and it outputs no HDR | Measure on low-end Android TV devices in P21; where libmpv cannot keep up, the app leaves it out of its capabilities and the server processes the source |
| connect-rust before 1.0 | `libs/client-rust` uses connect-rust, which is still in 0.x and may change its API | Pin the version; follow its releases |
| HDR on Linux | The Linux app draws video with OpenGL in a GTK 3 `GtkGLArea`, which presents SDR only, so HDR is tone mapped | Revisit when MyGo's Linux backend can present HDR surfaces |
| MyGo | `apps/desktop` builds on MyGo, started in September 2026 and still in 0.x, mostly by one maintainer. Its release has no element that places a native surface drawn by another renderer under its native UI, which the player needs for mpv's video (a view on macOS, a child window on Windows, a `GtkGLArea` on Linux) | The video element, done on macOS in the fork `mavioai/mygo` (branch `v0.4.5-mavio`), pinned to a commit and rebased on MyGo's releases; contribute it upstream; Windows with HDR10 next |
| mpv on macOS | mpv's macOS output cannot embed in a host's view; Mavio's patch in mavio-deps makes it, and must follow mpv's changes to that code | Rebase the patch with each mpv pin; offer it to mpv |

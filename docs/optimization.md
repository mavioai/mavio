# Optimization

> English | [简体中文](optimization.zh-CN.md)

> Related: [Architecture](architecture.md) · [Domain Model](domain.md) · [Development](development.md) · [Testing](testing.md) · [Roadmap](roadmap.md) · [AGENTS.md](../AGENTS.md)

> This document defines Mavio's global optimization architecture, covering storage I/O scheduling, speculative prefetching, low-latency playback pipelines, subtitle metadata resolution, and deterministic engineering invariants.

---

## 1. Goals and Design Principles

As a self-hosted media server, Mavio's primary deployment environments are home servers, multi-bay NAS appliances (e.g. Synology, TrueNAS, Unraid), and low-power hardware. These environments feature severe storage heterogeneity:
1. **Mechanical hard drives (HDD) have fragile throughput**: HDDs incur a 10~15ms physical seek penalty. Concurrent multi-task random reads cause violent head thrashing, causing throughput to plummet from 150MB/s down to single-digit MB/s.
2. **Network storage and external drives spin down easily**: SMB/NFS mount points and external USB drives automatically enter sleep/spin-down after several minutes of inactivity. Re-spinning the platter takes 5~10 seconds, which frequently causes client playback timeouts.
3. **Severe foreground vs. background I/O contention**: If background reconciliation scans or trickplay thumbnail generation saturate the disk I/O queue, real-time client playback and HLS segment delivery suffer severe buffering and stuttering.
4. **Growing and incomplete files produce dirty reads**: Partially downloaded media files from PT/BT, Aria2, or download automation tools cause `ffprobe` failures or register incorrect durations if scanned prematurely.

To address these challenges, Mavio establishes the following storage and media optimization principles:
* **Physical volume awareness to eliminate concurrent seeking**: Background I/O must be serialized into single-flight lanes per physical device, preserving peak sequential read throughput on mechanical disks.
* **Absolute priority for foreground streaming**: Whenever an active client playback session exists, background scans and offline tasks must yield and throttle down.
* **Targeted prefetching with minimal budget**: Avoid uncontrolled blind buffering; accurately target file heads (metadata) and file tails (indexes) with minimal memory overhead.
* **Zero-intrusion, cross-platform adaptability**: Seamlessly support Linux, Windows, and macOS using each operating system's most efficient native system calls—transparent and fast on SSDs, protective on HDDs.

---

## 2. Storage I/O Latency Mitigation and Speculative Warming System

### 2.1 Multi-Dimensional Storage & Protocol Identification

When scanning and reading media paths, Mavio identifies the underlying storage medium and mounting protocol through a cross-platform abstraction layer:

```text
               ┌───────────────────────────┐
               │    Storage Path Probe     │
               └─────────────┬─────────────┘
                             │
     ┌───────────────────────┼───────────────────────┐
     ▼                       ▼                       ▼
   Linux                  Windows                  macOS
┌──────────────┐      ┌──────────────┐      ┌──────────────┐
│ stat.st_dev  │      │ GetDriveType │      │ statfs       │
│ sysfs /queue │      │ DeviceIo-    │      │ f_fstypename │
│ statfs.f_type│      │ Control IOCTL│      │ f_fsid / dev │
└──────┬───────┘      └──────┬───────┘      └──────┬───────┘
       │                     │                     │
       └─────────────────────┼─────────────────────┘
                             ▼
               ┌───────────────────────────┐
               │    Storage Device Type    │
               │  • Local SSD              │
               │  • Local HDD (Rotational) │
               │  • Remote NAS (SMB/NFS)   │
               │  • Cloud Mount (Dataless) │
               └───────────────────────────┘
```

#### Linux
1. **Mechanical Disk (Rotational) Detection**:
   * Extract the device identifier `st.st_dev` via `stat(path, &st)`;
   * Read `/sys/dev/block/<major>:<minor>/queue/rotational`:
     * `1`: Rotational medium (mechanical HDD); automatically activates per-device single-flight anti-thrashing queues;
     * `0`: Non-rotational medium (NVMe / SATA SSD); allows regular high-concurrency I/O.
2. **Network Volumes and Protocol Detection**:
   * Inspect `buf.Type` via `statfs(path, &buf)`:
     * `0x517B` (`SMB_SUPER_MAGIC`) / `0xfe534d42` (`SMB2_MAGIC_NUMBER`): SMB share;
     * `0x6969` (`NFS_SUPER_MAGIC`): NFS mount;
     * `0x65735546` (`FUSE_SUPER_MAGIC`): FUSE mount (such as Rclone / Alist).

#### Windows
1. **Mechanical Disk Detection**:
   * Obtain volume handle via `CreateFileW` and invoke `DeviceIoControl` with `IOCTL_STORAGE_QUERY_PROPERTY` querying `StorageDeviceSeekPenaltyProperty`;
   * If `DEVICE_SEEK_PENALTY_DESCRIPTOR.IncursSeekPenalty` is `TRUE`, the drive is classified as an HDD.
2. **Network Volume Detection**:
   * Check if `GetDriveTypeW(rootPath) == DRIVE_REMOTE` or detect UNC path prefixes (`\\server\share`).
3. **Cloud Placeholder Interception**:
   * Check if file attributes include `FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS` (`0x00400000`);
   * Open background scan files with `FILE_FLAG_OPEN_NO_RECALL` to prevent triggering silent cloud downloads.

#### macOS
1. **Network Volume Detection**:
   * Match `statfs.f_fstypename` against `"smbfs"`, `"nfs"`, `"afpfs"`, `"webdav"`; and ensure `(sfs.f_flags & MNT_LOCAL) == 0`.
2. **Cloud and Placeholder File Interception**:
   * Path classification against `~/Library/Mobile Documents` (iCloud) and `~/Library/CloudStorage` (FileProvider cloud storage);
   * Thread-level system policy `IOPOL_TYPE_VFS_MATERIALIZE_DATALESS_FILES` set to `IOPOL_MATERIALIZE_DATALESS_FILES_OFF`.

---

### 2.2 Per-Device Single-Flight Serialization & Anti-Thrashing

* **Rationale**: When thousands of files reside on the same mechanical disk, concurrent probe tasks force drive heads to continuously seek back and forth between inner and outer platters, collapsing overall I/O latency.
* **Scheduling Mechanism**:
  * Introduce a `DeviceLedger` in `libs/library`.
  * Extract the physical device identifier (Linux: `st_dev` / major:minor; Windows: Volume Serial Number; macOS: `f_fsid`) as the lock key.
  * **Single-Device Semaphore**: For devices classified as HDDs or remote mounts, **only 1 background scan/probe worker may hold an active I/O token per physical device** at any given moment.
  * **Result**: Tasks run sequentially, keeping the drive's actuator arm in continuous sequential-read mode and maintaining hardware peak throughput while eliminating head thrashing and mechanical wear.

---

### 2.3 Active Streaming & Background I/O Quiet Gate

* **Rationale**: During full library scans, hashing, or trickplay sprite generation, active client playback can suffer severe buffering if foreground requests contend with background tasks for disk queues.
* **Scheduling Mechanism**:
  * The session manager maintains an active streaming session count (`activeStreamingSessions`).
  * When a client initiates playback, seeks, or requests HLS segments, an active quiet window is triggered (`quietUntil = now + quietPeriod`, default 3 seconds).
  * Background tasks (scan workers, metadata fetchers, keyframe extractors) must check the gate before issuing physical I/O:
    * While in a quiet window, workers yield via `time.Sleep` and pause disk contention;
    * Dynamically shrink `errgroup.SetLimit` to minimum concurrency;
    * Once all streaming sessions end and the quiet window expires, background queues resume full speed smoothly.

---

### 2.4 Targeted Metadata Budget Prefetching

* **Rationale**: Video container headers and indexes (MP4/MKV) are concentrated at the two ends: the beginning contains file signatures and track descriptors, while the end stores indexes (such as MP4 trailing `moov` boxes, MKV trailing `Cues`, and AVI `idx1`). Blindly buffering entire files wastes memory and bandwidth.
* **Prefetch Strategy**:
  * **Head 1 MB**: Read the first 1MB, sufficient for almost all MP4 `ftyp/moov`, MKV EBML headers, and MPEG-TS headers.
  * **Tail 256 KB**: Seek directly to the end and read the trailing 256KB to cover index data.
  * **Linux Kernel-Level Asynchronous Prefetch**:
    On Linux, the server avoids allocating application memory buffers and directly leverages kernel prefetching:
    ```c
    posix_fadvise(fd, 0, 1024 * 1024, POSIX_FADV_WILLNEED);
    posix_fadvise(fd, fileSize - 256 * 1024, 256 * 1024, POSIX_FADV_WILLNEED);
    ```
    The kernel asynchronously brings the target sectors into Page Cache, allowing subsequent `ffprobe` calls and playback demuxers to hit memory immediately without waiting.

---

### 2.5 Remote Volume & Spin-Down Prevention Heartbeat

* **Rationale**: External USB mechanical drives and NAS shares commonly spin down after 5~15 minutes of inactivity. When a user browses media in preparation for playback, if the disk is asleep, starting playback requires a 5~10 second platter spin-up delay, giving the illusion of a frozen server.
* **Heartbeat Mechanism**:
  * **Controlled Heartbeat Interval**: When an active client browsing session is detected (e.g. via persistent WebSocket/Connect-RPC) and the media resides on a remote or external disk, a lightweight heartbeat is emitted every 2.5 seconds.
  * **Cache-Bypassing Micro-Reads**:
    * Linux uses `O_DIRECT`, Windows uses `FILE_FLAG_NO_BUFFERING`, and macOS uses `fcntl(F_NOCACHE, 1)`;
    * Issues a 4KB `pread` at a random file offset, forcing the drive controller to process a physical read and keeping the motor spinning.
  * **Presence and Power Preservation**: The moment the client disconnects or becomes idle, heartbeats stop immediately, respecting storage power-saving features.

---

### 2.6 Growing & Incomplete File Lifecycle Management (Growth Policy)

* **Rationale**: Media directories often interface directly with download clients (qBittorrent, Aria2, Sonarr, Radarr). Forcing a full probe on an actively downloading file causes crashes, incorrect durations, and corrupt playback state.
* **Lifecycle State Machine**:
  * **Temporary Download Suffix Filtering**: The scanner silently ignores temporary extensions such as `.part`, `.crdownload`, `.download`, `.aria2`.
  * **Active Growth Detection**:
    * Track file `(size, mtime)` nanosecond stamps;
    * If `time.Now() - mtime < 10s` or active writer processes hold open locks (on Linux), mark the file as `Growing`.
  * **Deferred Reconciliation Queue**:
    * Files in `Growing` state bypass expensive `JobProbe` tasks and raise no errors;
    * They enter an in-memory deferred queue and undergo full reconciliation only after size and modification timestamps have remained stable beyond a set threshold (e.g. 30 seconds).

---

## 3. Media Pipeline & Playback Latency Optimization

### 3.1 Resume Seek Folding & Keyframe Snapping

* **Traditional Resume Latency**: The standard sequence is: Open file $\rightarrow$ Read metadata $\rightarrow$ Seek to timestamp $\rightarrow$ Seek backwards to prior keyframe $\rightarrow$ Decode and discard non-reference frames up to the target timestamp. On slow storage, these multi-step seeks take several seconds.
* **Optimization Strategy**:
  * **Fold Seek into Prepare**: Pass the resume timestamp during demuxer initialization, positioning the initial read cursor near the resume target to eliminate secondary random seeking after open.
  * **Keyframe Snapping**:
    If the target resume timestamp is within 5 seconds of the preceding keyframe (`diff <= 5.0s`), **snap playback start directly to that keyframe**.
    * Eliminates CPU overhead from decoding and discarding seconds of lead-in frames;
    * The first delivered frame is an immediate keyframe, allowing decoders to present picture on screen immediately, reducing first-frame latency by over 70%.

---

### 3.2 Dynamic Black Border Auto-Cropping & Failure Circuit Breaker

* **Black Border Auto-Cropping** (`libs/imaging`):
  * 2.35:1 movies formatted into 16:9 trickplay/BIF preview sprites contain wide letterbox bars, reducing effective picture area and wasting 20%~30% of image size.
  * Introduce a pure-CPU 1/4 subsampled border detection algorithm (supporting 8-bit YUV420 and 10-bit P010):
    * Samples along top, bottom, left, and right rows/columns;
    * Early-exit evaluation across the first 16 samples for instant black-line verification;
    * Crops borders or attaches crop bounds prior to sprite assembly, boosting thumbnail clarity and saving bandwidth.
* **Thumbnail Failure Circuit Breaker**:
  * Repeatedly retrying broken GOPs during thumbnail generation can hang background queues.
  * Introduce an in-memory `FailNote` cache (recording failed time ranges, TTL, and capacity limits); repeat extraction requests within the TTL window are suppressed, keeping background workers healthy.

---

### 3.3 Dolby Vision Profile 7 Dual-Layer Dependency Handling

* **UHD Blu-ray Challenges**: UHD Blu-ray discs often carry Dolby Vision Profile 7 (FEL / MEL) with two video streams: Base Layer (BL) and Enhancement Layer (EL). Passing both directly to standard FFmpeg transcode commands causes sync loss or crashes.
* **Planner Optimizations** (`libs/media/planner/hdr.go`):
  * Parse MPEG-TS PMT and MP4 `vdep` stream relationships, recognizing EL as a dependent stream;
  * When clients support only HDR10, configure FFmpeg filter graphs (`-map`) to strip the EL stream cleanly, preserving the pristine Base Layer for tone-mapping and encoding;
  * When clients support Profile 8.1, route through dedicated filters to restructure dual-layer metadata into single-layer Profile 8.1 streams.

---

### 3.4 Multichannel Downmixing & Dialogue Normalization

* **Downmixing Issues**: Downmixing 5.1/7.1 audio into stereo often results in faint spoken dialogue and deafening background sound effects.
* **Planner Optimizations** (`libs/media/planner/audio.go`):
  * Follow ITU-R BS.775 downmix matrices during audio transcode planning;
  * Apply center channel dialogue normalization boost (+3dB ~ +4.5dB);
  * Protect mobile and stereo TV listeners from constant manual volume adjustments.

---

## 4. Subtitle & Metadata Resolution

### 4.1 Subtitle Scoring Model & Comprehensive Chinese Alias Normalization

To handle widespread variations in subtitle filenames, `libs/naming` and `libs/library` standardize on a unified alias dictionary and scoring model:

#### Normalized Aliases
```text
"zh", "chi", "zho", "chs", "cht", "sc", "tc", "gb", "big5",
"zh-hans", "zh-hant", "zh-cn", "zh-tw", "zh-hk",
"简体", "繁體", "繁体", "简中", "繁中", "简", "繁", "中文", "双语" ──▶ "zh"
```

#### Scoring Rules
| Match Attribute | Score Adjustment | Rationale |
| :--- | :--- | :--- |
| **Exact Stem Match** | `+1000` | E.g. `Movie.mkv` matching `Movie.srt` |
| **Prefix Match + Boundary Delimiter** | `+500` | E.g. `Movie.1080p.mkv` matching `Movie.zh.srt` |
| **User Preferred Language Match** | `+200` | Matches user's configured preferred subtitle language |
| **Other Non-Preferred Language** | `-100` | Penalizes mismatched subtitle languages |
| **`.forced` Tag** | `-150` | Forced subtitles contain few dialogue lines; deprioritized by default |
| **`.sdh` / `.cc` / `.hi` Tag** | `-50` | Hearing-impaired commentary; secondary choice |
| **Format Bonus** | `ASS/SSA: +20` / `SRT: +10` | Favors richer styling formats |

---

## 5. Deterministic Engineering & Privacy Invariants

### 5.1 Pinned Dependencies & Zero-Fuzz Patch Discipline

* **Deterministic Dependency Pinning**: Official Docker images and external dependencies (`jellyfin-ffmpeg`, WASM toolchains, SQLite extensions) strictly lock archive SHA-256 checksums.
* **Zero-Fuzz Patch Rule**: Downstream patches must have pinned checksums in the dependency manifest. Patches are applied with zero fuzz; any upstream line drift fails the build immediately rather than silently shifting code hunks.

### 5.2 Zero-Telemetry & Fail-Closed Privacy Model

* **Zero Telemetry**: Mavio contains no playback analytics, device tracking, or telemetry reporting. All data strictly stays on the user's host.
* **Fail-Closed Silence**: In the absence of configured external metadata scrapers (such as a TMDB API key), Mavio operates silently using local NFOs and images without making unnecessary outbound network calls.

---

## 6. Implementation Index by Module

| Optimization Feature | Target Mavio Module | Target Phase |
| :--- | :--- | :--- |
| **Subtitle Scoring & Chinese Alias Normalization** | `libs/naming`, `libs/library` | P2 / P4 |
| **Growing File Policy & Deferred Reconciliation** | `libs/library/scan.go` | P4 |
| **Storage Medium Detection & Per-Device Queues** | `libs/library/fs.go`, `libs/library/scan.go` | P4 |
| **Active Streaming & Background I/O Quiet Gate** | `apps/server`, `libs/library/worker.go` | P5 |
| **Targeted Prefetching (Head/Tail Prefetch)** | `libs/media/probe`, `libs/library` | P3 / P4 |
| **Resume Seek Folding & Keyframe Snapping** | `libs/media/planner`, `libs/streaming` | P3 / P5 |
| **Thumbnail Auto-Cropping & Failure Circuit Breaker** | `libs/imaging`, `libs/media/keyframes` | P2 / P5 |
| **Dolby Vision Profile 7 Dependency & Downmix Boost** | `libs/media/planner/hdr.go`, `audio.go` | P3 |

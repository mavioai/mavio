# Optimization

> English | [简体中文](optimization.zh-CN.md)

> Related: [Architecture](architecture.md) · [Domain Model](domain.md) · [Development](development.md) · [Testing](testing.md) · [Roadmap](roadmap.md) · [AGENTS.md](../AGENTS.md)

> This document collects Mavio's optimization topics, one chapter per topic. Each chapter states the problem, the mechanisms adopted and an implementation index mapping every mechanism to the code that implements it, the phase that delivered it and where it departs from Jellyfin, so that later alignment with Jellyfin does not mistake a deliberate difference for a gap.

---

## 1. Storage I/O Latency Mitigation and Speculative Warming

### 1.1 Goals and Challenges

Mavio is deployed on home servers, NAS appliances (Synology, TrueNAS, Unraid) and low-power hardware, whose storage is heterogeneous:

1. **Hard disks thrash**: a seek costs 10–15 ms; concurrent random reads of one disk move the heads back and forth and drop its throughput from about 150 MB/s to a few MB/s.
2. **Disks and shares sleep**: external USB disks and NAS disks spin down after minutes of inactivity, and SMB/NFS shares disconnect; spinning up takes 5–10 seconds, long enough for a client to give up on starting playback.
3. **Background I/O competes with playback**: scans, probes and thumbnail extraction filling a disk's queue make playback stall.
4. **Files still being written**: files that download clients or automation tools (qBittorrent, Aria2, Sonarr, Radarr) are still writing make ffprobe fail or record wrong durations when scanned too early.
5. **Resuming costs seeks**: opening a file, reading its index, seeking to the resume position and then decoding from the previous keyframe takes several round trips on slow storage.

The principles adopted:

* **Know the device**: background reads of one hard disk or remote volume are serialized; solid-state drives keep full concurrency.
* **Playback first**: while clients stream, background reads wait.
* **Small, targeted reads ahead**: read the container header and index and the media around the start position, not whole files.
* **Wake before it is needed, and only while someone is there**: keep the volumes being played awake, and wake the volumes of media a present user is likely to play; let disks sleep otherwise.
* **Native on every platform**: each operating system's own calls, without CGO.

### 1.2 Storage Identification

`storage.DetectDevice` classifies the volume holding a path as a local SSD, a local hard disk, a remote share or a cloud mount, and returns a stable volume ID that keys every per-volume mechanism below. `storage.Detector` detects each folder once (at most 4,096 folders are remembered); a path that cannot be detected is unknown and treated as a solid-state drive.

| Platform | Volume ID | Hard disk | Remote share | Cloud mount |
| :--- | :--- | :--- | :--- | :--- |
| Linux | `st_dev` | `/sys/dev/block/<major>:<minor>/queue/rotational` is `1` | `statfs.f_type` SMB (`0x517B`), SMB2 (`0xFE534D42`), CIFS, NFS (`0x6969`) | FUSE (`0x65735546`: rclone, Alist, …) |
| Windows | volume serial number | `IOCTL_STORAGE_QUERY_PROPERTY` with `StorageDeviceSeekPenaltyProperty` | `GetDriveTypeW` is `DRIVE_REMOTE`, or a UNC path | path under OneDrive or iCloud Drive |
| macOS | `statfs.f_fsid` | volumes under `/Volumes` (external enclosures) | `f_fstypename` `smbfs`, `nfs`, `afpfs`, `webdav`, or no `MNT_LOCAL` | path under `~/Library/Mobile Documents` or `~/Library/CloudStorage` |

Two properties are derived from a device:

* **Serialized** (`DeviceInfo.Serialized`): hard disks, remote shares and cloud mounts; their background reads run one at a time.
* **Sleeps** (`DeviceInfo.Sleeps`): hard disks and remote shares; they are kept awake and woken. Cloud mounts are never read speculatively, since reading a placeholder downloads it.

### 1.3 Per-volume Serialization

* **`storage.VolumeLedger`**: one slot per volume ID, handed out first come, first served; `Acquire` waits for the slot or for the context to end, and returns its release. A volume's lane is dropped when no one holds or waits for it.
* **Scans**: the walker reads folders with 4 goroutines (the library's concurrency) on solid-state drives and with one on serialized volumes; each folder listing holds its volume's slot.
* **Probes**: probing a file on a serialized volume holds the volume's slot; versions of one item on one disk are probed one after another, the probe never waiting for itself.
* The result is that a hard disk serves one sequential reader at a time across all scans and probes of all libraries.

### 1.4 Quiet Gate for Background I/O

* **`storage.QuietGate`**: foreground activity extends a quiet deadline to now plus the quiet period (3 seconds by default) and never shortens it, so a burst of requests yields one quiet tail without a timer per request.
* **Activity**: starting a playback and every media request (HLS playlists and segments, direct-play range reads) note activity. Continuous streaming keeps background reads paused while it reads, and lets them run in its gaps; a playback does not hold the gate for its whole length, which would stop scans for as long as anyone watches.
* **Waiters**: before each folder a scan walker, and before reading a file a probe, black border detection, trickplay, chapter image and loudness jobs wait out the quiet deadline (`PauseOrCancel`), honoring cancellation.

### 1.5 Targeted Prefetching

* **Head and tail**: `storage.PrefetchHeadTail` reads ahead the first 1 MB (MP4 `ftyp`/`moov`, the MKV EBML header, MPEG-TS headers) and the last 256 KB (a trailing MP4 `moov`, MKV `Cues`, AVI `idx1`) of a file. It runs before every probe and before every playback.
* **Range**: `storage.PrefetchRange` reads ahead an arbitrary range clipped to the file. A playback starting past zero reads ahead the media around its start position, estimated from the source's size and duration: 1 MB before it and 3 seconds after it, at most 64 MB.
* **Mechanism**: on Linux, `posix_fadvise(POSIX_FADV_WILLNEED)` lets the kernel load the ranges into the page cache asynchronously without application buffers; elsewhere the ranges are read in 128 KB chunks and discarded, warming the system cache.

### 1.6 Keeping Volumes Awake and Waking Them

* **Heartbeat read** (`storage.VolumeHeartbeat`): 4 KB at a random 4 KB-aligned offset of a file, bypassing the cache so that the read reaches the disk or share: `O_DIRECT` on Linux (falling back to `POSIX_FADV_DONTNEED` and a normal read on file systems that refuse it), `F_NOCACHE` on macOS, and `FILE_FLAG_NO_BUFFERING | FILE_FLAG_OPEN_NO_RECALL` on Windows.
* **`storage.Keeper`** reads only volumes that sleep:
  * `Beat` reads each volume at most once per heartbeat interval (2.5 seconds), however many paths name it; a volume whose last read has not returned (a hung share) is skipped.
  * `Wake` reads a volume at most once per wake cooldown (30 seconds) and then prefetches the file's head and tail.
* **Three triggers** in the server:
  1. **Playback keep-alive** (`playback.Manager`): every heartbeat interval, the volumes of all playbacks in progress are beaten, so a paused playback resumes without waiting for a spin-up.
  2. **Presence warming** (`apps/server/internal/warming`): every 30 seconds, for each user with a client in use (an event stream open and a request within the last 5 minutes), the media of their 8 most recently played resumable items ("continue watching") is looked up; every heartbeat interval their volumes are beaten. When no user is present, nothing is read and the disks may sleep.
  3. **Opening an item** (`ItemService.GetItem`): the media sources of the item a client opens are woken, so the volume is spinning by the time the client asks to play.

### 1.7 Files Still Being Written (Growth Policy)

* **`storage.GrowthPolicy`** decides during a scan whether a file is still growing:
  * its name has a download suffix (`.part`, `.crdownload`, `.download`, `.aria2`, `.tmp`, `.incomplete`, `.!ut`, `.!qb`);
  * it was modified within the write window (10 seconds);
  * or it was deferred before and its size and modification time have not been stable for 30 seconds.
* **Deferral**: a growing file is left out of the scan without errors or probes and is deferred for its library (at most 4,096 files; one that never settles, such as a paused download, is forgotten after a day). Its folder is not marked unchanged, so a later scan lists it again although its modification time did not change.
* **Reconciliation** (`library.Scanner.RunDeferred`, every 10 seconds): deferred files are stat'ed; once a file went away (a finished download renamed) or stayed unchanged for 30 seconds after its write window, a scan of its library is queued, once per library.
* Without a policy, the scanner still skips files with download suffixes.

### 1.8 Resume Seek Folding and Keyframe Snapping

* **Keyframe snapping** (`playback.Playback.startPosition`): a playback asked to start at a position begins at a keyframe when one is at most 5 seconds before it, so the decoder shows a picture at once instead of decoding and dropping frames:
  * for HLS, the start of the segment holding the position, which the encoder or the source begins with a keyframe;
  * for direct play, the source's keyframe before the position (keyframes stored by the `media.keyframes` job).

  The position used is returned in `StartPlaybackResponse.start_position`, so the client seeks there and the player and the server agree.
* **Seek folding** (`streaming.Stream.Prepare`): starting an HLS playback starts ffmpeg at the segment holding the start position right away, instead of on the first segment request; the first request of that segment finds ffmpeg already reading at the right place and no restart is needed.
* **Reading ahead** (§1.5): the source's head, tail and the range around the start position are read ahead in the background as the playback starts.

### 1.9 Implementation Index

| Mechanism | Implementation | Phase | Relation to Jellyfin |
| :--- | :--- | :--- | :--- |
| Device and protocol identification | `libs/library/storage/device.go`, `device_{linux,darwin,windows,other}.go` (`DetectDevice`, `Detector`, `DeviceInfo.Serialized`, `DeviceInfo.Sleeps`) | P9 | Mavio only; Jellyfin does not classify storage |
| Per-volume serialization | `libs/library/storage/anti_thrashing.go` (`VolumeLedger`); used by `libs/library/scan.go` (`Scanner.walkers`, `scan.folder`) and `libs/library/jobs.go` (`Jobs.probeFile`) | P9 | Mavio only; Jellyfin scans and probes with fixed parallelism whatever the device |
| Quiet gate | `libs/library/storage/quiet_gate.go` (`QuietGate`); activity from `apps/server/internal/playback/manager.go` (`Manager.Start`) and `playback/http.go`; waited by `libs/library/scan.go`, `jobs.go`, `borders.go`, `mediaextras.go` | P9 | Mavio only; Jellyfin's scheduled tasks do not yield to playback |
| Head and tail prefetch | `libs/library/storage/prefetch.go`, `prefetch_linux.go`, `prefetch_other.go` (`PrefetchHeadTail`); used by `libs/library/jobs.go` (`Jobs.probeFile`), `storage/keeper.go` (`Keeper.Wake`) and `apps/server/internal/playback/resume.go` (`warmResume`) | P9 | Mavio only |
| Start-position prefetch | `libs/library/storage/prefetch.go` (`PrefetchRange`); `apps/server/internal/playback/resume.go` (`warmResume`) | P9 | Mavio only |
| Uncached heartbeat read | `libs/library/storage/heartbeat.go`, `heartbeat_{linux,darwin,windows,other}.go` (`VolumeHeartbeat`, `Heartbeats`) | P9 | Mavio only |
| Keep-alive and wake | `libs/library/storage/keeper.go` (`Keeper.Beat`, `Keeper.Wake`) | P9 | Mavio only |
| Playback keep-alive | `apps/server/internal/playback/manager.go` (`Manager.Run`), `playback/resume.go` (`Manager.keepAwake`) | P9 | Mavio only |
| Presence warming | `apps/server/internal/warming/warming.go` (`Warmer.Run`, `Warmer.Tick`); online sessions from `apps/server/internal/events/hub.go` (`Hub.OnlineSessions`) | P9 | Mavio only; reads disks only while a user is present |
| Wake on opening an item | `apps/server/internal/rpc/item.go` (`ItemService.Wake` in `GetItem`) → `warming.Warmer.Wake`; wired in `apps/server/internal/server/server.go` | P9 | Mavio only; `GetItem` has the side effect of a background read |
| Growth policy | `libs/library/storage/growth_policy.go` (`GrowthPolicy`, `HasDownloadSuffix`); used by `libs/library/scan.go` (`scan.list`) | P9 | **Differs**: Jellyfin indexes a file as soon as a scan or its file system monitor sees it (its monitor waits for changes to settle and for the file to be unlocked); Mavio leaves out files modified within 10 seconds and defers them until stable for 30 seconds, so a freshly copied file appears up to about 40 seconds later |
| Deferred reconciliation | `libs/library/deferred.go` (`Scanner.RunDeferred`, `Scanner.ReconcileDeferred`); run by `apps/server/internal/server/server.go` | P9 | **Differs**: replaces Jellyfin's real-time file system monitor for files still being written; Mavio has no file system monitor (full reconciliation scans, [Architecture §9](architecture.md)) |
| Keyframe snapping | `apps/server/internal/playback/resume.go` (`Playback.startPosition`); `libs/streaming/layout.go` (`Layout.Index`); `StartPlaybackResponse.start_position` in `libs/proto/mavio/playback/v1/playback.proto` | P9 | **Differs**: Jellyfin starts at the requested position; Mavio may start up to 5 seconds earlier and returns the position it used |
| Seek folding | `libs/streaming/stream.go` (`Stream.Prepare`); called by `apps/server/internal/playback/resume.go` (`Manager.prepareStart`) | P9 | **Differs**: Jellyfin starts ffmpeg on the first segment request; Mavio starts it when the playback starts |

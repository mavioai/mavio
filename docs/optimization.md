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
| Linux | `st_dev` | `/sys/dev/block/<major>:<minor>/queue/rotational` is `1`; FUSE file systems layered over local disks (`fuse.mergerfs`, `fuseblk`, `fuse.gocryptfs`, …) | `statfs.f_type` SMB (`0x517B`), SMB2 (`0xFE534D42`), CIFS, NFS (`0x6969`) served by another host; `fuse.sshfs` and other FUSE network protocols | FUSE (`0x65735546`) by the subtype in `/proc/self/mountinfo`: `fuse.rclone`, `fuse.s3fs`, cloud drive clients and unknown subtypes; NFS served by this machine (`127.0.0.1:/`, rclone nfsmount) |
| Windows | volume serial number | `IOCTL_STORAGE_QUERY_PROPERTY` with `StorageDeviceSeekPenaltyProperty` | `GetDriveTypeW` is `DRIVE_REMOTE`, or a UNC path | path under OneDrive or iCloud Drive; WinFsp volumes named `rclone` or `FUSE-<name>` |
| macOS | `statfs.f_fsid` | volumes under `/Volumes` (external enclosures) | `f_fstypename` `smbfs`, `nfs`, `afpfs`, `webdav`, or no `MNT_LOCAL` | path under `~/Library/Mobile Documents` or `~/Library/CloudStorage`; macFUSE mounts; NFS, SMB and WebDAV served by this machine (rclone nfsmount, FUSE-T) |
| Other | `filepath.VolumeName` | every volume | — | — |

Two properties are derived from a device:

* **Serialized** (`DeviceInfo.Serialized`): hard disks, remote shares and cloud mounts; their background reads run one at a time.
* **Sleeps** (`DeviceInfo.Sleeps`): hard disks and remote shares; they are kept awake and woken. Cloud mounts, and paths whose library classes them cloud (§2.1), are never read speculatively, since reading a placeholder downloads it.

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

---

## 2. Storage Access by Storage Class

### 2.1 Storage Classes

Reading a file costs differently by where it is: a disk of the server is cheap, a share on the local network slower but uncounted, and a cloud drive, mounted with rclone or a vendor client, counts every byte against a download quota and every request against a rate limit. Mavio classes each library path as `local`, `network` or `cloud` (`core.StorageClass`) and lets the class decide what the server reads of the media on its own.

* **Classification** (`storage.Classifier`): the class the library sets for the path (`Library.StorageClasses`, for storage the device detection would misclassify), else the class of the device holding it (§1.2): cloud mounts are `cloud`, remote shares `network`, every other device `local`. A library path is detected as a folder of its own, so that a mount point is classified by its mount and not by the folder above it.
* **Paths without a library** (`storage.PathClasses`): mechanisms that know only a path, such as keeping volumes awake, find the library holding it among the libraries, listed again every minute.
* `LibraryService` returns each path's effective class (`effective_storage_classes`) to administrators, beside the classes they set.

### 2.2 Storage Access Inventory

Everything Mavio reads from or writes to media storage, with its cost and when it happens:

| Source | Operation | Bytes or requests | When |
| :--- | :--- | :--- | :--- |
| Scan | one listing per folder, a stat per entry; unchanged folders are taken from the previous scan where the policy allows | API calls rather than bytes | scan interval, by hand, targeted scans after imports |
| Probe | ffprobe reads the container header, and the tail when the index is there | a few MB, 2–5 range reads | new and changed files |
| Keyframes | the container's index: Matroska Cues, or MP4 sample tables, `mfra` or `sidx` (§2.4); without one, ffprobe reads every packet of the video stream | a few header and index reads; without an index, the whole file | with the probe; without an index, after it where the policy allows whole-file reads |
| Black borders | a seek and a few decoded frames at 5 points | about 3–4 MB a point | after the probe, where the policy allows samples |
| Trickplay | Matroska: a keyframe a thumbnail, read through the Cues (§2.5); other files: ffmpeg decodes the keyframes, or failing that every frame | a keyframe a thumbnail; without an index, the whole file | when the library asks for it; without an index, where the policy allows whole-file reads |
| Chapter images | a seek per chapter | about 3–4 MB a chapter | when the library asks for it and the policy allows samples |
| Loudness | ffmpeg reads the whole track | the whole file | when the library asks for it and the policy allows whole-file reads |
| NFO, artwork, lyrics, sidecar subtitles | small files | small | refreshes, scans |
| OpenSubtitles hash | 64 KB at the head and at the tail | 2 range reads | subtitle searches |
| Playback | range reads for direct play, sequential reads by ffmpeg for remuxes and transcodes | proportional to what is watched | playback |
| Embedded subtitles | local and network: ffmpeg copies every text stream out at once, into the metadata folder (§2.6); cloud: the transcode playing the video writes them as it reads | local and network: the whole file once while it is unchanged; cloud: nothing beyond the playback | playback of an embedded text subtitle the client does not demux |
| Prefetch, keep-alive, wakes | head and tail, the start position, 4 KB heartbeats (§1.5, §1.6) | under 70 MB a playback, 4 KB a heartbeat | probes, playbacks, presence, where the policy allows speculative reads |
| Writes | NFO, artwork, subtitles, lyrics (written aside and renamed); imports move, copy or hard link; the recycle bin | small files; an import may copy a whole file | refreshes, downloads, imports |

### 2.3 Policy by Storage Class

`library.PolicyOf` turns a class into a `StoragePolicy`, consulted in one place by every reader:

| Reader | Local and network | Cloud |
| :--- | :--- | :--- |
| Whole-file reads: keyframes and trickplay of files without an index, loudness | yes | no; HLS of files without keyframes transcodes the video, and they have no trickplay |
| Samples: black borders, chapter images | yes | no; the borders stay unknown |
| Speculative reads: head and tail prefetch before probes, playback start prefetch, keep-alive and wakes | yes | no; storage is read only as playback asks |
| Pruning unchanged folders in scans | yes | no; folder modification times of cloud drives do not tell changes below them |
| Folders a scan lists at once | by device (§1.3) | 2 |
| Embedded text subtitles the client does not demux | extracted once, kept in the metadata folder | streamed by the HLS transcode; otherwise burned in |

The policy is checked both when the jobs are queued and when they run, so a class changed in between takes effect at once.

### 2.4 Keyframes from Container Indexes

HLS cuts copied video at its keyframes, so every video file needs their times. Containers that support seeking already list them, and Mavio reads that list instead of every packet (`keyframes.FromIndex`), with the probe of the file:

* **Matroska and WebM**: the SeekHead, and a second one it points to, locates Info (the timestamp scale and duration), Tracks (the first video track) and Cues; the cue points of the video track are its keyframes.
* **MP4 and QuickTime**: the first video track (`hdlr` `vide`) of the `moov`, found by walking the top-level box headers wherever it is in the file. Its keyframes are the sync samples (`stss`; every sample without it) at their decode times (`stts`) plus composition offsets (`ctts`), shifted by the edit list (`elst`). A fragmented file's sample tables are empty: the track's `tfra` in the `mfra` at the end of the file, found through `mfro`, or else a `sidx` before the first fragment whose subsegments start with a keyframe, gives them.
* Times are rounded to the microsecond, the precision of ffprobe's packet times, so that both give the same segments.

The reads are a few elements or boxes: on the test media, under 2% of each file. Only files without an index are read whole, by the `media.keyframes` job, and not on cloud storage. A playback copying the video of a file without keyframes transcodes it, as before.

### 2.5 Trickplay from Keyframes

A trickplay thumbnail needs one picture near its time, not every frame before it:

* **Sampled through the index** (Matroska and WebM): for each thumbnail time, the cue at or before it gives the cluster of its keyframe, and the cue's relative position the block within it (without one, the cluster's block headers are read until the video track's keyframe). Mavio writes a Matroska stream of the EBML header, Info, the video track's entry and a cluster per thumbnail holding that keyframe at the thumbnail's time (`Matroska.WriteSamples`), and pipes it to a single ffmpeg run that scales and tiles the pictures. It reads a keyframe a thumbnail, the same one once for neighboring times, and decodes nothing else.
* **Keyframes only** (other files, where whole-file reads are allowed): ffmpeg decodes the keyframes alone (`-skip_frame nokey`). It still reads the file but does a fraction of the decoding.
* **Every frame**: the fallback when the keyframes-only run fails.
* In every mode a thumbnail is the first picture at or after its time (`fps=…:round=down:eof_action=pass`), so the last thumbnail is kept when the keyframes end before the next time. ffmpeg runs on one thread and at a low priority (nice 10, or the below normal class on Windows), leaving the CPU to playbacks.

### 2.6 Embedded Subtitles

A client that does not demux a file's subtitles itself gets them from the server, as a file or as an HLS rendition. How the server reads them depends on the storage class:

* **Local and network storage**: the first playback that needs one extracts every embedded text stream of the file in a single ffmpeg run, which reads the file once (ASS keeps its styling, other text becomes SRT), into the item's metadata folder (`library.SubtitlesDir`), stamped with the file's size and modification time. Later playbacks of any stream read it from there until the file changes; playbacks asking at once wait for the one run.
* **Cloud storage, and sources read over HTTP**: an embedded stream is never extracted (`decision.Source.StreamedSubtitles`). Native clients demux it from the file they play. Clients that take subtitles as files get them as an HLS rendition instead: the playback becomes an HLS remux or transcode, and the ffmpeg run producing the video also writes each embedded text stream as WebVTT (`planner.HLSOutput.Subtitles`). A subtitle segment is served once the run has read past it (`Stream.Subtitle`); a seek restarts the run, and with it the subtitles, while what earlier runs read is kept. Clients that take neither get them burned in.
* **Sidecar files** beside the video are small and read as they are, on every storage class.

### 2.7 Implementation Index

| Mechanism | Implementation | Phase |
| :--- | :--- | :--- |
| Storage classes of library paths | `libs/core/library.go` (`StorageClass`, `Library.StorageClasses`, `Library.StorageClassOf`); `libs/store/libraries.go`; `storage_classes` and `effective_storage_classes` in `libs/proto/mavio/library/v1/library.proto`; `apps/server/internal/rpc/library.go` | P24 |
| Classification | `libs/library/storage/class.go` (`DeviceInfo.Class`, `Classifier`, `PathClasses`); `Detector.Folder` in `device.go` | P24 |
| FUSE and loopback detection | `libs/library/storage/fuse.go` (`fuseKind`, `loopbackSource`, `findMount`, `mountKind`); used by `device_{linux,darwin,windows}.go` | P24 |
| Storage policy | `libs/library/policy.go` (`StoragePolicy`, `PolicyOf`); used by `scan.go` (`scan.policy`, `Scanner.walkers`, `scan.state`), `jobs.go` (`Jobs.probeFile`), `keyframes.go`, `borders.go`, `mediaextras.go` (`Jobs.extrasJobs`, `trickplay`, `chapterImages`, `loudness`) | P24 |
| Keyframes from container indexes | `libs/media/keyframes/index.go` (`FromIndex`, `FromFile`, `ErrNoIndex`), `matroska.go`, `mp4.go`; `apps/server/internal/providers/keyframes.go`; `libs/library/jobs.go` (`Jobs.probe`), `keyframes.go` (`KeyframeExtractor`) | P24 |
| Trickplay from keyframes | `libs/media/keyframes/sample.go` (`Matroska.WriteSamples`), `matroska.go` (`ReadMatroska`); `libs/media/thumbnails/thumbnails.go` (`Mode`, `TrickplayArgs`, `Maker.Trickplay`), `priority_{unix,windows,other}.go`; `libs/library/mediaextras.go` (`Thumbnailer`, `Jobs.trickplay`) | P24 |
| Embedded subtitles | `apps/server/internal/playback/subcache.go` (`Manager.extracted`, `extractAll`), `subtitles.go` (`Playback.liveSubtitles`, `liveSubtitle`, `Manager.loadSubtitle`), `manager.go` (`Manager.Start`); `libs/library/mediaextras.go` (`SubtitlesDir`); `libs/media/decision/source.go` (`Source.StreamedSubtitles`), `subtitles.go`; `libs/media/planner/command.go` (`HLSOutput.Subtitles`); `libs/streaming/subtitles.go` (`Stream.Subtitle`) | P24 |
| No speculative reads of cloud storage | `libs/library/storage/keeper.go` (`Keeper.Class`); `apps/server/internal/playback/resume.go` (`Manager.prepareStart`); wired in `apps/server/internal/server/server.go` | P24 |
| Storage port | `libs/core/storage.go` (`Storage`, `StorageCapabilities`, `StorageTransfers`, `StorageFS`, `StorageName`, `StoragePath`); `libs/library/localstorage.go` (`Storages`, `OpenLocalStorage`, `RootOf`), `mediafile.go` (`MediaFile`); `libs/media/planner/inputs.go` (`InputURL`); `apps/server/internal/acquisition/files.go` (`Runtime.transfer`) | P25 |

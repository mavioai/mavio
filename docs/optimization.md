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

---

## 3. Reading Cloud Drives through rclone

Status: designed, not yet implemented (P25). Libraries on rclone remotes (§9.2 of the [Architecture](architecture.md)) read their media through `rclone rcd`; this chapter makes those reads cheap in requests to the drive, steady under seeking and reusable, by caching lookups and download links in the backends of mavioai/rclone, limiting their connections, and caching media blocks in Mavio.

### 3.1 Costs of a Read Today

Every read of a media byte range travels Mavio → `rclone rcd` (`--rc-serve`) → the drive's API → the drive's CDN, and each step costs more than it needs to:

* **Every request repeats the lookup.** `rcserver.serveRemote` calls `NewObject` for each HTTP request. The nine drive backends of the fork find a file by listing its whole parent folder, page by page: cloud115, baidunetdisk, cloud123, guangyadrive, quark and weiyun always list every page; aliyundrive, pikpak and thunder stop at the file. `dircache` caches only folder IDs, and the rcd drops a remote's `Fs`, with its `dircache`, 5 minutes after its last use.
* **Every open asks for a new download link.** cloud115, aliyundrive, baidunetdisk, cloud123, guangyadrive, quark and weiyun call their download-link API on each `Open` and cache nothing, although aliyundrive's links last 4 hours. pikpak and thunder keep the link on the `Object`, which `rcserver` builds afresh for each request. Only thunder fetches a new link when the CDN refuses an expired one.
* **Nothing limits connections.** No backend limits the downloads of one file or one account, while drives refuse or throttle many connections at once.
* **Errors are flattened.** `serve.Object` answers every failed `Open` with 404, so a 403, 429 or rate limit never reaches Mavio, which cannot back off.
* **Mavio reopens on every seek.** `rclone.Storage` names a remote's `Fs` by the library's root (`remote:dir`), so each library of a remote has an `Fs`, and caches, of its own. Its reader sends a new request for each `ReadAt` (the container-index reads of keyframes and trickplay are many small ones) and for each seek; ffmpeg does the same through the media proxy.

So a seek during playback costs a folder listing, a link request and a connection to the CDN, each with the drive's latency; seeks in quick succession, a probe beside a playback, or two viewers of one file add up to the API rates and connection counts at which drives answer 403 or close the account's downloads for a while.

### 3.2 Lookup and Link Caches in the Backends

Two shared packages of the fork, used by the backends that need them:

* **Object cache** (`lib/objcache`): the entries of every listing a backend makes, keyed by folder ID and name, holding what the backend needs to build an `Object` and open it (ID, size, modification time, hash, and the backend's own keys: 115's pick code, weiyun's folder key, 123's `S3KeyFlag`). `NewObject` looks the parent folder up in `dircache` and the file in the object cache; on a miss it lists the folder once (concurrent misses of one folder share the listing) and so caches all its files, which the next requests for its neighbors find. Entries live a few minutes (an option of each backend, 5 minutes by default); uploads, deletions, moves and renames through rclone update them, and folder changes drop the folder's entries. A change made elsewhere (the drive's web page) surfaces as an error of the cached ID, which drops the entry and looks the file up again once. Misses are not cached.
* **Link cache** (`lib/linkcache`): download links keyed by the file's ID and fingerprint (size and hash or modification time), with the headers and cookies the link is bound to and its expiry: the response's expiry field (aliyundrive's `Expiration`), else the URL's expiry parameter (pikpak's `expire`), else a lifetime the backend declares. A link is renewed at four fifths of its life; concurrent opens of one file share a request. When the CDN answers 401, 403, 404 or 410, the link is dropped, a new one fetched and the request retried once, as thunder does today.

| Backend | Object cache | Link cache | Also |
| :--- | :--- | :--- | :--- |
| cloud115 | yes | yes, by pick code; bound to the user agent and cookies | API calls pass their response to `shouldRetry`, so a 429 is seen |
| aliyundrive | yes | yes, 4 hours (`expire_sec`) | — |
| baidunetdisk | yes, and `dircache` (it has none) | yes, the `dlink` and the redirect it resolves | the HEAD and GET of `Open` go through the pacer and its retries |
| cloud123 | yes | yes; the listing's `DownloadURL` where present | — |
| guangyadrive, quark, weiyun | yes | yes | weiyun's download status errors become HTTP errors `shouldRetry` sees |
| pikpak | yes (its links then outlive a request) | its link on the `Object` | — |
| thunder | yes | its link on the `Object`, with an expiry check | — |

The upstream backends Mavio users reach most, Google Drive (filters by name on the server) and OneDrive (looks up by path), already find a file in one request and need neither cache.

Beside the backends:

* **Statuses through `--rc-serve`**: `serve.Object` answers a failed `Open` with the status of the drive's error (403, 429 with its `Retry-After`, 503) instead of 404.
* **One `Fs` per remote**: Mavio names objects from the remote's root (`remote:` and the full path), so every library of a remote shares one `Fs` and its caches, and starts the rcd with `--fs-cache-expire-duration 1h`.

### 3.3 Connection Limits

A limiter of the fork (`lib/connlimit`) caps the downloads a backend has open, per file and per account:

* `Open` takes a slot for the file (its ID) and one for the account before requesting the CDN, and returns the body wrapped so that `Close` frees both. The rcd closes the body when Mavio's request ends, so slots follow connections.
* When the file's slots are full, the connection of that file read least recently is closed and its slot given to the new open: players seeking and browsers requesting a new range open the next connection before closing the last, whose reader has moved on. The reader of the closed connection gets an error and reopens if it still reads.
* When the account's slots are full, the open waits for one, until its context ends; it never takes a slot of another file.
* The limits are options of each backend (`--<backend>-max-file-connections`, `--<backend>-max-connections`). Their defaults are set from measurements of each drive (§3.5); a drive not measured has no limit.

### 3.4 Block Cache

Mavio keeps the bytes it reads from rclone remotes on local disk, in `libs/rclone` beneath both of its read paths: the reader of `rclone.Storage.Open` (direct play, probes, the container-index reads of keyframes and trickplay, sidecars) and the media proxy ffmpeg reads (the proxy serves from the cache instead of forwarding). It follows rclone's VFS cache (`--vfs-cache-mode full`) where that works and departs from it where Mavio knows more:

| | rclone VFS cache | Mavio block cache |
| :--- | :--- | :--- |
| Data | a sparse file per object, at its full size | the same |
| Present data | sorted, coalesced byte ranges (`lib/ranges`) | the same |
| Metadata | JSON written at open and close; ranges not saved as data arrives | JSON with the ranges, saved at most every few seconds and at close, after the data it records is written |
| Staleness | fingerprint (size, modification time, hash) checked at open | the remote's ID, size and modification time from `Stat`, checked at open; a size change while reading drops the item |
| Missing range | a downloader whose position is within `--buffer-size` (16 MiB) of it is extended; otherwise a new downloader starts | the same, with the window as an option |
| Downloaders per file | unlimited | at most the file's connection limit (2 by default); beyond it the downloader furthest from every waiting read is stopped |
| Upstream requests | chunks of 128 MiB doubling without bound | chunks of 16 MiB doubling to 256 MiB, so that an abandoned read wastes little |
| Read-ahead | `--vfs-read-ahead`, 0 by default, plus a 16 MiB memory buffer | by intent (below); data goes to the file in 1 MiB writes without a memory buffer |
| Downloader end | idle for 5 s, or 1 MiB of present data in a row | the same |
| Errors | waiters fail after 10 errors in a row | the same, backing off on 403, 429 and 503 by `Retry-After` |
| Disk reads | under the item's lock | without the lock; only the ranges are guarded |
| Eviction | items not in use, least recently used first; open items over quota are discarded whole | items not in use first; then the ranges of open items furthest behind every reader, keeping what lies ahead of them |

Reads by intent, which a file system cache cannot know:

* **Probes**: the head (1 MB) and tail (256 KB) of a file are fetched at once in two requests before ffprobe runs; ffprobe then reads them from the cache.
* **Container indexes**: the many small reads of Matroska elements and MP4 boxes are rounded out to 256 KB, so neighboring elements share one request.
* **Seeks**: a playback starting or seeking to a time looks up its keyframe in the source's keyframes and starts a downloader at that keyframe's byte position before ffmpeg or the client asks.
* **HLS**: a request for a segment extends the downloader through the next segments' byte ranges, known from the keyframes' positions when the video is copied, otherwise estimated from the bitrate.
* **Direct play**: the downloader stays ahead of the client's latest range by 30 seconds of the source's bitrate, at most 256 MiB.
* **Background jobs** read without read-ahead.

Readers of one file, such as a probe and a playback or two viewers, share its item, and a range is downloaded once whoever waits for it. The cache's size is a setting (`rclone-cache-size`, 10 GiB by default) with a floor of free disk space (5%); items not read for 7 days are removed; the cache lives in `<cache>/rclone/blocks` and survives restarts. It applies to remotes of the `cloud` storage class (§2.1), the class of rclone paths unless the library sets another.

A remote whose CDN refuses a range that does not continue an open connection can be marked sequential (a setting of the remote): its downloaders then never start at a new position beyond the furthest one open, which instead reads on to it. A long seek on such a remote waits for the bytes before it, so the mode is set only for drives measured to need it (§3.5).

### 3.5 Measurements

Before the limits and modes get defaults, each drive is measured through the rcd with a test account: sequential reads, forward and backward jumps, and concurrent ranges of one file and of several files, counting the requests to the API and the CDN (`--dump headers`) and noting which pattern draws a 403, 429 or throttling, after how many requests and for how long. The results set each backend's connection limits, link lifetimes and whether its remotes are sequential, and are recorded in the fork's `MAVIO_PROVIDERS.md`.

### 3.6 Implementation Index

| Mechanism | Implementation | Phase |
| :--- | :--- | :--- |
| Object cache | mavioai/rclone `lib/objcache`; `NewObject`, `List` and the write operations of `backend/{cloud115,aliyundrive,baidunetdisk,cloud123,guangyadrive,quark,weiyun,pikpak,thunder}` | P25, planned |
| Link cache | mavioai/rclone `lib/linkcache`; `Object.Open` of the backends above | P25, planned |
| Statuses through `--rc-serve` | mavioai/rclone `lib/http/serve/serve.go` (`Object`) | P25, planned |
| One `Fs` per remote, longer `Fs` cache | `libs/rclone/storage.go` (`Storage.fsName`, `Client.objectURL`), `daemon.go` (`Daemon.launch`) | P25, planned |
| Connection limits | mavioai/rclone `lib/connlimit`; `Object.Open` of the backends above | P25, planned |
| Block cache | `libs/rclone` (cache, its downloaders and ranges); `storage.go` (`file`), `proxy.go` (`Proxy.ServeHTTP`); intents from `apps/server/internal/playback` and `libs/library` (`Jobs.probeFile`) | P25, planned |
| Measurements | mavioai/rclone `MAVIO_PROVIDERS.md` | P25, planned |

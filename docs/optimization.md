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
* The index also gives where each keyframe lies in the file: the cluster of a Matroska cue point; the offset of an MP4 sync sample, through its chunk (`stsc`, `stco` or `co64`) and the sizes of the samples before it in the chunk (`stsz`); the fragment a `tfra` entry or `sidx` subsegment points to. Playback fetches ahead with them on rclone remotes (§3.4).

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

## 3. Reading Remote Storage through rclone

Status: partly implemented (P25). Implemented: the object cache with its invalidation by `ChangeNotify` and its version checks, conditional reads in the backends that read by path, statuses, seeking through `fsinfo`, stream limits, the webdav, http and crypt fixes, one `Fs` per remote, and the block cache with its read intents, its read-ahead from keyframes and its eviction within open items. Planned, and marked so below: `FindByListing`, the link cache with the other single-backend fixes, per-backend stream limits, and the measurements ([Roadmap](roadmap.md) P25).

Libraries on rclone remotes (§9.2 of the [Architecture](architecture.md)) read their media through `rclone rcd`. This chapter makes those reads cheap in requests, steady under seeking and reusable, for every backend rclone offers:

* The rcd of mavioai/rclone keeps the objects it finds and the download links it gets, limits the streams it has open and reports the storage's errors.
* The backends that cost more than they must are fixed.
* Mavio caches media blocks.

### 3.1 Costs of a Read before These Changes

Every read of a media byte range travels Mavio → `rclone rcd` (`--rc-serve`) → the backend → the storage, and each step can cost more than it needs to.

**Every request repeats the lookup.** `rcserver.serveRemote` calls `NewObject` for each HTTP request and keeps nothing between requests. `dircache` caches only folder IDs, and the rcd drops a remote's `Fs`, with its `dircache`, 5 minutes after its last use. A lookup costs:

| Lookup | Backends |
| :--- | :--- |
| No request | mega (it keeps the node tree in memory) |
| One request: by path, by ID or by a name filter on the server | s3, b2, azureblob, azurefiles, googlecloudstorage, swift, oracleobjectstorage, qingstor, storj, sia, netstorage, filescom, webdav, sftp, smb, hdfs, http, ftp with `MLST`, seafile, drive, onedrive, dropbox, jottacloud, koofr, yandex, mailru, putio, sharefile, hidrive, filefabric, shade, pixeldrain, gofile, ulozto |
| Two requests | box, opendrive, quatrix, internxt, imagekit, cloudinary |
| A listing of the whole parent folder | cloud115, baidunetdisk, cloud123, guangyadrive, quark, weiyun, pcloud, iclouddrive, fichier, filelu (and a second request), protondrive (decrypting every name), chunker (over its base remote), ftp without `MLST` |
| A listing of the parent folder until the file | aliyundrive, pikpak, thunder, premiumizeme, sugarsync, zoho (at most 19 listings of one folder a minute), huaweidrive, drime, linkbox |
| More | filen (lists and decrypts every folder of the path), internetarchive (the item's whole metadata), googlephotos without an ID in the name (the whole album or month), union (every upstream), compress (its metadata object, read whole, then the data object) |

**Many opens ask for a download link first.** `rcserver` builds a new `Object` for each request, so a link kept on the `Object` lives for one request:

| Link | Backends |
| :--- | :--- |
| Requested on every `Open` and kept nowhere | cloud115, aliyundrive (valid 4 hours), baidunetdisk, cloud123, guangyadrive, quark, weiyun, yandex, putio, sharefile, quatrix, ulozto, filelu, iclouddrive (although its listing carries one), seafile, filescom, mega, protondrive (the link, keys and active revision), internxt (the file's shard index) |
| A single-use token on every `Open` | fichier |
| A redirect followed on every `Open` | onedrive, box, shade, internetarchive |
| Kept on the `Object` | pikpak, thunder, pcloud (until 30 s before its expiry), and the listing's link for premiumizeme, gofile, drime, linkbox and googlephotos, which is never renewed |

Only thunder fetches a new link when the storage refuses an expired one.

**Some seeks cost more than one ranged request:**

* crypt reads the file's 32-byte header for its nonce, then reopens at the block. That is two requests per seek.
* mega, protondrive and filen read whole chunks or blocks and discard what precedes the offset. protondrive reads from the start of the file when it lacks the block sizes.
* filelu sends no range and discards everything before the offset. So do seafile's encrypted libraries and the zip members of archive.
* webdav and http pass the range on without checking for a 206. A server that ignores the range returns the file from its start, and `serve.Object` sends those bytes as the requested range.
* shade's first request carries no range.

**Nothing limits the open streams.**

* `--max-connections` holds a pacer token only until a response's headers arrive, and the HTTP transport sets no `MaxConnsPerHost`.
* ftp and smb hold a pooled connection for each open stream, and their pools are unlimited by default.
* Storage refuses or throttles many concurrent streams:
  * Drives close an account's downloads for a while.
  * ftp servers cap the logins of a user.
  * mailru gives a download server at most 4 streams.

**Errors are flattened.**

* `serve.Object` answers every failed `Open` with 404, and a failed lookup gets a 500. So a 403, 429, quota or rate limit never reaches Mavio, which cannot back off.
* Inside the rcd, only onedrive, dropbox, b2, swift, zoho, putio, internxt, protondrive and seafile wait as long as the storage's `Retry-After` asks. The other backends back off by their pacer alone, and koofr, qingstor and http do not retry.
* Drive's daily download quota (`downloadQuotaExceeded`) and mega's transfer quota are retried like rate limits.

**Mavio reopens on every seek.**

* `rclone.Storage` names a remote's `Fs` by the library's root (`remote:dir`), so each library of a remote has an `Fs`, and caches, of its own.
* Its reader sends a new request for each `ReadAt` and each seek. The container-index reads of keyframes and trickplay are many small `ReadAt` calls.
* ffmpeg does the same through the media proxy.

So on the backends that list folders and request links, a seek during playback costs a folder listing, a link request and a new stream, each with the storage's latency. On every backend, seeks in quick succession, a probe beside a playback, or two viewers of one file add up to API rates and stream counts at which the storage answers 403 or 429, or closes the account's downloads for a while.

### 3.2 Objects and Links in the rcd

The fork changes the rcd and the backends as follows. Where a change can be made once for every backend, it is.

**Object cache** (`fs/objcache`): a cache in the rcd process, keyed by the `Fs`'s name and the object's path.

* **Filling it:**
  * `serveRemote` looks an object up here before calling `NewObject`, and caches what it finds.
  * `operations/list` and `operations/stat`, which serve Mavio's scans and `Stat`, always ask the backend and never read the cache. `stat` caches the object it returns and drops the entry of one not found. `list` replaces the entries of the objects it returns without adding any, and a complete listing of a folder (neither recursive nor filtered) drops the entries of the folder's objects missing from it. Only `--rc-serve` reads the cache, so scans see changes as soon as the storage does, and Mavio's `Stat` reports the storage's current version.
  * *Planned:* backends that find a file by listing its folder declare it with a feature of the fork, `FindByListing`. The backends are those of the last three rows of the lookup table, except union and compress. For them, a miss lists the parent folder once and caches all its objects, so the next requests for its neighbors hit; concurrent misses of one folder share the listing.
* **What lives on with the cached `Object`:**
  * the links of pikpak, thunder and pcloud;
  * the links that premiumizeme, gofile, drime, linkbox and googlephotos take from their listings;
  * the metadata of s3, swift and webdav;
  * the metadata of compress;
  * the nonce of crypt (below).
  * union's choice of upstream.
* **Lifetime and invalidation:**
  * Entries live 5 minutes, and the cache holds 10,000 of them, by default (the options `--rc-serve-cache-duration` and `--rc-serve-cache-size`).
  * Writes through the rcd (`operations/*`, `sync/*`, `backend/command`) drop the entries of the paths and folders they touch, in every `Fs` of the remote whatever its root.
  * While objects of a remote whose backend has `ChangeNotify` (drive, onedrive, dropbox, box, pcloud, mega and others) are in the cache, the rcd polls its changes, every minute by default (`--rc-serve-poll-interval`), and the changes drop the entries of the paths and folders they name. The polling stops when none of the remote's objects are left.
  * A change made elsewhere and not yet reported surfaces as a version that does not match (below) or as a failed `Open`. The entry is then replaced by the object looked up again. The open is retried once when the version is the same; when it changed, a request with `If-Match` gets 412, and one without is served the fresh object from the start.
  * Misses are not cached.
* **Version checks:** a read never mixes two versions of a file.
  * `--rc-serve` gives every response the object's version as its ETag: a hash of the object's ID and its fingerprint (size and modification time, the hash where the backend has one at no cost, and the storage's own ETag where the backend keeps one).
  * Mavio's readers keep the ETag of a file's first response and send it as `If-Match` with every later request. The block cache keeps it with the file's item, across restarts.
  * When a cached object does not match `If-Match`, the rcd looks the object up again. When the fresh object does not match either, it answers 412. Mavio takes that, or a response whose total size differs from the size `Stat` gave, as a changed file: the read fails with `ErrChanged`, and the file's block-cache item is dropped.
  * Mavio's `Stat` asks the backend and so refreshes the cached object. A cached object is older than the storage only between a change made elsewhere and the next `Stat` or listing of its folder, and at most for an entry's lifetime.
  * Within that window, a backend that reads by path would serve new content under the old version. s3, webdav, azureblob, swift, oracleobjectstorage and qingstor therefore read with `If-Match` set to the storage's ETag of the object they found, so that a file replaced since, even in the middle of a read, fails with 412 (`--s3-no-read-if-match` turns this off for S3-compatible storage that answers it wrongly; webdav sends only strong ETags). googlecloudstorage reads the generation it found, and b2 reads by file ID, so both read one version already.
  * A file rewritten in place while keeping its ID, size and modification time is detected only where the hash is part of the fingerprint.

**Link cache** (`lib/linkcache`, planned): used inside `Open` by every backend that requests a link.

* Links are keyed by the object's ID and fingerprint (size, and hash or modification time). Each entry keeps the headers and cookies the link is bound to.
* A link's expiry is taken from the response's expiry field (aliyundrive's `Expiration`), else from the URL's expiry parameter, else from a lifetime the backend declares. When the storage states no expiry, the declared lifetime comes from measurements (§3.5).
* A link is renewed at four fifths of its life. Concurrent opens of one file share a request.
* When the storage answers a link with 401, 403, 404 or 410, the link is dropped, a new one fetched and the request retried once, as thunder does today.
* The backends that follow a redirect cache the redirect's target the same way.

| Backend | What the link cache keeps | Also |
| :--- | :--- | :--- |
| cloud115 | the link, by pick code; bound to the user agent and cookies | API calls pass their response to `shouldRetry`, so a 429 is seen |
| aliyundrive | the link, 4 hours (`expire_sec`) | — |
| baidunetdisk | the `dlink` and the redirect it resolves | `dircache` (it has none); the HEAD and GET of `Open` go through the pacer and its retries |
| cloud123 | the link; the listing's `DownloadURL` where present | — |
| guangyadrive, quark, weiyun | the link | weiyun's download status errors become HTTP errors `shouldRetry` sees |
| yandex, putio, sharefile, quatrix, ulozto, seafile | the link | — |
| filelu | the link | `Open` sends a range instead of discarding, and its errors go through `shouldRetry` |
| iclouddrive | the listing's `downloadURL`, else the link | — |
| filescom | the `DownloadUri`, passed to the SDK | — |
| mega | the URL of the `g` command | — |
| protondrive | the link, the session key and the active revision | — |
| internxt | the file's bucket information | — |
| onedrive, box, shade, internetarchive | the redirect's target | shade asks for the range on its first request |
| pikpak, pcloud | — (the link on the cached `Object`) | — |
| thunder | — (the link on the cached `Object`) | an expiry check |
| fichier | — (its tokens are single-use) | — |

**Fixes to single backends** (filelu and the "Also" column of the table above are planned):

* webdav and http: when a range is asked for and the server answers 200, `Open` discards the bytes before the offset, as mailru does, so the body matches the range.
* crypt: keeps the nonce on its `Object` after the first read, so a seek costs one request.

**Through `--rc-serve` and `operations/fsinfo`:**

* **Statuses:** lookups and opens answer with the status of the storage's error, which the rcd finds in the error's chain, not 404 and 500:
  * 404 when the object is not found;
  * 412 when the object's version does not match the `If-Match` Mavio sent;
  * 403;
  * 429 and 503, with a `Retry-After` where the storage says when to retry (`fserrors.ErrorRetryAfter`);
  * a quota or bandwidth limit (drive's `downloadQuotaExceeded`, mega's transfer quota, 509) as 429;
  * any other error as 502.
* **Seeking:** `operations/fsinfo` reports remotes that read from the start to reach an offset, with the fork's feature `SeekReadsFromStart`: seafile's encrypted libraries, and archive's zip members. Mavio reads those remotes sequentially (§3.4).
* **One `Fs` per remote:** Mavio names objects from the remote's root (`remote:` and the full path), so every library of a remote shares one `Fs` and its caches. Mavio starts the rcd with `--fs-cache-expire-duration 1h`.

### 3.3 Connection Limits

The fork's limiter (`lib/connlimit`) caps the streams `--rc-serve` has open, for every backend.

* **Keys:** a file's streams are keyed by its object, and an account's by the remote that serves the bytes: the innermost remote that the wrappers' `UnWrap` reveal. So a crypt remote over a drive counts against the drive's account.
* **Taking slots:** `serve.Object` takes a slot for the file and one for the account before opening the object. It frees both when the request's body is closed, so slots follow streams.
* **When the file's slots are full:** the stream of that file read least recently is closed and its slot given to the new open. Players seeking and browsers requesting a new range open the next stream before closing the last, whose reader has moved on. The reader of the closed stream gets an error, and reopens if it still reads.
* **When the account's slots are full:** the open waits for a slot until its context ends. It never takes a slot of another file.
* **Limits:**
  * The options `--rc-serve-max-file-streams` and `--rc-serve-max-streams` set them. Unset, streams are not limited.
  * *Planned:* each backend declares default limits with features of the fork (`MaxFileStreams`, `MaxStreams`), set from measurements (§3.5) or from the storage's documented limits, such as mailru's 4 streams to a download server. Limits that a remote's own options already set win: ftp's `--ftp-concurrency` and sftp's `--sftp-connections`. A backend neither measured nor documented keeps no limit.

### 3.4 Block Cache

Mavio keeps the bytes it reads from rclone remotes on local disk, in `libs/rclone`. The cache sits beneath both of Mavio's read paths:

* The reader of `rclone.Storage.Open`, used for direct play, the container-index reads of keyframes and trickplay, and sidecars.
* The media proxy ffmpeg and ffprobe read. The proxy serves from the cache instead of forwarding.

The cache follows rclone's VFS cache (`--vfs-cache-mode full`) where that works, and departs from it where Mavio knows more:

| | rclone VFS cache | Mavio block cache |
| :--- | :--- | :--- |
| Data | a sparse file per object, at its full size | the same |
| Present data | sorted, coalesced byte ranges (`lib/ranges`) | the same |
| Metadata | JSON written at open and close; ranges not saved as data arrives | JSON with the ranges and the ETag, saved every 5 seconds while data arrives and at close, after syncing the data it records |
| Staleness | fingerprint (size, modification time, hash) checked at open | items are keyed by the ID, size and modification time `Stat` gives; the ETag of the first response is kept with the item and sent as `If-Match` (§3.2); a 412 or a size change while reading drops the item |
| Missing range | a downloader whose position is within `--buffer-size` (16 MiB) of it is extended; otherwise a new downloader starts | the same |
| Downloaders per file | unlimited | at most 2; beyond them, the downloader furthest from every waiting read is stopped |
| Upstream requests | chunks of 128 MiB doubling without bound | chunks of 16 MiB doubling to 256 MiB, never past the end of the read-ahead (rounded up to 1 MiB), so that an abandoned read wastes little |
| Read-ahead | `--vfs-read-ahead`, 0 by default, plus a 16 MiB memory buffer | by intent (below); data goes to the file in 1 MiB writes without a memory buffer |
| Downloader end | idle for 5 s, or 1 MiB of present data in a row | the same |
| Errors | waiters fail after 10 errors in a row | the same, waiting between requests as long as `Retry-After` asks, else from 200 ms doubling to 5 s |
| Disk reads | under the item's lock | without the lock; only the ranges are guarded |
| Eviction | items not in use, least recently used first; open items over quota are discarded whole | items not in use, least recently read first; then the ranges of open items furthest behind all their readers, in 16 MiB pieces, keeping each item's first MiB and the 16 MiB behind its readers; where the disk frees no ranges of files, open items are kept whole |

Read-ahead follows the intent of each read, which a file system cache cannot know. The intent (`core.ReadIntent`) travels in the context of `Storage.Open` and `Storage.MediaURL`:

* **Probes:** when a probe asks for a file's URL, the head (1 MiB) and tail (256 KiB) of the file are fetched at once, in two requests. ffprobe then reads them from the cache.
* **Container indexes:** a read that misses fetches at least 256 KiB, so that the many small reads of Matroska elements and MP4 boxes share requests.
* **Playback:** direct play, and ffmpeg's reads for a playback, stay ahead by 30 seconds of the source's bitrate, at most 256 MiB, or 32 MiB when the bitrate is unknown.
* **Background jobs:** they read without read-ahead.
* **Other reads:** 16 MiB.
* **Playback start:** when a playback starts, Mavio opens its source and reads where its keyframes lie from the container's index (§2.4); the index reads mostly hit what the probe left in the cache. Before ffmpeg or the client asks, it fetches from the keyframe at or before the start through the first keyframe after 30 seconds of media, at most 256 MiB. Without an index the range is estimated from the file's size and duration.
* **HLS:** each request for a segment, a seek included, fetches the same way from the segment's start, so that ffmpeg, paused while far enough ahead of the client, finds on resuming the next segments' bytes present.

Readers of one file share its item, such as a probe and a playback, or two viewers. A range is downloaded once, however many readers wait for it.

Each reader of an item marks where it reads: a client's request for direct play, ffmpeg's request through the proxy, and the file a playback fetches ahead with, which follows the playback's progress reports. When the items not in use do not free enough, eviction drops the ranges of open items behind all their readers, those furthest behind first, and frees their disk space by punching holes in the data (`fallocate` on Linux, `F_PUNCHHOLE` on macOS, sparse files on Windows). An item's metadata is saved without the ranges before their data goes, and ranges being read or written are left. A long film watched through keeps only what lies about its playback.

* **Size:** a setting, `rclone-cache-size`, 10 GiB by default, with a floor of 5% free disk space; 0 turns the cache off.
* **Age:** items not read for 7 days are removed.
* **Location:** `<cache>/rclone/blocks`; the cache survives restarts.
* **Scope:** folders of the `cloud` storage class (§2.1). rclone paths are in that class unless the library sets another, and so are rclone paths outside libraries, such as download folders.

Some remotes are read sequentially:

* those whose rcd reports that they read from the start (§3.2);
* *Planned:* those whose storage refuses a range that does not continue an open stream, as measured (§3.5) and set on the remote.

A file of a sequential remote has one downloader. A read ahead of it extends it, however far; a read behind it restarts it there. A long seek forward on such a remote waits for the bytes before it.

### 3.5 Measurements

Planned. The limits, link lifetimes and sequential settings get their defaults from measurements. Each backend is measured through the rcd with a test account, starting with the drives of the fork and the consumer drives (drive, onedrive, dropbox, box, pcloud, mega, yandex, protondrive). The measurements:

* sequential reads;
* forward and backward jumps;
* concurrent ranges of one file and of several files.

They count the requests to the API and to the storage (`--dump headers`). They note which pattern draws a 403, a 429 or throttling, after how many requests, and for how long.

The results set each backend's stream limits, link lifetimes and whether its remotes are sequential. They are recorded in the fork's `MAVIO_PROVIDERS.md`.

### 3.6 Implementation Index

| Mechanism | Implementation | Phase |
| :--- | :--- | :--- |
| Object cache | mavioai/rclone `fs/objcache`; `fs/rc/rcserver/serveobject.go` (`serveObject`), `fs/operations/lsjson.go` (`list`, `stat`), `fs/operations/rc.go` and `fs/sync/rc.go` (the write operations); the `ChangeNotify` pollers in `fs/objcache` (`Cache.startNotifier`, `Cache.notify`) and `--rc-serve-poll-interval` in `fs/rc/rc.go`; the `FindByListing` feature of the backends that list to find is planned | P25, implemented |
| Version checks | mavioai/rclone `fs/rc/rcserver/serveobject.go` (the ETag, `If-Match` and 412); `libs/rclone/version.go`, `storage.go` (`file`), `proxy.go`, `blocks.go`; `fs.VersionTagger` in `fs/types.go`, and `If-Match` in `Object.Open` of `backend/{s3,webdav,azureblob,swift,oracleobjectstorage,qingstor}` | P25, implemented |
| Link cache | mavioai/rclone `lib/linkcache`; `Object.Open` of the backends in the table of §3.2 | P25, planned |
| Backend fixes | mavioai/rclone `backend/webdav`, `backend/http` (ranges answered with 200, `lib/readers/skiprange.go`), `backend/crypt` (the nonce); `backend/filelu` (ranges) and the "Also" column of §3.2 are planned | P25, implemented |
| Statuses and seeking through the rcd | mavioai/rclone `lib/http/serve/status.go` (`ErrorStatus`), `serve.go` (`ObjectWith`), `fs/rc/rcserver/serveobject.go`, `fs/features.go` (`SeekReadsFromStart`) | P25, implemented |
| One `Fs` per remote, longer `Fs` cache | `libs/rclone/storage.go` (`Storage.fsName`, `Storage.abs`), `daemon.go` (`Daemon.launch`) | P25, implemented |
| Connection limits | mavioai/rclone `lib/connlimit`, `fs/rc/rcserver/serveobject.go` (`Server.open`), `fs/rc/rc.go` (the options); the `MaxFileStreams` and `MaxStreams` features are planned | P25, implemented |
| Block cache | `libs/rclone` (`blocks.go`, `ranges.go`, `cached.go`), `storage.go` (`Storage.Open`, `Storage.MediaURL`), `proxy.go` (`Proxy.serveCached`); `core.ReadIntent` from `libs/library` (probes, background jobs), `apps/server/internal/playback` (playback) and `apps/server/internal/acquisition` (probes of imports); `apps/server/internal/server/rclone.go` and the `rclone-cache-size` setting; eviction within open items in `blocks.go` (`BlockCache.trimOpen`, `blockItem.evict`) and `punch_{linux,darwin,windows,other}.go`; read-ahead from keyframes through `core.Prefetcher` (`cachedFile.Prefetch`), `libs/media/keyframes` (`Data.Offsets`) and `apps/server/internal/playback/prefetch.go` (`sourcePrefetch`; `Manager.prepareStart`, `serveSegment`, `Manager.Progress`) | P25, implemented |
| Measurements | mavioai/rclone `MAVIO_PROVIDERS.md` | P25, planned |

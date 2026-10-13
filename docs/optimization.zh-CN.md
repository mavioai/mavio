# 优化规范

> [English](optimization.md) | 简体中文

> 相关文档：[系统架构](architecture.zh-CN.md) · [领域模型](domain.zh-CN.md) · [开发指南](development.zh-CN.md) · [测试策略](testing.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

> 本文档汇集 Mavio 的优化主题，每个主题一章。每章说明问题、采用的机制，以及一张落地对照索引：每个机制对应的实现代码、交付它的阶段、与 Jellyfin 的差异，以免日后与 Jellyfin 对齐时把有意的差异误当作缺漏。

---

## 1. 存储 I/O 延迟缓解与推测式预热（Storage I/O Latency Mitigation and Speculative Warming）

### 1.1 目标与挑战

Mavio 部署在家用服务器、NAS（Synology、TrueNAS、Unraid）和低功耗硬件上，存储高度异构：

1. **机械硬盘会抖动**：一次寻道 10–15 ms；对同一块盘的并发随机读会让磁头来回移动，吞吐从约 150 MB/s 跌到个位数 MB/s。
2. **硬盘与共享会休眠**：外置 USB 硬盘和 NAS 硬盘闲置数分钟后停转，SMB/NFS 共享会断开；重新起转需要 5–10 秒，足以让客户端放弃起播。
3. **后台 I/O 与播放争抢**：扫描、探测和缩略图提取占满磁盘队列时，播放会卡顿。
4. **仍在写入的文件**：下载客户端或自动化工具（qBittorrent、Aria2、Sonarr、Radarr）仍在写的文件，过早扫描会让 ffprobe 失败或记录错误的时长。
5. **续播要多次寻道**：打开文件、读索引、跳到续播位置、再从前一个关键帧解码，在慢速存储上要往返好几次。

采用的原则：

* **认识设备**：对同一块机械硬盘或远程卷的后台读取串行化；固态硬盘保持完全并发。
* **播放优先**：客户端在拉流时，后台读取等待。
* **小而精准的预读**：预读容器头部与索引、起播位置附近的媒体，而非整个文件。
* **在需要之前唤醒，且只在有人时**：保持正在播放的卷不休眠，唤醒在场用户可能播放的媒体所在的卷；否则任由磁盘休眠。
* **各平台原生**：使用各操作系统自己的调用，不用 CGO。

### 1.2 存储识别

`storage.DetectDevice` 把某路径所在的卷归类为本地 SSD、本地机械硬盘、远程共享或云挂载，并返回一个稳定的卷 ID，作为下文所有按卷机制的键。`storage.Detector` 对每个文件夹只检测一次（最多记住 4,096 个文件夹）；无法检测的路径为未知，按固态硬盘对待。

| 平台 | 卷 ID | 机械硬盘 | 远程共享 | 云挂载 |
| :--- | :--- | :--- | :--- | :--- |
| Linux | `st_dev` | `/sys/dev/block/<major>:<minor>/queue/rotational` 为 `1`；叠加在本地磁盘上的 FUSE 文件系统（`fuse.mergerfs`、`fuseblk`、`fuse.gocryptfs` 等） | `statfs.f_type` 为 SMB（`0x517B`）、SMB2（`0xFE534D42`）、CIFS、NFS（`0x6969`）且由其他主机提供；`fuse.sshfs` 等走网络协议的 FUSE | FUSE（`0x65735546`）按 `/proc/self/mountinfo` 中的子类型：`fuse.rclone`、`fuse.s3fs`、各云盘客户端和未知子类型；由本机提供的 NFS（`127.0.0.1:/`，rclone nfsmount） |
| Windows | 卷序列号 | `IOCTL_STORAGE_QUERY_PROPERTY` 查询 `StorageDeviceSeekPenaltyProperty` | `GetDriveTypeW` 为 `DRIVE_REMOTE`，或 UNC 路径 | 位于 OneDrive 或 iCloud Drive 下的路径；名为 `rclone` 或 `FUSE-<name>` 的 WinFsp 卷 |
| macOS | `statfs.f_fsid` | `/Volumes` 下的卷（外置硬盘盒） | `f_fstypename` 为 `smbfs`、`nfs`、`afpfs`、`webdav`，或无 `MNT_LOCAL` | 位于 `~/Library/Mobile Documents` 或 `~/Library/CloudStorage` 下的路径；macFUSE 挂载；由本机提供的 NFS、SMB 和 WebDAV（rclone nfsmount、FUSE-T） |
| 其他 | `filepath.VolumeName` | 所有卷 | — | — |

由设备派生两个属性：

* **串行化**（`DeviceInfo.Serialized`）：机械硬盘、远程共享和云挂载；它们的后台读取一次只进行一个。
* **会休眠**（`DeviceInfo.Sleeps`）：机械硬盘和远程共享；对它们保活和唤醒。云挂载，以及所属媒体库将其归为云存储的路径（§2.1），从不做推测式读取，因为读取占位文件会触发下载。

### 1.3 按卷串行化

* **`storage.VolumeLedger`**：每个卷 ID 一个槽位，按先来先得发放；`Acquire` 等待槽位或上下文结束，返回释放函数。没人持有或等待时，该卷的通道即被丢弃。
* **扫描**：遍历器在固态硬盘上用 4 个 goroutine（媒体库的并发度）读取文件夹，在串行化的卷上只用 1 个；每次列文件夹都持有该卷的槽位。
* **探测**：探测串行化卷上的文件时持有该卷的槽位；同一条目在同一块盘上的多个版本依次探测，探测不会等待自己。
* 结果是：跨所有媒体库的所有扫描和探测，一块机械硬盘同一时刻只服务一个顺序读取者。

### 1.4 后台 I/O 静默闸门

* **`storage.QuietGate`**：前台活动把静默截止时间延长到“当前时间加静默期”（默认 3 秒），且从不缩短，因此一串请求只形成一个静默尾巴，无需每个请求一个计时器。
* **活动**：开始播放和每个媒体请求（HLS 播放列表与分段、直接播放的范围读取）都记一次活动。持续拉流会在读取时让后台读取暂停，在其间隙让它们运行；播放不会在整个播放期间占住闸门，否则只要有人在看，扫描就会一直停止。
* **等待者**：扫描遍历器在每个文件夹之前，以及探测、黑边检测、trickplay、章节图片和响度任务在读取文件之前，都会等过静默截止时间（`PauseOrCancel`），并响应取消。

### 1.5 精准预读

* **头部与尾部**：`storage.PrefetchHeadTail` 预读文件的前 1 MB（MP4 `ftyp`/`moov`、MKV EBML 头、MPEG-TS 头）和最后 256 KB（位于末尾的 MP4 `moov`、MKV `Cues`、AVI `idx1`）。每次探测和每次播放之前都会执行。
* **范围**：`storage.PrefetchRange` 预读裁剪到文件内的任意范围。从非零位置开始的播放会预读起播位置附近的媒体，按媒体源的大小和时长估算：之前 1 MB、之后 3 秒，最多 64 MB。
* **机制**：Linux 上用 `posix_fadvise(POSIX_FADV_WILLNEED)` 让内核异步把这些范围载入页缓存，无需应用缓冲区；其他平台按 128 KB 分块读取后丢弃，以预热系统缓存。

### 1.6 保活与唤醒卷

* **心跳读**（`storage.VolumeHeartbeat`）：在文件的一个随机 4 KB 对齐偏移处读 4 KB，并绕过缓存，使读取真正到达磁盘或共享：Linux 用 `O_DIRECT`（文件系统拒绝时退回 `POSIX_FADV_DONTNEED` 加普通读取），macOS 用 `F_NOCACHE`，Windows 用 `FILE_FLAG_NO_BUFFERING | FILE_FLAG_OPEN_NO_RECALL`。
* **`storage.Keeper`** 只读取会休眠的卷：
  * `Beat` 每个心跳间隔（2.5 秒）最多读一次每个卷，无论有多少路径指向它；上次读取尚未返回的卷（例如挂起的共享）会被跳过。
  * `Wake` 每个唤醒冷却期（30 秒）最多读一次每个卷，随后预读该文件的头部和尾部。
* 服务端的**三个触发点**：
  1. **播放保活**（`playback.Manager`）：每个心跳间隔，对所有进行中播放所在的卷做心跳，使暂停后的播放恢复时无需等待起转。
  2. **在场预热**（`apps/server/internal/warming`）：每 30 秒，对每个正在使用客户端的用户（开着事件流、且在最近 5 分钟内有请求），查出其最近播放的 8 个可续播条目（“继续观看”）的媒体；每个心跳间隔对这些卷做心跳。没有用户在场时不读取，磁盘可以休眠。
  3. **打开条目**（`ItemService.GetItem`）：唤醒客户端打开的条目的媒体源，使客户端请求播放时卷已在转动。

### 1.7 仍在写入的文件（增长策略）

* **`storage.GrowthPolicy`** 在扫描时判断文件是否仍在增长：
  * 文件名带下载后缀（`.part`、`.crdownload`、`.download`、`.aria2`、`.tmp`、`.incomplete`、`.!ut`、`.!qb`）；
  * 在写入窗口（10 秒）内被修改过；
  * 或此前被推迟过，且其大小和修改时间尚未稳定满 30 秒。
* **推迟**：增长中的文件被排除在扫描之外，不报错、不探测，并为其媒体库推迟（最多 4,096 个文件；永不稳定的文件，例如暂停的下载，一天后遗忘）。它所在的文件夹不会被标记为未变化，因此即使文件夹修改时间没变，之后的扫描也会重新列出它。
* **协调**（`library.Scanner.RunDeferred`，每 10 秒）：对被推迟的文件做 stat；一旦文件消失（下载完成后被重命名），或在写入窗口之后保持不变满 30 秒，就为其媒体库排入一次扫描，每个媒体库一次。
* 没有策略时，扫描器仍会跳过带下载后缀的文件。

### 1.8 续播寻道折叠与关键帧吸附

* **关键帧吸附**（`playback.Playback.startPosition`）：请求从某位置开始的播放，若该位置之前 5 秒内有关键帧，就从该关键帧开始，解码器可立即出画，无需解码再丢弃帧：
  * HLS 取包含该位置的分段起点，编码器或源会以关键帧开始该分段；
  * 直接播放取源在该位置之前的关键帧（由 `media.keyframes` 任务存储）。

  实际采用的位置通过 `StartPlaybackResponse.start_position` 返回，客户端跳到该位置，播放器与服务端保持一致。
* **寻道折叠**（`streaming.Stream.Prepare`）：开始 HLS 播放时立即让 ffmpeg 从包含起播位置的分段开始，而不是等第一个分段请求；该分段的第一个请求会发现 ffmpeg 已在正确的位置读取，无需重启。
* **预读**（§1.5）：播放开始时在后台预读源的头部、尾部以及起播位置附近的范围。

### 1.9 落地对照索引

| 机制 | 实现 | 阶段 | 与 Jellyfin 的关系 |
| :--- | :--- | :--- | :--- |
| 设备与协议识别 | `libs/library/storage/device.go`、`device_{linux,darwin,windows,other}.go`（`DetectDevice`、`Detector`、`DeviceInfo.Serialized`、`DeviceInfo.Sleeps`） | P9 | Mavio 独有；Jellyfin 不区分存储 |
| 按卷串行化 | `libs/library/storage/anti_thrashing.go`（`VolumeLedger`）；由 `libs/library/scan.go`（`Scanner.walkers`、`scan.folder`）和 `libs/library/jobs.go`（`Jobs.probeFile`）使用 | P9 | Mavio 独有；Jellyfin 无论设备都以固定并行度扫描和探测 |
| 静默闸门 | `libs/library/storage/quiet_gate.go`（`QuietGate`）；活动来自 `apps/server/internal/playback/manager.go`（`Manager.Start`）和 `playback/http.go`；由 `libs/library/scan.go`、`jobs.go`、`borders.go`、`mediaextras.go` 等待 | P9 | Mavio 独有；Jellyfin 的计划任务不为播放让路 |
| 头尾预读 | `libs/library/storage/prefetch.go`、`prefetch_linux.go`、`prefetch_other.go`（`PrefetchHeadTail`）；由 `libs/library/jobs.go`（`Jobs.probeFile`）、`storage/keeper.go`（`Keeper.Wake`）和 `apps/server/internal/playback/resume.go`（`warmResume`）使用 | P9 | Mavio 独有 |
| 起播位置预读 | `libs/library/storage/prefetch.go`（`PrefetchRange`）；`apps/server/internal/playback/resume.go`（`warmResume`） | P9 | Mavio 独有 |
| 绕过缓存的心跳读 | `libs/library/storage/heartbeat.go`、`heartbeat_{linux,darwin,windows,other}.go`（`VolumeHeartbeat`、`Heartbeats`） | P9 | Mavio 独有 |
| 保活与唤醒 | `libs/library/storage/keeper.go`（`Keeper.Beat`、`Keeper.Wake`） | P9 | Mavio 独有 |
| 播放保活 | `apps/server/internal/playback/manager.go`（`Manager.Run`）、`playback/resume.go`（`Manager.keepAwake`） | P9 | Mavio 独有 |
| 在场预热 | `apps/server/internal/warming/warming.go`（`Warmer.Run`、`Warmer.Tick`）；在线会话来自 `apps/server/internal/events/hub.go`（`Hub.OnlineSessions`） | P9 | Mavio 独有；只在用户在场时读盘 |
| 打开条目时唤醒 | `apps/server/internal/rpc/item.go`（`GetItem` 中的 `ItemService.Wake`）→ `warming.Warmer.Wake`；在 `apps/server/internal/server/server.go` 中装配 | P9 | Mavio 独有；`GetItem` 带有一次后台读取的副作用 |
| 增长策略 | `libs/library/storage/growth_policy.go`（`GrowthPolicy`、`HasDownloadSuffix`）；由 `libs/library/scan.go`（`scan.list`）使用 | P9 | **有差异**：Jellyfin 在扫描或其文件系统监视器看到文件时就入库（监视器会等变化平息、文件解锁）；Mavio 排除 10 秒内修改过的文件，并推迟到稳定满 30 秒，因此刚复制的文件最多晚约 40 秒出现 |
| 推迟协调 | `libs/library/deferred.go`（`Scanner.RunDeferred`、`Scanner.ReconcileDeferred`）；由 `apps/server/internal/server/server.go` 运行 | P9 | **有差异**：对仍在写入的文件，替代了 Jellyfin 的实时文件系统监视器；Mavio 没有文件系统监视器（全量协调扫描，见[架构 §9](architecture.zh-CN.md)） |
| 关键帧吸附 | `apps/server/internal/playback/resume.go`（`Playback.startPosition`）；`libs/streaming/layout.go`（`Layout.Index`）；`libs/proto/mavio/playback/v1/playback.proto` 中的 `StartPlaybackResponse.start_position` | P9 | **有差异**：Jellyfin 从请求的位置开始；Mavio 可能提前至多 5 秒开始，并返回实际采用的位置 |
| 寻道折叠 | `libs/streaming/stream.go`（`Stream.Prepare`）；由 `apps/server/internal/playback/resume.go`（`Manager.prepareStart`）调用 | P9 | **有差异**：Jellyfin 在第一个分段请求时启动 ffmpeg；Mavio 在播放开始时启动 |

---

## 2. 按存储类别访问存储（Storage Access by Storage Class）

### 2.1 存储类别

读取文件的代价因位置而异：服务器自己的磁盘很便宜；局域网共享较慢但不计量；云盘（用 rclone 或厂商客户端挂载）的每个字节计入下载配额，每个请求计入限流。Mavio 把每个媒体库路径归为 `local`、`network` 或 `cloud`（`core.StorageClass`），由类别决定服务端自行读取多少媒体。

* **分类**（`storage.Classifier`）：优先用媒体库为该路径设定的类别（`Library.StorageClasses`，用于设备检测会判错的存储），否则用所在设备的类别（§1.2）：云挂载为 `cloud`，远程共享为 `network`，其余设备为 `local`。媒体库路径按其自身检测，挂载点按挂载本身而不是上一级目录分类。
* **不知道媒体库的路径**（`storage.PathClasses`）：保活等只知道路径的机制，在媒体库中查找包含该路径的媒体库；媒体库列表每分钟重新读取。
* `LibraryService` 向管理员返回每个路径的实际类别（`effective_storage_classes`），与管理员设定的类别并列。

### 2.2 存储访问清单

Mavio 对媒体存储的全部读写，及其代价和时机：

| 来源 | 操作 | 读取量或请求数 | 时机 |
| :--- | :--- | :--- | :--- |
| 扫描 | 每个目录 list 一次，每个条目 stat；策略允许时，未变化的目录沿用上次扫描 | 主要是 API 调用次数 | 扫描周期、手动、导入后的定向扫描 |
| probe | ffprobe 读容器头部，索引在文件尾时再读文件尾 | 几 MB，2–5 次 Range | 新文件和变化的文件 |
| keyframes | 容器的索引：Matroska Cues，或 MP4 的样本表、`mfra`、`sidx`（§2.4）；没有索引时，ffprobe 读视频流的每个包 | 读几次头部和索引；没有索引时整个文件 | 随 probe 一起；没有索引时在 probe 之后，策略允许整文件读取时 |
| 黑边检测 | 在 5 个点各 seek 一次并解码几帧 | 每点约 3–4 MB | probe 之后，策略允许抽样时 |
| trickplay | Matroska：每张缩略图通过 Cues 读一个关键帧（§2.5）；其他文件：ffmpeg 只解码关键帧，失败时解码每一帧 | 每张缩略图一个关键帧；没有索引时整个文件 | 媒体库开启时；没有索引时，策略允许整文件读取时 |
| 章节图 | 每个章节 seek 一次 | 每章约 3–4 MB | 媒体库开启且策略允许抽样时 |
| 响度 | ffmpeg 读整条音轨 | 整个文件 | 媒体库开启且策略允许整文件读取时 |
| NFO、图片、歌词、外挂字幕 | 小文件 | 小 | 刷新、扫描 |
| OpenSubtitles 哈希 | 文件头尾各 64 KB | 2 次 Range | 搜索字幕时 |
| 播放 | 直接播放的 Range 读取，转封装和转码时 ffmpeg 顺序读 | 与观看量成正比 | 播放 |
| 内嵌字幕 | 本地和网络：ffmpeg 一次复制出所有文字字幕流，存入元数据目录（§2.6）；云：播放视频的转码边读边写出 | 本地和网络：文件不变时整个文件读一次；云：播放之外不再读取 | 客户端不自行解复用内嵌文字字幕的播放 |
| 预读、保活、唤醒 | 头尾、起播位置、4 KB 心跳（§1.5、§1.6） | 每次播放不到 70 MB，每次心跳 4 KB | probe、播放、用户在场时，策略允许推测式读取时 |
| 写入 | NFO、图片、字幕、歌词（先写到旁边再 rename）；导入时 move、copy 或硬链接；回收站 | 小文件；导入可能复制整个文件 | 刷新、下载、导入 |

### 2.3 按存储类别的策略

`library.PolicyOf` 把类别转换为 `StoragePolicy`，所有读取方都在同一处查询：

| 读取方 | 本地和网络 | 云 |
| :--- | :--- | :--- |
| 整文件读取：没有索引的文件的 keyframes 和 trickplay、响度 | 是 | 否；没有关键帧的文件走 HLS 时转码视频，也没有 trickplay |
| 抽样：黑边、章节图 | 是 | 否；黑边保持未知 |
| 推测式读取：probe 前的头尾预读、起播预读、保活和唤醒 | 是 | 否；只按播放的需要读取 |
| 扫描时跳过未变化的目录 | 是 | 否；云盘目录的修改时间反映不了下层的变化 |
| 扫描同时 list 的目录数 | 按设备（§1.3） | 2 |
| 客户端不自行解复用的内嵌文字字幕 | 提取一次，保存在元数据目录 | 由 HLS 转码边读边输出；否则烧录 |

入队和运行时都检查策略，因此中途修改的类别立即生效。

### 2.4 从容器索引读取关键帧

HLS 在关键帧处切分直接复制的视频，因此每个视频文件都需要关键帧时间。支持定位的容器本身就列出了关键帧，Mavio 读这份列表而不是读每个包（`keyframes.FromIndex`），随文件的 probe 一起进行：

* **Matroska 和 WebM**：通过 SeekHead（以及它指向的第二个 SeekHead）找到 Info（时间刻度和时长）、Tracks（第一条视频轨）和 Cues；视频轨的 cue 点就是它的关键帧。
* **MP4 和 QuickTime**：`moov` 中的第一条视频轨（`hdlr` 为 `vide`），逐个读取顶层 box 的头部找到 `moov`，不论它在文件的什么位置。关键帧是同步样本（`stss`；没有它时每个样本都是），时间为解码时间（`stts`）加合成偏移（`ctts`），再按编辑列表（`elst`）平移。分片文件的样本表是空的：由文件末尾 `mfra` 中该轨的 `tfra`（通过 `mfro` 找到），或第一个分片之前、各子分段以关键帧开头的 `sidx` 给出关键帧。
* 时间取整到微秒，即 ffprobe 包时间的精度，两者因此得到相同的分段。

读取的只是几个元素或 box：在测试媒体上不到每个文件的 2%。只有没有索引的文件才由 `media.keyframes` 任务整个读取，云存储上不读。复制没有关键帧的文件的视频的播放，和以前一样转码视频。

### 2.5 用关键帧生成 trickplay

一张 trickplay 缩略图只需要它所在时间附近的一幅画面，不需要之前的每一帧：

* **按索引抽样**（Matroska 和 WebM）：对每个缩略图时间，取不晚于它的 cue，由它得到关键帧所在的 cluster，再由 cue 的相对位置得到 cluster 中的块（没有相对位置时，依次读 cluster 中各块的头部，直到找到视频轨的关键帧）。Mavio 写出一个 Matroska 流：EBML 头、Info、视频轨的条目，以及每张缩略图一个 cluster，内含该关键帧并以缩略图的时间呈现（`Matroska.WriteSamples`）；这个流通过管道交给一次 ffmpeg 运行，缩放并拼接画面。每张缩略图读一个关键帧，相邻时间对应同一个关键帧时只读一次，其余什么都不解码。
* **只解码关键帧**（其他文件，允许整文件读取时）：ffmpeg 只解码关键帧（`-skip_frame nokey`）。仍要读整个文件，但解码量只是一小部分。
* **解码每一帧**：只解码关键帧失败时的兜底。
* 每种方式下，缩略图都取不早于其时间的第一幅画面（`fps=…:round=down:eof_action=pass`），因此关键帧在下一个时间之前结束时，最后一张缩略图仍然保留。ffmpeg 单线程、低优先级运行（nice 10，Windows 上为低于正常的优先级类），把 CPU 留给播放。

### 2.6 内嵌字幕

客户端不自行解复用文件中的字幕时，由服务端以文件或 HLS 字幕轨的形式提供。服务端如何读取，取决于存储类别：

* **本地和网络存储**：第一个需要内嵌字幕的播放，用一次 ffmpeg 运行提取文件中所有内嵌文字字幕流，只读一遍文件（ASS 保留样式，其他文本转为 SRT），存入条目的元数据目录（`library.SubtitlesDir`），并记下文件的大小和修改时间。之后任何字幕流的播放都从那里读取，直到文件变化；同时请求的播放等待这一次运行。
* **云存储，以及通过 HTTP 读取的媒体源**：内嵌字幕流绝不提取（`decision.Source.StreamedSubtitles`）。原生客户端从所播放的文件中自行解复用。以文件形式接收字幕的客户端改为通过 HLS 字幕轨获得：播放变为 HLS 转封装或转码，产生视频的那次 ffmpeg 运行同时把每条内嵌文字字幕流写成 WebVTT（`planner.HLSOutput.Subtitles`）。运行读过某个字幕分片之后才提供该分片（`Stream.Subtitle`）；拖动会重启运行，字幕随之重启，之前各次运行读到的内容仍然保留。两者都不接受的客户端则烧录字幕。
* **视频旁的字幕文件**很小，在所有存储类别上都直接读取。

### 2.7 落地对照索引

| 机制 | 实现 | 阶段 |
| :--- | :--- | :--- |
| 媒体库路径的存储类别 | `libs/core/library.go`（`StorageClass`、`Library.StorageClasses`、`Library.StorageClassOf`）；`libs/store/libraries.go`；`libs/proto/mavio/library/v1/library.proto` 中的 `storage_classes` 和 `effective_storage_classes`；`apps/server/internal/rpc/library.go` | P24 |
| 分类 | `libs/library/storage/class.go`（`DeviceInfo.Class`、`Classifier`、`PathClasses`）；`device.go` 中的 `Detector.Folder` | P24 |
| FUSE 与本机回环识别 | `libs/library/storage/fuse.go`（`fuseKind`、`loopbackSource`、`findMount`、`mountKind`）；由 `device_{linux,darwin,windows}.go` 使用 | P24 |
| 存储策略 | `libs/library/policy.go`（`StoragePolicy`、`PolicyOf`）；由 `scan.go`（`scan.policy`、`Scanner.walkers`、`scan.state`）、`jobs.go`（`Jobs.probeFile`）、`keyframes.go`、`borders.go`、`mediaextras.go`（`Jobs.extrasJobs`、`trickplay`、`chapterImages`、`loudness`）使用 | P24 |
| 从容器索引读取关键帧 | `libs/media/keyframes/index.go`（`FromIndex`、`FromFile`、`ErrNoIndex`）、`matroska.go`、`mp4.go`；`apps/server/internal/providers/keyframes.go`；`libs/library/jobs.go`（`Jobs.probe`）、`keyframes.go`（`KeyframeExtractor`） | P24 |
| 用关键帧生成 trickplay | `libs/media/keyframes/sample.go`（`Matroska.WriteSamples`）、`matroska.go`（`ReadMatroska`）；`libs/media/thumbnails/thumbnails.go`（`Mode`、`TrickplayArgs`、`Maker.Trickplay`）、`priority_{unix,windows,other}.go`；`libs/library/mediaextras.go`（`Thumbnailer`、`Jobs.trickplay`） | P24 |
| 内嵌字幕 | `apps/server/internal/playback/subcache.go`（`Manager.extracted`、`extractAll`）、`subtitles.go`（`Playback.liveSubtitles`、`liveSubtitle`、`Manager.loadSubtitle`）、`manager.go`（`Manager.Start`）；`libs/library/mediaextras.go`（`SubtitlesDir`）；`libs/media/decision/source.go`（`Source.StreamedSubtitles`）、`subtitles.go`；`libs/media/planner/command.go`（`HLSOutput.Subtitles`）；`libs/streaming/subtitles.go`（`Stream.Subtitle`） | P24 |
| 不对云存储做推测式读取 | `libs/library/storage/keeper.go`（`Keeper.Class`）；`apps/server/internal/playback/resume.go`（`Manager.prepareStart`）；在 `apps/server/internal/server/server.go` 中装配 | P24 |
| 存储端口 | `libs/core/storage.go`（`Storage`、`StorageCapabilities`、`StorageTransfers`、`StorageFS`、`StorageName`、`StoragePath`）；`libs/library/localstorage.go`（`Storages`、`OpenLocalStorage`、`RootOf`）、`mediafile.go`（`MediaFile`）；`libs/media/planner/inputs.go`（`InputURL`）；`apps/server/internal/acquisition/files.go`（`Runtime.transfer`） | P25 |

---

## 3. 通过 rclone 读取远端存储（Reading Remote Storage through rclone）

状态：已设计，尚未实现（P25）。rclone 远端上的媒体库（[架构](architecture.zh-CN.md) §9.2）经 `rclone rcd` 读取媒体。本章针对 rclone 提供的所有后端，使这些读取发出的请求更少、拖动时更稳定、读过的数据可以复用：

* mavioai/rclone 的 rcd 保留它找到的对象和取得的下载链接，限制同时打开的数据流，并如实报告存储返回的错误。
* 修正代价超出必要的后端。
* Mavio 缓存媒体数据块。

### 3.1 目前一次读取的代价

每次读取一段媒体字节都要经过 Mavio → `rclone rcd`（`--rc-serve`）→ 后端 → 存储，每一步都可能付出超出必要的代价。

**每个请求都重复查找文件。** `rcserver.serveRemote` 对每个 HTTP 请求调用 `NewObject`，请求之间什么也不保留。`dircache` 只缓存目录 ID，而 rcd 在一个远端的 `Fs` 最后一次使用 5 分钟后，将其连同 `dircache` 一起丢弃。一次查找的代价：

| 查找 | 后端 |
| :--- | :--- |
| 不发请求 | mega（节点树保存在内存中） |
| 一个请求：按路径、按 ID 或在服务端按名称过滤 | s3、b2、azureblob、azurefiles、googlecloudstorage、swift、oracleobjectstorage、qingstor、storj、sia、netstorage、filescom、webdav、sftp、smb、hdfs、http、支持 `MLST` 的 ftp、seafile、drive、onedrive、dropbox、jottacloud、koofr、yandex、mailru、putio、sharefile、hidrive、filefabric、shade、pixeldrain、gofile、ulozto |
| 两个请求 | box、opendrive、quatrix、internxt、imagekit、cloudinary |
| 列出整个父目录 | cloud115、baidunetdisk、cloud123、guangyadrive、quark、weiyun、pcloud、iclouddrive、fichier、filelu（另加一个请求）、protondrive（解密每个名称）、chunker（在其底层远端上）、不支持 `MLST` 的 ftp |
| 列举父目录直到找到文件 | aliyundrive、pikpak、thunder、premiumizeme、sugarsync、zoho（同一目录每分钟最多列举 19 次）、huaweidrive、drime、linkbox |
| 更多 | filen（列出并解密路径上的每一级目录）、internetarchive（条目的全部元数据）、名称中不含 ID 的 googlephotos（整个相册或月份）、union（每个上游）、compress（完整读取其元数据对象，再查找数据对象） |

**许多打开操作先要申请下载链接。** `rcserver` 每个请求都新建 `Object`，所以存在 `Object` 上的链接只存活一个请求：

| 链接 | 后端 |
| :--- | :--- |
| 每次 `Open` 都申请，不保存 | cloud115、aliyundrive（有效 4 小时）、baidunetdisk、cloud123、guangyadrive、quark、weiyun、yandex、putio、sharefile、quatrix、ulozto、filelu、iclouddrive（尽管列举结果中已带链接）、seafile、filescom、mega、protondrive（链接、密钥与当前版本）、internxt（文件的分片索引） |
| 每次 `Open` 申请一次性令牌 | fichier |
| 每次 `Open` 都跟随一次重定向 | onedrive、box、shade、internetarchive |
| 存在 `Object` 上 | pikpak、thunder、pcloud（直到过期前 30 秒）；premiumizeme、gofile、drime、linkbox 与 googlephotos 取自列举结果的链接，从不更新 |

只有 thunder 会在存储拒绝过期链接时重新获取链接。

**有些拖动不止花费一个范围请求：**

* crypt 先读取文件的 32 字节头部以取得 nonce，再从所在块重新打开，每次拖动两个请求。
* mega、protondrive 与 filen 读取整个分块，丢弃偏移之前的部分。protondrive 缺少块大小时从文件开头读起。
* filelu 不发送范围，丢弃偏移之前的全部数据。seafile 的加密资料库与 archive 的 zip 成员也是如此。
* webdav 与 http 传递范围，但不检查是否为 206。忽略范围的服务器会从文件开头返回数据，`serve.Object` 却把这些字节当作所请求的范围发出。
* shade 的第一个请求不带范围。

**没有任何机制限制打开的数据流。**

* `--max-connections` 只在收到响应头之前占用 pacer 令牌；HTTP transport 也没有设置 `MaxConnsPerHost`。
* ftp 与 smb 每个打开的数据流占用连接池中的一个连接，而连接池默认不设上限。
* 存储会拒绝或限速过多的并发数据流：
  * 云盘会暂时关闭账号的下载。
  * ftp 服务器限制每个用户的登录数。
  * mailru 的每台下载服务器最多 4 个数据流。

**错误被抹平。**

* `serve.Object` 把所有失败的 `Open` 都回应为 404，查找失败则回应 500。403、429、配额与限频错误永远到不了 Mavio，Mavio 也就无法退避。
* 在 rcd 内部，只有 onedrive、dropbox、b2、swift、zoho、putio、internxt、protondrive 与 seafile 会按存储的 `Retry-After` 等待。其他后端只按 pacer 退避，koofr、qingstor 与 http 则完全不重试。
* Drive 的每日下载配额（`downloadQuotaExceeded`）与 mega 的流量配额都被当作限频错误重试。

**Mavio 每次 seek 都重新打开。**

* `rclone.Storage` 以媒体库根目录命名远端的 `Fs`（`remote:dir`），同一远端的每个媒体库各有一个 `Fs` 及其缓存。
* 它的读取器对每次 `ReadAt` 和每次 seek 都发出新请求。keyframes 与 trickplay 读取容器索引时会发出大量小的 `ReadAt`。
* ffmpeg 经媒体代理读取时同样如此。

因此，在需要列目录和申请链接的后端上，播放中的一次拖动要付出一次目录列举、一次链接申请和一个新数据流，每一步都有存储的延迟。在任何后端上，快速连续的拖动、probe 与播放同时进行、两个人看同一个文件，叠加起来都会达到存储回应 403 或 429、或暂时关闭账号下载的 API 频率与数据流数量。

### 3.2 rcd 中的对象与链接

fork 对 rcd 与后端做以下修改；能对所有后端统一完成的，就统一完成。

**对象缓存**（`fs/objcache`）：rcd 进程内的缓存，以 `Fs` 的名称与对象路径为键。

* **缓存来源：**
  * `serveRemote` 在调用 `NewObject` 之前先在这里查找对象，并缓存找到的对象。
  * `operations/list` 与 `operations/stat` 服务于 Mavio 的扫描与 `Stat`，它们总是查询后端，从不读取缓存，并用返回的对象替换对应条目。只有 `--rc-serve` 读取缓存，所以扫描能立即看到存储上的变化，Mavio 的 `Stat` 给出的也总是存储上的当前版本。
  * 通过列目录查找文件的后端，以 fork 的特性 `FindByListing` 声明这一点，即查找表中最后三行除 union 与 compress 之外的后端。对它们而言，未命中时列一次父目录并缓存其中所有对象，随后对相邻文件的请求即可命中；同一目录的并发未命中共用一次列举。
* **随缓存的 `Object` 一起存活的数据：**
  * pikpak、thunder 与 pcloud 的链接；
  * premiumizeme、gofile、drime、linkbox 与 googlephotos 取自列举结果的链接；
  * s3、swift 与 webdav 的元数据；
  * compress 的元数据；
  * crypt 的 nonce（见下）；
  * union 选定的上游。
* **有效期与失效：**
  * 条目默认存活 5 分钟（可配置）。
  * 经 rcd 的写操作（`operations/*`、`sync/*`）会丢弃所涉及路径与目录的条目。
  * 后端支持 `ChangeNotify` 的远端（drive、onedrive、dropbox、box、pcloud、mega 等）留在缓存中期间，它报告的变化会丢弃对应条目。
  * 在别处做的修改表现为指纹不符（见下）或 `Open` 失败；此时丢弃条目、重新查找对象并重试一次。
  * 未命中不缓存。
* **版本校验：** 缓存的对象永远不会比 Mavio 打开文件时看到的版本更旧。
  * Mavio 的读取器打开文件时调用 `Stat`，并把结果随每个发往 `--rc-serve` 的请求一起发送：ID、大小、修改时间，以及后端列举结果中无需额外代价即可得到的哈希。
  * rcd 只在指纹相符时使用缓存的对象，否则重新查找。
  * 重新查找到的对象仍不相符时，rcd 回应 412。Mavio 将其视为文件已变化：丢弃该文件的块缓存项，并重新调用 `Stat`。
  * 按路径读取的后端（s3、b2、googlecloudstorage、azureblob、webdav 等）否则会以旧的大小提供新内容。对于支持按 ETag 条件读取的后端，`Open` 还会带上对象 ETag 的 `If-Match`。这样，读取中途被替换的文件会以 412 失败，而不会混入两个版本的数据。
  * 原地改写且 ID、大小与修改时间都不变的文件，只有在指纹包含哈希时才能发现。

**链接缓存**（`lib/linkcache`）：每个需要申请链接的后端在 `Open` 中使用。

* 以对象 ID 与指纹（大小，以及哈希或修改时间）为键，每个条目连同链接绑定的请求头与 cookie 一起保存。
* 过期时间依次取响应中的过期字段（aliyundrive 的 `Expiration`）、URL 中的过期参数、后端声明的有效期。存储未给出过期时间时，声明的有效期来自实测（§3.5）。
* 链接在有效期过去五分之四时更新；同一文件的并发打开共用一次申请。
* 存储对链接回应 401、403、404 或 410 时，丢弃链接、申请新链接并重试一次，与 thunder 现在的做法相同。
* 跟随重定向的后端以同样方式缓存重定向的目标地址。

| 后端 | 链接缓存保存的内容 | 另外 |
| :--- | :--- | :--- |
| cloud115 | 链接，按 pick code；绑定 user agent 与 cookie | API 调用把响应传给 `shouldRetry`，使 429 能被识别 |
| aliyundrive | 链接，4 小时（`expire_sec`） | — |
| baidunetdisk | `dlink` 及其解析出的重定向地址 | 加上 `dircache`（它目前没有）；`Open` 中的 HEAD 与 GET 经过 pacer 及其重试 |
| cloud123 | 链接；列举结果中有 `DownloadURL` 时直接使用 | — |
| guangyadrive、quark、weiyun | 链接 | weiyun 的下载状态错误转为 `shouldRetry` 能识别的 HTTP 错误 |
| yandex、putio、sharefile、quatrix、ulozto、seafile | 链接 | — |
| filelu | 链接 | `Open` 发送范围而不是丢弃数据，其错误经过 `shouldRetry` |
| iclouddrive | 列举结果中的 `downloadURL`，否则为链接 | — |
| filescom | `DownloadUri`，传给 SDK | — |
| mega | `g` 命令返回的 URL | — |
| protondrive | 链接、会话密钥与当前版本 | — |
| internxt | 文件的 bucket 信息 | — |
| onedrive、box、shade、internetarchive | 重定向的目标地址 | shade 在第一个请求中就带上范围 |
| pikpak、pcloud | —（链接存在缓存的 `Object` 上） | — |
| thunder | —（链接存在缓存的 `Object` 上） | 加上过期检查 |
| fichier | —（其令牌只能使用一次） | — |

**对个别后端的修正：**

* webdav 与 http：请求了范围而服务器回应 200 时，`Open` 丢弃偏移之前的字节（与 mailru 的做法相同），使 body 与范围一致。
* crypt：第一次读取后把 nonce 保存在其 `Object` 上，使一次拖动只需一个请求。

**经 `--rc-serve` 与 `operations/fsinfo`：**

* **状态码：** 查找与打开回应存储错误对应的状态码，而不是 404 与 500：
  * 对象不存在时为 404；
  * 对象版本与 Mavio 发送的指纹不符时为 412；
  * 403；
  * 429 与 503，带上它们的 `Retry-After`（`fserrors.ErrorRetryAfter`）；
  * 配额错误（drive 的 `downloadQuotaExceeded`、mega 的 509）回应为 429，存储给出重置时间时一并带上；
  * 其他错误为 502。
* **拖动：** `operations/fsinfo` 报告需要从开头读到偏移处的远端：seafile 的加密资料库与 archive 的 zip 成员。Mavio 顺序读取这些远端（§3.4）。
* **每个远端一个 `Fs`：** Mavio 以远端根目录命名对象（`remote:` 加完整路径），使同一远端的所有媒体库共用一个 `Fs` 及其缓存。Mavio 以 `--fs-cache-expire-duration 1h` 启动 rcd。

### 3.3 连接数限制

fork 的限制器（`lib/connlimit`）为所有后端限制 `--rc-serve` 同时打开的数据流。

* **键：** 文件的数据流以其对象为键；账号的数据流以实际提供字节的远端为键，即各层包装远端经 `UnWrap` 揭示出的最内层远端。所以云盘之上的 crypt 远端计入该云盘账号。
* **占用名额：** `serve.Object` 在打开对象之前取得该文件的一个名额和账号的一个名额，请求的 body 关闭时释放这两个名额，所以名额随数据流释放。
* **文件名额已满时：** 关闭该文件中最久没有读取的数据流，把名额交给新的打开。播放器拖动、浏览器请求新范围时，都是先打开下一个数据流再关闭上一个，而上一个的读取方已经不再读它。被关闭数据流的读取方会收到错误，如果仍要读取就重新打开。
* **账号名额已满时：** 打开操作等待名额，直到其 context 结束；它从不占用其他文件的名额。
* **限制值：**
  * 各后端以 fork 的特性（`MaxFileStreams`、`MaxStreams`）声明默认限制值，按实测结果（§3.5）或存储公布的限制设定，如 mailru 每台下载服务器 4 个数据流。
  * 远端自身选项已设定的值优先：
    * ftp 的 `--ftp-concurrency`；
    * sftp 的 `--sftp-connections`。
  * 选项 `--rc-serve-max-file-streams` 与 `--rc-serve-max-streams` 覆盖默认值。
  * 既未实测、也没有公布限制的后端不设限制。

### 3.4 块缓存

Mavio 把从 rclone 远端读取的字节保存在本地磁盘上，位于 `libs/rclone`。缓存处在 Mavio 的两条读取路径之下：

* `rclone.Storage.Open` 的读取器，用于直接播放、probe、keyframes 与 trickplay 的容器索引读取以及外挂文件。
* ffmpeg 读取的媒体代理。代理从缓存提供数据，不再直接转发。

它在 rclone VFS 缓存（`--vfs-cache-mode full`）行之有效的地方沿用其做法，在 Mavio 掌握更多信息的地方有所不同：

| | rclone VFS 缓存 | Mavio 块缓存 |
| :--- | :--- | :--- |
| 数据 | 每个对象一个稀疏文件，大小与原文件相同 | 相同 |
| 已有数据 | 有序、合并后的字节范围（`lib/ranges`） | 相同 |
| 元数据 | 打开与关闭时写入 JSON；数据到达时不保存范围 | 带范围的 JSON，最多每隔几秒及关闭时保存，且在它记录的数据写入之后 |
| 过期判断 | 打开时检查指纹（大小、修改时间、哈希） | 打开时检查 `Stat` 给出的 §3.2 中的指纹；读取中大小变化或收到 412 则丢弃该项 |
| 缺失范围 | 若某个下载器的位置与之相距不超过 `--buffer-size`（16 MiB），就延长该下载器；否则启动新的下载器 | 相同，窗口大小可配置 |
| 每个文件的下载器 | 不限 | 不超过该文件的连接数限制（默认 2）；超出时停止离所有等待中的读取最远的下载器 |
| 上游请求 | 128 MiB 的块，大小无上限地翻倍 | 16 MiB 的块，翻倍至 256 MiB 为止，使放弃的读取浪费很少 |
| 预读 | `--vfs-read-ahead`，默认 0，另有 16 MiB 内存缓冲 | 按意图预读（见下）；数据以 1 MiB 为单位直接写入文件，不经内存缓冲 |
| 下载器结束 | 空闲 5 秒，或连续遇到 1 MiB 已有数据 | 相同 |
| 错误 | 连续 10 次错误后，等待者失败 | 相同，并对 403、429 与 503 按 `Retry-After` 退避 |
| 磁盘读取 | 持有该项的锁 | 不持锁；只保护范围信息 |
| 淘汰 | 先淘汰未在使用的项，最久未用者优先；超出配额时整体丢弃已打开的项 | 先淘汰未在使用的项；再淘汰已打开项中落后于所有读取位置最远的范围，保留读取位置前方的数据 |

预读按每次读取的意图进行，这是文件系统层面的缓存无法知道的：

* **probe：** 在 ffprobe 运行前，用两个请求同时取回文件头（1 MB）和文件尾（256 KB）；ffprobe 随后从缓存读取。
* **容器索引：** 读取 Matroska 元素与 MP4 box 的大量小读取扩展到 256 KB 对齐，使相邻元素共用一个请求。
* **seek：** 播放开始或拖动到某个时间时，Mavio 在源的 keyframes 中查出对应关键帧，并在 ffmpeg 或客户端请求之前，从该关键帧的字节位置启动下载器。
* **HLS：** 请求某个分段时，下载器延伸到其后几个分段的字节范围。复制视频时这些范围由关键帧位置得出，否则按码率估算。
* **直接播放：** 下载器保持领先客户端最近一次请求的范围 30 秒的码率，最多 256 MiB。
* **后台任务：** 读取时不预读。

同一文件的多个读取方共用一个缓存项，如 probe 与播放，或两个观看者。一段范围无论有多少读取方在等待，都只下载一次。

* **大小：** 一项设置，`rclone-cache-size`，默认 10 GiB，并保留最少 5% 的空闲磁盘空间。
* **时限：** 7 天未读的项会被删除。
* **位置：** `<cache>/rclone/blocks`，重启后保留。
* **范围：** `cloud` 存储类别（§2.1）的远端。媒体库未另行设置时，rclone 路径即属此类。

以下远端按顺序读取：

* rcd 报告需要从开头读起的远端（§3.2）；
* 经实测（§3.5）其存储拒绝不能接续已打开数据流的范围请求、并在远端上设置了此项的远端。

在顺序读取的远端上，下载器不会在超出最远已打开位置的新位置启动，而是让那个数据流一直读到目标位置。在这种远端上远距离拖动要等待之前的字节。

### 3.5 实测

限制值、链接有效期与顺序读取设置的默认值来自实测。用测试账号经 rcd 逐个后端实测，先测 fork 的云盘与常用的个人云盘（drive、onedrive、dropbox、box、pcloud、mega、yandex、protondrive）。实测内容：

* 顺序读取；
* 向前与向后跳转；
* 同一文件与多个文件的并发范围请求。

实测统计对 API 与存储的请求数（`--dump headers`），并记录哪种模式会引发 403、429 或限速，在多少次请求之后发生、持续多久。

实测结果决定各后端的数据流限制、链接有效期以及其远端是否需要顺序读取，并记录在 fork 的 `MAVIO_PROVIDERS.md` 中。

### 3.6 实现索引

| 机制 | 实现 | 阶段 |
| :--- | :--- | :--- |
| 对象缓存 | mavioai/rclone `fs/objcache`；`fs/rc/rcserver`（`serveRemote`）、`fs/operations/rc.go`（`list`、`stat` 与各写操作），以及通过列目录查找文件的后端的 `FindByListing` 特性 | P25，计划中 |
| 版本校验 | mavioai/rclone `fs/objcache`、`lib/http/serve/serve.go`（指纹与 412）、支持 ETag 的后端的 `Object.Open` 中的 `If-Match`；`libs/rclone/storage.go`（`file`）、`proxy.go`（发送指纹） | P25，计划中 |
| 链接缓存 | mavioai/rclone `lib/linkcache`；§3.2 表中各后端的 `Object.Open` | P25，计划中 |
| 后端修正 | mavioai/rclone `backend/webdav`、`backend/http`（以 200 回应的范围请求）、`backend/crypt`（nonce）、`backend/filelu`（范围），以及 §3.2 表中"另外"一列 | P25，计划中 |
| 经 rcd 传递状态码与拖动方式 | mavioai/rclone `lib/http/serve/serve.go`（`Object`）、`fs/rc/rcserver`、`fs/operations/rc.go`（`fsinfo`） | P25，计划中 |
| 每个远端一个 `Fs`、更长的 `Fs` 缓存 | `libs/rclone/storage.go`（`Storage.fsName`、`Client.objectURL`）、`daemon.go`（`Daemon.launch`） | P25，计划中 |
| 连接数限制 | mavioai/rclone `lib/connlimit`；`lib/http/serve/serve.go`；`MaxFileStreams` 与 `MaxStreams` 特性 | P25，计划中 |
| 块缓存 | `libs/rclone`（缓存及其下载器与范围）；`storage.go`（`file`）、`proxy.go`（`Proxy.ServeHTTP`）；意图来自 `apps/server/internal/playback` 与 `libs/library`（`Jobs.probeFile`） | P25，计划中 |
| 实测 | mavioai/rclone `MAVIO_PROVIDERS.md` | P25，计划中 |

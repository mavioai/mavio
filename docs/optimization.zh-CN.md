# 优化规范

> [English](optimization.md) | 简体中文

> 相关文档：[系统架构](architecture.zh-CN.md) · [领域模型](domain.zh-CN.md) · [开发指南](development.zh-CN.md) · [测试策略](testing.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

> 本文档规范 Mavio 的全局性能优化体系。包含存储 I/O 调度与预热、媒体转码管线起播延迟、字幕元数据检索以及确定性工程构建规范。

---

## 1. 目标与设计原则

Mavio 作为自托管媒体服务器，核心部署环境通常是家庭服务器、多盘位 NAS（如群晖、TrueNAS、Unraid）或低功耗主机。这些环境具有极强的存储异构性：
1. **机械硬盘（HDD）吞吐脆弱**：机械盘存在 10~15ms 物理寻道开销，多任务并发随机读会引发磁头剧烈抖动（Head Thrashing），导致吞吐从 150MB/s 断崖式跌落至个位数 MB/s。
2. **网络存储与外接盘易休眠**：SMB/NFS 挂载点与 USB 外置硬盘在无读写数分钟后会自动休眠停转（Spindle Spin-down），重新起旋需 5~10 秒，极易造成客户端起播超时。
3. **前后台 I/O 争抢严重**：后台媒体库全量对账扫描（Reconciliation Scan）或雪碧图抽帧若打满磁盘 I/O 队列，前台用户的实时点播与 HLS 切片传输会陷入严重缓冲卡顿。
4. **下载与写入中文件易脏读**：PT/BT、Aria2 或自动化抓取工具下载中的半截视频如果被提前扫描，会导致 `ffprobe` 探测崩溃或入库错误时长。

为此，Mavio 确立以下存储与媒体优化原则：
* **物理卷感知，杜绝并发寻道**：后台 I/O 必须按物理驱动器（Physical Device）进行单并发单通道排队，永远保持机械盘最佳顺序读性能。
* **前台点播绝对优先**：当存在活跃的客户端播放流时，后台扫描与离线任务主动退避与降级。
* **最小预算精准预读**：拒绝无节制盲目缓冲，以极低内存开销精准命中文件头（元数据）与文件尾（索引）。
* **零侵入与跨平台自适应**：无缝支持 Linux、Windows 与 macOS，针对 OS 特性选用最高效的系统调用，对 SSD 透明高速，对 HDD 关键保护。

---

## 2. 存储 I/O 延迟抑制与推测性预热调度系统 (Storage I/O Latency Mitigation and Speculative Warming)

### 2.1 存储介质与挂载协议多维度感知

Mavio 服务端在扫描与读取媒体路径时，通过跨平台抽象层识别底层介质类型与挂载协议：

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
1. **机械盘（Rotational）识别**：
   * 通过 `stat(path, &st)` 提取设备主次设备号 `st.st_dev`；
   * 读取 `/sys/dev/block/<major>:<minor>/queue/rotational`：
     * `1`：旋转介质（机械硬盘 HDD），自动启用物理卷防抖动单并发排队；
     * `0`：非旋转介质（NVMe / SATA SSD），允许常规高并发 I/O。
2. **网络卷与挂载协议识别**：
   * 调用 `statfs(path, &buf)` 检查 `buf.Type`：
     * `0x517B` (`SMB_SUPER_MAGIC`) / `0xfe534d42` (`SMB2_MAGIC_NUMBER`)：SMB 共享；
     * `0x6969` (`NFS_SUPER_MAGIC`)：NFS 挂载；
     * `0x65735546` (`FUSE_SUPER_MAGIC`)：FUSE 挂载（如 Rclone / Alist）。

#### Windows
1. **机械盘识别**：
   * 通过 `CreateFileW` 获得卷句柄，调用 `DeviceIoControl` 发送 `IOCTL_STORAGE_QUERY_PROPERTY`，查询 `StorageDeviceSeekPenaltyProperty`；
   * 返回的 `DEVICE_SEEK_PENALTY_DESCRIPTOR.IncursSeekPenalty` 为 `TRUE` 即判定为机械硬盘。
2. **网络卷识别**：
   * `GetDriveTypeW(rootPath) == DRIVE_REMOTE`，或检测 UNC 路径（`\\server\share`）。
3. **云盘占位文件拦截**：
   * 检查文件属性是否含有 `FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS`（`0x00400000`）；
   * 后台扫描打开文件时附带 `FILE_FLAG_OPEN_NO_RECALL`，杜绝静默拉取未同步的云端文件。

#### macOS
1. **网络卷识别**：
   * `statfs.f_fstypename` 匹配 `"smbfs"`, `"nfs"`, `"afpfs"`, `"webdav"`；且 `(sfs.f_flags & MNT_LOCAL) == 0`。
2. **云盘与占位文件拦截**：
   * 路径识别 `~/Library/Mobile Documents`（iCloud）与 `~/Library/CloudStorage`（网盘 FileProvider）；
   * 线程级系统调用设置 `IOPOL_TYPE_VFS_MATERIALIZE_DATALESS_FILES` 为 `IOPOL_MATERIALIZE_DATALESS_FILES_OFF`。

---

### 2.2 物理驱动器单并发队列与防磁头抖动（Per-Device Serialization）

* **设计背景**：当媒体库有成千上万个文件分布在同一个机械硬盘上时，如果并发派发探测任务，磁头在盘片内圈与外圈剧烈往返寻道，会导致总 I/O 延迟急剧恶化。
* **调度机制**：
  * 在 `libs/library` 中引入物理设备账本（`DeviceLedger`）。
  * 提取物理设备标识（Linux: `st_dev` / major:minor；Windows: Volume Serial Number；macOS: `f_fsid`）作为锁的 Key。
  * **单设备信号量**：被识别为机械硬盘（HDD）或远端挂载卷的设备，同一物理设备上在任意时刻**仅允许 1 个后台扫描/探测任务持有 I/O 令牌**。
  * **效果**：任务顺序执行，机械硬盘磁臂以连续顺序读模式运行，吞吐保持在硬件峰值，杜绝磁头抖动噪声与机械磨损。

---

### 2.3 活跃点播与后台扫描 I/O 互锁门禁（Storage Quiet Gate）

* **设计背景**：自建媒体服务器在进行全库扫描、计算哈希或生成雪碧图时，一旦用户开始播放电影，前台与后台争抢磁盘队列，造成前台 HLS 切片或 Direct Play 严重卡顿。
* **调度机制**：
  * 服务端会话管理器实时维护活跃播放流计数（`activeStreamingSessions`）。
  * 当客户端触发点播、拖动进度（Seek）或请求分片时，激活全局或分卷的静默窗口（`quietUntil = now + quietPeriod`，默认静默期 3 秒）。
  * 后台任务（扫描 Worker、元数据抓取、关键帧提取）在发起物理 I/O 前必须检查门禁：
    * 若处于静默期，工作协程主动 `time.Sleep` 避让，暂停磁盘抢占；
    * 动态收缩 `errgroup.SetLimit`，将后台并发度压至最低；
    * 当所有播放会话结束且静默期过去后，后台队列平滑恢复全速。

---

### 2.4 定向元数据预算预读（Targeted Head/Tail Prefetching）

* **设计背景**：视频容器（MP4/MKV）的解码关键信息分布在两端：头部包含文件签名与基础轨信息；尾部经常存放索引（如 MP4 的尾部 `moov` box、MKV 的尾部 `Cues` 索引、AVI 的尾部 `idx1`）。如果盲目全量缓存，会浪费大量内存与带宽。
* **预算预读策略**：
  * **Head 1 MB**：读取文件起始 1MB，足以覆盖绝大多数 MP4 `ftyp/moov`、MKV EBML Header 及 MPEG-TS 首部。
  * **Tail 256 KB**：快速跳转至末尾读取最后 256KB，覆盖尾部索引数据。
  * **Linux 内核原生预读优化**：
    在 Linux 上，服务端无需分配应用层缓冲区做无谓的内存拷贝，直接使用系统调用：
    ```c
    posix_fadvise(fd, 0, 1024 * 1024, POSIX_FADV_WILLNEED);
    posix_fadvise(fd, fileSize - 256 * 1024, 256 * 1024, POSIX_FADV_WILLNEED);
    ```
    内核异步调度 I/O 线程将目标扇区灌入 Page Cache，后续 `ffprobe` 探测与起播解封装实现零等待直接命中内存。

---

### 2.5 远端网络卷与外部驱动器防休眠心跳（Volume Heartbeat）

* **设计背景**：外接 USB 机械移动硬盘或 NAS 共享存储通常配置了 5~15 分钟闲置休眠（Spin-down）以节能。用户在浏览媒体库准备看片时，若磁盘已休眠，点击播放需等待主轴马达加速 5~10 秒，极易产生“服务器卡死”的错觉。
* **心跳保活机制**：
  * **受控心跳周期**：当检测到客户端处于活跃浏览状态（如通过 WebSocket/Connect-RPC 保持长连接）且访问的媒体位于网络卷或外置机械盘时，每隔 2.5 秒发送一次微量保活心跳。
  * **穿透缓存的微量随机读**：
    * Linux 下使用 `O_DIRECT`，Windows 下使用 `FILE_FLAG_NO_BUFFERING`，macOS 下使用 `fcntl(F_NOCACHE, 1)`；
    * 在文件随机偏移处读取 4KB 数据（`pread`），强制物理磁盘接收微弱 I/O 命令，维持主轴马达旋转。
  * **在场检测与节能保护**：一旦客户端断开连接或进入空闲超时，保活心跳立即终止，不破坏存储设备的正常休眠节能逻辑。

---

### 2.6 “生长中/下载中文件”感知与延迟入库策略（Growth Policy）

* **设计背景**：媒体库目录通常直接对接下载工具（qBittorrent、Aria2、Sonarr、Radarr）。未下载完成的文件如果被定时扫描强行处理，会引发探测报错、记录错误时长甚至破坏播放历史。
* **生命周期状态机**：
  * **下载临时后缀过滤**：扫描器天然静默忽略 `.part`、`.crdownload`、`.download`、`.aria2` 等临时写入后缀。
  * **文件活跃增长检测**：
    * 记录文件的 `(size, mtime)` 纳秒时间戳；
    * 检查当前 `time.Now() - mtime < 10s`，或在 Linux 下检查文件是否有活跃写入进程（非空锁/写句柄打开）；
    * 若文件尺寸正在变化或处于上述时间窗内，标记为 `Growing` 状态。
  * **延迟对账队列（Deferred Reconciliation）**：
    * 对 `Growing` 状态的文件不执行高开销的 `JobProbe`，不入库不报错；
    * 将其挂入内存延迟队列，在文件尺寸与修改时间稳定静止超过阈值（如 30 秒）后再发起正式对账与探测。

---

## 3. 媒体管道与低延迟起播优化

### 3.1 续播快速定位折叠与关键帧吸附（Resume Seek Folding & Keyframe Snapping）

* **传统续播痛点**：通常流程为：打开文件 $\rightarrow$ 读取头部元数据 $\rightarrow$ 解封装器发起时间戳 Seek 跳转 $\rightarrow$ 寻道回退至前一个关键帧 $\rightarrow$ 解码丢弃数秒内的非参考帧追赶至续播点。在慢速存储上，这一连串二次寻道需要数秒。
* **优化策略**：
  * **Seek 折叠（Fold Seek into Prepare）**：在解封装初始化阶段直接将续播时间戳传入，初始读取游标直接在续播位置附近建立，消除打开后的二次随机寻道。
  * **关键帧吸附（Keyframe Snap）**：
    如果续播目标点与前序关键帧的时间差在 5 秒以内（`diff <= 5.0s`），**直接将起播时间戳吸附至该关键帧**。
    * 彻底免除解码丢弃（Drop & Catch up）数秒视频帧的 CPU 开销；
    * 首帧即为关键帧，客户端或转码器在读取首个视频包后立即可完成解码上屏，首帧画面呈现时间缩短 70% 以上。

---

### 3.2 动态黑边自适应剔除与预览抽帧熔断

* **缩略图黑边自适应剔除**（`libs/imaging`）：
  * 2.35:1 电影在生成 16:9 进度条预览雪碧图（Trickplay / BIF）时往往自带上下大黑边，导致小图有效画面过小且浪费 20%~30% 的体积。
  * 引入纯 CPU 1/4 降采样黑边检测算法（支持 8-bit YUV420 与 10-bit P010）：
    * 遍历顶、底、左、右各行/列；
    * 采用前 16 采样点短路阈值比对，快速识别黑边边界；
    * 在生成雪碧图前统一执行 Crop 或记录切边坐标，大幅提升移动端和网页端进度条悬浮预览的视觉质量与带宽利用率。
* **抽帧失败熔断（Failure Circuit Breaker）**：
  * 在批量抽取关键帧或缩略图时，遇到受损 GOP 若无节制重试会导致后台任务死循环。
  * 引入失败熔断缓存（`FailNote`：记录受损时间段、TTL 与容量上限），在 TTL 窗口期内直接抑制重复抽帧，保证后台队列畅通。

---

### 3.3 杜比视界 Profile 7 双层流感知与降级规划

* **蓝光原盘痛点**：UHD Blu-ray 原盘通常采用双层杜比视界（Profile 7 FEL / MEL），包含一个基础层（Base Layer）和一个增强层（Enhancement Layer）。如果直接交给通用 FFmpeg 转码，会导致音画不同步或转码崩溃。
* **规划器优化**（`libs/media/planner/hdr.go`）：
  * 解析 MPEG-TS PMT 与 MP4 `vdep` 依赖关系，将 EL 识别为依赖流；
  * 当客户端仅支持 HDR10 时，准确规划 FFmpeg 流过滤参数（`-map` 剔除 EL 流），纯净保留 Base Layer 执行 HDR10 转码与色调映射；
  * 当客户端支持 Profile 8.1 时，规划专用滤镜进行双层到单层的元数据重构降级，确保杜比视界动态元数据完好流转。

---

### 3.4 多声道（5.1 / 7.1）立体声下混对白增强

* **痛点**：将 5.1/7.1 电影下混为双声道立体声时，常出现“人声对白极轻、环境爆炸声震耳欲聋”的声学失衡。
* **规划器优化**（`libs/media/planner/audio.go`）：
  * 在音频下混规划中，严格遵循规范的声道衰减矩阵与对白归一化：
    * 采用 ITU-R BS.775 权重；
    * 对中置声道（Center Channel，对白专用声道）施加增益补偿（+3dB ~ +4.5dB）；
    * 避免客户端（手机、平板、立体声电视）用户在观影时频繁手动调节音量。

---

## 4. 字幕与元数据检索优化

### 4.1 外部字幕多别名归一化与权重打分模型

针对国内影视字幕命名极其混乱的现状，`libs/naming` 与 `libs/library` 整合完备的别名归一化字典与打分模型：

#### 别名标准化映射
```text
"zh", "chi", "zho", "chs", "cht", "sc", "tc", "gb", "big5",
"zh-hans", "zh-hant", "zh-cn", "zh-tw", "zh-hk",
"简体", "繁體", "繁体", "简中", "繁中", "简", "繁", "中文", "双语" ──▶ "zh"
```

#### 智能打分模型
| 匹配特征 | 得分调整 | 说明 |
| :--- | :--- | :--- |
| **基名完全一致** | `+1000` | 如 `Movie.mkv` 对应 `Movie.srt` |
| **前缀匹配 + 合法边界符** | `+500` | 如 `Movie.1080p.mkv` 对应 `Movie.zh.srt` |
| **偏好语言匹配** | `+200` | 与用户设置的首选字幕语言一致 |
| **其他非偏好语言** | `-100` | 降低非目标语言外挂字幕优先级 |
| **`.forced` 强制字幕** | `-150` | 强制字幕通常仅含极少量对白，默认不优先加载 |
| **`.sdh` / `.cc` / `.hi` 听障字幕** | `-50` | 包含音效描述，作为次选 |
| **格式加权** | `ASS/SSA: +20` / `SRT: +10` | 优先选择排版与样式更丰富的字幕格式 |

---

## 5. 工程确定性与隐私不变式

### 5.1 Checksum-Pinned 依赖与 Zero-Fuzz 补丁规范

* **依赖确定性封存**：在构建打包官方 Docker 镜像与 external 组件（`jellyfin-ffmpeg`、WASM 工具链、SQLite 扩展）时，严格锁定源码包 SHA-256。
* **Zero-Fuzz 补丁原则**：下游补丁自身必须计算 SHA-256 并锁定在依赖清单中；应用补丁时强制采用零容差（Zero Fuzz），一旦上游代码微调导致补丁行偏移，构建立即报错，严禁静默迁移补丁代码。

### 5.2 零遥测与 Fail-Closed 默认静默原则

* **零数据收集（Zero Telemetry）**：Mavio 不内置任何播放统计上报、设备追踪或用户行为分析埋点，全量数据严格留在用户本地。
* **默认静默（Fail-Closed）**：当未配置外部刮削源（如 TMDB API Key）时，完全静默使用本地 NFO 与图片，不发起任何无效的外部网络探测请求。

---

## 6. 模块落地对照索引

| 优化特性 | 对应 Mavio 模块 | 落地阶段 |
| :--- | :--- | :--- |
| **字幕打分模型与中文别名归一化** | `libs/naming`, `libs/library` | P2 / P4 |
| **生长中文件 Growth Policy 与延迟队列** | `libs/library/scan.go` | P4 |
| **存储介质识别与物理驱动器单并发队列** | `libs/library/fs.go`, `libs/library/scan.go` | P4 |
| **活跃点播与后台扫描 I/O 互锁门禁** | `apps/server`, `libs/library/worker.go` | P5 |
| **定向元数据预读（Head/Tail Prefetch）** | `libs/media/probe`, `libs/library` | P3 / P4 |
| **续播定位折叠与关键帧吸附** | `libs/media/planner`, `libs/streaming` | P3 / P5 |
| **缩略图黑边裁剪与抽帧熔断** | `libs/imaging`, `libs/media/keyframes` | P2 / P5 |
| **杜比视界 Profile 7 依赖规划与多声道下混** | `libs/media/planner/hdr.go`, `audio.go` | P3 |

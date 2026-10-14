# Mavio 架构蓝图

> [English](architecture.md) | 简体中文

> 相关文档：[客户端](clients.zh-CN.md) · [领域模型](domain.zh-CN.md) · [开发指南](development.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [测试策略](testing.zh-CN.md) · [性能与优化](optimization.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

> Mavio 是一个以 Go 为服务端核心的新一代自托管媒体服务器。它借鉴 Jellyfin 多年沉淀的领域知识、FFmpeg 管线与测试资产，但不以兼容 Jellyfin 生态为目标，选型一律取当下最现代、最合适的方案。

---

## 1. 定位与原则

### 1.1 目标
* **轻量高效**：低常驻内存、快速冷启动、高扫描吞吐与低首分片延迟来自现代技术栈本身——无运行时虚拟机的原生 Go 单二进制、以 WASM 沙箱代替重量级隔离、ffmpeg 硬件管线。
* **单二进制交付**：服务端为 `CGO_ENABLED=0` 的单一 Go 二进制；唯一必需的外部运行时依赖是 `jellyfin-ffmpeg`；位于云存储上的媒体库另需 `rclone`（§9.2）。
* **强类型契约**：对内对外统一使用 Protobuf + Connect-RPC，未来的客户端（[客户端](clients.zh-CN.md)）直接从契约生成 SDK。
* **插件隔离**：插件不进入宿主地址空间，WASM（wazero）沙箱与独立子进程（UDS）两种运行时共享同一套契约。
* **多数据库**：SQLite 与 PostgreSQL 均为一等公民。
* **默认隐私**：没有遥测、播放统计或设备追踪，所有数据都留在主机上。没有元数据插件时，服务端只使用本地 NFO 文件与图片，不发出元数据的外部请求；访问资源获取所用服务的插件无需管理员提供密钥（§13.2）。

### 1.2 非目标（当前阶段）
* 不兼容 Jellyfin 客户端生态、不提供 Jellyfin 数据迁移。Jellyfin API 兼容垫片（shim）的评估见 [Jellyfin 兼容性](jellyfin-compat.zh-CN.md)，在 P21 决定。
* 暂不实现任何 UI（Web / Mobile / Desktop），但目录与契约提前为其预留。

### 1.3 工程原则
1. **自底向上**：先实现零业务依赖的底层库，每个库在完成时即通过其测试，再向上组装。
2. **无 CGO**：任何引入 CGO 的依赖都需要单独论证。需要原生能力时，优先选择 *ffmpeg 子进程* 或 *wazero 运行的 WASM 模块*。
3. **库可独立使用**：每个 Go 库是独立模块，可以脱离 Nx 直接 `go test` / `go build`；Nx 只做编排与缓存。
4. **契约先行**：跨进程、跨语言边界一律以 `libs/proto` 为唯一事实来源。

### 1.4 许可证
Mavio 采用 **GPL-3.0**（`GPL-3.0-only`）。GPL-3.0 与 Apache-2.0 兼容，因此可以使用 wazero、ent、connect-go、Atlas、OpenTelemetry 等核心依赖。第三方依赖的许可证必须与 GPL-3.0 兼容（MIT、BSD、ISC、Apache-2.0、MPL-2.0、LGPL、GPL-3.0），禁止 GPL-2.0-only。

---

## 2. 技术选型

| 维度 | 选型 | 说明 |
| :--- | :--- | :--- |
| 服务端语言 | **Go（跟随最新稳定版，当前 1.27）** | 使用 `iter.Seq` 流水线、`log/slog`、`os.Root`（媒体路径沙箱）、`testing.B.Loop`、`testing/synctest`、go.mod `tool` 指令 |
| 契约与 RPC | **Protobuf + buf + Connect**（`connect-go`；TS 端 `protobuf-es` v2 / `connect-es` v2） | `buf lint` + `buf breaking` 守护契约演进；`protovalidate` 做声明式校验 |
| HTTP | 标准库 `net/http`（1.22+ 路由模式） | Connect handler 直接挂载；媒体流走普通 HTTP（Range / HLS） |
| API 参考 | **protoc-gen-connect-openapi** + **Scalar** | 从契约生成一份 OpenAPI 3.1 文档；Connect gRPC 反射（`grpcreflect`）供 `buf curl`、grpcurl 等工具使用 |
| ORM / 查询 | **ent + sqlc + Atlas** | 见 §5 |
| 数据库 | **SQLite + PostgreSQL** | SQLite 驱动 `ncruces/go-sqlite3`（WASM + wazero，纯 Go）；PostgreSQL 驱动 `pgx/v5`（以 `database/sql` 方式接入） |
| 音视频 | **jellyfin-ffmpeg**（外部进程） | 复用其硬件驱动补丁、色调映射滤镜与低功耗编码支持 |
| 图像 | **纯 Go + wazero WASM 编解码器 + ffmpeg** | 见 §6 |
| 流媒体 | **HLS + fMP4（CMAF）**；直放走 HTTP Range | hls.js / Media3 / AVPlayer 均原生支持 |
| 插件 | **wazero WASM + 子进程（统一 UDS）** | 见 §7 |
| 文件名规则 | **Jellyfin 的命名规则，基于 `dlclark/regexp2`** | 规则使用 .NET 正则（环视、原子组），`regexp2` 可原样执行；每次匹配限时一秒 |
| 并发 | `errgroup.SetLimit` + `iter.Seq` | |
| 变更检测 | **全量对账扫描**（定时、启动时、手动 / API 触发），以目录 mtime 剪枝 | 见 §9 |
| 后台任务 | 自建数据库任务队列（租约 + 重试） | 同时支持 SQLite / PostgreSQL |
| 可观测性 | `log/slog` + OpenTelemetry（trace / metrics） | |
| 命令行与设置 | **Cobra + Viper** | 命令行参数、`MAVIO_*` 环境变量与 Mavio 主目录中的 `config.toml`；见 §11 |

工程工具（Nx、mise、buf、golangci-lint、CI）见[开发指南](development.zh-CN.md)。

### 2.1 wazero 作为共享原生能力层
wazero 同时承载三类负载：SQLite（ncruces）、图像编解码器（§6）和 WASM 插件（§7）。这样原生 C 库能力全部通过 WASM 获得，整个服务端保持纯 Go、可任意交叉编译。

注意：wazero 的编译器后端只支持 amd64 / arm64，其他架构（如 armv7、riscv64）会退化为解释器，SQLite 与 WASM 编解码器在其上会慢很多。因此发布只面向 amd64 与 arm64（§12）；为其他架构提供纯 Go 兜底是[路线图风险](roadmap.zh-CN.md#4-风险与待决事项)中的待决事项。

---

## 3. 仓库结构

Monorepo 的构建与开发方式（Nx、Go 模块约定、代码生成、CI）见[开发指南](development.zh-CN.md)。

### 3.1 目录结构
`libs/` 下的库直接按领域命名；同一个库可以同时是 Go 模块和 npm 包（例如 `libs/proto`）。

```text
mavio/
├── go.work                      # 聚合全部 Go 模块
├── nx.json
├── package.json
├── pnpm-workspace.yaml
├── mise.toml
│
├── apps/
│   ├── server/                  # [Go] 服务端：cmd/mavio、依赖装配、配置、Connect 服务实现
│   ├── web/                     # [TS] Web UI，内嵌于服务端（见 clients.zh-CN.md）
│   ├── ios/ tvos/ macos/        # [Swift] iPhone 与 iPad、Apple TV、Mac（见 clients.zh-CN.md）
│   ├── android/ android-tv/     # [Kotlin] 手机与平板、Android TV（见 clients.zh-CN.md）
│   └── desktop/                 # (后期) [Go] macOS、Windows 与 Linux：MyGo、libmpv（见 clients.zh-CN.md）
│
├── libs/
│   ├── proto/                   # [Go + TS] 契约：*.proto 源文件、buf 配置、生成的 Go/TS 代码
│   ├── plugin/                  # [Go] 插件 SDK 与宿主运行时（wasm + 子进程）
│   ├── core/                    # [Go] 领域模型与仓储端口（见 domain.zh-CN.md）；不依赖任何基础设施
│   ├── store/                   # [Go] ent schema、sqlc 查询、Atlas 迁移、仓储实现
│   ├── naming/                  # [Go] 文件名 / 目录名解析
│   ├── metadata/                # [Go] NFO 读写、外部 ID、分级体系、元数据合并策略
│   ├── subtitle/                # [Go] SRT / ASS / SSA / WebVTT 字幕解析、转换与字符集检测；Subzero 的修正、语言与听障字幕识别；ftfy 的文本修复
│   ├── imaging/                 # [Go] 图片处理、拼贴、blurhash / thumbhash
│   ├── media/                   # [Go] 探测、关键帧、硬件加速、播放决策、转码规划、ffmpeg 进程守护
│   ├── library/                 # [Go] 扫描器、解析器链、任务调度
│   ├── streaming/               # [Go] HLS (CMAF) 播放列表、按需分片、分片缓存
│   ├── release/                 # [Go] release 标题解析：编号、质量、语言、版本、发布组
│   ├── acquire/                 # [Go] 资源获取引擎：档案、决策、搜索、队列、入库（movie、series）
│   ├── rclone/                  # [Go] rclone rcd：rc 客户端、远端上的存储、媒体代理、rcd 托管
│   ├── client-ts/               # [TS] Web UI 的 connect-es 客户端与 Query hooks（见 clients.zh-CN.md）
│   ├── client-rust/             # [Rust] 原生客户端的共享核心（见 clients.zh-CN.md）
│   ├── apple/                   # [Swift] Apple 各应用的共享代码（见 clients.zh-CN.md）
│   ├── android/                 # [Kotlin] Android 各应用的共享代码（见 clients.zh-CN.md）
│   └── design/                  # 所有客户端的设计 tokens 与图标（见 clients.zh-CN.md）
│
├── plugins/
│   ├── scraper-tmdb/            # [Go → wasm] TMDB 元数据刮削
│   ├── scraper-musicbrainz/     # [Go → wasm] 音乐元数据刮削
│   ├── scraper-theaudiodb/      # [Go → wasm] 音乐人简介与图片
│   ├── scraper-fanart/          # [Go → wasm] fanart.tv 图片
│   ├── scraper-openlibrary/     # [Go → wasm] Open Library 图书元数据
│   ├── subtitles/               # [Go → 子进程] 字幕提供者：OpenSubtitles.com 与 Bazarr 的其他站点
│   ├── scraper-servarr/         # [Go → wasm] Servarr 的元数据服务：SkyHook、Radarr 元数据、scene 映射、XEM
│   ├── indexers/                # [Go → 子进程] 索引器：Newznab、Torznab、Cardigann、Prowlarr 的站点
│   ├── download-clients/        # [Go → 子进程] 下载客户端：qBittorrent、Transmission、SABnzbd、NZBGet 等
│   ├── notifier-webhook/        # [Go → wasm] Webhook 通知
│   └── auth-ldap/               # [Go → 子进程] LDAP 认证（需要原生 TCP 长连接）
│
├── tools/
│   ├── nx-go/                   # 本地 Nx 插件：把 go.mod 推断为项目，并生成依赖图
│   ├── fixtures/                # 用 ffmpeg lavfi 确定性生成测试媒体（HDR10、隔行扫描、多音轨、多字幕）
│   └── testport/                # 从 Jellyfin C# 测试中提取用例并生成 testdata 的一次性工具
│
└── docs/
```

### 3.2 依赖方向
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
* `core` 不依赖任何其他库；`naming`、`subtitle`、`release` 是纯函数库。
* 第一方插件也可以使用纯函数库：字幕插件用 `release` 解析发布名。
* `acquire` 与 `library` 一样，只通过 `core` 中的端口访问存储、扫描与插件，这些端口由 `apps/server` 实现。
* `media` 只依赖 `core`，不感知数据库与插件。
* `rclone` 只依赖 `core`：它基于 rcd 的 HTTP API 实现 `core.Storage`，不链接 rclone 的任何代码。
* 只有 `apps/server` 负责装配。依赖方向由 depguard 规则在 CI 中强制。

---

## 4. 契约设计（libs/proto）

```text
libs/proto/
├── buf.yaml  buf.gen.yaml
├── mavio/
│   ├── library/v1/             # LibraryService、ItemService：媒体库、条目、媒体源与流、人员、扫描；
│                               #   MetadataService：编辑、重新识别、图片、字幕下载
│   ├── auth/v1/                # AuthService：首次运行、登录与退出、已登录设备、API 密钥
│   ├── user/v1/                # UserService、UserDataService：账户、权限、偏好、按条目的用户状态
│   ├── system/v1/              # SystemService：健康检查、服务端信息、设置、插件、文件夹、日志；
│                               #   TaskService、ActivityService、BackupService、LocalizationService
│   ├── playback/v1/            # PlaybackService：客户端能力、播放决策、进度上报
│   └── plugin/v1/              # 插件契约：PluginService（manifest、配置、生命周期）、
│                               #   MetadataProviderService、SubtitleProviderService、MediaSegmentProviderService、
│                               #   AuthProviderService、NotifierService
├── gen/go/                     # 生成的 protobuf-go + connect-go（Go 模块）
└── gen/ts/                     # 生成的 protobuf-es + connect-es（npm 包）
```

* 客户端能力用 Mavio 自己的 `ClientCapabilities` 表达（容器、编解码器、等级、HDR 能力、字幕交付方式、带宽），替代 Jellyfin 的 `DeviceProfile`。Go 类型位于 `libs/media/decision`，该包不依赖 `libs/proto`；`playback/v1` 中的消息与之对应，由 `apps/server` 负责二者转换。移植 StreamBuilder 测试时把 Jellyfin 的 profile 转换成该类型。
* 媒体流（直放、HLS 播放列表与分片、图片）不经过 Connect，走普通 HTTP，以便利用 Range、缓存与 CDN 语义。上传所选图片（至多 20 MiB）是 `MetadataService.SetItemImage` 的一个字段。
* API 消息与领域模型（[领域模型](domain.zh-CN.md)）对应，但从不暴露图片路径、密码哈希等文件系统与内部细节。
* `plugin/v1` 对插件作者发布，自成一体：它定义自己的精简消息（`Lookup`、`Metadata`、`PersonCredit`、`RemoteImage` 等），而不引用 `library/v1`，两份契约可以各自演进。

---

## 5. 存储层（libs/store）：ent + sqlc，双数据库

### 5.1 职责划分
| ent 负责 | sqlc 负责 |
| :--- | :--- |
| **Schema 的唯一来源**（`internal/ent/schema`） | ent 无法以可移植方式表达的方言专属 SQL：任务队列（`INSERT … ON CONFLICT … WHERE` 对待执行任务去重；原子租用，从不同时执行同一唯一 key 的两个任务，PostgreSQL 上用 `FOR UPDATE SKIP LOCKED`） |
| 增删改查、批量 upsert（`CreateBulk` + `ON CONFLICT`）、关系遍历 | |
| 动态条目查询：可选过滤条件、递归后代（两种方言都支持的递归 CTE）、关联用户数据、排序、分页 | |

条目查询用 ent 而不用 sqlc 构建，因为它的过滤条件与排序方式是动态组合的；用带可选参数的静态 SQL 会让查询计划失效。sqlc 查询按方言各写一份，因此只用于上述场景（预算 ≤ 30 条）。

### 5.2 生成流水线
```text
ent schema ──go generate──────────────────────▶ internal/ent（生成的客户端）
     │
     └──migrategen -dialect sqlite / postgres──▶ migrations/<dialect>/*.sql + atlas.sum
                                                        │
                                   sqlc generate ◀──────┘（把迁移当作 schema 读入）
                                        │
                                        ▼
                         internal/sqlcsqlite、internal/sqlcpg
```
* `internal/cmd/migrategen` 生成版本化迁移：在空的开发数据库（内存 SQLite，或临时的 PostgreSQL 容器）上重放已有迁移目录，用 Atlas 与 ent schema 比较差异，并把差异写成 Atlas 格式的新迁移。SQLite 迁移使用标准的双引号标识符，这是 sqlc 解析器的要求。
* 两套迁移目录各自版本化，提交前人工审核；已合并的迁移不得修改。
* `migrategen` 会删除已移除的列和索引，并跳过仅由 PostgreSQL 规范化部分索引谓词而产生的索引变更。
* 服务端启动时（`store.Open`）执行待应用的迁移：每个文件在独立事务中执行，并记录到 `schema_migrations`。重建表的 SQLite 迁移在其连接上关闭外键约束后执行（该 pragma 在事务内无效），并在提交前用 `PRAGMA foreign_key_check` 校验。
* 派生的搜索键与排序键带有版本号（`keysVersion`）；版本变化时，`store.Open` 会对所有已有行重新计算一次键，并把该版本记录到 `schema_migrations`。

### 5.3 双方言的做法
* **统一事务**：`Store.InTx` 开启一个 `*sql.Tx`，把 ent 客户端（通过嵌套事务为空操作的驱动）和 sqlc 查询都绑定到它上面，因此一个事务内可以任意组合各个仓储。
* **统一类型**：`core.ID` 实现了 `sql.Scanner` / `driver.Valuer`，在 PostgreSQL 中存为 `uuid`、在 SQLite 中存为文本；sqlc 的 `overrides` 把它和可空列映射为两个包中相同的 Go 类型，因此 PostgreSQL 适配层可以直接转换结构体。
* **多值属性**（流派、标签、工作室、艺人、专辑艺人）存放在 `item_values` 表中，使"匹配任意一个"的过滤可移植且可索引。
* **搜索与排序**使用在 Go 中计算、以 `LIKE` 匹配的键。`search_key`（条目、人员）与 `value_key`（条目值）保存 Jellyfin 的名称清洗形式（`cleanValue`：去除变音符号、转小写、标点视为空格、全角转半角），用于匹配、相关度与分组；`original_key` 保存转小写的原始标题，供原始搜索模式匹配。`sort_key` 保存排序名的 Jellyfin 排序形式（去掉冠词和标点、数字补零、非拉丁文字用 go-unidecode 转写），既用于名称排序，也用于匹配搜索词的排序形式。见领域模型 §9.2。
* **不区分大小写的唯一性**（用户名）通过带唯一索引的小写 `name_key` 列实现。
* **时间**以微秒精度存储（PostgreSQL 用 `timestamptz`，SQLite 用整数微秒），读出时为 UTC。
* **仓储端口**：`store` 只暴露 `core` 中定义的接口；ent 与 sqlc 的类型不会离开本包。
* **一致性测试**：同一套仓储测试分别跑在 SQLite（临时文件）与 PostgreSQL（由 `libs/store/pgtest` 启动的 testcontainers-go 容器，或 `MAVIO_TEST_POSTGRES_DSN`）上，两者都通过才算通过。

### 5.4 SQLite 运行参数
WAL 模式、`synchronous=NORMAL`、`busy_timeout`、外键开启。单连接的写池执行写入与事务，并使用 `_txlock=immediate`（事务开始时即获取写锁，避免锁升级死锁）；另一个连接池负责读取。时间值使用 `_timefmt=unixepoch_micro`。关闭时执行 `PRAGMA optimize`。

---

## 6. 图像处理（libs/imaging）

### 6.1 需求与实现方案
需求来源：Jellyfin 的 `src/Jellyfin.Drawing.Skia`、`MediaBrowser.Controller/Drawing/ImageProcessingOptions.cs`、Trickplay 与 Photos 模块。

| 能力 | Mavio 实现 |
| :--- | :--- |
| 解码 JPEG / PNG / GIF / BMP | Go 标准库 + `x/image` |
| 解码 WebP | `gen2brain/vpx`（libwebp 的纯 Go 移植） |
| 解码 AVIF / HEIC / JPEG XL | `gen2brain/avif`、`heic`、`jpegxl`（编译为 WASM，运行在 wazero 上） |
| 编码 WebP / JPEG / PNG | WebP 用 `gen2brain/vpx`，JPEG 与 PNG 用标准库 |
| 编码 AVIF（可选） | `gen2brain/avif` |
| 缩放 / 裁剪 / 填充（Width、Height、Max*、Fill*） | `x/image/draw` CatmullRom / ApproxBiLinear；大图先按 2 的幂降采样 |
| EXIF 方向自动校正 | 轻量 EXIF 解析（仅 Orientation 等少量标签） |
| 锐化、模糊、背景色、前景层 | 自写卷积 / 高斯模糊（可并行分块）+ `image/draw` 合成 |
| 未播放数 / 播放进度角标 | 由客户端渲染 |
| 媒体库、合集与播放列表的拼贴图 | `image/draw` 合成至多四张图片（`Collage`），每张铺满其格子；不含文字，由客户端为卡片标注名称 |
| 启动屏（含文字） | `image/draw` 合成 + `go-text/typesetting` 排版，内置 Noto 字体子集覆盖 CJK |
| Trickplay 缩略图拼图 | ffmpeg：`fps` + `scale` + `tile` 滤镜一步输出拼图，可走硬件解码 |
| 视频截图、章节图 | ffmpeg |
| Blurhash / Thumbhash | 纯 Go（`bbrks/go-blurhash`、`go.n16f.net/thumbhash`）：blurhash 按 Jellyfin 的分量数在 32px 缩略图上计算，thumbhash 在 100px 缩略图上计算 |
| SVG | 检查外部引用（`CheckSVG`：href、CSS `url()` 与 `@import`、嵌套 data URI、外部实体或实体爆炸）后原样下发给客户端；不做栅格化 |
| 读取图片尺寸 | `image.DecodeConfig` |

### 6.2 性能策略
媒体服务器的图像负载是"海报级"的：单张图片小、处理结果可缓存、调用模式是"一次处理，多次命中"。性能主要靠架构保证：
1. **派生图缓存**：按（源文件内容哈希、处理参数）做内容寻址的派生图缓存，并用 `singleflight` 合并对同一张图的并发请求。
2. **扫描时预生成**：扫描阶段预生成常用尺寸，请求路径上基本只读缓存。
3. **视频相关的图像全部交给 ffmpeg**：Trickplay、截图、章节图都在 ffmpeg 内完成，并可用硬件解码。
4. **兜底**：若纯 Go 缩放处理大图过慢，大图改用 ffmpeg 的 `scale` / `zscale` 处理。

---

## 7. 插件系统（libs/plugin）

本章介绍运行时以及服务端调用的插件；插件反过来能接触什么——宿主 API、事件、HTTP 路由、任务、远程设备与更多提供者类型——以及 DLNA 插件，见[插件平台](plugins.zh-CN.md)。

### 7.1 两种运行时，一套契约
| | WASM 插件（wazero） | 子进程插件（UDS） |
| :--- | :--- | :--- |
| 适用场景 | 元数据刮削、歌词 / 字幕提供者、通知、命名规则扩展等"请求-响应"型、轻量插件 | 需要原生网络协议（LDAP）、长连接、重计算、原生库或自带 HTTP 服务的插件 |
| 分发形态 | 单个 `.wasm` 文件，一次构建跨所有平台 | 每个平台一个二进制 |
| 隔离方式 | 内存沙箱；能力完全由宿主授予 | 操作系统进程隔离；可配合 cgroup / rlimit |
| 调用开销 | 进程内函数调用 + 一次内存拷贝 | 一次 UDS 往返 |
| 编写语言 | Go（`GOOS=wasip1` + `go:wasmexport`）、TinyGo、Rust 等 | 任何能实现 Connect / gRPC 服务的语言 |

两种运行时都实现 `libs/proto/mavio/plugin/v1` 中定义的同一组服务（`MetadataProvider`、`AuthProvider`、`Notifier`……）。宿主侧通过统一的 `plugin.Host` 接口调用，不感知插件具体跑在哪种运行时。

### 7.2 WASM 运行时
* **调用即 Connect 请求**：宿主的 Connect 客户端使用一个特殊的 HTTP transport：它不走网络连接，而是把请求（路径、头、body，HTTP 路由还有方法与查询字符串）编码成信封写入插件内存，再调用模块导出的 `mavio_call(ptr, len) -> (ptr << 32 | len)`；`mavio_alloc` / `mavio_free` 管理共享缓冲区。插件内部由 `guest/wasm` 把请求交给插件注册的标准 Connect handler，并返回响应信封（状态码、头、body）。因此两种运行时的插件代码完全相同。
* **构建模式**：插件是 WASI reactor（`GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared`）；宿主为每个实例执行一次 `_initialize`。
* **宿主函数**（模块 `mavio`）：
  * `http_fetch(ptr, len) -> (handle << 32 | len)`：若 manifest 的 `http_hosts` 允许目标主机，由宿主代为执行 HTTP 请求；`http_read(handle, ptr)` 把结果拷贝进插件内存。分两步是为了避免在宿主函数中回调插件，Go 的 wasip1 运行时不支持这种重入。插件侧 SDK 把它们封装成普通的 `*http.Client`（`guest.HTTPClient()`）。发往宿主 API（[插件平台 §3](plugins.zh-CN.md#3-宿主-api)）所在的 `mavio.host` 的请求则在进程内由服务端的处理器树处理（`guest.HostClient()`）。
  * 插件把日志写到 stderr（以及 stdout），由宿主转发到自己的日志。
  * 配置通过 `Configure` RPC 下发，而不是宿主函数。
* **文件系统**：manifest 中 `read_paths` 列出的目录以只读方式挂载在相同路径，插件的数据目录以可写方式挂载在 `/data`；除此之外不挂载任何目录。
* **实例**：每个插件有自己的 wazero 运行时和一个模块实例池（实例数即并发调用数）。调用中发生 trap、panic、退出或超时的实例会被丢弃并替换，因此失败的调用不会影响宿主或后续调用。插件接受的配置会在每个实例的下一次调用前重放。
* **资源限制**：每个实例的内存页上限；每次调用的超时（`WithCloseOnContextDone`）；编译缓存（`CompilationCache`）持久化到磁盘，因为编译模块的耗时远大于实例化。
* **约束**：`wasip1` 下的 Go 没有套接字，所有网络访问都经过 `http_fetch`。

### 7.3 子进程运行时
* **统一使用 Unix Domain Socket**：Linux / macOS 原生支持，Windows 10 1803 起支持 `AF_UNIX`。socket 位于一个权限为 0700 的私有目录中，放在较短的基础路径下，因为 socket 路径长度限制在 100 字节左右。
* **握手**：宿主通过 `MAVIO_PLUGIN_SOCKET` / `MAVIO_PLUGIN_TOKEN` 传入 socket 路径与一次性 token。插件开始监听后在 stdout 输出一行 JSON（`{"mavio_plugin":1,"plugin_id":…}`）；宿主检查协议版本与插件 ID。此后每个请求都以 `Authorization: Bearer …` 携带 token。宿主 API 在同一目录中的第二个 socket（`MAVIO_HOST_SOCKET`）上提供，要求相同的 token。
* **通信**：在 socket 上跑 Connect（HTTP/2 h2c），直接复用 `libs/proto` 生成的 handler / client。
* **监督**：stdout 与 stderr 转发到宿主日志；定期调用 `Health` RPC（连续三次失败即结束进程）；进程退出后，监督者以指数退避（1 秒起翻倍，最长一分钟）重启插件，期间的调用会等待插件恢复。
* **关闭**：`Close` 先发送 `Shutdown` RPC（插件侧 SDK 随后退出），然后 SIGTERM，最后 SIGKILL。在 Linux 上，宿主进程意外退出时内核也会结束插件（`Pdeathsig`）。
* **权限**：子进程插件拥有操作系统层面的网络与文件访问能力；`http_hosts` 与 `read_paths` 只由 WASM 运行时强制执行，因此不需要这些能力的插件应以 WASM 形式发布。
* **热插拔**：安装、更新、卸载插件都不需要重启宿主。

### 7.4 插件 SDK 结构
```text
libs/plugin/
├── manifest/        # manifest.json 加载与校验、主机权限、配置 JSON Schema 校验
├── host/            # 宿主侧：Open（任一运行时，并与 Describe 结果核对）、Configure、Plugin 接口
│   ├── wasm/        # wazero 运行时、实例池、http_fetch / http_read 宿主函数
│   └── process/     # 子进程、socket 握手、token、监督与重启
├── guest/           # 插件侧：Handle（注册 Connect handler）、HTTPClient、HostClient
│   ├── wasm/        # //go:build wasip1：模块导出、经宿主访问 HTTP
│   └── process/     # Serve：监听 socket、认证、握手、收到 Shutdown 后退出
├── internal/        # abi（WASM 信封）、proc（子进程协议）、clients、testplugin
└── schema/          # （后期）由 Go 结构体生成配置表单用的 JSON Schema
```
插件在 `init` 中用 `guest.Handle(pluginv1connect.New…ServiceHandler(impl))` 注册 handler；`wasip1` 构建导入 `guest/wasm`，原生构建调用 `process.Serve`。`internal/testplugin` 是同时为两种运行时构建的完整示例。

### 7.5 服务端中的插件
* **插件目录**：服务端通过 `libs/plugin/host` 启动插件目录（`--plugin-dir`）中每个含有 `manifest.json` 的文件夹。无法读取的文件夹、重复的插件 ID 或启动失败的插件会被报告为失败并跳过，服务端照常运行。在此之前，种子目录（`--plugin-seed-dir`，例如容器镜像附带的插件）中的插件会被复制到插件目录，每个只复制一次：插件目录中的标记文件记录已播种的插件，因此被卸载或升级的插件保持原样，插件目录中已有的插件不会被复制。
* **配置**：管理员设置的配置按插件 ID 保存（[领域模型](domain.zh-CN.md) §10），通过 `SystemService.SetPluginConfig` 设置：先按清单的 `config_schema` 校验，再通过 `Configure` RPC 下发，插件接受后才保存，无需重启。启动时每个插件会重新收到已保存的配置。
* **状态**：插件以其配置运行、或其 schema 接受空配置时为就绪；在等待其 schema 所要求的配置时为未配置；无法启动或拒绝已保存的配置时为失败。`SystemService.ListPlugins` 连同清单一起报告这些状态。
* **元数据提供者**：每个已启动的元数据插件都是媒体库刷新的提供者；未就绪时它不提供任何信息，因此之后才配置的插件会参与下一次刷新。只提供图片的提供者（如 fanart.tv）凭靠前的提供者找到的 ID 查找图片，靠后的提供者能看到这些 ID。
* **热插拔**：`SystemService.InstallPlugin` 从插件目录安装插件，或将其升级、降级；`UninstallPlugin` 卸载插件；期间服务端照常服务。安装时下载该版本的 zip 包，按插件目录核对其 SHA-256，解压到插件文件夹中的隐藏文件夹并检查清单；随后停止正在运行的版本，将其文件夹移到一旁，移入新版本并以已保存的配置启动；若新版本无法启动，则恢复之前的版本。卸载会停止插件并删除其文件夹与配置。各项能力在使用时查找，因此插件一旦就绪，刷新、搜索与通知即会用到它。
* **插件目录**：插件目录是服务端设置中某个 http 或 https URL 上的 JSON 文档，列出插件及其版本：版本号、API 版本、运行时（进程插件还有 os 与 arch）、相对于目录的包 URL、SHA-256、更新说明与发布时间。`ListCatalogPlugins` 给出服务端能运行的版本（最新的在前）以及已安装的版本；同一插件以最先列出它的目录为准，无法访问的目录被跳过。
* **宿主 API**：每个插件的宿主 API 请求带着该插件的授权（`auth.PluginHandler`）进入处理器树；认证拦截器按清单的 `permissions.api` 检查请求，并解析 `Mavio-User`（[插件平台 §3](plugins.zh-CN.md#3-宿主-api)）。
* **认证与通知**：`AuthProvider` 指向某个插件的用户，在该插件的 `Authenticate` 接受其用户名与密码时登录。活动日志中的每项活动都作为 `Event` 发送给通知插件。消费事件的插件通过各自的队列，分批收到其清单列出的事件，无论是否进入日志（[插件平台 §4](plugins.zh-CN.md#4-事件)）。
* **任务与数据目录**：插件的任务作为 `plugin.task` 作业在单独的 worker 上运行，并出现在 `TaskService` 中；每个插件在 `--plugin-data-dir` 中有一个数据目录（[插件平台 §6](plugins.zh-CN.md#6-任务与数据目录)）。
* **片段提供者**：对每个已探测的电影与剧集，以其 ID、时长与章节询问每个已启动的片段插件。
* **字幕提供者**：`MetadataService.SearchSubtitles` 以视频的 OpenSubtitles 哈希（`subtitle.FileHash`）、名称与 ID 搜索每个已启动的字幕插件；未就绪的插件什么也找不到。

---

## 8. 媒体管线（libs/media）与硬件加速

### 8.1 子包划分
```text
libs/media/
├── probe/        # 调用 ffprobe（JSON 输出），规范化为 core 中的 MediaInfo（视频、音频、字幕流、HDR10 / HDR10+ / DV 元数据）
├── keyframes/    # 关键帧提取（Matroska Cues 和 MP4 索引，否则 ffprobe 读包级关键帧标志），用于按关键帧对齐切分 HLS 分片
├── hwaccel/      # 硬件能力探测：-hwaccels / -encoders 列表 + 试编码验证，输出 Capabilities
├── decision/     # 播放决策：MediaInfo × ClientCapabilities → DirectPlay / Remux / Transcode（逐流决定）
├── planner/      # 转码规划：决策结果 × 硬件能力 → 滤镜图 IR → ffmpeg 命令行参数
└── supervisor/   # ffmpeg 进程守护：context 取消、空闲回收、节流、分片产出监听、进度解析
```

### 8.2 设计要点
* **`decision` 与 `planner` 都是纯函数**：不读文件、不启动进程，便于用移植来的测试做表驱动验证。
* **滤镜图中间表示（IR）**：`planner` 先产出结构化的滤镜图（节点为 decode / hwupload / scale / tonemap / overlay / encode，带上各自的像素格式与硬件上下文），再序列化为 ffmpeg 参数。测试可以对 IR 断言，而不是对脆弱的命令行字符串断言。
* **按硬件厂商拆分**：`planner` 内按 Intel、NVIDIA、AMD、Apple、Rockchip、V4L2、软件编码各自实现策略，避免 Jellyfin `EncodingHelper.cs`（8000+ 行）式的单文件膨胀。移植时以行为与测试为准重新组织结构。
* **`supervisor`**：每个转码会话一个 goroutine 组；客户端断开或空闲超时后终止 ffmpeg；按客户端播放进度节流；用 `testing/synctest` 编写确定性的超时测试。
* **黑边**：低优先级任务（`media.borders`，`libs/media/borders`）用 ffmpeg 把每个已探测视频的五帧采样为 8-bit 亮度，并用 `libs/imaging` 的检测器测量（对行与列采样，支持 8-bit 与 10-bit P010 平面）；只有每个非全黑样本都有的黑边才计入，且至少占画面的 2%，取偶数像素，记录在视频流上（`core.MediaStream.Crop`）。转码重新编码视频、且视频未旋转、未烧录字幕时，规划器在缩放之前裁掉黑边（服务端设置 `CropBlackBorders`），使宽银幕电影不必为黑边耗费码率；直接复制的视频从不裁剪。缩略图拼图与章节图片同样裁去黑边。
* **杜比视界 Profile 7**：独立的增强层轨道只根据明确的信令识别，即杜比视界配置有增强层而没有基础层（`core.MediaStream.IsDolbyVisionEnhancement`），从不依据轨道顺序、分辨率或名称。只要文件还有其他视频轨道，流选择就不会选中这样的轨道，因此播放、转码与复制都使用基础层。单轨 Profile 7 流交给声明支持带增强层杜比视界的客户端；对只支持 HDR10 的客户端，规划器去掉杜比视界元数据，保留兼容 HDR10 的基础层，为 SDR 客户端重新编码时进行色调映射（`libs/media/planner/hdr.go`，沿用 Jellyfin 的规则）。
* **立体声下混**：规划器带有 Jellyfin 的立体声下混算法（`libs/media/planner/audio.go`：Dave750、夜间对白模式、RFC 7845、AC-4，以及中置声道满电平的 ITU-R BS.775，7.1 先折叠为 5.1）；未指定算法时使用 ffmpeg 的下混。下混后的音频按服务端的下混增益放大（`DownmixBoost`，默认 2，与 Jellyfin 相同），使立体声设备上的对白清晰可闻。

### 8.3 硬件加速支持矩阵
| 平台 | 解码 / 编码 | 色调映射 | 字幕烧录 | 说明 |
| :--- | :--- | :--- | :--- | :--- |
| **Intel** QSV / VA-API | 全硬件、零拷贝 | `vpp_qsv` / `tonemap_vaapi` / OpenCL | `overlay_qsv` / `overlay_vaapi` | NAS 与迷你主机的主力平台，第一优先级 |
| **NVIDIA** NVENC / NVDEC | CUDA 显存管线 | `tonemap_cuda` | `overlay_cuda` | |
| **AMD** AMF / VA-API | Windows 用 AMF（D3D11），Linux 用 VA-API（radeonsi） | OpenCL / Vulkan | `overlay_vaapi` / OpenCL | |
| **Apple** VideoToolbox | 硬件 H.264 / HEVC | `tonemap_videotoolbox`（Metal） | `overlay_videotoolbox` | macOS 本机运行 |
| **Rockchip** RKMPP | 硬件解码与编码 | RGA / OpenCL | RGA overlay | ARM NAS 与开发板（RK3588 等） |
| **V4L2** M2M | 解码 / 编码 | — | — | 树莓派等，能力有限，以 remux 为主 |
| **软件** | libx264 / libx265 / SVT-AV1 | `zscale` + `tonemapx` | `subtitles` / `overlay` | 兜底 |

支持的操作系统：Linux（第一公民，含容器部署与 `/dev/dri`、`/dev/nvidia*` 设备映射）、Windows 10/11、macOS（Apple Silicon / Intel）。

---

## 9. 媒体库扫描与变更检测（libs/library）

Mavio 通过**全量对账扫描**发现媒体库变更，本地磁盘与网络文件系统（SMB / NFS）使用同一套机制：

* **触发时机**：服务启动时、按媒体库配置的周期定时执行、管理员手动触发、通过 API 触发（例如下载工具完成后回调）。
* **目录 mtime 剪枝**：数据库记录每个目录的 mtime 与 inode（Windows 上为文件 ID）；两者都没变的目录跳过 `readdir`。新增、删除、重命名文件都会更新父目录的 mtime，所以剪枝不会漏掉这类变更。只修改文件内容而不改变目录项的情况，靠文件自身的 size + mtime 比对发现，因此对未剪枝的目录仍要 stat 其中的文件；需要绝对保险时，可按媒体库配置周期性关闭剪枝做一次完整对账。
* **逐个文件夹解析**：解析器根据文件夹在媒体库中的位置（媒体库类型，以及它是否位于剧集、季或艺术家之下），把文件夹及其条目映射为：文件夹本身所代表的条目（自成文件夹的电影、光盘镜像、剧集、季、专辑、有声书）、由其中文件构成的条目，以及接下来要解析的子文件夹及其位置。规则遵循 Jellyfin 的命名与解析规则；附加内容从所属条目的文件夹及其附加内容文件夹中查找。解析器插件（`library.FolderResolver`，[plugins.zh-CN.md](plugins.zh-CN.md) §8.6）先剔除条目，并可先于内置解析器认领文件夹；解析器失败时该文件夹按不可读处理，保留其条目。由于文件夹的结果只取决于其列表与位置，未变化的文件夹无需重新解析。
* **对账只读元数据**：对账遍历目录、比对文件 stat，只把新增或变化的文件送去 ffprobe 和刮削。
* **并发**：按媒体库根目录和目录子树并发遍历（`errgroup.SetLimit`），网络文件系统上使用较低的并发度。
* **任务**：扫描、探测与元数据刷新都是持久化任务，由租用它们的 worker 执行（`library.scan`、`media.probe`、`media.keyframes`、`item.refresh`、`image.placeholders`）。扫描为每个新增或变化的媒体文件排入一次探测，并为每个新增的无媒体条目（如剧集、季或专辑）排入一次元数据刷新；扫描还把每个视频旁的字幕文件作为其外部流附加上去（§9.1）。对视频的探测还会排入一次低优先级的黑边检测（`media.borders`），转码时据此裁剪。随后按媒体库的设置排入媒体附加内容（`libs/media/thumbnails`，在黑边检测之后）：缩略图拼图（`media.trickplay`：ffmpeg 以 fps、crop、scale 与 tile 滤镜一次完成，每张缩略图用一个关键帧，通过 Matroska 文件的索引读取，否则从文件中只解码关键帧；宽 320 像素，每 10 秒一张，每张拼图 10 × 10，先写在元数据目录的旁边再换入；[优化 §2.5](optimization.zh-CN.md#25-用关键帧生成-trickplay)）、章节图片（`media.chapters`：每个章节一帧，宽 640 像素，第一个章节取开头稍后处）、由片段提供者插件给出的电影与剧集的媒体片段（`media.segments`；两个提供者的同类片段重叠时取前者），以及音轨的响度（`media.loudness`：ffmpeg `ebur128` 的积分响度；专辑在其所有音轨都测得后随之计算）。附加内容会等待静默闸门；某文件的图片或测量失败后，一天内不再读取（`FailNotes`，在内存中，至多 4,096 条）。`library.extras` 为某媒体库的每个条目排入这些任务。探测排入一次元数据刷新；对视频，它从容器索引读取 HLS 切分直接复制的视频所需的关键帧，没有索引的文件在存储类别允许时排入一次读取每个包的低优先级关键帧提取（[优化 §2.4](optimization.zh-CN.md#24-从容器索引读取关键帧)）。刷新先应用提供者的元数据，再应用本地元数据插件的（`library.LocalReader`），再应用本地 NFO 文件，然后由元数据处理器插件调整结果（`library.Processor`），不改动已锁定的字段，并为得到的分级计算分数（`metadata.RatingScore`）。随后由图片生成器插件（`library.ImageGenerator`）生成所有来源都未提供的图片类型，保存在元数据目录中。媒体旁的图片按 Jellyfin 的本地图片命名查找（`poster`、`folder`、`<文件名>-poster`、`fanart-1`、`season01-poster`、`<单集>-thumb` 等；多个视频共用的文件夹中只认以文件名为前缀的名称），优先于 NFO 文件指定的图片；照片以自身为主图，由图片路由提供并渲染；每种图片取自拥有它的最可信来源。再次找到的图片保留其 ID、尺寸与占位图，随后由占位图任务测量新图片并计算其 blurhash 与 thumbhash。对于不保存本地元数据的媒体库，管理员为条目选择的图片保存在元数据目录（`--metadata-dir`，`<id[:2]>/<id>/<种类>.<扩展名>`），优先于其他所有图片。在保存本地元数据的媒体库中，刷新会写出条目的 NFO 文件（`metadata.WriteNFO`，Kodi 格式加 Jellyfin 的锁定元素；覆盖已找到的 NFO 文件，否则电影与视频写为旁边的 `<文件名>.nfo`，其他按 Jellyfin 的命名）与元数据保存器插件的文件（`library.Saver`），并把提供者的图片按本地图片命名保存，替换同名但格式不同的文件；季、音轨与照片的图片仍留在远程。定时扫描在媒体库的扫描间隔后排入下一次；按需扫描与之并行排队，同一媒体库的扫描不会重叠。文件夹扫描（`library.scan_folder`）由入库为其写入的文件夹排入，对账该文件夹及其下的文件夹，以及从媒体库文件夹到它的各级上层文件夹，不标记任何缺失，留给该媒体库的下一次扫描。
* **存储感知**：扫描与探测对机械硬盘和远程卷的读取串行化，在客户端拉流时等待，并把仍在写入的文件留到稳定后再处理（[优化 §1](optimization.zh-CN.md)）。
* **存储端口**：所有读写媒体库文件的地方（扫描、本地元数据与图片、外挂文件、歌词、字幕下载、NFO 与图片写入、直接播放、导入与回收站）都经由 `core.Storage`，由 `library.Storages` 为媒体库路径打开：本地目录基于 `os.Root`，rclone 远端经由 `Storages.Remote`。媒体工具接收 `library.MediaFile`：ffmpeg 与 ffprobe 读取其 URL（本地为 `file:` URL），仅本地文件带有操作系统路径，用于预读和直接读取容器索引。导入时，本地文件在文件系统上移动和硬链接；远端在自身内部移动或复制文件（`core.StorageTransfers`）；其余情况经服务端复制，移动时再删除源文件。
* **一致性**：每次扫描在数据库中记一个"扫描代次"；扫描完成后，未被本代次看到的条目标记为缺失（先软删除，宽限期后再清理），避免挂载点暂时不可用导致整库被删。

### 9.1 外挂字幕与打分

字幕文件命名五花八门，中文字幕尤甚。`libs/subtitle` 归一化语言别名，并为字幕文件相对于视频打分。

归一化为 `zh` 的别名：

```text
"zh", "chi", "zho", "chs", "cht", "sc", "tc", "gb", "big5",
"zh-hans", "zh-hant", "zh-cn", "zh-tw", "zh-hk",
"简体", "繁體", "繁体", "简中", "繁中", "简", "繁", "中文", "双语" ──▶ "zh"
```

打分：

| 匹配特征 | 得分 | 示例 |
| :--- | :--- | :--- |
| 与视频同名 | `+1000` | `Movie.mkv` 与 `Movie.srt` |
| 以视频名加分隔符为前缀 | `+500` | `Movie.mkv` 与 `Movie.zh.srt` |
| 偏好语言 | `+200` | 用户偏好的字幕语言 |
| 其他语言 | `-100` | |
| 强制字幕 | `-150` | `.forced`：对白很少 |
| 听障字幕 | `-50` | `.sdh`、`.cc`、`.hi` |
| 格式 | ASS/SSA `+20`，SRT `+10` | 样式更丰富者优先 |

* **外挂字幕发现**（`libs/library`）：扫描把视频旁的字幕文件作为其外部流附加上去：文件名与视频相同，或以视频名加分隔符开头（基名得分至少 500）。语言、标题以及默认、强制、听障标记取自文件名，可识别中文变体（`zh-Hans`、`zh-Hant`、`chi`）；三字母代码只认同时有两字母代码的语言，使 `sdh` 等标记仍作为标记。它们按不含用户语言的得分排序，编号排在内嵌流之后；之后增删的文件无需重新探测视频即可更新这些流，重新探测时也会保留它们。字幕文件也在设置指定的字幕目录中查找：视频所在目录下的 `relative` 目录，其变化会使视频所在目录被视为已变化；以及元数据目录中该条目的 `metadata` 目录，在扫描保存视频所在目录时读取。播放与字幕服务按扫描记录的路径打开外挂字幕，也可以在媒体库之外。
* **播放选择**（`libs/media/decision`）：用户偏好的字幕语言按规范化后的别名匹配，因此偏好中文时可找到简体、繁体与地区变体，听障字幕排名较低。

### 9.2 rclone 远端（libs/rclone）

媒体库路径 `rclone:<remote>:<path>` 位于 rclone 的某个远端上，经 `rclone rcd` 的 HTTP API（rc API）访问；服务端不链接 rclone 的任何代码，rcd 可以运行在其他机器上。

* **托管的 rcd**：默认由服务端自己运行 `rclone rcd`（`rclone` 设置可执行文件；找不到时远端不可用，服务端其余部分照常工作）。它监听私有目录中的 Unix 套接字，用户名和密码在每次启动时随机生成；以 `rc/noopauth` 判断就绪；退出后延迟重启，延迟逐次翻倍、最长一分钟，运行满一分钟后重置；关闭时先请求其退出（`core/quit`），再终止，最后强制结束。它的日志按级别写入服务端日志。
* **外部 rcd**：`rclone-url` 连同 `rclone-user` 与 `rclone-pass` 连接不由服务端运行的 rcd，只允许 HTTPS、Unix 套接字，或仅限本机回环接口的 HTTP，因此凭证不会以明文穿过网络。
* **配置**：托管的 rcd 把远端保存在 `<home>/rclone/rclone.conf`。该文件由服务端创建时会被加密，随机口令保存在旁边的 `rclone.pass`（权限 0600），并以 `RCLONE_CONFIG_PASS` 交给 rcd；不是服务端创建的文件按原样读取。`mavio.storage.v1.StorageService`（仅管理员）列出提供者及其选项，供客户端生成表单（`config/providers`）；创建、更新与删除远端；回答 rclone 非交互式配置提出的问题，包括 OAuth（`config/create`、带 `continue` 的 `config/update`）；并列出目录供选择媒体库路径。列表中不含密码、敏感选项与令牌。
* **存储**：`rclone.Remotes` 为远端上的目录打开 `core.Storage`：列目录与 stat（`operations/list`、`operations/stat`，以后端的原始 ID 作为文件 ID）、上传（`operations/uploadfile`，以 multipart 流式发送）、目录、删除，以及在 rcd 内部的移动与复制（`operations/movefile`、`operations/copyfile`，后端支持时在服务端完成；能力来自 `operations/fsinfo`，缓存一分钟）。耗时操作作为 rc 任务运行（`_async`）并轮询；rcd 重启后不再认识的任务以 `rclone.ErrJobLost` 失败。运行中的传输记录在存储任务台账（`core.StorageJob`）中：服务端重启后再次请求同一传输（例如重试导入）时，会等待仍在运行的任务，而不是另起一个；已经完成的不会再运行。启动时，台账丢弃已结束或 rcd 已遗忘的任务。远端无法设置修改时间，调用方会跳过这一步。
* **媒体字节**：读取是对 rcd 的 `--rc-serve` 发起的 HTTP Range 请求。ffmpeg 与 ffprobe 经服务端在本机回环接口上运行的代理读取：每个媒体 URL 带一个随机令牌（最后一次使用后保留十二小时），代理把它映射到对象并加上 rcd 的凭证，因此凭证不会出现在 ffmpeg 的命令行上；代理流式转发响应，并转发 Range 与条件请求头。远端的每个目录都从远端根目录命名其对象，使它们在 rcd 中共用一个文件系统，rcd 保留它一小时。一次读取保存第一个响应的版本（ETag），之后的每个请求都要求该版本，文件已变化时以 `rclone.ErrChanged` 失败。`cloud` 存储类别的目录经本地磁盘上的块缓存读取（`rclone.BlockCache`，位于 `<cache>/rclone/blocks`，大小由 `rclone-cache-size` 设定），`Storage.Open` 与代理都从它读取，并按读取的 `core.ReadIntent` 预读；播放按其关键帧在文件中的位置向它预取（[优化 §3](optimization.zh-CN.md#3-通过-rclone-读取远端存储reading-remote-storage-through-rclone)）。
* **离线下载**：rclone 后端能自行下载链接的云盘（mavioai/rclone 的 `operations/addurl`、`listoffline` 与 `deleteoffline`，如 PikPak 与 115）是服务端内置的下载客户端（`downloads.Offline`，提供者 `dev.mavio.rclone-offline`），通过 `StorageService.ListOfflineClients` 与 `SetOfflineClients` 配置。一个客户端对应远端上的一个目录；电影的磁力链接下载到其中的 `movies` 目录，剧集下载到 `series`，下载以种子的 info hash 标识。台账保存每个下载的任务、标题、分类与最近一次看到的状态，据此把服务端发起的下载与云盘上的其他下载区分开，云盘不再列出的下载也能保留。已完成下载的输出路径位于远端上，因此导入到同一远端上的媒体库时，文件在远端内部移动。
* **从远端导入**：导入经存储读取远端上的下载目录，经媒体代理探测其中的文件，由远端移动或复制它们（`core.StorageTransfers`），并以 `operations/purge` 删除清空后的下载目录。

---

## 10. HLS 流媒体（libs/streaming）

* **预先生成播放列表**：一次播放的媒体播放列表在任何转码开始之前就列出整个媒体源的所有分片（VOD），客户端可以拖动到任意位置。编码输出的视频和音频按等长分片，并在分片边界强制关键帧；直接复制的视频只能在其关键帧处切分，因此按媒体源保存的关键帧，在每个分片长度整数倍之后的第一个关键帧处切分。主播放列表声明输出的 RFC 6381 编解码器字符串、分辨率、帧率与动态范围（`VIDEO-RANGE`），保留的杜比视界或 HDR10+ 以 `SUPPLEMENTAL-CODECS` 声明。
* **按需生成分片**：每个流由一次 ffmpeg 运行在客户端前方写出分片。请求尚未写出的分片时，等待该运行；若该分片位于运行起点之前，或超出已写内容 24 秒以上，则在该分片处重启 ffmpeg。已写出的分片在播放会话期间保留并可再次提供，向后拖动没有代价。对同一个流的请求逐个处理。
* **重启后对齐**：ffmpeg 的 HLS 复用器从每次运行的起点开始计算切分间隔，拖动后会让直接复制视频的切分点错位。因此直接复制的视频每个图像组（GOP）写成一个文件，按其关键帧编号，一个分片由连续的若干文件拼接提供；无论从哪里开始运行，写出的文件都相同。重启时从分片第一个图像组的中间读取直接复制的视频，因为对含重排帧的流，ffmpeg 会把定位向前移动几帧。编码输出的运行恰好从分片边界开始，切分点自然对齐。
* **统一时间轴**：分片携带源文件从起点算起的时间戳，seek 后从起始位置延续（`-output_ts_offset`），与 ffmpeg 提取的、作为文件交付的字幕使用同一时间轴。视频的时间戳允许为负（`-avoid_negative_ts disabled`，fMP4 另加 `frag_discont`）：含重排帧的视频不会被整体后移，首帧在零点播放，与字幕同步。fMP4 中早于零点开始的音频（编码器的预填充，以及 MP4 源中 AAC 的预填充）被平移到零点（`setts` 位流过滤器），因为分片的解码时间（`tfdt`）是无符号数，Media3 拒绝负值；音频因此晚播相应时长，至多几十毫秒。
* **安全替换**：ffmpeg 把每个文件先以临时名写出、完成后再重命名（`-hls_flags temp_file`）：以最终名称存在的文件即已完整，之后的运行替换它也不会截断客户端正在读取的内容。每次运行写出自己的初始化分片，保留并提供第一个。

---

## 11. API 与认证（apps/server）

* **装配**：`apps/server/internal/server` 把存储、插件、播放、图片、媒体库后台任务与处理器树组装起来并运行到关闭；`cmd/mavio` 只负责解析出它的配置，端到端测试运行的也是同一套装配。
* **主目录与设置**：服务端保存的一切都在其主目录中，默认为 `~/.mavio`，可由 `--home` 或 `MAVIO_HOME` 另行指定：`config.toml`（可选）、SQLite 数据库 `mavio.db`、`plugins/`、`plugin-data/`（插件的数据目录）、`metadata/`（选定的图片）、`backups/` 与 `cache/`（下载与缩放后的图片、编译后的插件，位于 `cache/transcodes/` 的进行中转码，以及位于 `cache/rclone/blocks/` 的 rclone 远端块缓存），缺失时自动创建。约定无需配置；确实存在的设置（`addr`、`database`、`ffmpeg`、`ffprobe`、`cache-dir`、`transcode-dir`、`plugin-dir`、`plugin-seed-dir`、`plugin-data-dir`、`metadata-dir`、`backup-dir`、`discovery-addr`、`rclone`、`rclone-config`、`rclone-url`、`rclone-user`、`rclone-pass`、`rclone-cache-size`、`dev`、`dev-library`）由 Viper 依次从命令的参数（`--cache-dir`）、环境变量（`MAVIO_CACHE_DIR`）与 `config.toml`（`cache-dir = "/srv/cache"`）读取，默认取上述布局；转码目录跟随缓存目录。`config.toml` 中的路径必须是绝对路径，填相对路径时服务端拒绝启动；参数与环境变量中的相对路径相对于工作目录；`plugin-dir`、`backup-dir` 或 `discovery-addr` 为空时分别关闭插件、备份或局域网发现，`rclone-cache-size` 为 0 时关闭块缓存。`config.toml` 中有未知的键时服务端拒绝启动。`mavio version` 打印版本，`--restore <备份>` 在提供服务前把备份恢复到空数据库中。
* **单一处理器树**：`apps/server/internal/httpserver` 把 Connect 服务（实现位于 `internal/rpc`）与普通 HTTP 媒体端点挂载在同一个 `http.ServeMux` 上。每个 Connect 请求在到达服务之前先按其 protovalidate 规则校验，违反规则返回 `invalid_argument`。
* **跨域请求**（`httpserver.cors`）：每个路由都响应来自任何来源的请求（CORS，包括预检请求与私有网络请求），使一台服务器提供的 Web UI 能登录其他服务器（[客户端 §5.4](clients.zh-CN.md#54-多台服务器)）。它们的凭据是 Bearer 令牌与不可猜测的 ID，从不是 Cookie，因此其他来源得不到它自己发出请求所得不到的东西。
* **账户**：密码以 argon2id（19 MiB、2 轮、1 条并行通道）散列为 PHC 字符串；使用其他参数的散列仍被接受，并在下一次成功登录时替换。以不存在的用户名登录与密码错误的耗时相同。通过插件认证的用户没有密码散列。
* **Quick Connect**：与 Jellyfin 一样，可以用已登录的设备让新设备登录。新设备无需凭据即可发起请求并显示六位数字代码；已登录用户输入该代码，即以该用户身份为设备登录；设备用请求的密钥（32 个随机字节，从不显示）轮询请求，随后领取一次访问令牌。请求保存在内存中，十分钟后过期，或在获得授权一分钟后过期。代码在待处理的请求中唯一。
* **首次运行**：在还没有任何用户时，`AuthService.CreateFirstUser` 无需凭据即可创建一个管理员并使其登录；此后该调用返回 `failed_precondition`。客户端通过 `AuthService.GetAuthInfo` 得知这一步是否尚未完成，并得到服务端的标识：保存在数据库中的 ID（同时登录多个服务端的客户端据此认出从另一个地址访问到的同一服务端）、名称与版本。
* **会话**：登录为一台设备签发由 32 个随机字节组成（base64url）的访问令牌；服务端只把它的 SHA-256 散列作为 `AuthSession` 保存（见[领域模型](domain.zh-CN.md)）。客户端以 `Authorization: Bearer <token>` 发送令牌。令牌不会过期；在客户端退出登录、用户吊销该会话或删除账户，或同一设备再次登录时失效。会话的最近活动时间最多每分钟记录一次。
* **授权**：拦截器把令牌解析为用户与会话并放入请求上下文；未知令牌与被禁用的用户返回 `unauthenticated`。只有 `GetAuthInfo`、`CreateFirstUser`、`Login` 以及 `SystemService` 的健康检查是公开的。管理员权限与媒体库访问权限由各服务根据上下文中的用户自行检查（`permission_denied`）。会导致没有任何已启用管理员的更改将失败（`failed_precondition`）；修改密码会结束该用户的其他会话。媒体库、条目与媒体源的文件夹及文件路径只向管理员显示。
* **媒体 URL**：播放器并不总能给媒体请求附加请求头（AVPlayer、`<video>`），因此媒体端点不接受 Bearer 令牌。通过需认证的 `PlaybackService` 开始的播放会获得一个不可猜测的 ID，其媒体在 `/media/{playback}/` 下提供：直接播放时是文件本身（`stream.<ext>`，支持范围请求，在其媒体库文件夹内打开），否则是来自 `libs/streaming` 的 `master.m3u8`、`main.m3u8`、`init.mp4` 与编号分片。客户端停止播放、五分钟内没有进度上报或媒体请求（保留最后上报的位置），或发起播放的登录会话已不存在（每 30 秒检查一次）后，这些 URL 随即失效。
* **播放**（`internal/playback`）：播放决策会考虑用户记住的或偏好的流、其媒体库与分级限制、转码权限与码率上限，以及用户的并发播放上限。转封装与转码只以 HLS 交付；关键帧尚未提取的媒体源在本次播放中转码视频，并排入一个紧急的关键帧任务；针对渐进式转封装的决策会按客户端的 HLS 配置重新决定，复制视频的播放报告为直接串流。进度上报与停止播放通过 `UserData.RecordPosition` 记录位置（见[领域模型](domain.zh-CN.md)），每次播放至多计一次播完。没有 ffmpeg 时只能直接播放。
* **事件**（`internal/events`、`EventService`）：每个已登录设备可以保持一条事件流，即一个 Connect 服务端流：以该设备的会话开始，每 30 秒发送一次心跳；同一设备的新流替换旧流，落后超过 256 个事件的流被断开，由客户端重连。服务端在内存中保存这些流。`events.Observe` 包装存储层，写入的内容在其事务提交后到达各流：媒体库与条目的变化先收集五秒，再按各设备用户可见的条目与媒体库发送；用户的条目状态发送到该用户的设备；媒体库扫描的开始与结束发送给管理员。插件配置变化与会话变化由引起它们的服务发送。
* **会话**（`SessionService`）：设备在事件流打开期间在线。会话列表列出用户的已登录设备（管理员可列出所有用户的），以及各设备正在播放的内容（`playback.Manager.NowPlaying`：条目，以及最近一次上报的位置与暂停状态）。当用户的某个会话上线、下线，或开始、停止、暂停、继续播放时，该用户的设备会得知。命令（播放条目、暂停、继续、停止、下一个、上一个、跳转、显示消息）经事件流发送到调用者的在线设备，管理员可发送到任何人的设备；设备执行命令并照常上报播放。
* **SyncPlay**（`internal/syncplay`、`SyncPlayService`）：在线设备以分组一起观看，分组保存在内存中，遵循 Jellyfin 的 SyncPlay。分组处于空闲、等待、暂停或播放状态。设置队列、跳转、切换队列条目，以及成员缓冲或加入，都会使分组等待；没有成员缓冲后开始播放，原本暂停则保持暂停。分组在最高成员延迟的两倍（至少 500 毫秒）之后开始播放，使每个成员都能及时收到命令。报告就绪的成员若与分组位置相差超过 500 毫秒，会被要求跳转；报告中的时间若与服务端相差超过两秒则被忽略。每次变化都向每个成员发送分组（状态、成员、队列，以及某一时刻所处的位置），成员需要行动时还附带以服务端时间计时的命令；客户端用 `GetTime` 估计其时钟偏差。一个设备至多属于一个分组，下线时离开分组，没有成员的分组随之结束。成员只能加入其可播放队列的分组，队列必须每个成员都可播放。
* **浏览**（`internal/browse`）：最近添加与下一集由用户媒体库与分级范围内的条目查询构建。最近添加列出叶子条目（电影、单集、曲目等），按添加时间排序；分组时与 Jellyfin 现在的服务端一样，专辑代表其新曲目，剧集代表在其最新一集前一天之内添加的单集：这些单集都在一个多季剧集的同一季时显示该季，剧集只有一季或单集跨越多季时显示剧集，只有一集时显示该集，用户无权看到容器时也显示该集。下一集按最近播放时间遍历用户的单集，按此顺序每部剧集取一次，交给 `NextEpisode`（见[领域模型](domain.zh-CN.md)）求出下一集，直到填满一页，因此最近播放的剧集排在前面。条目响应给出单集所属的剧集与季、曲目所属的专辑。
* **首页**（`internal/home`，`HomeService`）：首页与媒体库"推荐"标签页的推荐行（[客户端 §6.3.1](clients.zh-CN.md#631-首页)），每行是一页条目，带有其类型，以种子生成的推荐行还带有种子，由客户端以自己的语言命名。它们由用户媒体库与分级范围内的条目查询构建：继续观看（可续播条目与下一集，按最近活动排序）、最近添加（同最近添加条目）、最近上映与最近播出、评分最高与未看的高分作品、开始观看（尚未开始的剧集）、重温（一年多前最后播放的条目，剧集为半年多前），以及最近 30 天内已可观看的请求。以种子生成的推荐行仿照 Jellyfin 的电影推荐——因为你看过、因为你喜欢、同类型更多、主演、导演——种子取自用户最近 90 天的播放或其收藏，按用户、推荐行与日期的散列选取。热门列出发现提供者第一个热门目录中服务器没有的作品。管理员为每个媒体库设置哪些推荐行出现在首页与其标签页（`Library.Hubs`，[领域模型](domain.zh-CN.md) §4.1）；用户在客户端 `mavio`、视图 `home` 的显示偏好中置顶并排序媒体库、隐藏推荐行。
* **相似条目**（`ItemService.ListSimilarItems`）：同种类的条目，按 Jellyfin 本地相似条目提供者为电影评分的方式，即与该条目共有的类型（10）、标签（5）、工作室（5）、导演（50）与演员（15）评分（`ItemQuery.SimilarTo`，见[领域模型](domain.zh-CN.md) §9.1），限于用户的媒体库与分级范围；单集与季以其剧集代表。它们用于条目的"更多类似内容"与"因为你看过"推荐行。
* **合集与播放列表**（`CollectionService`、`PlaylistService`）：管理员整理合集，用户整理自己的播放列表，它们位于服务端在首次需要时创建的整理类媒体库中。每个用户都能看到播放列表媒体库，其中只有自己的播放列表；合集媒体库遵循媒体库权限。以合集或播放列表为 parent 调用 `ItemService.ListItems` 会按用户权限列出其中的条目，每个一次，未指定排序时按其顺序；`ListPlaylistEntries` 列出播放列表的条目项及其 ID。把剧集、季、专辑或合集加入播放列表时，按顺序加入其中可播放的条目。`LibraryService` 不能创建、修改或扫描整理类媒体库，启动时的扫描也跳过它们。
* **元数据管理**（`MetadataService`，仅限管理员）：`UpdateItem` 设置更新掩码指定的字段（包括锁定），通过 `library.Refresher.Update` 保存条目；`RefreshItem` 立即刷新条目，带 `replace_metadata` 时丢弃提供者此前给出的内容（外部 ID 与锁定字段除外），并忽略条目的 NFO 文件（在保存本地元数据的媒体库中也忽略其旁边的图片，由提供者的图片替换）。`SearchRemote` 按条目的名称与年份或给定的名称与年份，询问能搜索的元数据插件该条目可能是什么；`IdentifyItem` 用某个结果的外部 ID 替换条目的外部 ID，并以此刷新条目。`ListRemoteImages` 列出提供者拥有的条目图片；`SetItemImage` 下载其中一张或接收上传的图片，设为该种类的第一张，保存在媒体旁或元数据目录中；`DeleteItemImage` 删除图片及其文件。`SearchSubtitles` 与 `DownloadSubtitle` 通过字幕插件查找字幕，转换为 UTF-8 后保存在视频旁，命名为 `<文件名>.<语言>[.sdh][.forced].<格式>`，立即成为视频的字幕流。保存本地元数据的媒体库中，条目的修改会重写其 NFO 文件；启用自动合集的媒体库中，电影会加入其合集。
* **设置**（`internal/settings`，`SystemService.GetServerSettings` 与 `UpdateServerSettings`）：服务端设置（[领域模型](domain.zh-CN.md) §12）保存在数据库中，无需重启即可生效。服务端各部分注册自己应用设置的方式；一次修改依次交给各部分，若有一部分拒绝（例如证书无法读取），已接受的部分恢复之前的设置，且不保存任何内容。转码设置作用于此后开始的播放：`playback.Manager` 换用新的规划器与转码目录；`GetServerSettings` 还列出服务端 ffmpeg 支持的硬件加速。
* **网络**（`internal/network`）：处理器树既在根路径、也在基础 URL 下提供服务，访问不带斜杠的基础路径会重定向到带斜杠的路径。HTTPS 以配置的证书在单独端口运行，随设置变化而启动、重启或停止。局域网发现在 `--discovery-addr`（默认 7359）上应答 UDP 请求 `who is MavioServer?`，返回服务端 ID、名称、版本，以及请求到达的网卡上的地址。
* **任务与作业**（`TaskService`）：周期性任务为每个媒体库的扫描、插件的任务、获取作业（RSS 同步、队列检查与回收目录清理，间隔取自获取设置，以及每日刷新想要的电影）与每日的作业清理；`ListTasks` 给出每个任务的间隔、上次完成的运行、下次运行以及是否正在运行，`RunTask` 立即排入一次运行，`ListJobs` 分页列出作业队列。
* **API 密钥**（`AuthService.CreateApiKey`、`ListApiKeys`、`RevokeApiKey`）：拦截器在需要会话令牌之处也接受 API 密钥的令牌（[领域模型](domain.zh-CN.md) §12.2）；这类请求没有会话。
* **活动与日志**（`internal/activity`、`ActivityService`；`internal/logs`、`SystemService.ListLogs`）：活动被保存并排队发送给通知插件，90 天后清除。日志还把它不保存的事件（来自 `events.Observe` 与 hub 的条目、用户数据、任务与扫描事件）发布给事件消费者；插件管理器为每个消费者维护一个队列。活动日志先于 hub 与插件创建，插件启动后事件才到达插件。服务端在内存中保留最近 2,000 条 info 及以上级别的日志供 `ListLogs` 使用，日志照常输出。
* **文件夹**（`SystemService.ListDirectory`）：管理员浏览服务端的文件夹或根目录（Windows 上为各驱动器）以选择媒体库文件夹；隐藏条目不列出。
* **本地化数据**（`LocalizationService`）：国家、具有 ISO 639-1 代码的语言，以及某个国家分级体系中的分级，来自 `libs/metadata` 中 Jellyfin 的数据（`Countries`、`Languages`、`ParentalRatings`）。
* **备份**（`internal/backup`、`BackupService`）：备份是 `--backup-dir` 中的一个 zip 文件，包含 `manifest.json`、数据库的行（`store.Dump`：每张表为一份由带类型值组成的 JSON 行文件，取自同一快照）、元数据目录、插件目录与插件的数据目录。管理员带着自己的 bearer 令牌在 `/system/backups/{name}` 下载。`-restore <文件>` 在新服务端启动前把备份恢复到其空数据库中（`store.Restore`：数据库类型相同，且架构包含备份所含的每个迁移；按外键顺序插入行，条目排在其父条目之后，派生键重新计算），并放回各文件夹。
* **下载与渐进式流**：请求 `download` 的播放需要用户的下载权限，并以静态场景决策：客户端能播放文件本身时，以附件形式提供文件；否则按客户端的静态配置文件做渐进式转封装或转码，从不使用 HLS。没有 HLS 配置文件的流式客户端也得到渐进式流。ffmpeg 为每个请求把渐进式流（分片 MP4、Matroska 或 MPEG-TS）直接写入响应，因此不能跳转。
* **媒体附加内容**：`ItemService.GetItem` 列出条目的缩略图拼图与媒体片段；拼图在 `/images/trickplay/{item}/{width}/{sheet}.jpg`、章节图片在 `/images/chapters/{item}/{source}/{index}` 提供（接受图片参数），文件取自元数据目录，ID 即凭证。条目带有由其测得响度计算出的归一化增益。`ItemService.GetLyrics` 读取音轨旁的歌词文件（`.elrc`、`.lrc` 或 `.txt`，必要时从旧字符集转换），由 `metadata.ParseLyrics` 解析：LRC 的行及其时间与 ID 标签，增强 LRC 的逐字时间作为以 UTF-16 码元计的提示，与 Jellyfin 的解析器一致。播放会列出其片源附带的文件，例如供客户端渲染 ASS 字幕的字体；ffmpeg 在 `/media/{playback}/attachments/{index}` 处各提取一次。
* **图片**：图片在 `/images/{id}` 提供，接受 Jellyfin 的尺寸参数（`width`、`height`、`maxWidth`、`maxHeight`、`fillWidth`、`fillHeight`、`quality`）；与媒体 URL 一样，客户端从按访问权限过滤的 API 响应中得到的 ID 是唯一的凭证。本地图片在其条目所属媒体库文件夹或元数据目录内读取；提供者的图片只下载一次，存入缓存目录。不带尺寸参数时提供原图；`imaging.Process` 生成的图片采用请求的 `format`（`jpg`、`png`、`webp`），未指定时对 `Accept` 头接受 WebP 的客户端生成 WebP，否则为 JPEG（含透明时为 PNG）。生成的图片按源内容哈希与参数缓存，对同一结果的并发请求共享一次生成。SVG 通过 `CheckSVG` 后原样提供。
* **拼贴图**：`/images/collages/{id}` 以相同参数合成媒体库的图片（其最近添加条目的海报并排，16:9）、合集的图片（其条目的海报，2 × 2 网格，2:3）或播放列表的图片（其条目所属剧集与专辑的海报，或条目自身的海报，正方形网格），与 Jellyfin 的动态图片一样；所有者的 ID 即凭证。拼贴图按所用图片缓存，因此会随之更新；没有可用的图片时返回未找到。
* **开发用播放器**：带 `--dev` 启动时，服务端还提供 `/dev/player`：一个单页面，可登录、列出视频并通过 `PlaybackService` 播放，使用 hls.js 或 Safari 自带的 HLS 播放器（`?engine=native|hlsjs`），可选的串流码率上限会使视频转码（`?bitrate=<每秒比特数>`），用于在客户端完成之前于真实浏览器上检查播放。`-dev-library <dir>` 把 `pnpm nx run fixtures:dev-library` 生成的示例媒体库中的 `Movies` 与 `Shows` 文件夹添加为媒体库。
* **API 参考与反射**（`internal/apidocs`）：`/api/spec.json` 提供 `libs/proto/openapi` 中的 OpenAPI 文档（服务端提供的 Connect 服务，仅 JSON，以及 `openapi/base.yaml` 中的普通 HTTP 媒体路由），填入服务端版本，并把公开过程标为无需令牌。`/api` 用 Scalar 渲染该文档，Scalar 以固定版本从 jsdelivr CDN 加载，请求发往打开页面时的地址（包括基础 URL）。服务端还为所有服务应答 gRPC 反射（v1 与 v1alpha）。三者都无需令牌：它们描述 API，但不授予任何权限。
* **字幕**：以文件交付的文本字幕在 `/media/{playback}/subtitles/{index}.{format}` 提供，由 `libs/subtitle` 从媒体库中的外挂文件或内嵌流转换而来；内嵌流由 ffmpeg 连同文件中其他内嵌文字字幕流一起提取，文件不变时只提取一次，存入条目的元数据目录（ASS 保留样式，其他文本转为 SRT）。云存储上不提取内嵌流：这类客户端改为通过 HLS 字幕轨获得，由转码在读取视频时一并写出（[优化 §2.6](optimization.zh-CN.md#26-内嵌字幕)）。对接受清单内字幕的 HLS 客户端，每条文本字幕是主播放列表的一个字幕轨：一个 WebVTT 播放列表，语言以 BCP 47 标签表示，按视频分片切分：每个分片 `subtitles/{index}-{segment}.vtt` 包含在其期间显示的字幕，与视频一样以源文件的时间轴计时，最后一个分片还包含超出媒体结尾的字幕。图形字幕（PGS、VobSub）只能烧录；服务端无法写出的外挂文件，或没有 ffmpeg 时无法提取的内嵌字幕，将被丢弃。

---

## 12. 分发

* **服务端二进制**：`pnpm nx run server:dist` 以 `CGO_ENABLED=0` 和 `-trimpath` 将 `cmd/mavio` 交叉编译为 Linux、macOS、Windows 的 amd64 与 arm64 版本，输出到 `apps/server/dist/`；设置了 `$VERSION` 时将其写入 `internal/buildinfo`。所有二进制均为静态链接。
* **容器镜像**（`apps/server/Dockerfile`，从仓库根目录构建）：服务端在构建机自身平台上交叉编译，因此构建 linux/amd64 与 linux/arm64 镜像时 Go 部分无需模拟。镜像在 `/usr/lib/jellyfin-ffmpeg` 内置 jellyfin-ffmpeg 便携版，即 `mise.toml` 锁定的版本，并按 GitHub 为该发布文件给出的 SHA-256 摘要校验；基础镜像为 Debian slim，附带 CA 证书与时区数据；镜像还在 `/usr/local/bin/rclone` 内置 Mavio 的 rclone fork，即 `mise.toml` 锁定的发布版本的 Linux 构建，并按其 SHA-256 摘要校验，服务端将其作为 rcd 运行（§9.2），配置位于 `/config/rclone/rclone.conf`。
* **镜像布局**：服务端以 `mavio` 用户（uid 与 gid 均为 1000）在 8686 端口运行，并在 UDP 7359 端口应答局域网发现；主目录为 `/config`（`config.toml`、SQLite 数据库、插件目录、选定的图片与备份），图片、编译后的插件与转码文件位于 `/cache`，媒体库挂载在 `/media` 下。镜像通过 `MAVIO_*` 环境变量设定这些路径与 jellyfin-ffmpeg 的可执行文件，镜像名之后的参数与 `-e` 传入的变量可覆盖它们。DLNA 插件随服务端一起构建，并从 `/usr/lib/mavio/plugins` 播种；它在 UDP 1900 端口上的 SSDP 只有在 `--network host` 下才能到达容器。索引器、下载客户端与字幕插件同样从此播种；未配置索引器时第一个只每日拉取 Cardigann 定义，未配置客户端或提供者时另外两个什么也不做。健康检查调用 `SystemService.GetHealth`。

---

## 13. 资源获取（libs/acquire）

Mavio 为用户查找、下载并入库他们想要的电影与剧集，即 Seerr、Radarr、Sonarr 与 Prowlarr 在 Jellyfin 旁共同完成的事，但在一个服务端之内：一份条目目录、一套用户、一份配置。这部分工作是[路线图](roadmap.zh-CN.md)的 P15 至 P20 阶段。

### 13.1 组成
| 部分 | 位置 | 对应 |
| :--- | :--- | :--- |
| 发现 | `DiscoverService`；列表来自 `DISCOVERY_PROVIDER` 插件（`scraper-tmdb`、`scraper-servarr`） | Seerr 的发现 |
| 请求与问题 | `RequestService`、`IssueService`；规则在 `apps/server/internal/requests` 中 | Seerr 的请求与问题 |
| 资源获取引擎 | `libs/acquire`，含 `movie` 与 `series` 包，以及为文件命名的 `organize` 包；服务在 `apps/server` 中 | Radarr、Sonarr |
| 想要的条目 | `WantedService`：按 TMDB ID 加入的电影与按 TVDB ID 加入的剧集，作为 `apps/server/internal/acquisition` 作业执行的搜索、RSS 同步与 scene 映射更新 | Radarr 的电影、Sonarr 的剧集及其搜索 |
| 档案 | `ProfileService`：质量定义与质量档案、custom formats、延迟档案与发布档案、远程路径映射 | Radarr 与 Sonarr 的设置 |
| release 解析 | `libs/release` | Radarr 与 Sonarr 的解析器 |
| Servarr 的元数据 | `plugins/scraper-servarr`（WASM） | Radarr 与 Sonarr 的元数据源、scene 映射 |
| 索引器 | `plugins/indexers`（进程），`INDEXER_PROVIDER`；通过 `ReleaseService` 搜索 | Prowlarr |
| 下载客户端 | `plugins/download-clients`（进程），`DOWNLOAD_CLIENT_PROVIDER`；通过 `DownloadService` 调用 | Radarr 与 Sonarr 的下载客户端 |

* **状态在服务端，网络在插件。** 想要的条目、档案、决策、队列、历史、入库与请求都属于服务端，与它们所涉及的条目和用户放在一起。与索引器和下载客户端打交道、并随它们变化的部分放在插件中，从插件目录升级，无需发布服务端。
* **一个引擎。** 电影与剧集共用质量档案、custom formats、决策、队列与入库；`movie` 与 `series` 包的区别在于如何搜索、监控以及把 release 匹配到条目。共用的模型在根包 `acquire` 中：Radarr 的质量 ID 与各质量的定义（电影用 Radarr 的默认值，剧集用 Sonarr 的）、带分组、截止质量与格式分数的质量档案、包含 Radarr 全部规格及 Sonarr 的 release 类型的 custom formats、delay profile 以及 Sonarr 的 release profile。对来源的条件在剧集上也使用 Radarr 的来源与修饰符，Sonarr 的 raw 来源即 Radarr 的 Raw-HD 与 remux 修饰符。
* **插件之间不互相调用。** 服务端汇集各提供者插件给出的结果（与字幕提供者相同），并由自己调用索引器与下载客户端。

### 13.2 数据源与编号
* **Servarr 的服务**是默认数据源，无需密钥：Radarr 的元数据服务（`api.radarr.video`）提供电影与列表，Sonarr 的 SkyHook（`skyhook.sonarr.tv`）提供剧集与单集，Sonarr 的服务（`services.sonarr.tv`）提供 scene 映射与日播剧，以及 XEM（`thexem.info`）和 Prowlarr 的索引器定义（`indexers.prowlarr.com`）。对 Servarr 服务的请求携带该服务所属应用的 User-Agent，构造方式与该应用相同（`Sonarr/<version> (<os> <os version>)`）。
* **TMDB**：`scraper-tmdb` 在管理员设置了自己的 API Key 时使用该密钥，否则使用 Radarr 的 TMDB 只读访问令牌；TMDB 拒绝该令牌时，回退到插件内置的 Mavio 自己的 API Key。
* **磁盘上的编号遵循 TVDB**，与 Sonarr 一致：剧集的单集来自 SkyHook，按 TVDB 顺序，带绝对集数与播出日期；文件按这些编号命名（动漫还带绝对集数）；请求按此顺序列出季。剧集有类型：standard、daily 或 anime，决定如何匹配 release。
* **TMDB 的元数据**通过剧集的显示顺序挂上：即 Jellyfin 的 `DisplayOrder` 加 TMDB 的 episode groups，以及 TMDB 单集上已有的 TVDB ID。
* **命名不同的 release** 通过 Sonarr 的 scene 映射与 XEM 匹配；日播剧按播出日期匹配。XEM 给出的想要的剧集的 scene 编号保存在其单集上（`EpisodeState`），并像 Sonarr 的 `XemService` 那样推算到它未覆盖的单集；在剧集加入时以及每 3 小时随 scene 映射一同更新。

### 13.3 流程
1. **请求**：用户请求一部电影或若干季；由管理员审批，或在用户配额内自动通过，且只能在用户的分级限制之内。请求使电影或单集成为想要的条目，按某个质量档案受监控。
2. **搜索**：想要的条目立即搜索一次；RSS 同步以及对想要的条目与可升级条目的搜索按各自的间隔作为作业运行。
3. **release**：`ReleaseService` 按 ID、编号与分类向索引器查询电影或单集；`libs/release` 解析每个 release，再按 ID、标题与 scene 映射匹配到条目。
4. **决策**：每个 release 经过 Radarr 与 Sonarr 决策引擎的各项规格（质量是否允许、体积、custom format 分数、语言、是否已在队列、是否允许升级、延迟、黑名单、发布时长、做种数），通过的再排序。
5. **下载**：最优的 release 通过 `DownloadService` 发送给下载客户端，带上其类型对应的分类；队列通过轮询客户端跟踪它。
6. **入库**：完成的下载在应用远程路径映射后再次与条目匹配，并用 ffprobe 校验；文件与媒体库在同一卷上时硬链接进媒体库，否则复制或移动，按媒体库的命名模板命名，期间占用该卷在 `VolumeLedger` 中的位置，并通过 `QuietGate` 为播放让路。随后只扫描写入的文件夹而非整个媒体库，想要的条目成为有媒体的条目。被入库规则阻止的下载，或并非下载的文件夹，可通过 `DownloadService`（`ListManualImport`、`ManualImport`）手动入库，由用户选择文件、电影、其质量、语言与发布组，以及移动还是复制。
7. **升级与失败**：更好的 release 会替换文件，直到达到档案的截止质量，被替换的文件移入回收目录；失败或不匹配的下载加入黑名单并重新搜索。
8. **通知**：请求变为可用，并通过活动日志通知请求者。

### 13.4 索引器（`plugins/indexers`）
移植 Prowlarr 的第一方进程插件：Newznab 与 Torznab、在运行时从 `indexers.prowlarr.com` 拉取定义的 Cardigann 引擎，以及 Prowlarr 以代码实现的全部站点；能力与分类、限速与失败后的退避、索引器代理（HTTP、SOCKS、FlareSolverr）。其配置列出已配置的索引器；定义以及每个定义所需的字段由插件给出。`IndexerProviderService` 列出索引器及其能力，搜索它们（电影、剧集或文本，带 ID、季与集、分类），读取其 RSS，并获取 release 的种子文件、磁力链接或 NZB。

### 13.5 下载客户端（`plugins/download-clients`）
包含 Radarr 与 Sonarr 支持的全部下载客户端的第一方进程插件，如 qBittorrent、Transmission、Deluge、rTorrent、Aria2、SABnzbd、NZBGet 与监视目录。其配置列出已配置的客户端，每个客户端有电影与剧集各自的分类。`DownloadClientProviderService` 添加 release（种子文件、磁力链接或 NZB，使用其类型的分类与优先级，以及其索引器的做种限制），列出下载及其状态、进度与输出路径，将下载标记为已入库，并删除下载。远程路径映射属于服务端。云盘的离线下载是服务端内置的下载客户端（§9.2）。

### 13.6 存储
想要的状态、质量定义与质量档案、custom formats、delay profile 与 release profile、队列、历史、黑名单与请求都存放在 `libs/store` 中，支持两种数据库，并通过 `core` 中的端口访问。

### 13.7 发现、请求与问题
`apps/server/internal/requests` 包实现 Seerr 的规则；`DiscoverService`、`RequestService` 与 `IssueService` 封装它，`server.go` 把它连接到资源获取运行时、元数据提供者、单集列表提供者、发现提供者与活动日志。
* **权限**是用户策略中的布尔值：可请求电影、可请求剧集、各自自动通过、可报告问题；以及配额：若干天内电影的数量或剧集的季数，用户自己的或服务端的。管理员即 Seerr 的请求、问题与用户管理者：审批、拒绝与重试请求，为他人请求，忽略配额，且不受限制。服务端的请求设置给出新用户的权限，并规定通过的请求去向：电影媒体库与质量档案及最低可用性，剧集媒体库与质量档案及是否按季分目录、如何监控新季，动漫剧集单独的媒体库与质量档案，以及能否请求特别篇。
* **标题**通过元数据提供者按 TMDB ID 查询，使用服务端元数据设置的国家与语言，并缓存数小时：名称、年份、分级、简介、类型、标签、图片，剧集还有 TVDB ID。带动漫标签的剧集作为动漫剧集添加，设置了动漫媒体库与质量档案时使用它们。单集列表提供者给出剧集的季。
* **阻止列表**：管理员按 TMDB ID 阻止电影与剧集，如 Seerr，或阻止某个合集中由发现提供者的合集目录列出的电影；被阻止的标题标记为已阻止，优先于其他所有状态，对它的请求被拒绝。每周运行的阻止标签任务（`requests.blocklisted_tags`）如 Seerr：先把按标签阻止的作品移出阻止列表，再阻止每个阻止标签的电影与剧集，这些作品从其提供者按关键词过滤的热门目录中按每种排序读取，直到设置的页数上限（页数较少时按目录自身的排序），每次查询后暂停 250ms；手动阻止的作品不加标签，提供者不认识的关键词从设置中删除。作品先全部读取，再在一个事务中修改阻止列表。
* **创建请求**按 Seerr 的顺序检查：阻止列表、权限、分级限制（标题的分级按该国体系打分；未分级的通过，除非策略屏蔽未分级条目）、配额（被拒绝的请求与忽略配额的请求不计入），然后是重复：已请求、已想要或已可用的电影被拒绝；剧集中其他活动请求所请求的季、已想要或已有文件的季被剔除，剩余季数须在配额之内。请求随后处于待审批状态，或在用户该类型的请求自动通过时立即通过。用户锁与标题锁串行化检查与写入，因此并发请求既不会超出配额，也不会重复请求同一标题。
* **审批**使标题成为想要的条目：电影通过 `AddMovie` 并搜索；剧集通过不监控任何内容的 `AddSeries`，再监控并搜索所请求的季。媒体库中已有的标题被设为想要，并加入其季。失败时请求被标记为失败并记录错误，由管理员重试。
* **可用性**：资源获取运行时的入库钩子，以及为扫描发现的文件每十分钟运行一次的周期作业，会完成那些电影已有文件、或各季已播出单集全部有文件的已通过和失败的请求，按季逐一完成。该作业列入任务列表，可立即运行。
* **发现**把各提供者的目录合并为 `<plugin>/<catalog>`，并把一页中的每个标题标记为可用、部分可用、想要、已请求、已阻止、已删除（已完成请求的文件已不存在）或无，附带其条目、其活动请求以及用户能否请求它。`SearchTitles` 经第一个支持搜索的提供者按名称查找标题，同样标记；`GetTitle` 给出标题的详情及其各季的状态。用户的分级限制同样适用：仅当调用者指定分级或策略屏蔽未分级标题时，按认证分级过滤的目录才只被请求限制之内的分级，否则保留未分级标题；其余标题逐个查询，超出限制的被剔除。成人标题从不列出。每页统计其剔除的标题，总数为提供者的总数减去它们。
* **问题**报告用户可访问的条目在视频、音频、字幕或其他方面的问题，可指明季与集，并带有评论串。创建者与管理员可评论、解决与重新打开；管理员，或尚无回复的问题的创建者，可删除问题。
* **通知**是以相关用户为用户的活动：`request.pending`（面向管理员的通知器）、`request.approved`、`request.auto_approved`、`request.declined`、`request.failed`、`request.available`、`issue.created`、`issue.commented`、`issue.resolved` 与 `issue.reopened`，属性中带有请求或问题。通知器插件投递它们，`ActivityService.ListMyActivities` 列出用户自己的活动，记录时 `NotificationReceived` 事件送达相关用户的会话，待审批或失败的请求与报告的问题也送达管理员的会话。

---

## 14. 字幕获取（libs/acquire/subtitles）

Mavio 为电影与单集查找缺少的字幕，即 Bazarr 在 Sonarr 与 Radarr 旁完成的事，但在服务端之内：Bazarr 从 Sonarr 与 Radarr 同步的，就是服务端自己的条目；它的字幕索引，就是扫描与 probe 记录的流。这部分工作是[路线图](roadmap.zh-CN.md)的 P26 阶段。

### 14.1 组成
| 部分 | 位置 | Bazarr 中的对应 |
| :--- | :--- | :--- |
| 字幕配置、历史、阻止列表、失败记录 | `libs/core`、`libs/store` | 语言配置、`table_history*`、`table_blacklist*`、`failedAttempts` |
| 规则：缺少的字幕、评分、择优、升级、自适应搜索 | `libs/acquire/subtitles`（纯函数） | `list_missing_subtitles`、`subliminal_patch/score.py`、`download_best_subtitles`、`upgrade.py`、`adaptive_searching.py` |
| 运行时与 API | `apps/server/internal/subtitles`、`SubtitleService` | `subtitles/wanted`、`upgrade`、`manual`、调度器的作业 |
| 提供者、翻译器 | `plugins/subtitles`（进程）、`SUBTITLE_PROVIDER`、`SUBTITLE_TRANSLATOR` | `subliminal_patch/providers`、`pitcher.py`、`get_providers.py` 的限流、`subtitles/tools/translate` |
| 修正、编码、语言识别 | `libs/subtitle` | subzero、字符集检测、`guess_language` |
| 时间轴同步 | `libs/media/subsync`，语音由以 Go 运行的 Silero VAD 检测（`libs/media/vad`） | ffsubsync 及其 Silero 检测器 |

* **以服务端自己的数据代替 Bazarr 的同步。** 电影、剧集与单集是服务端的条目；文件来自的 release 是其入库所记录的抓取（Bazarr 的 `sceneName` 与历史 refiner），否则是文件名；release 名称由 `libs/release` 解析，代替 guessit；编码、分辨率与音轨语言来自已保存的 probe 结果。Bazarr 的 Sonarr 与 Radarr 同步、路径映射、Plex 与 Jellyfin 刷新、Apprise、调度器、备份与公告，分别对应入库与扫描、条目的流、活动日志与通知插件、`TaskService` 以及 P11。
* **规则与网络分开**，如资源获取（§13）：配置、决策、历史与阻止列表属于服务端；与字幕站点通信、随站点变化的部分在插件中，从插件目录升级，无需发布服务端。
* **修正与识别**（`libs/subtitle`）：`Subtitle.ApplyMods` 按 Bazarr 的标识符（`common`、`remove_tags`、`remove_HI`、`OCR_fixes`、`color(name=…)`、`emoji`、`reverse_rtl`、`fix_uppercase`、`shift_offset(…)`、`change_FPS(…)`、`two_point_fit(…)`）对解析出的字幕条目应用 Subzero 的修正，做法与 Subzero 相同：整个文件的修正在逐行修正之前或之后进行，逐行修正按 Subzero 的顺序逐行应用，互斥修正取最后一次出现，时间偏移相加，其他语言的修正以及并非以大写为主的字幕的大写修正不予应用，变空的字幕条目被丢弃；它按 Subzero 的方式返回所用的修正。SSA 与 ASS 文件如 pysubs2 为 Subzero 保留的那样，保留其脚本信息、样式、注释、其他段落以及每个事件的其他字段：以原格式写出时，只重写有变化的字幕条目的时间和文本，并移除被丢弃的条目。修正的正则表达式是 Subzero 的，由 regexp2 执行。`DetectHearingImpaired` 对文件文本应用 Bazarr 的表达式，阿拉伯语与波斯语中圆括号不计。`DetectLanguage` 如 guess_language 那样，按 Unicode 区块与三元组模型识别没有语言的外挂字幕的语言，不使用拼写词典；中文按 hanzidentifier 的字符集区分为 `zh-Hans` 或 `zh-Hant`，只出现简繁共用字符时为 `zh`。OCR 修正词典（Subtitle Edit 的列表加上 Subzero 的补充）、emoji、常规修正据以保留域名的公共后缀列表、guess_language 的模型与 CC-CEDICT 的字符集，由 `internal/cmd/subzerogen` 从各自来源生成，并记录每份数据的来源与许可证。测试将结果与 Bazarr 自身代码的结果比较（`testdata/mods/golden.py`）。`FixText` 以 Bazarr 的选项如 ftfy 的 `fix_text` 那样修复文本，Bazarr 对保存的每个字幕都会执行：文本不含 `<` 时解码 HTML 实体，按 ftfy 的 badness 启发式还原乱码，修复 C1 控制字符、拉丁连字、换行、代理项、终端转义与控制字符，并做 NFC 规范化，保留弯引号与全角字符；其字符表由 `internal/cmd/ftfygen` 从 Bazarr 附带的 ftfy 与 CPython 的编解码器生成，测试将其与 ftfy 比较（`testdata/fixtext/golden.py`）。

### 14.2 流程
1. **缺少的字幕**：电影或单集的文件入库、被扫描发现或 probe 时，或其字幕配置变化时，把配置想要的字幕与文件的字幕流比较：外挂的，以及设置要求计入时内嵌的，如 Bazarr 的 `list_missing_subtitles`：音轨已是该语言时排除、或仅在音轨是该语言时才要的语言，cutoff，听障字幕可代替普通字幕（配置排除时除外），以及 Bazarr 的语言等价关系。每部电影和每一集缺少的字幕会保存下来（`core.SubtitleWanted`，每个条目一行），使缺字幕列表与搜索不必读取条目的流：搜索、下载、上传、删除与提取会更新其条目的行，扫描会更新只有字幕文件变化的条目的行（`library.Scanner.OnSidecars`），而全部行的刷新（`subtitles.wanted_refresh`）在启动时以及字幕配置、其分配或设置变化后运行；文件缺失的条目的行在文件重新找到之前不出现在列表中。
2. **搜索**：电影或单集新增或变化的文件（入库的或扫描发现的）经 probe 且条目刷新后（`library.Jobs.Probed`），搜索缺少的字幕，使搜索知道其流与 ID；电影与单集的缺字幕搜索作业也会搜索；首次失败后，自适应搜索在一段延迟之后每个间隔只搜一次该语言，如 Bazarr 的 `is_search_active`。
3. **查询**：视频由其条目的名称、别名、ID、编号（季、集、绝对集数）、年份、release 名称及解析出的来源、发布组、分辨率、编码、版本与流媒体服务、帧率与 OpenSubtitles 哈希描述；按媒体库的顺序询问其字幕提供者（[插件 §8.11](plugins.zh-CN.md#811-按媒体库的提供者顺序)），一次询问所有缺少的语言。
4. **评分**：每个候选带有其提供者判定匹配的属性；服务端按 Bazarr 的权重评分（单集满分 360，哈希 359；电影满分 180，哈希 179；哈希仅在其他属性也吻合时计入，IMDb 与 TVDB ID 代表其蕴含的属性），取不低于最低分的最佳者，遵守配置的听障模式、必须包含与不得包含的词，单集还要求剧集、季与集匹配。
5. **保存**：字幕解码为 UTF-8，经设置所列的修正处理，经 `subtitle.FixText` 修复（有改动时以 UTF-8 保存），文本显示为听障字幕时加上听障标记，经 `core.Storage` 写在视频旁、视频所在目录下的某个目录中（`relative` 设置，单个目录名，如 `Subs`），或元数据目录中该条目的目录里（`metadata` 设置，`<id[:2]>/<id>/subtitles/`，取代 Bazarr 的绝对目录，适合无法或不宜写入的媒体库，如云盘），命名为 `<视频>.<语言>[.forced|.hi|.cc|.sdh].<扩展名>`，先写到旁边再改名；它成为条目的外挂流，无需重新 probe 视频。
6. **历史与通知**：下载连同提供者、分数与匹配项被记录，并记录一条 `subtitle.downloaded` 活动；解码失败或被管理员否决的字幕进入阻止列表，并搜索另一个。
7. **升级**：升级作业在设置给出的天数内，为低于满分下载的字幕再次搜索，只接受更好的，如 Bazarr 的 `upgrade_subtitles`；被替换的文件删除，历史把两者关联起来。
8. **手动**：管理员搜索视频的字幕并查看分数与匹配项，下载、上传、删除、修正与同步字幕，并在 §14.3 允许时提取内嵌字幕。

### 14.3 存储类别
字幕获取按[优化 §2.3](optimization.zh-CN.md#23-按存储类别的策略)的策略读取媒体：
* OpenSubtitles 哈希需要两次范围读取，每个文件只计算一次，保存到文件变化为止，重复搜索不再读取。
* 使用 probe 的媒体信息；不为字幕再次 probe。
* 内嵌文字字幕只在手动操作时、且只在本地和网络存储上提取为外挂字幕，来源是播放的提取缓存（[优化 §2.6](optimization.zh-CN.md#26-内嵌字幕)），缓存为空时由其单次 ffmpeg 运行填充；云存储上从不提取。
* 按音频同步需读取整条音轨，因此只在本地和网络存储上进行；云存储上的字幕只能手动按该视频的另一个字幕文件同步，以其字幕条代表语音；这在任何存储上都不读取视频。
* 语言识别与修正只读取字幕文件。
* Whisper 的音频（§14.5）只在本地和网络存储上提供，内嵌字幕也只在那里翻译；云存储上 Whisper 找不到字幕，只翻译字幕文件。

### 14.4 字幕提供者（`plugins/subtitles`）
移植 Bazarr 各提供者的第一方进程插件：位于 cookie、登录与 Cloudflare 之后的字幕站点（经与索引器同类的代理与 FlareSolverr），用管理员的 Anti-Captcha、CaptchaAI 或 DeathByCaptcha 账号识别验证码，压缩包（zip、RAR、7z），以及 WASM 清单无法列出地址的自托管提供者。其配置启用各提供者并给出设置；`SubtitleProviderService` 列出它们，按多种语言为一个视频搜索，返回带 release 名称、标记与匹配属性的候选，并下载其中之一。失败的提供者按 Bazarr 按失败类型给出的时长限流（请求过多、下载额度、服务不可用、认证），有的直到其站点时区的午夜，计数类失败在两分钟内达到五次才限流；状态保存在插件的数据目录中。OpenSubtitles.com 是其提供者之一。

### 14.5 翻译与转写
* **翻译**如 Bazarr 一样只能手动进行：管理员用 `SUBTITLE_TRANSLATOR` 插件的翻译器（[插件 §8.14](plugins.zh-CN.md#814-字幕翻译器)）把电影或剧集的一个外挂字幕文件，或本地与网络存储上的一条内嵌文字字幕流（取自播放的提取缓存），翻译成一种语言并带上其 forced 与听障标记：用指定的翻译器，否则用设置中的翻译器，否则用第一个已启用的。它作为 `subtitles.translate` 任务运行。保留 SubRip 标记的翻译器收到 SubRip 形式的字幕条，译文保存为 SubRip；其他翻译器收到字幕条的纯文本，配置文件保留原始格式时译文沿用字幕的格式（SubRip、ASS、SSA、WebVTT），否则为 SubRip。时间轴与样式保持不变，没有文字的字幕条不发送；设置要求时，SubRip 译文在第一条之前加一条署名（`# Subtitles translated with … #`），如 Bazarr 的 `add_translator_info`。译文如下载一样保存（§14.2），在历史中记为 `translated`，得分为设置中默认的最高分百分比。
* **转写**由 Whisper 完成，它如 Bazarr 一样是 `plugins/subtitles` 的一个提供者：把音频转写为其本身的语言，或翻译为英语，从不译为其他语言，也不经过翻译器。音频的语言取自音频流的标签；没有标签的流或标签含义不明（`und`、`mul`、`zxx` 等）时，Whisper 从第一条音频流的前 30 秒识别；有多条音频流时，转写与字幕语言相同的那条。插件从宿主 API（[插件 §8.13](plugins.zh-CN.md#813-字幕提供者)）把音频流式送往 whisper-asr-webservice 服务器，后者以 SubRip 应答。

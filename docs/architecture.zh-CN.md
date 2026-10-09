# Mavio 架构蓝图

> [English](architecture.md) | 简体中文

> 相关文档：[领域模型](domain.zh-CN.md) · [开发指南](development.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [测试策略](testing.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

> Mavio 是一个以 Go 为服务端核心的新一代自托管媒体服务器。它借鉴 Jellyfin 多年沉淀的领域知识、FFmpeg 管线与测试资产，但不以兼容 Jellyfin 生态为目标，选型一律取当下最现代、最合适的方案。

---

## 1. 定位与原则

### 1.1 目标
* **轻量高效**：低常驻内存、快速冷启动、高扫描吞吐与低首分片延迟来自现代技术栈本身——无运行时虚拟机的原生 Go 单二进制、以 WASM 沙箱代替重量级隔离、ffmpeg 硬件管线。
* **单二进制交付**：服务端为 `CGO_ENABLED=0` 的单一 Go 二进制；唯一外部运行时依赖是 `jellyfin-ffmpeg`。
* **强类型契约**：对内对外统一使用 Protobuf + Connect-RPC，未来的 Web / Expo / Desktop 客户端直接从契约生成 SDK。
* **插件隔离**：插件不进入宿主地址空间，WASM（wazero）沙箱与独立子进程（UDS）两种运行时共享同一套契约。
* **多数据库**：SQLite 与 PostgreSQL 均为一等公民。

### 1.2 非目标（当前阶段）
* 不兼容 Jellyfin 客户端生态、不提供 Jellyfin 数据迁移。Jellyfin API 兼容垫片（shim）留待后续单独评估。
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

工程工具（Nx、mise、buf、golangci-lint、CI）见[开发指南](development.zh-CN.md)。

### 2.1 wazero 作为共享原生能力层
wazero 同时承载三类负载：SQLite（ncruces）、图像编解码器（§6）和 WASM 插件（§7）。这样原生 C 库能力全部通过 WASM 获得，整个服务端保持纯 Go、可任意交叉编译。

注意：wazero 的编译器后端只支持 amd64 / arm64，其他架构（如 armv7、riscv64）会退化为解释器。对这些架构提供 build tag，切换为 `modernc.org/sqlite` 与纯 Go 编解码兜底，并在文档中标注为降级平台。

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
│   ├── web/                     # (后期) React + Vite + TanStack Router / Query
│   ├── mobile/                  # (后期) Expo
│   └── desktop/                 # (后期) Tauri 2，复用 web；可将 server 作为 sidecar 内嵌
│
├── libs/
│   ├── proto/                   # [Go + TS] 契约：*.proto 源文件、buf 配置、生成的 Go/TS 代码
│   ├── plugin/                  # [Go] 插件 SDK 与宿主运行时（wasm + 子进程）
│   ├── core/                    # [Go] 领域模型与仓储端口（见 domain.zh-CN.md）；不依赖任何基础设施
│   ├── store/                   # [Go] ent schema、sqlc 查询、Atlas 迁移、仓储实现
│   ├── naming/                  # [Go] 文件名 / 目录名解析
│   ├── metadata/                # [Go] NFO 读写、外部 ID、元数据合并策略
│   ├── subtitle/                # [Go] SRT / ASS / SSA / WebVTT 字幕解析、转换与字符集检测
│   ├── imaging/                 # [Go] 图片处理、拼贴、blurhash / thumbhash
│   ├── media/                   # [Go] 探测、关键帧、硬件加速、播放决策、转码规划、ffmpeg 进程守护
│   ├── library/                 # [Go] 扫描器、解析器链、任务调度
│   ├── streaming/               # [Go] HLS (CMAF) 播放列表、按需分片、分片缓存
│   ├── client/                  # (后期) [TS] 基于 connect-es 的客户端封装与 Query hooks
│   └── ui/                      # (后期) [TS] 跨端组件与设计 tokens
│
├── plugins/
│   ├── scraper-tmdb/            # [Go → wasm] TMDB 元数据刮削
│   ├── scraper-musicbrainz/     # [Go → wasm] 音乐元数据刮削
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
     ├──────▶ store ──▶ core
     └──────▶ plugin ──▶ proto

plugins/* ──▶ plugin ──▶ proto
```
* `core` 不依赖任何其他库；`naming`、`subtitle` 是纯函数库。
* `media` 只依赖 `core`，不感知数据库与插件。
* 只有 `apps/server` 负责装配。依赖方向由 depguard 规则在 CI 中强制。

---

## 4. 契约设计（libs/proto）

```text
libs/proto/
├── buf.yaml  buf.gen.yaml
├── mavio/
│   ├── library/v1/             # LibraryService、ItemService：媒体库、条目、媒体源与流、人员、扫描
│   ├── user/v1/                # UserService、UserDataService：账户、权限、偏好、按条目的用户状态
│   ├── system/v1/              # SystemService：健康检查、服务端信息
│   ├── playback/v1/            # （P3）客户端能力、播放决策、播放会话、进度上报
│   └── plugin/v1/              # 插件契约：PluginService（manifest、配置、生命周期）、
│                               #   MetadataProviderService、AuthProviderService、NotifierService
├── gen/go/                     # 生成的 protobuf-go + connect-go（Go 模块）
└── gen/ts/                     # (后期) 生成的 protobuf-es + connect-es（npm 包）
```

* 客户端能力用 Mavio 自己的 `ClientCapabilities` 表达（容器、编解码器、等级、HDR 能力、字幕交付方式、带宽），替代 Jellyfin 的 `DeviceProfile`。Go 类型位于 `libs/media/decision`，该包不依赖 `libs/proto`；`playback/v1` 中的消息与之对应，由 `apps/server` 负责二者转换。移植 StreamBuilder 测试时把 Jellyfin 的 profile 转换成该类型。
* 媒体流（直放、HLS 播放列表与分片、图片）不经过 Connect，走普通 HTTP，以便利用 Range、缓存与 CDN 语义。
* API 消息与领域模型（[领域模型](domain.zh-CN.md)）对应，但从不暴露图片路径、密码哈希等文件系统与内部细节。
* `plugin/v1` 对插件作者发布，自成一体：它定义自己的精简消息（`Lookup`、`Metadata`、`PersonCredit`、`RemoteImage` 等），而不引用 `library/v1`，两份契约可以各自演进。

---

## 5. 存储层（libs/store）：ent + sqlc，双数据库

### 5.1 职责划分
| ent 负责 | sqlc 负责 |
| :--- | :--- |
| **Schema 的唯一来源**（`internal/ent/schema`） | ent 无法以可移植方式表达的方言专属 SQL：任务队列（`INSERT … ON CONFLICT … WHERE` 去重；PostgreSQL 上用 `FOR UPDATE SKIP LOCKED` 原子租用） |
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
* **一致性测试**：同一套仓储测试分别跑在 SQLite（临时文件）与 PostgreSQL（testcontainers-go，或 `MAVIO_TEST_POSTGRES_DSN`）上，两者都通过才算通过。

### 5.4 SQLite 运行参数
WAL 模式、`synchronous=NORMAL`、`busy_timeout`、外键开启。单连接的写池执行写入与事务，并使用 `_txlock=immediate`（事务开始时即获取写锁，避免锁升级死锁）；另一个连接池负责读取。时间值使用 `_timefmt=unixepoch_micro`。关闭时执行 `PRAGMA optimize`。

---

## 6. 图像处理（libs/imaging）

### 6.1 需求与实现方案
需求来源：Jellyfin 的 `src/Jellyfin.Drawing.Skia`、`MediaBrowser.Controller/Drawing/ImageProcessingOptions.cs`、Trickplay 与 Photos 模块。

| 能力 | Mavio 实现 |
| :--- | :--- |
| 解码 JPEG / PNG / GIF / BMP | Go 标准库 + `x/image` |
| 解码 WebP | `x/image/webp` |
| 解码 AVIF / HEIC / JPEG XL | `gen2brain/avif`、`heic`、`jpegxl`（编译为 WASM，运行在 wazero 上） |
| 编码 WebP / JPEG / PNG | `gen2brain/webp`、`gen2brain/jpegli` 或标准库 JPEG、标准库 PNG |
| 编码 AVIF（可选） | `gen2brain/avif` |
| 缩放 / 裁剪 / 填充（Width、Height、Max*、Fill*） | `x/image/draw` CatmullRom / ApproxBiLinear；大图先按 2 的幂降采样 |
| EXIF 方向自动校正 | 轻量 EXIF 解析（仅 Orientation 等少量标签） |
| 锐化、模糊、背景色、前景层 | 自写卷积 / 高斯模糊（可并行分块）+ `image/draw` 合成 |
| 未播放数 / 播放进度角标 | 由客户端渲染 |
| 媒体库拼贴、启动屏（含文字） | `image/draw` 合成 + `go-text/typesetting` 排版，内置 Noto 字体子集覆盖 CJK |
| Trickplay 缩略图拼图 | ffmpeg：`fps` + `scale` + `tile` 滤镜一步输出拼图，可走硬件解码 |
| 视频截图、章节图 | ffmpeg |
| Blurhash / Thumbhash | 纯 Go，在 32px 缩略图上计算 |
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
* **调用即 Connect 请求**：宿主的 Connect 客户端使用一个特殊的 HTTP transport：它不走网络连接，而是把请求（路径、头、body）编码成信封写入插件内存，再调用模块导出的 `mavio_call(ptr, len) -> (ptr << 32 | len)`；`mavio_alloc` / `mavio_free` 管理共享缓冲区。插件内部由 `guest/wasm` 把请求交给插件注册的标准 Connect handler，并返回响应信封（状态码、头、body）。因此两种运行时的插件代码完全相同。
* **构建模式**：插件是 WASI reactor（`GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared`）；宿主为每个实例执行一次 `_initialize`。
* **宿主函数**（模块 `mavio`）：
  * `http_fetch(ptr, len) -> (handle << 32 | len)`：若 manifest 的 `http_hosts` 允许目标主机，由宿主代为执行 HTTP 请求；`http_read(handle, ptr)` 把结果拷贝进插件内存。分两步是为了避免在宿主函数中回调插件，Go 的 wasip1 运行时不支持这种重入。插件侧 SDK 把它们封装成普通的 `*http.Client`（`guest.HTTPClient()`）。
  * 插件把日志写到 stderr（以及 stdout），由宿主转发到自己的日志。
  * 配置通过 `Configure` RPC 下发，而不是宿主函数。
* **文件系统**：默认不挂载任何目录；manifest 中 `read_paths` 列出的目录以只读方式挂载在相同路径。
* **实例**：每个插件有自己的 wazero 运行时和一个模块实例池（实例数即并发调用数）。调用中发生 trap、panic、退出或超时的实例会被丢弃并替换，因此失败的调用不会影响宿主或后续调用。插件接受的配置会在每个实例的下一次调用前重放。
* **资源限制**：每个实例的内存页上限；每次调用的超时（`WithCloseOnContextDone`）；编译缓存（`CompilationCache`）持久化到磁盘，因为编译模块的耗时远大于实例化。
* **约束**：`wasip1` 下的 Go 没有套接字，所有网络访问都经过 `http_fetch`。

### 7.3 子进程运行时
* **统一使用 Unix Domain Socket**：Linux / macOS 原生支持，Windows 10 1803 起支持 `AF_UNIX`。socket 位于一个权限为 0700 的私有目录中，放在较短的基础路径下，因为 socket 路径长度限制在 100 字节左右。
* **握手**：宿主通过 `MAVIO_PLUGIN_SOCKET` / `MAVIO_PLUGIN_TOKEN` 传入 socket 路径与一次性 token。插件开始监听后在 stdout 输出一行 JSON（`{"mavio_plugin":1,"plugin_id":…}`）；宿主检查协议版本与插件 ID。此后每个请求都以 `Authorization: Bearer …` 携带 token。
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
├── guest/           # 插件侧：Handle（注册 Connect handler）、HTTPClient
│   ├── wasm/        # //go:build wasip1：模块导出、经宿主访问 HTTP
│   └── process/     # Serve：监听 socket、认证、握手、收到 Shutdown 后退出
├── internal/        # abi（WASM 信封）、proc（子进程协议）、clients、testplugin
└── schema/          # （后期）由 Go 结构体生成配置表单用的 JSON Schema
```
插件在 `init` 中用 `guest.Handle(pluginv1connect.New…ServiceHandler(impl))` 注册 handler；`wasip1` 构建导入 `guest/wasm`，原生构建调用 `process.Serve`。`internal/testplugin` 是同时为两种运行时构建的完整示例。

---

## 8. 媒体管线（libs/media）与硬件加速

### 8.1 子包划分
```text
libs/media/
├── probe/        # 调用 ffprobe（JSON 输出），规范化为 core 中的 MediaInfo（视频、音频、字幕流、HDR10 / HDR10+ / DV 元数据）
├── keyframes/    # 关键帧提取（ffprobe 读包级关键帧标志），用于按关键帧对齐切分 HLS 分片
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
* **逐个文件夹解析**：解析器根据文件夹在媒体库中的位置（媒体库类型，以及它是否位于剧集、季或艺术家之下），把文件夹及其条目映射为：文件夹本身所代表的条目（自成文件夹的电影、光盘镜像、剧集、季、专辑、有声书）、由其中文件构成的条目，以及接下来要解析的子文件夹及其位置。规则遵循 Jellyfin 的命名与解析规则；附加内容从所属条目的文件夹及其附加内容文件夹中查找。由于文件夹的结果只取决于其列表与位置，未变化的文件夹无需重新解析。
* **对账只读元数据**：对账遍历目录、比对文件 stat，只把新增或变化的文件送去 ffprobe 和刮削。
* **并发**：按媒体库根目录和目录子树并发遍历（`errgroup.SetLimit`），网络文件系统上使用较低的并发度。
* **一致性**：每次扫描在数据库中记一个"扫描代次"；扫描完成后，未被本代次看到的条目标记为缺失（先软删除，宽限期后再清理），避免挂载点暂时不可用导致整库被删。

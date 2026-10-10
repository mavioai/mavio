# AGENTS.md

> [English](AGENTS.md) | 简体中文

本文件是给 AI 编码代理（以及新加入的开发者）的仓库工作指南。本文件汇总必须遵守的规则，细节见下列文档；与这些文档冲突时以文档为准，并同步修正本文件。

| 文档 | 内容 |
| :--- | :--- |
| [docs/architecture.zh-CN.md](docs/architecture.zh-CN.md) | 系统设计：技术选型、仓库结构、依赖方向、存储、图像、插件、媒体管线、媒体库扫描 |
| [docs/domain.zh-CN.md](docs/domain.zh-CN.md) | 领域模型：实体、枚举、层级、规则、仓储端口 |
| [docs/development.zh-CN.md](docs/development.zh-CN.md) | 工具链、Nx、Go 模块约定、代码生成、代码质量、CI、发布 |
| [docs/testing.zh-CN.md](docs/testing.zh-CN.md) | 测试分层、从 Jellyfin 移植测试、测试媒体 |
| [docs/roadmap.zh-CN.md](docs/roadmap.zh-CN.md) | 阶段划分、完成标准、当前进度、风险 |
| [docs/optimization.zh-CN.md](docs/optimization.zh-CN.md) | 优化主题，每个附落地对照索引：存储 I/O 延迟缓解与推测式预热 |

## 项目概览

Mavio 是以 Go 为服务端核心的自托管媒体服务器，借鉴 Jellyfin 的领域知识与测试资产，但不兼容 Jellyfin 生态。许可证为 **GPL-3.0**。

当前阶段：**P6 服务端装配与分发**（见 [docs/roadmap.zh-CN.md](docs/roadmap.zh-CN.md)）。暂不实现任何 UI；`apps/web`、`apps/mobile`、`apps/desktop`、`libs/client`、`libs/ui` 尚未创建，不要提前创建。

## 目录结构

```text
apps/server/          Go 服务端：cmd/mavio、装配、Connect 服务实现（internal/）
libs/proto/           契约：*.proto、buf 配置、gen/go（生成代码，已提交）
libs/plugin/          插件 SDK 与宿主运行时（对外发布）
libs/core/            领域模型与端口接口，只依赖标准库
libs/store/           ent + sqlc + Atlas，SQLite 与 PostgreSQL
libs/naming/          文件名解析（纯函数）
libs/metadata/        NFO、外部 ID、分级体系、元数据合并
libs/subtitle/        字幕解析与转换（纯函数）
libs/imaging/         图片处理（无 CGO）
libs/media/           probe / keyframes / hwaccel / decision / planner / supervisor
libs/library/         扫描器、解析器链、任务调度
libs/streaming/       HLS (fMP4/CMAF)
plugins/<name>/       插件（默认编译为 wasip1 WASM）
tools/nx-go/          本地 Nx 插件：把每个 go.mod 推断为 Nx 项目并生成依赖图
tools/{fixtures,testport}/  测试媒体生成、测试用例移植
```

`libs/` 下的库按领域命名；一个库可以同时是 Go 模块和 npm 包。

## 环境与常用命令

工具版本由 `mise.toml` 锁定（go、node、pnpm、buf、golangci-lint、sqlc、jellyfin-ffmpeg）。

```bash
mise install            # 安装锁定版本的工具链
pnpm install            # 安装 Nx
```

```bash
pnpm nx run-many -t build test lint tidy-check     # 全量检查
pnpm nx affected -t build test lint tidy-check     # 仅检查受影响项目
pnpm nx run proto:generate                         # 重新生成 libs/proto/gen
pnpm nx run proto:buf-lint                         # buf lint + format 检查
pnpm nx run <project>:bench                        # 运行某个项目的基准测试
pnpm nx show projects                              # 列出项目（名称 = 目录名）
```

每个 Go 模块也能脱离 Nx 直接使用：`cd libs/naming && go test ./...`。

每个 Go 项目都有这些推断出来的 Nx 目标：`build`（`CGO_ENABLED=0`）、`test`、`bench`、`lint`、`tidy-check`（`GOWORK=off go mod tidy -diff`）。`libs/proto` 另有 `generate` 和 `buf-lint`。详见 [docs/development.zh-CN.md](docs/development.zh-CN.md) §2。

## Go 约定

### 模块与依赖
- 每个 `apps/*`、`libs/*`、`plugins/*` 目录是一个独立 Go 模块，模块路径为 `github.com/mavioai/mavio/<dir>`；`go` 指令统一为 `1.27.0`。
- 仓库内模块之间的依赖**同时**写进 `go.work` 和各自 `go.mod` 的 `require` + 相对路径 `replace`（例如 `apps/server/go.mod`）。这样 `GOWORK=off` 时 `go mod tidy` 和构建也能工作。新增内部依赖的命令：
  ```bash
  go mod edit -require=github.com/mavioai/mavio/libs/core@v0.0.0 -replace=github.com/mavioai/mavio/libs/core=../core
  GOWORK=off go mod tidy
  ```
- 新增模块：创建目录 → `go mod init github.com/mavioai/mavio/<dir>` → 把 `go` 指令改为 `1.27.0` → 加入 `go.work` 的 `use`。Nx 会自动识别，无需 `project.json`；只有需要覆盖默认目标时才添加（参考 `plugins/scraper-tmdb/project.json`）。
- `libs/proto` 和 `libs/plugin` 会被第三方插件引用，需要独立打 tag（如 `libs/proto/v0.1.0`）；发布 `libs/plugin` 前先发布它依赖的 `libs/proto` 版本，并更新 `require`。其余模块不承诺对外 API 稳定。
- 依赖方向由 `.golangci.yml` 中的 depguard 规则强制（架构文档 §3.2）：`core` 只依赖标准库；`naming`、`subtitle` 只依赖 `core`；`media`、`imaging`、`metadata`、`store` 不依赖 `library`、`streaming`、`plugin`；`library`、`streaming` 只通过 `core` 中的端口访问存储；`plugin` 只依赖 `proto`；任何 lib 都不依赖 `apps/*` 或 `plugins/*`。**只有 `apps/server` 负责装配。** 需要调整依赖方向时，先修改架构文档。

### 第三方依赖准则
- **禁止 CGO**：服务端以 `CGO_ENABLED=0` 构建。需要原生能力时，优先选择 ffmpeg 子进程或 wazero 运行的 WASM 模块。
- SQLite 驱动使用 `ncruces/go-sqlite3`；媒体库变更检测通过全量对账扫描实现（架构文档 §9）。
- 许可证必须兼容 GPL-3.0：MIT、BSD、ISC、Apache-2.0、MPL-2.0、LGPL、GPL-3.0 可用；**禁止 GPL-2.0-only**。
- 能用标准库或 `golang.org/x/*` 解决的，不引入第三方库（例如并发用 `errgroup.SetLimit`）。
- 从 Jellyfin 移植的文件名规则保留 .NET 正则语法，基于 `dlclark/regexp2` 执行；不要改写为 `regexp`。

### 代码风格
- 格式化：gofumpt + goimports（本仓库导入单独分组）。由 golangci-lint 检查。
- 日志一律用 `log/slog`，带上下文时用 `*Context` 版本；不用 `fmt.Print*` 输出日志。
- 第一个参数是 `context.Context`；可能阻塞的操作都必须响应取消。
- 错误用 `fmt.Errorf("...: %w", err)` 包装，用 `errors.Is` / `errors.As` 判断；不要吞掉错误。
- 不使用包级可变全局状态；依赖通过构造函数显式注入。
- 文件系统访问媒体库时使用 `os.Root` 限定在库根目录内。
- 优先使用现代标准库特性：`iter.Seq` 流水线、`slices`/`maps`、`net/http` 路由模式、`http.Protocols`（h2c）。
- 每个包都有包注释（`doc.go`）；导出标识符都要有文档注释。

## Protobuf / Connect 约定
- 契约源文件在 `libs/proto/mavio/<domain>/v1/`，使用 `edition = "2023"`；包名为 `mavio.<domain>.v1`。
- Go 代码生成使用 Opaque API（`default_api_level=API_OPAQUE`，通过 Getter/Setter 访问字段），Connect 使用 `simple` 签名（handler 直接收发消息，不再包一层 `connect.Request`）。
- 修改 `.proto` 后运行 `pnpm nx run proto:generate`，并**提交生成的 `gen/` 代码**；CI 会检查生成代码是否最新。不要手动编辑 `gen/` 下的文件。
- 必须通过 `buf lint`（STANDARD）和 `buf breaking`（FILE）。已发布的字段编号不得复用；需要破坏性变更时新增 `v2` 包。
- 只读 RPC 标注 `option idempotency_level = NO_SIDE_EFFECTS;`。
- 用 protovalidate 注解校验请求；规则只对已设置的字段生效，因此必填字段要标注 `(buf.validate.field).required = true`。
- 媒体字节流（直放、HLS、图片）走普通 HTTP，不走 Connect。

## 存储约定（libs/store）
- ent schema（`libs/store/internal/ent/schema`）是表结构的**唯一来源**。修改后运行 `pnpm nx run store:generate`，再在 `libs/store` 下运行 `go run ./internal/cmd/migrategen -dialect sqlite -name <名称>` 以及 `-dialect postgres` 的同样命令（需要 Docker），并审核两份迁移。**已合并的迁移文件不得修改**，只能追加新迁移。
- ent 负责 CRUD、关系与动态条目查询；sqlc 只用于任务队列这类方言专属 SQL，按方言各写一份（总数预算 ≤ 30 条）。
- 多步写入使用 `Store.InTx`，它把 ent 与 sqlc 绑定到同一个 `*sql.Tx`。
- 所有仓储测试必须同时在 SQLite 和 PostgreSQL 上通过。
- 对上层只暴露 `libs/core` 中定义的仓储接口，ent / sqlc 类型不得泄漏出 `libs/store`。

## 测试约定
- 测试优先写成表驱动测试；断言失败信息使用 `got = …, want = …` 格式。
- 单元测试不访问网络、不依赖真实 ffmpeg 或硬件；需要这些的测试放在集成测试中，条件不满足时 `t.Skip` 并写明原因。
- 测试媒体由 `pnpm nx run fixtures:media` 生成到 `.fixtures/`，不提交二进制媒体文件。测试通过 `fixtures.Require(t, name)` 获取样本，样本缺失时自动跳过。
- 涉及超时、定时器、空闲回收的并发逻辑，用 `testing/synctest` 写成确定性测试。
- 编写基准测试时使用 `b.Loop()`；基准测试不纳入 CI。
- **从 Jellyfin 移植的测试用例与测试数据**放在对应库的 `testdata/` 下，按用途命名子目录（如 `testdata/nfo/`、`testdata/probe/`），**不得创建以 Jellyfin 命名的目录**。这些文件由 `tools/testport` 生成并记录来源，不得手工修改；需要跳过的用例在 Go 测试中按用例 ID 跳过并写明原因，不允许静默丢弃。移植规则见 [docs/testing.zh-CN.md](docs/testing.zh-CN.md) §2。

## 文档
- 所有文档都提供英文与简体中文两个版本，内容逐节对应：英文为 `<name>.md`，中文为 `<name>.zh-CN.md`（如 `docs/architecture.md` 与 `docs/architecture.zh-CN.md`、`AGENTS.md` 与 `AGENTS.zh-CN.md`）。两个文件顶部互相链接。
- 修改任一语言版本时，必须在同一个提交中同步修改另一个版本。
- 文档只描述采用的方案；评估后未采用的方案不写入文档。

## 提交与 PR
- 使用 Conventional Commits，scope 为 Nx 项目名：`feat(naming): parse absolute episode numbers`、`build(nx): …`、`docs: …`。
- 每个提交保持单一目的；提交前确保 `pnpm nx affected -t build test lint tidy-check` 通过。
- 架构、依赖方向、技术选型的变更必须同步更新相关文档和本文件（中英文两个版本）。
- 修改 `libs/core` 中的领域模型时，必须在同一个提交中更新 [docs/domain.zh-CN.md](docs/domain.zh-CN.md)（中英文两个版本）。

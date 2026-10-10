# Mavio 开发指南

> [English](development.md) | 简体中文

> 相关文档：[架构](architecture.zh-CN.md) · [领域模型](domain.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [测试策略](testing.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

---

## 1. 工具链与环境

工具版本由 `mise.toml` 锁定：

| 工具 | 用途 |
| :--- | :--- |
| go | 服务端、库、插件 |
| node + pnpm | Nx，以及后续的 TS 应用 |
| buf | Protobuf 的 lint、格式化、破坏性变更检查与代码生成 |
| golangci-lint | Go 代码检查与格式检查 |
| sqlc | 生成 `libs/store` 中的方言专属查询代码 |
| jellyfin-ffmpeg | 媒体管线、测试媒体生成与集成测试（`PATH` 上的 `ffmpeg` / `ffprobe`） |

```bash
mise install            # 安装锁定版本的工具链
pnpm install            # 安装 Nx
```

迁移由 `libs/store/internal/cmd/migrategen` 生成，它以库的形式使用 Atlas，不需要 Atlas CLI。生成 PostgreSQL 迁移和运行 PostgreSQL 一致性测试需要 Docker。`libs/store/pgtest` 依次在 `DOCKER_HOST`、Docker CLI 当前上下文（`docker context use`）的引擎、Docker Desktop / OrbStack / Colima / Rancher Desktop / Podman 的常见 socket 中找到的第一个引擎上启动容器，无需额外配置。

---

## 2. Monorepo 与 Nx

### 2.1 为什么用 Nx
Go 是服务端主语言；未来还会有 Web（React + Vite）、Mobile（Expo）、Desktop（Tauri 2）三类 TS 应用，并共享由 `libs/proto` 生成的客户端代码。这是一个跨语言的 monorepo，需要统一的依赖图、受影响分析与任务缓存：
* Nx 对 Expo（`@nx/expo`）、Vite / React 有官方插件；
* Go 项目由仓库内的本地 Nx 插件 `tools/nx-go` 推断：每个 `go.mod` 自动成为一个项目，并根据 `go.mod` 中的仓库内 `require` 生成依赖图；
* 用 tags 表达边界：`type:app|lib|plugin`、`lang:go|ts`、`scope:server|client|shared`。TS 侧由 `@nx/enforce-module-boundaries` 约束，Go 侧由 depguard 约束（§5）。

Nx 只做编排与缓存：每个 Go 模块仍然可以独立地 `go build` / `go test`。

### 2.2 推断出的目标
`tools/nx-go` 为每个 Go 项目生成以下目标（项目名 = 目录名）：

| 目标 | 命令 | 缓存 |
| :--- | :--- | :--- |
| `build` | `go build ./...`，`CGO_ENABLED=0` | 是 |
| `test` | `go test ./...` | 是 |
| `bench` | `go test -run=NONE -bench=. -benchmem ./...` | 否 |
| `lint` | `golangci-lint run --allow-parallel-runners ./...`（Nx 会并行运行多个项目） | 是 |
| `tidy-check` | `go mod tidy -diff`，`GOWORK=off` | 是 |
| `generate` | `buf generate`（仅限含 `buf.yaml` 的项目） | 否 |
| `buf-lint` | `buf lint && buf format --diff --exit-code`（仅限含 `buf.yaml` 的项目） | 是 |

需要覆盖默认目标时，在 `go.mod` 旁放一个 `project.json`；例如 `plugins/scraper-tmdb/project.json` 把构建改为 `wasip1` WASM 模块。

### 2.3 常用命令
```bash
pnpm nx run-many -t build test lint tidy-check     # 全量检查
pnpm nx affected -t build test lint tidy-check     # 仅检查受影响项目
pnpm nx run proto:generate                         # 重新生成 libs/proto/gen
pnpm nx run proto:buf-lint                         # buf lint + format 检查
pnpm nx run <project>:bench                        # 运行某个项目的基准测试
pnpm nx show projects                              # 列出项目
pnpm nx graph                                      # 查看项目依赖图
```

`go test ./...` 不会跨模块执行；跨模块运行请使用 `nx run-many` / `nx affected`。

---

## 3. Go 模块

### 3.1 约定
* 每个 Go 库、应用、插件各有自己的 `go.mod`；模块路径为 `github.com/mavioai/mavio/<dir>`，例如 `github.com/mavioai/mavio/libs/media`。`go` 指令统一为 `1.27.0`。
* 仓库内模块之间的依赖同时写在 `go.work` 和各自 `go.mod` 的 `require` + 相对路径 `replace` 中：`go.work` 服务于 gopls 和跨模块开发，`replace` 保证 `GOWORK=off` 时 `go mod tidy` 和构建也能工作（`go mod tidy` 不读取 `go.work`）。`tidy-check` 目标据此校验每个模块的 `go.mod`。
* 二进制工具由 mise 锁定版本；代码生成插件（`protoc-gen-go`、`protoc-gen-connect-go`，以及后续的 `sqlc`、`ent`）通过所在模块 go.mod 的 `tool` 指令锁定版本。
* 若后续发现某些模块总是同步修改，应合并模块，避免过度拆分。

### 3.2 新增模块
1. 创建目录，运行 `go mod init github.com/mavioai/mavio/<dir>`。
2. 把 `go` 指令改为 `1.27.0`。
3. 把目录加入 `go.work` 的 `use`。
4. 添加带包注释的 `doc.go`。

Nx 会自动识别新模块，无需 `project.json`。

### 3.3 新增仓库内依赖
```bash
go mod edit -require=github.com/mavioai/mavio/libs/core@v0.0.0 -replace=github.com/mavioai/mavio/libs/core=../core
GOWORK=off go mod tidy
```
新的依赖必须符合[架构 §3.2](architecture.zh-CN.md#32-依赖方向) 的依赖方向；违反时会被 depguard 拦截（§5）。

---

## 4. 代码生成

### 4.1 Protobuf 与 Connect
* 契约源文件在 `libs/proto/mavio/<domain>/v1/`，使用 `edition = "2023"`；包名为 `mavio.<domain>.v1`。
* `buf generate`（配置见 `libs/proto/buf.gen.yaml`）以 `default_api_level=API_OPAQUE` 运行 `protoc-gen-go`（通过 Getter/Setter 访问字段），以 `simple` 运行 `protoc-gen-connect-go`（handler 直接收发消息）。managed 模式把 `go_package` 设置在 `github.com/mavioai/mavio/libs/proto/gen/go` 之下。
* 同一次运行还生成 `libs/proto/openapi/gen/mavio.openapi.json`，即服务端所提供服务的 OpenAPI 文档，由 `protoc-gen-connect-openapi` 合并 `openapi/base.yaml`（说明、Bearer 认证、普通 HTTP 媒体路由）。它通过 `go run` 以 `buf.gen.yaml` 中固定的版本运行，使其依赖不进入第三方插件所导入的 go.mod；`openapi` 包嵌入该文档，文档与 `gen/` 一样提交。
* 修改 `.proto` 后运行 `pnpm nx run proto:generate`，并**提交生成的 `gen/` 代码**。不要手动编辑 `gen/` 下的文件。
* `buf lint` 使用 STANDARD 规则，`buf breaking` 使用 FILE 规则。已发布的字段编号不得复用；破坏性变更放入新的 `v2` 包。
* 只读 RPC 标注 `option idempotency_level = NO_SIDE_EFFECTS;`。
* 请求校验使用 protovalidate 注解（`buf.validate`）。edition 2023 下每个字段都跟踪是否设置，规则只对已设置的字段生效，因此**必填字段必须标注 `(buf.validate.field).required = true`**。`libs/proto/validate_test.go` 检查这些规则。

### 4.2 数据库 Schema 与查询
流水线见[架构 §5.2](architecture.zh-CN.md#52-生成流水线)。修改 `libs/store/internal/ent/schema` 中的 ent schema 之后：

```bash
pnpm nx run store:generate                                   # 生成 ent 客户端与 sqlc 代码
# 在 libs/store 下
go run ./internal/cmd/migrategen -dialect sqlite -name <名称>
go run ./internal/cmd/migrategen -dialect postgres -name <名称>  # 会启动 PostgreSQL 容器，需要 Docker
```

审核两份迁移，并与 schema 修改一起提交。已合并的迁移文件不得修改，只能追加新迁移。方言专属查询放在 `libs/store/queries/<dialect>/` 中，由 `sqlc`（版本锁定在 `mise.toml`）在 `store:generate` 中生成。

### 4.3 插件
插件用 `guest.Handle` 注册 Connect handler，并在构建时选择运行时（见[架构 §7.4](architecture.zh-CN.md#74-插件-sdk-结构)）：

```bash
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .   # WASM 运行时
go build -o plugin .                                                     # 子进程运行时
```

插件目录包含 `manifest.json`，以及 `plugin.wasm` 或 `plugin`（Windows 上为 `plugin.exe`）。

---

## 5. 代码质量

`.golangci.yml`（golangci-lint v2）作用于所有 Go 模块：
* **检查器**：`standard` 集合，外加 `depguard`、`errorlint`、`misspell`、`nolintlint`、`unconvert`、`usestdlibvars`；生成代码不参与检查。
* **格式化**：`gofumpt` 与 `goimports`，`github.com/mavioai/mavio` 的导入单独分组。
* **depguard**：把[架构 §3.2](architecture.zh-CN.md#32-依赖方向) 的依赖方向写成规则（`core` 只依赖标准库、纯函数库只依赖 `core`、插件 SDK 只依赖 `libs/proto`、任何 lib 都不导入 `apps/*` 或 `plugins/*` 等）。调整依赖方向时，先修改架构文档，再修改规则。

---

## 6. 持续集成

`.github/workflows/ci.yml` 在推送到 `main` 和提交 PR 时运行：
* **check**（Linux）：`nx affected -t buf-lint lint tidy-check build test`；重新生成代码，有任何差异即失败；PR 上对照目标分支运行 `buf breaking`。
* **test**：在 linux arm64、macOS、Windows 上运行 `nx run-many -t test build`。
* **media**（Linux、macOS）：安装 jellyfin-ffmpeg，生成测试媒体，不使用 Nx 缓存运行 `media`、`streaming` 与 `server` 的测试，使 ffmpeg 集成测试与冒烟测试在 runner 的硬件上执行。

* **dist**（Linux）：`nx run server:dist`，确保服务端能交叉编译到所有受支持的平台。
* **image**（Linux）：为 linux/amd64 与 linux/arm64 构建容器镜像（不推送），再运行 amd64 镜像直到健康检查通过。

CI 中的工具链同样来自 `mise.toml`（`jdx/mise-action`）；**media** 以外的任务不安装 jellyfin-ffmpeg（`MISE_DISABLE_TOOLS`）。

---

## 7. 发布与版本

`libs/proto` 与 `libs/plugin` 会被第三方插件引用，使用带路径前缀的 tag 独立发布（如 `libs/proto/v0.1.0`、`libs/plugin/v0.1.0`）。它们的 `replace` 对使用方不生效，因此：
1. 先发布 `libs/proto`。
2. 把 `libs/plugin` 的 `require` 更新到该版本，再发布 `libs/plugin`。

其余模块只在仓库内使用，不承诺对外 API 稳定。

服务端的发布二进制用 `VERSION=v0.1.0 pnpm nx run server:dist` 构建；镜像在仓库根目录用 `docker buildx build -f apps/server/Dockerfile --build-arg VERSION=v0.1.0 --platform linux/amd64,linux/arm64 .` 构建。Dockerfile 锁定与 `mise.toml` 相同的 jellyfin-ffmpeg 版本，并记录其两个 Linux 便携版的 SHA-256 摘要；升级 jellyfin-ffmpeg 时需同时修改这两个文件，两处锁定不一致时一个测试（`apps/server/internal/buildinfo`）会失败。镜像还会构建 DLNA 插件，并将其放在插件种子目录 `/usr/lib/mavio/plugins` 中。

第一方进程插件用其 `dist` 目标打包：`pnpm nx run dlna:dist` 按清单中的版本为服务端的每个平台构建 DLNA 插件到 `plugins/dlna/dist/`，每个平台一个目录 zip 包，并生成列出它们及其 SHA-256 摘要的 `catalog.json`；zip 包的 URL 相对于目录。

---

## 8. 提交与文档

* 使用 Conventional Commits，scope 为 Nx 项目名：`feat(naming): parse absolute episode numbers`、`build(nx): …`、`docs: …`。每个提交保持单一目的；提交前确保 `pnpm nx affected -t build test lint tidy-check` 通过。
* 所有文档都提供英文（`<name>.md`）与简体中文（`<name>.zh-CN.md`）两个版本，内容逐节对应；两个文件顶部互相链接，并在同一个提交中同步修改。
* 文档只描述采用的方案。

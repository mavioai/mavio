# Mavio 开发指南

> [English](development.md) | 简体中文

> 相关文档：[架构](architecture.zh-CN.md) · [领域模型](domain.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [测试策略](testing.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

---

## 1. 工具链与环境

工具版本由 `mise.toml` 锁定：

| 工具 | 用途 |
| :--- | :--- |
| go | 服务端、库、插件 |
| node + pnpm | Nx、`protoc-gen-es`，以及后续的 TS 应用 |
| buf | Protobuf 的 lint、格式化、破坏性变更检查与代码生成 |
| golangci-lint | Go 代码检查与格式检查 |
| sqlc | 生成 `libs/store` 中的方言专属查询代码 |
| protoc-gen-buffa、protoc-gen-buffa-packaging、protoc-gen-connect-rust | `libs/proto` 的 Rust 代码生成，取自其发布构建 |
| java（Temurin）+ gradle | Android 各应用与 `libs/android` |
| jellyfin-ffmpeg | 媒体管线、测试媒体生成与集成测试（`PATH` 上的 `ffmpeg` / `ffprobe`） |

```bash
mise install            # 安装锁定版本的工具链
pnpm install            # 安装 Nx
```

Rust 由 `libs/client-rust/rust-toolchain.toml` 固定，rustup 在该目录首次运行 `cargo` 时安装它；只需安装 rustup 本身。

客户端的所有目标都在该工具链上构建，包括 tvOS 目标 `aarch64-apple-tvos` 与 `aarch64-apple-tvos-sim`，其标准库与其他目标一样由 rustup 安装（`rustup target add aarch64-apple-tvos`）。

迁移由 `libs/store/internal/cmd/migrategen` 生成，它以库的形式使用 Atlas，不需要 Atlas CLI。生成 PostgreSQL 迁移和运行 PostgreSQL 一致性测试需要 Docker。`libs/store/pgtest` 依次在 `DOCKER_HOST`、Docker CLI 当前上下文（`docker context use`）的引擎、Docker Desktop / OrbStack / Colima / Rancher Desktop / Podman 的常见 socket 中找到的第一个引擎上启动容器，无需额外配置。

---

## 2. Monorepo 与 Nx

### 2.1 为什么用 Nx
Go 是服务端主语言；未来还会加入以 TypeScript（Web UI）、Swift、Kotlin、Go（桌面应用）与 Rust（它们的共享核心）编写的[客户端](clients.zh-CN.md)，并使用由 `libs/proto` 生成的客户端代码。这是一个跨语言的 monorepo，需要统一的依赖图、受影响分析与任务缓存：
* Nx 对 Vite / React 有官方插件；Cargo、Gradle 与 `xcodebuild` 通过各自的目标运行；
* Go 项目由仓库内的本地 Nx 插件 `tools/nx-go` 推断：每个 `go.mod` 自动成为一个项目，并根据 `go.mod` 中的仓库内 `require` 生成依赖图；
* Cargo workspace 由本地 Nx 插件 `tools/nx-cargo` 推断：每个含 `[workspace]` 表的 `Cargo.toml` 成为一个项目，依赖其 `[workspace.metadata.nx]` 的 `implicitDependencies` 所列的项目（`libs/client-rust` 列出 `proto`，它包含后者生成的 Rust 代码）；
* 用 tags 表达边界：`type:app|lib|plugin`、`lang:go|ts|rust|swift|kotlin`、`scope:server|client|shared`。TS 侧由 `@nx/enforce-module-boundaries` 约束，Go 侧由 depguard 约束（§5）。

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

`tools/nx-cargo` 为每个 Cargo workspace 生成 `build`（`cargo build --locked --all-targets`）、`test`（`cargo test --locked`）、`bench`（`cargo bench --locked`，不缓存）与 `lint`（`cargo fmt --check` 与 `cargo clippy --locked --all-targets -- -D warnings`）。

`package.json` 的脚本同样是目标：`libs/proto` 与 `libs/design` 有 `typecheck`（`tsc -p tsconfig.json`，TypeScript 7），`libs/design` 还有 `generate`（§4.4）。同时含 `go.mod` 与 `package.json` 的目录（如 `libs/design`）是一个项目，具有两者的目标。

`libs/client-rust/capi` 既是 Cargo workspace 的 crate，又是 Go 模块，即 C ABI 的 Go 绑定；作为位于 Cargo workspace 目录中的 Go 项目，它是 Nx 项目 `capi`，Nx 把其中的文件算作它自己的，因此 `client-rust` 的 Cargo 目标在输入中列出它的 Rust 源码（`libs/client-rust/project.json`）。其 `test` 在 `client-rust:build` 之后以 `CGO_ENABLED=0` 运行（与应用的构建方式一致），从 `target/debug` 加载该库；`generate` 为 `libs/client-rust/proto` 中 C ABI 的消息运行 `buf generate`，把 Go 写入 `gen/`、Rust 写入 `src/gen/`（均提交入库），`buf-lint` 检查这些消息。

`libs/client-ts` 与 `apps/web` 同样有 `typecheck`，`libs/client-ts` 有 `test`（Vitest），`apps/web` 有 `build`（Vite），它把 Web UI 写入 `apps/server/internal/webui/dist`。服务端的 `build` 与 `dist` 依赖它（`apps/server/project.json`），使二进制内嵌该 UI；直接 `go build` 则内嵌该目录现有的内容，目录中只有 `.gitkeep` 时，服务端提供一个说明未随 UI 构建的页面。`pnpm --filter @mavio/web dev` 用 Vite 提供 UI，并把其 API 调用发往 8686 端口上的服务端。

Apple 各项目需要 Xcode，只在 macOS 上构建；它们带有标签 `platform:apple`，Linux 与 Windows 的 CI 任务将其排除。`libs/apple` 有 `core`，运行 `scripts/build-core.sh`：为每个 Apple 目标构建 `mavio-uniffi`，把每个共享库包装为动态 framework，写出 `build/MavioCoreFFI.xcframework`，以及 UniFFI 生成的 Swift API（`build/MavioCore`）；采用 framework 而非静态库，是因为应用中其他 Rust 代码的静态库（AetherEngine 的 LibDovi）各自带有 Rust 标准库，其符号会冲突。其 `ass` 目标运行 `scripts/build-ass.sh`：以 meson 为每个 Apple 目标从源码构建固定版本的 libass 及 FreeType、FriBidi 与 HarfBuzz，写出 `build/Libass.xcframework`，每个平台一个静态库，源码与中间构建保留在 `build/libass`；耗时约四分钟，脚本不变时再次运行什么也不做，CI 按脚本的哈希缓存其输出。其 `build` 与 `test` 在 `core` 与 `ass` 之后运行 `swift build` 与 `swift test`，`lint` 以仓库的 `.swift-format` 运行 `swift format lint --strict`。`apps/ios`、`apps/tvos` 与 `apps/macos` 有 `build`（不签名的 `xcodebuild`，iPhone 与 Apple TV 面向模拟器；只构建 arm64，与 AetherEngine 的模拟器库一致）、`lint`，以及 `generate`：它用 `libs/apple/scripts/gen-project.py` 写出应用的 `Mavio.xcodeproj`，该工程提交入库、不手工编辑，其源码是同步文件夹 `Mavio/`，添加文件无须改动工程。

Android 各项目需要 Android SDK（通过 `ANDROID_HOME` 找到），运行 `mise.toml` 锁定的 Gradle 与 JDK；每个 Gradle 构建的 `gradle/wrapper/gradle-wrapper.properties` 为 Android Studio 指明同一 Gradle 版本，不带 wrapper 脚本。它们带有标签 `platform:android`。`libs/android`（Nx 名称 `android-lib`）是由模块 `core`、`player` 与 `ui` 组成的 Gradle 构建，带有一份版本目录（`gradle/libs.versions.toml`），各应用的 settings 也读取它；其 `core` 目标运行 `scripts/build-core.sh`：用锁定的 NDK（缺失时以 `sdkmanager` 安装）为 `arm64-v8a`、`armeabi-v7a` 与 `x86_64` 构建 `mavio-uniffi`，把各个库、UniFFI 生成的 Kotlin API，以及为本机构建、由单元测试通过 JNA 加载的核心写入 `core/build/rust`。其 `mpv` 目标运行 `scripts/build-mpv.sh`：以 mpv-android 的构建脚本与 JNI 层，按锁定版本为同样的 ABI 从源码构建 libmpv、FFmpeg 及其依赖库，写入 `player/build/mpv`，源码与中间构建保存在 `build/mpv`；首次构建约需一小时，脚本未变时再次运行不做任何事。它需要 meson 与 ninja（`mise.toml`）、autotools、pkg-config 与 gperf，在 macOS 上还需要 GNU sed、coreutils 与 bash 4 或更高版本（Homebrew）；CI 按脚本的哈希缓存其输出。其 `ffmpeg` 目标运行 `scripts/build-ffmpeg.sh`：按版本目录中的 Media3 版本，把 Media3 的 FFmpeg 解码扩展构建到 `player/build/ffmpeg`——FFmpeg 的音频解码器按 LGPL 构建为静态库，与扩展的 JNI 层链接为各 ABI 的 `libffmpegJNI.so`，以及其音频渲染器的 Java 源码。其 `build`、`test` 与 `lint` 运行 `assembleDebug`、`testDebugUnitTest` 与 `lintDebug`。`apps/android` 与 `apps/android-tv` 是引入 `libs/android` 的 Gradle 构建，按坐标（`dev.mavio.android:core`）依赖其模块；它们有 `build` 与 `lint`。各应用的构建共享该 included build，因此 Gradle 目标逐个运行。

`libs/apple` 与 `libs/android` 的 `core` 目标对每个工具链只运行一次 Cargo 构建，覆盖其全部目标，使各目标并行编译，使用 `release` profile；设置 `MAVIO_CORE_PROFILE=dev` 则不做优化地构建，耗时只是其一小部分，CI 即如此。macOS 的核心、libass 与应用是通用二进制，包含 `arm64` 与 `x86_64`；设置 `MAVIO_MACOS_ARCHS=arm64` 则它们都只为 Apple 芯片构建，CI 即如此。

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
* 同一次运行还用 `protoc-gen-es` 把 TypeScript 生成到 `gen/ts`（protobuf-es 消息，以及 connect-es 使用的服务描述；`libs/proto` 同时是导出它们的 npm 包 `@mavio/proto`），用 `protoc-gen-buffa`（消息，在 `gen/rust/proto`）与 `protoc-gen-connect-rust`（客户端，在 `gen/rust/connect`）把 Rust 生成到 `gen/rust`，每棵树的 `mod.rs` 由 `protoc-gen-buffa-packaging` 生成。`libs/client-rust` 的 crate `mavio-api` 包含这两棵 Rust 树。
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

### 4.4 设计系统
`pnpm nx run design:generate` 运行 `libs/design/build/build.js`，把设计系统（[客户端 §6](clients.zh-CN.md#6-设计系统)）的源文件编译到 `libs/design/gen`：

| 源文件 | 内容 |
| :--- | :--- |
| `tokens/*.json` | DTCG 格式的设计 tokens，由 Style Dictionary 解析；每个主题（`color.dark`、`color.light`）、字号体系（`typography.default`、`typography.tv`）与设备形态（`layout.*`）各一个文件 |
| `icons/icons.json` | 语义图标名到 Phosphor 名称的映射；Regular 与 Fill 两种 SVG 取自固定版本的 `@phosphor-icons/core`，复制到 `icons/svg` |
| `strings/<locale>.json` | ICU MessageFormat 字符串，带说明及其限定的平台；英文（`en.json`）为源语言 |

| 输出 | 用于 |
| :--- | :--- |
| `gen/ts` | Web UI：`tokens.ts`、`tokens.css`（CSS 自定义属性，浅色主题位于 `prefers-color-scheme` 下）、`icons.ts`（路径数据）、`strings.ts`；由 npm 包 `@mavio/design` 导出 |
| `gen/swift` | Apple 平台：`MavioTokens.swift`（SwiftUI）、`Icons.xcassets`、`Localizable.xcstrings` |
| `gen/kotlin` | Android：`dev/mavio/design/MavioTokens.kt`（Compose）、`res/drawable`（矢量图）、`res/values*/strings.xml` |
| `gen/go` | 桌面应用：Go 包 `design`（模块 `github.com/mavioai/mavio/libs/design`），含 tokens、图标路径数据、字符串及其格式化函数 |

* 任一主题中有文字与背景组合低于 WCAG AA 对比度（4.5:1）、某条消息的复数缺少其语言的 CLDR 类别、或某条译文的参数与源语言不同时，生成器报错。
* 消息限于每个平台的格式都能表达的 ICU 子集：文本、`{arg}`，以及覆盖整条消息、各形式只含计数（`#`）与文本的一个复数。
* `gen/` 与其源文件一起提交，不得手工修改。

---

## 5. 代码质量

`.golangci.yml`（golangci-lint v2）作用于所有 Go 模块：
* **检查器**：`standard` 集合，外加 `depguard`、`errorlint`、`misspell`、`nolintlint`、`unconvert`、`usestdlibvars`；生成代码不参与检查。
* **格式化**：`gofumpt` 与 `goimports`，`github.com/mavioai/mavio` 的导入单独分组。
* **depguard**：把[架构 §3.2](architecture.zh-CN.md#32-依赖方向) 的依赖方向写成规则（`core` 只依赖标准库、纯函数库只依赖 `core`、插件 SDK 只依赖 `libs/proto`、任何 lib 都不导入 `apps/*` 或 `plugins/*` 等）。调整依赖方向时，先修改架构文档，再修改规则。

---

## 6. 持续集成

`.github/workflows/ci.yml` 在推送到 `main` 和提交 PR 时运行。**plan** 找出改动影响的范围后，所有任务同时开始，改动未影响其项目的任务被跳过；对 `.github` 或 `mise.toml` 的改动影响所有项目：
* **plan**（Linux）：列出受影响的 Apple 与 Android 项目，并判断服务端或构成它的库是否受影响。
* **check**（Linux）：`nx affected -t buf-lint lint tidy-check typecheck build test`；重新生成代码，有任何差异即失败；PR 上对照目标分支运行 `buf breaking`。
* **test**：在 linux arm64、macOS、Windows 上运行 `nx run-many -t typecheck test build`；PR 上改为运行 `nx affected`。
* **apple**（macOS），在 **plan** 发现 Apple 项目受影响时运行：对这些项目运行 `nx run-many -t lint build test`，核心以 `dev` profile 构建，生成的 Xcode 工程与 `generate` 写出的不一致时失败。
* **android**（Linux），在 **plan** 发现 Android 项目受影响时运行：对这些项目运行 `nx run-many -t lint build test`，核心以 `dev` profile 构建。
* **media**（Linux、macOS），在服务端受影响时运行：安装 jellyfin-ffmpeg，生成测试媒体，不使用 Nx 缓存运行 `media`、`streaming` 与 `server` 的测试，使 ffmpeg 集成测试与冒烟测试在 runner 的硬件上执行。
* **dist**（Linux），在服务端受影响时运行：`nx run server:dist`，确保服务端能交叉编译到所有受支持的平台；各平台并行构建。
* **image**（Linux），在服务端受影响时运行：为 linux/amd64 与 linux/arm64 构建容器镜像（不推送），再运行 amd64 镜像直到健康检查通过。

CI 中的工具链同样来自 `mise.toml`（`jdx/mise-action`）；**media** 以外的任务不安装 jellyfin-ffmpeg，**android** 以外的任务不安装 Java 与 Gradle（`MISE_DISABLE_TOOLS`）。Rust 来自 `rust-toolchain.toml`，由 runner 上的 rustup 安装；**check**、**test**、**apple** 与 **android** 缓存 Cargo 的构建（`Swatinem/rust-cache`），**apple** 与 **android** 以其核心的 profile 作为缓存键的一部分。**apple** 缓存 SwiftPM 与 `xcodebuild` 共用的 Swift 包克隆（`~/Library/Caches/org.swift.swiftpm`），以 `Package.resolved` 文件作为缓存键；**android** 缓存 Gradle 的用户目录，包括其依赖与构建缓存（`gradle/actions/setup-gradle`）。**check**、**test**、**media** 与 **dist** 缓存 Go 的构建与模块，**check** 另缓存 golangci-lint 的结果（`.github/actions/go-cache`）：缓存只在推送到 `main` 时保存，PR 恢复其中最新的一份。**media** 以 `-count=1` 运行 Go 的测试，避免重放缓存的测试结果。缓存只在推送到 `main` 时保存；这类推送的任务结束后，**caches** 删除 `main` 上已被更新缓存取代的缓存，即为较早提交或较早工具链保存的缓存；`.github/workflows/pr-caches.yml` 在 PR 关闭后删除其缓存。

---

## 7. 发布与版本

`libs/proto` 与 `libs/plugin` 会被第三方插件引用，使用带路径前缀的 tag 独立发布（如 `libs/proto/v0.1.0`、`libs/plugin/v0.1.0`）。它们的 `replace` 对使用方不生效，因此：
1. 先发布 `libs/proto`。
2. 把 `libs/plugin` 的 `require` 更新到该版本，再发布 `libs/plugin`。

其余模块只在仓库内使用，不承诺对外 API 稳定。

服务端的发布二进制用 `VERSION=v0.1.0 pnpm nx run server:dist` 构建；镜像在仓库根目录用 `docker buildx build -f apps/server/Dockerfile --build-arg VERSION=v0.1.0 --platform linux/amd64,linux/arm64 .` 构建。Dockerfile 锁定与 `mise.toml` 相同的 jellyfin-ffmpeg 版本，并记录其两个 Linux 便携版的 SHA-256 摘要；升级 jellyfin-ffmpeg 时需同时修改这两个文件，两处锁定不一致时一个测试（`apps/server/internal/buildinfo`）会失败。镜像还会构建 DLNA、索引器与下载客户端插件，并将它们放在插件种子目录 `/usr/lib/mavio/plugins` 中。

第一方进程插件用其 `dist` 目标打包：`pnpm nx run dlna:dist`（或 `indexers:dist`、`download-clients:dist`）按清单中的版本为服务端的每个平台构建 DLNA（或索引器、下载客户端）插件到 `plugins/<name>/dist/`，每个平台一个目录 zip 包，并生成列出它们及其 SHA-256 摘要的 `catalog.json`；zip 包的 URL 相对于目录。

`scraper-tmdb` 的发布构建把 `MAVIO_TMDB_KEY` 设为 Mavio 自己的 TMDB API 密钥，`pnpm nx run scraper-tmdb:build` 把它编入插件，作为 TMDB 拒绝 Radarr 令牌后使用的密钥（[架构 §13.2](architecture.zh-CN.md#132-数据源与编号)）；该密钥从不提交。

---

## 8. 提交与文档

* 使用 Conventional Commits，scope 为 Nx 项目名：`feat(naming): parse absolute episode numbers`、`build(nx): …`、`docs: …`。每个提交保持单一目的；提交前确保 `pnpm nx affected -t build test lint tidy-check` 通过。
* 所有文档都提供英文（`<name>.md`）与简体中文（`<name>.zh-CN.md`）两个版本，内容逐节对应；两个文件顶部互相链接，并在同一个提交中同步修改。
* 文档只描述采用的方案。

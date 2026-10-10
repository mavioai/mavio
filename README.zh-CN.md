# Mavio

> [English](README.md) | 简体中文

Mavio 是以 Go 为服务端核心的自托管媒体服务器：一个 `CGO_ENABLED=0` 的单一二进制加上 `jellyfin-ffmpeg`，强类型的 Protobuf / Connect API，同时支持 SQLite 与 PostgreSQL，插件运行在 WASM 沙箱或独立子进程中。

状态：早期开发（P6 服务端装配与分发），见[路线图](docs/roadmap.zh-CN.md)。

## 文档

| 文档 | 内容 |
| :--- | :--- |
| [架构](docs/architecture.zh-CN.md) | 系统设计：技术选型、仓库结构、存储、图像、插件、媒体管线、媒体库扫描 |
| [领域模型](docs/domain.zh-CN.md) | `libs/core` 的实体、枚举、层级、规则与仓储端口 |
| [开发指南](docs/development.zh-CN.md) | 工具链、Nx、Go 模块约定、代码生成、代码质量、CI、发布 |
| [测试策略](docs/testing.zh-CN.md) | 测试分层、从 Jellyfin 移植测试、测试媒体 |
| [路线图](docs/roadmap.zh-CN.md) | 阶段划分、完成标准、当前进度、风险 |
| [AGENTS.zh-CN.md](AGENTS.zh-CN.md) | AI 编码代理与贡献者的工作规则 |

## 快速开始

```bash
mise install
pnpm install
pnpm nx run-many -t build test
```

带示例媒体库运行服务端，开发用播放器位于 http://localhost:8686/dev/player：

```bash
pnpm nx run fixtures:dev-library
cd apps/server && go run ./cmd/mavio --dev --dev-library ../../.fixtures/dev-library
```

或以容器运行，把媒体挂载在 `/media` 下：

```bash
docker buildx build -f apps/server/Dockerfile -t mavio --load .
docker run -p 8686:8686 -v mavio-config:/config -v mavio-cache:/cache -v /path/to/media:/media:ro mavio
```

## 许可证

[GPL-3.0](LICENSE)

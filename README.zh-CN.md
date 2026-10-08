# Mavio

> [English](README.md) | 简体中文

Mavio 是以 Go 为服务端核心的自托管媒体服务器：一个 `CGO_ENABLED=0` 的单一二进制加上 `jellyfin-ffmpeg`，强类型的 Protobuf / Connect API，同时支持 SQLite 与 PostgreSQL，插件运行在 WASM 沙箱或独立子进程中。

状态：早期开发（P0 工程地基），见[路线图](docs/roadmap.zh-CN.md)。

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

## 许可证

[GPL-3.0](LICENSE)

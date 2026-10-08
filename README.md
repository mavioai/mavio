# Mavio

> English | [简体中文](README.zh-CN.md)

Mavio is a self-hosted media server with a Go server at its core: a single `CGO_ENABLED=0` binary plus `jellyfin-ffmpeg`, strongly typed Protobuf / Connect APIs, SQLite and PostgreSQL support, and sandboxed WASM and child-process plugins.

Status: early development (P0 engineering foundation). See the [roadmap](docs/roadmap.md).

## Documentation

| Document | Contents |
| :--- | :--- |
| [Architecture](docs/architecture.md) | System design: technology choices, repository structure, storage, imaging, plugins, media pipeline, library scanning |
| [Development](docs/development.md) | Toolchain, Nx, Go module conventions, code generation, code quality, CI, releases |
| [Testing](docs/testing.md) | Test layers, porting tests from Jellyfin, test media, benchmarks |
| [Roadmap](docs/roadmap.md) | Phases, gates and performance budgets, current status, risks |
| [AGENTS.md](AGENTS.md) | Working rules for AI coding agents and contributors |

## Quick Start

```bash
mise install
pnpm install
pnpm nx run-many -t build test
```

## License

[GPL-3.0](LICENSE)

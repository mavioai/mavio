# Mavio

> English | [简体中文](README.zh-CN.md)

Mavio is a self-hosted media server with a Go server at its core: a single `CGO_ENABLED=0` binary plus `jellyfin-ffmpeg`, strongly typed Protobuf / Connect APIs, SQLite and PostgreSQL support, and sandboxed WASM and child-process plugins.

Status: early development (P6 server assembly and distribution). See the [roadmap](docs/roadmap.md).

## Documentation

| Document | Contents |
| :--- | :--- |
| [Architecture](docs/architecture.md) | System design: technology choices, repository structure, storage, imaging, plugins, media pipeline, library scanning |
| [Domain Model](docs/domain.md) | Entities, enumerations, hierarchies, rules and repository ports of `libs/core` |
| [Development](docs/development.md) | Toolchain, Nx, Go module conventions, code generation, code quality, CI, releases |
| [Testing](docs/testing.md) | Test layers, porting tests from Jellyfin, test media |
| [Roadmap](docs/roadmap.md) | Phases, completion criteria, current status, risks |
| [AGENTS.md](AGENTS.md) | Working rules for AI coding agents and contributors |

## Quick Start

```bash
mise install
pnpm install
pnpm nx run-many -t build test
```

Run the server with the sample library and the development player at http://localhost:8686/dev/player:

```bash
pnpm nx run fixtures:dev-library
cd apps/server && go run ./cmd/mavio -dev -dev-library ../../.fixtures/dev-library
```

Or as a container, with media mounted under `/media`:

```bash
docker buildx build -f apps/server/Dockerfile -t mavio --load .
docker run -p 8686:8686 -v mavio-config:/config -v mavio-cache:/cache -v /path/to/media:/media:ro mavio
```

## License

[GPL-3.0](LICENSE)

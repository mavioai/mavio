# AGENTS.md

> English | [简体中文](AGENTS.zh-CN.md)

This is the repository working guide for AI coding agents (and new developers). It summarizes the rules; the details live in the documents below. When this file conflicts with them, the documents win and this file must be corrected accordingly.

| Document | Contents |
| :--- | :--- |
| [docs/architecture.md](docs/architecture.md) | System design: technology choices, repository structure, dependency direction, storage, imaging, plugins, media pipeline, library scanning |
| [docs/domain.md](docs/domain.md) | Domain model: entities, enumerations, hierarchies, rules, repository ports |
| [docs/development.md](docs/development.md) | Toolchain, Nx, Go module conventions, code generation, code quality, CI, releases |
| [docs/testing.md](docs/testing.md) | Test layers, porting tests from Jellyfin, test media |
| [docs/roadmap.md](docs/roadmap.md) | Phases, completion criteria, current status, risks |

## Project Overview

Mavio is a self-hosted media server with a Go server at its core. It draws on Jellyfin's domain knowledge and test assets but is not compatible with the Jellyfin ecosystem. It is licensed under **GPL-3.0**.

Current phase: **P0 engineering foundation** (see [docs/roadmap.md](docs/roadmap.md)). No UI is being built yet; `apps/web`, `apps/mobile`, `apps/desktop`, `libs/client` and `libs/ui` do not exist yet — do not create them ahead of time.

## Directory Structure

```text
apps/server/          Go server: cmd/mavio, wiring, Connect service implementations (internal/)
libs/proto/           Contracts: *.proto, buf config, gen/go (generated code, committed)
libs/plugin/          Plugin SDK and host runtimes (published)
libs/core/            Domain model and port interfaces; standard library only
libs/store/           ent + sqlc + Atlas, SQLite and PostgreSQL
libs/naming/          File name parsing (pure functions)
libs/metadata/        NFO, external IDs, metadata merging
libs/subtitle/        Subtitle parsing and conversion (pure functions)
libs/imaging/         Image processing (no CGO)
libs/media/           probe / keyframes / hwaccel / decision / planner / supervisor
libs/library/         Scanner, resolver chain, job scheduling
libs/streaming/       HLS (fMP4/CMAF)
plugins/<name>/       Plugins (built as wasip1 WASM by default)
tools/nx-go/          Local Nx plugin: infers an Nx project from every go.mod and builds the dependency graph
tools/{fixtures,testport}/  Test media generation, test case porting
```

Libraries under `libs/` are named by domain; a single library can be both a Go module and an npm package.

## Environment and Common Commands

Tool versions are pinned in `mise.toml` (go, node, pnpm, buf, golangci-lint).

```bash
mise install            # Install the pinned toolchain
pnpm install            # Install Nx
```

```bash
pnpm nx run-many -t build test lint tidy-check     # Check everything
pnpm nx affected -t build test lint tidy-check     # Check affected projects only
pnpm nx run proto:generate                         # Regenerate libs/proto/gen
pnpm nx run proto:buf-lint                         # buf lint + format check
pnpm nx run <project>:bench                        # Run a project's benchmarks
pnpm nx show projects                              # List projects (name = directory name)
```

Every Go module also works without Nx: `cd libs/naming && go test ./...`.

Every Go project has these inferred Nx targets: `build` (`CGO_ENABLED=0`), `test`, `bench`, `lint`, `tidy-check` (`GOWORK=off go mod tidy -diff`). `libs/proto` additionally has `generate` and `buf-lint`. See [docs/development.md](docs/development.md) §2.

## Go Conventions

### Modules and Dependencies
- Every `apps/*`, `libs/*` and `plugins/*` directory is an independent Go module with module path `github.com/mavioai/mavio/<dir>`; the `go` directive is `1.27.0` everywhere.
- Dependencies between in-repo modules are declared **both** in `go.work` and in each `go.mod` as `require` + relative-path `replace` (see `apps/server/go.mod`). This keeps `go mod tidy` and builds working with `GOWORK=off`. To add an internal dependency:
  ```bash
  go mod edit -require=github.com/mavioai/mavio/libs/core@v0.0.0 -replace=github.com/mavioai/mavio/libs/core=../core
  GOWORK=off go mod tidy
  ```
- New module: create the directory → `go mod init github.com/mavioai/mavio/<dir>` → change the `go` directive to `1.27.0` → add it to `use` in `go.work`. Nx picks it up automatically with no `project.json`; add one only to override default targets (see `plugins/scraper-tmdb/project.json`).
- `libs/proto` and `libs/plugin` are imported by third-party plugins and are tagged independently (e.g. `libs/proto/v0.1.0`); before releasing `libs/plugin`, release the `libs/proto` version it depends on and update its `require`. Other modules promise no stable external API.
- The dependency direction is enforced by the depguard rules in `.golangci.yml` (architecture document §3.2): `core` depends only on the standard library; `naming` and `subtitle` depend only on `core`; `media`, `imaging`, `metadata` and `store` do not depend on `library`, `streaming` or `plugin`; `library` and `streaming` access storage only through ports in `core`; `plugin` depends only on `proto`; no lib depends on `apps/*` or `plugins/*`. **Only `apps/server` does wiring.** To change the dependency direction, update the architecture document first.

### Third-party Dependency Guidelines
- **No CGO**: the server is built with `CGO_ENABLED=0`. When native capabilities are needed, prefer an ffmpeg child process or a WASM module running on wazero.
- The SQLite driver is `ncruces/go-sqlite3`; library change detection is done with full reconciliation scans (architecture document §9).
- Licenses must be compatible with GPL-3.0: MIT, BSD, ISC, Apache-2.0, MPL-2.0, LGPL and GPL-3.0 are fine; **GPL-2.0-only is not allowed**.
- Do not add a third-party library for anything the standard library or `golang.org/x/*` can do (e.g. use `errgroup.SetLimit` for concurrency).

### Code Style
- Formatting: gofumpt + goimports (this repository's imports in their own group), checked by golangci-lint.
- Always log with `log/slog`, using the `*Context` variants when a context is available; never use `fmt.Print*` for logging.
- `context.Context` is the first parameter; every potentially blocking operation must honor cancellation.
- Wrap errors with `fmt.Errorf("...: %w", err)` and inspect them with `errors.Is` / `errors.As`; never swallow errors.
- No mutable package-level global state; inject dependencies explicitly through constructors.
- Use `os.Root` to confine file system access to the library root when touching media libraries.
- Prefer modern standard library features: `iter.Seq` pipelines, `slices`/`maps`, `net/http` routing patterns, `http.Protocols` (h2c).
- Every package has a package comment (`doc.go`); every exported identifier has a doc comment.

## Protobuf / Connect Conventions
- Contract sources live in `libs/proto/mavio/<domain>/v1/` and use `edition = "2023"`; the package name is `mavio.<domain>.v1`.
- Go code is generated with the Opaque API (`default_api_level=API_OPAQUE`, fields accessed through getters/setters), and Connect uses `simple` signatures (handlers take and return messages directly, without a `connect.Request` wrapper).
- After changing a `.proto`, run `pnpm nx run proto:generate` and **commit the generated `gen/` code**; CI checks that generated code is up to date. Never edit files under `gen/` by hand.
- `buf lint` (STANDARD) and `buf breaking` (FILE) must pass. Published field numbers must never be reused; breaking changes go into a new `v2` package.
- Read-only RPCs are annotated with `option idempotency_level = NO_SIDE_EFFECTS;`.
- Validate requests with protovalidate annotations; rules apply only to set fields, so mark mandatory fields `(buf.validate.field).required = true`.
- Media byte streams (direct play, HLS, images) go over plain HTTP, not Connect.

## Storage Conventions (libs/store)
- The ent schema (`libs/store/internal/ent/schema`) is the **single source of truth** for the database schema. After changing it, run `pnpm nx run store:generate`, then `go run ./internal/cmd/migrategen -dialect sqlite -name <name>` and the same with `-dialect postgres` (needs Docker) in `libs/store`, and review both migrations. **Merged migration files must never be modified**; only append new migrations.
- ent handles CRUD, relationships and dynamic item queries; sqlc is used only for dialect-specific SQL such as the job queue, written once per dialect (total budget ≤ 30 queries).
- Use `Store.InTx` for multi-step writes; it binds ent and sqlc to one `*sql.Tx`.
- Every repository test must pass on both SQLite and PostgreSQL.
- Only the repository interfaces defined in `libs/core` are exposed upward; ent / sqlc types must not leak out of `libs/store`.

## Testing Conventions
- Prefer table-driven tests; failure messages use the `got = …, want = …` format.
- Unit tests do not touch the network or depend on a real ffmpeg or hardware; tests that need those are integration tests and call `t.Skip` with a reason when the prerequisites are missing.
- Test media is generated by `pnpm nx run fixtures:media` into `.fixtures/`; binary media files are never committed. Tests obtain fixtures with `fixtures.Require(t, name)`, which skips when a fixture is missing.
- Concurrency logic involving timeouts, timers or idle reaping is tested deterministically with `testing/synctest`.
- Benchmarks, when written, use `b.Loop()`; they are not part of CI.
- **Test cases and test data ported from Jellyfin** go under the corresponding library's `testdata/`, in subdirectories named by purpose (e.g. `testdata/nfo/`, `testdata/probe/`); **never create a directory named after Jellyfin**. They are generated by `tools/testport`, record their origin, and are never edited by hand; skip cases in the Go test by case ID with a reason — silently dropping cases is not allowed. See [docs/testing.md](docs/testing.md) §2 for the porting rules.

## Documentation
- Every document exists in English and Simplified Chinese with section-by-section correspondence: English is `<name>.md`, Chinese is `<name>.zh-CN.md` (e.g. `docs/architecture.md` and `docs/architecture.zh-CN.md`, `AGENTS.md` and `AGENTS.zh-CN.md`). The two files link to each other at the top.
- When changing either language version, update the other in the same commit.
- Documents describe only the approaches that were adopted; options that were evaluated and not adopted are not written into documents.

## Commits and PRs
- Use Conventional Commits with the Nx project name as scope: `feat(naming): parse absolute episode numbers`, `build(nx): …`, `docs: …`.
- Keep each commit to a single purpose; make sure `pnpm nx affected -t build test lint tidy-check` passes before committing.
- Changes to the architecture, dependency direction or technology choices must update the relevant documents and this file (both language versions).
- Changes to the domain model in `libs/core` must update [docs/domain.md](docs/domain.md) (both language versions) in the same commit.

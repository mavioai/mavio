# testport

> English | [简体中文](README.zh-CN.md)

Extracts parameterized test cases and test assets from a Jellyfin checkout into Mavio's `testdata/` directories. See `docs/testing.md` §2 for the format and rules.

```bash
# in tools/testport
go run ./cmd/testport -jellyfin ~/src/jellyfin -only libs/naming   # port one target
go run ./cmd/testport -jellyfin ~/src/jellyfin -dry-run             # report counts for every target
```

`-jellyfin` defaults to `$MAVIO_JELLYFIN`. `mapping.json` maps Jellyfin test projects and test data directories to Mavio targets; the longest matching source wins.

- `internal/csharp`: a C# lexer and a parser for the subset of declarations and expressions used in xUnit tests.
- `extract.go`: turns `[Theory]` / `[Fact]` methods into cases.
- `port.go`: applies the mapping, writes case files and copies assets with `SOURCES.json`.

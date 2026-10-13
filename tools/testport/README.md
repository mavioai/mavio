# testport

> English | [简体中文](README.zh-CN.md)

Extracts parameterized test cases and test assets from checkouts of Jellyfin, Sonarr, Radarr, Prowlarr and Bazarr into Mavio's `testdata/` directories. See `docs/testing.md` §2 for the format and rules.

```bash
# in tools/testport
go run ./cmd/testport -checkout jellyfin=~/src/jellyfin -only libs/naming   # port one target
go run ./cmd/testport -checkout sonarr=~/src/sonarr -checkout radarr=~/src/radarr -only libs/release
go run ./cmd/testport -dry-run                                              # report counts, checkouts from $MAVIO_<NAME>
```

`mapping.json` lists the source repositories; each maps its test projects, test data directories and constant files to Mavio targets, and the longest matching source wins. A source's checkout is given as `-checkout <name>=<dir>` or `$MAVIO_<NAME>` (e.g. `$MAVIO_JELLYFIN`); sources without a checkout are skipped.

- `internal/csharp`: a C# lexer and a parser for the subset of declarations and expressions used in xUnit and NUnit tests.
- `internal/python`: a Python tokenizer, and a reader of the test functions, decorators and literal values of pytest modules.
- `extract.go`: turns xUnit `[Theory]` / `[Fact]` and NUnit `[TestCase]` / `[TestCaseSource]` / `[Test]` methods into cases.
- `pytest.go`: turns pytest `test*` functions into cases: `@pytest.mark.parametrize` (stacked marks as their product, `pytest.param` with its marks) as theories, the rest as facts, with the reasons of `skip`, `skipif` and `xfail`.
- `port.go`: applies the mapping, writes case files, copies assets with `SOURCES.json` (each asset mapping can include and exclude files by name) and extracts the `const` fields of the mapped C# files.

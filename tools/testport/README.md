# testport

> English | [简体中文](README.zh-CN.md)

One-off tool that extracts parameterized test cases (`[InlineData]`,
`[MemberData]`, `TheoryData`) from Jellyfin's C# test projects into JSON/YAML
under each library's `testdata/` directory (in subdirectories named by
purpose, e.g. `testdata/nfo/`), recording the source file and test name for
every case.

See `docs/testing.md` §2 for the mapping from Jellyfin test projects to
Mavio libraries.

Status: not implemented yet (P0).

# testport

> [English](README.md) | 简体中文

一次性迁移工具：从 Jellyfin 的 C# 测试工程中提取参数化测试用例（`[InlineData]`、
`[MemberData]`、`TheoryData`），输出为 JSON / YAML，放到各库的 `testdata/` 目录下
（按用途命名子目录，如 `testdata/nfo/`），并为每个用例记录来源文件与测试名。

Jellyfin 测试工程与 Mavio 各库的对应关系见 `docs/testing.zh-CN.md` §2。

状态：尚未实现（P0）。

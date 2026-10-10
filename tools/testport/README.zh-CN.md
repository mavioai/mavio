# testport

> [English](README.md) | 简体中文

从 Jellyfin、Sonarr 与 Radarr 的仓库中提取参数化测试用例与测试资产，写入 Mavio 各库的 `testdata/` 目录。格式与规则见 `docs/testing.zh-CN.md` §2。

```bash
# 在 tools/testport 下
go run ./cmd/testport -checkout jellyfin=~/src/jellyfin -only libs/naming   # 移植一个目标
go run ./cmd/testport -checkout sonarr=~/src/sonarr -checkout radarr=~/src/radarr -only libs/release
go run ./cmd/testport -dry-run                                              # 统计，不写文件；仓库路径取自 $MAVIO_<NAME>
```

`mapping.json` 列出各来源仓库，每个来源把自己的测试工程、测试数据目录与常量文件映射到 Mavio 的目标位置，匹配时取最长的源路径。来源仓库路径用 `-checkout <name>=<dir>` 或 `$MAVIO_<NAME>`（如 `$MAVIO_JELLYFIN`）给出；没有给出路径的来源会被跳过。

- `internal/csharp`：C# 词法分析器，以及覆盖 xUnit 与 NUnit 测试所用声明与表达式子集的解析器。
- `extract.go`：把 xUnit 的 `[Theory]` / `[Fact]` 与 NUnit 的 `[TestCase]` / `[TestCaseSource]` / `[Test]` 方法转换为用例。
- `port.go`：应用映射，写出用例文件，复制资产并生成 `SOURCES.json`，提取所映射 C# 文件中的 `const` 字段。

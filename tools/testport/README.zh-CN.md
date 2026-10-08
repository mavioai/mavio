# testport

> [English](README.md) | 简体中文

从 Jellyfin 仓库中提取参数化测试用例与测试资产，写入 Mavio 各库的 `testdata/` 目录。格式与规则见 `docs/testing.zh-CN.md` §2。

```bash
# 在 tools/testport 下
go run ./cmd/testport -jellyfin ~/src/jellyfin -only libs/naming   # 移植一个目标
go run ./cmd/testport -jellyfin ~/src/jellyfin -dry-run             # 统计所有目标，不写文件
```

`-jellyfin` 默认取 `$MAVIO_JELLYFIN`。`mapping.json` 把 Jellyfin 的测试工程、测试数据目录与常量文件映射到 Mavio 的目标位置，匹配时取最长的源路径。

- `internal/csharp`：C# 词法分析器，以及覆盖 xUnit 测试所用声明与表达式子集的解析器。
- `extract.go`：把 `[Theory]` / `[Fact]` 方法转换为用例。
- `port.go`：应用映射，写出用例文件，复制资产并生成 `SOURCES.json`，提取所映射 C# 文件中的 `const` 字段。

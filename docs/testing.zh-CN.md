# Mavio 测试策略

> [English](testing.md) | 简体中文

> 相关文档：[架构](architecture.zh-CN.md) · [领域模型](domain.zh-CN.md) · [开发指南](development.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

---

## 1. 测试分层

| 层次 | 范围 | 位置 | 运行时机 |
| :--- | :--- | :--- | :--- |
| 单元测试 | 纯逻辑；不访问网络、不依赖真实 ffmpeg 或硬件。表驱动，很多用例移植自 Jellyfin | 代码旁的 `*_test.go`，数据放在 `testdata/` | 每次 CI，全部平台 |
| 一致性测试 | 同一套仓储测试分别跑在 SQLite（临时文件）与 PostgreSQL（testcontainers-go 容器，或 `MAVIO_TEST_POSTGRES_DSN` 指定的服务器；没有 Docker 或使用 `-short` 时跳过）上，两者都必须通过 | `libs/store` | 每次 CI（PostgreSQL 在 Linux 上运行） |
| 集成测试 | 真实 ffmpeg / ffprobe、生成的测试媒体、真实硬件编码器；条件不满足时 `t.Skip` 并写明原因 | 代码旁，依赖前置条件 | 具备条件的 CI；厂商路径只在现有硬件（维护者的机器、GitHub 托管 runner）上运行 |
| 冒烟测试 | 在服务端内把已完成的库串起来（见[路线图 §1](roadmap.zh-CN.md#1-推进原则)） | `apps/server/internal/smoke` | 每次 CI |

涉及超时、定时器、空闲回收的并发逻辑，用 `testing/synctest` 写成确定性测试。

---

## 2. 从 Jellyfin 移植测试用例

### 2.1 方法
提取由 `tools/testport` 完成；`tools/testport/mapping.json` 把 Jellyfin 的测试工程与测试数据目录映射到 Mavio 的目标位置（即 §2.2 的映射表）。

1. **参数化用例**：`[InlineData]` 用例，以及由 `TheoryData.Add(…)`、`yield return` 或集合初始化器提供数据的 `[MemberData]` 用例，按 C# 测试类各输出一个 JSON 文件，放在目标的 `testdata/cases/` 下，文件名为去掉 `Tests` 后缀的 snake_case（如 `TV/EpisodeNumberTests.cs` → `testdata/cases/tv/episode_number.json`）。参数按形参名记录，并补全声明的默认值。常量转为 JSON 值；枚举成员等符号转为 `{"$symbol": …}`，标志组合转为 `{"$flags": […]}`，构造调用转为 `{"$new": …}`，其余非常量表达式以源码原文保留为 `{"$expr": …}`。
2. **来源记录**：每个用例文件记录 Jellyfin 仓库、提交与源文件；每条用例记录方法、行号与稳定 ID（`Method/n`）。
3. **Jellyfin 自身的跳过**：Jellyfin 中标记跳过的用例（`Skip = …`），以及被 Jellyfin 注释掉的 `[InlineData]` 行（通常是 `// TODO:` / `// FIXME:` 标注的已知失败），保留并带上 `skip` 原因。
4. **生成的文件不得手工修改。** 依赖 .NET 特有行为、或与 Mavio 设计取舍不同的用例，在 Go 测试中按用例 ID 跳过并写明原因；**不允许静默丢弃**。
5. **非参数化测试**（`[Fact]`）列在每个用例文件的 `facts` 中，人工翻译成 Go 测试；运行时计算的 `[MemberData]` 与 `[ClassData]` 列在 `unsupported` 中并写明原因，人工移植。
6. **测试资产**（NFO 样例、ffprobe JSON、字幕文件、关键帧数据、设备配置）复制到目标的 `testdata/` 下，按用途命名子目录（如 `testdata/nfo/`、`testdata/probe/`），并附带 `SOURCES.json` 记录每个文件的来源。用例引用的、定义在其他 C# 文件中的常量（如捕获的 ffmpeg 输出）提取为一个 JSON 文件，其 `constants` 映射以 `Class.Name` 为键，测试据此解析 `{"$symbol": …}` 参数。仓库中不出现以 Jellyfin 命名的目录。
7. 每实现一个库时按目标运行：在 `tools/testport` 下执行 `go run ./cmd/testport -jellyfin <Jellyfin 仓库路径> -only libs/naming`。一个库移植完成后，新增用例直接写在 Mavio 中。

### 2.2 映射表
| Jellyfin 测试 | 资产与要点 | Mavio 目标 |
| :--- | :--- | :--- |
| `tests/Jellyfin.Naming.Tests`（TV / Video / Music / AudioBook / Book / ExternalFiles，约 575 条参数化用例） | 剧集 / 季 / 多集 / 绝对集数 / 日期型剧集；多版本、分卷（stack）、花絮（extras）、3D、日期清理；外挂文件 | `libs/naming` |
| `tests/Jellyfin.Server.Implementations.Tests/Library` | 电影、剧集、季、音频解析器；忽略规则（`.ignore`）；排序；媒体流选择（`MediaStreamSelectorTests`） | `libs/library`（解析器链、忽略规则）；`libs/media/decision`（流选择） |
| `tests/Jellyfin.XbmcMetadata.Tests`（Parsers + Test Data） | 电影、剧集、季、单集、音乐专辑、艺人、音乐视频的 NFO 样例 | `libs/metadata/nfo` |
| `tests/Jellyfin.Providers.Tests`（Tmdb / MediaInfo / Lyrics / Music / TV / ExternalId） | TMDB 工具函数与演职员处理、缺失集检测、歌词解析、外部 ID | `plugins/scraper-tmdb`、`libs/metadata` |
| `tests/Jellyfin.MediaEncoding.Tests/Probing`（ffprobe JSON 样本） | 探测结果规范化：码率缺失、隔行扫描、时长精度等边界情况 | `libs/media/probe` |
| `tests/Jellyfin.MediaEncoding.Tests/Subtitles` | SRT / ASS / SSA 解析与编码 | `libs/subtitle` |
| `tests/Jellyfin.MediaEncoding.Tests`（EncoderValidator、ApplePlatformHelper） | ffmpeg 版本解析、编码器能力解析、Apple 平台判定 | `libs/media/hwaccel` |
| `tests/Jellyfin.Controller.Tests/MediaEncoding`（`EncodingHelper*Tests`） | 转码参数推导、音频比特流过滤器、杜比视界（DV）处理、音频编码器推断 | `libs/media/planner` |
| `tests/Jellyfin.Model.Tests/Dlna`（`StreamBuilderTests` + 19 个 `DeviceProfile-*.json`） | 直放 / remux / 转码决策；把 profile 转换成 `ClientCapabilities` | `libs/media/decision` |
| `tests/Jellyfin.MediaEncoding.Keyframes.Tests` | 关键帧提取样本 | `libs/media/keyframes` |
| `tests/Jellyfin.MediaEncoding.Hls.Tests`、`tests/Jellyfin.Api.Tests/Controllers/DynamicHlsControllerTests.cs` | 基于关键帧的动态 HLS 播放列表生成；分片长度、杜比视界编解码器标签、转码重启 | `libs/streaming` |
| `tests/Jellyfin.Drawing.Skia.Tests` | 缩放尺寸计算、锐化、SVG 安全校验 | `libs/imaging` |
| `tests/Jellyfin.Server.Implementations.Tests/Trickplay` | Trickplay 生成参数 | `libs/imaging` / `libs/media` |
| `tests/Jellyfin.Controller.Tests/Entities/BaseItemTests.cs` | 排序名（`GetSortName`、`ModifySortChunks`） | `libs/store` |
| `tests/Jellyfin.Extensions.Tests/StringExtensionsTests.cs` | 搜索键使用的去变音符号 | `libs/store` |
| `tests/Jellyfin.Extensions.Tests`、`Jellyfin.Common.Tests`（其他测试类） | 字符串、路径等通用工具函数 | 按需并入对应库 |
| `Jellyfin.Api.Tests`、`Jellyfin.Server.Integration.Tests` | HTTP API 行为 | 暂不移植（与后续 shim 一起评估） |

---

## 3. 测试媒体

`tools/fixtures` 用 `ffmpeg -f lavfi`（`testsrc2`、`sine`）确定性地生成测试媒体：一组短片段，覆盖多种容器与编解码器（H.264、HEVC Main10、MPEG-2、VP9、AV1、AAC、AC-3、MP2、Opus、FLAC）、HDR10 标记与元数据、隔行扫描、带语言与 disposition 的多音轨和多字幕轨、章节、音乐标签，以及非整数帧率与时长。

* `pnpm nx run fixtures:media` 把样本生成到仓库根目录的 `.fixtures/`（已加入 git 忽略；可用 `MAVIO_FIXTURES` 覆盖）。不提交二进制媒体文件。
* 增量生成：`.fixtures/manifest.json` 为每个样本记录一个 key（ffmpeg 版本、参数与辅助文件的哈希），只重新生成缺失或有变化的样本。
* 本机 ffmpeg 缺少所需编码器的样本会被跳过，并给出警告。
* 测试通过 `github.com/mavioai/mavio/tools/fixtures` 的 `fixtures.Require(t, "<name>")` 获取样本路径；样本未生成时自动跳过测试。
* 在 `tools/fixtures` 下运行 `go run ./cmd/fixtures -list` 可列出全部样本。


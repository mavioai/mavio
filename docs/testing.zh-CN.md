# Mavio 测试策略

> [English](testing.md) | 简体中文

> 相关文档：[架构](architecture.zh-CN.md) · [开发指南](development.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

---

## 1. 测试分层

| 层次 | 范围 | 位置 | 运行时机 |
| :--- | :--- | :--- | :--- |
| 单元测试 | 纯逻辑；不访问网络、不依赖真实 ffmpeg 或硬件。表驱动，很多用例移植自 Jellyfin | 代码旁的 `*_test.go`，数据放在 `testdata/` | 每次 CI，全部平台 |
| 一致性测试 | 同一套仓储测试分别跑在 SQLite（内存 / 临时文件）与 PostgreSQL（testcontainers-go）上，两者都必须通过 | `libs/store` | 每次 CI（Linux） |
| 集成测试 | 真实 ffmpeg / ffprobe、生成的测试媒体、真实硬件编码器；条件不满足时 `t.Skip` 并写明原因 | 代码旁，依赖前置条件 | 具备条件的 CI；厂商路径在硬件 runner 上运行 |
| 冒烟测试 | 在服务端内把已完成的库串起来（见[路线图 §1](roadmap.zh-CN.md#1-推进原则)） | `apps/server/internal/smoke` | 每次 CI |

涉及超时、定时器、空闲回收的并发逻辑，用 `testing/synctest` 写成确定性测试。

---

## 2. 从 Jellyfin 移植测试用例

### 2.1 方法
1. `tools/testport` 从 Jellyfin 的 C# 测试中提取 `[InlineData]`、`[MemberData]`、`TheoryData` 等参数化用例，输出为 JSON / YAML，放到对应库的 `testdata/` 目录。这是一次性迁移工具，迁移后人工复核。
2. 非参数化的测试（逻辑断言型）人工翻译成 Go 表驱动测试。
3. 测试资产（NFO 样例、ffprobe JSON、字幕文件、关键帧数据）复制到对应库的 `testdata/` 下，按用途命名（如 `testdata/nfo/`、`testdata/probe/`）。每个用例文件或资产都在元数据中记录来源（Jellyfin 仓库中的文件路径与测试名）；仓库中不出现以 Jellyfin 命名的目录。
4. Jellyfin 中依赖 .NET 特有行为、或与 Mavio 设计取舍不同的用例，在 testdata 中标记为 `skip` 并写明原因，**不允许静默丢弃**。
5. 移植完成后不再依赖 Jellyfin 运行时；后续新增用例直接写在 Mavio 中。

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
| `tests/Jellyfin.MediaEncoding.Hls.Tests` | 基于关键帧的动态 HLS 播放列表生成 | `libs/streaming` |
| `tests/Jellyfin.Drawing.Skia.Tests` | 缩放尺寸计算、锐化、SVG 安全校验 | `libs/imaging` |
| `tests/Jellyfin.Server.Implementations.Tests/Trickplay` | Trickplay 生成参数 | `libs/imaging` / `libs/media` |
| `tests/Jellyfin.Extensions.Tests`、`Jellyfin.Common.Tests` | 字符串、路径等通用工具函数 | 按需并入对应库 |
| `Jellyfin.Api.Tests`、`Jellyfin.Server.Integration.Tests` | HTTP API 行为 | 暂不移植（与后续 shim 一起评估） |

---

## 3. 测试媒体

`tools/fixtures` 用 `ffmpeg -f lavfi`（`testsrc2`、`sine` 等）确定性地生成测试媒体：多种容器与编解码器、HDR10 / 杜比视界元数据、多音轨与多字幕轨、隔行扫描片源、非常规时长。生成的文件放在 `.fixtures/`（已加入 git 忽略），不提交二进制媒体文件。依赖这些文件的测试在文件缺失时跳过，并给出明确的提示信息。


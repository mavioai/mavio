# Jellyfin API 兼容垫片评估

> [English](jellyfin-compat.md) | 简体中文

> 相关文档：[系统架构](architecture.zh-CN.md) · [领域模型](domain.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

> Mavio 不兼容 Jellyfin 客户端生态（[架构 §1.2](architecture.zh-CN.md)）。本文档评估一个让现有 Jellyfin 客户端使用 Mavio 服务端的兼容垫片（shim）：需要做什么、与 Mavio 的设计在哪里冲突，以及若采用时的形态。**状态：未采用。** 是否采用在 P13 决定（[路线图](roadmap.zh-CN.md)），届时 Mavio 自有客户端已经起步。文中数据取自 Jellyfin 12.2（2026 年 10 月）。

---

## 1. 结论

垫片可行：Mavio 的标识符、客户端能力，以及播放、SyncPlay 与浏览规则都移植自 Jellyfin，因此大部分工作是转换数据形状，而不是重新实现行为。但现在不值得做：它要维护一个随 Jellyfin 发版演进的非强类型 REST 外壳，有违契约优先的原则，并且需要在 Mavio 授权媒体请求的方式上开一个例外（§3.2）。如果想在自有客户端之前用上真实的 Jellyfin 客户端，应从 §5 的小范围实验入手。

## 2. 接口规模

* Jellyfin API 有 60 个 Controller、397 个端点（GET 215、POST 144、DELETE 38）。
* 大多数端点返回的条目结构 `BaseItemDto` 约有 155 个属性。
* 单个客户端只用其中一部分。主线（登录、浏览、播放、上报进度、接收事件）需要几十个端点。

## 3. 与 Mavio 的对应

### 3.1 可以直接对应的部分
| Jellyfin | Mavio | 工作 |
| :--- | :--- | :--- |
| 条目、用户、媒体库的 GUID | `core.ID`，16 字节 | 输出为不带连字符的格式 |
| `DeviceProfile`（直放、转码、容器、编解码器、字幕 profile） | `playback.v1.ClientCapabilities`，五类 profile 相同 | 字段转换；决策规则就是 Jellyfin 的 |
| `PlaybackInfo`、进度与停止上报 | `PlaybackService` | 转换 |
| 最近添加、下一集、合集、播放列表 | `ItemService`、`CollectionService`、`PlaylistService` | 转换 |
| 图片参数、拼贴图、trickplay、章节图、媒体片段、歌词 | 语义相同（架构 §11） | 路由转换 |
| SyncPlay、Quick Connect | 遵循 Jellyfin 的语义 | 消息转换 |

### 3.2 难点
1. **媒体授权（主要冲突）。** Jellyfin 客户端以 `Authorization: MediaBrowser Token=…`、`X-Emby-Token`、`X-MediaBrowser-Token` 或 `api_key` 查询参数发送令牌，并自行拼接媒体 URL，例如 `/Videos/{id}/stream?static=true&api_key=…`。Mavio 的媒体 URL 有意不接受令牌：只有经认证的 `PlaybackService` 发放的不可猜 playback ID 才能访问，令牌因此不会进入 URL、日志或缓存（架构 §11）。垫片必须在它自己的媒体路由上接受 URL 中的令牌，并为每次请求在内部开启一次 playback。这个例外只能留在垫片内。
2. **事件。** Jellyfin 通过 WebSocket 推送会话、媒体库、用户数据、远程控制与 SyncPlay 消息（34 种消息类型）；Mavio 使用 Connect 服务端流（`EventService`）。垫片需要双向桥接。
3. **发现与版本。** 客户端通过 UDP 发送 `who is JellyfinServer?`，Mavio 的发现服务也要应答。`/System/Info/Public` 必须报告一个 Jellyfin 版本号，客户端据此启用功能，因此垫片要跟随 Jellyfin 发版。
4. **缺失的功能。** 直播电视、频道、插件商店、品牌定制与启动向导没有对应实现。垫片返回空结果或桩实现，部分客户端可能处理不好。
5. **Jellyfin Web。** 它要求由服务端托管在 `/web`。垫片面向原生客户端（Swiftfin、Findroid、Jellyfin Android TV、Infuse、Finamp、Streamyfin），不打包它。

## 4. 收益与代价
* **收益**：在 Mavio 自有客户端（P13）之前就有覆盖各平台的成熟客户端，并能借助它们在真实设备上测试播放。
* **代价**：一个要随 Jellyfin 演进维护的非强类型 REST 外壳；媒体的第二条授权路径；架构 §1.2 中的定位也要随之改变。

## 5. 若采用时的形态
* `apps/server` 中的一个可选包（例如 `internal/jellyfin`），默认关闭，由一个开关启用。它在进程内调用 `internal/rpc` 的服务实现，不改动领域模型与契约。
* 先只针对一个目标客户端（Findroid 或 Infuse），覆盖 §2 的主线；其他客户端与端点按需逐步加入。
* 响应以 Jellyfin 发布的 OpenAPI 文档校验。

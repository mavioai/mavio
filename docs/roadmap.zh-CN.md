# Mavio 路线图

> [English](roadmap.md) | 简体中文

> 相关文档：[架构](architecture.zh-CN.md) · [开发指南](development.zh-CN.md) · [测试策略](testing.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

---

## 1. 推进原则

* **自底向上**：先实现零业务依赖的底层库，再逐层向上组装；每一层只依赖已完成的下层。
* **完成标准**：一个阶段的库在完成时须同时满足：
  * **正确性门禁**：移植的测试用例全部通过（或有明确标记、写明原因的 skip），见 [测试策略 §2](testing.zh-CN.md#2-从-jellyfin-移植测试用例)；
  * **性能门禁**：基准测试满足本文档列出的预算。
* **性能预算**：表中数值为初始预算，在 P0 建成基准设施、拿到参考硬件的实测数据后校准。参考硬件为 Intel N100（amd64）与 RK3588（arm64）。
* **集成冒烟**：从 P1 起，每个阶段结束时在 `apps/server/internal/smoke` 增加一个把已完成的库串起来的集成测试。它不是 MVP，只用来尽早暴露接口漂移，降低自底向上方式"后期集成才发现问题"的风险。

---

## 2. 阶段总览

```mermaid
flowchart TD
    P0["P0 工程地基<br/>Nx / go.work / buf / mise / CI / 基准设施 / 测试媒体生成 / testport"]
    P1["P1 契约、存储与插件运行时<br/>libs/proto, libs/core, libs/store, libs/plugin"]
    P2["P2 纯计算库<br/>libs/naming, libs/subtitle, libs/metadata, libs/imaging"]
    P3["P3 媒体管线<br/>libs/media: probe, keyframes, hwaccel, decision, planner, supervisor"]
    P4["P4 扫描与插件落地<br/>libs/library, plugins/scraper-tmdb"]
    P5["P5 流媒体与 API<br/>libs/streaming, Connect 服务"]
    P6["P6 服务端装配与分发<br/>apps/server, 单二进制 + jellyfin-ffmpeg, 容器镜像"]
    P7["后期<br/>libs/client, libs/ui, apps/web, apps/desktop, apps/mobile, Jellyfin shim 评估"]

    P0 --> P1
    P1 --> P2
    P1 --> P3
    P2 --> P4
    P3 --> P5
    P4 --> P5
    P5 --> P6
    P6 --> P7
```

| 阶段 | 主题 | 状态 |
| :--- | :--- | :--- |
| P0 | 工程地基 | 🚧 进行中 |
| P1 | 契约、存储与插件运行时 | 未开始 |
| P2 | 纯计算库 | 未开始 |
| P3 | 媒体管线 | 未开始 |
| P4 | 扫描与插件落地 | 未开始 |
| P5 | 流媒体与 API | 未开始 |
| P6 | 服务端装配与分发 | 未开始 |
| P7 | 客户端与生态 | 未开始 |

---

## 3. 阶段详情

### P0 工程地基
**范围**：仓库骨架、mise、Nx、go.work、buf、golangci-lint + depguard；CI 矩阵（linux amd64/arm64、macOS、Windows）；benchstat 与持续基准；`tools/fixtures`、`tools/testport`。

**正确性门禁**：`nx affected` 可用；`tidy-check`（`GOWORK=off`）通过。

**性能门禁**：基准结果可在 PR 中对比展示。

**进度**：
- [x] Go 模块骨架与 `go.work`；Nx 及本地插件 `tools/nx-go`（自动推断项目与依赖图）
- [x] mise 锁定工具链版本
- [x] buf 与首个契约 `mavio.system.v1.SystemService`；`apps/server` 提供 Connect 健康检查接口
- [x] golangci-lint 与 depguard 依赖方向规则
- [x] CI 工作流：受影响项目检查、生成代码一致性检查、`buf breaking`、多平台测试矩阵（尚未在 GitHub 上实际运行）
- [ ] `tools/fixtures`：测试媒体生成
- [ ] `tools/testport`：Jellyfin 测试用例提取
- [ ] `tools/bench`：基准编排、benchstat 对比与持续基准

### P1 契约、存储与插件运行时
**范围**：`proto` 契约初版；`core` 领域模型；`store`（ent + Atlas + sqlc，双方言）与任务队列；`plugin` 两种运行时与握手。

**正确性门禁**：仓储一致性测试在 SQLite 与 PostgreSQL 上均通过；插件崩溃不影响宿主，并能自动重启。

**性能门禁**：
- 10 万条目库上浏览 / 过滤查询 p99：SQLite < 20ms，PostgreSQL < 30ms
- 批量 upsert ≥ 5k 条/秒
- WASM 插件空调用 < 50µs，UDS 空调用 < 300µs

### P2 纯计算库
**范围**：`naming`、`subtitle`、`metadata`（NFO）、`imaging`。

**正确性门禁**：对应的移植用例全部通过。

**性能门禁**：
- `naming` 单次解析 < 10µs
- 1080p JPEG → 400px WebP，单核 < 40ms（amd64）
- 图像处理内存峰值有界

### P3 媒体管线
**范围**：`probe`、`keyframes`、`hwaccel`、`decision`、`planner`、`supervisor`。

**正确性门禁**：StreamBuilder 与 EncodingHelper 的移植用例全部通过；参考硬件上各厂商路径的真实转码冒烟测试通过。

**性能门禁**：
- 单次决策加规划 < 1ms
- ffmpeg 启动到首个分片产出的额外开销 < 50ms
- 会话结束后 ffmpeg 进程零泄漏

### P4 扫描与插件落地
**范围**：`library` 扫描器（全量对账，见[架构 §9](architecture.zh-CN.md#9-媒体库扫描与变更检测libslibrary)）、解析器链、任务调度；`plugins/scraper-tmdb`（WASM）。

**正确性门禁**：库解析器移植用例通过；扫描结果在两种数据库上一致；新增 / 删除 / 重命名 / 内容修改 / 挂载点暂时不可用等场景下，对账都收敛到正确状态。

**性能门禁**：
- 1 万文件冷扫描（不含远程刮削）< 60s
- 无变更对账 < 2s（本地 SSD）
- 扫描期间 RSS < 150MB

### P5 流媒体与 API
**范围**：`streaming`（CMAF HLS、按需分片、seek、分片缓存）；Connect 服务（库、播放、用户、系统）。

**正确性门禁**：HLS 移植用例通过；hls.js / AVPlayer / Media3 实机播放验证。

**性能门禁**：
- remux 首分片 TTFB < 300ms
- seek 后首分片 < 1s

### P6 服务端装配与分发
**范围**：`apps/server` 装配；`CGO_ENABLED=0` 交叉编译；容器镜像内置 jellyfin-ffmpeg。

**正确性门禁**：端到端冒烟测试：扫描 → 刮削 → 播放决策 → HLS 播放。

**性能门禁**：
- 1 万条目库下空闲 RSS < 80MB
- 冷启动 < 500ms

### P7 客户端与生态
**范围**：`libs/client`、`libs/ui`；`apps/web`、`apps/desktop`、`apps/mobile`；Jellyfin API 兼容垫片（shim）评估。具体门禁在 P6 完成后制定。

---

## 4. 风险与待决事项

| 事项 | 说明 | 处理时机 |
| :--- | :--- | :--- |
| wazero 解释器平台 | armv7 / riscv64 上 SQLite 与 WASM 编解码器性能退化 | P1 实测；必要时为这些平台启用纯 Go 兜底 build tag |
| sqlc 双方言维护成本 | 热点查询需要两份 SQL | 坚持 ≤ 30 条预算；靠一致性测试兜底 |
| 纯 Go 图像缩放性能 | 若达不到 P2 预算 | 大图改走 ffmpeg 缩放（[架构 §6.2](architecture.zh-CN.md#62-性能策略)） |
| SVG 栅格化 | 纯 Go 方案不完善 | P2 评估 resvg 的 WASM 版本，或只做原样下发 |
| 硬件测试覆盖 | 需要 Intel、NVIDIA、AMD、Apple、Rockchip 实机 | P3 前搭建自托管 CI runner 或硬件测试池 |
| Go 模块拆分过细 | 依赖升级与 tidy 的摩擦 | 持续观察，必要时合并模块 |

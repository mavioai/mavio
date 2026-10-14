# 组件

> [English](components.md) | 简体中文

各客户端的核心组件（[客户端 §6.4](../../docs/clients.zh-CN.md#64-组件)）。每个客户端用 `gen/` 中的 tokens 原生实现它们；本文是各实现共享的规范。`代码` 形式的名称是 tokens：主题的 `color.*`、设备形态的 `layout.*`，以及 `typography.*`、`spacing.*`、`radius.*`、`duration.*` 与 `easing.*`。

## 状态

每个可交互组件都有以下状态；电视为每张卡片增加获得焦点状态。

| 状态 | 外观 |
| :--- | :--- |
| 默认 | 如下文所述 |
| 获得焦点（电视、键盘） | 以 `easing.standard` 在 `duration.fast` 内放大到 `layout.focusScale`，带 `elevation.level2`；其他平台的键盘焦点显示 2 pt 的 `color.focus` 边框 |
| 按下 | 内容不透明度降至 80% |
| 加载 | 与组件最终尺寸相同的 `color.surfaceRaised` 占位，内容到达时不发生移动 |
| 禁用 | 内容使用 `color.textDisabled` |

## 海报卡片

电影、剧集、季或合集的 2:3 卡片。

* **结构**：圆角 `radius.md` 的图片；其下为标题（`typography.headline`，一行）与副标题（`typography.footnote`，`color.textSecondary`，一行：年份或条目数）。
* **叠加层**：沿图片底边的观看进度条（3 pt，`color.progress` 叠在 `color.scrim` 上）；右上角的已看标记（图标 `played`，Fill）；剧集与季在该位置改为显示未看集数的 `color.accent` 徽标。
* **尺寸**：宽度 `layout.poster`；网格中宽度为可用空间除以能放下宽度不小于 `layout.poster` 的列数。
* **无图片时**：在 `color.surfaceRaised` 上居中显示 `typography.title3` 的标题。

## 缩略图卡片

剧集单集、视频或章节的 16:9 卡片。

* **结构**：圆角 `radius.md` 的图片；其下为标题（`typography.headline`），单集还有剧集与集数（`typography.footnote`，`color.textSecondary`，字符串 `item.episodeNumber`）。
* **叠加层**：同海报卡片，另在左下角显示未看完条目的剩余时间（`item.minutesLeft`）。
* **尺寸**：宽度 `layout.thumbnail`。

## 横向列表

带标题的一行卡片（继续观看、下一集、某媒体库最新内容）。

* **结构**：标题（`typography.title3`），桌面与触屏上带"更多"操作；其下为卡片，间距 `layout.gutter`，首张与 `layout.margin` 对齐。
* **行为**：可用触摸、滚轮与触控板横向滚动；电视上焦点逐张移动，列表滚动以保持焦点卡片可见；桌面上悬停时两端出现箭头。
* **为空时**：没有条目的横向列表隐藏。

## 详情页头部

条目详情页的顶部。

* **结构**：铺满宽度、渐隐到 `color.background` 的背景图；其下部叠加标志或标题（`typography.display`）、一行信息（`typography.callout`，`color.textSecondary`：年份、评分、时长、分辨率）、主操作（播放或继续播放，`color.button` 上的 `color.onButton`）与次要操作（收藏、标为已看、更多）。
* **布局**：手机上海报叠在文字上方；平板、桌面与电视把文字放在背景图左侧。
* **简介**：`typography.body`，手机上显示三行并带"更多"操作。

## 剧集行

季列表中的一集。

* **结构**：左侧为缩略图卡片的图片（手机上宽度为 `layout.thumbnail` / 2）；集号与标题（`typography.headline`）、时长与播出日期（`typography.footnote`，`color.textSecondary`），以及两行简介（`typography.callout`）。
* **叠加层**：同缩略图卡片。

## 人物卡片

演员或职员。

* **结构**：圆形照片（`radius.full`）、姓名（`typography.callout`）与角色（`typography.footnote`，`color.textSecondary`），各至多两行。
* **无照片时**：在 `color.surfaceRaised` 上显示姓名首字母。

## 音轨选择器

播放器与详情页中的音轨与字幕选择器。

* **结构**：音轨列表，每项显示语言名称与详细信息（`typography.footnote`，`color.textSecondary`：编码、声道、强制字幕、听障字幕）；选中项带勾选标记。字幕列表以 `player.subtitlesOff` 开头。
* **呈现**：Apple 平台与桌面上是菜单，Android 手机上是底部面板，电视上是侧边面板。

## 状态视图

页面或列表的空、错误、离线与加载状态。

* **结构**：图标（48 pt，`color.textTertiary`：`error`、`offline`）、标题（`typography.title3`：`state.empty`、`state.error`、`state.offline`）、可选的说明（`typography.body`，`color.textSecondary`）与可选的操作（`action.retry`）。
* **加载**：显示内容的占位（见"状态"），而不是加载指示器；首屏除外，显示应用标志。
* **离线**：已下载的条目仍列在该视图下方。

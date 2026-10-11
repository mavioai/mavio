# Mavio 客户端

> [English](clients.md) | 简体中文

> 相关文档：[系统架构](architecture.zh-CN.md) · [领域模型](domain.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [开发指南](development.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

> Mavio 的客户端包括内嵌于服务端二进制的 Web UI，以及面向每种观看方式的原生应用：iPhone 与 iPad、Apple TV、Mac、Android 手机与平板、Android TV，以及 Windows 与 Linux 桌面。决策由服务端做出；客户端声明自己能播放什么、负责播放，并离线保留所需内容。共享的 Rust 核心让所有原生应用的连接、事件、离线与播放上报逻辑只有一份实现。客户端在[路线图](roadmap.zh-CN.md)的 P21 阶段构建。

---

## 1. 概述

每个应用只为一种交互模型而构建——触控、电视遥控器、桌面的指针与键盘——而不是在多种模型之间拉伸。不取决于交互模型的部分在应用之下共享。

| 应用 | 技术 | 设备 |
| :--- | :--- | :--- |
| `apps/web` | React + Vite + TanStack Router / Query + Tailwind CSS，以 `//go:embed` 内嵌进服务端 | 浏览器 |
| `apps/ios` | Swift、SwiftUI（SwiftUI 不足处用 UIKit） | iPhone 与 iPad，一个通用应用 |
| `apps/tvos` | Swift、SwiftUI 与 UIKit | Apple TV |
| `apps/macos` | Swift、AppKit（合适处用 SwiftUI） | Mac |
| `apps/android` | Kotlin、Jetpack Compose | Android 手机、平板与折叠屏 |
| `apps/android-tv` | Kotlin、Compose for TV | Android TV 与 Google TV |
| `apps/windows-linux` | Go、[MyGo](https://github.com/egoist/mygo) 原生 UI、libmpv | Windows 与 Linux |

| 库 | 内容 | 使用者 |
| :--- | :--- | :--- |
| `libs/client-ts` | TypeScript：Connect 客户端（`connect-es`）与 TanStack Query hooks | `apps/web` |
| `libs/client-rust` | Rust：服务器、登录、事件、离线存储、播放上报、下载 | 所有原生应用：Swift 与 Kotlin 经 UniFFI，Go 经 C ABI |
| `libs/apple` | Swift 包：核心的 Swift API 与应用服务、播放器、共享的 SwiftUI 组件 | `apps/ios`、`apps/tvos`、`apps/macos` |
| `libs/android` | Kotlin 模块：核心的 Kotlin API 与应用服务、播放器、共享的 Compose 组件 | `apps/android`、`apps/android-tv` |
| `libs/design` | 设计 tokens（DTCG JSON）、图标、组件规范与字符串，为每个客户端编译 | 所有客户端 |

---

## 2. 架构

```mermaid
flowchart LR
    Server["Mavio 服务端（Go）"]
    Web["apps/web<br/>React"]
    ClientTS["libs/client-ts<br/>connect-es + Query"]

    subgraph Core["libs/client-rust（Rust）"]
        Conn["服务器、登录<br/>Connect 客户端"]
        Events["事件流"]
        Store["离线存储（SQLite）"]
        Report["播放上报<br/>下载"]
    end

    AppleLib["libs/apple<br/>AVPlayer + libmpv"]
    AndroidLib["libs/android<br/>Media3 + libmpv"]
    iOS["apps/ios"]
    tvOS["apps/tvos"]
    macOS["apps/macos"]
    Android["apps/android"]
    AndroidTV["apps/android-tv"]
    WinLin["apps/windows-linux<br/>MyGo · libmpv"]

    Server -->|"内嵌"| Web
    Web --> ClientTS
    ClientTS -->|"Connect"| Server
    Core -->|"Connect"| Server
    AppleLib -->|"UniFFI（Swift）"| Core
    AndroidLib -->|"UniFFI（Kotlin）"| Core
    WinLin -->|"C ABI（purego）"| Core
    iOS & tvOS & macOS --> AppleLib
    Android & AndroidTV --> AndroidLib
    Web & AppleLib & AndroidLib & WinLin -->|"HTTP：/media、/images"| Server
```

---

## 3. 服务端与客户端

凡取决于媒体库、用户或其他设备的决策都归服务端；取决于设备本身的归客户端。

| 关注点 | 归属 | 方式 |
| :--- | :--- | :--- |
| 播放决策 | 服务端 | `PlaybackService` 依据客户端的 `ClientCapabilities`，以 `libs/media/decision` 决定直接播放、重封装或转码，并结合用户的流偏好、码率上限、转码权限与播放数限制（[架构 §11](architecture.zh-CN.md#11-api-与认证appsserver)） |
| 能力 | 客户端 | 用平台自身的 API 探测：AVFoundation 与 VideoToolbox、`MediaCodecList` 与 `Display.HdrCapabilities`、浏览器的 `MediaCapabilities` 与 `MediaSource.isTypeSupported`，以及随附 libmpv 的解码器。以两个引擎播放的客户端，声明两者任一能播放的内容，使服务端只在两者都不能播放时才处理媒体（§4.2） |
| 进度 | 客户端上报，服务端记录 | 通过 `PlaybackService` 在播放时每 10 秒上报一次，并在暂停、跳转与停止时上报；服务端在最后一次上报五分钟后结束该播放 |
| 实时更新 | 服务端推送 | 每台设备一条 `EventService` 流：条目与媒体库变更、用户数据、会话、远程控制命令与 SyncPlay。客户端以退避重连，不轮询 |
| 媒体与图片 | 普通 HTTP | `/media/{playback}/…` 与 `/images/…`；ID 即凭据，播放器无需附加请求头 |
| 登录 | 服务端 | Bearer 令牌；电视应用通过 Quick Connect 登录 |
| 首页 | 服务端生成，客户端合并 | 每台服务器用 `HomeService` 为用户生成其推荐行（[架构 §11](architecture.zh-CN.md#11-api-与认证appsserver)）；客户端合并它所登录的每台服务器的推荐行（§5.4） |
| 发现服务器 | 客户端 | 局域网内 UDP 7359 端口的发现，或手动输入地址 |
| 资源获取、发现与请求 | 服务端 | 服务端自己的服务；客户端不调用其他任何服务 |

---

## 4. 各平台

### 4.1 Web（`apps/web`）
* **定位**：全部功能——浏览、播放、请求、管理与首次运行设置——无需安装。
* **提供方式**：Vite 构建到 `apps/server/internal/webui/dist`，服务端以 `//go:embed` 内嵌并在 `/` 提供，SPA 的路由回退到 `index.html`。仓库中该目录只有一个 `.gitkeep`；未带 Web UI 构建的服务端会提供一个说明此情况的页面。Nx 让服务端的 `build` 依赖 Web 的 `build`。API 参考保留在 `/api`。
* **服务器**：Web UI 登录到提供它的服务器，并像各应用一样添加其他服务器（§5.4），以跨域方式调用它们；服务器的所有路由允许任何来源，它们接受 Bearer 令牌与 ID 凭据，从不使用 Cookie。服务器列表（令牌除外）保存在提供它的服务器上用户的显示偏好中，从而跟随用户跨浏览器。
* **API**：`libs/client-ts`，基于 `connect-es` v2 与 `protobuf-es` v2 以及从 `libs/proto` 生成的代码，用 TanStack Query 做缓存与失效，由事件流驱动。
* **播放**：支持 Media Source Extensions 的浏览器用 hls.js，Safari 用原生 HLS 播放器；能力来自 `MediaCapabilities.decodingInfo`。ASS 字幕借助 JASSUB（WebAssembly 版 libass）保留样式。对浏览器不转码就无法播放的内容，播放器提供在外部播放器（mpv、VLC、IINA）中打开播放 URL。

### 4.2 Apple（`apps/ios`、`apps/tvos`、`apps/macos`、`libs/apple`）
* **每种交互模型一个应用。** iPhone 与 iPad 按 App Store 的惯例是一个通用应用，iPad 上有自己的布局：侧边栏与多栏、多窗口、台前调度、指针与硬件键盘。Apple TV 是独立应用，围绕焦点引擎与十英尺距离的遥控器构建，并提供 Top Shelf。Mac 是以 AppKit 编写的独立应用，AppKit 让播放器能完全掌控窗口、菜单栏、全屏、触控板以及同时打开多个播放器。
* **`libs/apple`**，三个应用共享的 Swift 包：
  * `MavioKit`：核心的 Swift API 及其上的应用服务——账户、能力探测、以后台 `URLSession` 下载、正在播放（`MPNowPlayingInfoCenter`、`MPRemoteCommandCenter`）；
  * `MavioPlayer`：一个播放器接口、两个引擎，以及画中画；
  * `MavioUI`：各应用以各自方式组合的 SwiftUI 组件（图片、海报、行），以及设计 tokens。
* **播放引擎**，优先顺序为：AVPlayer，其次 libmpv，最后由服务端处理。
  * **能力**：应用在 `direct_play` 中声明 AVPlayer 能播放的配置与 libmpv 能播放的配置。因此只要任一引擎能播放，服务端就直接播放该源；两者都不能播放时才重封装或转码。
  * **选择**：直接播放的源，AVPlayer 能原样播放时交给 AVPlayer，否则交给 libmpv；服务端重封装或转码的结果是 HLS，交给 AVPlayer。
  * **杜比视界**：libmpv 的配置排除杜比视界视频（libmpv 只能将其显示为 HDR10），因此 Matroska 中的此类源由服务端拷贝视频重封装为 HLS，由 AVPlayer 以杜比视界播放。
  * **AirPlay**：使用 AirPlay 时，应用只声明 AVPlayer 的配置，因为 libmpv 不支持 AirPlay。
  * **AVPlayer** 是首选引擎，播放 AVFoundation 能播放的内容——H.264、HEVC、硬件可解码时的 AV1、杜比视界与 HDR10 的 MP4 与 MOV，AAC、AC-3 及带 Atmos 的 E-AC-3——以及服务端产生的所有 HLS 流。它带来系统画中画、AirPlay、iOS 与 tvOS 上的 `AVPlayerViewController`，以及 tvOS 上的帧率与动态范围匹配（`AVDisplayCriteria`）。
  * **libmpv** 是第二引擎，直接播放 AVFoundation 不能播放的内容，如带样式 ASS 字幕的 Matroska、DTS 与 TrueHD，免去服务端转码。其渲染方式与 IINA 相同：mpv 的 render API（后端为 OpenGL 与软件渲染）渲染到 `CAOpenGLLayer`；HDR 时把该层切换为 EDR（`wantsExtendedDynamicRangeContent`、`RGBA16Float` 内容、PQ 色彩空间），mpv 设置 `target-trc=pq`，`target-peak` 取自屏幕；iOS 与 tvOS 上使用 OpenGL ES 后端。
* **画中画**：只用公开且被 App Store 允许的 `AVPictureInPictureController`。AVPlayer 直接支持；libmpv 播放通过其 `AVSampleBufferDisplayLayer` 内容源接入，帧渲染到以 IOSurface 为后备的像素缓冲区。

### 4.3 Android（`apps/android`、`apps/android-tv`、`libs/android`）
* **每种交互模型一个应用。** 手机、平板与折叠屏共用一个以 Compose 编写、采用自适应布局的应用；Android TV 与 Google TV 有自己的 Compose for TV 应用，围绕方向键焦点构建，并提供主屏上的 Watch Next 与频道。两者作为同一个 Play 商店条目发布，TV 应用面向 TV 设备类型。
* **`libs/android`**，两个应用以复合构建（composite build）引入的共享 Gradle 模块：
  * `core`：核心的 Kotlin API 及其上的应用服务——账户、能力探测、以 WorkManager 下载；
  * `player`：一个播放器接口、两个引擎，其上是提供后台播放与系统控制的 `MediaSessionService`，以及画中画（Android 的画中画可用于任何 `SurfaceView`）；
  * `ui`：共享的 Compose 组件与设计 tokens。
* **播放引擎**，与 Apple 平台相同的优先顺序：Media3，其次 libmpv，最后由服务端处理。
  * **能力**：应用在 `direct_play` 中声明 Media3 在该设备上能播放的配置与 libmpv 能播放的配置，使服务端只在两者都不能播放时才重封装或转码。
  * **选择**：直接播放的源，Media3 能原样播放时交给 Media3，否则交给 libmpv；服务端重封装或转码的结果交给 Media3。
  * **杜比视界与 HDR**：libmpv 的配置排除杜比视界视频，在有 HDR 输出的设备上还排除 HDR 视频——Media3 能把它们输出到显示器，libmpv 只能做色调映射。
  * **Media3** ExoPlayer 是首选引擎，播放 Matroska 与 MP4 中设备能解码的格式，借助其 FFmpeg 解码扩展播放平台缺少的音频格式（DTS、TrueHD 等），并支持 PGS 字幕与不带样式的 SSA。它渲染到 `SurfaceView` 以支持 HDR 并降低功耗，电视支持时使用隧道播放，并以 `Surface.setFrameRate` 匹配帧率。
  * **libmpv** 是第二引擎，按 mpv-android 的方式构建，直接播放 Media3 不能播放的内容，如带样式的 ASS 字幕与没有硬件解码器的编码；以 OpenGL ES 渲染到它自己的 `SurfaceView`，能用 MediaCodec 时以其解码。它由一个 Media3 `SimpleBasePlayer` 适配，使媒体会话与控制器同等对待两个引擎。

### 4.4 Windows 与 Linux（`apps/windows-linux`）
* **结构**：基于 MyGo 的 Go 应用，所有窗口都是 MyGo 在 GPU 上绘制的原生 UI（Windows 上为 Direct3D 11，Linux 上为 OpenGL）；不使用 Web 视图。它与服务端一样以 `CGO_ENABLED=0` 构建，随附两个共享库：libmpv 与核心。
* **核心**：通过 C ABI 使用 `libs/client-rust`，与 MyGo 加载原生 API 的方式相同，在运行时加载——Linux 上用 purego，Windows 上用 `syscall` 包。请求、结果与事件以核心 API 的 protobuf 消息跨越该接口，并为 Go 生成代码。
* **播放器**：libmpv，以同样方式加载，通过其 client API 驱动（命令、属性，以及由一个 goroutine 读取的事件）。视频由 mpv 自己绘制，位于它自己的原生表面中，该表面放在 MyGo 绘制 UI 的表面之下。MyGo 原生 UI 中的视频元素负责放置该表面：在视频显示处，它在窗口绘制的内容上打出一个洞，使 MyGo 绘制的屏幕控制器（§6.5）、菜单与对话框显示在视频之上，而 MyGo 绘有内容之处的指针事件仍归 MyGo。
  * **Windows**：mpv 以 Direct3D 11 与 `gpu-next` 渲染到子窗口（`wid`），并以 `target-colorspace-hint` 输出 HDR10；MyGo 的帧位于窗口最顶层的 DirectComposition 目标中，处于其上方。
  * **Linux**：mpv 的 OpenGL render API 绘制到 MyGo 表面之下、位于其 `GtkOverlay` 中的 `GtkGLArea`，X11 与 Wayland 相同；HDR 色调映射为 SDR。
* **系统集成**：Windows 上的 System Media Transport Controls 与 Linux 上的 MPRIS，同时提供媒体键；播放期间由 MyGo 的电源 API 保持屏幕常亮。
* **打包与更新**：使用 MyGo 提供的能力——Windows 安装程序、Debian 包与 Linux 安装脚本、代码签名，以及带增量更新的签名自动更新。

---

## 5. 共享核心（`libs/client-rust`）

### 5.1 职责
* **服务器与登录**：同时登录多台服务器（§5.4），支持发现、以密码或 Quick Connect 登录；访问令牌由应用通过它实现的接口保存在平台的安全存储中（Keychain、Android Keystore、Windows 凭据管理器、Secret Service）。
* **Connect 客户端**：[connect-rust](https://github.com/connectrpc/connect-rust)，基于 HTTP/1.1 与 HTTP/2，消息与客户端从 `libs/proto` 生成到 `libs/proto/gen/rust`，由 crate `mavio-api` 包含。
* **事件流**：每台服务器一条 `EventService` 流，以退避重连，事件分发给应用。
* **离线存储**：SQLite（`rusqlite`，内置 SQLite），保存最近浏览的条目以便应用以缓存内容启动、已下载的条目，以及离线时产生的进度上报，待服务器可达后发送。
* **播放上报**：开始播放、周期与最终进度上报，以及离线上报的补发。
* **下载**：队列、其状态与已下载的条目；字节由应用借助平台的后台传输搬运。
* **页面状态**：§6.3 中每个页面的视图模型——它显示的数据、加载、空与错误状态，以及它提供的操作——由应用观察，应用只负责绘制；因此各页面在所有平台上行为一致。`libs/client-ts` 以 hooks 形式为 Web UI 提供相同的视图模型。

### 5.2 边界
* 不含 GUI、图形与解码：视图模型只含状态，不含布局；核心从不接触视频表面或解码器，也不探测能力；应用把自己的 `ClientCapabilities` 传入。
* 基于 Tokio 的异步 API；每个调用都可取消。
* 目标平台：`aarch64-apple-ios`、`aarch64-apple-ios-sim`、`aarch64-apple-tvos`、`aarch64-apple-darwin`、`x86_64-apple-darwin`、`aarch64-linux-android`、`armv7-linux-androideabi`（许多 Android TV 设备运行 32 位系统）、`x86_64-linux-android`、`x86_64-pc-windows-msvc`、`aarch64-pc-windows-msvc`、`x86_64-unknown-linux-gnu`、`aarch64-unknown-linux-gnu`。

### 5.3 绑定
* **Swift**：UniFFI 生成 Swift API，与静态库一起打包为 `MavioKit` 所依赖的 XCFramework。
* **Kotlin**：UniFFI 生成 Kotlin API，与共享库一起打包在 `libs/android` 的 `core` 模块中。
* **Go**：一个由少量函数组成的 C ABI，参数与返回值是字节缓冲区，其中是核心 API 的 protobuf 消息，定义在 `libs/client-rust/proto`；结果与事件通过同一个回调返回，因此 purego 在整个进程中只需创建一个回调。Go API 由这些消息生成。

### 5.4 多台服务器
客户端同时登录用户添加的每台服务器——用户自己的以及亲友的——每台都有各自的用户与访问令牌。服务器彼此不知道对方；只有客户端按用户的服务器顺序把它们组合起来。
* **跨服务器的部分**：首页、搜索与下载。媒体库、条目详情与播放器属于某一台服务器；登录多台服务器时，条目会显示其服务器名称。
* **首页**，按 Plex 的方式构建：合并每台服务器的推荐行（§6.3）：
  * **继续观看**是跨服务器的一行：正在观看的条目与下一集，按最近活动排序；
  * **多台服务器上的同一作品**按 TMDB、TVDB 或 IMDb ID 匹配，只出现一次：取自用户最近播放它的服务器，否则取用户顺序中的第一台；其详情页提供其他服务器上的版本；
  * **其他推荐行**按用户的服务器顺序逐台排列，每台服务器内按其置顶媒体库的顺序；
  * **热门**来自发现功能，只出现一次，取自第一台提供发现的服务器；任一服务器已有的作品标为可用；请求发往该服务器。
* **搜索**同时查询每台服务器，并按服务器列出结果。
* **渐进与离线**：每台服务器的推荐行到达即绘制，位置固定，因此慢的服务器不会拖住其他服务器；无法连接的服务器的推荐行取自离线存储，并标为离线。
* **播放状态**保留在各自的服务器上：在一台服务器上看过的作品，不会在另一台上标为已看。

---

## 6. 设计系统

各客户端共享同一套品牌识别与结构，同时在各自平台上保持原生观感。设计分为三层：

| 层 | 原则 | 内容 |
| :--- | :--- | :--- |
| 一致层 | 定义一次，处处相同 | tokens、内容图标、页面及其导航、组件、屏幕控制器、文案 |
| 映射层 | 概念相同，以平台的方式表达 | 导航容器、更多操作菜单、设置中的控件 |
| 平台层 | 遵循平台本身 | 窗口边框、菜单栏、材质、系统字体、返回手势、电视焦点、无障碍、系统图标 |

### 6.1 设计 tokens
颜色、字体、间距、圆角、层级与动效在 `libs/design/tokens` 中以 W3C Design Tokens（DTCG）格式一次定义。由 Style Dictionary 解析，`pnpm nx run design:generate` 将其与图标、字符串一起编译到 `libs/design/gen/{ts,swift,kotlin,go}`。生成的代码提交入库，CI 检查其是否最新，与 `libs/proto/gen` 相同。
* **颜色**：深色主题优先，适合海报图与暗光环境，另有浅色主题；两者对比度均达到 WCAG AA，由生成器检查每一组文字与背景。
* **字体**：字号层级（字号、字重、行高）共享；字体族用平台的系统字体——Apple 平台用 SF Pro，Android 用 Roboto，Windows 用 Segoe UI Variable，Linux 与 Web 用随应用附带的 Inter——以便 Dynamic Type 与字体缩放生效。品牌字体只用于标志与展示性标题。
* **按设备形态取值**：tokens 有手机、平板、桌面与电视四档取值（电视按 3 米观看距离设计，并留出防过扫描边距）。

### 6.2 图标
内容与品牌图标使用 [Phosphor](https://phosphoricons.com/)（MIT），固定版本，`libs/design/icons/icons.json` 将语义名称映射到 Phosphor 名称；生成器把其 SVG 源复制到 `libs/design/icons/svg`，并编译为供 Web UI 与 MyGo（自行绘制）使用的路径数据、资源目录图像与 Android 矢量图。Regular 与 Fill 两种字重标示未激活与激活状态（收藏、静音、循环、字幕），不引起布局偏移。用户期待系统图标之处，系统控件使用平台图标——Apple 平台用 SF Symbols（分享、AirPlay、画中画、菜单），Android 用 Material Symbols。

### 6.3 信息架构
所有客户端拥有相同的页面、相同的名称与相同的到达路径：
* **一级目的地**：首页（§6.3.1）、各媒体库、搜索、下载（仅应用），以及含设置的个人资料。
* **下级页面**：媒体库（带排序与筛选的网格）、条目详情（电影、剧集、季、人物、合集）、播放器与设置。
* **导航容器**，按平台映射：iPhone 用标签栏；iPad、Mac、Windows、Linux 与宽 Web 窗口用侧边栏；Android 按窗口尺寸类别用导航栏或导航侧栏；Apple TV 用顶部标签栏；Android TV 用侧边菜单。

#### 6.3.1 首页
首页是一列推荐行，仿照 Plex：每一行条目都是观看某些内容的一个理由。
* **推荐行**：首先是继续观看与已可观看的请求，然后是每个置顶媒体库的推荐行——最近添加、最近上映、最近播出（单集）、评分最高、未看的高分作品、开始观看（尚未开始的剧集）、重温（很久以前看过的）、因为你看过、因为你喜欢、更多某类型、主演、导演，以及管理员推荐的合集——最后是热门：发现功能提供、但没有服务器拥有的作品，可供请求。
* **由谁选择**：管理员为每个媒体库选择哪些推荐行出现在其"推荐"标签页、哪些出现在首页；每个用户置顶并排序媒体库（这决定其推荐行的顺序），并可隐藏推荐行。继续观看不可隐藏。
* **轮换**：以某个条目、类型或人物为种子的推荐行（因为你看过、更多某类型、主演、导演）从用户最近播放的内容中选取种子，每天重新选择，因此首页每天不同，而一天之内保持不变。
* **媒体库标签页**：媒体库打开时显示其"推荐"标签页（即其推荐行），旁边是全部条目的网格。

### 6.4 组件
核心组件在 `libs/design/components.md` 中一次规定——结构、状态（默认、获得焦点、按下、加载、观看进度、已看完）、各设备形态的尺寸与所用 tokens——再在各平台原生实现；共享的是规范，不是代码。它们包括海报与缩略图卡片、横向列表、详情页头部、剧集行、人物卡片、音轨与字幕选择器，以及空、错误与离线状态。电视为每张卡片增加获得焦点状态（放大与阴影）。

### 6.5 屏幕控制器（OSC）
所有播放器遵循同一布局与状态机：
* **顶栏**：标题、季与集，以及投屏或 AirPlay 操作。
* **中央**：播放与暂停；触屏上双击跳转、纵向滑动调节亮度与音量、长按倍速。
* **底栏**：已播放与剩余时间、带章节标记与 trickplay 缩略图的进度条、音轨与字幕选择及字幕偏移、播放速度，以及全屏与画中画。
* **隐藏**：播放中 3 秒无输入后控件淡出。
* **输入**：桌面与 Web 上的键盘快捷键（空格、方向键、`f`、`m`、`j`/`l`）；电视上的方向键与遥控器，控制器遵循平台的焦点机制（tvOS 焦点引擎、Compose for TV 焦点），获得焦点时保持显示。

### 6.6 文案与本地化
所有客户端的字符串在 `libs/design/strings` 中以 ICU MessageFormat 一次定义，以英文为源语言，编译为 Web UI 的消息包、Apple 平台的 String Catalog（`.xcstrings`）、Android 的 `strings.xml`，以及供 Windows 与 Linux 使用的 Go 包。同一事物在各处使用同一术语，复数遵循 CLDR 规则，每个字符串只翻译一次。仅某一平台使用的字符串带有该平台的标签。

### 6.7 平台惯例
品牌一致不凌驾于平台之上：
* **窗口边框**：macOS 红绿灯在左；Windows 与 Linux 标题栏按钮在右。
* **菜单**：macOS 菜单栏；其他平台用上下文菜单与弹出层；更多操作在 Apple 平台与桌面上是菜单，在 Android 手机上是底部面板。
* **材质**：Apple 平台用 vibrancy，Windows 11 用 Mica。
* **触控**：点击目标至少 44 × 44 pt，并遵守安全区域。
* **导航**：使用平台的返回手势与返回键；设置使用平台的控件（开关、选择器、列表）。
* **无障碍**：VoiceOver、TalkBack 与屏幕阅读器，Dynamic Type 与字体缩放，减少动态效果，对比度达到 WCAG AA。

### 6.8 组件画廊与视觉评审
* **组件画廊**：每个平台以固定数据浏览所有组件的所有状态——Web 用 Storybook，Apple 平台用 SwiftUI 预览，Android 用 Compose 预览，Windows 与 Linux 用一个画廊窗口。
* **快照测试**：每个客户端在快照测试中以固定数据渲染同一组页面——Web 用 Playwright，Apple 平台用 swift-snapshot-testing，Android 用 Roborazzi，Windows 与 Linux 用 MyGo 的离屏渲染。CI 将其与入库的参考图比对，并生成一个把每个页面的各平台并排展示的页面，修改 UI 的 PR 附上该页面的链接以供评审。
* **功能对齐**：`libs/design/parity.md` 列出每个功能在各客户端的状态（已上线、计划中、不适用），使客户端缺少某功能只能出于决定；为某个客户端增加功能的 PR 同时更新它。

---

## 7. 依赖方向与工具

* `apps/web` → `libs/client-ts` → `libs/proto`（TypeScript）；所有客户端 → `libs/design`。
* `apps/ios`、`apps/tvos`、`apps/macos` → `libs/apple` → `libs/client-rust`（Swift 绑定）。
* `apps/android`、`apps/android-tv` → `libs/android` → `libs/client-rust`（Kotlin 绑定）。
* `apps/windows-linux` → `libs/client-rust`（C ABI）与 `libs/proto`（Go）。它与服务端的模块一样是 Go 模块，遵循仓库的 Go 约定；depguard 规则不允许它依赖任何服务端库。
* `libs/client-rust` → `libs/proto`（Rust）。
* 应用之间互不依赖。服务端只为内嵌的文件依赖 `apps/web`。
* 所有客户端由 Nx 编排：`tools/nx-go` 像推断其他 Go 模块一样推断 `apps/windows-linux`，Vite 与 TypeScript 用其 Nx 插件，Gradle 用 `@nx/gradle`，Cargo 与 `xcodebuild` 用各自的目标。macOS runner 成本高，CI 中的 Apple 与 Android 任务只在受影响时运行。

---

## 8. 目录结构

```text
mavio/
├── apps/
│   ├── server/                  # [Go] 服务端；internal/webui 内嵌 Web UI
│   ├── web/                     # [TS] Web UI（React + Vite）
│   ├── ios/                     # [Swift] iPhone 与 iPad
│   ├── tvos/                    # [Swift] Apple TV
│   ├── macos/                   # [Swift] Mac（AppKit）
│   ├── android/                 # [Kotlin] 手机、平板、折叠屏
│   ├── android-tv/              # [Kotlin] Android TV、Google TV
│   └── windows-linux/           # [Go] Windows 与 Linux（MyGo、libmpv）
│
├── libs/
│   ├── proto/                   # 契约；gen/go、gen/ts、gen/rust
│   ├── client-ts/               # [TS] connect-es 客户端与 Query hooks
│   ├── client-rust/             # [Rust] 共享核心的 Cargo workspace
│   │   ├── api/                 # 生成的消息与 Connect 客户端（mavio-api）
│   │   ├── core/                # 服务器、事件、离线存储、上报、下载
│   │   ├── uniffi/              # 面向 Swift 与 Kotlin 的 UniFFI 导出
│   │   ├── capi/                # 面向 Go 的 C ABI
│   │   └── proto/               # C ABI 的消息
│   ├── apple/                   # [Swift] MavioKit、MavioPlayer、MavioUI
│   ├── android/                 # [Kotlin] core、player、ui
│   └── design/                  # tokens、图标、组件、字符串；gen/{ts,swift,kotlin,go}
```

# 插件平台

> [English](plugins.md) | 简体中文

> 相关文档：[系统架构](architecture.zh-CN.md) · [领域模型](domain.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

> [架构 §7](architecture.zh-CN.md#7-插件系统libsplugin) 描述两种插件运行时，以及服务端如何调用插件。本文档描述插件如何获得 Jellyfin 插件所能获得的能力（频道与直播电视除外）：服务端 API、事件、HTTP 路由、任务、数据目录、远程设备以及其余提供者类型。本文档还描述 DLNA：Mavio 把它作为基于这些能力构建的第一方插件发布。相应工作是[路线图](roadmap.zh-CN.md)的 P13（平台）与 P14（DLNA）阶段。

---

## 1. 原则

* **契约，而非内部实现。** Jellyfin 插件运行在服务端进程内，通过依赖注入获取任意内部服务。Mavio 插件保持隔离：只通过服务端公开的 Connect API 与 `libs/proto/mavio/plugin/v1` 的契约接触服务端。插件能做什么由其 manifest 声明，由宿主授予。
* **一份契约，两种运行时。** 每种能力对 WASM 插件与进程插件的工作方式相同，WASM 沙箱做不到的除外（§9）。
* **只做增量。** 新能力、manifest 字段与宿主服务都追加到 `mavio.plugin.v1`；已发布的内容不改变含义。

## 2. Jellyfin 扩展点及其对应

| Jellyfin | Mavio | § |
| :--- | :--- | :--- |
| 依赖注入服务端服务（`IPluginServiceRegistrator`、`ILibraryManager`、`IUserDataManager`、`ISessionManager` 等） | 宿主 API：按作用域调用服务端的 Connect 服务，可代表某个用户 | §3 |
| `IEventConsumer<T>`、媒体库与会话事件 | `EVENT_CONSUMER`：按类型接收领域事件与活动事件 | §4 |
| 从插件程序集加载的 Controller（`AddApplicationPart`）、`IHasWebPages` | `HTTP_HANDLER`：插件自己在 `/plugins/{id}/` 下的路由 | §5 |
| `IScheduledTask` | `TASK_RUNNER`：在 manifest 中声明、由服务端调度的任务 | §6 |
| 插件数据目录（`DataFolderPath`） | 每个插件一个可写的数据目录 | §6 |
| `ISessionController` | `DEVICE_CONTROLLER`：插件注册并控制的远程设备 | §7 |
| `IRemoteMetadataProvider`、`IRemoteSearchProvider` | `METADATA_PROVIDER`（已有） | — |
| `IRemoteImageProvider` | `IMAGE_PROVIDER` | §8.1 |
| `IExternalId`、`IExternalUrlProvider` | manifest 中的外部 ID 声明 | §8.2 |
| `ILocalMetadataProvider` | `LOCAL_METADATA`：读取宿主传入的旁挂文件 | §8.3 |
| `IMetadataSaver` | `METADATA_SAVER`：返回由宿主写入的文件 | §8.3 |
| `ICustomMetadataProvider`、`IPreRefreshProvider` | `METADATA_PROCESSOR`：在刷新中修改条目的元数据 | §8.4 |
| `ILyricProvider` | `LYRICS_PROVIDER` | §8.5 |
| `ISubtitleProvider` | `SUBTITLE_PROVIDER`（已有） | — |
| `IMediaSegmentProvider` | `SEGMENT_PROVIDER`（已有） | — |
| `IItemResolver`、`IMultiItemResolver`、`IResolverIgnoreRule` | `RESOLVER`：忽略规则，以及由插件解析的文件夹 | §8.6 |
| `ILibraryPostScanTask` | `library.scanned` 的 `EVENT_CONSUMER` 配合宿主 API | §4 |
| `IIntroProvider` | `INTRO_PROVIDER` | §8.7 |
| `IDynamicImageProvider` | `IMAGE_GENERATOR` | §8.8 |
| `IMediaSourceProvider` | `MEDIA_SOURCE_PROVIDER` | §8.9 |
| `IAuthenticationProvider` | `AUTH_PROVIDER`（已有） | — |
| `IPasswordResetProvider` | `PASSWORD_RESET_PROVIDER` | §8.10 |
| 通知服务（webhook、邮件） | `NOTIFIER`（已有），现在接收 §4 的全部活动 | §4 |
| 按媒体库类型的提供者顺序（`MetadataFetcherOrder`、`ImageFetcherOrder` 等） | 按媒体库的提供者顺序与开关 | §8.11 |
| 插件仓库 | 目录（catalog，已有） | — |
| `IChannel`、`ILiveTvService`、`ITunerHost`、`IListingsProvider` | 不采用 | — |

## 3. 宿主 API

### 3.1 作用域
manifest 在 `permissions.api` 中列出插件可以调用的服务：`<package>.<Service>` 表示全部方法，`<package>.<Service>:read` 表示标注了 `NO_SIDE_EFFECTS` 的方法，例如 `mavio.library.v1.ItemService:read`。其他调用返回 `permission_denied`。

### 3.2 插件以谁的身份行事
* 默认情况下插件以自身身份行事：在其作用域内是管理员，但没有自己的用户，因此涉及调用者自身状态的方法（用户数据、播放）返回 `failed_precondition`。
* 声明 `permissions.act_as_users` 后，请求可以在 `Mavio-User` 头中指定一个用户；此时请求以该用户身份、在该用户的策略下执行，如同该用户亲自发出。被禁用或不存在的用户返回 `unauthenticated`。
* 代表用户行事的设备控制器（§7）还可以在 `Mavio-Device` 中指定自己的某个设备，使其开启的播放归属于该设备的会话。指定它未列出的设备，或指定设备却未指定用户，返回 `failed_precondition`。

### 3.3 传输
插件通过 `guest.HostClient()` 返回的客户端，以基础 URL `http://mavio.host` 访问 API；该客户端既承载 Connect 调用，也承载普通 HTTP 媒体路由：
* **WASM**：经 `http_fetch` 发往 `mavio.host` 的请求在进程内由服务端的处理器树处理，不经过网络。
* **进程**：宿主在插件的私有 socket 目录中的第二个 socket 上提供其处理器树，路径由 `MAVIO_HOST_SOCKET` 给出。

请求到达的通道即标识了插件：宿主会丢弃请求携带的任何凭据（`Authorization`、cookie），因此插件无法借用用户的会话。进程插件的宿主 API socket 要求与插件自身 socket 相同的、每次启动生成的 token。

### 3.4 限制
* WASM 插件调用宿主 API 时，它自身的调用正占用着它的一个实例；如果宿主调用需要同一个插件，会等待另一个空闲实例，直到调用超时仍无空闲实例则失败。
* WASM 插件一次性收到完整的宿主 API 响应，上限 64 MiB，因此 `EventService.Subscribe` 这类服务端流只供进程插件使用。

## 4. 事件

### 4.1 事件类型
事件是 `mavio.plugin.v1.Event`：ID、类型、时间、标题、消息、条目、用户与附加属性。位置以毫秒计。

| 类型 | 时机 | 活动日志 |
| :--- | :--- | :--- |
| `user.login`、`user.login_failed` | 登录 | 是 |
| `user.created`、`user.updated`、`user.deleted`、`user.password_changed` | 账户变更；事件的用户是该账户，附加属性 `by` 是做出变更的用户 | 是 |
| `apikey.created`、`apikey.revoked` | API 密钥 | 是 |
| `settings.updated`、`backup.created` | 管理操作 | 是 |
| `plugin.installed`、`plugin.updated`、`plugin.configured`、`plugin.uninstalled`、`plugin.failed` | 插件；插件启动失败时为 `plugin.failed` | 是 |
| `task.completed`、`task.failed` | 任务的一次运行结束；附加属性 `task`（任务在 `TaskService` 中的 ID），插件任务另有 `plugin`，其消息即该次运行的消息 | 仅失败 |
| `playback.started`、`playback.stopped` | 播放开始或停止，包括过期或登录结束时；附加属性 `playback`、`session` 与 `position`，停止时另有 `played` | 是 |
| `subtitle.downloaded`、`subtitle.download_failed` | 字幕下载；附加属性 `provider` 与 `subtitle` | 仅失败 |
| `item.added`、`item.updated`、`item.removed` | 媒体库变更，在其事务提交后；附加属性 `library`。仅在有插件消费时才计算 | 否 |
| `library.scanned` | 一次媒体库扫描结束；附加属性 `library` 与 `succeeded` | 否 |
| `userdata.changed` | 用户的条目状态变化：`played`、`favorite`、`position` | 否 |

### 4.2 投递
* `NOTIFIER` 插件像以前一样接收活动日志事件。
* `EVENT_CONSUMER` 插件通过 `EventConsumerService.Consume` 接收类型与 `permissions.events` 中模式（`item.added`、`item.*` 或 `*`）匹配的事件，每批最多 100 个，聚集一秒后发送。
* 每个插件有自己的队列，最多 10,000 个事件；失败的批次以退避方式重试三次，超出队列容量或用尽重试的事件被丢弃并记录警告。对每个插件按顺序投递，至多一次。

## 5. HTTP 路由
* 声明 `HTTP_HANDLER` 的插件在 `/plugins/{id}/` 下处理所有方法，在根路径与基础 URL 下均可访问，等待配置期间也是如此。宿主去掉前缀，把请求交给插件以 `guest.HandleHTTP` 注册的处理器，并以 `X-Forwarded-For`、`X-Forwarded-Host` 与 `X-Forwarded-Proto` 描述客户端的请求。在插件内部，路由位于一个保留路径下，与其 Connect 服务分开。
* 认证由插件负责：路由无需令牌即可访问，因为 DLNA 渲染器与 OAuth 回调需要如此。请求携带有效的 Bearer 令牌时，宿主去掉令牌并添加 `Mavio-User-Id`、`Mavio-User-Name` 与 `Mavio-User-Admin`；不带令牌的请求中的这些头会被去掉。其他 `Authorization` 头原样到达插件。
* 每个响应都带有 `Content-Security-Policy: sandbox allow-scripts allow-forms allow-popups`，使插件提供的页面运行在不透明源中，无法读取服务端源的存储。
* manifest 可以把其路由中的一个页面指定为 `config_page`，即相对于 `/plugins/{id}/` 的路径，例如 `settings`，客户端把它与配置 schema 描述的表单并列打开。
* **运行时**：进程插件通过其 socket 上的流式反向代理访问，代理在 `Mavio-Plugin-Token` 中出示插件的令牌，使请求保留自己的 `Authorization`。WASM 请求会被缓冲并受调用超时限制，每个方向至多 16 MiB；ABI 请求信封在请求体之后携带方法与查询字符串，旧的 guest 会忽略它们。

## 6. 任务与数据目录
* **任务**：`TASK_RUNNER` 插件的 manifest 列出任务（ID、名称、说明、至少一分钟的间隔、超时）。它们以 `plugin:{plugin}:{task}` 出现在 `TaskService` 中，并带有插件 ID；它们作为 `plugin.task` 作业在单独的 worker 上运行，调用 `TaskRunnerService.RunTask`：自插件启动起按间隔运行，每次运行排入下一次；也可按需运行。任务的超时（至多六小时，未设置时为一小时）在该次调用中取代 WASM 的调用超时。插件未就绪时跳过定时运行；已不存在的任务的运行被丢弃。
* **数据目录**：每个插件有一个可写目录 `<home>/plugin-data/<id>`（`--plugin-data-dir`），升级时保留，纳入备份，卸载时删除。两种运行时都通过 `MAVIO_PLUGIN_DATA` 告知插件其路径（`guest.DataDir()`）；WASM 插件在 `/data` 看到它。

## 7. 远程设备
* `DEVICE_CONTROLLER` 插件（还需要 `permissions.act_as_users`）通过 `HostService.SetDevices` 保持其设备列表为最新：ID、名称、产品，以及每个设备接受的命令种类。任何插件都可以不经作用域调用 `mavio.plugin.v1.HostService`；其他调用者返回 `permission_denied`。
* 服务端把每个设备列为 `SessionService.ListSessions` 中的一个会话，带插件 ID、没有用户；插件列出它期间为在线，并且是共享的——每个已登录用户都能看到它、向它发送命令，并获知它的变化。再次列出的设备保留原会话；插件不再列出的设备，或已停止插件的设备，连同其播放一并消失。
* 向设备 `SendCommand` 时，服务端在 30 秒内调用 `DeviceControllerService.SendCommand`，传入命令、设备及其会话，以及发送它的用户；设备不接受的命令返回 `failed_precondition`，插件的错误原样返回给发送者。插件以该用户和该设备的身份执行命令（§3.2）：带着设备的能力通过 `PlaybackService` 开启并上报播放，于是设备的会话显示它正在播放的内容。

## 8. 提供者能力

### 8.1 图片提供者
`ImageProviderService.GetImages` 针对一次查找返回远程图片（类型、URL、尺寸、语言、评分）。`MetadataService.ListRemoteImages` 把它们与元数据提供者的图片一起列出；刷新时按提供者顺序从中取得条目缺少的图片类型。

### 8.2 外部 ID
manifest 在 `external_id_kinds` 中声明外部 ID 类型：键、显示名称、携带它的媒体类型（包括人物），以及可选的 URL 模板，例如 `https://trakt.tv/movies/{id}`。内置类型涵盖 TMDB、IMDb、TheTVDB、TVmaze、AniDB、AniList、aniSearch 与 MusicBrainz；已启动插件的类型排在其后，已列出的键不会被重新定义。`MetadataService.ListExternalIdKinds` 列出它们，可只列某一条目类型的；`ItemService.GetItem` 与 `GetPerson` 返回 `external_urls`，即条目或人物各 ID 的页面，ID 按路径段转义。

### 8.3 本地元数据与保存器
* `LOCAL_METADATA` 插件在 manifest 的 `local_metadata_files` 中声明文件名模式，例如 `*.yaml`。刷新条目时，宿主读取条目文件夹（剧集等文件夹类条目即其自身）中匹配的文件，至多 16 个、每个至多 1 MiB，把文件名与内容连同条目媒体的名称传给 `LocalMetadataService.ReadMetadata`，后者像提供者一样返回元数据。本地元数据的优先级仅次于 NFO，高于提供者；替换元数据的刷新与忽略 NFO 一样忽略它。
* `METADATA_SAVER` 插件在 `MetadataSaverService.SaveMetadata` 中收到条目保存后的元数据（含演职人员与远程图片），返回文件（相对条目文件夹的名称与内容）；在保存本地元数据的媒体库中，宿主把它们与 NFO 文件一起写到该文件夹，除非其中有文件超出该文件夹、会替换媒体本身或超出上述限制。

### 8.4 元数据处理器
`MetadataProcessorService.ProcessMetadata` 在刷新结束、保存之前收到条目合并后的元数据（含最可信来源的演职人员），返回要覆盖其上的元数据：已设置的字段替换条目的字段（已锁定的除外），人员在演员表未锁定时替换其演职人员。处理器依次运行，每个都能看到之前处理器的结果；某个失败时不做任何改动。

### 8.5 歌词提供者
`LyricsProviderService.SearchLyrics`（曲目名、艺人、专辑、时长、外部 ID）返回歌词及其 ID，`DownloadLyrics` 返回歌词：同步歌词为 LRC，否则为纯文本，至多 1 MiB。`ItemService.SearchRemoteLyrics` 与 `DownloadRemoteLyrics` 仅供管理员使用，向每个歌词提供者搜索，并把选中的歌词以 `.lrc` 或 `.txt` 保存到曲目旁边，替换其其他歌词文件；无法解析的歌词或没有时间的同步歌词会被拒绝。在启用了歌词提供者的媒体库中，刷新时为没有歌词的曲目下载歌词（§8.11）。

### 8.6 解析器
`ResolverService.Ignore` 返回文件夹中哪些条目应被忽略；`ResolverService.Resolve` 可以认领一个文件夹并返回其中的条目，像内置解析器那样，且先于内置解析器执行。解析器在扫描的每个文件夹上运行，因此只能是 WASM 插件。

请求携带文件夹信息（媒体库类型、媒体库文件夹、路径、上级文件夹解析成了什么、所属季的编号）及其条目名称。先按插件顺序运行每个解析器的 `Ignore`；然后由第一个认领该文件夹的解析器解析它，没有解析器认领的文件夹交给内置解析器。被认领的文件夹本身可以是一个条目（`folder_item`：剧集、季、专辑），并包含由其文件构成的条目，每个条目带有名称、年份、序号、外部 ID 以及分段视频的后续文件；还包含接下来要扫描的子文件夹及该文件夹对它们而言是什么。条目必须是扫描会产生的类型，且必须对应已存在的条目；附加内容和多版本仍由内置解析器处理。解析器失败或返回无效结果时，该文件夹按不可读处理：扫描保留其下已存储的条目。

### 8.7 片头提供者
`IntroProviderService.GetIntros`（条目 ID、类型与名称，用户 ID）返回某用户播放某条目之前要播放的条目 ID，例如其本地预告片或片前广告，插件通过宿主 API 查找这些条目。`PlaybackService.ListIntros` 按提供者顺序返回它们，每个条目只出现一次，至多 20 个，并排除条目本身、不存在的条目、不能播放的条目以及用户无权访问的条目；某个提供者失败时保留其他提供者的结果。

### 8.8 图片生成器
`ImageGeneratorService.GenerateImages`（条目 ID、类型与名称，以及它缺少的图片类型）返回其中部分类型的图片，格式为 JPEG、PNG 或 WebP，每张至多 8 MiB。刷新结束时，按顺序向生成器请求所有来源都未提供的图片类型，每种类型只生成一次；生成的图片保存在元数据目录（`<id[:2]>/<id>/generated/<kind>.<ext>`），排在其他所有来源之后。之后的刷新保留它们而不再请求，替换式刷新会重新生成，一旦其他来源提供了该类型，生成的图片即被删除。未请求类型的图片或不是图片的内容会被拒绝。

### 8.9 媒体源提供者
`MediaSourceProviderService.GetMediaSources` 返回条目额外的媒体源，每个包含 ID、名称和一个 http(s) URL。播放开始时，服务端询问每个媒体源提供者，用 ffprobe 探测每个 URL（探测结果复用一小时，失败结果复用五分钟），并由插件、条目和插件给出的 ID 派生出稳定的媒体源 ID。播放决策在条目的文件之后考虑它们：服务端通过重定向到该 URL 直放它们，或由 ffmpeg 读取该 URL 进行转封装或转码；它们的码率不像本地文件那样受限。条目详情只列出条目的文件。

### 8.10 密码重置
`AuthService.ForgotPassword` 为使用密码登录的用户生成一个八位一次性 PIN，连同用户 ID、用户名与过期时间交给服务端设置中选定插件（`password_reset_plugin`）的 `PasswordResetService.StartReset`，由插件以自己的方式（邮件、聊天）投递。无论用户是否存在，它的应答都相同，投递在后台进行。服务端只在内存中保存 PIN 的哈希 30 分钟；新的 PIN 替换该用户之前的 PIN，输错五次即作废。`AuthService.ResetPassword` 凭 PIN 设置新密码，并结束该用户的所有会话。未选定插件时，`ForgotPassword` 以 `FAILED_PRECONDITION` 失败。

### 8.11 按媒体库的提供者顺序
媒体库按能力（元数据、图片、字幕、歌词、片段）保存使用哪些提供者以及顺序。未设置时表示按插件 ID 顺序使用所有就绪的提供者，与现在一致。

## 9. 运行时差异
| | WASM | 进程 |
| :--- | :--- | :--- |
| 宿主 API | 经 `http_fetch` | 经宿主 socket |
| HTTP 路由 | 缓冲，16 MiB | 流式 |
| 网络 | 仅 `http_hosts` | 操作系统的全部网络，包括 UDP |
| 解析器 | 可以 | 不可以 |
| 长任务 | 至多任务超时 | 不限 |

## 10. DLNA（`plugins/dlna`）
DLNA 是第一方进程插件：它需要 UDP 组播（WASM 没有 socket）和长期运行的监听。它为服务端支持的每个平台构建，发布在官方目录中，并打包进容器镜像。

### 10.1 能力
`HTTP_HANDLER`（设备描述、控制与事件 URL、媒体）、`DEVICE_CONTROLLER`（推送播放）、`EVENT_CONSUMER`（`item.*`，用于内容更新），以及带 `act_as_users` 和媒体库、条目、播放、用户、系统服务作用域的宿主 API。

### 10.2 媒体服务器
* **发现**：在每个网络接口的 239.255.255.250:1900 上运行 SSDP：发送 `NOTIFY` alive 与 byebye，并应答针对根设备、`MediaServer:1`、`ContentDirectory:1`、`ConnectionManager:1` 与 `X_MS_MediaReceiverRegistrar:1` 的 `M-SEARCH`。描述 URL 使用请求到达的网络接口上的服务端地址，以及 `HostService.GetServerInfo` 给出的 HTTP 端口与基础 URL。
* **内容**：以配置的用户身份执行 `ContentDirectory` 的 `Browse` 与 `Search`，覆盖该用户的媒体库、文件夹、剧集、季、专辑、艺人、播放列表与合集，以 DIDL-Lite 输出；`SystemUpdateID` 随媒体库事件变化，并通过 GENA 通告。
* **媒体**：每个 `res` URL 都是指向该条目的插件路由；对它的请求会通过 `PlaybackService` 按渲染器的 profile 开启一次播放（直放，或渐进式转封装、转码，因为多数渲染器不支持 HLS），并以支持 Range 的方式代理，带上渲染器期望的 DLNA 头（`contentFeatures.dlna.org`、`transferMode.dlna.org`）。profile 接受时，字幕以 `res` 或 `CaptionInfo.sec` 提供，否则烧录。

### 10.3 推送播放（Play To）
* 插件搜索 `MediaRenderer:1` 设备，读取其描述，并以匹配的 profile 把每个设备注册为设备（§7）。
* 命令映射到 `AVTransport` 与 `RenderingControl`：播放（以播放的 URL 调用 `SetAVTransportURI`，再 `Play`）、暂停、继续、停止、跳转、在插件维护的队列中切换下一项和上一项、音量与静音。插件每秒轮询一次传输状态并上报进度；连续三次轮询都不在的渲染器，其播放被停止。

### 10.4 设备 profile
根据渲染器的描述（制造商、型号、友好名称）与请求头（`User-Agent`、`X-AV-Client-Info`）把渲染器匹配到 profile；profile 以 `ClientCapabilities` 描述其接受的容器、编解码器与字幕格式，并记录其特殊行为（是否支持按时间跳转、DLNA 标志）。其余设备使用通用 profile；管理员可以在插件配置中添加或覆盖 profile。

### 10.5 配置
提供哪个用户的媒体库、显示的服务器名称、是否启用媒体服务器与推送播放、使用的网络接口、通告间隔（默认 30 分钟），以及 profile 覆盖。

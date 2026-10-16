# 插件平台

> [English](plugins.md) | 简体中文

> 相关文档：[系统架构](architecture.zh-CN.md) · [领域模型](domain.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

> [架构 §7](architecture.zh-CN.md#7-插件系统libsplugin) 描述两种插件运行时，以及服务端如何调用插件。本文档描述插件如何获得 Jellyfin 插件所能获得的能力（频道与直播电视除外）：服务端 API、事件、HTTP 路由、任务、数据目录、远程设备以及其余提供者类型。本文档还描述 DLNA：Mavio 把它作为基于这些能力构建的第一方插件发布；以及资源获取新增的提供者类型。相应工作是[路线图](roadmap.zh-CN.md)的 P13（平台）与 P14（DLNA）阶段，资源获取提供者在 P15 至 P18。

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
| 插件仓库 | 目录（catalog，已有）；第一方插件的目录是 mavioai/plugins 的最新发布版本（[架构 §7.5](architecture.zh-CN.md#75-服务端中的插件)） | — |
| 插件在其仓库中的分类 | 清单与目录中的 `category`：元数据、字幕、下载、身份验证、通知、设备或 AI，客户端据此对插件分组 | — |
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
| `ai.credential_changed`、`ai.login_started` | AI 提供者的凭据（§8.15）；`provider` 属性 | 是 |
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
* **宿主操作系统**：两种运行时都通过 `MAVIO_HOST_OS`（`guest.HostOS()`）告知宿主的操作系统及其版本，按 Servarr 应用在 User-Agent 中的写法小写，如 `ubuntu 22.04`：Linux 上取自 `/etc/*release` 的 `ID` 与 `VERSION_ID`，macOS 上取自系统版本属性列表，Windows 上取自 `RtlGetVersion`。

## 7. 远程设备
* `DEVICE_CONTROLLER` 插件（还需要 `permissions.act_as_users`）通过 `HostService.SetDevices` 保持其设备列表为最新：ID、名称、产品，以及每个设备接受的命令种类。任何插件都可以不经作用域调用 `mavio.plugin.v1.HostService`；其他调用者返回 `permission_denied`。其 `GetServerInfo` 给出服务端的名称、版本、HTTP 与 HTTPS 端口以及基础 URL，供插件生成交给局域网设备的 URL。
* 服务端把每个设备列为 `SessionService.ListSessions` 中的一个会话，带插件 ID、没有用户；插件列出它期间为在线，并且是共享的——每个已登录用户都能看到它、向它发送命令，并获知它的变化。再次列出的设备保留原会话；插件不再列出的设备，或已停止插件的设备，连同其播放一并消失。
* 命令可以把条目作为队列播放、暂停、继续、停止、在队列中前后切换、跳转、显示消息，或设置音量与静音。向设备 `SendCommand` 时，服务端在 30 秒内调用 `DeviceControllerService.SendCommand`，传入命令、设备及其会话，以及发送它的用户；设备不接受的命令返回 `failed_precondition`，插件的错误原样返回给发送者。插件以该用户和该设备的身份执行命令（§3.2）：带着设备的能力通过 `PlaybackService` 开启并上报播放，于是设备的会话显示它正在播放的内容。

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

请求携带文件夹信息（媒体库类型、媒体库文件夹、路径、上级文件夹解析成了什么、所属季的编号）及其条目名称。先按插件顺序运行每个解析器的 `Ignore`；然后由第一个认领该文件夹的解析器解析它，没有解析器认领的文件夹交给内置解析器。被认领的文件夹本身可以是一个条目（`folder_item`：剧集、季、专辑、相册），并包含由其文件构成的条目，每个条目带有名称、年份、序号、外部 ID 以及分段视频的后续文件；还包含接下来要扫描的子文件夹及该文件夹对它们而言是什么。条目必须是扫描会产生的类型，且必须对应已存在的条目；附加内容和多版本仍由内置解析器处理。解析器失败或返回无效结果时，该文件夹按不可读处理：扫描保留其下已存储的条目。

### 8.7 片头提供者
`IntroProviderService.GetIntros`（条目 ID、类型与名称，用户 ID）返回某用户播放某条目之前要播放的条目 ID，例如其本地预告片或片前广告，插件通过宿主 API 查找这些条目。`PlaybackService.ListIntros` 按提供者顺序返回它们，每个条目只出现一次，至多 20 个，并排除条目本身、不存在的条目、不能播放的条目以及用户无权访问的条目；某个提供者失败时保留其他提供者的结果。

### 8.8 图片生成器
`ImageGeneratorService.GenerateImages`（条目 ID、类型与名称，以及它缺少的图片类型）返回其中部分类型的图片，格式为 JPEG、PNG 或 WebP，每张至多 8 MiB。刷新结束时，按顺序向生成器请求所有来源都未提供的图片类型，每种类型只生成一次；生成的图片保存在元数据目录（`<id[:2]>/<id>/generated/<kind>.<ext>`），排在其他所有来源之后。之后的刷新保留它们而不再请求，替换式刷新会重新生成，一旦其他来源提供了该类型，生成的图片即被删除。未请求类型的图片或不是图片的内容会被拒绝。

### 8.9 媒体源提供者
`MediaSourceProviderService.GetMediaSources` 返回条目额外的媒体源，每个包含 ID、名称和一个 http(s) URL。播放开始时，服务端询问每个媒体源提供者，用 ffprobe 探测每个 URL（探测结果复用一小时，失败结果复用五分钟），并由插件、条目和插件给出的 ID 派生出稳定的媒体源 ID。播放决策在条目的文件之后考虑它们：服务端通过重定向到该 URL 直放它们，或由 ffmpeg 读取该 URL 进行转封装或转码；它们的码率不像本地文件那样受限。条目详情只列出条目的文件；`PlaybackService.ListMediaSources` 先列出这些文件，再列出媒体源提供者给出的媒体源（标记为 `remote`），供客户端选择播放。

### 8.10 密码重置
`AuthService.ForgotPassword` 为使用密码登录的用户生成一个八位一次性 PIN，连同用户 ID、用户名与过期时间交给服务端设置中选定插件（`password_reset_plugin`）的 `PasswordResetService.StartReset`，由插件以自己的方式（邮件、聊天）投递。无论用户是否存在，它的应答都相同，投递在后台进行。服务端只在内存中保存 PIN 的哈希 30 分钟；新的 PIN 替换该用户之前的 PIN，输错五次即作废。`AuthService.ResetPassword` 凭 PIN 设置新密码，并结束该用户的所有会话。未选定插件时，`ForgotPassword` 以 `FAILED_PRECONDITION` 失败。

### 8.11 按媒体库的提供者顺序
媒体库按能力（元数据、图片、字幕、歌词、片段、集列表）以插件 ID 保存使用哪些提供者以及顺序（`LibrarySpec.provider_order`）。未设置的列表表示按插件 ID 顺序使用所有就绪的提供者；空列表表示不使用任何提供者；未运行插件的 ID 被跳过。刷新、元数据与图片搜索、字幕与歌词的搜索和下载，以及媒体片段作业，都只按媒体库的顺序询问它的提供者。

设置 `download_lyrics` 后，刷新一个旁边没有歌词文件的音轨时，会搜索媒体库的歌词提供者，并保存第一个能下载的歌词，与管理员手动下载相同。

文件包含多集的单集，其查询以 `episode_number_end` 带上最后一集的集号；元数据提供者随后把这些单集作为一个条目描述，名称与简介以 ` / ` 连接，与 Jellyfin 的 TmdbEpisodeProvider 相同。`scraper-tmdb` 和 `scraper-servarr` 都这样做。

在条目保存的内容之外，元数据提供者还给出作品页显示的信息，与 Seerr 相同：电影的预算与票房、剧集下一集的播出日期，以及在查询所在国家以订阅方式播放该作品的流媒体服务。主机不保存这些信息，由 `DiscoverService.GetTitle` 返回。`scraper-tmdb` 从 TMDB 的详情与观看提供方中取得它们。它还在 `remote_trailers` 中给出作品在 YouTube 上的预告片，由条目保存，取自 TMDB 中查询语言与英语的视频。

### 8.12 资源获取提供者
资源获取（[架构 §13](architecture.zh-CN.md#13-资源获取libsacquire)）新增五种提供者类型，都由服务端汇集，彼此之间从不互相调用：
* `EPISODE_LIST_PROVIDER`：`EpisodeListProviderService.ListEpisodes` 按查询的 `display_order` 编号列出剧集的季与单集，附带绝对集数、元数据，以及标识它们的外部 ID（`provider`）；刷新剧集时，从媒体库中第一个认识该剧集的集列表提供者取得列表，把剧集缺少的单集建为虚拟条目（[领域模型 §4.8](domain.zh-CN.md#48-虚拟条目)）。`scraper-tmdb` 提供它：按播出顺序，或按该显示顺序类型的第一个 TMDB episode group，以其分组作为季。`scraper-servarr` 根据 Sonarr 的 SkyHook 提供它，无论显示顺序如何都按 TVDB 编号，与 Sonarr 给 release 和文件编号的方式一致；`scraper-tmdb` 按带有 TVDB ID 的单集的该 ID 查找它，从而无论这类单集如何编号都能给它们 TMDB 的元数据。
* `DISCOVERY_PROVIDER`：`DiscoveryProviderService` 列出可供发现的电影与剧集。`ListCatalogs` 声明提供者的目录，每个目录有其种类（趋势、热门、即将上映、高分、某作品的推荐或相似作品、别处整理的列表如榜单，或某个合集的电影），针对电影、剧集或两者，并给出它接受的过滤条件与排序；`Discover` 返回目录中一页作品，附带外部 ID、日期、类型、图片与评分，可按类型、制片公司、电视网、关键词、观看平台、原始语言、上映日期、片长、评分、投票数与内容分级过滤；`ListFilterValues` 列出某个过滤条件的取值，或与查询匹配的取值；`SearchTitles` 按名称逐页查找电影与剧集，不支持搜索的提供者返回 UNIMPLEMENTED。`DiscoverService` 标明服务端已有、想要或已被请求的作品。`scraper-tmdb` 按 Seerr 使用 TMDB 的方式提供它：当日趋势、通过 TMDB discover 接口的热门与即将上映作品、高分作品、推荐与相似作品，以及某个合集的电影（合集种类的目录，接受 `tmdb_collection` 外部 ID），并按 Seerr 的方式搜索，支持 `tmdb:`、`imdb:`、`tvdb:` 与 `year:` 前缀。`scraper-servarr` 提供 Radarr 的电影列表：TMDB 的趋势与热门电影、IMDb Top 250 与热门电影、按评分的 StevenLu 列表，以及某部电影的推荐；它不支持搜索。
* `SCENE_MAPPING_PROVIDER`：`SceneMappingProviderService` 说明 release 以不同于集列表的方式命名和编号剧集的情况，用于把 release 匹配到剧集与单集：`ListSceneMappings` 列出 release 对剧集或其部分季使用的标题，附带其编号的季、用于搜索索引器的标题，以及 release 必须匹配的正则表达式，每条映射来自提供者的某个来源，来源新列出的映射取代它之前列出的；`ListDailySeries` 列出 release 按播出日期命名的剧集；`GetSceneNumbering` 列出 release 对剧集单集的编号与其集列表不同之处。`scraper-servarr` 根据 Sonarr 的服务（scene 映射与日播剧）与 XEM（按 TVDB 的标题与编号）提供它，二者作为两个来源；其中一个不可用时只列出另一个的映射。
* `INDEXER_PROVIDER`：`IndexerProviderService` 搜索索引器。`ListDefinitions` 列出提供者实现的索引器种类（站点，或 Newznab 等协议）及每种所需的设置，并标明其中的机密；插件配置列出据此配置的索引器，`ListIndexers` 列出它们及其能力（Newznab 的搜索参数与分类）、是否因失败而暂被排除，以及服务器的决策与抓取所用的设置（标签、MULTi 发布的语言，torrent 索引器还有必需标志、最少做种数、分享率、做种时间、整季包做种时间，以及抓取时是否拒绝哈希已被拉黑的种子），以及总是接收该索引器 release 的下载客户端；`TestIndexer` 检查索引器的设置；`SearchReleases` 按 ID、标题、季与集在 Newznab 分类中、在时间与大小限制内搜索电影、剧集或文本，无任何搜索条件时读取最近的 release，返回 release（索引器提供时附带来源组、片源、容器、编码与分辨率）与失败的索引器；`GetReleaseFile` 获取 release 的种子文件或 NZB，或其下载链接指向的磁力链接，并拒绝无效的 NZB；`ReleaseService` 搜索每个就绪的索引器提供者，并保存其索引器一小时，直到插件重启或重新配置。`plugins/indexers` 把站点给出的内容（Cookie、Newznab 能力、API 令牌）与每个索引器的状态一起保存在其数据目录中，跨重启保留。由 `plugins/indexers` 提供。
* `DOWNLOAD_CLIENT_PROVIDER`：`DownloadClientProviderService` 把 release 发送给下载客户端。`ListDownloadClientDefinitions` 如索引器定义那样列出提供者实现的客户端种类及每种所需的设置；插件配置列出据此配置的客户端，每个客户端有电影与剧集各自的分类，`ListDownloadClients` 列出它们的协议、优先级、标签、主机（用于远程路径映射）、是否偏好种子文件而非磁力链接，以及是否因失败而暂被排除；`TestDownloadClient` 检查客户端的设置，对不妨碍其工作的问题（缺少分类、开发版本）返回警告；`AddDownload` 把电影或剧集的种子文件、磁力链接或 NZB 发送给客户端，并标明是否为近期发行、附带其索引器要求的做种条件、是否为整季（Pneumatic 拒绝整季）、release 及其电影或剧集的信息（质量、语言、发布组、索引器、年份、合集、制片公司、标题 slug，Flood 将其添加为标签），以及为其拉黑的种子哈希（release 未给出哈希的种子，若其哈希在其中则被拒绝），返回下载的 ID；客户端不接受磁力链接时以 unimplemented 失败，由服务端改发种子文件，客户端拒绝该 release 时以 failed precondition 失败；`ListDownloads` 列出客户端分类中的下载及其状态、大小、分享率与输出路径，以及各客户端自身的信息（输出目录、是否自行删除已完成的下载、其整理方式）；`GetDownload` 在入库前重新读取下载；`RemoveDownload` 删除下载，按需连同其文件；`MarkDownloadImported` 把已入库的下载移到客户端的已入库分类。`DownloadService` 如同 Radarr 的 DownloadClientProvider，把每个 release 发送给其协议的客户端：与电影或剧集共享某个标签的客户端，否则为无标签的客户端；索引器指定了下载客户端时即用它；否则在其中未因失败而暂被排除者里（若全被排除，则为全部，除非该 release 因等待客户端恢复而处于待定），优先级最高的那些轮流使用。由 `plugins/download-clients` 提供；它把抓取给出的各种子的做种限制保存在数据目录中（`seeds.json`），使其在重启后保留，并按 `history_limit` 列出 Usenet 客户端的历史条数。

两个面向网络的提供者都是进程插件：索引器与下载客户端的地址由管理员配置，WASM 清单的 `http_hosts` 无法列出它们，而且索引器需要代理与长期会话。

### 8.13 字幕提供者
`SUBTITLE_PROVIDER` 为字幕获取（[架构 §14](architecture.zh-CN.md#14-字幕获取libsacquiresubtitles)）服务，如 Bazarr 的提供者为其服务。`SubtitleProviderService.ListSubtitleProviders` 列出插件实现的提供者及各自的设置（标出机密项）、是否启用、是否被限流及限流到何时；`SearchSubtitles` 接受视频的描述（名称、别名、ID、季、集与绝对集数、年份、release 名称及其来源、发布组、分辨率、编码、版本与流媒体服务、帧率、文件名、大小与 OpenSubtitles 哈希）与多种语言及其 forced 与听障要求，返回带提供者、release 名称、语言、标记以及提供者判定匹配的属性的候选，由服务端评分；`DownloadSubtitle` 返回其中之一，已从压缩包中取出。失败带有类型（请求过多、下载额度、服务不可用、认证、配置、应阻止的字幕），插件据此限流该提供者。`plugins/subtitles` 以进程插件提供它：其站点位于登录、cookie、Cloudflare 与验证码之后，有的自托管在管理员给出的地址，且它在数据目录中保存会话、限流状态与验证码服务的账号。

对于读取视频本身的提供者（如 Whisper），视频的描述还带有其各音频流及文件中记录的语言标签，以及 `audio_url`：宿主 API 的一个 URL（`/subtitles/audio/{token}`），以 16 kHz 单声道 16 位 PCM 提供一条音频流，其起点与视频对齐；参数 `stream` 选择音频流，`seconds` 只取开头若干秒。服务端只在读取音频的地方给出它，即装有 ffmpeg 时的本地与网络存储；该路由只经宿主 API 应答，其不可猜测的令牌在最后一次给出它的搜索或下载之后一天内有效。

### 8.14 字幕翻译器
`SUBTITLE_TRANSLATOR` 如 Lingarr 一样以翻译服务翻译字幕（[架构 §14.5](architecture.zh-CN.md#145-翻译与转写)）。`SubtitleTranslatorService.ListSubtitleTranslators` 列出插件实现的翻译器及其设置、是否启用、每次接受多少字幕条、是否保留 SubRip 标记以及可翻译的源语言与目标语言（BCP 47，为空表示任意），服务端据此排列翻译器链；`TranslateSubtitleLines` 接受翻译器、源语言与目标语言、字幕条的文字（纯文本，或对保留标记的翻译器为 SubRip 中的形式）以及电影或剧集的描述，返回同样数量的译文（空译文保留原字幕条）及署名。服务端解析字幕、分批发送字幕条并写出译文。`plugins/subtitles` 实现了 Lingarr 的翻译服务：DeepL（以账户密钥调用其 API，每次请求五十条，以 `:fx` 结尾的密钥使用免费 API）与 LibreTranslate（服务器 URL 及可选密钥，每次请求 25 条），二者的语言由服务列出并保留一天；以及如 Lingarr 所用的 GTranslate 库那样无需密钥访问的 Google 翻译（其浏览器扩展的端点，带请求令牌）、Bing Translator（其页面的端点，使用页面的凭据，每小时重新读取）、Microsoft Translator（其 Android 应用的 API，请求按该应用的方式签名）与 Yandex.Translate（其 Android 应用的 API），每次请求一条、同时四条、每批二十条，语言为 Lingarr 为它们列出的语言。语言按 BCP 47 匹配到服务的代码（`zh` 为 `zh-Hans`，`pt` 为 `pt-BR`）。限速与服务不可用如实报告，由服务端重试；被拒的密钥、用尽的配额与服务缺少的语言不重试，任何一条失败则其所在批次失败。

### 8.15 AI 网关与智能体会话
`AI_GATEWAY` 代表服务端访问 AI 模型提供商，使服务端的任何功能都不持有凭据。`AiGatewayService.ListAiProviders` 列出提供商及其认证方式（API key、OAuth 登录、登录是否为订阅登录，如 Claude Pro 或 ChatGPT Plus）、已保存的凭据与是否可用；`ListAiModels` 列出其模型，以及上下文窗口、推理、图片输入与价格。`SetAiApiKey` 保存或删除 key，`LogoutAi` 删除凭据。OAuth 登录在插件中运行：`StartAiLogin` 开始登录，`GetAiLogin` 返回登录展示的内容（登录页面、设备码、进度）、它询问的内容以及是否结束，`WatchAiLogin` 在每次变化时以流式传送同样的内容，直到登录结束，`AnswerAiLogin` 作答，例如在提供商的重定向无法到达插件时提交浏览器被重定向到的地址；客户端经由服务端跟随登录。`CompleteAi` 请求模型给出一个回答，请求给出 schema 时回答为符合它的 JSON，并返回用量与费用；`StreamAi` 随生成以流式传送回答的文本与推理，最后给出同样的响应。失败带有 Connect 错误码：值得重试的失败（限流、过载、网络）为 unavailable，没有可用凭据为 unauthenticated，提供商或模型未知为 not found，上下文溢出为 invalid argument。

`AGENT_SESSION` 持久地保存与模型的会话（`AgentSessionService`）：以模型与指令创建，可列出、连同消息读取、以 `SendAgentMessage` 继续（其请求 ID 使重试成为同一条消息）或以 `StreamAgentMessage` 继续（随回答增长以流式传送目前为止的回答，最后给出回答），以及删除。每条消息与回答都在返回之前保存。

`plugins/agent` 以 Bun 编译的 TypeScript 子进程插件提供这两项能力，基于 [pi-ai](https://github.com/earendil-works/pi) 与 pi-durable：pi-ai 支持的所有提供商，带 API key、OAuth 登录（包括订阅登录；客户端提示风险并交由管理员决定）以及其配置添加的、由 OpenAI 兼容 API（如 Ollama 的）提供的提供商；凭据保存在其数据目录中（`credentials.json`，只有服务端的用户可读），会话保存在那里的 SQLite 数据库中，因此进行中的回答在重启后继续。pi-durable 永久保留对话的条目：插件记录已删除的会话（`deleted-sessions.json`），在下次启动、打开数据库之前删除它们的对话，连同它们拥有的对话及其文档。按 schema 的回答经由模型调用的工具给出，提供商支持时使用约束采样。

## 9. 运行时差异
| | WASM | 进程 |
| :--- | :--- | :--- |
| 宿主 API | 经 `http_fetch` | 经宿主 socket |
| HTTP 路由 | 缓冲，16 MiB | 流式 |
| 网络 | 仅 `http_hosts` | 操作系统的全部网络，包括 UDP |
| 解析器 | 可以 | 不可以 |
| 长任务 | 至多任务超时 | 不限 |

## 10. DLNA（`plugins/dlna`）
DLNA 是第一方进程插件：它需要 UDP 组播（WASM 没有 socket）和长期运行的监听。它为服务端支持的每个平台构建，并打包进容器镜像（§10.6）。

### 10.1 能力
`HTTP_HANDLER`（设备描述、控制与事件 URL、媒体）、`DEVICE_CONTROLLER`（推送播放）、`EVENT_CONSUMER`（`item.*`，用于内容更新），以及带 `act_as_users` 和 `LibraryService:read`、`ItemService:read`、`PlaybackService` 作用域的宿主 API。

### 10.2 媒体服务器
* **发现**：在每个网络接口的 239.255.255.250:1900 上运行 SSDP：发送 `NOTIFY` alive 与 byebye，并应答针对根设备、`MediaServer:1`、`ContentDirectory:1`、`ConnectionManager:1` 与 `X_MS_MediaReceiverRegistrar:1` 的 `M-SEARCH`。描述 URL 使用请求到达的网络接口上的服务端地址，以及 `HostService.GetServerInfo` 给出的 HTTP 端口与基础 URL。
* **内容**：以配置的用户身份执行 `ContentDirectory` 的 `Browse` 与 `Search`，覆盖该用户的媒体库、文件夹、剧集、季、专辑、艺人、播放列表与合集，以 DIDL-Lite 输出；`SystemUpdateID` 随媒体库事件变化，并通过 GENA 通告。
* **媒体**：每个 `res` URL 都是插件路由 `media/{item}/{file}`，profile 由 `?profile=` 指定或按请求头匹配。请求会通过 `PlaybackService` 按渲染器的 profile 开启一次播放（直放，或渐进式转封装、转码，因为多数渲染器不支持 HLS）；同一用户、条目、profile 与起点的请求共用一次播放，空闲四分钟后释放。宿主的媒体路由以支持 Range 的方式代理，并带上渲染器期望的 DLNA 头（`contentFeatures.dlna.org`，其 `DLNA.ORG_OP` 在直放时为 01、按时间跳转时为 10；以及 `transferMode.dlna.org`）；`TimeSeekRange.dlna.org` 让转码从给定时间重新开始。profile 接受时，字幕以指向 `subtitles/{item}/{file}`（SRT）的 `res` 或 `CaptionInfo.sec` 提供，否则烧录。图片经由 `images/{id}?size=` 以 JPEG 缩略图或大图提供。

### 10.3 推送播放（Play To）
* 插件每五分钟搜索一次 `MediaRenderer:1` 设备并跟随其通告，读取其描述，并以匹配的 profile 把每个设备注册为设备（§7）。配置中列出的渲染器（例如位于其他子网的渲染器）从其描述 URL 读取。
* 命令映射到 `AVTransport` 与 `RenderingControl`：播放（以路由 `play/{playback}/stream.{ext}` 及其 DIDL-Lite 调用 `SetAVTransportURI`，再 `Play`）、暂停、继续、停止、跳转（直放用 `REL_TIME`；转码从该位置重新开始）、在插件维护的队列中切换下一项和上一项、音量与静音。插件每秒轮询一次传输状态，至多每五秒上报一次进度；渲染器在结束前十秒内停止即视为播完，并开始下一项。渲染器改放其他 URI，或连续三次轮询失败，其播放被停止。

### 10.4 设备 profile
内置 profile 覆盖三星、LG、索尼与松下电视以及 Kodi、VLC，其余设备使用通用 profile。profile 包含 `name`；`match` 规则，即对 `friendly_name`、`manufacturer`、`model_name`、`model_number` 与请求 `headers` 的正则表达式，所设字段须全部匹配；以 JSON 形式的 `ClientCapabilities` 描述其接受内容的 `capabilities`；以及其特殊行为：`time_seek`、`caption_info`、`subtitle_res`，和替换容器 MIME 类型的 `mime_types`。配置中的 profile 替换同名的内置 profile，并先于其他 profile 尝试。

### 10.5 配置
提供其媒体库的 `user`（必填）、显示的 `server_name`、`media_server` 与 `play_to`（默认开启）、使用的 `interfaces`（默认为所有已启用、支持组播且有 IPv4 地址的接口）、`announce_interval_minutes`（默认 30）、`ssdp_port`（供测试）、`renderers`（额外的描述 URL）与 `profiles`。

### 10.6 打包
`tools/plugindist` 与其他第一方插件一起，为 Linux、macOS 与 Windows 的 amd64 和 arm64 构建插件，并把每个平台打成包含 `manifest.json` 与可执行文件的目录 zip 包（[开发指南 §7](development.zh-CN.md#7-发布与版本)）。容器镜像在其插件种子目录中附带该插件（架构文档 §12）；SSDP 组播只能到达使用宿主网络的容器。

# Plugin Platform

> English | [简体中文](plugins.zh-CN.md)

> Related: [Architecture](architecture.md) · [Domain Model](domain.md) · [Roadmap](roadmap.md) · [AGENTS.md](../AGENTS.md)

> [Architecture §7](architecture.md#7-plugin-system-libsplugin) describes the two plugin runtimes and how the server calls plugins. This document describes how plugins reach what Jellyfin's plugins reach, apart from channels and live TV: the server's API, events, HTTP routes, tasks, a data folder, remote devices and the remaining provider types. It also describes DLNA, which Mavio ships as a first-party plugin built on them. The work is phase P13 (platform) and P14 (DLNA) of the [Roadmap](roadmap.md).

---

## 1. Principles

* **Contracts, not internals.** Jellyfin plugins run in the server's process and take any internal service by dependency injection. Mavio plugins keep their isolation: they reach the server only through its public Connect API and the contracts of `libs/proto/mavio/plugin/v1`. What a plugin may do is declared in its manifest and granted by the host.
* **One contract, both runtimes.** Every capability works the same for WASM and process plugins, except where the WASM sandbox cannot (§9).
* **Additive.** New capabilities, manifest fields and host services are added to `mavio.plugin.v1`; nothing published changes meaning.

## 2. Jellyfin Extension Points and Their Counterparts

| Jellyfin | Mavio | § |
| :--- | :--- | :--- |
| Dependency injection of server services (`IPluginServiceRegistrator`, `ILibraryManager`, `IUserDataManager`, `ISessionManager`, …) | Host API: the server's Connect services, by scope, optionally acting as a user | §3 |
| `IEventConsumer<T>`, library and session events | `EVENT_CONSUMER`: domain and activity events, by type | §4 |
| Controllers loaded from the plugin assembly (`AddApplicationPart`), `IHasWebPages` | `HTTP_HANDLER`: the plugin's own routes under `/plugins/{id}/` | §5 |
| `IScheduledTask` | `TASK_RUNNER`: tasks declared in the manifest, scheduled by the server | §6 |
| Plugin data folder (`DataFolderPath`) | A writable data folder per plugin | §6 |
| `ISessionController` | `DEVICE_CONTROLLER`: remote devices the plugin registers and controls | §7 |
| `IRemoteMetadataProvider`, `IRemoteSearchProvider` | `METADATA_PROVIDER` (exists) | — |
| `IRemoteImageProvider` | `IMAGE_PROVIDER` | §8.1 |
| `IExternalId`, `IExternalUrlProvider` | External ID declarations in the manifest | §8.2 |
| `ILocalMetadataProvider` | `LOCAL_METADATA`: reads sidecar files the host passes | §8.3 |
| `IMetadataSaver` | `METADATA_SAVER`: returns files the host writes | §8.3 |
| `ICustomMetadataProvider`, `IPreRefreshProvider` | `METADATA_PROCESSOR`: changes an item's metadata during a refresh | §8.4 |
| `ILyricProvider` | `LYRICS_PROVIDER` | §8.5 |
| `ISubtitleProvider` | `SUBTITLE_PROVIDER` (exists) | — |
| `IMediaSegmentProvider` | `SEGMENT_PROVIDER` (exists) | — |
| `IItemResolver`, `IMultiItemResolver`, `IResolverIgnoreRule` | `RESOLVER`: ignore rules and folders the plugin resolves | §8.6 |
| `ILibraryPostScanTask` | `EVENT_CONSUMER` of `library.scanned` with the host API | §4 |
| `IIntroProvider` | `INTRO_PROVIDER` | §8.7 |
| `IDynamicImageProvider` | `IMAGE_GENERATOR` | §8.8 |
| `IMediaSourceProvider` | `MEDIA_SOURCE_PROVIDER` | §8.9 |
| `IAuthenticationProvider` | `AUTH_PROVIDER` (exists) | — |
| `IPasswordResetProvider` | `PASSWORD_RESET_PROVIDER` | §8.10 |
| Notification services (webhook, e-mail) | `NOTIFIER` (exists), now with every activity of §4 | §4 |
| Provider order per library type (`MetadataFetcherOrder`, `ImageFetcherOrder`, …) | Provider order and switches per library | §8.11 |
| Plugin repositories | Catalogs (exist) | — |
| `IChannel`, `ILiveTvService`, `ITunerHost`, `IListingsProvider` | Not adopted | — |

## 3. Host API

### 3.1 Scopes
A manifest lists the services a plugin may call in `permissions.api`, as `<package>.<Service>` for every method or `<package>.<Service>:read` for the methods marked `NO_SIDE_EFFECTS`, such as `mavio.library.v1.ItemService:read`. Any other call is `permission_denied`.

### 3.2 Who a Plugin Acts As
* By default a plugin acts as itself: an administrator within its scopes, with no user of its own, so methods about the caller's own state (user data, playbacks) fail with `failed_precondition`.
* With `permissions.act_as_users`, a request may name a user in the `Mavio-User` header; it then acts as that user, under the user's policy, as if the user made it. Disabled and unknown users are `unauthenticated`.
* A device controller (§7) may also name one of its devices in `Mavio-Device`, so that the playbacks it starts belong to that device's session.

### 3.3 Transport
Plugins reach the API at the base URL `http://mavio.host` with the client `guest.HostClient()` returns, which carries both Connect calls and the plain HTTP media routes:
* **WASM**: a request to `mavio.host` through `http_fetch` is served in process by the server's handler tree instead of the network.
* **Process**: the host serves its handler tree on a second socket in the plugin's private socket folder, named by `MAVIO_HOST_SOCKET`.

The channel a request arrives on identifies the plugin: the host drops any credentials it carries (`Authorization`, cookies), so a plugin cannot borrow a user's session. The host API socket of a process plugin requires the same per-start token as the plugin's own socket.

### 3.4 Limits
* A WASM plugin's call into the host API runs while its own call holds one of its instances; a host call that needs the same plugin waits for another instance and fails at the call timeout when none comes free.
* WASM plugins receive host API responses whole, up to 64 MiB, so server streams such as `EventService.Subscribe` are for process plugins only.

## 4. Events

### 4.1 Event Types
Events are `mavio.plugin.v1.Event`: ID, type, time, title, message, item, user and attributes.

| Type | When | Activity log |
| :--- | :--- | :--- |
| `user.login`, `user.login_failed` | Sign-in | Yes |
| `user.created`, `user.updated`, `user.deleted`, `user.password_changed` | Account changes | Yes |
| `apikey.created`, `apikey.revoked` | API keys | Yes |
| `settings.updated`, `backup.created` | Administration | Yes |
| `plugin.installed`, `plugin.updated`, `plugin.uninstalled`, `plugin.failed` | Plugins | Yes |
| `task.completed`, `task.failed` | A task's run ends | Failures only |
| `playback.started`, `playback.stopped` | A playback starts or stops; `position` and `played` attributes | Yes |
| `subtitle.downloaded`, `subtitle.download_failed` | Subtitle downloads | Failures only |
| `item.added`, `item.updated`, `item.removed` | Library changes, after their transaction commits; `library` attribute | No |
| `library.scanned` | A library scan ends | No |
| `userdata.changed` | A user's item state changes: `played`, `favorite`, `position` | No |

### 4.2 Delivery
* `NOTIFIER` plugins receive the activity log events as before.
* `EVENT_CONSUMER` plugins receive the events whose types match the patterns of `permissions.events` (`item.added`, `item.*` or `*`), through `EventConsumerService.Consume`, in batches of up to 100 events gathered for one second.
* Each plugin has its own queue of up to 10,000 events; a failed batch is retried three times with backoff, and events beyond the queue's size or past their retries are dropped with a warning. Delivery is at most once per plugin, in order.

## 5. HTTP Routes
* A plugin declaring `HTTP_HANDLER` serves every method under `/plugins/{id}/`, at the root and under the base URL. The host strips the prefix and delivers the request to the handler the plugin registers with `guest.HandleHTTP`.
* Authentication is the plugin's: routes are reachable without a token, as DLNA renderers and OAuth callbacks require. When a request carries a valid bearer token, the host removes it and adds `Mavio-User-Id`, `Mavio-User-Name` and `Mavio-User-Admin`; it strips those headers from requests without one.
* Every response carries `Content-Security-Policy: sandbox allow-scripts allow-forms allow-popups`, so that a page a plugin serves runs in an opaque origin and cannot read the server origin's storage.
* A manifest may name a page among its routes as `config_page`, which clients open next to the form the configuration schema describes.
* **Runtimes**: process plugins are reached by a streaming reverse proxy over their socket. WASM requests are buffered, at most 16 MiB each way; the ABI request envelope carries the method and query after the body, which earlier guests ignore.

## 6. Tasks and Data Folder
* **Tasks**: a manifest of a `TASK_RUNNER` plugin lists tasks (ID, name, description, interval of at least a minute, timeout). They appear in `TaskService` as `plugin:{plugin}:{task}`, with the plugin's ID, and run as `plugin.task` jobs on a worker of their own, calling `TaskRunnerService.RunTask`: on their interval from the plugin's start, each run queuing the next, or on demand. A task's timeout, at most six hours and one hour when unset, replaces the WASM call timeout for that call. Scheduled runs of a plugin that is not ready are skipped; runs of tasks that are gone are dropped.
* **Data folder**: each plugin has a writable folder `<home>/plugin-data/<id>` (`--plugin-data-dir`), kept across upgrades, included in backups and deleted on uninstall. Both runtimes tell plugins its path in `MAVIO_PLUGIN_DATA` (`guest.DataDir()`); WASM plugins see it at `/data`.

## 7. Remote Devices
* A `DEVICE_CONTROLLER` plugin keeps its devices up to date with the host service `HostService.SetDevices`: ID, name, product, the client capabilities playback decisions use, and the commands each takes.
* The server lists every device as a session of `SessionService.ListSessions`, online while the plugin lists it, shared: every signed-in user sees them and may send them commands.
* `SendCommand` to a device calls `DeviceControllerService.SendCommand` with the command and the user who sent it. The plugin carries it out acting as that user and device (§3.2): it starts and reports the playback through `PlaybackService`, so the device's session shows what it plays.

## 8. Provider Capabilities

### 8.1 Image Providers
`ImageProviderService.GetImages` returns remote images (kind, URL, size, language, rating) for a lookup. `MetadataService.ListRemoteImages` lists them with the metadata providers' images, and refreshes take images of kinds an item lacks from them in provider order.

### 8.2 External IDs
A manifest declares external ID kinds: key, display name, the item kinds that have them, and a URL template such as `https://trakt.tv/movies/{id}`. `MetadataService.ListExternalIdKinds` lists the built-in and declared kinds, and items carry `external_urls` built from their IDs.

### 8.3 Local Metadata and Savers
* `LOCAL_METADATA` plugins declare file name patterns in the manifest. While refreshing an item, the host reads the matching files beside it, up to 1 MiB each, and passes their names and contents to `LocalMetadataService.Read`, which returns metadata the way providers do; local metadata takes precedence over providers, after NFO.
* `METADATA_SAVER` plugins receive an item's saved metadata in `MetadataSaverService.Save` and return files (name relative to the item's folder, contents); the host writes them beside the media in libraries saving local metadata.

### 8.4 Metadata Processors
`MetadataProcessorService.Process` receives the merged metadata of an item at the end of a refresh, before it is saved, and returns metadata to apply over it; locked fields are kept.

### 8.5 Lyrics Providers
`LyricsProviderService.SearchLyrics` (track name, artists, album, duration) and `DownloadLyrics`. `ItemService.SearchRemoteLyrics` and `DownloadRemoteLyrics` save the lyrics beside the track as `.lrc` or `.txt`; refreshes download them for tracks without lyrics in libraries that enable a lyrics provider.

### 8.6 Resolvers
`ResolverService.Ignore` returns which entries of a folder to leave out; `ResolverService.Resolve` may claim a folder and return the items it holds, as the built-in resolvers do, before them. Resolvers run on every folder of a scan, so they are WASM plugins only.

### 8.7 Intro Providers
`IntroProviderService.GetIntros` returns the IDs of items to play before an item for a user, such as local trailers. `PlaybackService.ListIntros` returns them, filtered by the user's access.

### 8.8 Image Generators
`ImageGeneratorService.Generate` makes an image of a kind for an item that has none, returned as bytes and stored in the metadata folder as the item's image.

### 8.9 Media Source Providers
`MediaSourceProviderService.GetMediaSources` returns extra media sources of an item: an HTTP URL with its probed streams. Playback decisions consider them with the item's files; the server direct plays them by redirect and remuxes or transcodes them with ffmpeg reading the URL.

### 8.10 Password Reset
`AuthService.ForgotPassword` passes a user name to the `PasswordResetService.StartReset` of the plugin chosen in the server settings, which delivers a one-time PIN its own way (e-mail, chat); the server keeps the PIN's hash for 30 minutes, and `AuthService.ResetPassword` with the PIN sets a new password.

### 8.11 Provider Order per Library
A library stores, per capability (metadata, images, subtitles, lyrics, segments), which providers it uses and in which order. Unset means every ready provider in plugin ID order, as today.

## 9. Runtime Differences
| | WASM | Process |
| :--- | :--- | :--- |
| Host API | Through `http_fetch` | Through the host socket |
| HTTP routes | Buffered, 16 MiB | Streaming |
| Network | `http_hosts` only | The operating system's, including UDP |
| Resolvers | Yes | No |
| Long tasks | Up to the task timeout | Unlimited |

## 10. DLNA (`plugins/dlna`)
DLNA is a first-party process plugin: it needs UDP multicast, which WASM has no sockets for, and long-lived listeners. It is built for every platform the server is, published in the official catalog and bundled in the container image.

### 10.1 Capabilities
`HTTP_HANDLER` (descriptions, control and event URLs, media), `DEVICE_CONTROLLER` (Play To), `EVENT_CONSUMER` (`item.*`, for content updates), and the host API with `act_as_users` and scopes on the library, item, playback, user and system services.

### 10.2 Media Server
* **Discovery**: SSDP on 239.255.255.250:1900 on every interface: `NOTIFY` alive and byebye, answers to `M-SEARCH` for the root device, `MediaServer:1`, `ContentDirectory:1`, `ConnectionManager:1` and `X_MS_MediaReceiverRegistrar:1`. The description URL is the server's address on the interface the request came in on, with its HTTP port and base URL from `HostService.GetServerInfo`.
* **Content**: `ContentDirectory` `Browse` and `Search` as the configured user, over the user's libraries, folders, series, seasons, albums, artists, playlists and collections, as DIDL-Lite; `SystemUpdateID` follows library events, announced by GENA.
* **Media**: each `res` URL is a plugin route for the item; a request to it starts a playback for the renderer's profile through `PlaybackService` (direct play, or a progressive remux or transcode, since most renderers take no HLS) and is proxied with ranges, with the DLNA headers renderers expect (`contentFeatures.dlna.org`, `transferMode.dlna.org`). Subtitles go as `res` or `CaptionInfo.sec` when the profile takes them, else are burned in.

### 10.3 Play To
* The plugin searches for `MediaRenderer:1` devices, reads their descriptions and registers each as a device (§7) with the profile matching it.
* Commands map to `AVTransport` and `RenderingControl`: play (`SetAVTransportURI` with the playback's URL, `Play`), pause, resume, stop, seek, next and previous within the queue it keeps, volume and mute. The plugin polls the transport every second and reports progress; a renderer gone for three polls stops its playback.

### 10.4 Device Profiles
Renderers are matched by their description (manufacturer, model, friendly name) and request headers (`User-Agent`, `X-AV-Client-Info`) to profiles holding the containers, codecs and subtitle formats they take, as `ClientCapabilities`, and their quirks (time-seek support, DLNA flags). A generic profile applies to the rest; administrators add or override profiles in the plugin's configuration.

### 10.5 Configuration
The user whose libraries are served, the server name shown, enabling the media server and Play To, the interfaces to use, the announcement interval (default 30 minutes) and profile overrides.

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
* A device controller (§7) acting as a user may also name one of its devices in `Mavio-Device`, so that the playbacks it starts belong to that device's session. A device it does not list, or a device without a user, is `failed_precondition`.

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
Events are `mavio.plugin.v1.Event`: ID, type, time, title, message, item, user and attributes. Positions are in milliseconds.

| Type | When | Activity log |
| :--- | :--- | :--- |
| `user.login`, `user.login_failed` | Sign-in | Yes |
| `user.created`, `user.updated`, `user.deleted`, `user.password_changed` | Account changes; the event's user is the account, the `by` attribute the user who changed it | Yes |
| `apikey.created`, `apikey.revoked` | API keys | Yes |
| `settings.updated`, `backup.created` | Administration | Yes |
| `plugin.installed`, `plugin.updated`, `plugin.configured`, `plugin.uninstalled`, `plugin.failed` | Plugins; `plugin.failed` when a plugin fails to start | Yes |
| `task.completed`, `task.failed` | A task's run ends; `task` attribute (the task's ID in `TaskService`), and `plugin` for plugin tasks, whose message is the run's | Failures only |
| `playback.started`, `playback.stopped` | A playback starts or stops, also when it expires or its sign-in ends; `playback`, `session` and `position` attributes, and `played` when it stops | Yes |
| `subtitle.downloaded`, `subtitle.download_failed` | Subtitle downloads; `provider` and `subtitle` attributes | Failures only |
| `item.added`, `item.updated`, `item.removed` | Library changes, after their transaction commits; `library` attribute. They are worked out only while a plugin consumes them | No |
| `library.scanned` | A library scan ends; `library` and `succeeded` attributes | No |
| `userdata.changed` | A user's item state changes: `played`, `favorite`, `position` | No |

### 4.2 Delivery
* `NOTIFIER` plugins receive the activity log events as before.
* `EVENT_CONSUMER` plugins receive the events whose types match the patterns of `permissions.events` (`item.added`, `item.*` or `*`), through `EventConsumerService.Consume`, in batches of up to 100 events gathered for one second.
* Each plugin has its own queue of up to 10,000 events; a failed batch is retried three times with backoff, and events beyond the queue's size or past their retries are dropped with a warning. Delivery is at most once per plugin, in order.

## 5. HTTP Routes
* A plugin declaring `HTTP_HANDLER` serves every method under `/plugins/{id}/`, at the root and under the base URL, also while it waits for its configuration. The host strips the prefix and delivers the request to the handler the plugin registers with `guest.HandleHTTP`, with `X-Forwarded-For`, `X-Forwarded-Host` and `X-Forwarded-Proto` describing the client's request. Inside the plugin, routes live under a reserved path apart from its Connect services.
* Authentication is the plugin's: routes are reachable without a token, as DLNA renderers and OAuth callbacks require. When a request carries a valid bearer token, the host removes it and adds `Mavio-User-Id`, `Mavio-User-Name` and `Mavio-User-Admin`; it strips those headers from requests without one. Other `Authorization` headers reach the plugin unchanged.
* Every response carries `Content-Security-Policy: sandbox allow-scripts allow-forms allow-popups`, so that a page a plugin serves runs in an opaque origin and cannot read the server origin's storage.
* A manifest may name a page among its routes as `config_page`, a path relative to `/plugins/{id}/` such as `settings`, which clients open next to the form the configuration schema describes.
* **Runtimes**: process plugins are reached by a streaming reverse proxy over their socket, which presents the plugin's token in `Mavio-Plugin-Token` so that the request keeps its own `Authorization`. WASM requests are buffered and bounded by the call timeout, at most 16 MiB each way; the ABI request envelope carries the method and query after the body, which earlier guests ignore.

## 6. Tasks and Data Folder
* **Tasks**: a manifest of a `TASK_RUNNER` plugin lists tasks (ID, name, description, interval of at least a minute, timeout). They appear in `TaskService` as `plugin:{plugin}:{task}`, with the plugin's ID, and run as `plugin.task` jobs on a worker of their own, calling `TaskRunnerService.RunTask`: on their interval from the plugin's start, each run queuing the next, or on demand. A task's timeout, at most six hours and one hour when unset, replaces the WASM call timeout for that call. Scheduled runs of a plugin that is not ready are skipped; runs of tasks that are gone are dropped.
* **Data folder**: each plugin has a writable folder `<home>/plugin-data/<id>` (`--plugin-data-dir`), kept across upgrades, included in backups and deleted on uninstall. Both runtimes tell plugins its path in `MAVIO_PLUGIN_DATA` (`guest.DataDir()`); WASM plugins see it at `/data`.

## 7. Remote Devices
* A `DEVICE_CONTROLLER` plugin, which also needs `permissions.act_as_users`, keeps its devices up to date with `HostService.SetDevices`: ID, name, product and the kinds of commands each takes. Every plugin may call `mavio.plugin.v1.HostService` without a scope; other callers are `permission_denied`. Its `GetServerInfo` tells the server's name, version, HTTP and HTTPS ports and base URL, for the URLs a plugin gives devices on the local network.
* The server lists every device as a session of `SessionService.ListSessions`, with the plugin's ID and no user, online while the plugin lists it, shared: every signed-in user sees them, may send them commands and learns of their changes. A device listed again keeps its session; devices the plugin no longer lists, or of a plugin that stops, are gone, and so are their playbacks.
* Commands play items as a queue, pause, resume, stop, skip within the queue, seek, show a message, or set the volume and mute. `SendCommand` to a device calls `DeviceControllerService.SendCommand`, within 30 seconds, with the command, the device and its session, and the user who sent it; a command the device does not take is `failed_precondition`, and the plugin's error is the sender's. The plugin carries it out acting as that user and device (§3.2): it starts and reports the playback through `PlaybackService` with the device's capabilities, so the device's session shows what it plays.

## 8. Provider Capabilities

### 8.1 Image Providers
`ImageProviderService.GetImages` returns remote images (kind, URL, size, language, rating) for a lookup. `MetadataService.ListRemoteImages` lists them with the metadata providers' images, and refreshes take images of kinds an item lacks from them in provider order.

### 8.2 External IDs
A manifest declares external ID kinds in `external_id_kinds`: key, display name, the media kinds that carry them (persons included), and an optional URL template such as `https://trakt.tv/movies/{id}`. The built-in kinds cover TMDB, IMDb, TheTVDB, TVmaze, AniDB, AniList, aniSearch and MusicBrainz; a started plugin's kinds follow them, and a key already listed is not redefined. `MetadataService.ListExternalIdKinds` lists them, optionally for one item kind; `ItemService.GetItem` and `GetPerson` return `external_urls`, the pages of the item's or person's IDs, with each ID escaped as a path segment.

### 8.3 Local Metadata and Savers
* `LOCAL_METADATA` plugins declare file name patterns in the manifest's `local_metadata_files`, such as `*.yaml`. While refreshing an item, the host reads the matching files in the item's folder (the item itself for folders such as series), at most 16 of up to 1 MiB each, and passes their names and contents, with the name of the item's media, to `LocalMetadataService.ReadMetadata`, which returns metadata the way providers do. Local metadata takes precedence over providers, after NFO; refreshes that replace metadata ignore it, as they do NFO.
* `METADATA_SAVER` plugins receive an item's saved metadata, with its credits and remote images, in `MetadataSaverService.SaveMetadata` and return files (name relative to the item's folder, contents); in libraries saving local metadata the host writes them there with the NFO file, unless one of them leaves the folder, replaces the media or exceeds the limits above.

### 8.4 Metadata Processors
`MetadataProcessorService.ProcessMetadata` receives the merged metadata of an item at the end of a refresh, with the credits of the most trusted source, before it is saved, and returns metadata to apply over it: set fields replace the item's, except locked ones, and people replace its credits unless the cast is locked. Processors run in order, each seeing what the ones before did; one failing changes nothing.

### 8.5 Lyrics Providers
`LyricsProviderService.SearchLyrics` (track name, artists, album, duration, external IDs) returns lyrics with their IDs, and `DownloadLyrics` returns them as LRC when synced and plain text otherwise, at most 1 MiB. `ItemService.SearchRemoteLyrics` and `DownloadRemoteLyrics`, for administrators, search every lyrics provider and save the chosen lyrics beside the track as `.lrc` or `.txt`, replacing its other lyric files; lyrics that do not parse, or synced lyrics without times, are refused. Refreshes download them for tracks without lyrics in libraries that enable a lyrics provider (§8.11).

### 8.6 Resolvers
`ResolverService.Ignore` returns which entries of a folder to leave out; `ResolverService.Resolve` may claim a folder and return the items it holds, as the built-in resolvers do, before them. Resolvers run on every folder of a scan, so they are WASM plugins only.

A request carries the folder (library kind, library folder, path, what its containing folder resolved to, the containing season's number) and the names of its entries. Every resolver's `Ignore` runs first, in plugin order; then the first resolver that claims the folder resolves it, and folders no resolver claims go to the built-in resolvers. A claimed folder may be an item itself (`folder_item`: a series, a season, an album, a photo album) and holds items made of its files, each with its name, year, indexes, external IDs and the further files of a stacked video, and the subfolders to scan next with what the folder is to them. Items must be of kinds scans find and name existing entries; extras and versions stay with the built-in resolvers. A resolver that fails or returns an invalid result makes the folder count as unreadable: the scan keeps the items already stored under it.

### 8.7 Intro Providers
`IntroProviderService.GetIntros` (item ID, kind and name, user ID) returns the IDs of items to play before an item for a user, such as its local trailers or a pre-roll, which plugins find through the host API. `PlaybackService.ListIntros` returns them in provider order, each once and at most 20, leaving out the item itself, unknown items, items that do not play and items the user cannot access; a provider failing leaves the others' picks.

### 8.8 Image Generators
`ImageGeneratorService.GenerateImages` (item ID, kind and name, the kinds of images it lacks) returns images of some of those kinds, JPEG, PNG or WebP of at most 8 MiB each. At the end of a refresh, the kinds no source gave are asked of the generators in order, each kind once; the images are kept in the metadata folder (`<id[:2]>/<id>/generated/<kind>.<ext>`) and come after every other source. Later refreshes keep them without asking again, a replacing refresh makes them anew, and one is removed once another source gives its kind. Images of kinds not asked for, or that are not images, are refused.

### 8.9 Media Source Providers
`MediaSourceProviderService.GetMediaSources` returns extra media sources of an item, each an ID, a name and an http(s) URL. When a playback starts, the server asks every media source provider, probes each URL with ffprobe (a probe is reused for an hour, a failure for five minutes) and gives the source a stable ID derived from the plugin, the item and the plugin's ID. Playback decisions consider them after the item's files: the server direct plays them by redirecting to the URL and remuxes or transcodes them with ffmpeg reading the URL; their bitrate is not capped as a local file's is. Item details list only the item's files; `PlaybackService.ListMediaSources` lists the files and then the media source providers' sources, marked `remote`, so that a client can choose one to play.

### 8.10 Password Reset
`AuthService.ForgotPassword` makes an eight-digit one-time PIN for a user who signs in with a password and passes it, with the user's ID and name and its expiry, to the `PasswordResetService.StartReset` of the plugin chosen in the server settings (`password_reset_plugin`), which delivers it its own way (e-mail, chat). It answers alike whether or not the user exists, delivering in the background. The server keeps only the PIN's hash, in memory, for 30 minutes; a new PIN replaces the user's previous one, and five wrong PINs end it. `AuthService.ResetPassword` with the PIN sets a new password and ends the user's sessions. Without a chosen plugin, `ForgotPassword` fails with `FAILED_PRECONDITION`.

### 8.11 Provider Order per Library
A library stores, per capability (metadata, images, subtitles, lyrics, segments), which providers it uses and in which order, by plugin ID (`LibrarySpec.provider_order`). An unset list means every ready provider in plugin ID order; an empty one means none; IDs of plugins not running are skipped. Refreshes, metadata and image searches, subtitle and lyrics searches and downloads, and media segment jobs ask only the library's providers, in its order.

With `download_lyrics` set, a refresh of a track without a lyric file beside it searches the library's lyrics providers and saves the first lyrics that download, as an administrator's download would.

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

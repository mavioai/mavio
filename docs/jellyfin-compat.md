# Jellyfin API Compatibility Shim Evaluation

> English | [简体中文](jellyfin-compat.zh-CN.md)

> Related: [Architecture](architecture.md) · [Domain Model](domain.md) · [Roadmap](roadmap.md) · [AGENTS.md](../AGENTS.md)

> Mavio is not compatible with the Jellyfin client ecosystem ([Architecture §1.2](architecture.md)). This document evaluates a shim that would let existing Jellyfin clients use a Mavio server: what it would take, where it conflicts with Mavio's design, and the form it would take if adopted. **Status: not adopted.** The decision belongs to P13 ([Roadmap](roadmap.md)), once Mavio's own clients are under way. The figures were taken from Jellyfin 12.2 (October 2026).

---

## 1. Conclusion

A shim is feasible: Mavio's identifiers, client capabilities and playback, SyncPlay and browsing rules were ported from Jellyfin, so most of the work is reshaping data rather than reimplementing behavior. It is not worth building now. It would keep an untyped REST surface in step with Jellyfin's releases, against the contract-first principle, and it needs an exception to how Mavio authorizes media requests (§3.2). Should real Jellyfin clients be wanted before Mavio's own, the narrow experiment of §5 is the way in.

## 2. Size of the Surface

* Jellyfin's API has 60 controllers and 397 endpoints (215 GET, 144 POST, 38 DELETE).
* `BaseItemDto`, the item shape most endpoints return, has about 155 properties.
* A client uses a fraction of this. The main path — sign in, browse, play, report progress, receive events — needs a few dozen endpoints.

## 3. Mapping onto Mavio

### 3.1 What Maps Directly
| Jellyfin | Mavio | Work |
| :--- | :--- | :--- |
| GUID item, user and library IDs | `core.ID`, 16 bytes | Format without hyphens |
| `DeviceProfile` (direct play, transcoding, container, codec, subtitle profiles) | `playback.v1.ClientCapabilities`, with the same five kinds of profile | Field translation; the decision rules are Jellyfin's |
| `PlaybackInfo`, progress and stop reports | `PlaybackService` | Translation |
| Latest items, next up, collections, playlists | `ItemService`, `CollectionService`, `PlaylistService` | Translation |
| Image parameters, collages, trickplay, chapter images, media segments, lyrics | Same semantics (Architecture §11) | Route translation |
| SyncPlay, Quick Connect | Follow Jellyfin's | Message translation |

### 3.2 Difficulties
1. **Media authorization (the main conflict).** Jellyfin clients send their token as `Authorization: MediaBrowser Token=…`, `X-Emby-Token`, `X-MediaBrowser-Token` or the `api_key` query parameter, and build media URLs themselves, such as `/Videos/{id}/stream?static=true&api_key=…`. Mavio's media URLs take no token by design; only an unguessable playback ID, issued by the authenticated `PlaybackService`, grants access, so tokens never reach URLs, logs or caches (Architecture §11). The shim would have to accept tokens in URLs on its media routes and start a playback internally for each. This exception must stay inside the shim.
2. **Events.** Jellyfin pushes session, library, user data, remote control and SyncPlay messages (34 message types) over a WebSocket; Mavio uses a Connect server stream (`EventService`). The shim needs a bridge in both directions.
3. **Discovery and version.** Clients send `who is JellyfinServer?` over UDP, which Mavio's discovery would have to answer as well. `/System/Info/Public` must report a Jellyfin version, from which clients enable features, so the shim follows Jellyfin's releases.
4. **Missing features.** Live TV, channels, the plugin catalog, branding and the startup wizard have no counterpart. The shim returns empty results or stubs, which some clients may not handle well.
5. **Jellyfin Web.** It expects the server to host it at `/web`. The shim would target native clients — Swiftfin, Findroid, Jellyfin for Android TV, Infuse, Finamp, Streamyfin — and not bundle it.

## 4. Benefits and Costs
* **Benefits**: mature clients on every platform before Mavio's own exist (P13), and playback tested on real devices through them.
* **Costs**: an untyped REST surface to maintain alongside Jellyfin's evolution; a second authorization path for media; and the positioning in Architecture §1.2 would change.

## 5. Form if Adopted
* An optional package in `apps/server` (such as `internal/jellyfin`), off by default and enabled by a flag. It calls the service implementations of `internal/rpc` in process and changes neither the domain model nor the contracts.
* First one target client (Findroid or Infuse), covering the main path of §2; further clients and endpoints only as each needs them.
* Responses are checked against Jellyfin's published OpenAPI document.

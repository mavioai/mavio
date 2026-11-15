// The fixed data of libs/design/snapshots/screens.json as the server's
// answers to the web UI.
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";

import type { MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { DiscoverService, type DiscoveredTitleSchema, MediaStatus, type TitleSchema } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { DownloadService } from "@mavio/proto/mavio/acquisition/v1/download_pb";
import { IssueService } from "@mavio/proto/mavio/acquisition/v1/issue_pb";
import { MediaKind, ProfileService } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { ReleaseService } from "@mavio/proto/mavio/acquisition/v1/release_pb";
import { RequestService, RequestStatus } from "@mavio/proto/mavio/acquisition/v1/request_pb";
import { SubtitleService } from "@mavio/proto/mavio/acquisition/v1/subtitle_pb";
import { WantedService } from "@mavio/proto/mavio/acquisition/v1/wanted_pb";
import { AssistantService } from "@mavio/proto/mavio/assistant/v1/assistant_pb";
import { AuthService } from "@mavio/proto/mavio/auth/v1/auth_pb";
import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { HubService } from "@mavio/proto/mavio/hub/v1/hub_pb";
import { CollectionService } from "@mavio/proto/mavio/library/v1/collection_pb";
import { CreditKind, ImageKind, ItemKind, ItemService, type ItemSchema } from "@mavio/proto/mavio/library/v1/item_pb";
import { HubKind, LibraryKind, LibraryService } from "@mavio/proto/mavio/library/v1/library_pb";
import { StreamKind, VideoRange } from "@mavio/proto/mavio/library/v1/media_pb";
import { MetadataService } from "@mavio/proto/mavio/library/v1/metadata_pb";
import { PlaylistService } from "@mavio/proto/mavio/library/v1/playlist_pb";
import { PlaybackService, PlayMethod, SubtitleMethod } from "@mavio/proto/mavio/playback/v1/playback_pb";
import { CatalogKind, DiscoverFilter, DiscoverSort } from "@mavio/proto/mavio/plugin/v1/discovery_pb";
import { Category } from "@mavio/proto/mavio/plugin/v1/plugin_pb";
import { EventService, SessionService } from "@mavio/proto/mavio/session/v1/session_pb";
import { type SyncPlayGroupSchema, SyncPlayService, SyncPlayState } from "@mavio/proto/mavio/session/v1/syncplay_pb";
import { StorageService } from "@mavio/proto/mavio/storage/v1/storage_pb";
import { ActivityService } from "@mavio/proto/mavio/system/v1/activity_pb";
import { AiConversationService, AiProviderService } from "@mavio/proto/mavio/system/v1/ai_pb";
import { BackupService } from "@mavio/proto/mavio/system/v1/backup_pb";
import { LocalizationService } from "@mavio/proto/mavio/system/v1/localization_pb";
import { PluginManagementService, PluginState } from "@mavio/proto/mavio/system/v1/plugin_pb";
import { ProxyService } from "@mavio/proto/mavio/system/v1/proxy_pb";
import { SystemService } from "@mavio/proto/mavio/system/v1/system_pb";
import { TaskService } from "@mavio/proto/mavio/system/v1/task_pb";
import { DisplayPreferencesService } from "@mavio/proto/mavio/user/v1/display_preferences_pb";
import { UserDataService, UserService } from "@mavio/proto/mavio/user/v1/user_pb";

import { Fixtures, origin, type Services } from "./server.ts";

interface Size {
  width: number;
  height: number;
  scale: number;
}

interface FixtureItem {
  kind: "movie" | "series" | "episode";
  name: string;
  year?: number;
  runtimeMinutes?: number;
  library: string;
  series?: string;
  season?: number;
  episode?: number;
}

interface FixtureSeason {
  series: string;
  season: number;
  name: string;
  library: string;
}

type Status = "requested" | "wanted" | "partiallyAvailable" | "available";

interface FixtureTitle {
  kind: "movie" | "series";
  tmdb: number;
  name: string;
  year: number;
  premiere: string;
  rating: number;
  votes: number;
  genres: string[];
  status?: Status;
  item?: string;
  runtimeMinutes?: number;
  tagline?: string;
  overview?: string;
  studios?: string[];
  keywords?: string[];
  credits?: { name: string; role: string; kind: "actor" | "director" | "writer" }[];
}

interface FixturePlugin {
  id: string;
  name: string;
  category: "metadata" | "subtitles" | "downloads" | "notifications" | "authentication" | "devices" | "ai";
  description: string;
  author: string;
}

/** The screens and their data, shared by every client. */
export const screens = JSON.parse(
  readFileSync(createRequire(import.meta.url).resolve("@mavio/design/snapshots/screens.json"), "utf8"),
) as {
  now: string;
  sizes: Record<"phone" | "tablet" | "desktop" | "tv", Size>;
  server: { id: string; name: string; version: string; userId: string; userName: string; admin: boolean };
  users: { id: string; name: string; admin?: boolean }[];
  libraries: { id: string; name: string; kind: "movies" | "shows" }[];
  items: Record<string, FixtureItem>;
  seasons: Record<string, FixtureSeason>;
  home: {
    kind: "continueWatching" | "recentlyAdded";
    library?: string;
    items: { id: string; progressMinutes?: number; played?: boolean }[];
  }[];
  played: string[];
  popularSearches: string[];
  search: { query: string };
  syncPlay: {
    group: {
      id: string;
      name: string;
      state: "paused";
      positionMinutes: number;
      members: { userName: string; deviceName: string }[];
      queue: string[];
      current: number;
    };
  };
  plugins: {
    catalog: (FixturePlugin & { versions: string[]; installed?: string })[];
    installed: ({ id: string; state: "ready" | "unconfigured" | "disabled" } & Partial<FixturePlugin> & { version?: string })[];
  };
  discover: {
    plugin: string;
    genres: { id: string; name: string }[];
    catalogs: {
      id: string;
      name: string;
      kind: "trending" | "popular" | "upcoming" | "recommendations";
      mediaKinds: ("movie" | "series")[];
      mixed?: boolean;
      filters?: ("genre" | "releaseDate" | "rating")[];
      sorts?: ("popularity" | "releaseDate" | "rating")[];
      titles: string[];
    }[];
    titles: Record<string, FixtureTitle>;
    requests: { id: string; title: string; seasons?: number[]; status: "pending" | "approved" | "completed"; by: string; created: string }[];
    quota: Record<"movie" | "series", { limit: number; days: number; used: number }>;
  };
};

/** The services the web UI calls. */
export const services: Services = Object.fromEntries(
  [
    AccountService,
    ActivityService,
    AiConversationService,
    AiProviderService,
    AssistantService,
    AuthService,
    BackupService,
    CollectionService,
    DiscoverService,
    DisplayPreferencesService,
    DownloadService,
    EventService,
    HubService,
    IssueService,
    ItemService,
    LibraryService,
    LocalizationService,
    MetadataService,
    PlaybackService,
    PlaylistService,
    PluginManagementService,
    ProfileService,
    ProxyService,
    ReleaseService,
    RequestService,
    SessionService,
    StorageService,
    SubtitleService,
    SyncPlayService,
    SystemService,
    TaskService,
    UserDataService,
    UserService,
    WantedService,
  ].map((s) => [s.typeName, s]),
);

const minutes = (m: number) => ({ seconds: BigInt(m * 60) });

const kinds = { movie: ItemKind.MOVIE, series: ItemKind.SERIES, episode: ItemKind.EPISODE };

function item(id: string, user: { progressMinutes?: number; played?: boolean } = {}): MessageInitShape<typeof ItemSchema> {
  const i = screens.items[id];
  const series = i.series ? screens.items[i.series] : undefined;
  const season = Object.entries(screens.seasons).find(([, s]) => s.series === i.series && s.season === i.season)?.[0];
  return {
    id,
    libraryId: i.library,
    kind: kinds[i.kind],
    name: i.name,
    productionYear: i.year ?? 0,
    runtime: i.runtimeMinutes ? minutes(i.runtimeMinutes) : undefined,
    parentId: season ?? "",
    seriesId: i.series ?? "",
    seriesName: series?.name ?? "",
    seasonId: season ?? "",
    parentIndexNumber: i.season ?? 0,
    indexNumber: i.episode ?? 0,
    images: [
      i.kind === "episode"
        ? { id: `${id}-thumb`, kind: ImageKind.PRIMARY, width: 1920, height: 1080, url: `images/${id}-thumb` }
        : { id: `${id}-poster`, kind: ImageKind.PRIMARY, width: 1000, height: 1500, url: `images/${id}-poster` },
      { id: `${id}-backdrop`, kind: ImageKind.BACKDROP, width: 1920, height: 1080, url: `images/${id}-backdrop` },
    ],
    userData: {
      itemId: id,
      played: user.played ?? screens.played.includes(id),
      position: user.progressMinutes ? minutes(user.progressMinutes) : undefined,
    },
  };
}

/** A season of a series. */
function season(id: string): MessageInitShape<typeof ItemSchema> {
  const s = screens.seasons[id];
  return {
    id,
    libraryId: s.library,
    kind: ItemKind.SEASON,
    name: s.name,
    parentId: s.series,
    seriesId: s.series,
    seriesName: screens.items[s.series].name,
    indexNumber: s.season,
    images: [{ id: `${id}-poster`, kind: ImageKind.PRIMARY, width: 1000, height: 1500, url: `images/${id}-poster` }],
  };
}

/** What the user has watched of an item, as Home shows it. */
function watched(id: string) {
  return screens.home.flatMap((h) => h.items).find((i) => i.id === id);
}

/** An item's one file: 1080p HEVC in HDR10 with 5.1 sound and subtitles. */
function source(id: string) {
  const i = screens.items[id];
  return {
    id: `${id}-source`,
    itemId: id,
    path: `/media/${i.name}/${i.name}.mkv`,
    container: "mkv",
    sizeBytes: 8_400_000_000n,
    streams: [
      { index: 0, kind: StreamKind.VIDEO, codec: "hevc", width: 1920, height: 1080, range: VideoRange.HDR10 },
      { index: 1, kind: StreamKind.AUDIO, codec: "eac3", language: "eng", channels: 6, default: true },
      { index: 2, kind: StreamKind.SUBTITLE, codec: "subrip", language: "eng" },
    ],
  };
}

const mediaKinds = { movie: MediaKind.MOVIE, series: MediaKind.SERIES };

const statuses: Record<Status, MediaStatus> = {
  requested: MediaStatus.REQUESTED,
  wanted: MediaStatus.WANTED,
  partiallyAvailable: MediaStatus.PARTIALLY_AVAILABLE,
  available: MediaStatus.AVAILABLE,
};

/** Generated artwork of a discovered title, served by the fake server as the library's. */
const remoteImage = (id: string) => `${origin}/images/${id}`;

/** A discovered title as a catalog lists it. */
function discovered(id: string): MessageInitShape<typeof DiscoveredTitleSchema> {
  const d = screens.discover.titles[id];
  const status = d.status ? statuses[d.status] : MediaStatus.UNSPECIFIED;
  return {
    mediaKind: mediaKinds[d.kind],
    name: d.name,
    originalTitle: d.name,
    originalLanguage: "en",
    overview: d.overview ?? "",
    premiereDate: d.premiere,
    year: d.year,
    externalIds: { tmdb: String(d.tmdb) },
    genres: d.genres.map((g) => screens.discover.genres.find((x) => x.name === g)?.id ?? ""),
    posterUrl: remoteImage(`title-${id}-poster`),
    backdropUrl: remoteImage(`title-${id}-backdrop`),
    communityRating: d.rating,
    voteCount: d.votes,
    status,
    itemId: d.item ?? "",
    requestable: status === MediaStatus.UNSPECIFIED,
  };
}

const creditKinds = { actor: CreditKind.ACTOR, director: CreditKind.DIRECTOR, writer: CreditKind.WRITER };

/** A discovered title as its page shows it. */
function title(id: string): MessageInitShape<typeof TitleSchema> {
  const d = screens.discover.titles[id];
  const status = d.status ? statuses[d.status] : MediaStatus.UNSPECIFIED;
  return {
    mediaKind: mediaKinds[d.kind],
    tmdbId: d.tmdb,
    name: d.name,
    originalTitle: d.name,
    overview: d.overview ?? "",
    premiereDate: d.premiere,
    theatricalReleaseDate: d.kind === "movie" ? d.premiere : "",
    year: d.year,
    officialRating: "PG-13",
    ratingCountry: "US",
    genres: d.genres,
    genreRefs: d.genres.map((g) => ({ id: screens.discover.genres.find((x) => x.name === g)?.id ?? "", name: g })),
    communityRating: d.rating,
    voteCount: d.votes,
    externalIds: { tmdb: String(d.tmdb) },
    posterUrl: remoteImage(`title-${id}-poster`),
    backdropUrl: remoteImage(`title-${id}-backdrop`),
    status,
    itemId: d.item ?? "",
    requestable: status === MediaStatus.UNSPECIFIED,
    originalLanguage: "en",
    studios: d.studios ?? [],
    productionCountries: ["US"],
    productionLocations: ["United States of America"],
    runtimeMinutes: d.runtimeMinutes ?? 0,
    tagline: d.tagline ?? "",
    keywords: d.keywords ?? [],
    credits: (d.credits ?? []).map((c, i) => ({
      name: c.name,
      kind: creditKinds[c.kind],
      role: c.role,
      order: i,
      imageUrl: remoteImage(`person-${i}`),
      externalIds: { tmdb: String(9000 + i) },
    })),
    seasons:
      d.kind === "series"
        ? [1, 2].map((n) => ({ seasonNumber: n, episodeCount: 8, premiereDate: `${d.year + n - 1}-01-09`, status, requestable: status === MediaStatus.UNSPECIFIED }))
        : [],
  };
}

const catalogKinds = {
  trending: CatalogKind.TRENDING,
  popular: CatalogKind.POPULAR,
  upcoming: CatalogKind.UPCOMING,
  recommendations: CatalogKind.RECOMMENDATIONS,
};
const filters = { genre: DiscoverFilter.GENRE, releaseDate: DiscoverFilter.RELEASE_DATE, rating: DiscoverFilter.RATING };
const sorts = { popularity: DiscoverSort.POPULARITY, releaseDate: DiscoverSort.RELEASE_DATE, rating: DiscoverSort.RATING };

const requestStatuses = { pending: RequestStatus.PENDING, approved: RequestStatus.APPROVED, completed: RequestStatus.COMPLETED };

const categories = {
  metadata: Category.METADATA,
  subtitles: Category.SUBTITLES,
  downloads: Category.DOWNLOADS,
  notifications: Category.NOTIFICATIONS,
  authentication: Category.AUTHENTICATION,
  devices: Category.DEVICES,
  ai: Category.AI,
};
const pluginStates = { ready: PluginState.READY, unconfigured: PluginState.UNCONFIGURED, disabled: PluginState.DISABLED };

/** The Watch Together group the user joins, paused at its first entry. */
function group(): MessageInitShape<typeof SyncPlayGroupSchema> {
  const g = screens.syncPlay.group;
  return {
    id: g.id,
    name: g.name,
    state: SyncPlayState.PAUSED,
    members: g.members.map((m, i) => ({ sessionId: `session-${i}`, userName: m.userName, deviceName: m.deviceName })),
    queue: g.queue.map((itemId, i) => ({ id: `entry-${i}`, itemId })),
    currentIndex: g.current,
    position: minutes(g.positionMinutes),
    positionTime: timestampFromDate(new Date(screens.now)),
  };
}

/** The server of the screens, as the web UI keeps it once signed in. */
export const signedIn = [
  {
    id: screens.server.id,
    name: screens.server.name,
    version: screens.server.version,
    url: origin,
    userId: screens.server.userId,
    userName: screens.server.userName,
    token: "snapshot",
  },
];

/** The answers of the server of the screens. */
export function fixtures(): Fixtures {
  const s = screens.server;
  const d = screens.discover;
  const library = (id?: string) => screens.libraries.find((l) => l.id === id);
  const user = (name: string) => screens.users.find((u) => u.name === name);
  return (
    new Fixtures()
      .on(AuthService, "getAuthInfo", () => ({ serverId: s.id, serverName: s.name, version: s.version }))
      .on(UserService, "getCurrentUser", () => ({ user: { id: s.userId, name: s.userName, admin: s.admin } }))
      .on(UserService, "listUsers", () => ({ users: screens.users.map((u) => ({ id: u.id, name: u.name, admin: u.admin ?? false })) }))
      .on(LibraryService, "listLibraries", () => ({
        libraries: screens.libraries.map((l) => ({
          id: l.id,
          name: l.name,
          kind: l.kind === "movies" ? LibraryKind.MOVIES : LibraryKind.SHOWS,
        })),
      }))
      .on(HubService, "listHubs", () => ({
        hubs: screens.home.map((h) => ({
          id: h.library ? `${h.kind}/${h.library}` : h.kind,
          kind: h.kind === "continueWatching" ? HubKind.CONTINUE_WATCHING : HubKind.RECENTLY_ADDED,
          libraryId: h.library ?? "",
          libraryName: library(h.library)?.name ?? "",
          items: h.items.map((i) => item(i.id, i)),
        })),
      }))
      .on(LibraryService, "getLibrary", (r) => {
        const l = library(r.id);
        return l ? { library: { id: l.id, name: l.name, kind: l.kind === "movies" ? LibraryKind.MOVIES : LibraryKind.SHOWS } } : {};
      })
      .on(ItemService, "getItem", (r) => {
        const id = r.id;
        if (screens.seasons[id]) return { item: season(id) };
        const i = screens.items[id];
        if (!i) return {};
        return { item: item(id, watched(id)), mediaSources: i.kind === "series" ? [] : [source(id)] };
      })
      .on(ItemService, "listItems", (r) => {
        // A series' seasons, its episodes, a season's episodes, the items
        // whose names hold a search's text, or a library's movies and series.
        if (r.parentId && !r.recursive && screens.items[r.parentId]) {
          const ids = Object.keys(screens.seasons).filter((id) => screens.seasons[id].series === r.parentId);
          return { items: ids.map(season), total: ids.length };
        }
        const of = r.parentId ? screens.seasons[r.parentId] : undefined;
        const search = r.search.toLowerCase();
        const ids = Object.keys(screens.items).filter((id) => {
          const i = screens.items[id];
          if (of) return i.series === of.series && i.season === of.season;
          if (r.parentId) return i.series === r.parentId;
          if (search) return i.name.toLowerCase().includes(search) && (!r.kinds.length || r.kinds.includes(kinds[i.kind]));
          return r.libraryIds.includes(i.library) && i.kind !== "episode";
        });
        return { items: ids.map((id) => item(id, watched(id))), total: ids.length };
      })
      .on(ItemService, "listPopularSearches", () => ({ queries: screens.popularSearches }))
      .on(PlaybackService, "startPlayback", (r) => ({
        playbackId: "playback",
        mediaSourceId: `${r.itemId}-source`,
        method: PlayMethod.DIRECT_PLAY,
        url: `videos/${r.itemId}.mkv`,
        audioStreamIndex: 1,
        subtitleStreamIndex: -1,
        subtitles: [{ streamIndex: 2, language: "eng", method: SubtitleMethod.EXTERNAL, format: "vtt", url: `subtitles/${r.itemId}.vtt` }],
      }))
      .on(ItemService, "listSimilarItems", (r) => {
        const of = screens.items[r.id];
        const ids = Object.keys(screens.items).filter((id) => id !== r.id && screens.items[id].kind === of?.kind);
        return { items: ids.map((id) => item(id)) };
      })
      // Discover: the catalogs of the discovery plugin and their titles.
      .on(DiscoverService, "listCatalogs", () => ({
        catalogs: d.catalogs.map((c) => ({
          id: c.id,
          name: c.name,
          kind: catalogKinds[c.kind],
          mediaKinds: c.mediaKinds.map((k) => mediaKinds[k]),
          mixed: c.mixed ?? false,
          filters: (c.filters ?? []).map((f) => filters[f]),
          sorts: (c.sorts ?? []).map((f) => sorts[f]),
          pluginId: d.plugin,
        })),
      }))
      .on(DiscoverService, "discover", (r) => {
        const c = d.catalogs.find((c) => c.id === r.catalog);
        const ids = (c?.titles ?? []).filter((id) => {
          const t = d.titles[id];
          if (r.mediaKind && mediaKinds[t.kind] !== r.mediaKind) return false;
          return !r.genres.length || t.genres.some((g) => r.genres.includes(d.genres.find((x) => x.name === g)?.id ?? ""));
        });
        return { titles: ids.map(discovered), page: 1, totalPages: 1, totalResults: ids.length };
      })
      .on(DiscoverService, "searchTitles", (r) => {
        const ids = Object.keys(d.titles).filter((id) => d.titles[id].name.toLowerCase().includes(r.query.toLowerCase()));
        return { titles: ids.map(discovered), page: 1, totalPages: 1, totalResults: ids.length, pluginId: d.plugin };
      })
      .on(DiscoverService, "listFilterValues", (r) =>
        r.filter === DiscoverFilter.GENRE ? { values: d.genres.filter((g) => !r.ids.length || r.ids.includes(g.id)) } : {},
      )
      .on(DiscoverService, "getTitle", (r) => {
        const id = Object.keys(d.titles).find((id) => d.titles[id].tmdb === r.tmdbId);
        return id ? { title: title(id) } : {};
      })
      // Requests: everyone's, as the administrator sees them, and the quotas.
      .on(RequestService, "listRequests", () => ({
        requests: d.requests.map((q) => {
          const t = d.titles[q.title];
          return {
            id: q.id,
            mediaKind: mediaKinds[t.kind],
            tmdbId: t.tmdb,
            title: t.name,
            year: t.year,
            itemId: t.item ?? "",
            seasons: (q.seasons ?? []).map((n) => ({ seasonNumber: n, status: requestStatuses[q.status] })),
            status: requestStatuses[q.status],
            requestedBy: user(q.by)?.id ?? "",
            modifiedBy: q.status === "pending" ? "" : s.userId,
            createTime: timestampFromDate(new Date(q.created)),
            updateTime: timestampFromDate(new Date(q.created)),
          };
        }),
        total: d.requests.length,
      }))
      .on(RequestService, "getQuota", () => ({
        movie: { ...d.quota.movie, remaining: d.quota.movie.limit - d.quota.movie.used, restricted: true },
        series: { ...d.quota.series, remaining: d.quota.series.limit - d.quota.series.used, restricted: true },
      }))
      // Plugins: what the catalog offers and what the server runs.
      .on(PluginManagementService, "listCatalogPlugins", () => ({
        plugins: screens.plugins.catalog.map((p) => ({
          id: p.id,
          name: p.name,
          description: p.description,
          author: p.author,
          catalogUrl: "https://plugins.mavio.dev/catalog.json",
          versions: p.versions.map((v) => ({ version: v, releaseTime: timestampFromDate(new Date(screens.now)) })),
          installedVersion: p.installed ?? "",
          category: categories[p.category],
        })),
      }))
      .on(PluginManagementService, "listPlugins", () => ({
        plugins: screens.plugins.installed.map((i) => {
          const p = { ...screens.plugins.catalog.find((c) => c.id === i.id), ...i };
          return {
            manifest: {
              id: p.id,
              name: p.name ?? p.id,
              version: p.installed ?? p.version ?? "",
              description: p.description ?? "",
              author: p.author ?? "",
              license: "GPL-3.0",
              category: p.category ? categories[p.category] : Category.UNSPECIFIED,
            },
            state: pluginStates[i.state],
          };
        }),
      }))
      // Watch Together: the group to join.
      .on(SyncPlayService, "listGroups", () => ({ groups: [group()] }))
      .on(SyncPlayService, "joinGroup", () => ({ group: group() }))
  );
}

// The fixed data of libs/design/snapshots/screens.json as the server's
// answers to the web UI.
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";

import type { MessageInitShape } from "@bufbuild/protobuf";
import { AuthService } from "@mavio/proto/mavio/auth/v1/auth_pb";
import { HubService } from "@mavio/proto/mavio/hub/v1/hub_pb";
import { ImageKind, ItemKind, ItemService, type ItemSchema } from "@mavio/proto/mavio/library/v1/item_pb";
import { HubKind, LibraryKind, LibraryService } from "@mavio/proto/mavio/library/v1/library_pb";
import { StreamKind, VideoRange } from "@mavio/proto/mavio/library/v1/media_pb";
import { PlaybackService, PlayMethod, SubtitleMethod } from "@mavio/proto/mavio/playback/v1/playback_pb";
import { EventService, SessionService } from "@mavio/proto/mavio/session/v1/session_pb";
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

/** The screens and their data, shared by every client. */
export const screens = JSON.parse(
  readFileSync(createRequire(import.meta.url).resolve("@mavio/design/snapshots/screens.json"), "utf8"),
) as {
  now: string;
  sizes: Record<"phone" | "tablet" | "desktop" | "tv", Size>;
  server: { id: string; name: string; version: string; userId: string; userName: string };
  libraries: { id: string; name: string; kind: "movies" | "shows" }[];
  items: Record<string, FixtureItem>;
  home: {
    kind: "continueWatching" | "recentlyAdded";
    library?: string;
    items: { id: string; progressMinutes?: number; played?: boolean }[];
  }[];
};

/** The services the web UI calls. */
export const services: Services = Object.fromEntries(
  [
    AuthService,
    HubService,
    ItemService,
    LibraryService,
    EventService,
    SessionService,
    PlaybackService,
    DisplayPreferencesService,
    UserService,
    UserDataService,
  ].map((s) => [s.typeName, s]),
);

const minutes = (m: number) => ({ seconds: BigInt(m * 60) });

const kinds = { movie: ItemKind.MOVIE, series: ItemKind.SERIES, episode: ItemKind.EPISODE };

function item(id: string, user: { progressMinutes?: number; played?: boolean } = {}): MessageInitShape<typeof ItemSchema> {
  const i = screens.items[id];
  const series = i.series ? screens.items[i.series] : undefined;
  return {
    id,
    libraryId: i.library,
    kind: kinds[i.kind],
    name: i.name,
    productionYear: i.year ?? 0,
    runtime: i.runtimeMinutes ? minutes(i.runtimeMinutes) : undefined,
    seriesId: i.series ?? "",
    seriesName: series?.name ?? "",
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
      played: user.played ?? false,
      position: user.progressMinutes ? minutes(user.progressMinutes) : undefined,
    },
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
  const library = (id?: string) => screens.libraries.find((l) => l.id === id);
  return new Fixtures()
    .on(AuthService, "getAuthInfo", () => ({ serverId: s.id, serverName: s.name, version: s.version }))
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
      const id = r.id ?? "";
      const i = screens.items[id];
      if (!i) return {};
      return { item: item(id, watched(id)), mediaSources: i.kind === "series" ? [] : [source(id)] };
    })
    .on(ItemService, "listItems", (r) => {
      const ids = Object.keys(screens.items).filter((id) => {
        const i = screens.items[id];
        if (r.parentId) return i.series === r.parentId;
        return (r.libraryIds ?? []).includes(i.library) && i.kind !== "episode";
      });
      return { items: ids.map((id) => item(id, watched(id))), total: ids.length };
    })
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
      const of = screens.items[r.id ?? ""];
      const ids = Object.keys(screens.items).filter((id) => id !== r.id && screens.items[id].kind === of?.kind);
      return { items: ids.map((id) => item(id)) };
    });
}

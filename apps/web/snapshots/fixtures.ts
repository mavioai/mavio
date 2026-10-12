// The fixed data of libs/design/snapshots/screens.json as the server's
// answers to the web UI.
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";

import type { MessageInitShape } from "@bufbuild/protobuf";
import { AuthService } from "@mavio/proto/mavio/auth/v1/auth_pb";
import { HomeService } from "@mavio/proto/mavio/home/v1/home_pb";
import { ImageKind, ItemKind, ItemService, type ItemSchema } from "@mavio/proto/mavio/library/v1/item_pb";
import { HubKind, LibraryKind, LibraryService } from "@mavio/proto/mavio/library/v1/library_pb";
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
    HomeService,
    ItemService,
    LibraryService,
    EventService,
    SessionService,
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
        ? { id: `${id}-thumb`, kind: ImageKind.PRIMARY, width: 1920, height: 1080 }
        : { id: `${id}-poster`, kind: ImageKind.PRIMARY, width: 1000, height: 1500 },
      { id: `${id}-backdrop`, kind: ImageKind.BACKDROP, width: 1920, height: 1080 },
    ],
    userData: {
      itemId: id,
      played: user.played ?? false,
      position: user.progressMinutes ? minutes(user.progressMinutes) : undefined,
    },
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
    .on(HomeService, "listHubs", () => ({
      hubs: screens.home.map((h) => ({
        id: h.library ? `${h.kind}/${h.library}` : h.kind,
        kind: h.kind === "continueWatching" ? HubKind.CONTINUE_WATCHING : HubKind.RECENTLY_ADDED,
        libraryId: h.library ?? "",
        libraryName: library(h.library)?.name ?? "",
        items: h.items.map((i) => item(i.id, i)),
      })),
    }));
}

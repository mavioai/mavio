import { create } from "@bufbuild/protobuf";
import { timestampFromMs } from "@bufbuild/protobuf/wkt";
import { DiscoveredTitleSchema } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { type Hub, HubSchema } from "@mavio/proto/mavio/hub/v1/hub_pb";
import { type Item, ItemKind, ItemSchema } from "@mavio/proto/mavio/library/v1/item_pb";
import { HubKind } from "@mavio/proto/mavio/library/v1/library_pb";
import { UserDataSchema } from "@mavio/proto/mavio/user/v1/user_pb";
import { expect, test } from "vitest";

import { type Hubs, merge, type Section, type ServerHubs } from "./home.ts";

function item(id: string, kind: ItemKind, ids: Record<string, string>, playedAt?: number): Item {
  return create(ItemSchema, {
    id,
    name: id,
    kind,
    externalIds: ids,
    userData:
      playedAt === undefined ? undefined : create(UserDataSchema, { lastPlayedTime: timestampFromMs(playedAt * 1000) }),
  });
}

const movie = (id: string, tmdb: string, playedAt?: number) => item(id, ItemKind.MOVIE, { tmdb }, playedAt);
const hub = (id: string, kind: HubKind, items: Item[]): Hub => create(HubSchema, { id, kind, items });
const server = (id: string, hubs: Hubs): ServerHubs => ({ serverId: id, serverName: id.toUpperCase(), hubs });
const fresh = (...hubs: Hub[]): Hubs => ({ state: "fresh", hubs });

/** The sections as "<id>: <server>:<item>…". */
function summary(sections: Section[]): string[] {
  return sections.map((s) => {
    const c = s.content;
    const content =
      c.type === "items"
        ? c.entries.map((e) => `${e.serverId}:${e.item.id}`).join(" ")
        : c.type === "titles"
          ? c.titles.map((t) => `${t.title.name}[${t.available.map((v) => v.serverId).join(",")}]`).join(" ")
          : c.type;
    return `${s.id}: ${content}`;
  });
}

const CW = HubKind.CONTINUE_WATCHING;
const RA = HubKind.RECENTLY_ADDED;

test("Continue Watching spans servers by last activity", () => {
  const a = server(
    "a",
    fresh(
      hub("continue-watching", CW, [
        movie("m1", "1", 300),
        // A next episode: no activity of its own.
        item("e1", ItemKind.EPISODE, {}),
        movie("m2", "2", 100),
      ]),
    ),
  );
  const b = server("b", fresh(hub("continue-watching", CW, [movie("m3", "3", 200)])));
  expect(summary(merge([a, b]))).toEqual(["continue-watching: a:m1 a:e1 b:m3 a:m2"]);
});

test("a title on several servers shows once", () => {
  // Brazil (68) is on both, played last on b; Amélie (194) on both, played on neither.
  const a = server(
    "a",
    fresh(
      hub("continue-watching", CW, [movie("a-brazil", "68", 100)]),
      hub("recently-added/l", RA, [movie("a-amelie", "194"), movie("a-only", "5")]),
    ),
  );
  const b = server(
    "b",
    fresh(
      hub("continue-watching", CW, [movie("b-brazil", "68", 200)]),
      hub("recently-added/l", RA, [movie("b-amelie", "194")]),
    ),
  );
  const sections = merge([a, b]);
  expect(summary(sections)).toEqual(["continue-watching: b:b-brazil", "a/recently-added/l: a:a-amelie a:a-only"]);
  const cw = sections[0].content;
  expect(cw.type === "items" && cw.entries[0].versions).toEqual([{ serverId: "a", itemId: "a-brazil" }]);
});

test("IDs of different providers link one title", () => {
  const a = server("a", fresh(hub("recently-added/l", RA, [item("x", ItemKind.MOVIE, { imdb: "tt1" })])));
  const b = server(
    "b",
    fresh(hub("recently-added/l", RA, [item("y", ItemKind.MOVIE, { tmdb: "9", imdb: "tt1" })])),
  );
  // A movie and a series with the same TMDB number are different.
  const c = server("c", fresh(hub("recently-added/l", RA, [item("z", ItemKind.SERIES, { tmdb: "9" })])));
  expect(summary(merge([a, b, c]))).toEqual(["a/recently-added/l: a:x", "c/recently-added/l: c:z"]);
});

test("servers keep their places while loading and offline", () => {
  const sections = merge([
    server("a", { state: "loading" }),
    server("b", { state: "cached", hubs: [hub("recently-added/l", RA, [movie("m", "1")])] }),
    server("c", { state: "unavailable", reason: "refused" }),
  ]);
  expect(summary(sections)).toEqual(["a/: loading", "b/recently-added/l: b:m", "c/: unavailable"]);
  expect(sections.map((s) => s.offline)).toEqual([false, true, false]);
});

test("Trending once from the first server that discovers", () => {
  const title = (name: string, tmdb: string) =>
    create(DiscoveredTitleSchema, { name, mediaKind: MediaKind.MOVIE, externalIds: { tmdb } });
  const trending = (...titles: ReturnType<typeof title>[]) =>
    create(HubSchema, { id: "trending", kind: HubKind.TRENDING, titles });
  const a = server("a", fresh(hub("recently-added/l", RA, [movie("m", "1")])));
  const b = server("b", fresh(trending(title("Dune", "1"), title("Up", "2"))));
  const c = server("c", fresh(trending(title("Other", "3"))));
  const sections = merge([a, b, c]);
  expect(summary(sections)).toEqual(["a/recently-added/l: a:m", "trending: Dune[a] Up[]"]);
  expect(sections[1].serverId).toBe("b");
});

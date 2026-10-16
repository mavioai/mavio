import { create, toJson } from "@bufbuild/protobuf";
import { type Item, ItemSchema, ListItemsRequestSchema } from "@mavio/proto/mavio/library/v1/item_pb";
import { expect, test } from "vitest";

import { type Filter, type ItemValue, libraryItemsRequest, nextEpisode, seriesNextUp } from "./hooks.ts";

test("the next episode follows by season and number", () => {
  const ep = (id: string, season: number, episode: number) =>
    create(ItemSchema, { id, parentIndexNumber: season, indexNumber: episode });
  const episodes = [ep("s2e1", 2, 1), ep("s1e2", 1, 2), ep("s1e10", 1, 10), ep("s1e1", 1, 1)];
  const cases: [string, string | undefined][] = [
    ["s1e1", "s1e2"],
    ["s1e2", "s1e10"],
    // The last of a season leads to the next season.
    ["s1e10", "s2e1"],
    ["s2e1", undefined],
    ["missing", undefined],
  ];
  for (const [id, want] of cases) {
    expect(nextEpisode(episodes, id)?.id, id).toBe(want);
  }
});

test("a series' next episode follows what was played last", () => {
  const ep = (id: string, season: number, episode: number, u: { played?: boolean; position?: number; at?: number } = {}) =>
    create(ItemSchema, {
      id,
      parentIndexNumber: season,
      indexNumber: episode,
      userData: {
        played: u.played,
        position: u.position ? { seconds: BigInt(u.position) } : undefined,
        lastPlayedTime: u.at ? { seconds: BigInt(u.at) } : undefined,
      },
    });
  const cases: [string, Item[], string | undefined][] = [
    ["nothing played starts at the first", [ep("s1e2", 1, 2), ep("s1e1", 1, 1)], "s1e1"],
    ["specials wait", [ep("s0e1", 0, 1), ep("s1e1", 1, 1)], "s1e1"],
    ["only specials", [ep("s0e1", 0, 1)], "s0e1"],
    ["the unfinished one resumes", [ep("s1e1", 1, 1, { played: true, at: 1 }), ep("s1e2", 1, 2, { position: 60, at: 2 })], "s1e2"],
    ["the one after the last played", [ep("s1e1", 1, 1, { played: true, at: 5 }), ep("s1e2", 1, 2), ep("s2e1", 2, 1)], "s1e2"],
    [
      "played last, not furthest",
      [ep("s1e1", 1, 1, { played: true, at: 9 }), ep("s1e2", 1, 2), ep("s1e3", 1, 3, { played: true, at: 3 })],
      "s1e2",
    ],
    ["rewatching from the start", [ep("s1e1", 1, 1, { played: true, at: 1 }), ep("s1e2", 1, 2, { played: true, at: 2 })], "s1e1"],
    ["none", [], undefined],
  ];
  for (const [name, episodes, want] of cases) {
    expect(seriesNextUp(episodes)?.id, name).toBe(want);
  }
});

test("a library's grid sorts and filters", () => {
  const base = { libraryIds: ["lib"], topLevel: true, offset: 60, limit: 60 };
  const sort = [{ field: "SORT_FIELD_DATE_ADDED", descending: true }];
  const cases: [Filter, object][] = [
    ["all", {}],
    ["played", { played: true }],
    // Unplayed sends false; all leaves the field unset.
    ["unplayed", { played: false }],
    ["favorites", { favorite: true }],
  ];
  for (const [filter, want] of cases) {
    const got = toJson(ListItemsRequestSchema, libraryItemsRequest("lib", "dateAdded", filter, 60));
    expect(got, filter).toEqual({ ...base, sort, ...want });
  }
});

test("a library's grid shows a genre's, tag's or studio's items", () => {
  const cases: [ItemValue, object][] = [
    [{ kind: "genre", value: "Drama" }, { genres: ["Drama"] }],
    [{ kind: "tag", value: "space" }, { tags: ["space"] }],
    [{ kind: "studio", value: "A24" }, { studios: ["A24"] }],
  ];
  for (const [value, want] of cases) {
    const got = toJson(ListItemsRequestSchema, libraryItemsRequest("lib", "name", "all", 0, value));
    expect(got, value.kind).toEqual({ libraryIds: ["lib"], topLevel: true, offset: 0, limit: 60, sort: [{ field: "SORT_FIELD_NAME", descending: false }], ...want });
  }
});

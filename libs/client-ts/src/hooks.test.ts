import { create, toJson } from "@bufbuild/protobuf";
import { ItemSchema, ListItemsRequestSchema } from "@mavio/proto/mavio/library/v1/item_pb";
import { expect, test } from "vitest";

import { type Filter, libraryItemsRequest, nextEpisode } from "./hooks.ts";

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

import { toJson } from "@bufbuild/protobuf";
import { ListItemsRequestSchema } from "@mavio/proto/mavio/library/v1/item_pb";
import { expect, test } from "vitest";

import { type Filter, libraryItemsRequest } from "./hooks.ts";

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

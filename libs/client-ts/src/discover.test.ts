import { DiscoverSort } from "@mavio/proto/mavio/plugin/v1/discovery_pb";
import { expect, test } from "vitest";

import { type CatalogFilters, discoverRequestOf, filterCount } from "./discover.ts";

test.each<{ name: string; f: CatalogFilters; want: Record<string, unknown>; count: number }>([
  {
    name: "none",
    f: {},
    want: { sort: DiscoverSort.UNSPECIFIED, genres: [], releasedFrom: "", releasedTo: "", watchRegion: "" },
    count: 0,
  },
  {
    name: "years and named values",
    f: { sort: DiscoverSort.RATING, ascending: true, yearFrom: 1990, yearTo: 1999, keywords: [{ id: "9715", name: "superhero" }], genres: ["28"] },
    want: { sort: DiscoverSort.RATING, ascending: true, releasedFrom: "1990-01-01", releasedTo: "1999-12-31", keywords: ["9715"], genres: ["28"] },
    count: 3,
  },
  {
    name: "the watch region only with providers",
    f: { watchRegion: "US" },
    want: { watchProviders: [], watchRegion: "" },
    count: 0,
  },
  {
    name: "providers in their region",
    f: { watchProviders: ["8"], watchRegion: "US", runtimeMax: 120 },
    want: { watchProviders: ["8"], watchRegion: "US", runtimeMin: 0, runtimeMax: 120 },
    count: 2,
  },
  {
    name: "a collection, which is not a filter",
    f: { collectionId: 1166519 },
    want: { tmdbCollectionId: 1166519 },
    count: 0,
  },
])("discoverRequestOf: $name", ({ f, want, count }) => {
  expect(discoverRequestOf(f)).toMatchObject(want);
  expect(filterCount(f)).toBe(count);
});

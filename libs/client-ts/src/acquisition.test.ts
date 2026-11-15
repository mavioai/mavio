import { create } from "@bufbuild/protobuf";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { RequestSettingsSchema } from "@mavio/proto/mavio/acquisition/v1/request_pb";
import { LibraryKind } from "@mavio/proto/mavio/library/v1/library_pb";
import { expect, test } from "vitest";

import { wantedTarget } from "./acquisition.ts";

const libraries = [
  { id: "movies", kind: LibraryKind.MOVIES },
  { id: "films", kind: LibraryKind.MOVIES },
  { id: "shows", kind: LibraryKind.SHOWS },
  { id: "anime", kind: LibraryKind.SHOWS },
];
const profiles = [
  { id: "hd", kind: MediaKind.MOVIE },
  { id: "uhd", kind: MediaKind.MOVIE },
  { id: "web", kind: MediaKind.SERIES },
  { id: "bd", kind: MediaKind.SERIES },
];

test.each([
  { name: "the first of the kind without settings", kind: MediaKind.MOVIE, anime: false, settings: {}, want: { libraryId: "movies", qualityProfileId: "hd" } },
  {
    name: "the movies' settings",
    kind: MediaKind.MOVIE,
    anime: false,
    settings: { movieLibraryId: "films", movieQualityProfileId: "uhd", seriesLibraryId: "shows" },
    want: { libraryId: "films", qualityProfileId: "uhd" },
  },
  {
    name: "a deleted library's setting falls back to the first",
    kind: MediaKind.MOVIE,
    anime: false,
    settings: { movieLibraryId: "gone", movieQualityProfileId: "uhd" },
    want: { libraryId: "movies", qualityProfileId: "uhd" },
  },
  {
    name: "a series' settings",
    kind: MediaKind.SERIES,
    anime: false,
    settings: { seriesLibraryId: "anime", seriesQualityProfileId: "bd", animeLibraryId: "shows" },
    want: { libraryId: "anime", qualityProfileId: "bd" },
  },
  {
    name: "anime's own settings",
    kind: MediaKind.SERIES,
    anime: true,
    settings: { seriesLibraryId: "shows", seriesQualityProfileId: "web", animeLibraryId: "anime" },
    want: { libraryId: "anime", qualityProfileId: "web" },
  },
])("wantedTarget: $name", ({ kind, anime, settings, want }) => {
  const got = wantedTarget(kind, anime, create(RequestSettingsSchema, settings), libraries, profiles);
  expect(got).toEqual(want);
});

test("wantedTarget: nothing of the kind", () => {
  expect(wantedTarget(MediaKind.SERIES, false, undefined, libraries.slice(0, 2), profiles.slice(0, 2))).toEqual({ libraryId: "", qualityProfileId: "" });
});

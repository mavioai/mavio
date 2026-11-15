import { create } from "@bufbuild/protobuf";
import { SubtitleProfileSchema } from "@mavio/proto/mavio/acquisition/v1/subtitle_pb";
import { expect, test } from "vitest";

import { itemSubtitleProfile } from "./subtitles.ts";

const english = create(SubtitleProfileSchema, { id: "en", name: "English" });
const french = create(SubtitleProfileSchema, { id: "fr", name: "French", tag: "French" });
const profiles = [english, french];
const tags = { movieProfileTags: true, seriesProfileTags: false, noProfileTags: ["nosubs"] };

test.each([
  { name: "nothing assigned", assigned: {}, tags: [], want: { effective: undefined, byTag: false } },
  { name: "the library's", assigned: { lib: "en" }, tags: [], want: { library: english, effective: english, byTag: false } },
  { name: "its own over the library's", assigned: { lib: "en", it: "fr" }, tags: [], want: { own: french, library: english, effective: french, byTag: false } },
  { name: "a tag over its own", assigned: { it: "en" }, tags: ["french"], want: { own: english, effective: french, byTag: true } },
  { name: "a tag of its own profile", assigned: { it: "fr" }, tags: ["French"], want: { own: french, effective: french, byTag: false } },
  { name: "a no-profile tag", assigned: { it: "en" }, tags: ["NoSubs", "french"], want: { own: english, effective: undefined, byTag: true } },
  { name: "a deleted profile", assigned: { lib: "en", it: "gone" }, tags: [], want: { library: english, effective: undefined, byTag: false } },
])("itemSubtitleProfile: $name", ({ assigned, tags: itemTags, want }) => {
  const got = itemSubtitleProfile(profiles, new Map(Object.entries(assigned)), { id: "it", libraryId: "lib", tags: itemTags }, tags, false);
  expect(got).toEqual({ own: undefined, library: undefined, ...want });
});

test("itemSubtitleProfile: tags not used for series", () => {
  const got = itemSubtitleProfile(profiles, new Map([["lib", "en"]]), { id: "it", libraryId: "lib", tags: ["french", "nosubs"] }, tags, true);
  expect(got.effective).toBe(english);
});

import { create } from "@bufbuild/protobuf";
import { durationFromMs } from "@bufbuild/protobuf/wkt";
import { LyricCueSchema, LyricLineSchema } from "@mavio/proto/mavio/library/v1/item_pb";
import { expect, test } from "vitest";

import { lyricLineAt, lyricSungLength } from "./lyrics.ts";

const line = (start: number, text: string) => create(LyricLineSchema, { text, start: durationFromMs(start) });
const lines = [line(1000, "one"), line(5000, "two"), line(9000, "three")];

test.each([
  { ms: 0, want: -1 },
  { ms: 1000, want: 0 },
  { ms: 4999, want: 0 },
  { ms: 5000, want: 1 },
  { ms: 60_000, want: 2 },
])("lyricLineAt($ms)", ({ ms, want }) => {
  expect(lyricLineAt(lines, ms)).toBe(want);
});

test("lyricLineAt: unsynced", () => {
  expect(lyricLineAt([create(LyricLineSchema, { text: "plain" })], 5000)).toBe(-1);
});

const cue = (position: number, endPosition: number, start: number) => create(LyricCueSchema, { position, endPosition, start: durationFromMs(start) });
const words = create(LyricLineSchema, { text: "hello big world", start: durationFromMs(1000), cues: [cue(0, 6, 1000), cue(6, 10, 2000), cue(10, 15, 3000)] });

test.each([
  { name: "before", ms: 500, want: 0 },
  { name: "first word", ms: 1000, want: 6 },
  { name: "second word", ms: 2500, want: 10 },
  { name: "all", ms: 3000, want: 15 },
  { name: "no cues", ms: 0, want: 3, line: line(0, "one") },
])("lyricSungLength: $name", ({ ms, want, line: l }) => {
  expect(lyricSungLength(l ?? words, ms)).toBe(want);
});

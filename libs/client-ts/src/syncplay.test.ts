import { create } from "@bufbuild/protobuf";
import { durationFromMs, timestampFromMs } from "@bufbuild/protobuf/wkt";
import { SyncPlayGroupSchema, SyncPlayState } from "@mavio/proto/mavio/session/v1/syncplay_pb";
import { expect, test } from "vitest";

import { clockSample, groupPositionAt } from "./syncplay.ts";

test.each([
  // The server's clock 1 s ahead, 100 ms each way, 10 ms on the server.
  { t: [0, 1100, 1110, 210], want: { offset: 1000, rtt: 200 } },
  // The server's clock 500 ms behind, an asymmetric round trip.
  { t: [1000, 700, 700, 1400], want: { offset: -500, rtt: 400 } },
])("clockSample($t)", ({ t: [t0, t1, t2, t3], want }) => {
  expect(clockSample(t0, t1, t2, t3)).toEqual(want);
});

test.each([
  { name: "playing advances", state: SyncPlayState.PLAYING, at: 15_000, want: 65_000 },
  { name: "playing waits for its start", state: SyncPlayState.PLAYING, at: 5_000, want: 60_000 },
  { name: "paused stays", state: SyncPlayState.PAUSED, at: 15_000, want: 60_000 },
  { name: "waiting stays", state: SyncPlayState.WAITING, at: 15_000, want: 60_000 },
])("groupPositionAt: $name", ({ state, at, want }) => {
  const group = create(SyncPlayGroupSchema, { state, position: durationFromMs(60_000), positionTime: timestampFromMs(10_000) });
  expect(groupPositionAt(group, at)).toBe(want);
});

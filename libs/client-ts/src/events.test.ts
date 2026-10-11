import { create } from "@bufbuild/protobuf";
import {
  ConnectedSchema,
  type Event,
  EventSchema,
  HeartbeatSchema,
  LibraryChangedSchema,
  UserDataChangedSchema,
} from "@mavio/proto/mavio/session/v1/session_pb";
import { expect, test } from "vitest";

import { backoff, refreshes } from "./events.ts";

test("backoff doubles to 30 seconds with jitter", () => {
  const cases: [attempt: number, random: number, want: number][] = [
    [0, 0, 500],
    [0, 1, 1000],
    [3, 1, 8000],
    [5, 0.5, 22_500],
    [20, 1, 30_000],
  ];
  for (const [attempt, random, want] of cases) expect(backoff(attempt, () => random), `${attempt}`).toBe(want);
});

test("changes of items refresh the server's queries", () => {
  const cases: [Event["kind"], boolean][] = [
    [{ case: "libraryChanged", value: create(LibraryChangedSchema, { changedItemIds: ["i"] }) }, true],
    [{ case: "userDataChanged", value: create(UserDataChangedSchema) }, true],
    [{ case: "connected", value: create(ConnectedSchema, { sessionId: "s" }) }, true],
    [{ case: "heartbeat", value: create(HeartbeatSchema) }, false],
  ];
  for (const [kind, want] of cases) expect(refreshes(create(EventSchema, { kind })), kind.case).toBe(want);
});

import { create } from "@bufbuild/protobuf";
import {
  ConnectedSchema,
  type Event,
  EventSchema,
  HeartbeatSchema,
  LibraryChangedSchema,
  SessionsChangedSchema,
  UserDataChangedSchema,
} from "@mavio/proto/mavio/session/v1/session_pb";
import type { QueryKey } from "@tanstack/react-query";
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

test("events refresh the queries they touch", () => {
  const library = { case: "libraryChanged", value: create(LibraryChangedSchema, { changedItemIds: ["i"] }) } as const;
  const cases: [Event["kind"], QueryKey, boolean][] = [
    [library, ["mavio", "s", "item", "i"], true],
    [library, ["mavio", "s", "items", "l", "name", "all"], true],
    [library, ["mavio", "s", "discover", "title", 1], true],
    [library, ["mavio", "s", "discover", "ratings", 1, 155], false],
    [library, ["mavio", "s", "plugins"], false],
    [library, ["mavio", "s", "sessions"], false],
    [{ case: "userDataChanged", value: create(UserDataChangedSchema) }, ["mavio", "s", "item", "i"], false],
    [{ case: "connected", value: create(ConnectedSchema, { sessionId: "s" }) }, ["mavio", "s", "plugins"], true],
    [{ case: "sessionsChanged", value: create(SessionsChangedSchema) }, ["mavio", "s", "sessions"], true],
    [{ case: "sessionsChanged", value: create(SessionsChangedSchema) }, ["mavio", "s", "item", "i"], false],
    [{ case: "heartbeat", value: create(HeartbeatSchema) }, ["mavio", "s", "item", "i"], false],
  ];
  for (const [kind, key, want] of cases) {
    expect(refreshes(create(EventSchema, { kind }), key), `${kind.case} ${key.join("/")}`).toBe(want);
  }
});

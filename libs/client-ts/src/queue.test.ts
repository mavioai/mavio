import { expect, test } from "vitest";

import { maxQueueItems, playQueue, queueFor, queueStep, shuffledQueue } from "./queue.ts";

test.each([
  { name: "none", ids: [], start: 0, want: undefined },
  { name: "one plays alone", ids: ["a"], start: 0, want: undefined },
  { name: "from the first", ids: ["a", "b"], start: 0, want: { itemIds: ["a", "b"], index: 0 } },
  { name: "from an index", ids: ["a", "b", "c"], start: 2, want: { itemIds: ["a", "b", "c"], index: 2 } },
  { name: "out of range", ids: ["a", "b"], start: 5, want: { itemIds: ["a", "b"], index: 0 } },
  { name: "negative", ids: ["a", "b"], start: -1, want: { itemIds: ["a", "b"], index: 0 } },
  { name: "empty IDs dropped", ids: ["a", "", "b"], start: 1, want: { itemIds: ["a", "b"], index: 1 } },
  { name: "the same item twice", ids: ["a", "a"], start: 1, want: { itemIds: ["a", "a"], index: 1 } },
])("playQueue: $name", ({ ids, start, want }) => {
  expect(playQueue(ids, start)).toEqual(want);
});

test("playQueue: at most maxQueueItems", () => {
  const q = playQueue(Array.from({ length: maxQueueItems + 5 }, (_, i) => `i${i}`));
  expect(q?.itemIds.length).toBe(maxQueueItems);
});

test("shuffledQueue: every item once, from the first", () => {
  // A random of 0 swaps each item with the first: a, b, c, d → b, c, d, a.
  const q = shuffledQueue(["a", "b", "c", "d"], () => 0);
  expect(q).toEqual({ itemIds: ["b", "c", "d", "a"], index: 0 });
  const r = shuffledQueue(["a", "b", "c", "d"]);
  expect([...(r?.itemIds ?? [])].sort()).toEqual(["a", "b", "c", "d"]);
});

test("shuffledQueue: leaves its input", () => {
  const ids = ["a", "b", "c"];
  shuffledQueue(ids, () => 0);
  expect(ids).toEqual(["a", "b", "c"]);
});

test.each([
  { name: "next", index: 0, by: 1 as const, want: 1 },
  { name: "previous", index: 1, by: -1 as const, want: 0 },
  { name: "past the end", index: 2, by: 1 as const, want: undefined },
  { name: "before the first", index: 0, by: -1 as const, want: undefined },
])("queueStep: $name", ({ index, by, want }) => {
  const got = queueStep({ itemIds: ["a", "b", "c"], index }, by);
  expect(got?.index).toBe(want);
});

test("queueStep: no queue", () => {
  expect(queueStep(undefined, 1)).toBeUndefined();
});

test.each([
  { name: "a queue of the item", value: { itemIds: ["a", "b"], index: 1 }, itemId: "b", want: { itemIds: ["a", "b"], index: 1 } },
  { name: "another item playing", value: { itemIds: ["a", "b"], index: 1 }, itemId: "a", want: undefined },
  { name: "nothing", value: undefined, itemId: "a", want: undefined },
  { name: "not a queue", value: "a", itemId: "a", want: undefined },
  { name: "IDs not text", value: { itemIds: [1, 2], index: 0 }, itemId: "1", want: undefined },
  { name: "no index", value: { itemIds: ["a", "b"] }, itemId: "a", want: undefined },
  { name: "one item", value: { itemIds: ["a"], index: 0 }, itemId: "a", want: undefined },
])("queueFor: $name", ({ value, itemId, want }) => {
  expect(queueFor(value, itemId)).toEqual(want);
});

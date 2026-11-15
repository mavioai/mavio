import { expect, test } from "vitest";

import { choicesOf, moved, orderOf, toggled } from "./order.ts";

test("choicesOf: unset holds every value", () => {
  expect(choicesOf(["a", "b"], undefined)).toEqual([
    { value: "a", on: true },
    { value: "b", on: true },
  ]);
});

test("choicesOf: the order first, the rest off, one gone kept", () => {
  expect(choicesOf(["a", "b", "c"], ["c", "x"])).toEqual([
    { value: "c", on: true },
    { value: "x", on: true },
    { value: "a", on: false },
    { value: "b", on: false },
  ]);
});

test("choicesOf: an empty order holds none", () => {
  expect(orderOf(choicesOf(["a", "b"], []))).toEqual([]);
});

test("toggled and orderOf", () => {
  const choices = toggled(choicesOf(["a", "b", "c"], undefined), 1);
  expect(orderOf(choices)).toEqual(["a", "c"]);
  expect(orderOf(toggled(choices, 1))).toEqual(["a", "b", "c"]);
});

test.each([
  { i: 0, by: 1, want: ["b", "a", "c"] },
  { i: 2, by: -1, want: ["a", "c", "b"] },
  { i: 2, by: 1, want: ["a", "b", "c"] },
  { i: 0, by: -5, want: ["a", "b", "c"] },
])("moved($i, $by)", ({ i, by, want }) => {
  expect(moved(["a", "b", "c"], i, by)).toEqual(want);
});

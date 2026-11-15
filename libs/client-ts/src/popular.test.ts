import { describe, expect, it } from "vitest";

import { mergePopularSearches } from "./popular.ts";

describe("mergePopularSearches", () => {
  it("takes the servers' queries in turn, each once", () => {
    expect(mergePopularSearches([["Up", "Heat", "Lost"], ["heat", "Bluey"], []])).toEqual(["Up", "heat", "Bluey", "Lost"]);
  });
  it("collapses spacing and drops empty queries", () => {
    expect(mergePopularSearches([["  The   Matrix ", " "], ["the matrix"]])).toEqual(["The Matrix"]);
  });
  it("stops at the limit", () => {
    expect(mergePopularSearches([["a", "b", "c"], ["d", "e"]], 3)).toEqual(["a", "d", "b"]);
    expect(mergePopularSearches([])).toEqual([]);
  });
});

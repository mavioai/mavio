import { expect, test } from "vitest";

import { Sentences } from "./assistant.ts";

test("Sentences", () => {
  const s = new Sentences();
  expect(s.push("Playing Dune. It starts")).toEqual(["Playing Dune."]);
  expect(s.push(" at 1:20. Version 2.5 is")).toEqual(["It starts at 1:20."]);
  expect(s.push(" new! 好的。继续播放")).toEqual(["Version 2.5 is new!", "好的。"]);
  expect(s.end()).toEqual(["继续播放"]);
  expect(s.end()).toEqual([]);
});

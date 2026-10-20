import { afterEach, expect, test, vi } from "vitest";

import { keptSpeech, Sentences, speechFormats } from "./assistant.ts";
import type { ServerEntry, Servers } from "./servers.ts";

afterEach(() => vi.unstubAllGlobals());

test("Sentences", () => {
  const s = new Sentences();
  expect(s.push("Playing Dune. It starts")).toEqual(["Playing Dune."]);
  expect(s.push(" at 1:20. Version 2.5 is")).toEqual(["It starts at 1:20."]);
  expect(s.push(" new! 好的。继续播放")).toEqual(["Version 2.5 is new!", "好的。"]);
  expect(s.end()).toEqual(["继续播放"]);
  expect(s.end()).toEqual([]);
});

test("speechFormats are what the browser plays", () => {
  expect(speechFormats()).toEqual([]);
  // Safari before Opus.
  const plays = new Set(["audio/aac", "audio/mpeg", "audio/wav"]);
  vi.stubGlobal("document", { createElement: () => ({ canPlayType: (t: string) => (plays.has(t) ? "maybe" : "") }) });
  expect(speechFormats()).toEqual(["aac", "mp3", "wav"]);
});

test("keptSpeech names the formats played", async () => {
  const urls: string[] = [];
  vi.stubGlobal("fetch", async (url: string) => {
    urls.push(url);
    return new Response("audio");
  });
  const servers = { token: async () => undefined } as unknown as Servers;
  const server = { id: "s", url: "https://mavio.test/" } as ServerEntry;
  await keptSpeech(servers, server, "assistant/conversations/52/speech/ab/0", undefined, ["aac", "mp3"]);
  await keptSpeech(servers, server, "assistant/conversations/52/speech/ab/1", undefined, []);
  expect(urls).toEqual([
    "https://mavio.test/assistant/conversations/52/speech/ab/0?formats=aac,mp3",
    "https://mavio.test/assistant/conversations/52/speech/ab/1",
  ]);
});

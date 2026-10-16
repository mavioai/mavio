// The cases of libs/intent/testdata/cases against the WASM module in Node,
// which the crate's tests and the server's run too: the three hosts must
// agree (docs/assistant.md §4.6).
import { existsSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { IntentMatch } from "@mavio/proto/mavio/intent/v1/intent_pb";
import { describe, expect, test } from "vitest";
import { parse } from "yaml";

import { Grammar, slotOf } from "./intent.ts";

const root = fileURLToPath(new URL("../../intent/", import.meta.url));
const wasm = `${root}dist/intent.wasm`;
const built = existsSync(wasm);

interface Case {
  text: string;
  languages?: string[];
  confirmation?: boolean;
  results?: number;
  intent?: string | null;
  slots?: Record<string, string | number>;
  partial?: boolean;
}

/** Why an intent's slot is not what a case wants, if it is not. */
function slotDiff(got: ReturnType<typeof slotOf>, want: string | number): string | undefined {
  const v = got?.value;
  if (!got) return "none";
  if (typeof want === "number") {
    if ((v?.case === "number" || v?.case === "seconds" || v?.case === "ordinal") && Math.abs(v.value - want) < 1e-9) return undefined;
  } else if (v?.case === "language" || v?.case === "keyword") {
    if (v.value === want) return undefined;
  } else if (v?.case === undefined && got.text === want) return undefined;
  return `${JSON.stringify(got.text)} ${v?.case}=${v?.value}`;
}

// Missing, the module is built by nx run intent:wasm.
describe.skipIf(!built)("intent cases", async () => {
  const g = built ? await Grammar.load(readFileSync(wasm)) : undefined;
  const langs = g?.languages() ?? [];

  test("has languages", () => {
    expect(langs.length).toBeGreaterThan(0);
  });

  for (const lang of langs) {
    const cases = parse(readFileSync(`${root}testdata/cases/${lang}.yaml`, "utf8")) as Case[];
    test.each(cases)(`${lang}: $text`, (c) => {
      const intents = g!.parse({
        text: c.text,
        languages: c.languages ?? [lang],
        state: c.confirmation || c.results ? { confirmation: !!c.confirmation, results: c.results ?? 0 } : undefined,
      });
      if (c.intent == null) {
        expect(intents[0]?.name, "got = an intent, want = none").toBeUndefined();
        return;
      }
      const got = intents[0];
      expect(got?.name).toBe(c.intent);
      expect(got.match === IntentMatch.PARTIAL, "partial").toBe(!!c.partial);
      for (const [name, want] of Object.entries(c.slots ?? {})) {
        expect(slotDiff(slotOf(got, name), want), `slot ${name}: want = ${want}`).toBeUndefined();
      }
    });
  }

  test("replies and examples", () => {
    expect(g!.reply("en-US", "playing", { title: "Dune" })).toBe("Playing Dune.");
    expect(g!.reply("zh-TW", "resumed")).toBe("繼續播放。");
    expect(g!.examples("de").length).toBeGreaterThan(0);
  });
});

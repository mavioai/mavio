import { describe, expect, test } from "bun:test";
import { type Memory, rank, shown, tokenize } from "./memory.ts";

describe("tokenize", () => {
	test("splits words, and Han into pairs", () => {
		expect(tokenize("Prefers the ORIGINAL audio.")).toEqual(["prefers", "the", "original", "audio"]);
		expect(tokenize("喜欢原声")).toEqual(["喜欢", "欢原", "原声"]);
	});
});

describe("rank", () => {
	test("orders matches by BM25", () => {
		const docs = [
			{ id: "a", text: "Has a daughter called Mia." },
			{ id: "b", text: "Prefers the original audio." },
			{ id: "c", text: "Watches anime with the original Japanese audio and English subtitles." },
			{ id: "d", text: "不看恐怖片" },
		];
		expect(rank("original audio", docs, 10)).toEqual(["b", "c"]);
		expect(rank("daughter", docs, 10)).toEqual(["a"]);
		expect(rank("恐怖", docs, 10)).toEqual(["d"]);
		expect(rank("cooking", docs, 10)).toEqual([]);
	});
});

describe("shown", () => {
	test("keeps the most used within the budget", () => {
		const memory = (text: string, used: number): Memory => ({ text, created: 0, updated: 0, used, session: "" });
		const many: Record<string, Memory> = {};
		for (let i = 0; i < 200; i++) many[`m${i}`] = memory("x".repeat(100), i);
		const { ids, more } = shown(many);
		expect(more).toBe(true);
		expect(ids[0]).toBe("m199");
		expect(ids.length).toBeLessThan(200);
	});
});

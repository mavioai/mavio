// What the model remembers about an owner: memories kept per owner, given
// to the model in its instructions while they fit a budget and found with
// recall beyond it, learned with the remember and forget tools and
// consolidated from each conversation that ends.

import { randomUUID } from "node:crypto";
import { BACKGROUND_CONTEXT } from "@earendil-works/chord/context";
import { type Models, Type } from "@earendil-works/pi-ai";
import { defineDocFamily, defineExtension, defineTool, type Harness, section } from "@earendil-works/pi-durable";
import { type SessionMeta, SessionMetaDoc } from "./host.ts";

/** A memory. */
export interface Memory {
	[key: string]: string | number;
	text: string;
	created: number;
	updated: number;
	/** When it was last given to the model; 0 never. */
	used: number;
	/** The session it was learned in; empty when the owner wrote it. */
	session: string;
}

/** An owner's memories, by ID. */
export const MemoryDoc = defineDocFamily<{ memories: Record<string, Memory> }, string>({
	kind: "mavio.memories",
	version: 1,
	family: true,
	scope: "session",
	initial: () => ({ memories: {} }),
});

/** How much of the instructions memories take, in characters (about 2,000 tokens). */
const budget = 8000;

/** The most memories an owner has. */
export const maxMemories = 500;

/** tokenize splits text into words, and Han, kana and Hangul into pairs of characters. */
export function tokenize(text: string): string[] {
	const out: string[] = [];
	for (const run of text.toLowerCase().normalize("NFKC").match(/[\p{L}\p{N}]+/gu) ?? []) {
		if (/[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/u.test(run)) {
			const chars = [...run];
			if (chars.length === 1) out.push(run);
			for (let i = 0; i + 1 < chars.length; i++) out.push(chars[i] + chars[i + 1]);
		} else {
			out.push(run);
		}
	}
	return out;
}

/** rank orders documents by their Okapi BM25 score for query, best first, leaving out those that do not match. */
export function rank(query: string, docs: readonly { id: string; text: string }[], limit: number): string[] {
	const terms = [...new Set(tokenize(query))];
	if (terms.length === 0 || docs.length === 0) return [];
	const k1 = 1.2;
	const b = 0.75;
	const tokens = docs.map((d) => tokenize(d.text));
	const avg = tokens.reduce((n, t) => n + t.length, 0) / docs.length || 1;
	const df = new Map<string, number>();
	for (const t of tokens) for (const term of new Set(t)) df.set(term, (df.get(term) ?? 0) + 1);
	const scored = docs.map((d, i) => {
		const t = tokens[i] ?? [];
		let score = 0;
		for (const term of terms) {
			const f = t.filter((x) => x === term).length;
			if (f === 0) continue;
			const n = df.get(term) ?? 0;
			const idf = Math.log(1 + (docs.length - n + 0.5) / (n + 0.5));
			score += (idf * f * (k1 + 1)) / (f + k1 * (1 - b + (b * t.length) / avg));
		}
		return { id: d.id, score };
	});
	return scored
		.filter((s) => s.score > 0)
		.sort((x, y) => y.score - x.score)
		.slice(0, limit)
		.map((s) => s.id);
}

/** shown returns the memories that fit the instructions, the most used first, and whether some did not fit. */
export function shown(memories: Readonly<Record<string, Memory>>): { ids: string[]; more: boolean } {
	const all = Object.entries(memories).sort(([, a], [, b]) => b.used - a.used || b.updated - a.updated);
	const ids: string[] = [];
	let size = 0;
	for (const [id, m] of all) {
		size += m.text.length + id.length + 4;
		if (size > budget) return { ids, more: true };
		ids.push(id);
	}
	return { ids, more: false };
}

const line = (id: string, m: Memory) => `[${id}] ${m.text}`;

/** ownerOf returns the owner whose memories a session uses, if it uses them. */
const ownerOf = (meta: Readonly<SessionMeta> | null | undefined) => (meta?.memory && meta.owner !== "" ? meta.owner : undefined);

/** The memory extension: the memories section and the remember, forget and recall tools. */
export function memoryExtension(now: () => number = Date.now) {
	return defineExtension({
		name: "mavio.memory",
		sections: [
			section("memories", async (input, context) => {
				const who = ownerOf(await input.read.snapshot(SessionMetaDoc, input.conversationId, context));
				if (!who) return undefined;
				const memories = (await input.read.snapshot(MemoryDoc, who, context))?.memories ?? {};
				const { ids, more } = shown(memories);
				const lines = [
					"What you remember about the user, from earlier conversations. Use it without mentioning it unless asked; remember lasting facts and preferences the user tells you with remember, and forget what turns out wrong.",
					...ids.map((id) => line(id, memories[id] as Memory)),
				];
				if (more) lines.push("More memories are not shown here: search them with recall.");
				return lines.join("\n");
			}),
		],
		tools: [
			defineTool({
				name: "remember",
				description:
					"Remember something lasting the user told about themselves that the server does not hold, such as who they watch with or what they dislike. Not for what the server's settings or tools can set.",
				parameters: Type.Object({ text: Type.String({ description: "The fact, as a short sentence about the user." }) }),
				replay: "safe",
				async execute(args, api, context) {
					const who = ownerOf(await api.snapshot(SessionMetaDoc, api.conversationId, context));
					if (!who) return { output: [{ type: "text", text: "Memory is off." }], isError: true };
					// The ID comes from the call, so that a replayed call adds the memory once.
					const id = `m${api.conversationId}.${api.taskId}`;
					const t = now();
					await api.commit(async (tx) => {
						const doc = await tx.doc(MemoryDoc, who, who);
						if (doc.memories[id] || Object.values(doc.memories).some((m) => m.text === args.text)) return;
						if (Object.keys(doc.memories).length >= maxMemories) {
							const [oldest] = Object.entries(doc.memories).sort(([, a], [, b]) => a.used - b.used || a.updated - b.updated)[0] ?? [];
							if (oldest) delete doc.memories[oldest];
						}
						doc.memories[id] = { text: args.text, created: t, updated: t, used: t, session: String(api.conversationId) };
					}, context);
					return { output: [{ type: "text", text: `Remembered as ${id}.` }] };
				},
			}),
			defineTool({
				name: "forget",
				description: "Forget memories that are wrong or that the user asks you to forget, by their IDs in brackets.",
				parameters: Type.Object({ ids: Type.Array(Type.String()) }),
				replay: "safe",
				async execute(args, api, context) {
					const who = ownerOf(await api.snapshot(SessionMetaDoc, api.conversationId, context));
					if (!who) return { output: [{ type: "text", text: "Memory is off." }], isError: true };
					await api.commit(async (tx) => {
						const doc = await tx.doc(MemoryDoc, who, who);
						for (const id of args.ids) delete doc.memories[id];
					}, context);
					return { output: [{ type: "text", text: "Forgotten." }] };
				},
			}),
			defineTool({
				name: "recall",
				description: "Search what you remember about the user beyond the memories shown in your instructions.",
				parameters: Type.Object({ query: Type.String(), limit: Type.Optional(Type.Integer({ minimum: 1, maximum: 50 })) }),
				replay: "safe",
				async execute(args, api, context) {
					const who = ownerOf(await api.snapshot(SessionMetaDoc, api.conversationId, context));
					if (!who) return { output: [{ type: "text", text: "Memory is off." }], isError: true };
					const memories = (await api.snapshot(MemoryDoc, who, context))?.memories ?? {};
					const ids = rank(
						args.query,
						Object.entries(memories).map(([id, m]) => ({ id, text: m.text })),
						args.limit ?? 10,
					);
					const t = now();
					await api.commit(async (tx) => {
						const doc = await tx.doc(MemoryDoc, who, who);
						for (const id of ids) {
							const m = doc.memories[id];
							if (m) m.used = t;
						}
					}, context);
					const text = ids.length > 0 ? ids.map((id) => line(id, memories[id] as Memory)).join("\n") : "Nothing remembered matches.";
					return { output: [{ type: "text", text }] };
				},
			}),
		],
	});
}

/** markUsed records that the memories shown to a new conversation were used. */
export async function markUsed(harness: Harness, owner: string, now: number): Promise<void> {
	await harness.commit(async (tx) => {
		const doc = await tx.doc(MemoryDoc, owner, owner);
		for (const id of shown(doc.memories).ids) {
			const m = doc.memories[id];
			if (m) m.used = now;
		}
	}, BACKGROUND_CONTEXT);
}

/** A change consolidation proposes. */
interface Changes {
	add?: string[];
	update?: { id: string; text: string }[];
	remove?: string[];
}

const consolidation = `You maintain what an assistant of a media server remembers about its user between conversations.
Given the memories and a conversation that ended, propose changes: add lasting facts and preferences the user told about themselves that are not remembered yet; update memories the conversation corrects or makes more precise, and merge duplicates by updating one and removing the others; remove memories the conversation contradicts.
Remember only what the user said about themselves, never what the server holds (history, ratings, favorites, settings) or passing requests.
Answer with JSON only: {"add": ["..."], "update": [{"id": "...", "text": "..."}], "remove": ["..."]}. Answer {} when nothing changes.`;

/**
 * consolidate updates an owner's memories from a conversation's
 * transcript with the conversation's model.
 */
export async function consolidate(
	harness: Harness,
	models: Models,
	o: { owner: string; session: string; provider: string; model: string; transcript: string },
	now: () => number = Date.now,
): Promise<void> {
	const model = models.getModel(o.provider, o.model);
	if (!model) throw new Error(`no model ${o.provider}/${o.model}`);
	const memories = (await harness.snapshot(MemoryDoc, o.owner, BACKGROUND_CONTEXT))?.memories ?? {};
	const listed = Object.entries(memories)
		.map(([id, m]) => line(id, m))
		.join("\n");
	const answer = await models.completeSimple(model, {
		systemPrompt: consolidation,
		messages: [
			{
				role: "user",
				content: `Memories:\n${listed || "(none)"}\n\nConversation:\n${o.transcript}`,
				timestamp: now(),
			},
		],
	});
	const text = answer.content
		.filter((c) => c.type === "text")
		.map((c) => c.text)
		.join("");
	const json = text.slice(text.indexOf("{"), text.lastIndexOf("}") + 1);
	if (json === "") return;
	const changes = JSON.parse(json) as Changes;
	const t = now();
	await harness.commit(async (tx) => {
		const doc = await tx.doc(MemoryDoc, o.owner, o.owner);
		for (const id of changes.remove ?? []) delete doc.memories[id];
		for (const u of changes.update ?? []) {
			const m = doc.memories[u.id];
			if (m && typeof u.text === "string" && u.text.trim() !== "") {
				m.text = u.text.trim().slice(0, 1000);
				m.updated = t;
			}
		}
		for (const a of changes.add ?? []) {
			if (typeof a !== "string" || a.trim() === "" || Object.keys(doc.memories).length >= maxMemories) continue;
			if (Object.values(doc.memories).some((m) => m.text === a.trim())) continue;
			doc.memories[`c${randomUUID().slice(0, 8)}`] = { text: a.trim().slice(0, 1000), created: t, updated: t, used: 0, session: o.session };
		}
	}, BACKGROUND_CONTEXT);
}

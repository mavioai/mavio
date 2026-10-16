// AgentSessionService: conversations with a model kept by pi-durable, so a
// conversation and an answer in progress survive the plugin restarting.
// A conversation may offer the model the host's tools, whose calls the
// host runs, and remember what its owner tells about themselves.

import { create } from "@bufbuild/protobuf";
import { timestampFromMs } from "@bufbuild/protobuf/wkt";
import { BACKGROUND_CONTEXT, type Context, withAbortSignal } from "@earendil-works/chord/context";
import { Code, ConnectError, type ServiceImpl } from "@connectrpc/connect";
import type { AssistantMessage, Message, Models, Usage, UserMessage } from "@earendil-works/pi-ai";
import {
	AssistantEntry,
	type Conversation,
	type ConversationId,
	defineDoc,
	type Extension,
	type Harness,
	LiveDoc,
	type LiveState,
	type Registry,
	type SettledSubmissionRecord,
	UsageDoc,
	UserEntry,
} from "@earendil-works/pi-durable";
import {
	type AgentMemory,
	AgentMemorySchema,
	type AgentMessage,
	AgentMessageSchema,
	type AgentSession,
	AgentSessionSchema,
	type AgentSessionService,
	AiRole,
	type SendAgentMessageRequest,
	type SendAgentMessageResponse,
	SendAgentMessageResponseSchema,
} from "@mavio/proto/mavio/plugin/v1/ai_pb";
import { text, usage } from "./gateway.ts";
import { checkTools, type HostCalls, hostExtension, hostTools, type SessionMeta, SessionMetaDoc } from "./host.ts";
import { consolidate, markUsed, type Memory, MemoryDoc, maxMemories } from "./memory.ts";
import type { Deleted } from "./purge.ts";

/** What the plugin keeps of a session besides its conversation. */
interface SessionInfo {
	[key: string]: string | number | boolean | Record<string, string>;
	title: string;
	provider: string;
	model: string;
	created: number;
	updated: number;
	// Sessions kept before the host's tools have none of the fields below.
	owner: string;
	labels: Record<string, string>;
	/** EndAgentSession ended it. */
	ended: boolean;
	/** Its owner's memories wait to be consolidated from it. */
	consolidate: boolean;
}

/**
 * The sessions, by conversation ID. Deleting a session removes it here;
 * its conversation is purged when the plugin next starts (purge.ts).
 */
const SessionIndex = defineDoc<{ sessions: Record<string, SessionInfo> }>({
	kind: "mavio.sessions",
	version: 1,
	scope: "session",
	initial: () => ({ sessions: {} }),
});

/** The kind of the entries AddAgentMessages adds. */
const addedKind = "mavio.added";

/** The most messages GetAgentSession returns, the latest ones. */
const maxMessages = 1000;

/** The most of a conversation consolidation reads, in characters, the latest. */
const maxTranscript = 60_000;

const zero: Usage = {
	input: 0,
	output: 0,
	cacheRead: 0,
	cacheWrite: 0,
	totalTokens: 0,
	cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 },
};

function add(a: Usage, b: Usage): Usage {
	return {
		input: a.input + b.input,
		output: a.output + b.output,
		cacheRead: a.cacheRead + b.cacheRead,
		cacheWrite: a.cacheWrite + b.cacheWrite,
		totalTokens: a.totalTokens + b.totalTokens,
		cost: {
			input: a.cost.input + b.cost.input,
			output: a.cost.output + b.cost.output,
			cacheRead: a.cost.cacheRead + b.cost.cacheRead,
			cacheWrite: a.cost.cacheWrite + b.cost.cacheWrite,
			total: a.cost.total + b.cost.total,
		},
	};
}

function userText(m: UserMessage): string {
	return typeof m.content === "string"
		? m.content
		: m.content
				.filter((c) => c.type === "text")
				.map((c) => c.text)
				.join("");
}

/** A session's conversation, whose ID it is in decimal. */
function conversationId(id: string): ConversationId {
	return Number(id) as ConversationId;
}

/** message converts a model message to a conversation's message, if it shows. */
function message(m: Message | undefined): AgentMessage | undefined {
	if (m?.role === "user") {
		return create(AgentMessageSchema, { role: AiRole.USER, text: userText(m), time: timestampFromMs(m.timestamp) });
	}
	if (m?.role === "assistant" && m.stopReason === "stop") {
		const t = text(m);
		if (t !== "") return create(AgentMessageSchema, { role: AiRole.ASSISTANT, text: t, time: timestampFromMs(m.timestamp) });
	}
	return undefined;
}

/** messages returns a conversation's messages, the latest max, oldest first. */
async function messages(conversation: Conversation, max: number): Promise<AgentMessage[]> {
	const out: AgentMessage[] = [];
	let cursor;
	do {
		const page = await conversation.entries({}, 200, cursor, BACKGROUND_CONTEXT);
		for (const entry of page.items) {
			// Entries come newest first, and the messages of one entry oldest first.
			let models: readonly Message[] = [];
			if (UserEntry.is(entry) || AssistantEntry.is(entry)) models = entry.model?.slice(0, 1) ?? [];
			else if (entry.kind === addedKind) models = entry.model ?? [];
			for (const m of [...models].reverse()) {
				const shown = message(m);
				if (shown) out.push(shown);
			}
		}
		cursor = page.next;
	} while (cursor !== undefined && out.length < max);
	return out.slice(0, max).reverse();
}

/** What sessions runs with. */
export interface SessionsOptions {
	harness: Harness;
	deleted: Deleted;
	registry: Registry;
	models: Models;
	calls: HostCalls;
	memory: Extension;
	/** Code mode's extension; undefined offers none. */
	codemode?: Extension;
	now?: () => number;
	/** Receives failures of background work. */
	report?: (error: unknown) => void;
}

/** Sessions implements AgentSessionService over a harness. */
export interface Sessions {
	service: Partial<ServiceImpl<typeof AgentSessionService>>;
	/** restore installs the host's tools of the sessions kept, before the harness resumes, and consolidates the memories that wait. */
	restore(): Promise<void>;
	/** idle resolves when background work is done, for tests. */
	idle(): Promise<void>;
}

/** sessions implements AgentSessionService. */
export function sessions(o: SessionsOptions): Sessions {
	const { harness, deleted, registry, models, calls } = o;
	const now = o.now ?? Date.now;
	const report = o.report ?? ((e: unknown) => console.error("agent sessions:", e));
	const background = new Set<Promise<void>>();
	const run = (work: Promise<void>) => {
		const p = work.catch(report).finally(() => background.delete(p));
		background.add(p);
	};

	const index = async () => (await harness.snapshot(SessionIndex, BACKGROUND_CONTEXT))?.sessions ?? {};

	const find = async (id: string): Promise<{ info: SessionInfo; conversation: Conversation }> => {
		const info = (await index())[id];
		const conversation = info && (await harness.conversation(conversationId(id), BACKGROUND_CONTEXT));
		if (!info || !conversation) throw new ConnectError(`no agent session ${id}`, Code.NotFound);
		return { info, conversation };
	};

	const open = async (id: string) => {
		const found = await find(id);
		if (found.info.ended) throw new ConnectError(`agent session ${id} has ended`, Code.FailedPrecondition);
		return found;
	};

	const session = async (id: string, info: SessionInfo): Promise<AgentSession> => {
		const spent = await harness.snapshot(UsageDoc, conversationId(id), BACKGROUND_CONTEXT);
		const total = Object.values(spent?.models ?? {}).reduce<Usage>((sum, u) => add(sum, u as Usage), zero);
		return create(AgentSessionSchema, {
			id,
			title: info.title,
			provider: info.provider,
			model: info.model,
			created: timestampFromMs(info.created),
			updated: timestampFromMs(info.updated),
			usage: usage(total),
			owner: info.owner ?? "",
			labels: info.labels ?? {},
			ended: info.ended ?? false,
		});
	};

	const submit = async (req: SendAgentMessageRequest, context: Context) => {
		const { conversation } = await open(req.sessionId);
		const submission = await conversation.submit(
			{ type: "input", content: req.text, ...(req.requestId !== "" ? { requestId: req.requestId } : {}) },
			context,
		);
		return { conversation, submission };
	};

	/** answer returns the answer a submission settled with. */
	const answer = async (id: string, conversation: Conversation, settled: SettledSubmissionRecord): Promise<SendAgentMessageResponse> => {
		await touch(id);
		if (settled.type !== "input" || settled.status !== "done") {
			throw new ConnectError(`the model did not answer: ${settled.reason ?? settled.status}`, Code.Unavailable);
		}
		const entry = await conversation.commit((tx) => tx.entry(AssistantEntry, settled.answer), BACKGROUND_CONTEXT);
		const m = entry?.model?.[0];
		if (m?.role !== "assistant") throw new ConnectError("the answer is missing", Code.Internal);
		const a = m as AssistantMessage;
		return create(SendAgentMessageResponseSchema, {
			answer: create(AgentMessageSchema, { role: AiRole.ASSISTANT, text: text(a), time: timestampFromMs(a.timestamp) }),
			usage: usage(a.usage),
		});
	};

	const touch = (id: string) =>
		harness.commit(async (tx) => {
			const info = (await tx.doc(SessionIndex)).sessions[id];
			if (info) info.updated = now();
		}, BACKGROUND_CONTEXT);

	/** consolidation consolidates the owner's memories from an ended session, once. */
	const consolidation = async (id: string) => {
		const { info, conversation } = await find(id);
		const meta = await harness.snapshot(SessionMetaDoc, conversationId(id), BACKGROUND_CONTEXT);
		if (meta?.memory && meta.owner !== "") {
			const lines = (await messages(conversation, maxMessages)).map((m) => `${m.role === AiRole.USER ? "User" : "Assistant"}: ${m.text}`);
			let transcript = lines.join("\n");
			if (transcript.length > maxTranscript) transcript = transcript.slice(-maxTranscript);
			if (transcript !== "") {
				await consolidate(harness, models, { owner: meta.owner, session: id, provider: info.provider, model: info.model, transcript }, now);
			}
		}
		await harness.commit(async (tx) => {
			const kept = (await tx.doc(SessionIndex)).sessions[id];
			if (kept) kept.consolidate = false;
		}, BACKGROUND_CONTEXT);
	};

	const memory = (id: string, m: Memory): AgentMemory =>
		create(AgentMemorySchema, {
			id,
			text: m.text,
			createTime: timestampFromMs(m.created),
			updateTime: timestampFromMs(m.updated),
			...(m.used > 0 ? { useTime: timestampFromMs(m.used) } : {}),
			sessionId: m.session,
		});

	const service: Partial<ServiceImpl<typeof AgentSessionService>> = {
		async createAgentSession(req) {
			const tools = hostTools(req.tools);
			try {
				checkTools(tools);
			} catch (e) {
				throw new ConnectError(e instanceof Error ? e.message : String(e), Code.InvalidArgument);
			}
			if (req.memory && req.owner === "") throw new ConnectError("memory needs an owner", Code.InvalidArgument);
			const extensions: Extension[] = [];
			if (tools.length > 0) extensions.push(hostExtension(registry, tools, calls, now));
			if (req.memory) extensions.push(o.memory);
			if (req.codeMode) {
				if (!o.codemode) throw new ConnectError("code mode is not available", Code.FailedPrecondition);
				extensions.push(o.codemode);
			}
			const meta: SessionMeta = { owner: req.owner, memory: req.memory, codeMode: req.codeMode, tools };
			const conversation = await harness.createConversation(
				{
					ownership: { kind: "ownerless" },
					agent: {
						model: { provider: req.provider, modelId: req.model },
						extensions,
						...(req.instructions !== "" ? { instructions: req.instructions } : {}),
					},
					init: async (tx, id) => {
						Object.assign(await tx.doc(SessionMetaDoc, id), meta);
					},
				},
				BACKGROUND_CONTEXT,
			);
			const id = String(conversation.id);
			const t = now();
			const info: SessionInfo = {
				title: req.title,
				provider: req.provider,
				model: req.model,
				created: t,
				updated: t,
				owner: req.owner,
				labels: { ...req.labels },
				ended: false,
				consolidate: false,
			};
			await harness.commit(async (tx) => {
				(await tx.doc(SessionIndex)).sessions[id] = info;
			}, BACKGROUND_CONTEXT);
			if (req.memory) await markUsed(harness, req.owner, t);
			return { session: await session(id, info) };
		},

		async listAgentSessions(req) {
			const all = Object.entries(await index())
				.filter(([, info]) => req.owner === "" || info.owner === req.owner)
				.sort(([, a], [, b]) => b.updated - a.updated);
			return { sessions: await Promise.all(all.map(([id, info]) => session(id, info))) };
		},

		async getAgentSession(req) {
			const { info, conversation } = await find(req.id);
			return { session: await session(req.id, info), messages: await messages(conversation, maxMessages) };
		},

		async sendAgentMessage(req, ctx) {
			const context = withAbortSignal(ctx.signal, BACKGROUND_CONTEXT);
			const { conversation, submission } = await submit(req, context);
			return answer(req.sessionId, conversation, await submission.wait(context));
		},

		async *streamAgentMessage(req, ctx) {
			const msg = req.message;
			if (!msg) throw new ConnectError("message is required", Code.InvalidArgument);
			const context = withAbortSignal(ctx.signal, BACKGROUND_CONTEXT);
			const { conversation, submission } = await submit(msg, context);
			// The run the submission started presents its answer in progress.
			const watch = await harness.watchDoc(LiveDoc, conversationId(msg.sessionId), context);
			let shown = "";
			let pending: string | undefined;
			let wake = () => {};
			const show = (live: Readonly<LiveState> | null) => {
				const partial = live?.generation?.message;
				const t = partial ? text(partial as AssistantMessage) : "";
				if (t !== "" && t !== shown) {
					shown = pending = t;
					wake();
				}
			};
			if (watch) {
				show(watch.value);
				watch.start(async (live) => show(live));
			}
			let settled: { value: SendAgentMessageResponse } | { error: unknown } | undefined;
			submission.wait(context).then(
				async (s) => {
					try {
						settled = { value: await answer(msg.sessionId, conversation, s) };
					} catch (error) {
						settled = { error };
					}
					wake();
				},
				(error: unknown) => {
					settled = { error };
					wake();
				},
			);
			try {
				for (;;) {
					if (pending !== undefined) {
						const t = pending;
						pending = undefined;
						yield { event: { case: "text", value: t } };
						continue;
					}
					if (settled) break;
					await new Promise<void>((resolve) => {
						wake = resolve;
					});
				}
			} finally {
				await watch?.stop();
			}
			if ("error" in settled) throw settled.error;
			yield { event: { case: "done", value: settled.value } };
		},

		async deleteAgentSession(req) {
			const { conversation } = await find(req.id);
			await conversation.abort(BACKGROUND_CONTEXT);
			await harness.commit(async (tx) => {
				delete (await tx.doc(SessionIndex)).sessions[req.id];
			}, BACKGROUND_CONTEXT);
			await deleted.add(req.id);
			return {};
		},

		async addAgentMessages(req, ctx) {
			const { info, conversation } = await open(req.sessionId);
			const model = models.getModel(info.provider, info.model);
			const added: Message[] = [];
			for (const m of req.messages) {
				const timestamp = m.time ? Number(m.time.seconds) * 1000 + Math.floor(m.time.nanos / 1e6) : now();
				if (m.text === "") throw new ConnectError("a message has no text", Code.InvalidArgument);
				switch (m.role) {
					case AiRole.USER:
						added.push({ role: "user", content: m.text, timestamp });
						break;
					case AiRole.ASSISTANT:
						added.push({
							role: "assistant",
							content: [{ type: "text", text: m.text }],
							api: model?.api ?? "openai-completions",
							provider: info.provider,
							model: info.model,
							usage: zero,
							stopReason: "stop",
							timestamp,
						});
						break;
					default:
						throw new ConnectError("messages are the user's or the assistant's", Code.InvalidArgument);
				}
			}
			// A write waits for the run in progress to reach a boundary.
			await conversation.submit(
				{ type: "write", entry: { kind: addedKind, model: added }, ...(req.requestId !== "" ? { requestId: req.requestId } : {}) },
				withAbortSignal(ctx.signal, BACKGROUND_CONTEXT),
			);
			await touch(req.sessionId);
			return {};
		},

		async endAgentSession(req) {
			const { info } = await find(req.id);
			if (info.ended) return {};
			const meta = await harness.snapshot(SessionMetaDoc, conversationId(req.id), BACKGROUND_CONTEXT);
			const remember = !!meta?.memory && meta.owner !== "";
			await harness.commit(async (tx) => {
				const kept = (await tx.doc(SessionIndex)).sessions[req.id];
				if (kept) {
					kept.ended = true;
					kept.consolidate = remember;
				}
			}, BACKGROUND_CONTEXT);
			if (remember) run(consolidation(req.id));
			return {};
		},

		async *watchAgentToolCalls(_req, ctx) {
			for await (const call of calls.watch(ctx.signal)) yield { call };
		},

		async submitAgentToolResult(req) {
			calls.submit(req);
			return {};
		},

		async listAgentMemories(req) {
			const memories = (await harness.snapshot(MemoryDoc, req.owner, BACKGROUND_CONTEXT))?.memories ?? {};
			return {
				memories: Object.entries(memories)
					.sort(([, a], [, b]) => b.updated - a.updated)
					.map(([id, m]) => memory(id, m)),
			};
		},

		async putAgentMemory(req) {
			const t = now();
			const id = req.id !== "" ? req.id : `u${crypto.randomUUID().slice(0, 8)}`;
			const m = await harness.commit(async (tx) => {
				const doc = await tx.doc(MemoryDoc, req.owner, req.owner);
				const old = doc.memories[id];
				if (req.id !== "" && !old) return undefined;
				if (!old && Object.keys(doc.memories).length >= maxMemories) return null;
				const next: Memory = old ? { ...old, text: req.text, updated: t } : { text: req.text, created: t, updated: t, used: 0, session: "" };
				doc.memories[id] = next;
				return next;
			}, BACKGROUND_CONTEXT);
			if (m === undefined) throw new ConnectError(`no memory ${req.id}`, Code.NotFound);
			if (m === null) throw new ConnectError(`at most ${maxMemories} memories are kept`, Code.ResourceExhausted);
			return { memory: memory(id, m) };
		},

		async deleteAgentMemories(req) {
			await harness.commit(async (tx) => {
				const doc = await tx.doc(MemoryDoc, req.owner, req.owner);
				if (req.ids.length === 0) doc.memories = {};
				for (const id of req.ids) delete doc.memories[id];
			}, BACKGROUND_CONTEXT);
			return {};
		},
	};

	return {
		service,
		async restore() {
			for (const [id, info] of Object.entries(await index())) {
				const meta = await harness.snapshot(SessionMetaDoc, conversationId(id), BACKGROUND_CONTEXT);
				if (meta && meta.tools.length > 0) hostExtension(registry, meta.tools, calls, now);
				if (info.consolidate) run(consolidation(id));
			}
		},
		async idle() {
			while (background.size > 0) await Promise.all(background);
		},
	};
}

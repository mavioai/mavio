// AgentSessionService: conversations with a model kept by pi-durable, so a
// conversation and an answer in progress survive the plugin restarting.

import { create } from "@bufbuild/protobuf";
import { timestampFromMs } from "@bufbuild/protobuf/wkt";
import { BACKGROUND_CONTEXT, withAbortSignal } from "@earendil-works/chord/context";
import { Code, ConnectError, type ServiceImpl } from "@connectrpc/connect";
import type { AssistantMessage, Usage, UserMessage } from "@earendil-works/pi-ai";
import { AssistantEntry, type Conversation, type ConversationId, defineDoc, type Harness, UsageDoc, UserEntry } from "@earendil-works/pi-durable";
import {
	type AgentMessage,
	AgentMessageSchema,
	type AgentSession,
	AgentSessionSchema,
	type AgentSessionService,
	AiRole,
} from "@mavio/proto/mavio/plugin/v1/ai_pb";
import { text, usage } from "./gateway.ts";

/** What the plugin keeps of a session besides its conversation. */
interface SessionInfo {
	[key: string]: string | number;
	title: string;
	provider: string;
	model: string;
	created: number;
	updated: number;
}

/**
 * The sessions, by conversation ID. Deleting a session removes it here;
 * pi-durable keeps its entries.
 */
const SessionIndex = defineDoc<{ sessions: Record<string, SessionInfo> }>({
	kind: "mavio.sessions",
	version: 1,
	scope: "session",
	initial: () => ({ sessions: {} }),
});

/** The most messages GetAgentSession returns, the latest ones. */
const maxMessages = 1000;

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

/** sessions implements AgentSessionService over a harness. */
export function sessions(harness: Harness, now: () => number = Date.now): Partial<ServiceImpl<typeof AgentSessionService>> {
	const index = async () => (await harness.snapshot(SessionIndex, BACKGROUND_CONTEXT))?.sessions ?? {};

	const find = async (id: string): Promise<{ info: SessionInfo; conversation: Conversation }> => {
		const info = (await index())[id];
		const conversation = info && (await harness.conversation(conversationId(id), BACKGROUND_CONTEXT));
		if (!info || !conversation) throw new ConnectError(`no agent session ${id}`, Code.NotFound);
		return { info, conversation };
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
		});
	};

	const touch = (id: string) =>
		harness.commit(async (tx) => {
			const info = (await tx.doc(SessionIndex)).sessions[id];
			if (info) info.updated = now();
		}, BACKGROUND_CONTEXT);

	return {
		async createAgentSession(req) {
			const conversation = await harness.createConversation(
				{
					ownership: { kind: "ownerless" },
					agent: {
						model: { provider: req.provider, modelId: req.model },
						...(req.instructions !== "" ? { instructions: req.instructions } : {}),
					},
				},
				BACKGROUND_CONTEXT,
			);
			const id = String(conversation.id);
			const t = now();
			const info: SessionInfo = { title: req.title, provider: req.provider, model: req.model, created: t, updated: t };
			await harness.commit(async (tx) => {
				(await tx.doc(SessionIndex)).sessions[id] = info;
			}, BACKGROUND_CONTEXT);
			return { session: await session(id, info) };
		},

		async listAgentSessions() {
			const all = Object.entries(await index()).sort(([, a], [, b]) => b.updated - a.updated);
			return { sessions: await Promise.all(all.map(([id, info]) => session(id, info))) };
		},

		async getAgentSession(req) {
			const { info, conversation } = await find(req.id);
			const messages: AgentMessage[] = [];
			let cursor;
			do {
				const page = await conversation.entries({}, 200, cursor, BACKGROUND_CONTEXT);
				for (const entry of page.items) {
					const m = entry.model?.[0];
					if (UserEntry.is(entry) && m?.role === "user") {
						messages.push(create(AgentMessageSchema, { role: AiRole.USER, text: userText(m), time: timestampFromMs(m.timestamp) }));
					} else if (AssistantEntry.is(entry) && m?.role === "assistant" && m.stopReason === "stop") {
						const t = text(m as AssistantMessage);
						if (t !== "") messages.push(create(AgentMessageSchema, { role: AiRole.ASSISTANT, text: t, time: timestampFromMs(m.timestamp) }));
					}
				}
				cursor = page.next;
			} while (cursor !== undefined && messages.length < maxMessages);
			// Entries come newest first.
			return { session: await session(req.id, info), messages: messages.slice(0, maxMessages).reverse() };
		},

		async sendAgentMessage(req, ctx) {
			const { conversation } = await find(req.sessionId);
			const context = withAbortSignal(ctx.signal, BACKGROUND_CONTEXT);
			const submission = await conversation.submit(
				{ type: "input", content: req.text, ...(req.requestId !== "" ? { requestId: req.requestId } : {}) },
				context,
			);
			const settled = await submission.wait(context);
			await touch(req.sessionId);
			if (settled.type !== "input" || settled.status !== "done") {
				throw new ConnectError(`the model did not answer: ${settled.reason ?? settled.status}`, Code.Unavailable);
			}
			const entry = await conversation.commit((tx) => tx.entry(AssistantEntry, settled.answer), BACKGROUND_CONTEXT);
			const m = entry?.model?.[0];
			if (m?.role !== "assistant") throw new ConnectError("the answer is missing", Code.Internal);
			const answer = m as AssistantMessage;
			return {
				answer: create(AgentMessageSchema, { role: AiRole.ASSISTANT, text: text(answer), time: timestampFromMs(answer.timestamp) }),
				usage: usage(answer.usage),
			};
		},

		async deleteAgentSession(req) {
			const { conversation } = await find(req.id);
			await conversation.abort(BACKGROUND_CONTEXT);
			await harness.commit(async (tx) => {
				delete (await tx.doc(SessionIndex)).sessions[req.id];
			}, BACKGROUND_CONTEXT);
			return {};
		},
	};
}

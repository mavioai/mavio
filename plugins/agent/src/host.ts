// The host's tools: tools a session's creator declares as data, whose
// calls the host runs. A call waits in the plugin until the host answers it
// through SubmitAgentToolResult; the tools are safe to replay, so a call
// interrupted by a restart is handed to the host again with the same ID,
// and the host answers a repeated call without running it again.

import { createHash } from "node:crypto";
import { create } from "@bufbuild/protobuf";
import { timestampFromMs } from "@bufbuild/protobuf/wkt";
import type { Context } from "@earendil-works/chord/context";
import type { TSchema } from "@earendil-works/pi-ai";
import { defineDoc, defineExtension, defineTool, type Extension, type Registry, type ToolRegistration } from "@earendil-works/pi-durable";
import {
	type AgentTool,
	type AgentToolCall,
	AgentToolCallSchema,
	type SubmitAgentToolResultRequest,
} from "@mavio/proto/mavio/plugin/v1/ai_pb";

/** A tool of the host, as the session keeps it. */
export interface HostTool {
	[key: string]: string;
	name: string;
	description: string;
	parameters: string;
	result: string;
}

/** What a session's conversation keeps of its creator's request. */
export interface SessionMeta {
	[key: string]: string | boolean | HostTool[];
	owner: string;
	memory: boolean;
	codeMode: boolean;
	tools: HostTool[];
}

/** A session's settings, written when it is created. */
export const SessionMetaDoc = defineDoc<SessionMeta>({
	kind: "mavio.session",
	version: 1,
	scope: "conversation",
	history: "latest",
	fork: "current",
	initial: () => ({ owner: "", memory: false, codeMode: false, tools: [] }),
});

/** How long a call waits for the host before it fails. */
const callTimeout = 15 * 60 * 1000;

/** How long an answer to a call not waiting is kept for it. */
const earlyTTL = 60 * 60 * 1000;

interface Waiting {
	call: AgentToolCall;
	answer(result: SubmitAgentToolResultRequest): void;
}

/** HostCalls are the calls of the host's tools waiting for the host. */
export class HostCalls {
	private readonly waiting = new Map<string, Waiting>();
	/** Answers that came before their call: after a restart, the host may answer before the call runs again. */
	private readonly early = new Map<string, { result: SubmitAgentToolResultRequest; at: number }>();
	private readonly watchers = new Set<(call: AgentToolCall) => void>();

	constructor(private readonly now: () => number = Date.now) {}

	/** wait hands a call to the host and returns its answer. */
	wait(call: AgentToolCall, context: Context): Promise<SubmitAgentToolResultRequest> {
		const early = this.early.get(call.id);
		if (early) {
			this.early.delete(call.id);
			return Promise.resolve(early.result);
		}
		return new Promise((resolve, reject) => {
			const signal = context.abortSignal;
			const done = () => {
				clearTimeout(timer);
				signal?.removeEventListener("abort", aborted);
				this.waiting.delete(call.id);
			};
			const aborted = () => {
				done();
				reject(signal?.reason ?? new Error("aborted"));
			};
			const timer = setTimeout(() => {
				done();
				reject(new Error("the host did not answer the call"));
			}, callTimeout);
			if (signal?.aborted) return aborted();
			signal?.addEventListener("abort", aborted, { once: true });
			this.waiting.set(call.id, {
				call,
				answer: (result) => {
					done();
					resolve(result);
				},
			});
			for (const w of this.watchers) w(call);
		});
	}

	/** submit answers a call; an answer to a call not waiting is kept for it a while. */
	submit(result: SubmitAgentToolResultRequest): void {
		const w = this.waiting.get(result.callId);
		if (w) return w.answer(result);
		const now = this.now();
		for (const [id, e] of this.early) if (now - e.at > earlyTTL) this.early.delete(id);
		this.early.set(result.callId, { result, at: now });
	}

	/** watch yields the calls waiting, then each new one, until signal aborts. */
	async *watch(signal: AbortSignal): AsyncGenerator<AgentToolCall> {
		const queue: AgentToolCall[] = [...this.waiting.values()].map((w) => w.call);
		let wake = () => {};
		const watcher = (call: AgentToolCall) => {
			queue.push(call);
			wake();
		};
		const stop = () => wake();
		this.watchers.add(watcher);
		signal.addEventListener("abort", stop, { once: true });
		try {
			while (!signal.aborted) {
				const call = queue.shift();
				if (call) {
					yield call;
					continue;
				}
				await new Promise<void>((resolve) => {
					wake = resolve;
				});
			}
		} finally {
			this.watchers.delete(watcher);
			signal.removeEventListener("abort", stop);
		}
	}
}

/** hostTools converts the declarations of a request. */
export function hostTools(tools: readonly AgentTool[]): HostTool[] {
	return tools.map((t) => ({ name: t.name, description: t.description, parameters: t.parametersJson, result: t.resultJson }));
}

function schema(json: string, what: string): TSchema {
	let s: unknown;
	try {
		s = JSON.parse(json);
	} catch (e) {
		throw new Error(`${what} is not JSON: ${e instanceof Error ? e.message : String(e)}`);
	}
	if (typeof s !== "object" || s === null || Array.isArray(s)) throw new Error(`${what} is not a JSON Schema object`);
	return s as TSchema;
}

/** checkTools validates declarations before a session is created with them. */
export function checkTools(tools: readonly HostTool[]): void {
	const names = new Set<string>();
	for (const t of tools) {
		if (names.has(t.name)) throw new Error(`tool ${t.name} is declared twice`);
		names.add(t.name);
		schema(t.parameters, `the parameters of ${t.name}`);
		if (t.result !== "") schema(t.result, `the result of ${t.name}`);
	}
}

function hostTool(t: HostTool, calls: HostCalls, now: () => number): ToolRegistration {
	return defineTool({
		name: t.name,
		description: t.description,
		parameters: schema(t.parameters, `the parameters of ${t.name}`),
		...(t.result !== "" ? { structuredOutputSchema: schema(t.result, `the result of ${t.name}`) } : {}),
		replay: "safe",
		async execute(args, api, context) {
			const meta = await api.snapshot(SessionMetaDoc, api.conversationId, context);
			// The call keeps its creation time across restarts, as its ID.
			const created = await api.memo("created", now(), context);
			const call = create(AgentToolCallSchema, {
				id: `${api.conversationId}.${api.taskId}`,
				sessionId: String(api.conversationId),
				owner: meta?.owner ?? "",
				tool: t.name,
				argumentsJson: JSON.stringify(args ?? {}),
				createTime: timestampFromMs(created),
			});
			const result = await calls.wait(call, context);
			let structured: unknown;
			if (result.resultJson !== "" && t.result !== "" && !result.failed) {
				try {
					structured = JSON.parse(result.resultJson);
				} catch {
					// The text alone reaches the model.
				}
			}
			return {
				output: [{ type: "text", text: result.text !== "" ? result.text : result.resultJson }],
				...(structured !== undefined ? { structuredOutput: structured as never } : {}),
				...(result.failed ? { isError: true } : {}),
			};
		},
	});
}

/**
 * hostExtension returns the extension of a set of the host's tools,
 * installing it once: sessions declaring the same tools share it.
 */
export function hostExtension(registry: Registry, tools: readonly HostTool[], calls: HostCalls, now: () => number = Date.now): Extension {
	const name = `mavio.host.${createHash("sha256").update(JSON.stringify(tools)).digest("hex").slice(0, 16)}`;
	const installed = registry.snapshot().extension(name);
	if (installed) return installed;
	const extension = defineExtension({ name, tools: tools.map((t) => hostTool(t, calls, now)) });
	registry.install(extension);
	return extension;
}

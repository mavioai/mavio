// AiGatewayService: the AI providers the plugin reaches through pi-ai, their
// credentials and logins, and completions for the server's features.

import { create, isFieldSet } from "@bufbuild/protobuf";
import { Code, ConnectError, type ServiceImpl } from "@connectrpc/connect";
import {
	type Api,
	type AssistantMessage,
	type AuthEvent,
	type AuthPrompt,
	type Context,
	type CredentialStore,
	isContextOverflow,
	isRetryableAssistantError,
	type Message,
	type Model,
	type MutableModels,
	type Provider,
	type ThinkingLevel,
	type Tool,
	type TSchema,
	type Usage,
} from "@earendil-works/pi-ai";
import {
	type AiGatewayService,
	AiCredential,
	type AiLoginEvent,
	AiLoginEventSchema,
	AiLoginPromptKind,
	AiLoginPromptSchema,
	AiLoginState,
	type AiModel,
	AiModelSchema,
	type AiProvider,
	AiProviderSchema,
	AiRole,
	AiThinking,
	type AiUsage,
	AiUsageSchema,
	type CompleteAiRequest,
	CompleteAiRequestSchema,
	CompleteAiResponseSchema,
} from "@mavio/proto/mavio/plugin/v1/ai_pb";
import type { Logins, LoginState, LoginView } from "./logins.ts";

/** The tool a model answers through when the request has an output schema. */
export const outputTool = "respond";

// Unset and OFF ask for no reasoning.
const thinkingLevels: Partial<Record<AiThinking, ThinkingLevel>> = {
	[AiThinking.MINIMAL]: "minimal",
	[AiThinking.LOW]: "low",
	[AiThinking.MEDIUM]: "medium",
	[AiThinking.HIGH]: "high",
};

const loginStates: Record<LoginState, AiLoginState> = {
	running: AiLoginState.RUNNING,
	succeeded: AiLoginState.SUCCEEDED,
	failed: AiLoginState.FAILED,
	canceled: AiLoginState.CANCELED,
};

const promptKinds: Record<AuthPrompt["type"], AiLoginPromptKind> = {
	text: AiLoginPromptKind.TEXT,
	secret: AiLoginPromptKind.SECRET,
	select: AiLoginPromptKind.SELECT,
	manual_code: AiLoginPromptKind.MANUAL_CODE,
};

/** usage converts pi-ai's usage. */
export function usage(u: Usage): AiUsage {
	return create(AiUsageSchema, {
		inputTokens: BigInt(u.input),
		outputTokens: BigInt(u.output),
		cacheReadTokens: BigInt(u.cacheRead),
		cacheWriteTokens: BigInt(u.cacheWrite),
		cost: u.cost.total,
	});
}

function model(m: Model<Api>): AiModel {
	return create(AiModelSchema, {
		provider: m.provider,
		id: m.id,
		name: m.name,
		reasoning: m.reasoning,
		images: m.input.includes("image"),
		contextWindow: BigInt(m.contextWindow),
		maxOutputTokens: BigInt(m.maxTokens),
		cost: { input: m.cost.input, output: m.cost.output, cacheRead: m.cost.cacheRead, cacheWrite: m.cost.cacheWrite },
	});
}

function event(e: AuthEvent): AiLoginEvent {
	switch (e.type) {
		case "info":
			return create(AiLoginEventSchema, {
				event: { case: "info", value: { message: e.message, links: (e.links ?? []).map((l) => ({ url: l.url, label: l.label ?? "" })) } },
			});
		case "auth_url":
			return create(AiLoginEventSchema, { event: { case: "url", value: { url: e.url, instructions: e.instructions ?? "" } } });
		case "device_code":
			return create(AiLoginEventSchema, {
				event: {
					case: "deviceCode",
					value: { userCode: e.userCode, verificationUri: e.verificationUri, expiresInSeconds: e.expiresInSeconds ?? 0 },
				},
			});
		case "progress":
			return create(AiLoginEventSchema, { event: { case: "progress", value: e.message } });
	}
}

/** failure converts a failed completion into the error the server retries or reports. */
export function failure(m: AssistantMessage): ConnectError {
	const message = m.errorMessage || `the model stopped: ${m.stopReason}`;
	if (m.stopReason === "aborted") return new ConnectError(message, Code.Canceled);
	if (isContextOverflow(m)) return new ConnectError(message, Code.InvalidArgument);
	if (isRetryableAssistantError(m)) return new ConnectError(message, Code.Unavailable);
	return new ConnectError(message, Code.Unknown);
}

/** A message's text, without its thinking and tool calls. */
export function text(m: AssistantMessage): string {
	return m.content
		.filter((c) => c.type === "text")
		.map((c) => c.text)
		.join("");
}

/** The parts of the gateway the plugin's state supplies. */
export interface GatewayOptions {
	models: MutableModels;
	credentials: CredentialStore;
	logins: Logins;
	/** The IDs of the providers the administrator added. */
	custom: () => ReadonlySet<string>;
}

/** gateway implements AiGatewayService. */
export function gateway(o: GatewayOptions): Partial<ServiceImpl<typeof AiGatewayService>> {
	const provider = (id: string): Provider => {
		const p = o.models.getProvider(id);
		if (!p) throw new ConnectError(`no AI provider ${id}`, Code.NotFound);
		return p;
	};
	return {
		async listAiProviders(_req, ctx) {
			const custom = o.custom();
			const providers: AiProvider[] = [];
			for (const p of o.models.getProviders()) {
				const [stored, check] = await Promise.all([
					o.credentials.read(p.id, { signal: ctx.signal }),
					o.models.checkAuth(p.id, { signal: ctx.signal }).catch(() => undefined),
				]);
				providers.push(
					create(AiProviderSchema, {
						id: p.id,
						name: p.name,
						apiKey: p.auth.apiKey?.login !== undefined,
						oauth: p.auth.oauth !== undefined,
						subscription: p.auth.oauth?.isSubscription ?? false,
						oauthLabel: p.auth.oauth ? (p.auth.oauth.loginLabel ?? p.auth.oauth.name) : "",
						credential:
							stored?.type === "oauth" ? AiCredential.OAUTH : stored?.type === "api_key" ? AiCredential.API_KEY : AiCredential.UNSPECIFIED,
						available: check !== undefined,
						source: check?.source ?? "",
						custom: custom.has(p.id),
					}),
				);
			}
			return { providers };
		},

		async listAiModels(req, ctx) {
			const id = req.provider || undefined;
			if (id) provider(id);
			const models = req.available ? await o.models.getAvailable(id, { signal: ctx.signal }) : o.models.getModels(id);
			return { models: models.map(model) };
		},

		async setAiApiKey(req, ctx) {
			const p = provider(req.provider);
			if (!p.auth.apiKey) throw new ConnectError(`${p.id} takes no API key`, Code.FailedPrecondition);
			if (req.key === "") {
				const stored = await o.credentials.read(p.id, { signal: ctx.signal });
				if (stored?.type === "api_key") await o.credentials.delete(p.id, { signal: ctx.signal });
			} else {
				await o.credentials.modify(p.id, async (current) => ({ type: "api_key", key: req.key, ...(current?.type === "api_key" && current.env ? { env: current.env } : {}) }), {
					signal: ctx.signal,
				});
			}
			return {};
		},

		async startAiLogin(req) {
			const p = provider(req.provider);
			if (!p.auth.oauth) throw new ConnectError(`${p.id} has no OAuth login; set its API key`, Code.FailedPrecondition);
			return { loginId: o.logins.start(p.id, "oauth") };
		},

		async getAiLogin(req) {
			const login = o.logins.get(req.loginId);
			if (!login) throw new ConnectError(`no login ${req.loginId}`, Code.NotFound);
			return loginState(login);
		},

		async *watchAiLogin(req, ctx) {
			if (!o.logins.get(req.loginId)) throw new ConnectError(`no login ${req.loginId}`, Code.NotFound);
			for await (const login of o.logins.watch(req.loginId, ctx.signal)) yield loginState(login);
		},

		async answerAiLogin(req) {
			if (!o.logins.answer(req.loginId, req.promptId, req.value)) {
				throw new ConnectError(`login ${req.loginId} does not wait for prompt ${req.promptId}`, Code.FailedPrecondition);
			}
			return {};
		},

		async cancelAiLogin(req) {
			if (!o.logins.cancel(req.loginId)) throw new ConnectError(`no login ${req.loginId}`, Code.NotFound);
			return {};
		},

		async logoutAi(req, ctx) {
			provider(req.provider);
			await o.models.logout(req.provider, { signal: ctx.signal });
			return {};
		},

		async completeAi(req, ctx) {
			const c = await completion(o.models, req, ctx.signal);
			let answer: AssistantMessage;
			try {
				answer = await o.models.completeSimple(c.model, c.context, c.options);
			} catch (e) {
				throw ConnectError.from(e);
			}
			return response(req, answer);
		},

		async *streamAi(req, ctx) {
			if (!req.completion) throw new ConnectError("completion is required", Code.InvalidArgument);
			const c = await completion(o.models, req.completion, ctx.signal);
			let answer: AssistantMessage | undefined;
			try {
				for await (const e of o.models.streamSimple(c.model, c.context, c.options)) {
					if (e.type === "text_delta") yield { event: { case: "text", value: e.delta } };
					else if (e.type === "thinking_delta") yield { event: { case: "thinking", value: e.delta } };
					else if (e.type === "done") answer = e.message;
					else if (e.type === "error") answer = e.error;
				}
			} catch (e) {
				throw ConnectError.from(e);
			}
			if (!answer) throw new ConnectError("the model's answer ended early", Code.Unavailable);
			yield { event: { case: "done", value: create(CompleteAiResponseSchema, response(req.completion, answer)) } };
		},
	};
}


/** loginState converts what a login has shown. */
function loginState(login: LoginView) {
	const p = login.prompt?.prompt;
	return {
		state: loginStates[login.state],
		events: login.events.map(event),
		...(login.prompt && p
			? {
					prompt: create(AiLoginPromptSchema, {
						id: login.prompt.id,
						kind: promptKinds[p.type],
						message: p.message,
						placeholder: "placeholder" in p ? (p.placeholder ?? "") : "",
						options: p.type === "select" ? p.options.map((x) => ({ id: x.id, label: x.label, description: x.description ?? "" })) : [],
					}),
				}
			: {}),
		error: login.error ?? "",
	};
}

/** A completion pi-ai is asked for. */
interface Completion {
	model: Model<Api>;
	context: Context;
	options: Parameters<MutableModels["completeSimple"]>[2];
}

/** completion converts a request into what pi-ai is asked, checking the model and its credential. */
async function completion(models: MutableModels, req: CompleteAiRequest, signal: AbortSignal): Promise<Completion> {
	const m = models.getModel(req.provider, req.model);
	if (!m) throw new ConnectError(`no model ${req.provider}/${req.model}`, Code.NotFound);
	const now = Date.now();
	const messages: Message[] = req.messages.map((x) =>
		x.role === AiRole.ASSISTANT
			? {
					role: "assistant",
					content: [{ type: "text", text: x.text }],
					api: m.api,
					provider: m.provider,
					model: m.id,
					usage: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, totalTokens: 0, cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 } },
					stopReason: "stop",
					timestamp: now,
				}
			: { role: "user", content: x.text, timestamp: now },
	);
	const context: Context = { messages };
	let system = req.system;
	if (req.outputSchema !== "") {
		let parameters: TSchema;
		try {
			parameters = JSON.parse(req.outputSchema) as TSchema;
		} catch (e) {
			throw new ConnectError(`output_schema: ${e instanceof Error ? e.message : e}`, Code.InvalidArgument);
		}
		const tool: Tool = {
			name: outputTool,
			description: "Respond with the result.",
			parameters,
			constrainedSampling: { type: "json_schema", strict: "prefer" },
		};
		context.tools = [tool];
		system = [system, `Respond only by calling the ${outputTool} tool once with the result.`].filter((s) => s !== "").join("\n\n");
	}
	if (system !== "") context.systemPrompt = system;
	if (!(await models.checkAuth(m.provider, { signal }))) {
		throw new ConnectError(`no credential for ${m.provider}`, Code.Unauthenticated);
	}
	const reasoning = thinkingLevels[req.thinking];
	return {
		model: m,
		context,
		options: {
			signal,
			...(reasoning ? { reasoning } : {}),
			...(req.maxTokens > 0 ? { maxTokens: req.maxTokens } : {}),
			...(isFieldSet(req, CompleteAiRequestSchema.field.temperature) ? { temperature: req.temperature } : {}),
		},
	};
}

/** response converts a model's answer, failing for a failed one. */
function response(req: CompleteAiRequest, answer: AssistantMessage) {
	if (answer.stopReason === "error" || answer.stopReason === "aborted") throw failure(answer);
	const r = { text: text(answer), outputJson: "", usage: usage(answer.usage), stopReason: answer.stopReason };
	if (req.outputSchema !== "") {
		const call = answer.content.find((c) => c.type === "toolCall" && c.name === outputTool);
		if (call?.type === "toolCall") {
			r.outputJson = JSON.stringify(call.arguments);
		} else {
			// Models that ignore the tool often answer with the JSON itself.
			const json = r.text.replace(/^\s*```(?:json)?\s*|\s*```\s*$/g, "");
			try {
				r.outputJson = JSON.stringify(JSON.parse(json));
			} catch {
				throw new ConnectError("the model did not answer with the output schema", Code.Unknown);
			}
		}
	}
	return r;
}

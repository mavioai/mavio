import { afterEach, describe, expect, test } from "bun:test";
import { mkdtempSync, readFileSync, statSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError, createClient, createRouterTransport } from "@connectrpc/connect";
import { createProvider, type OAuthAuth, type Provider } from "@earendil-works/pi-ai";
import { fauxAssistantMessage, fauxProvider, fauxToolCall } from "@earendil-works/pi-ai/providers/faux";
import {
	AgentSessionService,
	AiCredential,
	AiGatewayService,
	AiLoginPromptKind,
	AiLoginState,
	AiRole,
	CompleteAiRequestSchema,
} from "@mavio/proto/mavio/plugin/v1/ai_pb";
import { PluginService } from "@mavio/proto/mavio/plugin/v1/plugin_pb";
import { openPlugin, type Plugin } from "./plugin.ts";

const open: Plugin[] = [];

afterEach(async () => {
	for (const p of open.splice(0)) await p.close();
});

async function start(providers: Provider[], dataDir?: string) {
	const plugin = await openPlugin({ providers, ...(dataDir ? { dataDir } : {}) });
	open.push(plugin);
	const transport = createRouterTransport((router) => plugin.routes(router));
	return {
		plugin,
		lifecycle: createClient(PluginService, transport),
		ai: createClient(AiGatewayService, transport),
		agents: createClient(AgentSessionService, transport),
	};
}

async function code(p: Promise<unknown>): Promise<Code | undefined> {
	try {
		await p;
		return undefined;
	} catch (e) {
		return ConnectError.from(e).code;
	}
}

/** An OAuth provider whose login shows a link and asks for the code. */
function oauthProvider(): Provider {
	const oauth: OAuthAuth = {
		name: "Example (Pro)",
		isSubscription: true,
		loginLabel: "Sign in with Example Pro",
		async login(interaction) {
			interaction.notify({ type: "auth_url", url: "https://example.com/authorize", instructions: "Open the link" });
			const code = await interaction.prompt({ type: "manual_code", message: "Paste the code" });
			return { type: "oauth", access: `access-${code}`, refresh: "refresh", expires: Date.now() + 3_600_000 };
		},
		async refresh(credential) {
			return credential;
		},
		async toAuth(credential) {
			return { apiKey: credential.access };
		},
	};
	const faux = fauxProvider({ provider: "example" });
	return createProvider({ id: "example", name: "Example", auth: { oauth }, models: faux.models, api: faux.api as never });
}

describe("gateway", () => {
	test("lists providers and models", async () => {
		const faux = fauxProvider();
		const { ai } = await start([faux.provider, oauthProvider()]);
		const { providers } = await ai.listAiProviders({});
		const byId = Object.fromEntries(providers.map((p) => [p.id, p]));
		expect(byId.faux?.available).toBe(true);
		expect(byId.example?.oauth).toBe(true);
		expect(byId.example?.subscription).toBe(true);
		expect(byId.example?.oauthLabel).toBe("Sign in with Example Pro");
		expect(byId.example?.available).toBe(false);
		const { models } = await ai.listAiModels({ provider: "faux" });
		expect(models.map((m) => m.id)).toEqual(["faux-1"]);
		expect(await code(ai.listAiModels({ provider: "missing" }))).toBe(Code.NotFound);
	});

	test("completes", async () => {
		const faux = fauxProvider();
		faux.setResponses([
			(context) => {
				const system = context.messages.find((m) => m.role === "system");
				return fauxAssistantMessage(`system: ${typeof system?.content === "string" ? system.content : ""}`);
			},
		]);
		const { ai } = await start([faux.provider]);
		const res = await ai.completeAi({
			provider: "faux",
			model: "faux-1",
			system: "Translate.",
			messages: [{ role: AiRole.USER, text: "Hallo" }],
		});
		expect(res.text).toBe("system: Translate.");
		expect(res.stopReason).toBe("stop");
		expect(res.usage).toBeDefined();
	});

	test("streams completions", async () => {
		const faux = fauxProvider({ tokenSize: { min: 2, max: 2 } });
		faux.setResponses([fauxAssistantMessage("Bonjour tout le monde"), fauxAssistantMessage("", { stopReason: "error", errorMessage: "429 Too Many Requests" })]);
		const { ai } = await start([faux.provider]);
		const completion = { provider: "faux", model: "faux-1", messages: [{ role: AiRole.USER, text: "Hello everyone" }] };
		const texts: string[] = [];
		let done;
		for await (const r of ai.streamAi({ completion })) {
			if (r.event.case === "text") texts.push(r.event.value);
			else if (r.event.case === "done") done = r.event.value;
		}
		expect(texts.length).toBeGreaterThan(1);
		expect(texts.join("")).toBe("Bonjour tout le monde");
		expect(done?.text).toBe("Bonjour tout le monde");
		expect(done?.usage).toBeDefined();
		expect(await code(Array.fromAsync(ai.streamAi({ completion })))).toBe(Code.Unavailable);
	});

	test("answers with the output schema", async () => {
		const faux = fauxProvider();
		const translations = { translations: [{ position: 1, line: "Hello" }] };
		faux.setResponses([
			fauxAssistantMessage(fauxToolCall("respond", translations), { stopReason: "toolUse" }),
			fauxAssistantMessage('```json\n{"translations":[]}\n```'),
			fauxAssistantMessage("no JSON here"),
		]);
		const { ai } = await start([faux.provider]);
		const req = create(CompleteAiRequestSchema, {
			provider: "faux",
			model: "faux-1",
			messages: [{ role: AiRole.USER, text: '[{"position":1,"line":"Hallo"}]' }],
			outputSchema: JSON.stringify({ type: "object", properties: { translations: { type: "array" } } }),
		});
		expect(JSON.parse((await ai.completeAi(req)).outputJson)).toEqual(translations);
		expect(JSON.parse((await ai.completeAi(req)).outputJson)).toEqual({ translations: [] });
		expect(await code(ai.completeAi(req))).toBe(Code.Unknown);
	});

	test("reports failures the server may retry", async () => {
		const faux = fauxProvider();
		faux.setResponses([
			fauxAssistantMessage("", { stopReason: "error", errorMessage: "429 Too Many Requests" }),
			fauxAssistantMessage("", { stopReason: "error", errorMessage: "invalid model" }),
		]);
		const { ai } = await start([faux.provider]);
		const req = { provider: "faux", model: "faux-1", messages: [{ role: AiRole.USER, text: "Hi" }] };
		expect(await code(ai.completeAi(req))).toBe(Code.Unavailable);
		expect(await code(ai.completeAi(req))).toBe(Code.Unknown);
		expect(await code(ai.completeAi({ ...req, model: "missing" }))).toBe(Code.NotFound);
	});

	test("keeps API keys", async () => {
		const dir = mkdtempSync(join(tmpdir(), "mavio-agent-"));
		const { ai } = await start([fauxProvider().provider, customKeyProvider()], dir);
		await ai.setAiApiKey({ provider: "keyed", key: "secret" });
		let keyed = (await ai.listAiProviders({})).providers.find((p) => p.id === "keyed");
		expect(keyed?.credential).toBe(AiCredential.API_KEY);
		expect(keyed?.available).toBe(true);
		const file = join(dir, "credentials.json");
		expect(JSON.parse(readFileSync(file, "utf8")).keyed.key).toBe("secret");
		if (process.platform !== "win32") expect(statSync(file).mode & 0o777).toBe(0o600);
		await ai.setAiApiKey({ provider: "keyed", key: "" });
		keyed = (await ai.listAiProviders({})).providers.find((p) => p.id === "keyed");
		expect(keyed?.credential).toBe(AiCredential.UNSPECIFIED);
		expect(keyed?.available).toBe(false);
	});

	test("relays OAuth logins", async () => {
		const { ai } = await start([oauthProvider()]);
		const { loginId } = await ai.startAiLogin({ provider: "example" });
		const watch = ai.watchAiLogin({ loginId })[Symbol.asyncIterator]();
		let login = (await watch.next()).value!;
		while (!login.prompt) login = (await watch.next()).value!;
		expect(login.state).toBe(AiLoginState.RUNNING);
		expect((await ai.getAiLogin({ loginId })).prompt?.id).toBe(login.prompt.id);
		expect(login.events[0]?.event).toEqual({
			case: "url",
			value: expect.objectContaining({ url: "https://example.com/authorize", instructions: "Open the link" }),
		});
		expect(login.prompt?.kind).toBe(AiLoginPromptKind.MANUAL_CODE);
		expect(await code(ai.answerAiLogin({ loginId, promptId: "stale", value: "x" }))).toBe(Code.FailedPrecondition);
		await ai.answerAiLogin({ loginId, promptId: login.prompt.id, value: "42" });
		for (let r = await watch.next(); !r.done; r = await watch.next()) login = r.value;
		expect(login.state).toBe(AiLoginState.SUCCEEDED);
		expect(await code(ai.watchAiLogin({ loginId: "missing" })[Symbol.asyncIterator]().next())).toBe(Code.NotFound);
		const example = (await ai.listAiProviders({})).providers.find((p) => p.id === "example");
		expect(example?.credential).toBe(AiCredential.OAUTH);
		expect(example?.available).toBe(true);
		await ai.logoutAi({ provider: "example" });
		expect((await ai.listAiProviders({})).providers.find((p) => p.id === "example")?.available).toBe(false);
	});

	test("cancels logins", async () => {
		const { ai } = await start([oauthProvider()]);
		const { loginId } = await ai.startAiLogin({ provider: "example" });
		await ai.cancelAiLogin({ loginId });
		let state = AiLoginState.UNSPECIFIED;
		for await (const login of ai.watchAiLogin({ loginId })) state = login.state;
		expect(state).toBe(AiLoginState.CANCELED);
		expect(await code(ai.startAiLogin({ provider: "keyed" }))).toBe(Code.NotFound);
	});
});

/** A provider that is available once its key is set. */
function customKeyProvider(): Provider {
	const faux = fauxProvider({ provider: "keyed" });
	return createProvider({
		id: "keyed",
		auth: {
			apiKey: {
				name: "Keyed API key",
				login: async () => ({ type: "api_key", key: "" }),
				resolve: async ({ credential }) => (credential?.key ? { auth: { apiKey: credential.key }, source: "stored credential" } : undefined),
			},
		},
		models: faux.models,
		api: faux.api as never,
	});
}

describe("configuration", () => {
	test("adds OpenAI-compatible providers", async () => {
		let auth = "";
		const server = Bun.serve({
			port: 0,
			hostname: "127.0.0.1",
			async fetch(req) {
				auth = req.headers.get("authorization") ?? "";
				const body = (await req.json()) as { model: string };
				const chunk = (delta: object, finish: string | null) =>
					`data: ${JSON.stringify({ id: "1", object: "chat.completion.chunk", created: 0, model: body.model, choices: [{ index: 0, delta, finish_reason: finish }] })}\n\n`;
				const stream = chunk({ role: "assistant", content: "Bonjour" }, null) + chunk({}, "stop") + "data: [DONE]\n\n";
				return new Response(stream, { headers: { "content-type": "text/event-stream" } });
			},
		});
		try {
			const { lifecycle, ai } = await start([fauxProvider().provider]);
			const config = { providers: [{ id: "local", name: "Local", base_url: `http://127.0.0.1:${server.port}/v1`, models: [{ id: "llama" }] }] };
			await lifecycle.configure({ configJson: JSON.stringify(config) });
			const local = (await ai.listAiProviders({})).providers.find((p) => p.id === "local");
			expect(local?.custom).toBe(true);
			expect(local?.available).toBe(true);
			const res = await ai.completeAi({ provider: "local", model: "llama", messages: [{ role: AiRole.USER, text: "Hello" }] });
			expect(res.text).toBe("Bonjour");
			expect(auth).toBe("Bearer none");

			await lifecycle.configure({ configJson: "{}" });
			expect((await ai.listAiProviders({})).providers.some((p) => p.id === "local")).toBe(false);
			const invalid = [
				{ providers: [{ id: "faux", base_url: "http://x", models: [{ id: "m" }] }] },
				{ providers: [{ id: "a", base_url: "ftp://x", models: [{ id: "m" }] }] },
				{ providers: [{ id: "a", base_url: "http://x", models: [] }] },
			];
			for (const c of invalid) expect(await code(lifecycle.configure({ configJson: JSON.stringify(c) }))).toBe(Code.InvalidArgument);
		} finally {
			server.stop(true);
		}
	});
});

describe("sessions", () => {
	test("keep conversations across restarts", async () => {
		const dir = mkdtempSync(join(tmpdir(), "mavio-agent-"));
		const faux = fauxProvider();
		faux.setResponses([fauxAssistantMessage("Paris."), fauxAssistantMessage("Lyon.")]);
		let { plugin, agents } = await start([faux.provider], dir);
		const { session } = await agents.createAgentSession({ provider: "faux", model: "faux-1", title: "Geography", instructions: "Answer in one word." });
		const id = session!.id;
		const sent = await agents.sendAgentMessage({ sessionId: id, text: "Capital of France?", requestId: "q1" });
		expect(sent.answer?.text).toBe("Paris.");
		expect(sent.answer?.role).toBe(AiRole.ASSISTANT);
		await plugin.close();
		open.splice(open.indexOf(plugin), 1);

		({ agents } = await start([faux.provider], dir));
		await agents.sendAgentMessage({ sessionId: id, text: "Second city?" });
		const got = await agents.getAgentSession({ id });
		expect(got.session?.title).toBe("Geography");
		expect(got.messages.map((m) => [m.role, m.text])).toEqual([
			[AiRole.USER, "Capital of France?"],
			[AiRole.ASSISTANT, "Paris."],
			[AiRole.USER, "Second city?"],
			[AiRole.ASSISTANT, "Lyon."],
		]);
		const { sessions } = await agents.listAgentSessions({});
		expect(sessions.map((s) => s.id)).toEqual([id]);
		await agents.deleteAgentSession({ id });
		expect((await agents.listAgentSessions({})).sessions).toEqual([]);
		expect(await code(agents.getAgentSession({ id }))).toBe(Code.NotFound);
		expect(await code(agents.sendAgentMessage({ sessionId: "nope", text: "Hi" }))).toBe(Code.NotFound);
	});

	test("stream answers", async () => {
		const faux = fauxProvider({ tokenSize: { min: 1, max: 1 }, tokensPerSecond: 50 });
		faux.setResponses([fauxAssistantMessage("The capital of France is Paris.")]);
		const { agents } = await start([faux.provider]);
		const { session } = await agents.createAgentSession({ provider: "faux", model: "faux-1" });
		const texts: string[] = [];
		let done;
		for await (const r of agents.streamAgentMessage({ message: { sessionId: session!.id, text: "Capital of France?" } })) {
			if (r.event.case === "text") texts.push(r.event.value);
			else if (r.event.case === "done") done = r.event.value;
		}
		expect(done?.answer?.text).toBe("The capital of France is Paris.");
		expect(texts.length).toBeGreaterThan(0);
		for (const t of texts) expect("The capital of France is Paris.".startsWith(t)).toBe(true);
		expect(await code(Array.fromAsync(agents.streamAgentMessage({ message: { sessionId: "nope", text: "Hi" } })))).toBe(Code.NotFound);
	});

	test("are purged once deleted", async () => {
		const dir = mkdtempSync(join(tmpdir(), "mavio-agent-"));
		const faux = fauxProvider();
		faux.setResponses([fauxAssistantMessage("Paris."), fauxAssistantMessage("Rome.")]);
		let { plugin, agents } = await start([faux.provider], dir);
		const ids: string[] = [];
		for (const q of ["Capital of France?", "Capital of Italy?"]) {
			const { session } = await agents.createAgentSession({ provider: "faux", model: "faux-1" });
			await agents.sendAgentMessage({ sessionId: session!.id, text: q });
			ids.push(session!.id);
		}
		await agents.deleteAgentSession({ id: ids[0]! });
		await plugin.close();
		open.splice(open.indexOf(plugin), 1);

		const rows = (id: string) => {
			const db = new DatabaseSync(join(dir, "agent.db"));
			try {
				return (db.prepare("SELECT count(*) AS n FROM entries WHERE conversation_id = ?").get(Number(id)) as { n: number }).n;
			} finally {
				db.close();
			}
		};
		expect(rows(ids[0]!)).toBeGreaterThan(0);
		({ agents } = await start([faux.provider], dir));
		expect(rows(ids[0]!)).toBe(0);
		expect(rows(ids[1]!)).toBeGreaterThan(0);
		const kept = await agents.getAgentSession({ id: ids[1]! });
		expect(kept.messages.map((m) => m.text)).toEqual(["Capital of Italy?", "Rome."]);
		expect(readFileSync(join(dir, "deleted-sessions.json"), "utf8")).toBe("[]");
	});
});

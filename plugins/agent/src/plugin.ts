// The plugin's state: the AI providers with their credentials, the logins
// in progress and the agent sessions, and the Connect services over them.

import { join } from "node:path";
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError, type ConnectRouter } from "@connectrpc/connect";
import { BACKGROUND_CONTEXT } from "@earendil-works/chord/context";
import { createModels, type MutableModels, type Provider } from "@earendil-works/pi-ai";
import { registerBunOAuthFlows } from "@earendil-works/pi-ai/bun-oauth";
import { builtinModels } from "@earendil-works/pi-ai/providers/all";
import { InlineCodemodeSandbox, loadQuickJSWasm } from "@earendil-works/pi-codemode";
import { createRegistry, type Extension, Harness, MemoryStorage, type Storage } from "@earendil-works/pi-durable";
import { createCodemodeExtension } from "@earendil-works/pi-durable/codemode";
import { openNodeSqliteStorage } from "@earendil-works/pi-durable/storage/sqlite/node";
import { AgentSessionService, AiGatewayService } from "@mavio/proto/mavio/plugin/v1/ai_pb";
import { HealthResponseSchema, ManifestSchema, PluginService } from "@mavio/proto/mavio/plugin/v1/plugin_pb";
import manifest from "../manifest.json" with { type: "json" };
import { customProvider, parseConfig } from "./config.ts";
import { FileCredentialStore } from "./credentials.ts";
import { gateway } from "./gateway.ts";
import { HostCalls } from "./host.ts";
import { Logins } from "./logins.ts";
import { Deleted } from "./purge.ts";
import { memoryExtension } from "./memory.ts";
import { sessions } from "./sessions.ts";
import quickjs from "quickjs-wasi/quickjs.wasm" with { type: "file" };

/** The plugin's ID and version, as its manifest gives them. */
export const id = manifest.id;
export const version = manifest.version;

/** What the plugin runs with. */
export interface PluginOptions {
	/** The plugin's data folder; nothing is kept without one. */
	dataDir?: string;
	/** Replace the built-in providers, for tests. */
	providers?: Provider[];
}

/** A running plugin. */
export interface Plugin {
	readonly models: MutableModels;
	routes(router: ConnectRouter): void;
	/** idle resolves when the plugin's background work is done. */
	idle(): Promise<void>;
	close(): Promise<void>;
}

let oauthRegistered = false;

/** openPlugin opens the plugin's state in its data folder. */
export async function openPlugin(o: PluginOptions = {}): Promise<Plugin> {
	if (!oauthRegistered) {
		// The OAuth flows load lazily, which a compiled executable cannot.
		registerBunOAuthFlows();
		oauthRegistered = true;
	}
	const credentials = new FileCredentialStore(o.dataDir ? join(o.dataDir, "credentials.json") : undefined);
	const models = o.providers ? createModels({ credentials }) : builtinModels({ credentials });
	for (const p of o.providers ?? []) models.setProvider(p);
	const builtIn = new Set(models.getProviders().map((p) => p.id));
	let custom = new Set<string>();
	const logins = new Logins(models);
	const deleted = new Deleted(o.dataDir ? join(o.dataDir, "deleted-sessions.json") : undefined);
	let storage: Storage = new MemoryStorage();
	if (o.dataDir) {
		const db = join(o.dataDir, "agent.db");
		await deleted.purge(db);
		storage = await openNodeSqliteStorage(db);
	}
	// Conversations offer only the extensions they are created with.
	const registry = createRegistry();
	const memory = memoryExtension();
	registry.install(memory);
	// Scripts of code mode run in QuickJS on the plugin's thread: they
	// mostly wait for the host's tools.
	let codemode: Extension | undefined;
	try {
		codemode = createCodemodeExtension({ sandbox: new InlineCodemodeSandbox({ wasm: await loadQuickJSWasm(quickjs) }) });
		registry.install(codemode);
	} catch (e) {
		console.error("agent: code mode is not available:", e);
	}
	const harness = await Harness.open(storage, { models, registry, settings: { extensions: [] } }, BACKGROUND_CONTEXT);
	const agentSessions = sessions({ harness, deleted, registry, models, calls: new HostCalls(), memory, codemode });
	// The host's tools are installed before the calls the last process left
	// unfinished run again.
	await agentSessions.restore();
	// Answers the last process left unfinished go on.
	harness.resume();

	return {
		models,
		routes(router) {
			router.service(PluginService, {
				describe: () => ({ manifest: create(ManifestSchema, { id, version }) }),
				configure: (req) => {
					let config;
					try {
						config = parseConfig(req.configJson, builtIn);
					} catch (e) {
						throw new ConnectError(e instanceof Error ? e.message : String(e), Code.InvalidArgument);
					}
					const providers = (config.providers ?? []).map(customProvider);
					for (const p of custom) models.deleteProvider(p);
					for (const p of providers) models.setProvider(p);
					custom = new Set(providers.map((p) => p.id));
					return {};
				},
				health: () => create(HealthResponseSchema, { healthy: true }),
				shutdown: () => ({}),
			});
			router.service(AiGatewayService, gateway({ models, credentials, logins, custom: () => custom }));
			router.service(AgentSessionService, agentSessions.service);
		},
		idle: () => agentSessions.idle(),
		async close() {
			await agentSessions.idle();
			logins.close();
			await harness.close(BACKGROUND_CONTEXT);
		},
	};
}

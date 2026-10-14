// The plugin's state: the AI providers with their credentials, the logins
// in progress and the agent sessions, and the Connect services over them.

import { join } from "node:path";
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError, type ConnectRouter } from "@connectrpc/connect";
import { BACKGROUND_CONTEXT } from "@earendil-works/chord/context";
import { createModels, type MutableModels, type Provider } from "@earendil-works/pi-ai";
import { registerBunOAuthFlows } from "@earendil-works/pi-ai/bun-oauth";
import { builtinModels } from "@earendil-works/pi-ai/providers/all";
import { createRegistry, Harness, MemoryStorage, type Storage } from "@earendil-works/pi-durable";
import { openNodeSqliteStorage } from "@earendil-works/pi-durable/storage/sqlite/node";
import { AgentSessionService, AiGatewayService } from "@mavio/proto/mavio/plugin/v1/ai_pb";
import { HealthResponseSchema, ManifestSchema, PluginService } from "@mavio/proto/mavio/plugin/v1/plugin_pb";
import manifest from "../manifest.json" with { type: "json" };
import { customProvider, parseConfig } from "./config.ts";
import { FileCredentialStore } from "./credentials.ts";
import { gateway } from "./gateway.ts";
import { Logins } from "./logins.ts";
import { sessions } from "./sessions.ts";

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
	const storage: Storage = o.dataDir ? await openNodeSqliteStorage(join(o.dataDir, "agent.db")) : new MemoryStorage();
	const harness = await Harness.open(storage, { models, registry: createRegistry() }, BACKGROUND_CONTEXT);
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
			router.service(AgentSessionService, sessions(harness));
		},
		async close() {
			logins.close();
			await harness.close(BACKGROUND_CONTEXT);
		},
	};
}

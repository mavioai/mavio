// The plugin's configuration: providers the administrator adds, served by
// OpenAI-compatible APIs such as Ollama's, LocalAI's, LM Studio's or vLLM's.

import { createProvider, type ApiKeyAuth, type Model, type Provider } from "@earendil-works/pi-ai";
import { openAICompletionsApi } from "@earendil-works/pi-ai/api/openai-completions.lazy";

/** A model of a custom provider. */
export interface CustomModel {
	id: string;
	name?: string;
	context_window?: number;
	max_tokens?: number;
	reasoning?: boolean;
	images?: boolean;
}

/** A provider the administrator adds. */
export interface CustomProvider {
	id: string;
	name?: string;
	base_url: string;
	/** The key the API takes; keyless servers need none. */
	api_key?: string;
	headers?: Record<string, string>;
	models: CustomModel[];
}

/** The plugin's configuration. */
export interface Config {
	providers?: CustomProvider[];
}

const idPattern = /^[a-z0-9][a-z0-9._-]*$/;

/**
 * parseConfig reads and checks a configuration; builtIn are the IDs that
 * custom providers cannot take.
 */
export function parseConfig(json: string, builtIn: ReadonlySet<string>): Config {
	const config = (json.trim() === "" ? {} : JSON.parse(json)) as Config;
	if (typeof config !== "object" || config === null || Array.isArray(config)) {
		throw new Error("the configuration is not an object");
	}
	const seen = new Set<string>();
	for (const [i, p] of (config.providers ?? []).entries()) {
		const at = `providers[${i}]`;
		if (typeof p.id !== "string" || !idPattern.test(p.id)) throw new Error(`${at}: id ${JSON.stringify(p.id)} is invalid`);
		if (builtIn.has(p.id)) throw new Error(`${at}: ${p.id} is a built-in provider`);
		if (seen.has(p.id)) throw new Error(`${at}: ${p.id} is listed twice`);
		seen.add(p.id);
		let url: URL;
		try {
			url = new URL(p.base_url);
		} catch {
			throw new Error(`${at}: base_url ${JSON.stringify(p.base_url)} is not a URL`);
		}
		if (url.protocol !== "http:" && url.protocol !== "https:") throw new Error(`${at}: base_url must be http or https`);
		if (!Array.isArray(p.models) || p.models.length === 0) throw new Error(`${at}: models are required`);
		for (const [j, m] of p.models.entries()) {
			if (typeof m.id !== "string" || m.id === "") throw new Error(`${at}.models[${j}]: id is required`);
		}
	}
	return config;
}

/**
 * keyAuth resolves a stored key first, then the configured one; a keyless
 * server gets a placeholder, as OpenAI's client requires a key.
 */
function keyAuth(name: string, key: string | undefined): ApiKeyAuth {
	return {
		name: `${name} API key`,
		login: async (interaction) => ({
			type: "api_key",
			key: await interaction.prompt({ type: "secret", message: `Enter the ${name} API key` }),
		}),
		resolve: async ({ credential }) => {
			if (credential?.key) return { auth: { apiKey: credential.key }, source: "stored credential" };
			if (key) return { auth: { apiKey: key }, source: "configuration" };
			return { auth: { apiKey: "none" }, source: "keyless" };
		},
	};
}

/** customProvider builds a provider the administrator added. */
export function customProvider(p: CustomProvider): Provider {
	const name = p.name || p.id;
	const models: Model<"openai-completions">[] = p.models.map((m) => ({
		id: m.id,
		name: m.name || m.id,
		api: "openai-completions",
		provider: p.id,
		baseUrl: p.base_url,
		input: m.images ? ["text", "image"] : ["text"],
		cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
		reasoning: m.reasoning ?? false,
		contextWindow: m.context_window ?? 128_000,
		maxTokens: m.max_tokens ?? 16_384,
	}));
	return createProvider({
		id: p.id,
		name,
		baseUrl: p.base_url,
		...(p.headers ? { headers: p.headers } : {}),
		auth: { apiKey: keyAuth(name, p.api_key) },
		models,
		api: openAICompletionsApi(),
	});
}

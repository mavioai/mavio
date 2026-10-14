// Mavio's agent plugin, a process plugin: an AI gateway to the providers
// pi-ai supports, with their credentials and logins, as the AI_GATEWAY
// capability, and agent sessions kept by pi-durable, as AGENT_SESSION.

import { dataDir, serve } from "@mavio/plugin";
import { id, openPlugin } from "./plugin.ts";

try {
	const plugin = await openPlugin({ dataDir: dataDir() || undefined });
	await serve({ pluginId: id, routes: (router) => plugin.routes(router) });
	await plugin.close();
	process.exit(0);
} catch (e) {
	console.error("agent:", e);
	process.exit(1);
}

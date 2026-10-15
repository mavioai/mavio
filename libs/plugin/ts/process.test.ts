import { mkdtempSync } from "node:fs";
import * as net from "node:net";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { PassThrough } from "node:stream";
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError, createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-node";
import { HealthResponseSchema, PluginService } from "@mavio/proto/mavio/plugin/v1/plugin_pb";
import { afterEach, beforeEach, describe, expect, test } from "vitest";
import { env, loopback, serve } from "./process.ts";

const token = "secret";

function client(handshake: { address?: string }, socket: string, auth = token) {
	return createClient(
		PluginService,
		createConnectTransport({
			baseUrl: "http://plugin",
			httpVersion: "2",
			interceptors: [
				(next) => (req) => {
					req.header.set("Authorization", "Bearer " + auth);
					return next(req);
				},
			],
			nodeOptions: {
				createConnection: () => {
					if (handshake.address) {
						const i = handshake.address.lastIndexOf(":");
						return net.connect({ host: handshake.address.slice(0, i), port: Number(handshake.address.slice(i + 1)) });
					}
					return net.connect(socket);
				},
			},
		}),
	);
}

describe("serve", () => {
	let socket: string;
	beforeEach(() => {
		socket = join(mkdtempSync(join(tmpdir(), "mavio-plugin-")), "p.sock");
		process.env[env.socket] = socket;
		process.env[env.token] = token;
	});
	afterEach(() => {
		delete process.env[env.socket];
		delete process.env[env.token];
	});

	test("serves the plugin until Shutdown", async () => {
		const stdout = new PassThrough();
		const line = new Promise<string>((resolve) => stdout.once("data", (b: Buffer) => resolve(b.toString())));
		const done = serve({
			pluginId: "dev.mavio.test",
			stdout,
			routes(router) {
				router.service(PluginService, {
					health: () => create(HealthResponseSchema, { healthy: true, message: "ok" }),
					shutdown: () => ({}),
				});
			},
		});
		const handshake = JSON.parse(await line);
		expect(handshake.mavio_plugin).toBe(1);
		expect(handshake.plugin_id).toBe("dev.mavio.test");
		expect(handshake.address !== undefined).toBe(loopback());

		const denied = await client(handshake, socket, "wrong")
			.health({})
			.catch((e: unknown) => ConnectError.from(e));
		expect(denied).toBeInstanceOf(ConnectError);
		expect((denied as ConnectError).code).toBe(Code.Unauthenticated);

		const c = client(handshake, socket);
		const h = await c.health({});
		expect(h.message).toBe("ok");
		await c.shutdown({});
		await done;
	});

	test("requires the host's environment", async () => {
		delete process.env[env.token];
		await expect(serve({ pluginId: "dev.mavio.test", routes() {} })).rejects.toThrow(env.token);
	});
});

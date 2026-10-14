// The guest side of child-process plugins written in TypeScript, as
// guest/process is for Go: it serves the plugin's Connect services to the
// host, authenticates the host's token, prints the handshake line and
// returns after the Shutdown RPC or a termination signal.

import { timingSafeEqual } from "node:crypto";
import { rmSync } from "node:fs";
import * as http2 from "node:http2";
import * as net from "node:net";
import type { ConnectRouter, Interceptor, Transport } from "@connectrpc/connect";
import { connectNodeAdapter, createConnectTransport } from "@connectrpc/connect-node";

/** Environment variables the host sets for the plugin. */
export const env = {
	socket: "MAVIO_PLUGIN_SOCKET",
	token: "MAVIO_PLUGIN_TOKEN",
	hostSocket: "MAVIO_HOST_SOCKET",
	hostAddress: "MAVIO_HOST_ADDRESS",
	data: "MAVIO_PLUGIN_DATA",
	hostOS: "MAVIO_HOST_OS",
} as const;

/** The handshake protocol version. */
export const protocolVersion = 1;

/** The base URL of the host API. */
export const hostURL = "http://mavio.host";

/** The prefix of requests to the plugin's HTTP routes. */
export const httpPrefix = "/_mavio/http";

const shutdownPath = "/mavio.plugin.v1.PluginService/Shutdown";
const tokenHeader = "mavio-plugin-token";

/**
 * Whether the plugin serves on loopback TCP instead of the socket: Node's
 * and Bun's runtimes have no Unix sockets on Windows.
 */
export function loopback(platform: NodeJS.Platform = process.platform): boolean {
	return platform === "win32";
}

/** The plugin's writable data folder. */
export function dataDir(): string {
	return process.env[env.data] ?? "";
}

/** The host's operating system and its version, such as "ubuntu 22.04". */
export function hostOS(): string {
	return process.env[env.hostOS] ?? "";
}

/** What serve serves. */
export interface ServeOptions {
	/** The plugin's ID, reported in the handshake. */
	pluginId: string;
	/** Registers the plugin's Connect services. */
	routes: (router: ConnectRouter) => void;
	/** Serves the plugin's HTTP routes, with the prefix removed. */
	http?: (req: http2.Http2ServerRequest, res: http2.Http2ServerResponse) => void;
	/** Where the handshake line goes; stdout by default. */
	stdout?: NodeJS.WritableStream;
}

function sameToken(got: string | undefined, want: string): boolean {
	if (got === undefined) return false;
	const a = Buffer.from(got);
	const b = Buffer.from(want);
	return a.length === b.length && timingSafeEqual(a, b);
}

/**
 * serve runs the plugin until the host shuts it down. It resolves once the
 * server is closed; the plugin then cleans up and exits.
 */
export async function serve(options: ServeOptions): Promise<void> {
	const socket = process.env[env.socket];
	const token = process.env[env.token];
	if (!socket || !token) {
		throw new Error(`${env.socket} and ${env.token} must be set; plugins are started by the Mavio host`);
	}
	const connect = connectNodeAdapter({ routes: options.routes });
	const sessions = new Set<http2.ServerHttp2Session>();
	const server = http2.createServer();
	let stopping: Promise<void> | undefined;
	const stop = () => {
		stopping ??= new Promise<void>((resolve) => {
			server.close(() => resolve());
			for (const s of sessions) s.close();
		});
		return stopping;
	};

	server.on("session", (s) => {
		sessions.add(s);
		s.on("close", () => sessions.delete(s));
	});
	server.on("request", (req, res) => {
		const path = req.url ?? "";
		if (path.startsWith(httpPrefix + "/")) {
			if (!options.http || !sameToken(req.headers[tokenHeader] as string | undefined, token)) {
				res.writeHead(options.http ? 401 : 404).end();
				return;
			}
			delete req.headers[tokenHeader];
			req.url = path.slice(httpPrefix.length);
			options.http(req, res);
			return;
		}
		if (!sameToken(req.headers.authorization, "Bearer " + token)) {
			res.writeHead(401).end();
			return;
		}
		if (path === shutdownPath) {
			res.on("finish", () => void stop());
		}
		connect(req, res);
	});

	const signals = () => void stop();
	process.once("SIGTERM", signals);
	process.once("SIGINT", signals);

	let address: string | undefined;
	await new Promise<void>((resolve, reject) => {
		server.once("error", reject);
		if (loopback()) {
			server.listen(0, "127.0.0.1", () => {
				const a = server.address() as net.AddressInfo;
				address = `127.0.0.1:${a.port}`;
				resolve();
			});
		} else {
			rmSync(socket, { force: true });
			server.listen(socket, () => resolve());
		}
	});
	const handshake: Record<string, unknown> = { mavio_plugin: protocolVersion, plugin_id: options.pluginId };
	if (address) handshake.address = address;
	(options.stdout ?? process.stdout).write(JSON.stringify(handshake) + "\n");

	await new Promise<void>((resolve) => server.once("close", () => resolve()));
	await stop();
	process.off("SIGTERM", signals);
	process.off("SIGINT", signals);
}

/**
 * hostTransport is a Connect transport to the host API, carrying the
 * plugin's token; it fails outside a Mavio host.
 */
export function hostTransport(interceptors: Interceptor[] = []): Transport {
	const token = process.env[env.token] ?? "";
	const socket = process.env[env.hostSocket];
	const address = process.env[env.hostAddress];
	const auth: Interceptor = (next) => (req) => {
		req.header.set("Authorization", "Bearer " + token);
		return next(req);
	};
	return createConnectTransport({
		baseUrl: hostURL,
		httpVersion: "2",
		interceptors: [auth, ...interceptors],
		nodeOptions: {
			createConnection: () => {
				if (loopback() && address) {
					const i = address.lastIndexOf(":");
					return net.connect({ host: address.slice(0, i), port: Number(address.slice(i + 1)) });
				}
				if (!socket) throw new Error("the host API is not available outside a Mavio host");
				return net.connect(socket);
			},
		},
	});
}

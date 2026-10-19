// The ID of the machine, which logins such as Sign in with ChatGPT send the
// provider to tell installations apart: the host's (MAVIO_DEVICE_ID), from
// the machine's hardware ID or the server's ID; outside a host, made on
// first use and kept in the data folder, so that every later login sends
// the same.

import { randomUUID } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname } from "node:path";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/**
 * deviceId returns a function giving the machine's ID: host's when it is a
 * UUID, else one read from file or, when it holds none, made and written
 * there; without a file the ID lasts as long as the process.
 */
export function deviceId(host: string, file?: string): () => string {
	let id: string | undefined = uuid.test(host) ? host : undefined;
	return () => {
		if (id) return id;
		if (file) {
			try {
				const kept = readFileSync(file, "utf8").trim();
				if (uuid.test(kept)) return (id = kept);
			} catch (e) {
				if ((e as NodeJS.ErrnoException).code !== "ENOENT") throw e;
			}
		}
		const made = randomUUID();
		if (file) {
			mkdirSync(dirname(file), { recursive: true });
			writeFileSync(file, `${made}\n`, { mode: 0o600 });
		}
		return (id = made);
	};
}

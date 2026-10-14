// The AI providers' credentials, kept in a file of the plugin's data folder
// that only the server's user can read.

import { chmodSync, existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import { dirname } from "node:path";
import type { AuthOperationOptions, Credential, CredentialInfo, CredentialStore } from "@earendil-works/pi-ai";

/**
 * FileCredentialStore keeps one credential per provider in a JSON file,
 * written whole on every change. Writes are serialized, so an OAuth refresh
 * never races a login.
 */
export class FileCredentialStore implements CredentialStore {
	readonly #path: string | undefined;
	readonly #credentials: Map<string, Credential>;
	#tail: Promise<unknown> = Promise.resolve();

	/** Opens the store at path, or one in memory without a path. */
	constructor(path?: string) {
		this.#path = path;
		this.#credentials = new Map();
		if (path && existsSync(path)) {
			const stored = JSON.parse(readFileSync(path, "utf8")) as Record<string, Credential>;
			for (const [id, credential] of Object.entries(stored)) this.#credentials.set(id, credential);
		}
	}

	async read(providerId: string, options?: AuthOperationOptions): Promise<Credential | undefined> {
		options?.signal?.throwIfAborted();
		return this.#credentials.get(providerId);
	}

	async list(options?: AuthOperationOptions): Promise<readonly CredentialInfo[]> {
		options?.signal?.throwIfAborted();
		return [...this.#credentials].map(([providerId, credential]) => ({ providerId, type: credential.type }));
	}

	modify(
		providerId: string,
		fn: (current: Credential | undefined) => Promise<Credential | undefined>,
		options?: AuthOperationOptions,
	): Promise<Credential | undefined> {
		return this.#serialize(async () => {
			options?.signal?.throwIfAborted();
			const current = this.#credentials.get(providerId);
			const next = await fn(current);
			options?.signal?.throwIfAborted();
			if (next === undefined) return current;
			this.#credentials.set(providerId, next);
			this.#save();
			return next;
		});
	}

	delete(providerId: string, options?: AuthOperationOptions): Promise<void> {
		return this.#serialize(async () => {
			options?.signal?.throwIfAborted();
			if (this.#credentials.delete(providerId)) this.#save();
		});
	}

	#serialize<T>(task: () => Promise<T>): Promise<T> {
		const run = this.#tail.then(task, task);
		this.#tail = run.catch(() => {});
		return run;
	}

	#save(): void {
		if (!this.#path) return;
		mkdirSync(dirname(this.#path), { recursive: true });
		const temp = this.#path + ".tmp";
		writeFileSync(temp, JSON.stringify(Object.fromEntries(this.#credentials), null, "\t"), { mode: 0o600 });
		chmodSync(temp, 0o600);
		renameSync(temp, this.#path);
	}
}

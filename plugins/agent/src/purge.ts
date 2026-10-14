// Deleted sessions purged from pi-durable's database. pi-durable keeps
// every entry of a conversation; the plugin records the sessions it
// deletes and removes their conversations' rows when it next opens the
// database, before the harness reads it.

import { existsSync } from "node:fs";
import { readFile, rename, writeFile } from "node:fs/promises";
import type { SqliteExecutor } from "@earendil-works/pi-durable/storage/sqlite";
import { openNodeSqliteDatabase } from "@earendil-works/pi-durable/storage/sqlite/node";

/** Deleted records the sessions deleted until they are purged. */
export class Deleted {
	readonly #file: string | undefined;
	#line: Promise<unknown> = Promise.resolve();

	/** file keeps the list; without one nothing is kept. */
	constructor(file?: string) {
		this.#file = file;
	}

	/** Adds a session to purge. */
	add(id: string): Promise<void> {
		const file = this.#file;
		if (!file) return Promise.resolve();
		const next = this.#line.then(async () => {
			const ids = await read(file);
			if (ids.includes(id)) return;
			ids.push(id);
			const tmp = `${file}.tmp`;
			await writeFile(tmp, JSON.stringify(ids), { mode: 0o600 });
			await rename(tmp, file);
		});
		this.#line = next.catch(() => {});
		return next;
	}

	/**
	 * Removes the conversations of the sessions deleted from the database at
	 * path, with the conversations they own and their documents, and
	 * forgets them. The database must not be open.
	 */
	async purge(path: string): Promise<number> {
		const file = this.#file;
		if (!file) return 0;
		const ids = (await read(file)).map(Number).filter(Number.isSafeInteger);
		if (ids.length > 0 && existsSync(path)) {
			const db = await openNodeSqliteDatabase(path);
			try {
				await db.transaction((tx) => purge(tx, ids));
			} finally {
				await db.close();
			}
		}
		if (ids.length > 0) await writeFile(file, "[]", { mode: 0o600 });
		return ids.length;
	}
}

async function read(file: string): Promise<string[]> {
	let data: string;
	try {
		data = await readFile(file, "utf8");
	} catch (e) {
		if ((e as NodeJS.ErrnoException).code === "ENOENT") return [];
		throw e;
	}
	const ids: unknown = JSON.parse(data);
	return Array.isArray(ids) ? ids.map(String) : [];
}

/** purge deletes conversations with those they own, through their tasks too. */
async function purge(tx: SqliteExecutor, roots: number[]): Promise<void> {
	const conversations = new Set<number>();
	let next = roots;
	while (next.length > 0) {
		for (const id of next) conversations.add(id);
		const found: number[] = [];
		for (const id of next) {
			const owned = await tx.all<{ id: number }>(
				`SELECT id FROM conversations WHERE owner_conversation_id = ?
				 UNION SELECT c.id FROM conversations c JOIN tasks t ON c.owner_task_id = t.id WHERE t.conversation_id = ?`,
				id,
				id,
			);
			for (const o of owned) if (!conversations.has(Number(o.id))) found.push(Number(o.id));
		}
		next = found;
	}
	for (const id of conversations) {
		const documents = `SELECT id FROM documents WHERE (scope_kind = 'conversation' AND owner_id = ?)
			OR (scope_kind = 'task' AND owner_id IN (SELECT id FROM tasks WHERE conversation_id = ?))`;
		await tx.run(`DELETE FROM document_revisions WHERE document_id IN (${documents})`, id, id);
		await tx.run(`DELETE FROM documents WHERE id IN (${documents})`, id, id);
		for (const table of ["entries", "tasks", "submissions"]) {
			await tx.run(`DELETE FROM ${table} WHERE conversation_id = ?`, id);
		}
		await tx.run("DELETE FROM conversations WHERE id = ?", id);
	}
}

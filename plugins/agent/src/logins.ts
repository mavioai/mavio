// Logins relayed to an administrator: a provider's login flow runs in the
// plugin while the server polls it for the links, codes and prompts to
// show and answers its prompts.

import { randomUUID } from "node:crypto";
import type { AuthEvent, AuthPrompt, AuthType, Models } from "@earendil-works/pi-ai";

/** A login's state. */
export type LoginState = "running" | "succeeded" | "failed" | "canceled";

/** A prompt a login waits for an answer to. */
export interface PendingPrompt {
	readonly id: string;
	readonly prompt: AuthPrompt;
}

/** What a login has shown so far. */
export interface LoginView {
	readonly state: LoginState;
	readonly events: readonly AuthEvent[];
	readonly prompt?: PendingPrompt;
	readonly error?: string;
}

interface Login {
	state: LoginState;
	events: AuthEvent[];
	prompt?: PendingPrompt & { resolve(value: string): void; reject(error: Error): void };
	error?: string;
	abort: AbortController;
	ended?: number;
}

/** How long a finished login stays readable. */
const keep = 10 * 60_000;

/** The most events a login keeps. */
const maxEvents = 100;

/** Logins runs the login flows the server starts. */
export class Logins {
	readonly #models: Models;
	readonly #now: () => number;
	readonly #logins = new Map<string, Login>();

	constructor(models: Models, now: () => number = Date.now) {
		this.#models = models;
		this.#now = now;
	}

	/** Starts logging in to a provider with type and returns the login's ID. */
	start(providerId: string, type: AuthType): string {
		this.#expire();
		const id = randomUUID();
		const login: Login = { state: "running", events: [], abort: new AbortController() };
		this.#logins.set(id, login);
		const interaction = {
			signal: login.abort.signal,
			prompt: (prompt: AuthPrompt) =>
				new Promise<string>((resolve, reject) => {
					if (prompt.signal?.aborted) {
						reject(new Error("prompt canceled"));
						return;
					}
					const pending = { id: randomUUID(), prompt, resolve, reject };
					login.prompt = pending;
					const settle = () => {
						if (login.prompt === pending) login.prompt = undefined;
					};
					const abort = () => {
						settle();
						reject(new Error("prompt canceled"));
					};
					prompt.signal?.addEventListener("abort", abort, { once: true });
					pending.resolve = (value) => {
						prompt.signal?.removeEventListener("abort", abort);
						settle();
						resolve(value);
					};
					pending.reject = (error) => {
						prompt.signal?.removeEventListener("abort", abort);
						settle();
						reject(error);
					};
				}),
			notify: (event: AuthEvent) => {
				login.events.push(event);
				if (login.events.length > maxEvents) login.events.splice(0, login.events.length - maxEvents);
			},
		};
		this.#models.login(providerId, type, interaction).then(
			() => this.#end(login, "succeeded"),
			(error: unknown) =>
				this.#end(
					login,
					login.abort.signal.aborted ? "canceled" : "failed",
					error instanceof Error ? error.message : String(error),
				),
		);
		return id;
	}

	/** What a login has shown, or undefined for an unknown one. */
	get(id: string): LoginView | undefined {
		this.#expire();
		const login = this.#logins.get(id);
		if (!login) return undefined;
		return {
			state: login.state,
			events: [...login.events],
			...(login.prompt ? { prompt: { id: login.prompt.id, prompt: login.prompt.prompt } } : {}),
			...(login.error !== undefined ? { error: login.error } : {}),
		};
	}

	/**
	 * Answers a login's prompt; false when the login is unknown or no longer
	 * waits for that prompt.
	 */
	answer(id: string, promptId: string, value: string): boolean {
		const prompt = this.#logins.get(id)?.prompt;
		if (!prompt || prompt.id !== promptId) return false;
		prompt.resolve(value);
		return true;
	}

	/** Cancels a running login; false for an unknown one. */
	cancel(id: string): boolean {
		const login = this.#logins.get(id);
		if (!login) return false;
		if (login.state === "running") {
			login.abort.abort();
			login.prompt?.reject(new Error("login canceled"));
		}
		return true;
	}

	/** Cancels every running login. */
	close(): void {
		for (const id of this.#logins.keys()) this.cancel(id);
	}

	#end(login: Login, state: LoginState, error?: string): void {
		login.state = state;
		login.prompt = undefined;
		if (error !== undefined && state === "failed") login.error = error;
		login.ended = this.#now();
	}

	#expire(): void {
		const now = this.#now();
		for (const [id, login] of this.#logins) {
			if (login.ended !== undefined && now - login.ended > keep) this.#logins.delete(id);
		}
	}
}

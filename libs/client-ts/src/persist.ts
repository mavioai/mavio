// The query cache kept offline (docs/clients.md §4.1): what Home, the
// libraries and an item's details showed is kept in IndexedDB, one entry
// per sign-in, and shown at once when the web UI opens again, offline or
// while its servers answer. Only those queries are kept, of the servers
// signed in to; never a token, which the sessions in local storage hold.
// The images' signed URLs in the items stay, expiring on their own within
// a day and with the session, and are fetched anew with the items. The
// cache is dropped by another version of the web UI and on signing out.
import {
  type PersistedClient,
  type Persister,
  persistQueryClientRestore,
  persistQueryClientSubscribe,
} from "@tanstack/query-persist-client-core";
import { defaultShouldDehydrateQuery, type Query, type QueryClient } from "@tanstack/react-query";
import { createStore, del, get, keys, set } from "idb-keyval";

import { serverKey } from "./hooks.ts";
import type { SignIn } from "./servers.ts";

/** The kinds of a server's queries kept: Home, the libraries and the details of their items. */
export const keptKinds: ReadonlySet<string> = new Set([
  "home",
  "recommended",
  "hub",
  "libraries",
  "library",
  "items",
  "item",
  "children",
  "episodes",
  "similar",
  "extras",
  "copies",
  "person",
]);

/** How long a kept cache is shown at most, since it was last written. */
export const keptMaxAge = 7 * 24 * 60 * 60_000;

/** How long the kept kinds' queries stay in memory unused, so that they are kept even when not shown for a while. */
export const keptGcTime = 24 * 60 * 60_000;

/** How often the cache is written at most while it changes. */
const writeInterval = 1000;

/** The prefix of the entries kept. */
const prefix = "queries:";

/** Where the cache is kept: IndexedDB in the browser. */
export interface CacheStorage {
  get(key: string): Promise<unknown>;
  set(key: string, value: unknown): Promise<void>;
  del(key: string): Promise<void>;
  keys(): Promise<string[]>;
}

/** The sign-ins whose changes are followed: the servers' store. */
export interface SignIns {
  subscribe(listener: () => void): () => void;
  signInSnapshot(): SignIn | undefined;
}

/**
 * The entry of a sign-in's cache: a Mavio account's, or a standalone
 * server's user's; undefined when signed out.
 */
export function cacheScope(signIn: SignIn | undefined): string | undefined {
  if (!signIn) return undefined;
  if (signIn.account) return `${prefix}account/${signIn.account.id}`;
  const s = signIn.servers[0];
  return s ? `${prefix}user/${s.id}/${s.userId}` : undefined;
}

/** Whether a query is kept: one of the kept kinds, of a server signed in to, that succeeded. */
export function keepsQuery(query: Query, signIn: SignIn | undefined): boolean {
  const [root, serverId, kind] = query.queryKey;
  return (
    root === "mavio" &&
    typeof kind === "string" &&
    keptKinds.has(kind) &&
    !!signIn?.servers.some((s) => s.id === serverId) &&
    defaultShouldDehydrateQuery(query)
  );
}

/** IndexedDB, in a database of its own. */
export function indexedDBStorage(): CacheStorage {
  const store = createStore("mavio-cache", "queries");
  return {
    get: (key) => get(key, store),
    set: (key, value) => set(key, value, store),
    del: (key) => del(key, store),
    keys: async () => (await keys(store)).filter((k): k is string => typeof k === "string"),
  };
}

/** A persister of one entry that writes at most once per interval, the latest state; IndexedDB clones it as it is. */
function entryPersister(storage: CacheStorage, key: string): Persister & { cancel(): void } {
  let pending: PersistedClient | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  return {
    persistClient(client) {
      pending = client;
      timer ??= setTimeout(() => {
        timer = undefined;
        const c = pending;
        pending = undefined;
        // A cache that cannot be written is not kept; the web UI works on.
        if (c) storage.set(key, c).catch(() => undefined);
      }, writeInterval);
    },
    restoreClient: async () => (await storage.get(key)) as PersistedClient | undefined,
    removeClient: () => storage.del(key),
    cancel() {
      clearTimeout(timer);
      timer = undefined;
      pending = undefined;
    },
  };
}

/** Deletes every sign-in's cache. */
export async function clearKeptQueries(storage: CacheStorage): Promise<void> {
  const all = await storage.keys();
  await Promise.all(all.filter((k) => k.startsWith(prefix)).map((k) => storage.del(k)));
}

/**
 * Keeps the query cache of the sign-in: restores it, then writes it as it
 * changes, and follows the sign-in as it changes — another account or
 * user keeps a cache of their own, and signing out deletes them all.
 * restored settles once the first sign-in's cache is in the client, or
 * could not be read; stop stops following.
 */
export function keepQueries(
  qc: QueryClient,
  signIns: SignIns,
  options: { buster: string; storage?: CacheStorage },
): { restored: Promise<void>; stop(): void } {
  let opened: CacheStorage;
  try {
    opened = options.storage ?? indexedDBStorage();
  } catch {
    // No IndexedDB here, as in some private windows: nothing is kept.
    return { restored: Promise.resolve(), stop: () => undefined };
  }
  const storage = opened;
  const { buster } = options;
  let scope: string | undefined;
  let stopSaving = () => {};
  const follow = (): Promise<void> => {
    const signIn = signIns.signInSnapshot();
    for (const s of signIn?.servers ?? []) {
      for (const kind of keptKinds) qc.setQueryDefaults(serverKey(s.id, kind), { gcTime: keptGcTime });
    }
    const next = cacheScope(signIn);
    if (next === scope) return Promise.resolve();
    stopSaving();
    stopSaving = () => {};
    scope = next;
    if (!next) return clearKeptQueries(storage).catch(() => undefined);
    const persister = entryPersister(storage, next);
    let stopped = false;
    let unsubscribe = () => {};
    stopSaving = () => {
      stopped = true;
      unsubscribe();
      persister.cancel();
    };
    return persistQueryClientRestore({ queryClient: qc, persister, maxAge: keptMaxAge, buster })
      .catch(() => undefined)
      .then(() => {
        if (stopped) return;
        unsubscribe = persistQueryClientSubscribe({
          queryClient: qc,
          persister,
          buster,
          dehydrateOptions: { shouldDehydrateQuery: (q) => keepsQuery(q, signIns.signInSnapshot()) },
        });
      });
  };
  const restored = follow();
  const unsubscribe = signIns.subscribe(() => void follow());
  return {
    restored,
    stop() {
      unsubscribe();
      stopSaving();
    },
  };
}

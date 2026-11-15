import { QueryClient } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { serverKey } from "./hooks.ts";
import { type CacheStorage, cacheScope, keepQueries, keepsQuery, type SignIns } from "./persist.ts";
import type { ServerEntry, SignIn } from "./servers.ts";

const server = (id: string, userId = "u1"): ServerEntry => ({ id, name: id, version: "1", url: `https://${id}`, userId, userName: "Ann" });

/** IndexedDB's way of storing: a structured clone. */
function memoryStorage(): CacheStorage & { map: Map<string, unknown> } {
  const map = new Map<string, unknown>();
  return {
    map,
    get: async (k) => structuredClone(map.get(k)),
    set: async (k, v) => void map.set(k, structuredClone(v)),
    del: async (k) => void map.delete(k),
    keys: async () => [...map.keys()],
  };
}

function signIns(initial: SignIn | undefined): SignIns & { set(s: SignIn | undefined): void } {
  let current = initial;
  const listeners = new Set<() => void>();
  return {
    subscribe: (l) => (listeners.add(l), () => listeners.delete(l)),
    signInSnapshot: () => current,
    set(s) {
      current = s;
      for (const l of listeners) l();
    },
  };
}

const standalone = (s: ServerEntry): SignIn => ({ kind: "standalone", name: "Ann", servers: [s] });

describe("cacheScope", () => {
  it("is the account's, or the standalone server's user's", () => {
    expect(cacheScope(undefined)).toBeUndefined();
    expect(cacheScope({ kind: "mavioDev", name: "Ann", account: { id: "a1", name: "Ann" }, servers: [server("s1")] })).toBe("queries:account/a1");
    expect(cacheScope(standalone(server("s1", "u2")))).toBe("queries:user/s1/u2");
    expect(cacheScope({ kind: "standalone", name: "Ann", servers: [] })).toBeUndefined();
  });
});

describe("keepsQuery", () => {
  it("keeps Home, libraries and details of the servers signed in to, once they succeed", async () => {
    const qc = new QueryClient();
    const signIn = standalone(server("s1"));
    qc.setQueryData(serverKey("s1", "home"), [1]);
    qc.setQueryData(serverKey("s1", "item", "i1"), { id: "i1" });
    qc.setQueryData(serverKey("s1", "search", "up"), []);
    qc.setQueryData(serverKey("s1", "playback", "i1"), {});
    qc.setQueryData(serverKey("s2", "home"), [2]);
    qc.setQueryData(["other"], 1);
    await qc.prefetchQuery({ queryKey: serverKey("s1", "library", "l1"), queryFn: () => Promise.reject(new Error("down")) });
    const kept = qc
      .getQueryCache()
      .getAll()
      .filter((q) => keepsQuery(q, signIn))
      .map((q) => q.queryKey);
    expect(kept).toEqual([serverKey("s1", "home"), serverKey("s1", "item", "i1")]);
    expect(qc.getQueryCache().getAll().filter((q) => keepsQuery(q, undefined))).toEqual([]);
  });
});

describe("keepQueries", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("writes the sign-in's cache, restores it, and deletes it on signing out", async () => {
    const storage = memoryStorage();
    const ann = signIns(standalone(server("s1")));
    const qc = new QueryClient();
    const kept = keepQueries(qc, ann, { buster: "v1", storage });
    await kept.restored;
    qc.setQueryData(serverKey("s1", "home"), { hubs: [{ id: "h1", total: 3n, thumbhash: new Uint8Array([1, 2]) }] });
    qc.setQueryData(serverKey("s1", "search", "up"), ["found"]);
    // Kept queries outlive the default garbage collection.
    expect(qc.getQueryCache().find({ queryKey: serverKey("s1", "home") })?.gcTime).toBe(24 * 60 * 60_000);
    expect(storage.map.size).toBe(0);
    await vi.advanceTimersByTimeAsync(1000);
    expect([...storage.map.keys()]).toEqual(["queries:user/s1/u1"]);
    kept.stop();

    // Opened again: Home shows from the cache, the search does not.
    const again = new QueryClient();
    await keepQueries(again, signIns(standalone(server("s1"))), { buster: "v1", storage }).restored;
    expect(again.getQueryData(serverKey("s1", "home"))).toEqual({ hubs: [{ id: "h1", total: 3n, thumbhash: new Uint8Array([1, 2]) }] });
    expect(again.getQueryData(serverKey("s1", "search", "up"))).toBeUndefined();

    // Another user of the server has none; another version drops it.
    const bob = new QueryClient();
    await keepQueries(bob, signIns(standalone(server("s1", "u2"))), { buster: "v1", storage }).restored;
    expect(bob.getQueryData(serverKey("s1", "home"))).toBeUndefined();
    const newer = new QueryClient();
    const upgraded = signIns(standalone(server("s1")));
    const k = keepQueries(newer, upgraded, { buster: "v2", storage });
    await k.restored;
    expect(newer.getQueryData(serverKey("s1", "home"))).toBeUndefined();
    expect(storage.map.size).toBe(0);

    // Signing out deletes every cache, and writes nothing more.
    newer.setQueryData(serverKey("s1", "home"), { hubs: [] });
    await storage.set("queries:account/a1", {});
    upgraded.set(undefined);
    await vi.advanceTimersByTimeAsync(1000);
    expect(storage.map.size).toBe(0);
    k.stop();
  });
});

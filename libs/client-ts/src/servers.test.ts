import { SystemService } from "@mavio/proto/mavio/system/v1/system_pb";
import { afterEach, expect, test, vi } from "vitest";

import { memoryDeviceKey, refreshMessage } from "./device-key.ts";
import { isTotpRequired } from "./hooks.ts";
import { normalizeUrl, type ServerEntry, Servers, type Storage } from "./servers.ts";

afterEach(() => vi.unstubAllGlobals());

function memory(): Storage & { data: Map<string, string> } {
  const data = new Map<string, string>();
  return { data, getItem: (k) => data.get(k) ?? null, setItem: (k, v) => void data.set(k, v) };
}

/** The JSON body of a Connect call. */
function body(init: RequestInit): unknown {
  const b = init.body;
  if (!b) return {};
  return JSON.parse(typeof b === "string" ? b : new TextDecoder().decode(b as Uint8Array));
}

/** Answers Connect calls by method name. */
function serve(answers: Record<string, (body: unknown, init: RequestInit) => unknown>) {
  const fetch = vi.fn(async (url: string, init: RequestInit) => {
    const method = url.slice(url.lastIndexOf("/") + 1);
    const answer = answers[method];
    if (!answer) return Response.json({ code: "unimplemented", message: method }, { status: 404 });
    const out = await answer(body(init), init);
    return out instanceof Response ? out : Response.json(out);
  });
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

test("normalizes typed addresses", () => {
  const cases: [string, string][] = [
    ["192.168.1.2", "http://192.168.1.2:8686"],
    ["media.example.com/", "http://media.example.com:8686"],
    ["https://media.example.com/mavio/", "https://media.example.com/mavio"],
    ["http://den:9000", "http://den:9000"],
  ];
  for (const [input, want] of cases) expect(normalizeUrl(input), input).toBe(want);
  expect(() => normalizeUrl("ftp://den")).toThrow();
  expect(() => normalizeUrl("  ")).toThrow();
});

test("signs in to several servers, each with its own token", async () => {
  const fetch = serve({});
  fetch.mockImplementation(async (url: string, init: RequestInit) => {
    const method = url.slice(url.lastIndexOf("/") + 1);
    const server = url.startsWith("http://den") ? "den" : "attic";
    if (method === "GetAuthInfo") return Response.json({ serverId: server, serverName: server, version: "1.0" });
    if (method === "Login") {
      return Response.json({ accessToken: `${server}-token`, user: { id: "u", name: (body(init) as { name: string }).name } });
    }
    return Response.json({});
  });
  const servers = new Servers(memory(), "http://den:8686");
  await servers.signIn("den", "ann", "pw");
  await servers.signIn("https://attic.example.com", "bob", "pw");
  expect(servers.snapshot().map((s) => [s.id, s.url, s.userName, s.token])).toEqual([
    ["den", "http://den:8686", "ann", "den-token"],
    ["attic", "https://attic.example.com", "bob", "attic-token"],
  ]);
  // The list, without tokens, follows the user in the serving server's preferences.
  const saved = fetch.mock.calls.filter(([url]) => url.endsWith("/SetDisplayPreferences")).at(-1)!;
  expect(saved[0]).toBe("http://den:8686/mavio.user.v1.DisplayPreferencesService/SetDisplayPreferences");
  const list = JSON.parse((body(saved[1]) as { values: { list: string } }).values.list) as ServerEntry[];
  expect(list.map((s) => [s.id, s.token])).toEqual([
    ["den", undefined],
    ["attic", undefined],
  ]);
});

test("rejects an address that is not a Mavio server", async () => {
  serve({ GetAuthInfo: () => ({}) });
  await expect(new Servers(memory(), "").probe("example.com")).rejects.toThrow(/not a Mavio server/);
});

test("a rejected token signs the user out of that server alone", async () => {
  const storage = memory();
  const entry = (id: string): ServerEntry => ({
    id,
    name: id,
    version: "1",
    url: `https://${id}.example.com`,
    userId: "u",
    userName: "ann",
    token: `${id}-token`,
  });
  storage.setItem("mavio.servers", JSON.stringify([entry("a"), entry("b")]));
  const servers = new Servers(storage, "https://a.example.com");
  let changes = 0;
  servers.subscribe(() => changes++);
  serve({
    GetHealth: (_, init) =>
      new Headers(init.headers).get("authorization") === "Bearer b-token"
        ? Response.json({ code: "unauthenticated", message: "expired" }, { status: 401 })
        : {},
  });
  await servers.client(SystemService, servers.get("a")!).getHealth({});
  await expect(servers.client(SystemService, servers.get("b")!).getHealth({})).rejects.toThrow(/expired/);
  expect(servers.signedIn().map((s) => s.id)).toEqual(["a"]);
  expect(servers.get("b")?.userName).toBe("ann");
  expect(changes).toBe(1);
  expect(JSON.parse(storage.data.get("mavio.servers")!)[1].token).toBeUndefined();
});

test("orders servers and adds those of other browsers signed out", async () => {
  const storage = memory();
  const mk = (id: string, token?: string): ServerEntry => ({
    id,
    name: id,
    version: "1",
    url: `http://${id}:8686`,
    userId: "u",
    userName: "ann",
    token,
  });
  storage.setItem("mavio.servers", JSON.stringify([mk("home", "t"), mk("b", "tb")]));
  const servers = new Servers(storage, "http://home:8686");
  serve({
    GetDisplayPreferences: () => ({
      preferences: { values: { list: JSON.stringify([mk("c"), mk("home"), mk("b")]) } },
    }),
    SetDisplayPreferences: () => ({}),
  });
  await servers.load();
  expect(servers.snapshot().map((s) => [s.id, !!s.token])).toEqual([
    ["c", false],
    ["home", true],
    ["b", true],
  ]);
  servers.order(["b", "home"]);
  expect(servers.snapshot().map((s) => s.id)).toEqual(["b", "home", "c"]);
});

const base64 = (b: Uint8Array) => btoa(String.fromCharCode(...b));
const unbase64 = (s: string) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0));

test("refreshes the access token with the device key", async () => {
  const key = memoryDeviceKey();
  const publicKey = await crypto.subtle.importKey("raw", (await key.publicKey()) as BufferSource, "Ed25519", false, [
    "verify",
  ]);
  const sessionId = "01890a5d-ac96-774b-bcce-b302099a8057";
  const nonce = crypto.getRandomValues(new Uint8Array(32));
  const storage = memory();
  const soon = Date.now() + 60_000;
  storage.setItem(
    "mavio.servers",
    JSON.stringify([
      { id: "a", name: "a", version: "1", url: "https://a.example.com", userId: "u", userName: "ann", sessionId, token: "old", expires: soon },
    ] satisfies ServerEntry[]),
  );
  const servers = new Servers(storage, "https://a.example.com", key);
  const calls: string[] = [];
  serve({
    StartRefresh: (b) => (calls.push(`start ${(b as { sessionId: string }).sessionId}`), { nonce: base64(nonce) }),
    RefreshSession: async (b) => {
      const r = b as { nonce: string; signature: string };
      const ok = await crypto.subtle.verify(
        "Ed25519",
        publicKey,
        unbase64(r.signature) as BufferSource,
        refreshMessage(sessionId, unbase64(r.nonce)) as BufferSource,
      );
      calls.push(`refresh ${ok}`);
      return ok
        ? { accessToken: "new", accessTokenExpireTime: new Date(Date.now() + 3_600_000).toISOString() }
        : Response.json({ code: "unauthenticated" }, { status: 401 });
    },
    GetHealth: (_, init) => (calls.push(`health ${new Headers(init.headers).get("authorization")}`), {}),
  });
  await servers.client(SystemService, servers.get("a")!).getHealth({});
  expect(calls).toEqual([`start ${sessionId}`, "refresh true", "health Bearer new"]);
  expect(servers.get("a")?.expires).toBeGreaterThan(soon);
});

test("an expired token is refreshed and the call made again", async () => {
  const storage = memory();
  storage.setItem(
    "mavio.servers",
    JSON.stringify([
      { id: "a", name: "a", version: "1", url: "https://a.example.com", userId: "u", userName: "ann", sessionId: "01890a5d-ac96-774b-bcce-b302099a8057", token: "old" },
    ] satisfies ServerEntry[]),
  );
  const servers = new Servers(storage, "https://a.example.com", memoryDeviceKey());
  serve({
    StartRefresh: () => ({ nonce: base64(new Uint8Array(32)) }),
    RefreshSession: () => ({ accessToken: "new" }),
    GetHealth: (_, init) =>
      new Headers(init.headers).get("authorization") === "Bearer new"
        ? {}
        : Response.json({ code: "unauthenticated", message: "access token expired" }, { status: 401 }),
  });
  await servers.client(SystemService, servers.get("a")!).getHealth({});
  expect(servers.get("a")?.token).toBe("new");
});

test("signs in with a TOTP code and switches users in place", async () => {
  const logins: unknown[] = [];
  serve({
    GetAuthInfo: () => ({ serverId: "den", serverName: "Den", version: "1.0", mode: "ACCOUNT_MODE_STANDALONE" }),
    Login: (b) => {
      logins.push(b);
      if (!(b as { totpCode?: string }).totpCode) {
        return Response.json({ code: "failed_precondition", message: "a TOTP code is needed" }, { status: 400 });
      }
      return { accessToken: "t", session: { id: "s" }, user: { id: "ann", name: "ann" } };
    },
    SwitchUser: (b) => ({ user: { id: (b as { userId: string }).userId, name: "kid" } }),
  });
  const servers = new Servers(memory(), "http://den:8686", memoryDeviceKey());
  const err = await servers.signIn("den", "ann", "pw").catch((e: unknown) => e);
  expect(isTotpRequired(err)).toBe(true);
  const entry = await servers.signIn("den", "ann", "pw", "123456");
  expect(logins.map((b) => (b as { totpCode?: string }).totpCode)).toEqual([undefined, "123456"]);
  expect(entry.mode).toBe("standalone");

  const kid = "0193a5b0-0000-7000-8000-00000000000c";
  await servers.switchUser(entry, kid, "1234");
  expect(servers.get("den")).toMatchObject({ userId: kid, userName: "kid", token: "t", sessionId: "s" });
});

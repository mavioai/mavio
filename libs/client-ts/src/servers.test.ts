import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { SystemService } from "@mavio/proto/mavio/system/v1/system_pb";
import { afterEach, expect, test, vi } from "vitest";

import { assertionMessage, memoryDeviceKey, refreshMessage, renewalMessage } from "./device-key.ts";
import { isTotpRequired } from "./hooks.ts";
import { type ServerEntry, Servers, shareableUrl, type Storage } from "./servers.ts";

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

const api = "https://mavio.dev/api";
const den = { serverId: "den", serverName: "Den", version: "1.0", mode: "ACCOUNT_MODE_MAVIO_DEV", mavioDevUrl: api };
const standalone = { serverId: "den", serverName: "Den", version: "1.0", mode: "ACCOUNT_MODE_STANDALONE" };

/** A mavio.dev and servers answering by URL: OAuth endpoints by name, Connect calls by method. */
function world(answers: Record<string, (body: unknown, init: RequestInit, url: string) => unknown>) {
  const calls: string[] = [];
  const fetch = vi.fn(async (url: string, init: RequestInit) => {
    const oauth = url.startsWith(`${api}/oauth/`) ? `oauth/${url.slice(`${api}/oauth/`.length)}` : undefined;
    const method = oauth ?? url.slice(url.lastIndexOf("/") + 1);
    calls.push(`${new URL(url).host} ${method}`);
    const answer = answers[method];
    if (!answer) return Response.json({ code: "unimplemented", message: method }, { status: 404 });
    const b = oauth ? Object.fromEntries(init.body as URLSearchParams) : body(init);
    const out = await answer(b, init, url);
    return out instanceof Response ? out : Response.json(out);
  });
  vi.stubGlobal("fetch", fetch);
  return calls;
}

const token = (account = { id: "acct-1", userName: "ann", displayName: "Ann" }) => ({
  access_token: "mds1.access",
  token_type: "Bearer",
  expires_in: 600,
  session_id: "sess-1",
  account,
});

const signedInWithAssertion = (_: unknown, __: RequestInit, url: string) => ({
  accessToken: `${new URL(url).hostname}-token`,
  session: { id: "0193a5b0-0000-7000-8000-000000000001" },
  user: { id: "u", name: "ann" },
});

test("rejects an address that is not a Mavio server", async () => {
  serve({ GetAuthInfo: () => ({}) });
  await expect(new Servers(memory(), "http://den").probe("http://example.com")).rejects.toThrow(/not a Mavio server/);
});

test("signs a standalone server's user in with a TOTP code and switches users in place", async () => {
  const logins: unknown[] = [];
  serve({
    GetAuthInfo: () => standalone,
    Login: (b) => {
      logins.push(b);
      if (!(b as { totpCode?: string }).totpCode) {
        return Response.json({ code: "failed_precondition", message: "a TOTP code is needed" }, { status: 400 });
      }
      return { accessToken: "t", session: { id: "s" }, user: { id: "ann", name: "ann" } };
    },
    SwitchUser: (b) => ({ user: { id: (b as { userId: string }).userId, name: "kid" } }),
  });
  const servers = new Servers(memory(), "http://den:8686", { key: memoryDeviceKey() });
  const err = await servers.signInWithPassword("ann", "pw").catch((e: unknown) => e);
  expect(isTotpRequired(err)).toBe(true);
  const entry = await servers.signInWithPassword("ann", "pw", "123456");
  expect(logins.map((b) => (b as { totpCode?: string }).totpCode)).toEqual([undefined, "123456"]);
  expect(entry).toMatchObject({ url: "http://den:8686", mode: "standalone" });
  expect(servers.signInSnapshot()).toMatchObject({ kind: "standalone", name: "ann" });

  const kid = "0193a5b0-0000-7000-8000-00000000000c";
  await servers.switchUser(entry, kid, "1234");
  expect(servers.get("den")).toMatchObject({ userId: kid, userName: "kid", token: "t", sessionId: "s" });
});

test("a rejected token signs a standalone server's user out", async () => {
  const storage = memory();
  const entry: ServerEntry = { id: "a", name: "a", version: "1", url: "https://a.example.com", userId: "u", userName: "ann", token: "a-token" };
  storage.setItem("mavio.servers", JSON.stringify([entry]));
  const servers = new Servers(storage, "https://a.example.com");
  serve({ GetHealth: () => Response.json({ code: "unauthenticated", message: "expired" }, { status: 401 }) });
  await expect(servers.client(SystemService, servers.get("a")!).getHealth({})).rejects.toThrow(/expired/);
  expect(servers.signedIn()).toEqual([]);
  expect(servers.signInSnapshot()).toBeUndefined();
});

test("signs a Mavio account in through mavio.dev to each of its servers", async () => {
  const key = memoryDeviceKey();
  let exchanged: Record<string, string> = {};
  const signed: { assertion: string; signature: string }[] = [];
  const started: Record<string, unknown>[] = [];
  const calls = world({
    StartSignIn: (b) => (started.push(b as Record<string, unknown>), { url: `https://mavio.dev/authorize?request_uri=r${started.length}` }),
    GetAuthInfo: (_, __, url) => (url.includes("attic") ? { ...den, serverId: "attic", serverName: "Attic" } : den),
    "oauth/token": (b) => ((exchanged = b as Record<string, string>), token()),
    ListServers: () => ({
      servers: [
        { id: "den", name: "Den", version: "1.0", connections: ["http://192.168.1.2:8686"] },
        { id: "attic", name: "Attic", version: "1.0", connections: ["http://unreachable:8686", "http://attic:8686"] },
      ],
    }),
    CreateAssertion: (b) => ({ assertion: `assertion-for-${(b as { serverId: string }).serverId}` }),
    SignInWithAssertion: (b, init, url) => (signed.push(b as (typeof signed)[0]), signedInWithAssertion(b, init, url)),
  });
  const servers = new Servers(memory(), "http://den:8686", { key });
  // The serving server pushes the request to mavio.dev.
  expect(await servers.startSignIn("/library")).toBe("https://mavio.dev/authorize?request_uri=r1");
  expect(started[0]).toMatchObject({ redirectUri: "http://den:8686/auth/callback" });
  expect(calls).toContain("den:8686 StartSignIn");

  await expect(servers.finishCallback(new URLSearchParams({ code: "c", state: "another" }))).rejects.toThrow(/not started here/);
  await servers.startSignIn("/library");
  // Finishing it twice, as React's StrictMode does, spends the code once.
  const params = new URLSearchParams({ code: "the code", state: started[1].state as string });
  const [back, again] = await Promise.all([servers.finishCallback(params), servers.finishCallback(params)]);
  expect([back, again]).toEqual(["/library", "/library"]);
  expect(calls.filter((c) => c.endsWith("oauth/token"))).toHaveLength(1);
  expect(exchanged).toMatchObject({ grant_type: "authorization_code", code: "the code", client_id: "den" });
  expect(servers.account()).toEqual({ id: "acct-1", name: "Ann" });
  // The serving server at its own address, the other at the first URL that answers as it.
  expect(servers.snapshot().map((s) => [s.id, s.url, s.token])).toEqual([
    ["den", "http://den:8686", "den-token"],
    ["attic", "http://attic:8686", "attic-token"],
  ]);
  expect(servers.signInSnapshot()).toMatchObject({ kind: "mavioDev", name: "Ann" });
  const pub = await crypto.subtle.importKey("raw", (await key.publicKey()).slice(), { name: "Ed25519" }, false, ["verify"]);
  for (const s of signed) {
    const signature = Uint8Array.from(atob(s.signature), (c) => c.charCodeAt(0));
    expect(await crypto.subtle.verify({ name: "Ed25519" }, pub, signature, assertionMessage(s.assertion).slice())).toBe(true);
  }
  expect(calls).toContain("mavio.dev ListServers");

  // Signing out ends every session and the account's at mavio.dev.
  calls.length = 0;
  await servers.signOut();
  expect(calls.toSorted()).toEqual(["attic:8686 Logout", "den:8686 Logout", "mavio.dev oauth/revoke"]);
  expect(servers.snapshot()).toEqual([]);
  expect(servers.account()).toBeUndefined();
});

test("app.mavio.dev signs in as the app and reaches servers over HTTPS only", async () => {
  vi.stubGlobal("location", { protocol: "https:" });
  let client = "";
  world({
    "oauth/token": (b) => ((client = (b as { client_id: string }).client_id), token()),
    ListServers: () => ({ servers: [{ id: "den", name: "Den", connections: ["http://192.168.1.2:8686"] }] }),
  });
  const servers = new Servers(memory(), "https://app.mavio.dev", { mavioDevUrl: api, key: memoryDeviceKey() });
  expect(servers.app).toBe(true);
  const page = new URL(await servers.startSignIn());
  expect(page.searchParams.get("client_id")).toBe("app");
  await servers.finishCallback(new URLSearchParams({ code: "c", state: page.searchParams.get("state")! }));
  expect(client).toBe("app");
  expect(servers.snapshot().map((s) => [s.id, s.url, !!s.token])).toEqual([["den", "", false]]);
});

test("a television signs in with a link code", async () => {
  let polls = 0;
  world({
    GetAuthInfo: () => den,
    "oauth/device": (b) => {
      expect(b).toMatchObject({ client_id: "den" });
      return {
        device_code: "dc",
        user_code: "ABCD-EFGH",
        verification_uri: "https://mavio.dev/link",
        verification_uri_complete: "https://mavio.dev/link?code=ABCD-EFGH",
        expires_in: 600,
        interval: 5,
      };
    },
    "oauth/token": () =>
      ++polls < 2 ? Response.json({ error: "authorization_pending", error_description: "not yet" }, { status: 400 }) : token(),
    ListServers: () => ({ servers: [] }),
  });
  const servers = new Servers(memory(), "http://den:8686", { key: memoryDeviceKey() });
  const link = await servers.startLink();
  expect(link).toMatchObject({ userCode: "ABCD-EFGH", interval: 5000 });
  expect(await servers.pollLink(link)).toBe(false);
  expect(await servers.pollLink(link)).toBe(true);
  expect(servers.account()?.name).toBe("Ann");
});

test("renews mavio.dev's access token with the device key, and signs out once it is refused", async () => {
  const key = memoryDeviceKey();
  const pub = await crypto.subtle.importKey("raw", (await key.publicKey()).slice(), { name: "Ed25519" }, false, ["verify"]);
  const storage = memory();
  storage.setItem(
    "mavio.account",
    JSON.stringify({ api, clientId: "den", sessionId: "sess-1", token: "old", expires: Date.now() + 10_000, account: { id: "acct-1", name: "Ann" } }),
  );
  let refuse = false;
  const auth: string[] = [];
  world({
    "oauth/token": async (b) => {
      const r = b as Record<string, string>;
      const signature = Uint8Array.from(atob(r.signature.replace(/-/g, "+").replace(/_/g, "/")), (c) => c.charCodeAt(0));
      const ok = await crypto.subtle.verify({ name: "Ed25519" }, pub, signature, renewalMessage("sess-1", Number(r.timestamp)).slice());
      if (refuse || !ok) return Response.json({ error: "invalid_grant" }, { status: 400 });
      return { access_token: "new", expires_in: 600, session_id: "sess-1" };
    },
    GetAccount: (_, init) => (auth.push(new Headers(init.headers).get("authorization") ?? ""), { account: { id: "acct-1" } }),
    "oauth/revoke": () => ({}),
  });
  const servers = new Servers(storage, "http://den:8686", { key });
  await servers.mavioDev(AccountService).getAccount({});
  expect(auth).toEqual(["Bearer new"]);
  // Renewed no sooner than needed.
  await servers.mavioDev(AccountService).getAccount({});
  expect(auth).toEqual(["Bearer new", "Bearer new"]);

  storage.setItem(
    "mavio.account",
    JSON.stringify({ api, clientId: "den", sessionId: "sess-1", token: "old", expires: Date.now(), account: { id: "acct-1", name: "Ann" } }),
  );
  refuse = true;
  const later = new Servers(storage, "http://den:8686", { key });
  await expect(later.mavioDev(AccountService).getAccount({})).rejects.toThrow(/signed out/);
  expect(later.account()).toBeUndefined();
});

test("a server's ended session signs the Mavio account in again", async () => {
  const storage = memory();
  storage.setItem(
    "mavio.account",
    JSON.stringify({ api, clientId: "den", sessionId: "sess-1", token: "t", expires: Date.now() + 600_000, account: { id: "acct-1", name: "Ann" } }),
  );
  const entry: ServerEntry = { id: "den", name: "Den", version: "1", url: "http://den:8686", userId: "u", userName: "ann", mode: "mavioDev", token: "stale" };
  storage.setItem("mavio.servers", JSON.stringify([entry]));
  world({
    GetHealth: (_, init) =>
      new Headers(init.headers).get("authorization") === "Bearer den-token" ? {} : Response.json({ code: "unauthenticated" }, { status: 401 }),
    CreateAssertion: () => ({ assertion: "a" }),
    SignInWithAssertion: signedInWithAssertion,
  });
  const servers = new Servers(storage, "http://den:8686", { key: memoryDeviceKey() });
  await servers.client(SystemService, servers.get("den")!).getHealth({});
  expect(servers.get("den")?.token).toBe("den-token");
});

test("orders servers", () => {
  const storage = memory();
  const mk = (id: string): ServerEntry => ({ id, name: id, version: "1", url: `http://${id}:8686`, userId: "u", userName: "ann", token: "t" });
  storage.setItem("mavio.servers", JSON.stringify([mk("a"), mk("b"), mk("c")]));
  const servers = new Servers(storage, "http://a:8686");
  servers.order(["c", "a"]);
  expect(servers.snapshot().map((s) => s.id)).toEqual(["c", "a", "b"]);
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
  const servers = new Servers(storage, "https://a.example.com", { key });
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
  const servers = new Servers(storage, "https://a.example.com", { key: memoryDeviceKey() });
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

test("a server's address for others is the best it published", () => {
  const cases: [string, Pick<ServerEntry, "url" | "urls">, string][] = [
    ["the first published", { url: "http://localhost:5174", urls: ["https://a.mavio.site:8920", "http://192.168.1.10:8686"] }, "https://a.mavio.site:8920"],
    ["a standalone server as reached", { url: "https://media.example.com" }, "https://media.example.com"],
  ];
  for (const [name, s, want] of cases) expect(shareableUrl(s), name).toBe(want);
});

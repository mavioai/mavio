import { type Client, Code } from "@connectrpc/connect";
import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { describe, expect, test } from "vitest";

import { renewGrant, s256 } from "../src/server/authorization.ts";
import { renewalMessage } from "../src/server/device-sessions.ts";
import { base64url } from "../src/server/ids.ts";
import { apiURL, claimed, code, newServer, newSite } from "./harness.ts";

const local = "http://192.168.1.20:8686";
const redirect = `${local}/auth/callback`;
const verifier = "a verifier of the web interface, long enough to be one";

type Site = Awaited<ReturnType<typeof newSite>>;

/** newDevice returns a device's Ed25519 key and a signer of its renewals. */
async function newDevice() {
  const keys = (await crypto.subtle.generateKey({ name: "Ed25519" }, false, ["sign", "verify"])) as CryptoKeyPair;
  const publicKey = new Uint8Array(await crypto.subtle.exportKey("raw", keys.publicKey));
  return {
    publicKey,
    async sign(sessionId: string, timestamp: number) {
      return base64url(new Uint8Array(await crypto.subtle.sign({ name: "Ed25519" }, keys.privateKey, renewalMessage(sessionId, timestamp))));
    },
  };
}

/**
 * pushed has a server push its web UI's authorization request, which the
 * account of asAccount answers, and returns where the browser goes next.
 */
async function pushed(
  site: Site,
  server: Awaited<ReturnType<typeof newServer>>,
  asAccount: Client<typeof AccountService>,
  deviceKey: Uint8Array,
  fields: { claim?: boolean; approve?: boolean } = {},
): Promise<URL> {
  const { authorizationRequest } = await site.serverClient(server).createAuthorizationRequest({
    publicKey: server.publicKey,
    redirectUri: redirect,
    codeChallenge: await s256(verifier),
    deviceKey,
    claim: fields.claim,
  });
  const { redirectUrl } = await asAccount.answerAuthorizationRequest({
    authorizationRequest,
    approve: fields.approve ?? true,
  });
  return new URL(redirectUrl);
}

function post(site: Site, path: string, fields: Record<string, string>, origin = local) {
  return site.fetch(new Request(`${apiURL}/oauth/${path}`, { method: "POST", headers: { origin }, body: new URLSearchParams(fields) }));
}

describe("authorization codes", () => {
  test("are exchanged once with the verifier for a session of the device key", async () => {
    const { site, alice, asAlice, server } = await claimed();
    const device = await newDevice();
    const newCode = async () => (await pushed(site, server, asAlice, device.publicKey)).searchParams.get("code") ?? "";
    const exchange = (c: string, fields: Record<string, string> = {}, origin = local) =>
      post(site, "token", { grant_type: "authorization_code", code: c, client_id: server.id, redirect_uri: redirect, code_verifier: verifier, ...fields }, origin);

    // The code of another origin, client or verifier is refused, and spent.
    const first = await newCode();
    let res = await exchange(first, { code_verifier: "another verifier" });
    expect(res.status).toBe(400);
    expect((await res.json()).error).toBe("invalid_grant");
    expect((await exchange(first)).status).toBe(400);
    expect((await exchange(await newCode(), {}, "https://evil.example")).status).toBe(400);
    expect((await exchange(await newCode(), { client_id: "app" })).status).toBe(400);

    const third = await newCode();
    res = await exchange(third);
    expect(res.status).toBe(200);
    expect(res.headers.get("access-control-allow-origin")).toBe(local);
    const body = await res.json();
    expect(body).toMatchObject({ token_type: "Bearer", expires_in: 600, account: { id: alice.id, userName: "alice" } });
    expect((await exchange(third)).status).toBe(400);

    // The access token calls the account's services from the web UI's origin.
    const asDevice = site.client(AccountService, { authorization: `Bearer ${body.access_token}`, origin: local });
    expect((await asDevice.getAccount({})).account?.id).toBe(alice.id);
    const { assertion } = await asDevice.createAssertion({ serverId: server.id, deviceKey: device.publicKey });
    const claims = JSON.parse(Buffer.from(assertion.split(".")[1], "base64url").toString());
    expect(claims).toMatchObject({ aud: server.id, sub: alice.id, device_key: base64url(device.publicKey) });
    const preflight = await site.fetch(
      new Request(`${apiURL}/mavio.cloud.v1.AccountService/GetAccount`, { method: "OPTIONS", headers: { origin: local } }),
    );
    expect(preflight.status).toBe(204);
    expect(preflight.headers.get("access-control-allow-headers")).toContain("authorization");
  });

  test("sign in to a server's web UI only its administrator", async () => {
    const { site, asAlice, server } = await claimed();
    const bob = await site.signUp("bob");
    const asBob = site.client(AccountService, bob.headers);
    await asAlice.grantAccess({ serverId: server.id, userName: "bob", access: { allLibraries: true } });
    await asBob.acceptInvitation({ serverId: server.id });
    expect(await code(pushed(site, server, asBob, new Uint8Array(32)))).toBe(Code.PermissionDenied);
    expect(await code(pushed(site, server, site.client(AccountService), new Uint8Array(32)))).toBe(Code.Unauthenticated);
    const api = site.serverClient(server);
    const req = { publicKey: server.publicKey, redirectUri: redirect, codeChallenge: await s256(verifier), deviceKey: new Uint8Array(32) };
    expect(await code(api.createAuthorizationRequest({ ...req, redirectUri: "javascript:alert(1)" }))).toBe(Code.InvalidArgument);
    expect(await code(api.createAuthorizationRequest({ ...req, codeChallenge: "short" }))).toBe(Code.InvalidArgument);
    // A server not claimed yet pushes only requests that claim it.
    const other = await newServer();
    expect(await code(site.serverClient(other).createAuthorizationRequest({ ...req, publicKey: other.publicKey }))).toBe(
      Code.FailedPrecondition,
    );
    const request = (await api.createAuthorizationRequest(req)).authorizationRequest;
    expect(await asBob.getAuthorizationRequest({ authorizationRequest: request })).toMatchObject({
      serverId: server.id,
      serverName: "Home",
      claim: false,
      claimed: true,
      administrator: false,
    });
  });

  test("sign any account in to app.mavio.dev", async () => {
    const site = await newSite();
    const bob = await site.signUp("bob");
    const app = "https://app.mavio.dev/auth/callback";
    const { code: c } = await site
      .client(AccountService, bob.headers)
      .createAuthorizationCode({ redirectUri: app, codeChallenge: await s256(verifier), deviceKey: new Uint8Array(32).fill(1) });
    const res = await post(
      site,
      "token",
      { grant_type: "authorization_code", code: c, client_id: "app", redirect_uri: app, code_verifier: verifier },
      "https://app.mavio.dev",
    );
    expect(res.status).toBe(200);
    expect((await res.json()).account.id).toBe(bob.id);
    const config = await site.fetch(new Request(`${apiURL}/config`));
    expect(await config.json()).toEqual({ appURL: "https://app.mavio.dev" });
  });
});

describe("device sessions", () => {
  async function session() {
    const site = await newSite();
    const alice = await site.signUp("alice");
    const device = await newDevice();
    const { code: c } = await site
      .client(AccountService, alice.headers)
      .createAuthorizationCode({ redirectUri: "https://app.mavio.dev/auth/callback", codeChallenge: await s256(verifier), deviceKey: device.publicKey });
    const res = await post(site, "token", {
      grant_type: "authorization_code",
      code: c,
      client_id: "app",
      redirect_uri: "https://app.mavio.dev/auth/callback",
      code_verifier: verifier,
    }, "https://app.mavio.dev");
    const body = await res.json();
    return { site, alice, device, sid: body.session_id as string, token: body.access_token as string };
  }

  test("renew with a signature of a later time", async () => {
    const { site, device, sid } = await session();
    const renew = async (ts: number, signature?: string) =>
      post(site, "token", { grant_type: renewGrant, session_id: sid, timestamp: String(ts), signature: signature ?? (await device.sign(sid, ts)) });
    const now = Date.now();
    let res = await renew(now);
    expect(res.status).toBe(200);
    const token = (await res.json()).access_token;
    expect((await site.client(AccountService, { authorization: `Bearer ${token}` }).getAccount({})).account?.userName).toBe("alice");
    // A replayed, earlier, distant or foreign signature is refused.
    expect((await renew(now)).status).toBe(400);
    expect((await renew(now - 1)).status).toBe(400);
    expect((await renew(now + 10 * 60_000)).status).toBe(400);
    expect((await renew(now + 1, await (await newDevice()).sign(sid, now + 1))).status).toBe(400);
    expect((await renew(now + 2)).status).toBe(200);
  });

  test("end when revoked or when the account reports a compromise", async () => {
    let { site, alice, token } = await session();
    const asDevice = () => site.client(AccountService, { authorization: `Bearer ${token}` });
    expect((await post(site, "revoke", { token })).status).toBe(200);
    expect(await code(asDevice().getAccount({}))).toBe(Code.Unauthenticated);

    ({ site, alice, token } = await session());
    await site.client(AccountService, alice.headers).reportCompromised({});
    expect(await code(asDevice().getAccount({}))).toBe(Code.Unauthenticated);
    expect(await code(site.client(AccountService, { authorization: `Bearer ${token.slice(0, -2)}xx` }).getAccount({}))).toBe(
      Code.Unauthenticated,
    );
  });
});

describe("authorization requests that claim the server", () => {
  test("claim it for the account that approves, which signs in", async () => {
    const site = await newSite();
    const alice = await site.signUp("alice");
    const asAlice = site.client(AccountService, alice.headers);
    const server = await newServer();
    const api = site.serverClient(server);
    const device = await newDevice();
    const created = await api.createAuthorizationRequest({
      publicKey: server.publicKey,
      name: "Home",
      version: "1.0",
      redirectUri: redirect,
      codeChallenge: await s256(verifier),
      deviceKey: device.publicKey,
      state: "s1",
      claim: true,
    });
    expect(created.url).toBe(`http://localhost:3000/authorize?request_uri=${created.authorizationRequest}`);
    const claim = { authorizationRequest: created.authorizationRequest, publicKey: server.publicKey };
    expect(await code(api.claimServer(claim))).toBe(Code.FailedPrecondition);
    expect(await asAlice.getAuthorizationRequest({ authorizationRequest: created.authorizationRequest })).toMatchObject({
      serverId: server.id,
      serverName: "Home",
      redirectUri: redirect,
      claim: true,
      claimed: false,
      administrator: false,
    });

    const { redirectUrl } = await asAlice.answerAuthorizationRequest({ authorizationRequest: created.authorizationRequest, approve: true });
    const to = new URL(redirectUrl);
    expect(to.origin + to.pathname).toBe(redirect);
    expect(to.searchParams.get("state")).toBe("s1");
    expect(await code(asAlice.answerAuthorizationRequest({ authorizationRequest: created.authorizationRequest, approve: true }))).toBe(
      Code.NotFound,
    );

    const resp = await api.claimServer(claim);
    expect(resp.administrator?.id).toBe(alice.id);
    expect(await code(api.claimServer(claim))).toBe(Code.InvalidArgument);
    const res = await post(site, "token", {
      grant_type: "authorization_code",
      code: to.searchParams.get("code") ?? "",
      client_id: server.id,
      redirect_uri: redirect,
      code_verifier: verifier,
    });
    expect(res.status).toBe(200);

    await api.publishConnections({ urls: ["https://192-168-1-20.x.mavio.site:8920", local] });
    const { servers } = await asAlice.listServers({});
    expect(servers[0].connections).toEqual(["https://192-168-1-20.x.mavio.site:8920", local]);
  });

  test("declined send the browser back with an error", async () => {
    const site = await newSite();
    const alice = await site.signUp("alice");
    const server = await newServer();
    const to = await pushed(site, server, site.client(AccountService, alice.headers), new Uint8Array(32), { claim: true, approve: false });
    expect(to.searchParams.get("error")).toBe("access_denied");
    expect((await site.client(AccountService, alice.headers).listServers({})).servers).toHaveLength(0);
  });
});

describe("link codes", () => {
  test("sign a television in once an account approves", async () => {
    const site = await newSite();
    const alice = await site.signUp("alice");
    const asAlice = site.client(AccountService, alice.headers);
    const device = await newDevice();
    let res = await post(site, "device", { client_id: "dev.mavio.tvos", device_key: base64url(device.publicKey) });
    expect(res.status).toBe(200);
    const d = await res.json();
    expect(d).toMatchObject({ verification_uri: "http://localhost:3000/link", interval: 5, expires_in: 600 });
    expect(d.user_code).toMatch(/^[A-Z0-9]{4}-[A-Z0-9]{4}$/);
    const poll = () => post(site, "token", { grant_type: "urn:ietf:params:oauth:grant-type:device_code", device_code: d.device_code, client_id: "dev.mavio.tvos" });
    expect((await (await poll()).json()).error).toBe("authorization_pending");
    expect((await (await poll()).json()).error).toBe("slow_down");

    expect(await asAlice.getLinkCode({ userCode: d.user_code.toLowerCase() })).toMatchObject({ clientId: "dev.mavio.tvos", serverId: "" });
    await asAlice.answerLinkCode({ userCode: d.user_code, approve: true });
    res = await poll();
    expect(res.status).toBe(200);
    expect((await res.json()).account.id).toBe(alice.id);
    expect((await (await poll()).json()).error).toBe("invalid_grant");

    expect((await post(site, "device", { client_id: "example", device_key: base64url(device.publicKey) })).status).toBe(400);
  });

  test("sign in to a server's web UI only its administrator", async () => {
    const { site, server } = await claimed();
    const bob = await site.signUp("bob");
    const res = await post(site, "device", { client_id: server.id, device_key: base64url(new Uint8Array(32)) });
    const d = await res.json();
    const asBob = site.client(AccountService, bob.headers);
    expect((await asBob.getLinkCode({ userCode: d.user_code })).serverName).toBe("Home");
    expect(await code(asBob.answerLinkCode({ userCode: d.user_code, approve: true }))).toBe(Code.PermissionDenied);
    await asBob.answerLinkCode({ userCode: d.user_code, approve: false });
    const poll = await post(site, "token", {
      grant_type: "urn:ietf:params:oauth:grant-type:device_code",
      device_code: d.device_code,
      client_id: server.id,
    });
    expect((await poll.json()).error).toBe("access_denied");
  });
});

import { Code } from "@connectrpc/connect";
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

function post(site: Site, path: string, fields: Record<string, string>, origin = local) {
  return site.fetch(new Request(`${apiURL}/oauth/${path}`, { method: "POST", headers: { origin }, body: new URLSearchParams(fields) }));
}

describe("authorization codes", () => {
  test("are exchanged once with the verifier for a session of the device key", async () => {
    const { site, alice, asAlice, server } = await claimed();
    const device = await newDevice();
    const challenge = await s256(verifier);
    const newCode = async () =>
      (await asAlice.createAuthorizationCode({ serverId: server.id, redirectUri: redirect, codeChallenge: challenge, deviceKey: device.publicKey }))
        .code;
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
    const invitations = await asBob.listInvitations({});
    expect(invitations.invitations).toHaveLength(1);
    await asBob.acceptInvitation({ serverId: server.id });
    const req = { serverId: server.id, redirectUri: redirect, codeChallenge: await s256(verifier), deviceKey: new Uint8Array(32) };
    expect(await code(asBob.createAuthorizationCode(req))).toBe(Code.PermissionDenied);
    expect(await code(site.client(AccountService).createAuthorizationCode(req))).toBe(Code.Unauthenticated);
    expect(await code(asAlice.createAuthorizationCode({ ...req, redirectUri: "javascript:alert(1)" }))).toBe(Code.InvalidArgument);
    expect(await code(asAlice.createAuthorizationCode({ ...req, codeChallenge: "short" }))).toBe(Code.InvalidArgument);
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

describe("claim requests", () => {
  test("claim the server for the account that confirms, which signs in", async () => {
    const site = await newSite();
    const alice = await site.signUp("alice");
    const asAlice = site.client(AccountService, alice.headers);
    const server = await newServer();
    const api = site.serverClient(server);
    const device = await newDevice();
    const created = await api.createClaimRequest({
      publicKey: server.publicKey,
      name: "Home",
      version: "1.0",
      redirectUri: redirect,
      codeChallenge: await s256(verifier),
      deviceKey: device.publicKey,
      state: "s1",
    });
    expect(created.url).toBe(`http://localhost:3000/claim?request=${created.claimRequest}`);
    const claim = { claimRequest: created.claimRequest, publicKey: server.publicKey };
    expect(await code(api.claimServer(claim))).toBe(Code.FailedPrecondition);
    expect(await asAlice.getClaimRequest({ claimRequest: created.claimRequest })).toMatchObject({
      serverId: server.id,
      serverName: "Home",
      redirectUri: redirect,
      claimed: false,
    });

    const { redirectUrl } = await asAlice.answerClaimRequest({ claimRequest: created.claimRequest, confirm: true });
    const to = new URL(redirectUrl);
    expect(to.origin + to.pathname).toBe(redirect);
    expect(to.searchParams.get("state")).toBe("s1");
    expect(await code(asAlice.answerClaimRequest({ claimRequest: created.claimRequest, confirm: true }))).toBe(Code.NotFound);

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
    const api = site.serverClient(server);
    const created = await api.createClaimRequest({
      publicKey: server.publicKey,
      redirectUri: redirect,
      codeChallenge: await s256(verifier),
      deviceKey: new Uint8Array(32),
    });
    const { redirectUrl } = await site
      .client(AccountService, alice.headers)
      .answerClaimRequest({ claimRequest: created.claimRequest, confirm: false });
    expect(new URL(redirectUrl).searchParams.get("error")).toBe("access_denied");
    expect(await code(api.claimServer({ claimRequest: created.claimRequest, publicKey: server.publicKey }))).toBe(Code.InvalidArgument);
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

import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { AccessSchema, AccountRevoked_Reason } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { describe, expect, test } from "vitest";

import { fromBase64url } from "../src/server/ids.ts";
import { apiURL, newServer, newSite } from "./harness.ts";

const libraryA = "0193a5b0-0000-7000-8000-00000000000a";
const libraryB = "0193a5b0-0000-7000-8000-00000000000b";

async function code(err: Promise<unknown>): Promise<Code | undefined> {
  try {
    await err;
  } catch (e) {
    return ConnectError.from(e).code;
  }
  return undefined;
}

/** claimed returns a site with a server alice claimed and published two libraries to. */
async function claimed() {
  const site = await newSite();
  const alice = await site.signUp("alice");
  const asAlice = site.client(AccountService, alice.headers);
  const server = await newServer();
  const api = site.serverClient(server);
  const { claimCode } = await asAlice.createClaimCode({});
  const resp = await api.claimServer({ claimCode, publicKey: server.publicKey, name: "Home", version: "1.0" });
  await api.publishLibraries({
    libraries: [
      { id: libraryA, name: "Movies", kind: "movies" },
      { id: libraryB, name: "Shows", kind: "shows" },
    ],
  });
  return { site, alice, asAlice, server, api, resp };
}

describe("ServerService", () => {
  test("claims a server for the code's account", async () => {
    const { site, asAlice, server, api, resp } = await claimed();
    expect(resp.administrator?.userName).toBe("alice");
    expect(resp.cursor).toBe(0n);
    const { servers } = await asAlice.listServers({});
    expect(servers.map((s) => [s.id, s.name, s.administrator?.userName, s.libraries.map((l) => l.name)])).toEqual([
      [server.id, "Home", "alice", ["Movies", "Shows"]],
    ]);
    // A code claims once.
    const other = await newServer();
    const { claimCode } = await asAlice.createClaimCode({});
    await api.claimServer({ claimCode, publicKey: server.publicKey });
    expect(await code(site.serverClient(other).claimServer({ claimCode, publicKey: other.publicKey }))).toBe(Code.InvalidArgument);
  });

  test("refuses tokens that are not the server's", async () => {
    const { site, server } = await claimed();
    const other = await newServer();
    const call = (token: string) => code(site.serverClient({ token: async () => token }).listChanges({}));
    expect(await call(await other.token())).toBe(Code.Unauthenticated); // unclaimed
    expect(await call(await server.token({ aud: "https://example.com/api" }))).toBe(Code.Unauthenticated);
    expect(await call(await server.token({ exp: Math.floor(Date.now() / 1000) + 3600 }))).toBe(Code.Unauthenticated);
    expect(await call((await server.token()).slice(0, -4) + "AAAA")).toBe(Code.Unauthenticated);
    const once = await server.token();
    expect(await call(once)).toBeUndefined();
    expect(await call(once)).toBe(Code.Unauthenticated); // replayed
    // A claim must be signed by the key it registers.
    const { claimCode } = await site.client(AccountService, (await site.signUp("bob")).headers).createClaimCode({});
    expect(
      await code(site.serverClient(other).claimServer({ claimCode, publicKey: server.publicKey })),
    ).toBe(Code.Unauthenticated);
  });
});

describe("access records", () => {
  test("take effect when accepted and reach the server as changes", async () => {
    const { site, asAlice, server, api } = await claimed();
    const bob = await site.signUp("bob");
    const asBob = site.client(AccountService, bob.headers);
    const access = create(AccessSchema, { libraryIds: [libraryA], maxParentalRating: 13, allowDownload: true });

    // Granting invites; nothing reaches the server yet.
    const { record } = await asAlice.grantAccess({ serverId: server.id, userName: "Bob", access });
    expect(record?.invitation).toBe(true);
    expect((await api.listChanges({})).changes).toEqual([]);
    expect(await code(asBob.createAssertion({ serverId: server.id, deviceKey: new Uint8Array(32) }))).toBe(Code.PermissionDenied);
    const { invitations } = await asBob.listInvitations({});
    expect(invitations.map((i) => [i.serverId, i.serverName, i.administrator?.userName])).toEqual([[server.id, "Home", "alice"]]);

    await asBob.acceptInvitation({ serverId: server.id });
    let page = await api.listChanges({});
    expect(page.cursor).toBe(1n);
    expect(page.changes[0].kind.case).toBe("accessGranted");
    const grant = page.changes[0].kind.value as { account: { id: string; userName: string }; access: typeof access };
    expect([grant.account.id, grant.account.userName, grant.access.libraryIds, grant.access.maxParentalRating]).toEqual([
      bob.id,
      "bob",
      [libraryA],
      13,
    ]);
    const { servers } = await asBob.listServers({});
    expect(servers.map((s) => [s.id, s.libraries.map((l) => l.name)])).toEqual([[server.id, ["Movies"]]]);

    // A record in effect changes at once.
    await asAlice.grantAccess({ serverId: server.id, userName: "bob", access: create(AccessSchema, { allLibraries: true }) });
    page = await api.listChanges({ after: 1n });
    expect(page.changes.map((c) => [c.cursor, c.kind.case])).toEqual([[2n, "accessGranted"]]);

    // Records name the server's libraries only, and never the administrator.
    expect(
      await code(asAlice.grantAccess({ serverId: server.id, userName: "bob", access: create(AccessSchema, { libraryIds: ["0193a5b0-0000-7000-8000-0000000000ff"] }) })),
    ).toBe(Code.InvalidArgument);
    expect(await code(asAlice.grantAccess({ serverId: server.id, userName: "alice", access }))).toBe(Code.InvalidArgument);
    expect(await code(asBob.grantAccess({ serverId: server.id, userName: "alice", access }))).toBe(Code.NotFound);
    expect(await code(asAlice.grantAccess({ serverId: server.id, userName: "nobody", access }))).toBe(Code.NotFound);

    const { records } = await asAlice.listAccess({ serverId: server.id });
    expect(records.map((r) => [r.account?.userName, r.invitation, r.access?.allLibraries])).toEqual([["bob", false, true]]);

    await asAlice.revokeAccess({ serverId: server.id, accountId: bob.id });
    page = await api.listChanges({ after: 2n });
    expect(page.changes.map((c) => [c.cursor, c.kind.case, (c.kind.value as { accountId: string }).accountId])).toEqual([
      [3n, "accessRevoked", bob.id],
    ]);
    expect(await asBob.listServers({})).toMatchObject({ servers: [] });
  });

  test("an account declines invitations and leaves servers", async () => {
    const { site, asAlice, server, api } = await claimed();
    const bob = await site.signUp("bob");
    const asBob = site.client(AccountService, bob.headers);
    const access = create(AccessSchema, { allLibraries: true });
    await asAlice.grantAccess({ serverId: server.id, userName: "bob", access });
    await asBob.declineInvitation({ serverId: server.id });
    expect((await asAlice.listAccess({ serverId: server.id })).records).toEqual([]);
    expect(await code(asBob.acceptInvitation({ serverId: server.id }))).toBe(Code.NotFound);

    await asAlice.grantAccess({ serverId: server.id, userName: "bob", access });
    await asBob.acceptInvitation({ serverId: server.id });
    await asBob.leaveServer({ serverId: server.id });
    expect((await api.listChanges({})).changes.map((c) => c.kind.case)).toEqual(["accessGranted", "accessRevoked"]);
  });

  test("pages changes", async () => {
    const { site, asAlice, server, api } = await claimed();
    for (const name of ["bob", "carol", "dave"]) {
      const u = await site.signUp(name);
      await asAlice.grantAccess({ serverId: server.id, userName: name, access: create(AccessSchema, { allLibraries: true }) });
      await site.client(AccountService, u.headers).acceptInvitation({ serverId: server.id });
    }
    const first = await api.listChanges({ pageSize: 2 });
    expect([first.changes.length, first.more, first.cursor]).toEqual([2, true, 2n]);
    const rest = await api.listChanges({ after: first.cursor, pageSize: 2 });
    expect([rest.changes.length, rest.more, rest.cursor]).toEqual([1, false, 3n]);
    expect((await api.listChanges({ after: 3n })).cursor).toBe(3n);
  });
});

describe("sign-in assertions", () => {
  test("are signed by the site's keys for one server and device", async () => {
    const { site, asAlice, server } = await claimed();
    const device = new Uint8Array(32).fill(7);
    const { assertion, expireTime } = await asAlice.createAssertion({ serverId: server.id, deviceKey: device });
    const [h, c, s] = assertion.split(".");
    const header = JSON.parse(new TextDecoder().decode(fromBase64url(h)));
    const claims = JSON.parse(new TextDecoder().decode(fromBase64url(c)));
    expect(header.alg).toBe("EdDSA");
    expect(claims).toMatchObject({ iss: apiURL, aud: server.id, device_key: "BwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwc" });
    expect(claims.exp - claims.iat).toBe(120);
    expect(BigInt(claims.exp)).toBe(expireTime?.seconds);
    expect(claims.nonce).toMatch(/^[A-Za-z0-9_-]{22}$/);

    // The key set servers fetch checks it.
    const jwks = await (await site.fetch(new Request(`${apiURL}/auth/jwks`))).json();
    const jwk = jwks.keys.find((k: { kid: string }) => k.kid === header.kid);
    expect(jwk).toMatchObject({ kty: "OKP", crv: "Ed25519" });
    const key = await crypto.subtle.importKey("jwk", jwk, { name: "Ed25519" }, false, ["verify"]);
    expect(await crypto.subtle.verify({ name: "Ed25519" }, key, fromBase64url(s)!, new TextEncoder().encode(`${h}.${c}`))).toBe(true);

    expect(await code(asAlice.createAssertion({ serverId: "unknown", deviceKey: device }))).toBe(Code.PermissionDenied);
    expect(await code(asAlice.createAssertion({ serverId: server.id, deviceKey: new Uint8Array(3) }))).toBe(Code.InvalidArgument);
    expect(await code(site.client(AccountService).createAssertion({ serverId: server.id, deviceKey: device }))).toBe(Code.Unauthenticated);
  });
});

describe("account changes", () => {
  test("renames, compromise and deletion reach the account's servers", async () => {
    const { site, asAlice, server, api } = await claimed();
    const bob = await site.signUp("bob");
    const asBob = site.client(AccountService, bob.headers);
    await asAlice.grantAccess({ serverId: server.id, userName: "bob", access: create(AccessSchema, { allLibraries: true }) });
    await asBob.acceptInvitation({ serverId: server.id });

    await site.auth.api.updateUser({ headers: new Headers(bob.headers), body: { name: "Robert" } });
    let changes = (await api.listChanges({ after: 1n })).changes;
    expect(changes.map((c) => [c.kind.case, (c.kind.value as { displayName: string }).displayName])).toEqual([["accountChanged", "Robert"]]);

    await asBob.reportCompromised({});
    expect(await code(asBob.getAccount({}))).toBe(Code.Unauthenticated);
    changes = (await api.listChanges({ after: 2n })).changes;
    expect(changes.map((c) => [c.kind.case, (c.kind.value as { reason: number }).reason])).toEqual([
      ["accountRevoked", AccountRevoked_Reason.COMPROMISED],
    ]);

    // An administrator cannot leave its servers behind; other accounts go with their records.
    await expect(
      site.auth.api.deleteUser({ headers: new Headers((await signIn(site, "alice")).headers), body: { password: "alice password" } }),
    ).rejects.toThrow(/servers/);
    await site.auth.api.deleteUser({ headers: new Headers((await signIn(site, "bob")).headers), body: { password: "bob password" } });
    changes = (await api.listChanges({ after: 3n })).changes;
    expect(changes.map((c) => [c.kind.case, (c.kind.value as { reason: number }).reason])).toEqual([
      ["accountRevoked", AccountRevoked_Reason.DELETED],
    ]);
  });

  test("a server claimed by another account revokes the previous administrator", async () => {
    const { site, server, api, alice } = await claimed();
    const carol = await site.signUp("carol");
    const { claimCode } = await site.client(AccountService, carol.headers).createClaimCode({});
    const resp = await api.claimServer({ claimCode, publicKey: server.publicKey });
    expect([resp.administrator?.userName, resp.cursor]).toEqual(["carol", 0n]);
    const { changes } = await api.listChanges({ after: resp.cursor });
    expect(changes.map((c) => [c.kind.case, (c.kind.value as { accountId: string }).accountId])).toEqual([["accessRevoked", alice.id]]);
    expect((await site.client(AccountService, alice.headers).listServers({})).servers).toEqual([]);
  });

  test("an administrator removes a server", async () => {
    const { site, asAlice, server } = await claimed();
    expect(await code(site.client(AccountService, (await site.signUp("bob")).headers).removeServer({ serverId: server.id }))).toBe(Code.NotFound);
    await asAlice.removeServer({ serverId: server.id });
    expect((await asAlice.listServers({})).servers).toEqual([]);
  });
});

async function signIn(site: Awaited<ReturnType<typeof newSite>>, username: string) {
  const res = await site.auth.api.signInUsername({ body: { username, password: `${username} password` } });
  return { headers: { authorization: `Bearer ${res?.token}` } };
}

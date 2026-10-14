import { create } from "@bufbuild/protobuf";
import { Code } from "@connectrpc/connect";
import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { AccessSchema, AccountKind, AccountRevoked_Reason } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { Friend_State, FriendService, HomeService } from "@mavio/proto/mavio/cloud/v1/home_pb";
import { RestrictionProfile } from "@mavio/proto/mavio/user/v1/user_pb";
import { describe, expect, test } from "vitest";

import { schema } from "../src/server/db/index.ts";
import { baseURL, claimed, code, libraryA, newSite } from "./harness.ts";

type Site = Awaited<ReturnType<typeof newSite>>;

/** switchTo switches a session to another member, returning the new session's headers and the response's status. */
async function switchTo(site: Site, headers: Record<string, string>, body: { accountId: string; pin?: string; password?: string }) {
  const res = await site.fetch(
    new Request(`${baseURL}/api/auth/home/switch`, {
      method: "POST",
      headers: { ...headers, "content-type": "application/json", origin: baseURL },
      body: JSON.stringify(body),
    }),
  );
  const token = res.headers.get("set-auth-token");
  return { status: res.status, headers: token ? { authorization: `Bearer ${token}` } : headers };
}

/** givePass gives an account a Mavio Pass for a day. */
async function givePass(site: Site, accountId: string) {
  await site.db.insert(schema.passes).values({ accountId, expiresAt: new Date(Date.now() + 86_400_000) });
}

describe("friends", () => {
  test("are requested, accepted and removed", async () => {
    const site = await newSite();
    const [alice, bob] = [await site.signUp("alice"), await site.signUp("bob")];
    const [asAlice, asBob] = [site.client(FriendService, alice.headers), site.client(FriendService, bob.headers)];
    expect((await asAlice.addFriend({ name: "bob@example.com" })).friend?.state).toBe(Friend_State.SENT);
    expect((await asBob.listFriends({})).friends.map((f) => [f.account?.userName, f.state])).toEqual([["alice", Friend_State.RECEIVED]]);
    // Adding back accepts.
    expect((await asBob.addFriend({ name: "Alice" })).friend?.state).toBe(Friend_State.FRIEND);
    expect((await asAlice.listFriends({})).friends.map((f) => [f.account?.userName, f.state])).toEqual([["bob", Friend_State.FRIEND]]);
    await asBob.removeFriend({ accountId: alice.id });
    expect((await asAlice.listFriends({})).friends).toEqual([]);
    expect(await code(asAlice.addFriend({ name: "alice" }))).toBe(Code.InvalidArgument);
    expect(await code(asAlice.addFriend({ name: "nobody" }))).toBe(Code.NotFound);
  });

  test("are requested along with access", async () => {
    const { site, asAlice, server } = await claimed();
    const bob = await site.signUp("bob");
    await asAlice.grantAccess({ serverId: server.id, userName: "bob", access: create(AccessSchema, { allLibraries: true }) });
    const asBob = site.client(FriendService, bob.headers);
    expect((await asBob.listFriends({})).friends.map((f) => [f.account?.userName, f.state])).toEqual([["alice", Friend_State.RECEIVED]]);
    await asBob.acceptFriend({ accountId: (await site.client(AccountService, bob.headers).listInvitations({})).invitations[0].administrator!.id });
    expect((await asBob.listFriends({})).friends[0].state).toBe(Friend_State.FRIEND);
  });
});

describe("Homes", () => {
  test("hold managed accounts that the administrator switches to, restricts and removes", async () => {
    const { site, alice, asAlice, server, api } = await claimed();
    const home = site.client(HomeService, alice.headers);
    await home.createHome({ name: "Family" });
    const { member } = await home.createManagedAccount({
      userName: "Kid",
      displayName: "The Kid",
      restriction: RestrictionProfile.OLDER_CHILD,
      pin: "1234",
    });
    const kid = member!.account!;
    expect([kid.userName, kid.kind, kid.restriction, member?.hasPin]).toEqual(["Kid", AccountKind.MANAGED, RestrictionProfile.OLDER_CHILD, true]);

    // The administrator's grants to a managed account take effect at once.
    await asAlice.grantAccess({ serverId: server.id, userName: "kid", access: create(AccessSchema, { libraryIds: [libraryA] }) });
    let page = await api.listChanges({});
    const granted = page.changes[0].kind.value as { account: { kind: AccountKind; restriction: RestrictionProfile } };
    expect([page.changes[0].kind.case, granted.account.kind, granted.account.restriction]).toEqual([
      "accessGranted",
      AccountKind.MANAGED,
      RestrictionProfile.OLDER_CHILD,
    ]);

    await home.updateManagedAccount({ accountId: kid.id, displayName: "Kid", restriction: RestrictionProfile.TEEN });
    page = await api.listChanges({ after: page.cursor });
    expect(page.changes.map((c) => [c.kind.case, (c.kind.value as { restriction: RestrictionProfile }).restriction])).toEqual([
      ["accountChanged", RestrictionProfile.TEEN],
    ]);

    // Switching needs the PIN, and wrong PINs slow down.
    expect((await switchTo(site, alice.headers, { accountId: kid.id })).status).toBe(403);
    expect((await switchTo(site, alice.headers, { accountId: kid.id, pin: "0000" })).status).toBe(403);
    const asKid = await switchTo(site, alice.headers, { accountId: kid.id, pin: "1234" });
    expect(asKid.status).toBe(200);
    const kidAccount = await site.client(AccountService, asKid.headers).getAccount({});
    expect([kidAccount.account?.userName, kidAccount.email, kidAccount.hasPin]).toEqual(["Kid", "", true]);
    // The administrator's session became the kid's.
    expect(await code(asAlice.getAccount({}))).toBe(Code.Unauthenticated);

    // A managed account has no credentials, friends or servers of its own.
    expect(await code(site.client(AccountService, asKid.headers).createClaimCode({}))).toBe(Code.PermissionDenied);
    expect(await code(site.client(FriendService, asKid.headers).listFriends({}))).toBe(Code.PermissionDenied);
    expect(await code(site.client(HomeService, asKid.headers).setPin({ accountId: kid.id, pin: "9999" }))).toBe(Code.PermissionDenied);
    const update = await site.fetch(
      new Request(`${baseURL}/api/auth/update-user`, {
        method: "POST",
        headers: { ...asKid.headers, "content-type": "application/json", origin: baseURL },
        body: JSON.stringify({ name: "Boss" }),
      }),
    );
    expect(update.status).toBe(403);

    // The administrator has no PIN, so switching back needs none.
    const back = await switchTo(site, asKid.headers, { accountId: alice.id });
    expect(back.status).toBe(200);
    const asAdmin = site.client(HomeService, back.headers);
    const { home: h } = await asAdmin.getHome({});
    expect(h?.members.map((m) => [m.account?.userName, m.servers.map((s) => s.name)])).toEqual([
      ["alice", []],
      ["Kid", ["Home"]],
    ]);

    // Removing the managed account deletes it and its data on its servers.
    await asAdmin.removeMember({ accountId: kid.id });
    page = await api.listChanges({ after: page.cursor });
    expect(page.changes.map((c) => [c.kind.case, (c.kind.value as { reason: AccountRevoked_Reason }).reason])).toEqual([
      ["accountRevoked", AccountRevoked_Reason.DELETED],
    ]);
    expect(await site.db.query.user.findFirst({ where: (u, { eq }) => eq(u.id, kid.id) })).toBeUndefined();
  });

  test("invite full accounts with the administrator's Pass", async () => {
    const { site, alice, server, api } = await claimed();
    const bob = await site.signUp("bob");
    const home = site.client(HomeService, alice.headers);
    const asBob = site.client(HomeService, bob.headers);
    await home.createHome({ name: "Family" });
    const offers = [{ serverId: server.id, access: create(AccessSchema, { allLibraries: true }) }];

    // Without a Pass the Home is frozen.
    expect((await home.getHome({})).home?.frozen).toBe(true);
    expect(await code(home.inviteToHome({ userName: "bob", offers }))).toBe(Code.FailedPrecondition);
    await givePass(site, alice.id);
    const { invitation } = await home.inviteToHome({ userName: "bob", offers });
    expect(invitation?.offers.map((o) => o.serverName)).toEqual(["Home"]);

    expect((await asBob.getHome({})).invitations.map((i) => i.homeName)).toEqual(["Family"]);
    await asBob.acceptHomeInvitation({ homeId: invitation!.homeId });
    const page = await api.listChanges({});
    expect(page.changes.map((c) => [c.kind.case, (c.kind.value as { account: { id: string } }).account.id])).toEqual([
      ["accessGranted", bob.id],
    ]);
    // Bob shares the Pass.
    const bobAccount = await site.client(AccountService, bob.headers).getAccount({});
    expect([bobAccount.passExpireTime, bobAccount.effectivePass]).toEqual([undefined, true]);

    // A full member with a PIN; switching to it after five wrong PINs needs its password.
    await asBob.setPin({ accountId: bob.id, pin: "4321" });
    await site.db.update(schema.pins).set({ failures: 5 });
    expect((await switchTo(site, alice.headers, { accountId: bob.id, pin: "4321" })).status).toBe(403);
    const asBobAgain = await switchTo(site, alice.headers, { accountId: bob.id, password: "bob password" });
    expect(asBobAgain.status).toBe(200);

    // Leaving makes bob alice's friend, and keeps his access.
    await site.client(HomeService, asBobAgain.headers).leaveHome({});
    const friends = await site.client(FriendService, asBobAgain.headers).listFriends({});
    expect(friends.friends.map((f) => [f.account?.userName, f.state])).toEqual([["alice", Friend_State.FRIEND]]);
    expect((await api.listChanges({ after: page.cursor })).changes).toEqual([]);
  });

  test("convert managed accounts into full accounts", async () => {
    const site = await newSite();
    const alice = await site.signUp("alice");
    const home = site.client(HomeService, alice.headers);
    await home.createHome({ name: "Family" });
    const { member } = await home.createManagedAccount({ userName: "teen", displayName: "Teen", restriction: RestrictionProfile.TEEN });
    await home.convertManagedAccount({ accountId: member!.account!.id, email: "teen@example.com", password: "teen password" });
    const signedIn = await site.auth.api.signInEmail({ body: { email: "teen@example.com", password: "teen password" } });
    const account = await site.client(AccountService, { authorization: `Bearer ${signedIn.token}` }).getAccount({});
    expect([account.account?.kind, account.account?.restriction]).toEqual([AccountKind.FULL, RestrictionProfile.UNSPECIFIED]);
    // The converted account stays in the Home, and the administrator cannot leave or be deleted.
    expect((await home.getHome({})).home?.members.map((m) => m.account?.userName)).toEqual(["alice", "teen"]);
    expect(await code(home.leaveHome({}))).toBe(Code.FailedPrecondition);
    await expect(site.auth.api.deleteUser({ body: {}, headers: alice.headers })).rejects.toThrow(/Dissolve/);
    await home.dissolveHome({});
    expect((await home.getHome({})).home).toBeUndefined();
  });
});

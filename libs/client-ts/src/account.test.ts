import { create } from "@bufbuild/protobuf";
import { createClient, createRouterTransport } from "@connectrpc/connect";
import { AccountService, ServerSchema } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { AccessSchema } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { type Friend, Friend_State, FriendSchema, FriendService, HomeService } from "@mavio/proto/mavio/cloud/v1/home_pb";
import { RestrictionProfile } from "@mavio/proto/mavio/user/v1/user_pb";
import { QueryClient } from "@tanstack/react-query";
import { expect, test } from "vitest";

import { type Cloud, cloudKey, cloudQueries, runAccountAction, serversByRole, sortFriends, type Topic } from "./account.ts";

const me = "acct-ann";

/** A mavio.dev answering from fixed data, recording the calls it gets, with a sync counted. */
function fakeCloud() {
  const calls: string[] = [];
  const friends: Friend[] = [
    create(FriendSchema, { account: { id: "c", userName: "cy" }, state: Friend_State.SENT }),
    create(FriendSchema, { account: { id: "b", userName: "bo", displayName: "Bo" }, state: Friend_State.FRIEND }),
    create(FriendSchema, { account: { id: "a", userName: "al", displayName: "Al" }, state: Friend_State.FRIEND }),
    create(FriendSchema, { account: { id: "d", userName: "di" }, state: Friend_State.RECEIVED }),
  ];
  const transport = createRouterTransport(({ service }) => {
    service(AccountService, {
      getAccount: () => (calls.push("getAccount"), { account: { id: me, userName: "ann" }, email: "ann@example.com", hasPin: true }),
      listServers: () => (calls.push("listServers"), { servers: [{ id: "den", name: "Den", administrator: { id: me } }] }),
      listInvitations: () => (calls.push("listInvitations"), { invitations: [{ serverId: "attic", serverName: "Attic" }] }),
      listAccess: (r) => (calls.push(`listAccess ${r.serverId}`), { records: [{ account: { id: "b" }, invitation: true }] }),
      acceptInvitation: (r) => (calls.push(`acceptInvitation ${r.serverId}`), {}),
      declineInvitation: (r) => (calls.push(`declineInvitation ${r.serverId}`), {}),
      grantAccess: (r) => (calls.push(`grantAccess ${r.serverId} ${r.userName} ${r.access?.allLibraries}`), {}),
    });
    service(FriendService, {
      listFriends: () => (calls.push("listFriends"), { friends }),
    });
    service(HomeService, {
      getHome: () => (calls.push("getHome"), {}),
      updateManagedAccount: (r) => (calls.push(`updateManagedAccount ${r.accountId} ${r.displayName} ${r.restriction}`), {}),
      setPin: (r) => (calls.push(`setPin ${r.accountId} ${JSON.stringify(r.pin)}`), {}),
    });
  });
  let syncs = 0;
  const cloud: Cloud = {
    accounts: createClient(AccountService, transport),
    friends: createClient(FriendService, transport),
    homes: createClient(HomeService, transport),
    sync: async () => void syncs++,
  };
  return { cloud, calls, syncs: () => syncs };
}

/** A query client holding data for every topic, to see which an action marks stale. */
function filled(): QueryClient {
  const qc = new QueryClient();
  for (const topic of ["account", "servers", "invitations", "friends", "home"] as Topic[]) qc.setQueryData(cloudKey(me, topic), {});
  qc.setQueryData(cloudKey(me, "access", "den"), []);
  // Another account's queries are left alone.
  qc.setQueryData(cloudKey("acct-bo", "servers"), {});
  return qc;
}

function stale(qc: QueryClient): string[] {
  return qc
    .getQueryCache()
    .getAll()
    .filter((q) => q.state.isInvalidated)
    .map((q) => q.queryKey.slice(1).join(" "))
    .sort();
}

test("the queries call mavio.dev's services as the account", async () => {
  const { cloud, calls } = fakeCloud();
  const q = cloudQueries(() => cloud, me);
  const qc = new QueryClient();
  expect((await qc.fetchQuery(q.account)).email).toBe("ann@example.com");
  expect((await qc.fetchQuery(q.servers)).map((s) => s.id)).toEqual(["den"]);
  expect((await qc.fetchQuery(q.invitations)).map((i) => i.serverId)).toEqual(["attic"]);
  expect((await qc.fetchQuery(q.access("den")))[0].invitation).toBe(true);
  expect((await qc.fetchQuery(q.friends)).map((f) => f.account?.id)).toEqual(["d", "a", "b", "c"]);
  await qc.fetchQuery(q.home);
  expect(calls).toEqual(["getAccount", "listServers", "listInvitations", "listAccess den", "listFriends", "getHome"]);
  expect(q.access("den").queryKey).toEqual(["mavio.dev", me, "access", "den"]);
});

test("an action refreshes what it changes, and follows the servers when they change", async () => {
  const cases: { name: string; run: (c: Cloud, qc: QueryClient) => Promise<void>; calls: string[]; stale: string[]; syncs: number }[] = [
    {
      name: "accepting an invitation",
      run: (c, qc) => runAccountAction(c, qc, me, "acceptInvitation", "attic"),
      calls: ["acceptInvitation attic"],
      stale: [`${me} invitations`, `${me} servers`],
      syncs: 1,
    },
    {
      name: "declining an invitation",
      run: (c, qc) => runAccountAction(c, qc, me, "declineInvitation", "attic"),
      calls: ["declineInvitation attic"],
      stale: [`${me} invitations`],
      syncs: 0,
    },
    {
      name: "granting access, which may ask to be friends",
      run: (c, qc) => runAccountAction(c, qc, me, "grantAccess", { serverId: "den", userName: "bo", access: create(AccessSchema, { allLibraries: true }) }),
      calls: ["grantAccess den bo true"],
      stale: [`${me} access den`, `${me} friends`],
      syncs: 0,
    },
    {
      name: "editing a managed account with its PIN removed",
      run: (c, qc) =>
        runAccountAction(c, qc, me, "updateManagedAccount", { accountId: "kid", displayName: "Kid", restriction: RestrictionProfile.TEEN, pin: "" }),
      calls: [`updateManagedAccount kid Kid ${RestrictionProfile.TEEN}`, 'setPin kid ""'],
      stale: [`${me} home`],
      syncs: 0,
    },
    {
      name: "editing a managed account, its PIN kept",
      run: (c, qc) => runAccountAction(c, qc, me, "updateManagedAccount", { accountId: "kid", displayName: "Kid", restriction: RestrictionProfile.TEEN }),
      calls: [`updateManagedAccount kid Kid ${RestrictionProfile.TEEN}`],
      stale: [`${me} home`],
      syncs: 0,
    },
  ];
  for (const c of cases) {
    const { cloud, calls, syncs } = fakeCloud();
    const qc = filled();
    await c.run(cloud, qc);
    expect(calls, c.name).toEqual(c.calls);
    expect(stale(qc), c.name).toEqual(c.stale);
    expect(syncs(), c.name).toBe(c.syncs);
  }
});

test("an action that fails refreshes nothing", async () => {
  const { cloud } = fakeCloud();
  const qc = filled();
  // The fake answers no friend requests.
  await expect(runAccountAction(cloud, qc, me, "addFriend", "bo")).rejects.toThrow();
  expect(stale(qc)).toEqual([]);
});

test("friends show the requests received first, then friends, then those sent, each by name", () => {
  const f = (id: string, state: Friend_State, displayName = "") => create(FriendSchema, { account: { id, userName: id, displayName }, state });
  const got = sortFriends([f("zed", Friend_State.SENT), f("x", Friend_State.FRIEND, "Bea"), f("y", Friend_State.FRIEND, "Ada"), f("w", Friend_State.RECEIVED)]);
  expect(got.map((x) => x.account?.id)).toEqual(["w", "y", "x", "zed"]);
});

test("servers split into those administered and those shared", () => {
  const s = (id: string, admin: string) => create(ServerSchema, { id, administrator: { id: admin } });
  const { administered, shared } = serversByRole([s("den", me), s("attic", "bo"), s("loft", me)], me);
  expect(administered.map((x) => x.id)).toEqual(["den", "loft"]);
  expect(shared.map((x) => x.id)).toEqual(["attic"]);
});

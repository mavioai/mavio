// The Mavio account's matters at mavio.dev (docs/accounts.md §3, §7,
// docs/clients.md §4.1): the account, the servers it reaches and the
// invitations to them, the access records of the servers it administers,
// its friends and its Home, called with the device's session at mavio.dev.
// Query keys start with ["mavio.dev", accountId], apart from the servers'.
import type { Client } from "@connectrpc/connect";
import type { Access } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { AccountService, type Server } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { type Friend, Friend_State, FriendService, HomeService, type ServerAccess } from "@mavio/proto/mavio/cloud/v1/home_pb";
import type { RestrictionProfile } from "@mavio/proto/mavio/user/v1/user_pb";
import { type QueryClient, type QueryKey, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { useServersStore, useSignIn } from "./hooks.ts";
import type { Servers } from "./servers.ts";

/** mavio.dev's services, called as the account, and following the account's servers once they change. */
export interface Cloud {
  accounts: Client<typeof AccountService>;
  friends: Client<typeof FriendService>;
  homes: Client<typeof HomeService>;
  /** Signs in to the servers the account newly reaches and out of those it no longer does. */
  sync: () => Promise<void>;
}

/** The services of mavio.dev with the sign-in's session. */
export function cloudOf(servers: Servers): Cloud {
  return {
    accounts: servers.mavioDev(AccountService),
    friends: servers.mavioDev(FriendService),
    homes: servers.mavioDev(HomeService),
    sync: () => servers.sync(),
  };
}

/** What the account's queries are about, each invalidated by the actions changing it. */
export type Topic = "account" | "servers" | "invitations" | "access" | "friends" | "home";

/** The key of a query about the account at mavio.dev. */
export function cloudKey(accountId: string, topic: Topic, ...rest: unknown[]): QueryKey {
  return ["mavio.dev", accountId, topic, ...rest];
}

/**
 * Friends in the order they are shown: the requests received, waiting on
 * the account, then its friends, then the requests it sent, each by name.
 */
export function sortFriends(friends: Friend[]): Friend[] {
  const rank = { [Friend_State.RECEIVED]: 0, [Friend_State.FRIEND]: 1, [Friend_State.SENT]: 2, [Friend_State.UNSPECIFIED]: 3 };
  const name = (f: Friend) => f.account?.displayName || f.account?.userName || "";
  return friends.toSorted((a, b) => rank[a.state] - rank[b.state] || name(a).localeCompare(name(b)));
}

/** The servers an account administers, and those shared with it, in mavio.dev's order. */
export function serversByRole(servers: Server[], accountId: string): { administered: Server[]; shared: Server[] } {
  return {
    administered: servers.filter((s) => s.administrator?.id === accountId),
    shared: servers.filter((s) => s.administrator?.id !== accountId),
  };
}

/** The queries of the account, over the services cloud gives as each is fetched. */
export function cloudQueries(cloud: () => Cloud, accountId: string) {
  return {
    account: { queryKey: cloudKey(accountId, "account"), queryFn: () => cloud().accounts.getAccount({}) },
    servers: { queryKey: cloudKey(accountId, "servers"), queryFn: async () => (await cloud().accounts.listServers({})).servers },
    invitations: {
      queryKey: cloudKey(accountId, "invitations"),
      queryFn: async () => (await cloud().accounts.listInvitations({})).invitations,
    },
    access: (serverId: string) => ({
      queryKey: cloudKey(accountId, "access", serverId),
      queryFn: async () => (await cloud().accounts.listAccess({ serverId })).records,
    }),
    friends: { queryKey: cloudKey(accountId, "friends"), queryFn: async () => sortFriends((await cloud().friends.listFriends({})).friends) },
    home: { queryKey: cloudKey(accountId, "home"), queryFn: () => cloud().homes.getHome({}) },
  };
}

/** An action at mavio.dev: its call, the topics it changes and whether it changes the servers the account reaches. */
interface Action<I> {
  run: (cloud: Cloud, input: I) => Promise<unknown>;
  changes: Topic[];
  sync?: boolean;
}

function action<I>(run: (cloud: Cloud, input: I) => Promise<unknown>, changes: Topic[], sync = false): Action<I> {
  return { run, changes, sync };
}

/** A managed account's changes: its name and restriction profile, and its PIN when pin is set, "" removing it. */
export interface ManagedChanges {
  accountId: string;
  displayName: string;
  restriction: RestrictionProfile;
  pin?: string;
}

/** What the account does at mavio.dev. */
export const accountActions = {
  // Invitations to servers, and leaving one.
  acceptInvitation: action((c, serverId: string) => c.accounts.acceptInvitation({ serverId }), ["invitations", "servers"], true),
  declineInvitation: action((c, serverId: string) => c.accounts.declineInvitation({ serverId }), ["invitations"]),
  leaveServer: action((c, serverId: string) => c.accounts.leaveServer({ serverId }), ["servers"], true),
  // The access records of a server administered; granting to someone not a friend asks them to be one.
  grantAccess: action(
    (c, r: { serverId: string; userName: string; access: Access }) => c.accounts.grantAccess(r),
    ["access", "friends"],
  ),
  revokeAccess: action((c, r: { serverId: string; accountId: string }) => c.accounts.revokeAccess(r), ["access"]),
  // Friends.
  addFriend: action((c, name: string) => c.friends.addFriend({ name }), ["friends"]),
  acceptFriend: action((c, accountId: string) => c.friends.acceptFriend({ accountId }), ["friends"]),
  removeFriend: action((c, accountId: string) => c.friends.removeFriend({ accountId }), ["friends"]),
  // The Home: a member leaving or removed becomes the administrator's friend.
  createHome: action((c, name: string) => c.homes.createHome({ name }), ["home"]),
  dissolveHome: action((c, _: void) => c.homes.dissolveHome({}), ["home", "friends"]),
  leaveHome: action((c, _: void) => c.homes.leaveHome({}), ["home", "friends", "account"]),
  acceptHomeInvitation: action(
    (c, homeId: string) => c.homes.acceptHomeInvitation({ homeId }),
    ["home", "account", "friends", "invitations", "servers"],
    true,
  ),
  declineHomeInvitation: action((c, homeId: string) => c.homes.declineHomeInvitation({ homeId }), ["home"]),
  inviteToHome: action((c, r: { userName: string; offers: ServerAccess[] }) => c.homes.inviteToHome(r), ["home"]),
  cancelHomeInvitation: action((c, accountId: string) => c.homes.cancelHomeInvitation({ accountId }), ["home"]),
  removeMember: action((c, accountId: string) => c.homes.removeMember({ accountId }), ["home", "friends", "access"]),
  createManagedAccount: action(
    (c, r: { userName: string; displayName: string; restriction: RestrictionProfile; pin?: string }) => c.homes.createManagedAccount(r),
    ["home"],
  ),
  updateManagedAccount: action(async (c, r: ManagedChanges) => {
    await c.homes.updateManagedAccount({ accountId: r.accountId, displayName: r.displayName, restriction: r.restriction });
    if (r.pin !== undefined) await c.homes.setPin({ accountId: r.accountId, pin: r.pin });
  }, ["home"]),
  convertManagedAccount: action(
    (c, r: { accountId: string; email: string; password: string }) => c.homes.convertManagedAccount(r),
    ["home"],
  ),
  // A PIN, the account's own or a managed account's; "" removes it.
  setPin: action((c, r: { accountId: string; pin: string }) => c.homes.setPin(r), ["account", "home"]),
};

/** An action of the account. */
export type AccountActionName = keyof typeof accountActions;

/** What an action takes. */
export type AccountActionInput<N extends AccountActionName> = (typeof accountActions)[N] extends Action<infer I> ? I : never;

/**
 * Runs an action, then marks the queries of the topics it changes stale
 * and, when it changes the servers the account reaches, follows them.
 */
export async function runAccountAction<N extends AccountActionName>(
  cloud: Cloud,
  queryClient: QueryClient,
  accountId: string,
  name: N,
  input: AccountActionInput<N>,
): Promise<void> {
  const a = accountActions[name] as Action<AccountActionInput<N>>;
  await a.run(cloud, input);
  await Promise.all(a.changes.map((topic) => queryClient.invalidateQueries({ queryKey: cloudKey(accountId, topic) })));
  if (a.sync) {
    try {
      await cloud.sync();
    } catch {
      // The servers are followed again as the web UI opens.
    }
  }
}

/** The Mavio account signed in, and its services; undefined for a standalone server's user. */
function useCloud(): { accountId: string; cloud: () => Cloud } | undefined {
  const servers = useServersStore();
  const id = useSignIn()?.account?.id;
  return id ? { accountId: id, cloud: () => cloudOf(servers) } : undefined;
}

function useCloudQuery<T>(pick: (q: ReturnType<typeof cloudQueries>) => { queryKey: QueryKey; queryFn: () => Promise<T> }) {
  const c = useCloud();
  const q = pick(cloudQueries(c?.cloud ?? (() => undefined as never), c?.accountId ?? ""));
  return useQuery({ ...q, enabled: !!c, staleTime: 30_000 });
}

/** The account: its name, e-mail address, Mavio Pass and whether it has a PIN. */
export function useMavioAccount() {
  return useCloudQuery((q) => q.account);
}

/** The servers the account reaches as mavio.dev lists them, with their administrators, libraries and the account's access. */
export function useMavioServers() {
  return useCloudQuery((q) => q.servers);
}

/** The invitations to servers the account has not answered. */
export function useServerInvitations() {
  return useCloudQuery((q) => q.invitations);
}

/** The access records of a server the account administers, invitations included. */
export function useAccessRecords(serverId: string) {
  return useCloudQuery((q) => q.access(serverId));
}

/** The account's friends and friend requests, the requests received first. */
export function useFriends() {
  return useCloudQuery((q) => q.friends);
}

/** The account's Home, if it is in one, and the invitations to Homes it has not answered. */
export function useMavioHome() {
  return useCloudQuery((q) => q.home);
}

/** An action of the account at mavio.dev, refreshing what it changes. */
export function useAccountAction<N extends AccountActionName>(name: N) {
  const c = useCloud();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: AccountActionInput<N>) => {
      if (!c) throw new Error("no Mavio account is signed in");
      return runAccountAction(c.cloud(), queryClient, c.accountId, name, input);
    },
  });
}

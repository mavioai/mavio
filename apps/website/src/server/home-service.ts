// mavio.cloud.v1.HomeService (docs/accounts.md §3.5–§3.7, docs/mavio-dev.md §2).
import { create, fromJson } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError, type HandlerContext, type ServiceImpl } from "@connectrpc/connect";
import { AccessGrantedSchema, AccessSchema } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import {
  AcceptHomeInvitationResponseSchema,
  CancelHomeInvitationResponseSchema,
  ConvertManagedAccountResponseSchema,
  CreateHomeResponseSchema,
  CreateManagedAccountResponseSchema,
  DeclineHomeInvitationResponseSchema,
  DissolveHomeResponseSchema,
  GetHomeResponseSchema,
  type HomeInvitation,
  HomeInvitationSchema,
  HomeMemberSchema,
  HomeSchema,
  type HomeService,
  InviteToHomeResponseSchema,
  LeaveHomeResponseSchema,
  RemoveMemberResponseSchema,
  type ServerAccess,
  ServerAccessSchema,
  ServerSummarySchema,
  SetPinResponseSchema,
  UpdateManagedAccountResponseSchema,
} from "@mavio/proto/mavio/cloud/v1/home_pb";
import { and, asc, count, eq, inArray } from "drizzle-orm";

import type { Auth } from "./auth.ts";
import { accessJson, accountChanged, accountOf, addChange, managed, reachedServers } from "./changes.ts";
import { type Database, schema, type Tx } from "./db/index.ts";
import { frozen, homeOf, maxMembers, removeMember, setPin } from "./home.ts";
import * as password from "./password.ts";
import type { Deps } from "./server-service.ts";

type Db = Database | Tx;
type User = typeof schema.user.$inferSelect;
type Home = typeof schema.homes.$inferSelect;
type StoredOffer = { serverId: string; access: unknown };

/** homeService implements HomeService. */
export function homeService({ db, auth, now = () => new Date() }: Deps & { auth: Auth }): ServiceImpl<typeof HomeService> {
  async function signedIn(ctx: HandlerContext): Promise<User> {
    const session = await auth.api.getSession({ headers: ctx.requestHeader });
    const u = session && (await db.query.user.findFirst({ where: eq(schema.user.id, session.user.id) }));
    if (!u) throw new ConnectError("sign in to mavio.dev", Code.Unauthenticated);
    return u;
  }

  async function full(ctx: HandlerContext): Promise<User> {
    const me = await signedIn(ctx);
    if (managed(me)) throw new ConnectError("managed accounts cannot do this", Code.PermissionDenied);
    return me;
  }

  // administered returns the Home the account administers.
  async function administered(tx: Db, me: User): Promise<Home> {
    const home = await tx.query.homes.findFirst({ where: eq(schema.homes.administratorId, me.id) });
    if (!home) throw new ConnectError("you administer no Home", Code.FailedPrecondition);
    return home;
  }

  // managedMember returns a managed account of the administrator's Home.
  async function managedMember(tx: Db, home: Home, accountId: string): Promise<User> {
    const m = await tx.query.homeMembers.findFirst({
      where: and(eq(schema.homeMembers.accountId, accountId), eq(schema.homeMembers.homeId, home.id)),
      with: { account: true },
    });
    if (!m || !managed(m.account)) throw new ConnectError("no such managed account in your Home", Code.NotFound);
    return m.account;
  }

  // checkRoom refuses a new member of a full Home, counting invitations too.
  async function checkRoom(tx: Db, home: Home, invitations: boolean) {
    const [{ members }] = await tx.select({ members: count() }).from(schema.homeMembers).where(eq(schema.homeMembers.homeId, home.id));
    let pending = 0;
    if (invitations) {
      [{ pending }] = await tx
        .select({ pending: count() })
        .from(schema.homeInvitations)
        .where(eq(schema.homeInvitations.homeId, home.id));
    }
    if (members + pending >= maxMembers) {
      throw new ConnectError(`a Home holds at most ${maxMembers} accounts`, Code.ResourceExhausted);
    }
  }

  async function serverNames(tx: Db, ids: string[]) {
    if (ids.length === 0) return new Map<string, string>();
    const rows = await tx.select({ id: schema.servers.id, name: schema.servers.name }).from(schema.servers).where(inArray(schema.servers.id, ids));
    return new Map(rows.map((r) => [r.id, r.name]));
  }

  async function invitationToProto(tx: Db, i: typeof schema.homeInvitations.$inferSelect, home: Home): Promise<HomeInvitation> {
    const [administrator, account] = await Promise.all([
      tx.query.user.findFirst({ where: eq(schema.user.id, home.administratorId) }),
      tx.query.user.findFirst({ where: eq(schema.user.id, i.accountId) }),
    ]);
    const offers = i.offers as StoredOffer[];
    const names = await serverNames(tx, offers.map((o) => o.serverId));
    return create(HomeInvitationSchema, {
      homeId: home.id,
      homeName: home.name,
      administrator: administrator && accountOf(administrator),
      account: account && accountOf(account),
      offers: offers.map((o) =>
        create(ServerAccessSchema, {
          serverId: o.serverId,
          serverName: names.get(o.serverId) ?? "",
          access: fromJson(AccessSchema, o.access as never),
        }),
      ),
      createTime: timestampFromDate(i.createdAt),
    });
  }

  // homeToProto returns a Home as the account sees it.
  async function homeToProto(tx: Db, home: Home, me: User) {
    const isAdmin = home.administratorId === me.id;
    const members = await tx.query.homeMembers.findMany({
      where: eq(schema.homeMembers.homeId, home.id),
      with: { account: true },
      orderBy: [asc(schema.homeMembers.joinedAt)],
    });
    members.sort((a, b) => Number(b.accountId === home.administratorId) - Number(a.accountId === home.administratorId));
    const ids = members.map((m) => m.accountId);
    const pins = new Set(
      (await tx.select({ id: schema.pins.accountId }).from(schema.pins).where(inArray(schema.pins.accountId, ids))).map((p) => p.id),
    );
    const out = [];
    for (const m of members) {
      let servers: ReturnType<typeof create<typeof ServerSummarySchema>>[] = [];
      if (isAdmin && managed(m.account)) {
        const reached = await reachedServers(tx, m.accountId);
        const names = await serverNames(tx, reached);
        servers = reached.map((id) => create(ServerSummarySchema, { id, name: names.get(id) ?? "" }));
      }
      out.push(
        create(HomeMemberSchema, {
          account: accountOf(m.account),
          hasPin: pins.has(m.accountId),
          servers,
          joinTime: timestampFromDate(m.joinedAt),
        }),
      );
    }
    const invitations = isAdmin
      ? await Promise.all(
          (await tx.query.homeInvitations.findMany({ where: eq(schema.homeInvitations.homeId, home.id) })).map((i) =>
            invitationToProto(tx, i, home),
          ),
        )
      : [];
    return create(HomeSchema, {
      id: home.id,
      name: home.name,
      administrator: accountOf(members[0].account),
      members: out,
      invitations,
      frozen: await frozen(tx, home, now()),
      createTime: timestampFromDate(home.createdAt),
    });
  }

  // checkOffers refuses offers on servers the administrator does not administer.
  async function checkOffers(tx: Db, me: User, offers: ServerAccess[]) {
    for (const o of offers) {
      const server = await tx.query.servers.findFirst({
        where: and(eq(schema.servers.id, o.serverId), eq(schema.servers.administratorId, me.id)),
      });
      if (!server) throw new ConnectError("no such server of yours", Code.NotFound);
      const a = o.access ?? create(AccessSchema);
      if (a.allLibraries && a.libraryIds.length > 0) {
        throw new ConnectError("every library or some, not both", Code.InvalidArgument);
      }
      if (a.libraryIds.length > 0) {
        const known = await tx
          .select({ id: schema.serverLibraries.id })
          .from(schema.serverLibraries)
          .where(and(eq(schema.serverLibraries.serverId, o.serverId), inArray(schema.serverLibraries.id, a.libraryIds)));
        if (known.length !== new Set(a.libraryIds).size) {
          throw new ConnectError("the server has no such library", Code.InvalidArgument);
        }
      }
    }
  }

  const newID = () => crypto.randomUUID().replaceAll("-", "");

  return {
    async getHome(_, ctx) {
      const me = await signedIn(ctx);
      const home = await homeOf(db, me.id);
      const received = await db.query.homeInvitations.findMany({
        where: eq(schema.homeInvitations.accountId, me.id),
        with: { home: true },
      });
      return create(GetHomeResponseSchema, {
        home: home && (await homeToProto(db, home, me)),
        invitations: await Promise.all(received.map((i) => invitationToProto(db, i, i.home))),
      });
    },

    async createHome(req, ctx) {
      const me = await full(ctx);
      const at = now();
      return db.transaction(async (tx) => {
        if (await homeOf(tx, me.id)) throw new ConnectError("you already belong to a Home", Code.FailedPrecondition);
        const [home] = await tx.insert(schema.homes).values({ id: newID(), name: req.name, administratorId: me.id, createdAt: at }).returning();
        await tx.insert(schema.homeMembers).values({ accountId: me.id, homeId: home.id, joinedAt: at });
        return create(CreateHomeResponseSchema, { home: await homeToProto(tx, home, me) });
      });
    },

    async dissolveHome(_, ctx) {
      const me = await full(ctx);
      const at = now();
      await db.transaction(async (tx) => {
        const home = await administered(tx, me);
        const members = await tx.query.homeMembers.findMany({ where: eq(schema.homeMembers.homeId, home.id), with: { account: true } });
        for (const m of members) {
          if (m.accountId !== me.id) await removeMember(tx, home, m.account, at);
        }
        await tx.delete(schema.homes).where(eq(schema.homes.id, home.id));
      });
      return create(DissolveHomeResponseSchema);
    },

    async inviteToHome(req, ctx) {
      const me = await full(ctx);
      const at = now();
      return db.transaction(async (tx) => {
        const home = await administered(tx, me);
        if (await frozen(tx, home, at)) {
          throw new ConnectError("inviting full accounts needs your Mavio Pass", Code.FailedPrecondition);
        }
        const u = await tx.query.user.findFirst({ where: eq(schema.user.username, req.userName.toLowerCase()) });
        if (!u || managed(u)) throw new ConnectError(`no account ${req.userName}`, Code.NotFound);
        if ((await homeOf(tx, u.id))?.id === home.id) throw new ConnectError("the account is in your Home", Code.AlreadyExists);
        await checkOffers(tx, me, req.offers);
        const offers: StoredOffer[] = req.offers.map((o) => ({ serverId: o.serverId, access: accessJson(o.access ?? create(AccessSchema)) }));
        const existing = await tx.query.homeInvitations.findFirst({
          where: and(eq(schema.homeInvitations.homeId, home.id), eq(schema.homeInvitations.accountId, u.id)),
        });
        if (!existing) await checkRoom(tx, home, true);
        const [i] = await tx
          .insert(schema.homeInvitations)
          .values({ homeId: home.id, accountId: u.id, offers, createdAt: at })
          .onConflictDoUpdate({ target: [schema.homeInvitations.homeId, schema.homeInvitations.accountId], set: { offers } })
          .returning();
        return create(InviteToHomeResponseSchema, { invitation: await invitationToProto(tx, i, home) });
      });
    },

    async cancelHomeInvitation(req, ctx) {
      const me = await full(ctx);
      const home = await administered(db, me);
      const removed = await db
        .delete(schema.homeInvitations)
        .where(and(eq(schema.homeInvitations.homeId, home.id), eq(schema.homeInvitations.accountId, req.accountId)))
        .returning();
      if (removed.length === 0) throw new ConnectError("no such invitation", Code.NotFound);
      return create(CancelHomeInvitationResponseSchema);
    },

    async acceptHomeInvitation(req, ctx) {
      const me = await full(ctx);
      const at = now();
      await db.transaction(async (tx) => {
        const [i] = await tx
          .delete(schema.homeInvitations)
          .where(and(eq(schema.homeInvitations.homeId, req.homeId), eq(schema.homeInvitations.accountId, me.id)))
          .returning();
        if (!i) throw new ConnectError("no such invitation", Code.NotFound);
        const home = await tx.query.homes.findFirst({ where: eq(schema.homes.id, i.homeId) });
        if (!home) throw new ConnectError("no such invitation", Code.NotFound);
        if (await frozen(tx, home, at)) throw new ConnectError("the Home is frozen", Code.FailedPrecondition);
        const current = await homeOf(tx, me.id);
        if (current?.administratorId === me.id) {
          throw new ConnectError("dissolve your Home before joining another", Code.FailedPrecondition);
        }
        if (current) await removeMember(tx, current, me, at);
        await checkRoom(tx, home, false);
        await tx.insert(schema.homeMembers).values({ accountId: me.id, homeId: home.id, joinedAt: at });
        // The offered access records take effect on the servers the administrator still administers.
        for (const o of i.offers as StoredOffer[]) {
          const server = await tx.query.servers.findFirst({
            where: and(eq(schema.servers.id, o.serverId), eq(schema.servers.administratorId, home.administratorId)),
          });
          if (!server) continue;
          await tx
            .insert(schema.accessRecords)
            .values({ serverId: o.serverId, accountId: me.id, access: o.access, invitation: false, createdAt: at, updatedAt: at })
            .onConflictDoUpdate({
              target: [schema.accessRecords.serverId, schema.accessRecords.accountId],
              set: { access: o.access, invitation: false, updatedAt: at },
            });
          const access = fromJson(AccessSchema, o.access as never);
          await addChange(tx, o.serverId, { case: "accessGranted", value: create(AccessGrantedSchema, { account: accountOf(me), access }) }, at);
        }
      });
      return create(AcceptHomeInvitationResponseSchema);
    },

    async declineHomeInvitation(req, ctx) {
      const me = await full(ctx);
      const removed = await db
        .delete(schema.homeInvitations)
        .where(and(eq(schema.homeInvitations.homeId, req.homeId), eq(schema.homeInvitations.accountId, me.id)))
        .returning();
      if (removed.length === 0) throw new ConnectError("no such invitation", Code.NotFound);
      return create(DeclineHomeInvitationResponseSchema);
    },

    async leaveHome(_, ctx) {
      const me = await full(ctx);
      await db.transaction(async (tx) => {
        const home = await homeOf(tx, me.id);
        if (!home) throw new ConnectError("you belong to no Home", Code.FailedPrecondition);
        if (home.administratorId === me.id) throw new ConnectError("the administrator dissolves the Home", Code.FailedPrecondition);
        await removeMember(tx, home, me, now());
      });
      return create(LeaveHomeResponseSchema);
    },

    async removeMember(req, ctx) {
      const me = await full(ctx);
      await db.transaction(async (tx) => {
        const home = await administered(tx, me);
        if (req.accountId === me.id) throw new ConnectError("the administrator dissolves the Home", Code.InvalidArgument);
        const m = await tx.query.homeMembers.findFirst({
          where: and(eq(schema.homeMembers.accountId, req.accountId), eq(schema.homeMembers.homeId, home.id)),
          with: { account: true },
        });
        if (!m) throw new ConnectError("no such member of your Home", Code.NotFound);
        await removeMember(tx, home, m.account, now());
      });
      return create(RemoveMemberResponseSchema);
    },

    async createManagedAccount(req, ctx) {
      const me = await full(ctx);
      const at = now();
      return db.transaction(async (tx) => {
        const home = await administered(tx, me);
        await checkRoom(tx, home, true);
        const username = req.userName.toLowerCase();
        if (await tx.query.user.findFirst({ where: eq(schema.user.username, username) })) {
          throw new ConnectError(`the user name ${req.userName} is taken`, Code.AlreadyExists);
        }
        const id = newID();
        const [u] = await tx
          .insert(schema.user)
          .values({
            id,
            name: req.displayName,
            // No sign-in accepts this address (docs/mavio-dev.md §2).
            email: `managed:${id}`,
            emailVerified: false,
            username,
            displayUsername: req.userName,
            kind: "managed",
            restriction: req.restriction,
            createdAt: at,
            updatedAt: at,
          })
          .returning();
        await tx.insert(schema.homeMembers).values({ accountId: id, homeId: home.id, joinedAt: at });
        if (req.pin) await setPin(tx, id, req.pin);
        return create(CreateManagedAccountResponseSchema, {
          member: create(HomeMemberSchema, { account: accountOf(u), hasPin: !!req.pin, joinTime: timestampFromDate(at) }),
        });
      });
    },

    async updateManagedAccount(req, ctx) {
      const me = await full(ctx);
      return db.transaction(async (tx) => {
        const home = await administered(tx, me);
        await managedMember(tx, home, req.accountId);
        const [u] = await tx
          .update(schema.user)
          .set({ name: req.displayName, restriction: req.restriction, updatedAt: now() })
          .where(eq(schema.user.id, req.accountId))
          .returning();
        await accountChanged(tx, accountOf(u));
        const pin = await tx.query.pins.findFirst({ where: eq(schema.pins.accountId, u.id) });
        return create(UpdateManagedAccountResponseSchema, { member: create(HomeMemberSchema, { account: accountOf(u), hasPin: !!pin }) });
      });
    },

    async convertManagedAccount(req, ctx) {
      const me = await full(ctx);
      const at = now();
      const hash = await password.hash(req.password);
      await db.transaction(async (tx) => {
        const home = await administered(tx, me);
        await managedMember(tx, home, req.accountId);
        const email = req.email.toLowerCase();
        if (await tx.query.user.findFirst({ where: eq(schema.user.email, email) })) {
          throw new ConnectError("the e-mail address is taken", Code.AlreadyExists);
        }
        const [u] = await tx
          .update(schema.user)
          .set({ email, kind: "full", restriction: 0, updatedAt: at })
          .where(eq(schema.user.id, req.accountId))
          .returning();
        await tx.insert(schema.account).values({
          id: newID(),
          accountId: u.id,
          providerId: "credential",
          userId: u.id,
          password: hash,
          createdAt: at,
          updatedAt: at,
        });
        await accountChanged(tx, accountOf(u));
      });
      return create(ConvertManagedAccountResponseSchema);
    },

    async setPin(req, ctx) {
      const me = await signedIn(ctx);
      await db.transaction(async (tx) => {
        if (req.accountId === me.id) {
          // A managed account's PIN is its administrator's to set.
          if (managed(me)) throw new ConnectError("your Home's administrator sets your PIN", Code.PermissionDenied);
        } else {
          await managedMember(tx, await administered(tx, me), req.accountId);
        }
        await setPin(tx, req.accountId, req.pin ?? "");
      });
      return create(SetPinResponseSchema);
    },
  };
}

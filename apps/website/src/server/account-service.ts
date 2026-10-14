// mavio.cloud.v1.AccountService (docs/mavio-dev.md §2), which signed-in
// accounts call with their mavio.dev sessions.
import { create, fromJson } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError, type HandlerContext, type ServiceImpl } from "@connectrpc/connect";
import {
  AcceptInvitationResponseSchema,
  type AccountService,
  AccessRecordSchema,
  CreateAssertionResponseSchema,
  CreateClaimCodeResponseSchema,
  CreateRelayTicketResponseSchema,
  DeclineInvitationResponseSchema,
  GetAccountResponseSchema,
  GrantAccessResponseSchema,
  InvitationSchema,
  LeaveServerResponseSchema,
  ListAccessResponseSchema,
  ListInvitationsResponseSchema,
  ListServersResponseSchema,
  RemoveServerResponseSchema,
  ReportCompromisedResponseSchema,
  RevokeAccessResponseSchema,
  ServerSchema,
} from "@mavio/proto/mavio/cloud/v1/account_pb";
import {
  type Access,
  AccessGrantedSchema,
  AccessRevokedSchema,
  AccessSchema,
  LibrarySchema,
} from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { and, eq, inArray } from "drizzle-orm";

import type { Auth } from "./auth.ts";
import { AccountRevoked_Reason, accessJson, accountOf, accountRevoked, addChange, managed } from "./changes.ts";
import { claimCodeHash, claimCodeLifetime, newClaimCode } from "./claim-codes.ts";
import { schema, type Tx } from "./db/index.ts";
import { between, effectivePass, homeOf, ownPass } from "./home.ts";
import { base64url } from "./ids.ts";
import { relayHost, relayTicket, ticketLifetime } from "./relay.ts";
import { available, type Deps } from "./server-service.ts";

/** AssertionLifetime is how long a sign-in assertion is valid, in seconds. */
export const assertionLifetime = 120;

type User = typeof schema.user.$inferSelect;
type Record = typeof schema.accessRecords.$inferSelect;

const access = (r: Record): Access => fromJson(AccessSchema, r.access as never);

/** accountService implements AccountService. */
export function accountService({ db, apiURL, auth, relays, now = () => new Date() }: Deps & { auth: Auth }): ServiceImpl<typeof AccountService> {
  // signedIn returns the account of the request's session.
  async function signedIn(ctx: HandlerContext): Promise<User> {
    const session = await auth.api.getSession({ headers: ctx.requestHeader });
    if (!session) throw new ConnectError("sign in to mavio.dev", Code.Unauthenticated);
    const u = await db.query.user.findFirst({ where: eq(schema.user.id, session.user.id) });
    if (!u) throw new ConnectError("sign in to mavio.dev", Code.Unauthenticated);
    return u;
  }

  // full returns the account of the request's session, refusing managed accounts.
  async function full(ctx: HandlerContext): Promise<User> {
    const me = await signedIn(ctx);
    if (managed(me)) throw new ConnectError("managed accounts cannot do this", Code.PermissionDenied);
    return me;
  }

  // administered returns a server the account administers.
  async function administered(tx: typeof db | Tx, account: User, serverId: string) {
    const server = await tx.query.servers.findFirst({
      where: and(eq(schema.servers.id, serverId), eq(schema.servers.administratorId, account.id)),
    });
    if (!server) throw new ConnectError("no such server of yours", Code.NotFound);
    return server;
  }

  function recordToProto(r: Record, u: User) {
    return create(AccessRecordSchema, {
      account: accountOf(u),
      access: access(r),
      invitation: r.invitation,
      createTime: timestampFromDate(r.createdAt),
      updateTime: timestampFromDate(r.updatedAt),
    });
  }

  // checkAccess rejects records naming libraries the server did not publish.
  async function checkAccess(tx: Tx, serverId: string, a: Access) {
    if (a.allLibraries && a.libraryIds.length > 0) {
      throw new ConnectError("every library or some, not both", Code.InvalidArgument);
    }
    if (a.libraryIds.length === 0) return;
    const known = await tx
      .select({ id: schema.serverLibraries.id })
      .from(schema.serverLibraries)
      .where(and(eq(schema.serverLibraries.serverId, serverId), inArray(schema.serverLibraries.id, a.libraryIds)));
    if (known.length !== new Set(a.libraryIds).size) {
      throw new ConnectError("the server has no such library", Code.InvalidArgument);
    }
  }

  const granted = (tx: Tx, serverId: string, u: User, a: Access, at: Date) =>
    addChange(tx, serverId, { case: "accessGranted", value: create(AccessGrantedSchema, { account: accountOf(u), access: a }) }, at);
  const revoked = (tx: Tx, serverId: string, accountId: string, at: Date) =>
    addChange(tx, serverId, { case: "accessRevoked", value: create(AccessRevokedSchema, { accountId }) }, at);

  return {
    async getAccount(_, ctx) {
      const me = await signedIn(ctx);
      const at = now();
      const [pass, effective, pin] = await Promise.all([
        ownPass(db, me.id, at),
        effectivePass(db, me.id, at),
        db.query.pins.findFirst({ where: eq(schema.pins.accountId, me.id) }),
      ]);
      return create(GetAccountResponseSchema, {
        account: accountOf(me),
        email: managed(me) ? "" : me.email,
        passExpireTime: pass && timestampFromDate(pass),
        effectivePass: effective,
        hasPin: !!pin,
      });
    },

    async reportCompromised(_, ctx) {
      const me = await signedIn(ctx);
      await db.transaction(async (tx) => {
        await tx.delete(schema.session).where(eq(schema.session.userId, me.id));
        await accountRevoked(tx, me.id, AccountRevoked_Reason.COMPROMISED);
      });
      return create(ReportCompromisedResponseSchema);
    },

    async createClaimCode(_, ctx) {
      const me = await full(ctx);
      const code = newClaimCode();
      const expiresAt = new Date(now().getTime() + claimCodeLifetime);
      await db.insert(schema.claimCodes).values({ codeHash: await claimCodeHash(code), accountId: me.id, expiresAt });
      return create(CreateClaimCodeResponseSchema, { claimCode: code, expireTime: timestampFromDate(expiresAt) });
    },

    async listServers(_, ctx) {
      const me = await signedIn(ctx);
      const rows = await db.query.servers.findMany({
        with: {
          administrator: true,
          libraries: true,
          accessRecords: { where: and(eq(schema.accessRecords.accountId, me.id), eq(schema.accessRecords.invitation, false)) },
        },
        orderBy: (s, { asc }) => [asc(s.name), asc(s.id)],
      });
      const servers = rows
        .filter((s) => s.administratorId === me.id || s.accessRecords.length > 0)
        .map((s) => {
          const record = s.accessRecords[0];
          const reached = record ? access(record) : undefined;
          return create(ServerSchema, {
            id: s.id,
            publicKey: s.publicKey,
            name: s.name,
            version: s.version,
            administrator: accountOf(s.administrator),
            access: reached,
            libraries: s.libraries
              .filter((l) => !reached || reached.allLibraries || reached.libraryIds.includes(l.id))
              .sort((a, b) => a.name.localeCompare(b.name))
              .map((l) => create(LibrarySchema, { id: l.id, name: l.name, kind: l.kind })),
            claimTime: timestampFromDate(s.claimedAt),
          });
        });
      return create(ListServersResponseSchema, { servers });
    },

    async removeServer(req, ctx) {
      const me = await signedIn(ctx);
      await db.transaction(async (tx) => {
        await administered(tx, me, req.serverId);
        await tx.delete(schema.servers).where(eq(schema.servers.id, req.serverId));
      });
      return create(RemoveServerResponseSchema);
    },

    async createAssertion(req, ctx) {
      const me = await signedIn(ctx);
      const server = await db.query.servers.findFirst({
        where: eq(schema.servers.id, req.serverId),
        with: { accessRecords: { where: and(eq(schema.accessRecords.accountId, me.id), eq(schema.accessRecords.invitation, false)) } },
      });
      if (!server || (server.administratorId !== me.id && server.accessRecords.length === 0)) {
        throw new ConnectError("the account has no access to the server", Code.PermissionDenied);
      }
      const iat = Math.floor(now().getTime() / 1000);
      const exp = iat + assertionLifetime;
      const nonce = base64url(crypto.getRandomValues(new Uint8Array(16)));
      const { token } = await auth.api.signJWT({
        body: {
          payload: { iss: apiURL, aud: server.id, sub: me.id, iat, exp, device_key: base64url(req.deviceKey), nonce },
        },
      });
      return create(CreateAssertionResponseSchema, { assertion: token, expireTime: timestampFromDate(new Date(exp * 1000)) });
    },

    async createRelayTicket(req, ctx) {
      const me = await signedIn(ctx);
      const r = available(relays, "relays");
      const server = await db.query.servers.findFirst({
        where: eq(schema.servers.id, req.serverId),
        with: { accessRecords: { where: and(eq(schema.accessRecords.accountId, me.id), eq(schema.accessRecords.invitation, false)) } },
      });
      if (!server || (server.administratorId !== me.id && server.accessRecords.length === 0)) {
        throw new ConnectError("the account has no access to the server", Code.PermissionDenied);
      }
      if (!server.relayRegion || !r.urls.has(server.relayRegion)) {
        throw new ConnectError("the server keeps no relay connection", Code.FailedPrecondition);
      }
      const at = now();
      // The relay is a Mavio Pass feature, of the viewer or the administrator.
      const pass = (await effectivePass(db, me.id, at)) || (await effectivePass(db, server.administratorId, at));
      if (!pass) throw new ConnectError("the relay needs a Mavio Pass", Code.FailedPrecondition);
      const exp = Math.floor(at.getTime() / 1000) + ticketLifetime;
      const ticket = await relayTicket(r.secret, server.id, server.relayRegion, exp);
      return create(CreateRelayTicketResponseSchema, {
        host: relayHost(server.relayRegion, ticket, server.id, r.zone),
        expireTime: timestampFromDate(new Date(exp * 1000)),
      });
    },

    async listAccess(req, ctx) {
      const me = await signedIn(ctx);
      await administered(db, me, req.serverId);
      const rows = await db.query.accessRecords.findMany({
        where: eq(schema.accessRecords.serverId, req.serverId),
        with: { account: true },
      });
      rows.sort((a, b) => (a.account.username ?? "").localeCompare(b.account.username ?? ""));
      return create(ListAccessResponseSchema, { records: rows.map((r) => recordToProto(r, r.account)) });
    },

    async grantAccess(req, ctx) {
      const me = await signedIn(ctx);
      const a = req.access ?? create(AccessSchema);
      const at = now();
      return db.transaction(async (tx) => {
        await administered(tx, me, req.serverId);
        const u = await tx.query.user.findFirst({ where: eq(schema.user.username, req.userName.toLowerCase()) });
        if (!u) throw new ConnectError(`no account ${req.userName}`, Code.NotFound);
        if (u.id === me.id) throw new ConnectError("the administrator needs no access record", Code.InvalidArgument);
        await checkAccess(tx, req.serverId, a);
        if (managed(u)) {
          // A managed account reaches only its Home administrator's servers,
          // and its administrator's grants take effect at once.
          const home = await homeOf(tx, u.id);
          if (home?.administratorId !== me.id) {
            throw new ConnectError("a managed account reaches only its Home administrator's servers", Code.PermissionDenied);
          }
          const [record] = await tx
            .insert(schema.accessRecords)
            .values({ serverId: req.serverId, accountId: u.id, access: accessJson(a), invitation: false, createdAt: at, updatedAt: at })
            .onConflictDoUpdate({
              target: [schema.accessRecords.serverId, schema.accessRecords.accountId],
              set: { access: accessJson(a), invitation: false, updatedAt: at },
            })
            .returning();
          await granted(tx, req.serverId, u, a, at);
          return create(GrantAccessResponseSchema, { record: recordToProto(record, u) });
        }
        // Access to an account that is not yet a friend sends a friend request along.
        if (!(await tx.query.friendships.findFirst({ where: between(me.id, u.id) }))) {
          await tx.insert(schema.friendships).values({ requesterId: me.id, addresseeId: u.id, accepted: false, createdAt: at });
        }
        const key = and(eq(schema.accessRecords.serverId, req.serverId), eq(schema.accessRecords.accountId, u.id));
        const existing = await tx.query.accessRecords.findFirst({ where: key });
        let record: Record;
        if (existing) {
          [record] = await tx.update(schema.accessRecords).set({ access: accessJson(a), updatedAt: at }).where(key).returning();
          // A record in effect reaches the server at once; an invitation when accepted.
          if (!existing.invitation) await granted(tx, req.serverId, u, a, at);
        } else {
          [record] = await tx
            .insert(schema.accessRecords)
            .values({ serverId: req.serverId, accountId: u.id, access: accessJson(a), invitation: true, createdAt: at, updatedAt: at })
            .returning();
        }
        return create(GrantAccessResponseSchema, { record: recordToProto(record, u) });
      });
    },

    async revokeAccess(req, ctx) {
      const me = await signedIn(ctx);
      await db.transaction(async (tx) => {
        await administered(tx, me, req.serverId);
        const [r] = await tx
          .delete(schema.accessRecords)
          .where(and(eq(schema.accessRecords.serverId, req.serverId), eq(schema.accessRecords.accountId, req.accountId)))
          .returning();
        if (!r) throw new ConnectError("no such access record", Code.NotFound);
        if (!r.invitation) await revoked(tx, req.serverId, req.accountId, now());
      });
      return create(RevokeAccessResponseSchema);
    },

    async listInvitations(_, ctx) {
      const me = await signedIn(ctx);
      const rows = await db.query.accessRecords.findMany({
        where: and(eq(schema.accessRecords.accountId, me.id), eq(schema.accessRecords.invitation, true)),
        with: { server: { with: { administrator: true } } },
      });
      return create(ListInvitationsResponseSchema, {
        invitations: rows.map((r) =>
          create(InvitationSchema, {
            serverId: r.serverId,
            serverName: r.server.name,
            administrator: accountOf(r.server.administrator),
            access: access(r),
            createTime: timestampFromDate(r.createdAt),
          }),
        ),
      });
    },

    async acceptInvitation(req, ctx) {
      const me = await signedIn(ctx);
      const at = now();
      await db.transaction(async (tx) => {
        const [r] = await tx
          .update(schema.accessRecords)
          .set({ invitation: false, updatedAt: at })
          .where(
            and(
              eq(schema.accessRecords.serverId, req.serverId),
              eq(schema.accessRecords.accountId, me.id),
              eq(schema.accessRecords.invitation, true),
            ),
          )
          .returning();
        if (!r) throw new ConnectError("no such invitation", Code.NotFound);
        await granted(tx, req.serverId, me, access(r), at);
      });
      return create(AcceptInvitationResponseSchema);
    },

    async declineInvitation(req, ctx) {
      const me = await signedIn(ctx);
      const [r] = await db
        .delete(schema.accessRecords)
        .where(
          and(
            eq(schema.accessRecords.serverId, req.serverId),
            eq(schema.accessRecords.accountId, me.id),
            eq(schema.accessRecords.invitation, true),
          ),
        )
        .returning();
      if (!r) throw new ConnectError("no such invitation", Code.NotFound);
      return create(DeclineInvitationResponseSchema);
    },

    async leaveServer(req, ctx) {
      const me = await signedIn(ctx);
      await db.transaction(async (tx) => {
        const [r] = await tx
          .delete(schema.accessRecords)
          .where(
            and(
              eq(schema.accessRecords.serverId, req.serverId),
              eq(schema.accessRecords.accountId, me.id),
              eq(schema.accessRecords.invitation, false),
            ),
          )
          .returning();
        if (!r) throw new ConnectError("no access to the server", Code.NotFound);
        await revoked(tx, req.serverId, me.id, now());
      });
      return create(LeaveServerResponseSchema);
    },
  };
}

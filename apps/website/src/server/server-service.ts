// mavio.cloud.v1.ServerService (docs/mavio-dev.md §2), which servers call
// with JWTs signed by their identity keys.
import { create, fromJson } from "@bufbuild/protobuf";
import { Code, ConnectError, type HandlerContext, type ServiceImpl } from "@connectrpc/connect";
import {
  AccessRevokedSchema,
  ChangeSchema,
  ClaimServerResponseSchema,
  ClearAcmeChallengeResponseSchema,
  CreateAcmeBindingResponseSchema,
  ListChangesResponseSchema,
  PublishLibrariesResponseSchema,
  type ServerService,
  ListRelaysResponseSchema,
  SetAcmeChallengeResponseSchema,
  SetRelayRegionResponseSchema,
} from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { and, asc, eq, gt } from "drizzle-orm";

import type { Acme } from "./acme.ts";
import { accountOf, addChange } from "./changes.ts";
import { claimCodeHash } from "./claim-codes.ts";
import type { Database } from "./db/index.ts";
import { schema } from "./db/index.ts";
import type { Relays } from "./relay.ts";
import { bearer, keyID, verifyServerToken } from "./server-token.ts";

/** Deps are what mavio.dev's services use. */
export interface Deps {
  db: Database;
  /** The API's base URL, servers' audience and assertions' issuer. */
  apiURL: string;
  now?: () => Date;
  /** Servers' ACME challenges and bindings; undefined where certificates are not offered. */
  acme?: Acme;
  /** The relays; undefined where there are none. */
  relays?: Relays;
}

const defaultPageSize = 100;

/** authenticateServer returns the claimed server whose JWT the request carries. */
export async function authenticateServer({ db, apiURL, now = () => new Date() }: Deps, headers: Headers) {
  const token = bearer(headers);
  const server = await db.query.servers.findFirst({ where: eq(schema.servers.id, keyID(token)) });
  if (!server) throw new ConnectError("the server is not claimed", Code.Unauthenticated);
  await verifyServerToken(db, token, server.publicKey, apiURL, now());
  return server;
}

/** serverService implements ServerService. */
export function serverService(deps: Deps): ServiceImpl<typeof ServerService> {
  const { db, apiURL, now = () => new Date() } = deps;
  const authenticate = (ctx: HandlerContext) => authenticateServer(deps, ctx.requestHeader);

  return {
    async claimServer(req, ctx) {
      const at = now();
      const id = await verifyServerToken(db, bearer(ctx.requestHeader), req.publicKey, apiURL, at);
      const codeHash = await claimCodeHash(req.claimCode);
      return db.transaction(async (tx) => {
        const [code] = await tx.delete(schema.claimCodes).where(eq(schema.claimCodes.codeHash, codeHash)).returning();
        if (!code || code.expiresAt <= at) {
          throw new ConnectError("unknown, used or expired claim code", Code.InvalidArgument);
        }
        const admin = await tx.query.user.findFirst({ where: eq(schema.user.id, code.accountId) });
        if (!admin) throw new ConnectError("unknown, used or expired claim code", Code.InvalidArgument);
        const existing = await tx.query.servers.findFirst({ where: eq(schema.servers.id, id) });
        let cursor = 0;
        const fields = { name: req.name, version: req.version, administratorId: admin.id, claimedAt: at };
        if (existing) {
          // Claimed again after its command line unclaimed it: the changes
          // stay, and a previous administrator keeps no access.
          await tx.update(schema.servers).set(fields).where(eq(schema.servers.id, id));
          cursor = existing.cursor;
          if (existing.administratorId !== admin.id) {
            await addChange(
              tx,
              id,
              { case: "accessRevoked", value: create(AccessRevokedSchema, { accountId: existing.administratorId }) },
              at,
            );
          }
        } else {
          await tx.insert(schema.servers).values({ id, publicKey: req.publicKey, ...fields });
        }
        // The administrator reaches every library without a record.
        await tx
          .delete(schema.accessRecords)
          .where(and(eq(schema.accessRecords.serverId, id), eq(schema.accessRecords.accountId, admin.id)));
        return create(ClaimServerResponseSchema, { administrator: accountOf(admin), cursor: BigInt(cursor) });
      });
    },

    async listChanges(req, ctx) {
      const server = await authenticate(ctx);
      const size = req.pageSize || defaultPageSize;
      const rows = await db
        .select()
        .from(schema.changes)
        .where(and(eq(schema.changes.serverId, server.id), gt(schema.changes.cursor, Number(req.after))))
        .orderBy(asc(schema.changes.cursor))
        .limit(size + 1);
      const page = rows.slice(0, size);
      return create(ListChangesResponseSchema, {
        changes: page.map((r) => fromJson(ChangeSchema, r.change as never)),
        more: rows.length > size,
        cursor: page.length > 0 ? BigInt(page[page.length - 1].cursor) : req.after,
      });
    },

    async publishLibraries(req, ctx) {
      const server = await authenticate(ctx);
      await db.transaction(async (tx) => {
        await tx.delete(schema.serverLibraries).where(eq(schema.serverLibraries.serverId, server.id));
        if (req.libraries.length > 0) {
          await tx
            .insert(schema.serverLibraries)
            .values(req.libraries.map((l) => ({ serverId: server.id, id: l.id, name: l.name, kind: l.kind })))
            .onConflictDoNothing();
        }
      });
      return create(PublishLibrariesResponseSchema);
    },

    async setAcmeChallenge(req, ctx) {
      const server = await authenticate(ctx);
      const acme = available(deps.acme);
      await acme.dns.set(`${server.id}.${acme.domain}`, req.values);
      return create(SetAcmeChallengeResponseSchema);
    },

    async clearAcmeChallenge(_req, ctx) {
      const server = await authenticate(ctx);
      const acme = available(deps.acme);
      await acme.dns.clear(`${server.id}.${acme.domain}`);
      return create(ClearAcmeChallengeResponseSchema);
    },

    async createAcmeBinding(_req, ctx) {
      await authenticate(ctx);
      const bindings = available(deps.acme?.bindings);
      return create(CreateAcmeBindingResponseSchema, await bindings.create());
    },

    async listRelays(_req, ctx) {
      await authenticate(ctx);
      const urls = deps.relays?.urls ?? new Map<string, string>();
      return create(ListRelaysResponseSchema, { relays: [...urls].map(([region, url]) => ({ region, url })) });
    },

    async setRelayRegion(req, ctx) {
      const server = await authenticate(ctx);
      if (req.region && !deps.relays?.urls.has(req.region)) {
        throw new ConnectError(`no relay in region ${req.region}`, Code.InvalidArgument);
      }
      await db.update(schema.servers).set({ relayRegion: req.region }).where(eq(schema.servers.id, server.id));
      return create(SetRelayRegionResponseSchema);
    },
  };
}

/** available returns what a method needs, which a deployment may lack. */
export function available<T>(v: T | undefined, what = "certificates"): T {
  if (!v) throw new ConnectError(`${what} are not available here`, Code.Unimplemented);
  return v;
}

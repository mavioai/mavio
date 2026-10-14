// mavio.cloud.v1.ServerService (docs/mavio-dev.md §2), which servers call
// with JWTs signed by their identity keys.
import { create, fromJson } from "@bufbuild/protobuf";
import { Code, ConnectError, type HandlerContext, type ServiceImpl } from "@connectrpc/connect";
import {
  AccessRevokedSchema,
  ChangeSchema,
  ClaimServerResponseSchema,
  ListChangesResponseSchema,
  PublishLibrariesResponseSchema,
  type ServerService,
} from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { and, asc, eq, gt } from "drizzle-orm";

import { accountOf, addChange } from "./changes.ts";
import { claimCodeHash } from "./claim-codes.ts";
import type { Database } from "./db/index.ts";
import { schema } from "./db/index.ts";
import { bearer, keyID, verifyServerToken } from "./server-token.ts";

/** Deps are what mavio.dev's services use. */
export interface Deps {
  db: Database;
  /** The API's base URL, servers' audience and assertions' issuer. */
  apiURL: string;
  now?: () => Date;
}

const defaultPageSize = 100;

/** serverService implements ServerService. */
export function serverService({ db, apiURL, now = () => new Date() }: Deps): ServiceImpl<typeof ServerService> {
  // authenticate returns the claimed server that signed the request.
  async function authenticate(ctx: HandlerContext) {
    const token = bearer(ctx.requestHeader);
    const server = await db.query.servers.findFirst({ where: eq(schema.servers.id, keyID(token)) });
    if (!server) throw new ConnectError("the server is not claimed", Code.Unauthenticated);
    await verifyServerToken(db, token, server.publicKey, apiURL, now());
    return server;
  }

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
  };
}

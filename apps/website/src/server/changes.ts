// The changes mavio.dev keeps for each server (docs/mavio-dev.md §3): every
// change takes the server's next cursor, and servers fetch the changes after
// their own.
import { create, toJson } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
  type Access,
  AccessSchema,
  type Account,
  AccountKind,
  AccountRevoked_Reason,
  AccountRevokedSchema,
  AccountSchema,
  type Change,
  ChangeSchema,
} from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { RestrictionProfile } from "@mavio/proto/mavio/user/v1/user_pb";
import { and, eq, or, sql } from "drizzle-orm";

import { type Database, schema, type Tx } from "./db/index.ts";

type Kind = Change["kind"];

/** addChange records a change for a server and returns its cursor. */
export async function addChange(tx: Database | Tx, serverId: string, kind: Kind, now = new Date()): Promise<number> {
  const [s] = await tx
    .update(schema.servers)
    .set({ cursor: sql`${schema.servers.cursor} + 1` })
    .where(eq(schema.servers.id, serverId))
    .returning({ cursor: schema.servers.cursor });
  if (!s) throw new Error(`no server ${serverId}`);
  const change = create(ChangeSchema, { cursor: BigInt(s.cursor), createTime: timestampFromDate(now), kind });
  await tx.insert(schema.changes).values({ serverId, cursor: s.cursor, change: toJson(ChangeSchema, change), createdAt: now });
  return s.cursor;
}

/** AccountUser is what accountOf reads of a better-auth user. */
export interface AccountUser {
  id: string;
  name: string;
  username?: string | null;
  displayUsername?: string | null;
  kind?: string | null;
  restriction?: number | null;
}

/** managed reports whether a user is a managed account. */
export const managed = (u: AccountUser) => u.kind === "managed";

/** accountOf returns the account of a better-auth user as servers see it. */
export function accountOf(u: AccountUser): Account {
  return create(AccountSchema, {
    id: u.id,
    userName: u.displayUsername || u.username || "",
    displayName: u.name,
    kind: managed(u) ? AccountKind.MANAGED : AccountKind.FULL,
    restriction: managed(u) ? ((u.restriction ?? 0) as RestrictionProfile) : RestrictionProfile.UNSPECIFIED,
  });
}

/** accessJson returns an access record's access in its stored form. */
export function accessJson(a: Access) {
  return toJson(AccessSchema, a);
}

/** reachedServers returns the IDs of the servers an account administers or has access to. */
export async function reachedServers(tx: Database | Tx, accountId: string): Promise<string[]> {
  const rows = await tx
    .selectDistinct({ id: schema.servers.id })
    .from(schema.servers)
    .leftJoin(
      schema.accessRecords,
      and(eq(schema.accessRecords.serverId, schema.servers.id), eq(schema.accessRecords.invitation, false)),
    )
    .where(or(eq(schema.servers.administratorId, accountId), eq(schema.accessRecords.accountId, accountId)));
  return rows.map((r) => r.id);
}

/** accountChanged tells an account's servers its names, kind and restriction profile. */
export async function accountChanged(db: Database | Tx, account: Account): Promise<void> {
  const record = async (tx: Database | Tx) => {
    for (const id of await reachedServers(tx, account.id)) {
      await addChange(tx, id, { case: "accountChanged", value: account });
    }
  };
  if ("rollback" in db) await record(db);
  else await db.transaction(record);
}

/** accountRevoked tells an account's servers that it was suspended, deleted or reported compromised. */
export async function accountRevoked(db: Database | Tx, accountId: string, reason: AccountRevoked_Reason): Promise<void> {
  const record = async (tx: Database | Tx) => {
    for (const id of await reachedServers(tx, accountId)) {
      await addChange(tx, id, { case: "accountRevoked", value: create(AccountRevokedSchema, { accountId, reason }) });
    }
  };
  if ("rollback" in db) await record(db);
  else await db.transaction(record);
}

export { AccountRevoked_Reason };

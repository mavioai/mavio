// Homes, friends, PINs and Mavio Pass (docs/accounts.md §3.4–§3.7): what
// HomeService, FriendService and switching share.
import { Code, ConnectError } from "@connectrpc/connect";
import { and, eq, or } from "drizzle-orm";

import { AccountRevoked_Reason, accountRevoked } from "./changes.ts";
import { type Database, schema, type Tx } from "./db/index.ts";
import * as password from "./password.ts";

type Db = Database | Tx;
type User = typeof schema.user.$inferSelect;
type Home = typeof schema.homes.$inferSelect;

/** maxMembers is how many accounts a Home holds, its administrator included. */
export const maxMembers = 15;

/** pinFailuresForPassword is how many wrong PINs make switching to a full account need its password. */
export const pinFailuresForPassword = 5;

/** pinDelay is how long switching waits after the given number of wrong PINs. */
export function pinDelay(failures: number): number {
  if (failures < 3) return 0;
  return Math.min(30_000 * 2 ** (failures - 3), 15 * 60_000);
}

/** homeOf returns the Home an account belongs to. */
export async function homeOf(db: Db, accountId: string): Promise<Home | undefined> {
  const m = await db.query.homeMembers.findFirst({ where: eq(schema.homeMembers.accountId, accountId), with: { home: true } });
  return m?.home;
}

/** ownPass returns when an account's own Mavio Pass ends, if it has one in effect. */
export async function ownPass(db: Db, accountId: string, at: Date): Promise<Date | undefined> {
  const p = await db.query.passes.findFirst({ where: eq(schema.passes.accountId, accountId) });
  return p && p.expiresAt > at ? p.expiresAt : undefined;
}

/** frozen reports whether a Home's administrator's Mavio Pass lapsed. */
export async function frozen(db: Db, home: Home, at: Date): Promise<boolean> {
  return !(await ownPass(db, home.administratorId, at));
}

/** effectivePass reports whether an account has its own Pass, or its Home administrator's while the Home is not frozen. */
export async function effectivePass(db: Db, accountId: string, at: Date): Promise<boolean> {
  if (await ownPass(db, accountId, at)) return true;
  const home = await homeOf(db, accountId);
  return !!home && home.administratorId !== accountId && !(await frozen(db, home, at));
}

/** befriend makes two accounts friends, replacing any request between them. */
export async function befriend(db: Db, a: string, b: string, at: Date): Promise<void> {
  await db.delete(schema.friendships).where(between(a, b));
  await db.insert(schema.friendships).values({ requesterId: a, addresseeId: b, accepted: true, createdAt: at });
}

/** between matches the friendship rows of two accounts, either way. */
export function between(a: string, b: string) {
  return or(
    and(eq(schema.friendships.requesterId, a), eq(schema.friendships.addresseeId, b)),
    and(eq(schema.friendships.requesterId, b), eq(schema.friendships.addresseeId, a)),
  );
}

/** deleteManaged deletes a managed account, telling its servers to delete its user and data. */
export async function deleteManaged(tx: Tx, accountId: string): Promise<void> {
  await accountRevoked(tx, accountId, AccountRevoked_Reason.DELETED);
  await tx.delete(schema.user).where(eq(schema.user.id, accountId));
}

/** removeMember takes an account out of its Home: a full member becomes the administrator's friend, a managed account is deleted. */
export async function removeMember(tx: Tx, home: Home, member: User, at: Date): Promise<void> {
  if (member.kind === "managed") {
    await deleteManaged(tx, member.id);
    return;
  }
  await tx.delete(schema.homeMembers).where(eq(schema.homeMembers.accountId, member.id));
  await befriend(tx, home.administratorId, member.id, at);
}

/** setPin sets an account's PIN, or clears it when pin is empty. */
export async function setPin(db: Db, accountId: string, pin: string): Promise<void> {
  if (!pin) {
    await db.delete(schema.pins).where(eq(schema.pins.accountId, accountId));
    return;
  }
  const hash = await password.hash(pin);
  await db
    .insert(schema.pins)
    .values({ accountId, hash })
    .onConflictDoUpdate({ target: schema.pins.accountId, set: { hash, failures: 0, retryAt: null } });
}

/** Proof is what switching to an account with a PIN presents. */
export interface Proof {
  pin?: string;
  /** The account's password, which switching to a full account needs after too many wrong PINs. */
  password?: string;
  /** The hash of the account's password, if it has one. */
  passwordHash?: string;
}

/**
 * checkPin checks the proof for switching to an account, counting wrong
 * PINs; it throws when switching is refused.
 */
export async function checkPin(db: Database, target: User, proof: Proof, at: Date): Promise<void> {
  const pin = await db.query.pins.findFirst({ where: eq(schema.pins.accountId, target.id) });
  if (!pin) return;
  if (pin.retryAt && pin.retryAt > at) {
    throw new ConnectError("too many wrong PINs; try again later", Code.ResourceExhausted);
  }
  const reset = () => db.update(schema.pins).set({ failures: 0, retryAt: null }).where(eq(schema.pins.accountId, target.id));
  if (target.kind !== "managed" && pin.failures >= pinFailuresForPassword) {
    if (!proof.password || !proof.passwordHash || !(await password.verify({ hash: proof.passwordHash, password: proof.password }))) {
      throw new ConnectError("too many wrong PINs; enter the account's password", Code.PermissionDenied);
    }
    await reset();
    return;
  }
  if (!proof.pin) throw new ConnectError("enter the account's PIN", Code.PermissionDenied);
  if (await password.verify({ hash: pin.hash, password: proof.pin })) {
    await reset();
    return;
  }
  const failures = pin.failures + 1;
  const delay = pinDelay(failures);
  await db
    .update(schema.pins)
    .set({ failures, retryAt: delay ? new Date(at.getTime() + delay) : null })
    .where(eq(schema.pins.accountId, target.id));
  throw new ConnectError("wrong PIN", Code.PermissionDenied);
}

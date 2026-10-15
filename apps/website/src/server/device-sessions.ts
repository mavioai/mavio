// Devices' sessions at mavio.dev (docs/mavio-dev.md §2): a web UI or an
// app holds a session of its device key, given by the token endpoint, and
// calls the API with a short access token, which it renews by signing the
// time with the key. Access tokens are mds1.<claims>.<HMAC-SHA256 of the
// claims under better-auth's secret>, and every request checks that their
// session still exists, so that ending a session takes effect at once.
import { Code, ConnectError } from "@connectrpc/connect";
import { eq } from "drizzle-orm";

import type { Auth } from "./auth.ts";
import { type Database, schema } from "./db/index.ts";
import { base64url, fromBase64url } from "./ids.ts";

type User = typeof schema.user.$inferSelect;
type Session = typeof schema.deviceSessions.$inferSelect;

/** accessTokenLifetime is how long an access token is valid, in seconds. */
export const accessTokenLifetime = 600;

/** renewalSkew is how far a renewal's time may be from mavio.dev's, in milliseconds. */
export const renewalSkew = 120_000;

/** appClient is the client ID of app.mavio.dev. */
export const appClient = "app";

const prefix = "mds1";
// lastSeenAt is written at most this often, in milliseconds.
const seenEvery = 5 * 60_000;

/** appClientID reports whether a client ID is app.mavio.dev's or one of Mavio's apps' reverse-DNS IDs. */
export function appClientID(id: string): boolean {
  return id === appClient || /^dev\.mavio\.[a-z0-9.-]+$/.test(id);
}

async function hmac(secret: string, data: string): Promise<Uint8Array> {
  const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, [
    "sign",
  ]);
  return new Uint8Array(await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(data)));
}

/** createDeviceSession starts a session of a device key for an account. */
export async function createDeviceSession(
  db: Database,
  s: { accountId: string; clientId: string; deviceKey: Uint8Array },
  at: Date,
): Promise<Session> {
  const [row] = await db
    .insert(schema.deviceSessions)
    .values({ id: base64url(crypto.getRandomValues(new Uint8Array(18))), ...s, createdAt: at, lastSeenAt: at })
    .returning();
  return row;
}

/** accessToken returns an access token of a session, valid accessTokenLifetime seconds. */
export async function accessToken(secret: string, session: Session, at: Date): Promise<string> {
  const exp = Math.floor(at.getTime() / 1000) + accessTokenLifetime;
  const claims = base64url(new TextEncoder().encode(JSON.stringify({ sid: session.id, sub: session.accountId, exp })));
  return `${prefix}.${claims}.${base64url(await hmac(secret, `${prefix}.${claims}`))}`;
}

/**
 * readAccessToken returns the session ID and account of an access token
 * signed with secret, undefined when it is not one or, unless expired is
 * accepted, has expired.
 */
export async function readAccessToken(
  secret: string,
  token: string,
  at: Date,
  acceptExpired = false,
): Promise<{ sid: string; sub: string } | undefined> {
  const parts = token.split(".");
  if (parts.length !== 3 || parts[0] !== prefix) return undefined;
  const mac = fromBase64url(parts[2]);
  const want = await hmac(secret, `${prefix}.${parts[1]}`);
  if (!mac || mac.length !== want.length || !equal(mac, want)) return undefined;
  let claims: unknown;
  try {
    claims = JSON.parse(new TextDecoder().decode(fromBase64url(parts[1])));
  } catch {
    return undefined;
  }
  const { sid, sub, exp } = (claims ?? {}) as { sid?: unknown; sub?: unknown; exp?: unknown };
  if (typeof sid !== "string" || typeof sub !== "string" || typeof exp !== "number") return undefined;
  if (!acceptExpired && exp <= Math.floor(at.getTime() / 1000)) return undefined;
  return { sid, sub };
}

function equal(a: Uint8Array, b: Uint8Array): boolean {
  let d = 0;
  for (let i = 0; i < a.length; i++) d |= a[i] ^ b[i];
  return d === 0;
}

/** renewalMessage returns what a device signs to renew a session at a time in milliseconds. */
export function renewalMessage(sessionId: string, timestamp: number): Uint8Array<ArrayBuffer> {
  return new TextEncoder().encode(`mavio-dev-renew-v1\0${sessionId}\0${timestamp}`);
}

/**
 * renewDeviceSession checks a device's signature of a time, later than
 * its last renewal and within renewalSkew of at, and returns the session.
 */
export async function renewDeviceSession(
  db: Database,
  r: { sessionId: string; timestamp: number; signature: Uint8Array },
  at: Date,
): Promise<Session | undefined> {
  const session = await db.query.deviceSessions.findFirst({ where: eq(schema.deviceSessions.id, r.sessionId) });
  if (!session || !Number.isSafeInteger(r.timestamp)) return undefined;
  if (r.timestamp <= session.renewedAt || Math.abs(r.timestamp - at.getTime()) > renewalSkew) return undefined;
  const key = await crypto.subtle.importKey("raw", session.deviceKey.slice(), { name: "Ed25519" }, false, ["verify"]);
  if (!(await crypto.subtle.verify({ name: "Ed25519" }, key, r.signature.slice(), renewalMessage(session.id, r.timestamp)))) {
    return undefined;
  }
  const [renewed] = await db
    .update(schema.deviceSessions)
    .set({ renewedAt: r.timestamp, lastSeenAt: at })
    .where(eq(schema.deviceSessions.id, session.id))
    .returning();
  return renewed;
}

/** endDeviceSession ends the session of an access token, expired or not; it reports whether there was one. */
export async function endDeviceSession(db: Database, secret: string, token: string, at: Date): Promise<boolean> {
  const t = await readAccessToken(secret, token, at, true);
  if (!t) return false;
  const ended = await db.delete(schema.deviceSessions).where(eq(schema.deviceSessions.id, t.sid)).returning();
  return ended.length > 0;
}

/**
 * accountOfRequest returns the account of a request: the session of a
 * device's access token, or a session of better-auth's cookie or bearer
 * token. It is undefined when the request has none.
 */
export async function accountOfRequest(db: Database, auth: Auth, headers: Headers, at: Date): Promise<User | undefined> {
  const m = /^Bearer (mds1\.\S+)$/.exec(headers.get("authorization") ?? "");
  if (m) {
    const t = await readAccessToken((await auth.$context).secret, m[1], at);
    const session = t && (await db.query.deviceSessions.findFirst({ where: eq(schema.deviceSessions.id, t.sid) }));
    if (!session || session.accountId !== t.sub) return undefined;
    if (at.getTime() - session.lastSeenAt.getTime() > seenEvery) {
      await db.update(schema.deviceSessions).set({ lastSeenAt: at }).where(eq(schema.deviceSessions.id, session.id));
    }
    return db.query.user.findFirst({ where: eq(schema.user.id, session.accountId) });
  }
  const session = await auth.api.getSession({ headers });
  return session ? db.query.user.findFirst({ where: eq(schema.user.id, session.user.id) }) : undefined;
}

/** signedIn returns the account of a request, or fails with UNAUTHENTICATED. */
export async function signedIn(db: Database, auth: Auth, headers: Headers, at: Date): Promise<User> {
  const u = await accountOfRequest(db, auth, headers, at);
  if (!u) throw new ConnectError("sign in to mavio.dev", Code.Unauthenticated);
  return u;
}

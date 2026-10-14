// Servers' JWTs (private_key_jwt, docs/mavio-dev.md §2): EdDSA, signed with
// the server's identity key, with the server's ID as kid, iss and sub, the
// API's base URL as aud, a random jti accepted once, and an expiry at most
// a minute after it was issued.
import { Code, ConnectError } from "@connectrpc/connect";
import { lt } from "drizzle-orm";

import { type Database, schema } from "./db/index.ts";
import { fromBase64url, serverID } from "./ids.ts";

const lifetime = 60;
const skew = 30;

interface Claims {
  iss?: unknown;
  sub?: unknown;
  aud?: unknown;
  iat?: unknown;
  exp?: unknown;
  jti?: unknown;
}

const unauthenticated = (why: string) => new ConnectError(`invalid server token: ${why}`, Code.Unauthenticated);

/** bearer returns the bearer token of a request's headers. */
export function bearer(headers: Headers): string {
  const m = /^Bearer (\S+)$/.exec(headers.get("authorization") ?? "");
  if (!m) throw new ConnectError("a server token is required", Code.Unauthenticated);
  return m[1];
}

/** keyID returns the kid of a JWT's header, unverified. */
export function keyID(token: string): string {
  const header = decode(token.split(".")[0] ?? "");
  if (typeof header?.kid !== "string") throw unauthenticated("no key ID");
  return header.kid;
}

function decode(part: string): Record<string, unknown> | undefined {
  const raw = fromBase64url(part);
  if (!raw) return undefined;
  try {
    const v = JSON.parse(new TextDecoder().decode(raw));
    return typeof v === "object" && v !== null ? v : undefined;
  } catch {
    return undefined;
  }
}

/**
 * verifyServerToken checks a server's JWT against its identity public key
 * and records its jti, returning the server's ID.
 */
export async function verifyServerToken(
  db: Database,
  token: string,
  publicKey: Uint8Array,
  audience: string,
  now = new Date(),
): Promise<string> {
  const parts = token.split(".");
  if (parts.length !== 3) throw unauthenticated("not a JWT");
  const header = decode(parts[0]);
  const claims = decode(parts[1]) as Claims | undefined;
  const signature = fromBase64url(parts[2]);
  if (!header || !claims || !signature) throw unauthenticated("malformed");
  if (header.alg !== "EdDSA") throw unauthenticated("not signed with EdDSA");
  const id = await serverID(publicKey);
  const key = await crypto.subtle.importKey("raw", publicKey.slice(), { name: "Ed25519" }, false, ["verify"]);
  const signed = new TextEncoder().encode(`${parts[0]}.${parts[1]}`);
  if (!(await crypto.subtle.verify({ name: "Ed25519" }, key, signature, signed))) throw unauthenticated("bad signature");
  const t = Math.floor(now.getTime() / 1000);
  const { iss, sub, aud, iat, exp, jti } = claims;
  if (header.kid !== id || iss !== id || sub !== id) throw unauthenticated("for another server");
  if (aud !== audience) throw unauthenticated("for another audience");
  if (typeof iat !== "number" || typeof exp !== "number" || typeof jti !== "string" || jti === "") {
    throw unauthenticated("missing claims");
  }
  if (t > exp + skew) throw unauthenticated("expired");
  if (exp - iat > lifetime || iat > t + skew) throw unauthenticated("valid for too long");
  await db.delete(schema.serverTokens).where(lt(schema.serverTokens.expiresAt, now));
  const accepted = await db
    .insert(schema.serverTokens)
    .values({ id: `${id}:${jti}`, expiresAt: new Date((exp + skew) * 1000) })
    .onConflictDoNothing()
    .returning({ id: schema.serverTokens.id });
  if (accepted.length === 0) throw unauthenticated("already used");
  return id;
}

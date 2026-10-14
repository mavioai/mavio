// Signing accounts in to servers' web interfaces (docs/accounts.md §6): an
// authorization code flow with PKCE whose client is the server. The
// account approves at /authorize, which gets a code from
// AccountService.CreateAuthorizationCode and sends it to the redirect URI;
// the web interface exchanges it at /api/oauth/token, with the verifier,
// for a sign-in assertion of the device key it named.
import { toJson } from "@bufbuild/protobuf";
import { AccountSchema } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { and, eq, lt } from "drizzle-orm";

import type { Auth } from "./auth.ts";
import { accountOf } from "./changes.ts";
import { type Database, schema } from "./db/index.ts";
import { base64url } from "./ids.ts";

/** AssertionLifetime is how long a sign-in assertion is valid, in seconds. */
export const assertionLifetime = 120;

/** AuthorizationCodeLifetime is how long an authorization code is valid, in milliseconds. */
export const authorizationCodeLifetime = 60_000;

/** tokenPath is where web interfaces exchange their codes. */
export const tokenPath = "/api/oauth/token";

/** hasAccess reports whether an account administers a server or has access to it. */
export async function hasAccess(db: Database, serverId: string, accountId: string): Promise<boolean> {
  const server = await db.query.servers.findFirst({
    where: eq(schema.servers.id, serverId),
    with: {
      accessRecords: { where: and(eq(schema.accessRecords.accountId, accountId), eq(schema.accessRecords.invitation, false)) },
    },
  });
  return !!server && (server.administratorId === accountId || server.accessRecords.length > 0);
}

/** signAssertion returns a sign-in assertion of an account for a server and a device key (docs/accounts.md §5.5). */
export async function signAssertion(
  auth: Auth,
  apiURL: string,
  serverId: string,
  accountId: string,
  deviceKey: Uint8Array,
  at: Date,
): Promise<{ assertion: string; exp: number }> {
  const iat = Math.floor(at.getTime() / 1000);
  const exp = iat + assertionLifetime;
  const nonce = base64url(crypto.getRandomValues(new Uint8Array(16)));
  const { token } = await auth.api.signJWT({
    body: { payload: { iss: apiURL, aud: serverId, sub: accountId, iat, exp, device_key: base64url(deviceKey), nonce } },
  });
  return { assertion: token, exp };
}

/** newAuthorizationCode returns a random code. */
export function newAuthorizationCode(): string {
  return base64url(crypto.getRandomValues(new Uint8Array(32)));
}

/** sha256Hex returns the hex SHA-256 of a string, as codes are stored. */
export async function sha256Hex(s: string): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(s));
  return Buffer.from(digest).toString("hex");
}

/** s256 returns the S256 PKCE challenge of a verifier (RFC 7636 §4.2). */
export async function s256(verifier: string): Promise<string> {
  return base64url(new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier))));
}

/** redirectURI returns a redirect URI parsed, or undefined unless it is an HTTP or HTTPS URL without a fragment. */
export function redirectURI(s: string): URL | undefined {
  let u: URL;
  try {
    u = new URL(s);
  } catch {
    return undefined;
  }
  if ((u.protocol !== "http:" && u.protocol !== "https:") || u.hash !== "" || u.username !== "" || u.password !== "") {
    return undefined;
  }
  return u;
}

/** deleteExpiredCodes removes an account's codes that expired. */
export async function deleteExpiredCodes(db: Database, accountId: string, at: Date) {
  await db
    .delete(schema.authorizationCodes)
    .where(and(eq(schema.authorizationCodes.accountId, accountId), lt(schema.authorizationCodes.expiresAt, at)));
}

interface TokenDeps {
  db: Database;
  auth: Auth;
  apiURL: string;
  now: () => Date;
}

/**
 * handleToken exchanges an authorization code (RFC 6749 §4.1.3) for a
 * sign-in assertion. The request is a form, which browsers send across
 * origins without a preflight; the answer allows the redirect URI's origin.
 */
export async function handleToken(req: Request, { db, auth, apiURL, now }: TokenDeps): Promise<Response> {
  const origin = req.headers.get("origin");
  const cors: Record<string, string> = origin ? { "access-control-allow-origin": origin, vary: "Origin" } : {};
  if (req.method === "OPTIONS") {
    return new Response(null, {
      status: 204,
      headers: { ...cors, "access-control-allow-methods": "POST", "access-control-allow-headers": "content-type" },
    });
  }
  const fail = (status: number, error: string, description: string) =>
    Response.json({ error, error_description: description }, { status, headers: { ...cors, "cache-control": "no-store" } });
  if (req.method !== "POST") return fail(405, "invalid_request", "POST the form");
  let form: FormData;
  try {
    form = await req.formData();
  } catch {
    return fail(400, "invalid_request", "the request is not a form");
  }
  const field = (k: string) => {
    const v = form.get(k);
    return typeof v === "string" ? v : "";
  };
  if (field("grant_type") !== "authorization_code") {
    return fail(400, "unsupported_grant_type", "only authorization_code is supported");
  }
  const code = field("code");
  if (!code) return fail(400, "invalid_request", "no code");
  // A code is exchanged once, whatever comes of it.
  const [row] = await db
    .delete(schema.authorizationCodes)
    .where(eq(schema.authorizationCodes.codeHash, await sha256Hex(code)))
    .returning();
  const at = now();
  if (
    !row ||
    row.expiresAt < at ||
    row.serverId !== field("client_id") ||
    row.redirectUri !== field("redirect_uri") ||
    row.codeChallenge !== (await s256(field("code_verifier")))
  ) {
    return fail(400, "invalid_grant", "the code is unknown, expired or not this client's");
  }
  if (origin && origin !== new URL(row.redirectUri).origin) {
    return fail(400, "invalid_grant", "the code was given to another origin");
  }
  if (!(await hasAccess(db, row.serverId, row.accountId))) {
    return fail(400, "invalid_grant", "the account no longer has access to the server");
  }
  const account = await db.query.user.findFirst({ where: eq(schema.user.id, row.accountId) });
  if (!account) return fail(400, "invalid_grant", "the account was deleted");
  const { assertion, exp } = await signAssertion(auth, apiURL, row.serverId, row.accountId, row.deviceKey, at);
  return Response.json(
    {
      assertion,
      expires_in: exp - Math.floor(at.getTime() / 1000),
      account: toJson(AccountSchema, accountOf(account)),
    },
    { headers: { ...cors, "cache-control": "no-store" } },
  );
}

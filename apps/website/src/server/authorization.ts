// Signing devices in to mavio.dev (docs/accounts.md §6): an authorization
// code flow with PKCE whose client is a server's web UI, by the server's
// ID, or app.mavio.dev; and link codes for televisions. The account
// approves at /authorize, which gets a code from
// AccountService.CreateAuthorizationCode and sends it to the redirect URI,
// or at /claim, or at /link; the device then gets a session of its key at
// /api/oauth/token (device-sessions.ts), with which it asks for sign-in
// assertions of its servers.
import { toJson } from "@bufbuild/protobuf";
import { AccountSchema } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { and, eq, lt } from "drizzle-orm";

import type { Auth } from "./auth.ts";
import { accountOf } from "./changes.ts";
import { type Database, schema } from "./db/index.ts";
import {
  accessToken,
  accessTokenLifetime,
  appClient,
  appClientID,
  createDeviceSession,
  endDeviceSession,
  renewDeviceSession,
} from "./device-sessions.ts";
import { base64url, fromBase64url } from "./ids.ts";
import { linkCodeLifetime, linkPollInterval, newUserCode, userCodeKey } from "./link-codes.ts";

/** AssertionLifetime is how long a sign-in assertion is valid, in seconds. */
export const assertionLifetime = 120;

/** AuthorizationCodeLifetime is how long an authorization code is valid, in milliseconds. */
export const authorizationCodeLifetime = 60_000;

/** tokenPath is the token endpoint. */
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

/** OAuthDeps are what the OAuth endpoints use. */
export interface OAuthDeps {
  db: Database;
  auth: Auth;
  /** The site's origin, such as https://mavio.dev. */
  baseURL: string;
  now: () => Date;
}

/** renewGrant is the grant type that renews a device's session with a signature of its key. */
export const renewGrant = "urn:mavio:params:oauth:grant-type:device-signature";

const deviceCodeGrant = "urn:ietf:params:oauth:grant-type:device_code";

/** cors returns the CORS headers of an answer to any origin, without credentials. */
export function cors(req: Request): Record<string, string> {
  const origin = req.headers.get("origin");
  return origin ? { "access-control-allow-origin": origin, vary: "Origin" } : {};
}

/** preflight answers a CORS preflight request. */
export function preflight(req: Request): Response {
  return new Response(null, {
    status: 204,
    headers: {
      ...cors(req),
      "access-control-allow-methods": "GET, POST",
      "access-control-allow-headers": "authorization, content-type, connect-protocol-version, connect-timeout-ms",
      "access-control-max-age": "600",
    },
  });
}

/**
 * handleOAuth serves the OAuth endpoints under /api/oauth: the token
 * endpoint (RFC 6749 §3.2), which gives devices' sessions for
 * authorization codes, link codes' device codes and devices' signatures;
 * the device authorization endpoint (RFC 8628 §3.1); and revocation
 * (RFC 7009), which ends a session. Requests are forms, from any origin.
 */
export async function handleOAuth(req: Request, pathname: string, deps: OAuthDeps): Promise<Response | undefined> {
  const endpoint = { [tokenPath]: token, [devicePath]: device, [revokePath]: revoke }[pathname];
  if (!endpoint) return undefined;
  if (req.method === "OPTIONS") return preflight(req);
  const headers = { ...cors(req), "cache-control": "no-store" };
  const fail = (status: number, error: string, description: string) =>
    Response.json({ error, error_description: description }, { status, headers });
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
  return endpoint({ req, field, fail, ok: (body: object) => Response.json(body, { headers }) }, deps);
}

/** devicePath is the device authorization endpoint. */
export const devicePath = "/api/oauth/device";

/** revokePath is where devices end their sessions. */
export const revokePath = "/api/oauth/revoke";

interface Call {
  req: Request;
  field: (k: string) => string;
  fail: (status: number, error: string, description: string) => Response;
  ok: (body: object) => Response;
}

// sessionResponse starts a session of a device key and answers with its first access token.
async function sessionResponse(
  { ok, fail }: Call,
  { db, auth }: OAuthDeps,
  s: { accountId: string; clientId: string; deviceKey: Uint8Array },
  at: Date,
): Promise<Response> {
  const account = await db.query.user.findFirst({ where: eq(schema.user.id, s.accountId) });
  if (!account) return fail(400, "invalid_grant", "the account was deleted");
  const session = await createDeviceSession(db, s, at);
  return ok({
    access_token: await accessToken((await auth.$context).secret, session, at),
    token_type: "Bearer",
    expires_in: accessTokenLifetime,
    session_id: session.id,
    account: toJson(AccountSchema, accountOf(account)),
  });
}

// administers reports whether an account administers a server.
async function administers(db: Database, serverId: string, accountId: string): Promise<boolean> {
  const server = await db.query.servers.findFirst({ where: eq(schema.servers.id, serverId) });
  return server?.administratorId === accountId;
}

async function token(call: Call, deps: OAuthDeps): Promise<Response> {
  const { field, fail } = call;
  const { db, auth, now } = deps;
  const at = now();
  switch (field("grant_type")) {
    case "authorization_code": {
      const code = field("code");
      if (!code) return fail(400, "invalid_request", "no code");
      // A code is exchanged once, whatever comes of it.
      const [row] = await db
        .delete(schema.authorizationCodes)
        .where(eq(schema.authorizationCodes.codeHash, await sha256Hex(code)))
        .returning();
      const origin = call.req.headers.get("origin");
      if (
        !row ||
        row.expiresAt < at ||
        (row.serverId ?? appClient) !== field("client_id") ||
        row.redirectUri !== field("redirect_uri") ||
        row.codeChallenge !== (await s256(field("code_verifier")))
      ) {
        return fail(400, "invalid_grant", "the code is unknown, expired or not this client's");
      }
      if (origin && origin !== new URL(row.redirectUri).origin) {
        return fail(400, "invalid_grant", "the code was given to another origin");
      }
      if (row.serverId && !(await administers(db, row.serverId, row.accountId))) {
        return fail(400, "invalid_grant", "the account no longer administers the server");
      }
      return sessionResponse(call, deps, { accountId: row.accountId, clientId: field("client_id"), deviceKey: row.deviceKey }, at);
    }
    case deviceCodeGrant: {
      const hash = await sha256Hex(field("device_code"));
      const row = await db.query.linkCodes.findFirst({ where: eq(schema.linkCodes.deviceCodeHash, hash) });
      if (!row || row.clientId !== field("client_id")) return fail(400, "invalid_grant", "unknown device code");
      const done = () => db.delete(schema.linkCodes).where(eq(schema.linkCodes.deviceCodeHash, hash));
      if (row.expiresAt < at) {
        await done();
        return fail(400, "expired_token", "the code expired");
      }
      if (row.denied) {
        await done();
        return fail(400, "access_denied", "the code was refused");
      }
      if (!row.accountId) {
        const early = row.polledAt && at.getTime() - row.polledAt.getTime() < linkPollInterval * 1000;
        await db.update(schema.linkCodes).set({ polledAt: at }).where(eq(schema.linkCodes.deviceCodeHash, hash));
        return early ? fail(400, "slow_down", "poll less often") : fail(400, "authorization_pending", "not approved yet");
      }
      await done();
      if (!appClientID(row.clientId) && !(await administers(db, row.clientId, row.accountId))) {
        return fail(400, "access_denied", "the account does not administer the server");
      }
      return sessionResponse(call, deps, { accountId: row.accountId, clientId: row.clientId, deviceKey: row.deviceKey }, at);
    }
    case renewGrant: {
      const signature = fromBase64url(field("signature"));
      const session =
        signature &&
        (await renewDeviceSession(db, { sessionId: field("session_id"), timestamp: Number(field("timestamp")), signature }, at));
      if (!session) return fail(400, "invalid_grant", "the session ended, or the signature is not its device's");
      return call.ok({
        access_token: await accessToken((await auth.$context).secret, session, at),
        token_type: "Bearer",
        expires_in: accessTokenLifetime,
        session_id: session.id,
      });
    }
    default:
      return fail(400, "unsupported_grant_type", `unsupported grant type ${field("grant_type")}`);
  }
}

async function device(call: Call, { db, baseURL, now }: OAuthDeps): Promise<Response> {
  const { field, fail, ok } = call;
  const clientId = field("client_id");
  const deviceKey = fromBase64url(field("device_key"));
  if (!deviceKey || deviceKey.length !== 32) return fail(400, "invalid_request", "device_key is not an Ed25519 public key");
  if (!appClientID(clientId) && !(await db.query.servers.findFirst({ where: eq(schema.servers.id, clientId) }))) {
    return fail(400, "invalid_client", "unknown client");
  }
  const at = now();
  await db.delete(schema.linkCodes).where(lt(schema.linkCodes.expiresAt, at));
  const deviceCode = base64url(crypto.getRandomValues(new Uint8Array(32)));
  const expiresAt = new Date(at.getTime() + linkCodeLifetime);
  for (;;) {
    const userCode = newUserCode();
    const inserted = await db
      .insert(schema.linkCodes)
      .values({ deviceCodeHash: await sha256Hex(deviceCode), userCode: userCodeKey(userCode), clientId, deviceKey, expiresAt })
      .onConflictDoNothing()
      .returning();
    if (inserted.length === 0) continue;
    const link = `${baseURL.replace(/\/$/, "")}/link`;
    return ok({
      device_code: deviceCode,
      user_code: userCode,
      verification_uri: link,
      verification_uri_complete: `${link}?code=${userCode}`,
      expires_in: linkCodeLifetime / 1000,
      interval: linkPollInterval,
    });
  }
}

async function revoke({ field, ok }: Call, { db, auth, now }: OAuthDeps): Promise<Response> {
  await endDeviceSession(db, (await auth.$context).secret, field("token"), now());
  return ok({});
}

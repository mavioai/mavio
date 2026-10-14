// Accounts at mavio.dev (docs/mavio-dev.md §2), built on better-auth:
// e-mail and password, passkeys, TOTP, user names, code sign-in for
// televisions (RFC 8628), bearer sessions for native clients, and the JWT
// plugin whose keys sign sign-in assertions.
import { passkey } from "@better-auth/passkey";
import { betterAuth } from "better-auth";
import { drizzleAdapter } from "better-auth/adapters/drizzle";
import { APIError, createAuthMiddleware } from "better-auth/api";
import { bearer, deviceAuthorization, jwt, twoFactor, username } from "better-auth/plugins";
import { tanstackStartCookies } from "better-auth/tanstack-start";
import { eq } from "drizzle-orm";

import { AccountRevoked_Reason, accountChanged, accountOf, accountRevoked } from "./changes.ts";
import { type Database, schema } from "./db/index.ts";
import * as password from "./password.ts";

/** Config is the configuration of mavio.dev's accounts. */
export interface Config {
  /** The site's origin, such as https://mavio.dev. */
  baseURL: string;
  /** The secret better-auth signs cookies and seals keys with. */
  secret: string;
}

/** apiURL returns the API's base URL, which servers call and which issues assertions. */
export function apiURL(baseURL: string): string {
  return `${baseURL.replace(/\/$/, "")}/api`;
}

/** createAuth returns better-auth over db. */
export function createAuth(db: Database, config: Config) {
  const { hostname } = new URL(config.baseURL);
  return betterAuth({
    appName: "Mavio",
    baseURL: config.baseURL,
    basePath: "/api/auth",
    secret: config.secret,
    database: drizzleAdapter(db, { provider: "pg", schema }),
    emailAndPassword: {
      enabled: true,
      password: { hash: password.hash, verify: password.verify },
    },
    user: { deleteUser: { enabled: true } },
    plugins: [
      username({ minUsernameLength: 3, maxUsernameLength: 32 }),
      twoFactor({ issuer: "Mavio" }),
      passkey({ rpID: hostname, rpName: "Mavio", origin: config.baseURL }),
      jwt({
        jwks: { keyPairConfig: { alg: "EdDSA", crv: "Ed25519" } },
        jwt: { issuer: apiURL(config.baseURL) },
      }),
      deviceAuthorization({
        verificationUri: "/link",
        expiresIn: "10m",
        interval: "5s",
        // Mavio's own clients, by their reverse-DNS IDs.
        validateClient: (id) => /^dev\.mavio\.[a-z0-9.-]+$/.test(id),
      }),
      bearer(),
      tanstackStartCookies(),
    ],
    hooks: {
      // A password hashed with other parameters is hashed again once it signs in.
      after: createAuthMiddleware(async (ctx) => {
        const session = ctx.context.newSession;
        if (ctx.path !== "/sign-in/email" && ctx.path !== "/sign-in/username") return;
        if (!session || typeof ctx.body?.password !== "string") return;
        const account = await ctx.context.internalAdapter.findCredentialAccount(session.user.id);
        if (account?.password && password.outdated(account.password)) {
          await ctx.context.internalAdapter.updatePassword(session.user.id, await password.hash(ctx.body.password));
        }
      }),
    },
    databaseHooks: {
      user: {
        update: {
          after: async (u, ctx) => {
            if (ctx?.path === "/update-user") await accountChanged(db, accountOf(u));
          },
        },
        delete: {
          before: async (u) => {
            const [administered] = await db
              .select({ id: schema.servers.id })
              .from(schema.servers)
              .where(eq(schema.servers.administratorId, u.id))
              .limit(1);
            if (administered) {
              throw new APIError("BAD_REQUEST", { message: "Remove your servers or have them claimed by another account first." });
            }
            await accountRevoked(db, u.id, AccountRevoked_Reason.DELETED);
          },
        },
      },
    },
  });
}

/** Auth is mavio.dev's better-auth instance. */
export type Auth = ReturnType<typeof createAuth>;

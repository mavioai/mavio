// Accounts at mavio.dev (docs/mavio-dev.md §2), built on better-auth:
// e-mail and password, verified addresses and passwords reset by e-mail,
// passkeys, TOTP, user names, bearer sessions, and the JWT plugin whose
// keys sign sign-in assertions. Devices hold sessions
// of their own keys instead (device-sessions.ts).
import { passkey } from "@better-auth/passkey";
import { betterAuth } from "better-auth";
import { drizzleAdapter } from "better-auth/adapters/drizzle";
import { APIError, createAuthMiddleware } from "better-auth/api";
import { admin, bearer, jwt, twoFactor, username } from "better-auth/plugins";
import { tanstackStartCookies } from "better-auth/tanstack-start";
import { eq } from "drizzle-orm";

import { cancel, type Stripe } from "./billing.ts";
import { AccountRevoked_Reason, accountChanged, accountOf, accountRevoked } from "./changes.ts";
import { type Database, schema } from "./db/index.ts";
import { homes } from "./homes-plugin.ts";
import { type Mailer, resetMail, verificationMail } from "./mail.ts";
import { pass } from "./pass-plugin.ts";
import * as password from "./password.ts";
import { staff, staffRole, staffRoles } from "./staff-plugin.ts";

/** Config is the configuration of mavio.dev's accounts. */
export interface Config {
  /** The site's origin, such as https://mavio.dev. */
  baseURL: string;
  /** The secret better-auth signs cookies and seals keys with. */
  secret: string;
  /** mailer sends the e-mails verifying addresses and resetting passwords. */
  mailer: Mailer;
  /** stripe sells Mavio Pass; without it nothing can be bought. */
  stripe?: Stripe;
}

/** How long the links of e-mails stay valid, in seconds. */
export const mailLinkLifetime = 60 * 60;

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
      // An address is someone's once they show it is: friends and Homes
      // find accounts by it.
      requireEmailVerification: true,
      password: { hash: password.hash, verify: password.verify },
      resetPasswordTokenExpiresIn: mailLinkLifetime,
      sendResetPassword: async ({ user, url }, req) => {
        await config.mailer.send(resetMail(user, url, req?.headers.get("accept-language")));
      },
      // A new password ends every session, devices' included.
      revokeSessionsOnPasswordReset: true,
      onPasswordReset: async ({ user }) => {
        await db.delete(schema.deviceSessions).where(eq(schema.deviceSessions.accountId, user.id));
      },
    },
    emailVerification: {
      sendOnSignUp: true,
      // Signing in with an address not verified sends the link again.
      sendOnSignIn: true,
      autoSignInAfterVerification: true,
      expiresIn: mailLinkLifetime,
      sendVerificationEmail: async ({ user, url }, req) => {
        await config.mailer.send(verificationMail(user, url, req?.headers.get("accept-language")));
      },
    },
    user: {
      deleteUser: {
        enabled: true,
        // An account administering servers or a Home stays until they are given up.
        beforeDelete: async (u) => {
          const [administered] = await db
            .select({ id: schema.servers.id })
            .from(schema.servers)
            .where(eq(schema.servers.administratorId, u.id))
            .limit(1);
          if (administered) {
            throw new APIError("BAD_REQUEST", { message: "Remove your servers or have them claimed by another account first." });
          }
          const home = await db.query.homes.findFirst({ where: eq(schema.homes.administratorId, u.id) });
          if (home) throw new APIError("BAD_REQUEST", { message: "Dissolve your Home first." });
          if (config.stripe) await cancel(db, config.stripe, u.id);
        },
      },
      additionalFields: {
        // "full" or "managed" (docs/accounts.md §3.1).
        kind: { type: "string", required: false, defaultValue: "full", input: false },
        // A managed account's mavio.user.v1.RestrictionProfile.
        restriction: { type: "number", required: false, defaultValue: 0, input: false },
      },
    },
    plugins: [
      username({ minUsernameLength: 3, maxUsernameLength: 32 }),
      twoFactor({ issuer: "Mavio" }),
      passkey({ rpID: hostname, rpName: "Mavio", origin: config.baseURL }),
      jwt({
        jwks: { keyPairConfig: { alg: "EdDSA", crv: "Ed25519" } },
        jwt: { issuer: apiURL(config.baseURL) },
      }),
      bearer(),
      // Staff suspend accounts (staff-plugin.ts); no role is given by the API.
      admin({ roles: staffRoles, adminRoles: [staffRole], defaultRole: "user", bannedUserMessage: "This account is suspended." }),
      homes(db),
      pass(db, config.baseURL, config.stripe),
      staff(db),
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
            // A suspended account's sessions end, devices' and servers' too.
            if (ctx?.path === "/admin/ban-user") {
              await db.delete(schema.deviceSessions).where(eq(schema.deviceSessions.accountId, u.id));
              await accountRevoked(db, u.id, AccountRevoked_Reason.SUSPENDED);
            }
          },
        },
        delete: {
          before: async (u) => {
            await accountRevoked(db, u.id, AccountRevoked_Reason.DELETED);
          },
        },
      },
    },
  });
}

/** Auth is mavio.dev's better-auth instance. */
export type Auth = ReturnType<typeof createAuth>;

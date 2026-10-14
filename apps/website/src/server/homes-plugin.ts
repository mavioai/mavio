// Homes in better-auth (docs/accounts.md §3.1, §3.6): switching between the
// members of a Home, an endpoint of better-auth's since the device's
// session becomes the target's, and managed accounts kept away from
// credentials.
import { Code, ConnectError } from "@connectrpc/connect";
import type { BetterAuthPlugin, GenericEndpointContext } from "better-auth";
import { APIError, createAuthEndpoint, createAuthMiddleware, sessionMiddleware } from "better-auth/api";
import { setSessionCookie } from "better-auth/cookies";
import { and, eq } from "drizzle-orm";
import * as z from "zod";

import { type Database, schema } from "./db/index.ts";
import { checkPin } from "./home.ts";

const switchBody = z.object({
  accountId: z.string().min(1),
  pin: z.string().regex(/^[0-9]{4}$/).optional(),
  password: z.string().max(128).optional(),
});

// The endpoints managed accounts may not call: they have no credentials, and
// their Home's administrator names them.
const forbiddenToManaged = [
  "/update-user",
  "/change-email",
  "/change-password",
  "/set-password",
  "/delete-user",
  "/passkey/",
  "/two-factor/",
  "/link-social",
];

// sessionUser returns the user of a request's session, from its bearer
// token or session cookie; other hooks do not see the bearer plugin's
// cookie.
async function sessionUser(ctx: GenericEndpointContext) {
  const authorization = ctx.headers?.get("authorization");
  const token = authorization?.toLowerCase().startsWith("bearer ")
    ? authorization.slice(7).trim().split(".")[0]
    : await ctx.getSignedCookie(ctx.context.authCookies.sessionToken.name, ctx.context.secret);
  if (!token) return undefined;
  return (await ctx.context.internalAdapter.findSession(token))?.user as { kind?: string } | undefined;
}

/**
 * homes adds POST /home/switch, which signs the device's session out and
 * the target member in, after the target's PIN when it has one, and
 * refuses managed accounts the endpoints of credentials.
 */
export function homes(db: Database, now: () => Date = () => new Date()) {
  return {
    id: "mavio-homes",
    hooks: {
      before: [
        {
          matcher: (ctx) => forbiddenToManaged.some((p) => ctx.path?.startsWith(p)),
          handler: createAuthMiddleware(async (ctx) => {
            const user = await sessionUser(ctx);
            if (user?.kind === "managed") {
              throw new APIError("FORBIDDEN", { message: "Managed accounts are changed by their Home's administrator." });
            }
          }),
        },
      ],
    },
    endpoints: {
      switchAccount: createAuthEndpoint(
        "/home/switch",
        { method: "POST", body: switchBody, use: [sessionMiddleware] },
        async (ctx) => {
          const current = ctx.context.session;
          const [mine, theirs] = await Promise.all([
            db.query.homeMembers.findFirst({ where: eq(schema.homeMembers.accountId, current.user.id) }),
            db.query.homeMembers.findFirst({
              where: eq(schema.homeMembers.accountId, ctx.body.accountId),
              with: { account: true },
            }),
          ]);
          if (!mine || !theirs || mine.homeId !== theirs.homeId) {
            throw new APIError("NOT_FOUND", { message: "no such member of your Home" });
          }
          if (theirs.accountId === current.user.id) return ctx.json({ user: { id: current.user.id } });
          const credential = await db.query.account.findFirst({
            where: and(eq(schema.account.userId, theirs.accountId), eq(schema.account.providerId, "credential")),
          });
          try {
            await checkPin(db, theirs.account, { ...ctx.body, passwordHash: credential?.password ?? undefined }, now());
          } catch (e) {
            const err = ConnectError.from(e);
            throw new APIError(err.code === Code.ResourceExhausted ? "TOO_MANY_REQUESTS" : "FORBIDDEN", {
              message: err.rawMessage,
            });
          }
          const session = await ctx.context.internalAdapter.createSession(theirs.accountId);
          const user = await ctx.context.internalAdapter.findUserById(theirs.accountId);
          if (!user) throw new APIError("NOT_FOUND", { message: "no such member of your Home" });
          await setSessionCookie(ctx, { session, user });
          await ctx.context.internalAdapter.deleteSession(current.session.token);
          return ctx.json({ user: { id: user.id } });
        },
      ),
    },
  } satisfies BetterAuthPlugin;
}

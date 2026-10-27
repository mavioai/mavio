// mavio.dev's staff in better-auth (docs/mavio-dev.md §2): accounts whose
// role is "staff" find accounts, suspend and restore them through
// better-auth's admin plugin, whose roles allow them nothing else, and see
// and set an account's Mavio Pass.
import type { BetterAuthPlugin } from "better-auth";
import { createAccessControl } from "better-auth/plugins/access";
import { defaultStatements } from "better-auth/plugins/admin/access";
import { APIError, createAuthEndpoint, createAuthMiddleware, sessionMiddleware } from "better-auth/api";
import { count, eq } from "drizzle-orm";
import * as z from "zod";

import { subscriptionOf } from "./billing.ts";
import { type Database, schema } from "./db/index.ts";
import { effectivePass, homeOf } from "./home.ts";

/** The role of mavio.dev's staff. */
export const staffRole = "staff";

const ac = createAccessControl(defaultStatements);

/**
 * staffRoles are the admin plugin's roles: staff list, look up, suspend and
 * restore accounts, and no one impersonates, creates or deletes accounts,
 * or sets their passwords or roles through it.
 */
export const staffRoles = {
  [staffRole]: ac.newRole({ user: ["list", "get", "ban"], session: [] }),
  user: ac.newRole({ user: [], session: [] }),
};

// requireStaff refuses a session whose account is not staff.
function requireStaff(user: object) {
  if ((user as { role?: unknown }).role !== staffRole) throw new APIError("FORBIDDEN", { message: "Only mavio.dev's staff may do this." });
}

const passBody = z.object({ accountId: z.string().min(1), expiresAt: z.iso.datetime().nullable() });

/**
 * staff adds GET /staff/account, an account's Pass, Home, servers and
 * devices, and POST /staff/pass, which sets or ends its own Pass.
 */
export function staff(db: Database, now: () => Date = () => new Date()) {
  return {
    id: "mavio-staff",
    hooks: {
      before: [
        {
          // Staff are suspended by changing their role first.
          matcher: (ctx) => ctx.path === "/admin/ban-user",
          handler: createAuthMiddleware(async (ctx) => {
            const target = await db.query.user.findFirst({ where: eq(schema.user.id, String(ctx.body?.userId ?? "")) });
            if (target?.role === staffRole) throw new APIError("FORBIDDEN", { message: "Staff accounts cannot be suspended." });
          }),
        },
      ],
    },
    endpoints: {
      staffAccount: createAuthEndpoint(
        "/staff/account",
        { method: "GET", query: z.object({ accountId: z.string().min(1) }), use: [sessionMiddleware] },
        async (ctx) => {
          requireStaff(ctx.context.session.user);
          const id = ctx.query.accountId;
          const at = now();
          const [p, effective, subscription, home, servers, [devices]] = await Promise.all([
            db.query.passes.findFirst({ where: eq(schema.passes.accountId, id) }),
            effectivePass(db, id, at),
            subscriptionOf(db, id),
            homeOf(db, id),
            db.select({ id: schema.servers.id, name: schema.servers.name }).from(schema.servers).where(eq(schema.servers.administratorId, id)),
            db.select({ n: count() }).from(schema.deviceSessions).where(eq(schema.deviceSessions.accountId, id)),
          ]);
          return ctx.json({
            passExpiresAt: p?.expiresAt.toISOString() ?? null,
            effectivePass: effective,
            subscription: { ...subscription, periodEnd: subscription.periodEnd?.toISOString() },
            home: home ? { id: home.id, administratorId: home.administratorId } : null,
            servers,
            deviceSessions: devices?.n ?? 0,
          });
        },
      ),
      staffPass: createAuthEndpoint("/staff/pass", { method: "POST", body: passBody, use: [sessionMiddleware] }, async (ctx) => {
        requireStaff(ctx.context.session.user);
        const { accountId, expiresAt } = ctx.body;
        if (expiresAt === null) {
          await db.delete(schema.passes).where(eq(schema.passes.accountId, accountId));
        } else {
          const at = new Date(expiresAt);
          await db
            .insert(schema.passes)
            .values({ accountId, expiresAt: at })
            .onConflictDoUpdate({ target: schema.passes.accountId, set: { expiresAt: at } });
        }
        return ctx.json({ ok: true });
      }),
    },
  } satisfies BetterAuthPlugin;
}

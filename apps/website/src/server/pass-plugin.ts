// Buying Mavio Pass in better-auth (docs/mavio-dev.md §2): the account's
// subscription, Stripe Checkout to start one and the customer portal to
// change or cancel it.
import type { BetterAuthPlugin } from "better-auth";
import { APIError, createAuthEndpoint, sessionMiddleware } from "better-auth/api";
import * as z from "zod";

import { BillingError, checkout, portal, type Stripe, subscriptionOf } from "./billing.ts";
import type { Database } from "./db/index.ts";

const checkoutBody = z.object({ plan: z.enum(["month", "year"]) });

/**
 * pass adds GET /pass/subscription, POST /pass/checkout and POST
 * /pass/portal; without stripe nothing can be bought.
 */
export function pass(db: Database, baseURL: string, stripe?: Stripe) {
  const need = () => {
    if (!stripe) throw new APIError("NOT_IMPLEMENTED", { message: "Mavio Pass cannot be bought here." });
    return stripe;
  };
  const refuse = (e: unknown): never => {
    if (e instanceof BillingError) throw new APIError("CONFLICT", { message: e.message });
    throw e;
  };
  return {
    id: "mavio-pass",
    endpoints: {
      passSubscription: createAuthEndpoint("/pass/subscription", { method: "GET", use: [sessionMiddleware] }, async (ctx) => {
        const s = await subscriptionOf(db, ctx.context.session.user.id);
        return ctx.json({ available: !!stripe, ...s, periodEnd: s.periodEnd?.toISOString() });
      }),
      passCheckout: createAuthEndpoint("/pass/checkout", { method: "POST", body: checkoutBody, use: [sessionMiddleware] }, async (ctx) => {
        const user = ctx.context.session.user;
        const url = await checkout(db, need(), user, ctx.body.plan, baseURL).catch(refuse);
        return ctx.json({ url });
      }),
      passPortal: createAuthEndpoint("/pass/portal", { method: "POST", use: [sessionMiddleware] }, async (ctx) => {
        const url = await portal(db, need(), ctx.context.session.user.id, baseURL).catch(refuse);
        return ctx.json({ url });
      }),
    },
  } satisfies BetterAuthPlugin;
}

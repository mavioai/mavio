// Mavio Pass bought through Stripe Billing (docs/mavio-dev.md §2): Checkout
// starts a subscription, the customer portal changes or cancels it, and
// Stripe's webhooks extend the account's Pass by each paid period.
import { eq } from "drizzle-orm";

import { type Database, schema } from "./db/index.ts";

/** Plan is how often a Pass is paid for. */
export type Plan = "month" | "year";

/** StripeConfig is the Stripe account Mavio Pass is sold through. */
export interface StripeConfig {
  /** The secret API key. */
  secretKey: string;
  /** The signing secret of the webhook endpoint at /api/billing/stripe. */
  webhookSecret: string;
  /** The IDs of the recurring prices, by plan. */
  prices: Record<Plan, string>;
  fetch?: typeof fetch;
}

/** The API version requests are made in and webhooks must be sent in. */
export const stripeVersion = "2025-09-30.clover";

/** How long a Pass lasts past the period paid for, for a renewal's payment to be retried. */
export const passGrace = 24 * 60 * 60 * 1000;

/** The subscription states in which an account cannot start another. */
const live = new Set(["active", "trialing", "past_due", "incomplete", "unpaid"]);

/** Stripe calls Stripe's API. */
export class Stripe {
  constructor(private readonly config: StripeConfig) {}

  get prices() {
    return this.config.prices;
  }

  /** request calls the API with a form-encoded body. */
  async request<T>(method: "GET" | "POST" | "DELETE", path: string, params: Record<string, string> = {}): Promise<T> {
    const body = new URLSearchParams(params);
    const res = await (this.config.fetch ?? fetch)(`https://api.stripe.com/v1/${path}`, {
      method,
      headers: {
        authorization: `Bearer ${this.config.secretKey}`,
        "stripe-version": stripeVersion,
        ...(method === "POST" ? { "content-type": "application/x-www-form-urlencoded" } : {}),
      },
      body: method === "POST" ? body : undefined,
    });
    const json = (await res.json()) as T & { error?: { message?: string } };
    if (!res.ok) throw new Error(`Stripe: ${res.status} ${json.error?.message ?? ""}`.trim());
    return json;
  }

  /** verify returns the event of a webhook request, or undefined when its signature is not Stripe's. */
  async verify(payload: string, header: string | null, now: Date): Promise<StripeEvent | undefined> {
    if (!header) return undefined;
    const parts = header.split(",").map((p) => p.split("=", 2) as [string, string]);
    const t = parts.find(([k]) => k === "t")?.[1];
    const signatures = parts.filter(([k]) => k === "v1").map(([, v]) => v);
    if (!t || signatures.length === 0 || Math.abs(now.getTime() / 1000 - Number(t)) > 300) return undefined;
    const expected = await hmacHex(this.config.webhookSecret, `${t}.${payload}`);
    if (!signatures.some((s) => equal(s, expected))) return undefined;
    return JSON.parse(payload) as StripeEvent;
  }
}

/** StripeEvent is the part of a webhook event the Pass reads. */
export interface StripeEvent {
  id: string;
  type: string;
  data: { object: Record<string, unknown> };
}

async function hmacHex(secret: string, message: string): Promise<string> {
  const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  const mac = new Uint8Array(await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(message)));
  return Array.from(mac, (b) => b.toString(16).padStart(2, "0")).join("");
}

// equal compares two strings in time independent of where they differ.
function equal(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  let d = 0;
  for (let i = 0; i < a.length; i++) d |= a.charCodeAt(i) ^ b.charCodeAt(i);
  return d === 0;
}

/** Subscription is an account's subscription as the pages show it. */
export interface Subscription {
  /** Stripe's state of the subscription, empty without one. */
  status: string;
  plan?: Plan;
  /** When the period paid for ends. */
  periodEnd?: Date;
  /** Whether it ends with the period rather than renewing. */
  canceling: boolean;
}

/** subscriptionOf returns an account's subscription. */
export async function subscriptionOf(db: Database, accountId: string): Promise<Subscription> {
  const b = await db.query.billing.findFirst({ where: eq(schema.billing.accountId, accountId) });
  return {
    status: b?.status ?? "",
    plan: b?.plan === "month" || b?.plan === "year" ? b.plan : undefined,
    periodEnd: b?.periodEnd ?? undefined,
    canceling: b?.canceling ?? false,
  };
}

/** checkout returns the page of Stripe Checkout where an account subscribes to a plan. */
export async function checkout(db: Database, stripe: Stripe, user: { id: string; email: string }, plan: Plan, baseURL: string): Promise<string> {
  const b = await db.query.billing.findFirst({ where: eq(schema.billing.accountId, user.id) });
  if (b?.status && live.has(b.status)) throw new BillingError("already subscribed");
  const site = baseURL.replace(/\/$/, "");
  const session = await stripe.request<{ url: string }>("POST", "checkout/sessions", {
    mode: "subscription",
    "line_items[0][price]": stripe.prices[plan],
    "line_items[0][quantity]": "1",
    client_reference_id: user.id,
    "subscription_data[metadata][account_id]": user.id,
    ...(b ? { customer: b.customerId } : { customer_email: user.email }),
    allow_promotion_codes: "true",
    locale: "auto",
    success_url: `${site}/pass?checkout=done`,
    cancel_url: `${site}/pass`,
  });
  return session.url;
}

/** portal returns the page of Stripe's customer portal where an account manages its subscription. */
export async function portal(db: Database, stripe: Stripe, accountId: string, baseURL: string): Promise<string> {
  const b = await db.query.billing.findFirst({ where: eq(schema.billing.accountId, accountId) });
  if (!b) throw new BillingError("no subscription");
  const session = await stripe.request<{ url: string }>("POST", "billing_portal/sessions", {
    customer: b.customerId,
    return_url: `${baseURL.replace(/\/$/, "")}/pass`,
  });
  return session.url;
}

/** cancel ends an account's subscription at once, as its account is deleted. */
export async function cancel(db: Database, stripe: Stripe, accountId: string): Promise<void> {
  const b = await db.query.billing.findFirst({ where: eq(schema.billing.accountId, accountId) });
  if (b?.subscriptionId && b.status && live.has(b.status)) await stripe.request("DELETE", `subscriptions/${b.subscriptionId}`);
}

/** BillingError is a request the account's subscription does not allow. */
export class BillingError extends Error {}

type StripeObject = Record<string, unknown>;

const str = (v: unknown) => (typeof v === "string" ? v : undefined);
const obj = (v: unknown) => (v && typeof v === "object" ? (v as StripeObject) : undefined);
const list = (v: unknown) => (Array.isArray(obj(v)?.data) ? (obj(v)!.data as StripeObject[]) : []);
const seconds = (v: unknown) => (typeof v === "number" ? new Date(v * 1000) : undefined);

/** handleEvent applies a webhook event to the accounts' subscriptions and Passes. */
export async function handleEvent(db: Database, event: StripeEvent): Promise<void> {
  const o = event.data.object;
  switch (event.type) {
    case "checkout.session.completed": {
      const accountId = str(o.client_reference_id);
      const customerId = str(o.customer);
      if (accountId && customerId) await linkCustomer(db, accountId, customerId, { subscriptionId: str(o.subscription) });
      return;
    }
    case "customer.subscription.created":
    case "customer.subscription.updated":
    case "customer.subscription.deleted": {
      const customerId = str(o.customer);
      if (!customerId) return;
      const item = list(o.items)[0];
      const interval = str(obj(obj(item?.price)?.recurring)?.interval);
      const fields = {
        subscriptionId: str(o.id),
        status: str(o.status) ?? "",
        plan: interval === "month" || interval === "year" ? interval : null,
        periodEnd: seconds(item?.current_period_end) ?? null,
        canceling: o.cancel_at_period_end === true || typeof o.cancel_at === "number",
      };
      const accountId = str(obj(o.metadata)?.account_id) ?? (await accountOfCustomer(db, customerId));
      if (accountId) await linkCustomer(db, accountId, customerId, fields);
      return;
    }
    case "invoice.paid": {
      const customerId = str(o.customer);
      const accountId = customerId && (await accountOfCustomer(db, customerId));
      const ends = list(o.lines)
        .map((l) => seconds(obj(l.period)?.end))
        .filter((d): d is Date => !!d);
      if (!accountId || ends.length === 0) return;
      await extendPass(db, accountId, new Date(Math.max(...ends.map((d) => d.getTime())) + passGrace));
      return;
    }
  }
}

async function accountOfCustomer(db: Database, customerId: string): Promise<string | undefined> {
  return (await db.query.billing.findFirst({ where: eq(schema.billing.customerId, customerId) }))?.accountId;
}

async function linkCustomer(db: Database, accountId: string, customerId: string, fields: Partial<typeof schema.billing.$inferInsert>) {
  const set = Object.fromEntries(Object.entries({ customerId, ...fields }).filter(([, v]) => v !== undefined));
  await db
    .insert(schema.billing)
    .values({ accountId, customerId, ...set })
    .onConflictDoUpdate({ target: schema.billing.accountId, set });
}

/** extendPass makes an account's Pass last at least until a time. */
export async function extendPass(db: Database, accountId: string, until: Date): Promise<void> {
  const p = await db.query.passes.findFirst({ where: eq(schema.passes.accountId, accountId) });
  if (p && p.expiresAt >= until) return;
  await db
    .insert(schema.passes)
    .values({ accountId, expiresAt: until })
    .onConflictDoUpdate({ target: schema.passes.accountId, set: { expiresAt: until } });
}

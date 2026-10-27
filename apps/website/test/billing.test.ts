import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { AccountRevoked_Reason } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { eq } from "drizzle-orm";
import { describe, expect, test } from "vitest";

import { stripeWebhookPath } from "../src/server/api.ts";
import { passGrace, Stripe } from "../src/server/billing.ts";
import { schema } from "../src/server/db/index.ts";
import { staffRole } from "../src/server/staff-plugin.ts";
import { apiURL, baseURL, claimed, newSite } from "./harness.ts";

type Site = Awaited<ReturnType<typeof newSite>>;

const webhookSecret = "whsec_test";

/** fakeStripe returns a Stripe whose API records its requests and answers each with a page's URL. */
function fakeStripe() {
  const requests: { method: string; path: string; params: Record<string, string> }[] = [];
  const stripe = new Stripe({
    secretKey: "sk_test",
    webhookSecret,
    prices: { month: "price_month", year: "price_year" },
    fetch: async (input, init) => {
      const url = new URL(String(input));
      const params = Object.fromEntries(new URLSearchParams(String(init?.body ?? "")));
      requests.push({ method: init?.method ?? "GET", path: url.pathname.replace("/v1/", ""), params });
      return Response.json({ id: "obj_1", url: `https://stripe.test/${url.pathname}` });
    },
  });
  return { stripe, requests };
}

async function sign(payload: string, t: number, secret = webhookSecret) {
  const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  const mac = new Uint8Array(await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(`${t}.${payload}`)));
  return `t=${t},v1=${Array.from(mac, (b) => b.toString(16).padStart(2, "0")).join("")}`;
}

async function webhook(site: Site, event: unknown, secret?: string) {
  const payload = JSON.stringify(event);
  const signature = await sign(payload, Math.floor(Date.now() / 1000), secret);
  return site.fetch(new Request(`${baseURL}${stripeWebhookPath}`, { method: "POST", headers: { "stripe-signature": signature }, body: payload }));
}

function call(site: Site, method: "GET" | "POST", path: string, headers: Record<string, string>, body?: unknown) {
  return site.fetch(
    new Request(`${apiURL}/auth${path}`, {
      method,
      headers: { ...headers, origin: baseURL, ...(body ? { "content-type": "application/json" } : {}) },
      body: body ? JSON.stringify(body) : undefined,
    }),
  );
}

const day = 24 * 60 * 60;

describe("Mavio Pass", () => {
  test("cannot be bought without Stripe", async () => {
    const site = await newSite();
    const alice = await site.signUp("alice");
    expect(await (await call(site, "GET", "/pass/subscription", alice.headers)).json()).toMatchObject({ available: false, status: "" });
    expect((await call(site, "POST", "/pass/checkout", alice.headers, { plan: "month" })).status).toBe(501);
    expect((await webhook(site, { id: "evt", type: "invoice.paid", data: { object: {} } })).status).toBe(501);
  });

  test("is bought through Checkout and extended by each paid invoice", async () => {
    const { stripe, requests } = fakeStripe();
    const site = await newSite({ stripe });
    const alice = await site.signUp("alice");

    const res = await call(site, "POST", "/pass/checkout", alice.headers, { plan: "year" });
    expect(res.status).toBe(200);
    expect((await res.json()).url).toMatch(/^https:\/\/stripe\.test\//);
    expect(requests[0]).toMatchObject({
      method: "POST",
      path: "checkout/sessions",
      params: {
        mode: "subscription",
        "line_items[0][price]": "price_year",
        client_reference_id: alice.id,
        customer_email: "alice@example.com",
        success_url: `${baseURL}/pass?checkout=done`,
      },
    });

    // Webhooks not signed with the endpoint's secret change nothing.
    const completed = { id: "evt_1", type: "checkout.session.completed", data: { object: { client_reference_id: alice.id, customer: "cus_1", subscription: "sub_1" } } };
    expect((await webhook(site, completed, "whsec_other")).status).toBe(400);
    expect(await site.db.query.billing.findFirst()).toBeUndefined();

    expect((await webhook(site, completed)).status).toBe(200);
    const periodEnd = Math.floor(Date.now() / 1000) + 365 * day;
    await webhook(site, {
      id: "evt_2",
      type: "customer.subscription.updated",
      data: {
        object: {
          id: "sub_1",
          customer: "cus_1",
          status: "active",
          cancel_at_period_end: false,
          metadata: { account_id: alice.id },
          items: { data: [{ current_period_end: periodEnd, price: { recurring: { interval: "year" } } }] },
        },
      },
    });
    await webhook(site, { id: "evt_3", type: "invoice.paid", data: { object: { customer: "cus_1", lines: { data: [{ period: { end: periodEnd } }] } } } });

    const pass = await site.db.query.passes.findFirst({ where: eq(schema.passes.accountId, alice.id) });
    expect(pass?.expiresAt.getTime()).toBe(periodEnd * 1000 + passGrace);
    expect(await (await call(site, "GET", "/pass/subscription", alice.headers)).json()).toEqual({
      available: true,
      status: "active",
      plan: "year",
      periodEnd: new Date(periodEnd * 1000).toISOString(),
      canceling: false,
    });

    // One subscription at a time; the portal changes it, as the existing customer.
    expect((await call(site, "POST", "/pass/checkout", alice.headers, { plan: "month" })).status).toBe(409);
    expect((await call(site, "POST", "/pass/portal", alice.headers)).status).toBe(200);
    expect(requests.at(-1)).toMatchObject({ path: "billing_portal/sessions", params: { customer: "cus_1", return_url: `${baseURL}/pass` } });

    // An older invoice never shortens it.
    await webhook(site, { id: "evt_4", type: "invoice.paid", data: { object: { customer: "cus_1", lines: { data: [{ period: { end: periodEnd - day } }] } } } });
    expect((await site.db.query.passes.findFirst({ where: eq(schema.passes.accountId, alice.id) }))?.expiresAt.getTime()).toBe(periodEnd * 1000 + passGrace);

    // Deleting the account ends its subscription.
    await site.auth.api.deleteUser({ headers: new Headers(alice.headers), body: { password: "alice password" } });
    expect(requests.at(-1)).toMatchObject({ method: "DELETE", path: "subscriptions/sub_1" });
  });
});

describe("staff", () => {
  async function staffOf(site: Site) {
    const staff = await site.signUp("staff");
    await site.db.update(schema.user).set({ role: staffRole }).where(eq(schema.user.id, staff.id));
    return staff;
  }

  test("suspend accounts, whose servers end their sessions", async () => {
    const { site, alice, api } = await claimed();
    const staff = await staffOf(site);
    const bob = await site.signUp("bob");

    // Others cannot.
    expect((await call(site, "POST", "/admin/ban-user", bob.headers, { userId: alice.id })).status).toBe(403);
    expect((await call(site, "GET", `/staff/account?accountId=${alice.id}`, bob.headers)).status).toBe(403);

    const found = await call(site, "GET", "/admin/list-users?searchValue=ali&searchField=name", staff.headers);
    expect((await found.json()).users.map((u: { id: string }) => u.id)).toEqual([alice.id]);
    const details = await (await call(site, "GET", `/staff/account?accountId=${alice.id}`, staff.headers)).json();
    expect(details).toMatchObject({ passExpiresAt: null, servers: [{ name: "Home" }], deviceSessions: 0 });

    // Staff do nothing else of the admin plugin's, nor suspend each other.
    expect((await call(site, "POST", "/admin/set-role", staff.headers, { userId: bob.id, role: staffRole })).status).toBe(403);
    expect((await call(site, "POST", "/admin/impersonate-user", staff.headers, { userId: bob.id })).status).toBe(403);
    expect((await call(site, "POST", "/admin/remove-user", staff.headers, { userId: bob.id })).status).toBe(403);
    await site.db.update(schema.user).set({ role: staffRole }).where(eq(schema.user.id, bob.id));
    expect((await call(site, "POST", "/admin/ban-user", staff.headers, { userId: bob.id })).status).toBe(403);

    const { cursor } = await api.listChanges({});
    expect((await call(site, "POST", "/admin/ban-user", staff.headers, { userId: alice.id, banReason: "abuse" })).status).toBe(200);
    const changes = (await api.listChanges({ after: cursor })).changes;
    expect(changes.map((c) => [c.kind.case, (c.kind.value as { reason: number }).reason])).toEqual([["accountRevoked", AccountRevoked_Reason.SUSPENDED]]);
    expect(await (await call(site, "GET", "/get-session", alice.headers)).json()).toBeNull();
    const signIn = await call(site, "POST", "/sign-in/username", {}, { username: "alice", password: "alice password" });
    expect([signIn.status, (await signIn.json()).code]).toEqual([403, "BANNED_USER"]);

    expect((await call(site, "POST", "/admin/unban-user", staff.headers, { userId: alice.id })).status).toBe(200);
    expect((await call(site, "POST", "/sign-in/username", {}, { username: "alice", password: "alice password" })).status).toBe(200);
  });

  test("set an account's Pass", async () => {
    const site = await newSite();
    const staff = await staffOf(site);
    const alice = await site.signUp("alice");
    const until = new Date(Date.now() + 30 * day * 1000).toISOString();
    expect((await call(site, "POST", "/staff/pass", alice.headers, { accountId: alice.id, expiresAt: until })).status).toBe(403);
    expect((await call(site, "POST", "/staff/pass", staff.headers, { accountId: alice.id, expiresAt: until })).status).toBe(200);
    expect((await (await call(site, "GET", `/staff/account?accountId=${alice.id}`, staff.headers)).json()).passExpiresAt).toBe(until);
    expect((await site.client(AccountService, alice.headers).getAccount({})).effectivePass).toBe(true);
    await call(site, "POST", "/staff/pass", staff.headers, { accountId: alice.id, expiresAt: null });
    expect(await site.db.query.passes.findFirst()).toBeUndefined();
  });
});

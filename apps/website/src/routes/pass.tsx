import { timestampDate } from "@bufbuild/protobuf/wkt";
import { AccountKind } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { useMutation, useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";

import { accounts } from "../lib/api.ts";
import { authClient } from "../lib/auth-client.ts";
import { describe, useLocale, useT } from "../lib/locale.tsx";
import { useSignedIn } from "../lib/session.ts";
import { Block, Button, ErrorText, Group, Icon, type IconName, Loading, Muted, Page, Row } from "../ui.tsx";

export const Route = createFileRoute("/pass")({
  validateSearch: (s: Record<string, unknown>): { checkout?: "done" } => (s.checkout === "done" ? { checkout: "done" } : {}),
  component: PassPage,
});

type Plan = "month" | "year";

/** The prices of the plans, as Stripe charges them (docs/mavio-dev.md §2). */
const prices: Record<Plan, number> = { month: 4, year: 35 };

/** The subscription as GET /api/auth/pass/subscription returns it. */
interface Subscription {
  available: boolean;
  status: string;
  plan?: Plan;
  periodEnd?: string;
  canceling: boolean;
}

/** The subscription states in which the account has one to manage rather than plans to choose. */
const live = new Set(["active", "trialing", "past_due", "incomplete", "unpaid"]);

/** call calls one of the Pass's endpoints of better-auth, throwing its error. */
async function call<T>(path: string, body?: object): Promise<T> {
  const res = await authClient.$fetch<T>(path, body ? { method: "POST", body } : { method: "GET" });
  if (res.error) throw res.error;
  return res.data as T;
}

const benefits: [IconName, "relay" | "home" | "support"][] = [
  ["network", "relay"],
  ["users", "home"],
  ["favorite", "support"],
];

/** Mavio Pass: what it gives, the account's own and the plans to buy or the subscription to manage. */
function PassPage() {
  const t = useT();
  const signedIn = useSignedIn();
  const { checkout } = Route.useSearch();
  return (
    <Page title={t("site.nav.pass")}>
      <Muted>{t("site.pass.intro")}</Muted>
      {checkout === "done" && (
        <p role="status" className="flex items-start gap-2 type-callout text-success">
          <Icon name="check" size={18} className="mt-px shrink-0" />
          {t("site.pass.thanks")}
        </p>
      )}
      <Group>
        {benefits.map(([icon, key]) => (
          <Row
            key={key}
            leading={
              <span className="flex size-10 shrink-0 items-center justify-center self-start rounded-[10px] bg-fill text-accent">
                <Icon name={icon} size={22} />
              </span>
            }
            label={<span className="font-semibold">{t(`site.pass.${key}.title`)}</span>}
            detail={t(`site.pass.${key}.body`)}
          />
        ))}
      </Group>
      {signedIn ? <Yours /> : <Plans />}
    </Page>
  );
}

/** The account's Pass and subscription, and the plans when it has none. */
function Yours() {
  const t = useT();
  const locale = useLocale();
  const account = useQuery({ queryKey: ["account"], queryFn: () => accounts.getAccount({}) });
  const subscription = useQuery({ queryKey: ["pass"], queryFn: () => call<Subscription>("/pass/subscription") });
  const portal = useMutation({
    mutationFn: () => call<{ url: string }>("/pass/portal", {}),
    onSuccess: ({ url }) => window.location.assign(url),
  });
  if (!account.data || !subscription.data) return account.error || subscription.error ? <ErrorText error={describe(t, account.error ?? subscription.error)} /> : <Loading />;
  const a = account.data;
  const s = subscription.data;
  const date = (d: Date) => d.toLocaleDateString(locale, { dateStyle: "long" });
  const expires = a.passExpireTime && timestampDate(a.passExpireTime);
  const managed = a.account?.kind === AccountKind.MANAGED;
  const subscribed = live.has(s.status);
  const state =
    s.status === "past_due" || s.status === "unpaid"
      ? t("site.pass.pastDue")
      : s.periodEnd && s.canceling
        ? t("site.pass.ends", { date: date(new Date(s.periodEnd)) })
        : s.periodEnd && s.plan
          ? t("site.pass.renews", { plan: t(s.plan === "year" ? "site.pass.year" : "site.pass.month").toLowerCase(), date: date(new Date(s.periodEnd)) })
          : undefined;
  return (
    <>
      <Group title={t("site.pass.yours")} footer={<ErrorText error={describe(t, portal.error)} />}>
        <Row
          leading={<Icon name="key" size={24} className={expires || a.effectivePass ? "text-accent" : "text-text-tertiary"} />}
          label={expires ? t("site.pass.until", { date: date(expires) }) : a.effectivePass ? t("site.pass.shared") : t("site.pass.none")}
          detail={subscribed ? state : undefined}
        >
          {subscribed && (
            <Button variant="secondary" disabled={portal.isPending} onClick={() => portal.mutate()}>
              {t("site.pass.manage")}
            </Button>
          )}
        </Row>
        {managed && (
          <Block>
            <Muted>{t("site.pass.managed")}</Muted>
          </Block>
        )}
      </Group>
      {!managed && !subscribed && <Plans available={s.available} />}
    </>
  );
}

/**
 * The plans with their prices, each subscribing through Stripe Checkout;
 * visitors sign in first, and nothing is bought where Stripe is not set up.
 */
function Plans({ available }: { available?: boolean }) {
  const t = useT();
  const locale = useLocale();
  const signedIn = available !== undefined;
  const subscribe = useMutation({
    mutationFn: (plan: Plan) => call<{ url: string }>("/pass/checkout", { plan }),
    onSuccess: ({ url }) => window.location.assign(url),
  });
  const price = (n: number) => new Intl.NumberFormat(locale, { style: "currency", currency: "USD", maximumFractionDigits: 0 }).format(n);
  const saving = Math.round((1 - prices.year / (prices.month * 12)) * 100);
  return (
    <section className="flex flex-col gap-2">
      <h2 className="px-1 type-headline">{t("site.pass.plans")}</h2>
      <div className="grid gap-3 sm:grid-cols-2">
        {(["month", "year"] as const).map((plan) => (
          <div key={plan} className="flex flex-col gap-4 rounded-[14px] bg-surface p-5">
            <div className="flex items-center justify-between gap-2">
              <span className="type-headline">{t(plan === "year" ? "site.pass.year" : "site.pass.month")}</span>
              {plan === "year" && <span className="rounded-full bg-fill px-2.5 py-0.5 type-footnote font-semibold text-accent-text">{t("site.pass.saving", { percent: saving })}</span>}
            </div>
            <span className="type-title2">{t(plan === "year" ? "site.pass.perYear" : "site.pass.perMonth", { price: price(prices[plan]) })}</span>
            {signedIn ? (
              <Button disabled={!available || subscribe.isPending} onClick={() => subscribe.mutate(plan)}>
                {t("site.pass.subscribe")}
              </Button>
            ) : (
              <Link
                to="/sign-in"
                search={{ next: "/pass" }}
                className="inline-flex min-h-9 items-center justify-center rounded-md bg-button px-4 type-headline text-on-button hover:opacity-85"
              >
                {t("site.pass.signInToSubscribe")}
              </Link>
            )}
          </div>
        ))}
      </div>
      <div className="flex flex-col gap-1 px-1">
        {signedIn && !available && <p className="type-footnote text-warning">{t("site.pass.unavailable")}</p>}
        <ErrorText error={describe(t, subscribe.error)} />
        <p className="type-footnote text-text-secondary">{t("site.pass.payments")}</p>
      </div>
    </section>
  );
}

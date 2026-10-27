import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";

import { authClient } from "../lib/auth-client.ts";
import { describe, useLocale, useT } from "../lib/locale.tsx";
import { useAccount } from "../lib/session.ts";
import { Actions, Avatar, Block, Button, Empty, ErrorText, Field, Group, Loading, Muted, Page, Row } from "../ui.tsx";

export const Route = createFileRoute("/staff")({ component: StaffPage });

/** An account as the admin plugin lists it. */
interface Account {
  id: string;
  name: string;
  email: string;
  username?: string | null;
  role?: string | null;
  kind?: string | null;
  banned?: boolean | null;
  banReason?: string | null;
  createdAt: string | Date;
}

/** An account's details as GET /api/auth/staff/account returns them. */
interface Details {
  passExpiresAt: string | null;
  effectivePass: boolean;
  subscription: { status: string; plan?: string; periodEnd?: string; canceling: boolean };
  home: { id: string; administratorId: string } | null;
  servers: { id: string; name: string }[];
  deviceSessions: number;
}

/** call calls one of better-auth's endpoints, throwing its error. */
async function call<T>(path: string, options: { method?: "GET" | "POST"; query?: Record<string, string>; body?: object } = {}): Promise<T> {
  const res = await authClient.$fetch<T>(path, { method: options.method ?? (options.body ? "POST" : "GET"), query: options.query, body: options.body });
  if (res.error) throw res.error;
  return res.data as T;
}

/** The staff's tools: finding accounts, suspending and restoring them, and setting their Pass. */
function StaffPage() {
  const t = useT();
  const me = useAccount() as (ReturnType<typeof useAccount> & { role?: string | null }) | undefined;
  const [search, setSearch] = useState("");
  const [open, setOpen] = useState<string>();
  const found = useQuery({
    queryKey: ["staff", "accounts", search],
    enabled: me?.role === "staff" && search.length > 0,
    queryFn: async () => {
      const res = await authClient.admin.listUsers({
        query: { searchValue: search, searchField: search.includes("@") ? "email" : "name", searchOperator: "contains", limit: 20 },
      });
      if (res.error) throw res.error;
      return res.data.users as unknown as Account[];
    },
  });
  if (!me) return <Loading />;
  if (me.role !== "staff") {
    return (
      <Page title={t("site.nav.staff")}>
        <Empty icon="lock" title={t("site.staff.forbidden")} />
      </Page>
    );
  }
  return (
    <Page title={t("site.nav.staff")}>
      <form
        className="flex items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          setSearch(String(new FormData(e.currentTarget).get("q")).trim());
          setOpen(undefined);
        }}
      >
        <Field label={t("site.staff.search")} name="q" type="search" placeholder={t("site.staff.searchHint")} autoFocus />
        <Button type="submit">{t("site.staff.search")}</Button>
      </form>
      <ErrorText error={describe(t, found.error)} />
      {found.isFetching && <Loading />}
      {found.data && found.data.length === 0 && <Empty icon="search" title={t("site.staff.none")} />}
      {found.data && found.data.length > 0 && (
        <Group>
          {found.data.map((a) => (
            <AccountRow key={a.id} account={a} open={open === a.id} onToggle={() => setOpen(open === a.id ? undefined : a.id)} />
          ))}
        </Group>
      )}
    </Page>
  );
}

/** A found account: its name and badges, opening its details and actions. */
function AccountRow({ account: a, open, onToggle }: { account: Account; open: boolean; onToggle: () => void }) {
  const t = useT();
  const managed = a.kind === "managed";
  const badge = "rounded-full bg-fill px-2 py-0.5 type-caption font-semibold";
  return (
    <div>
      <button type="button" onClick={onToggle} aria-expanded={open} className="w-full text-start transition-colors hover:bg-fill">
        <Row
          leading={<Avatar name={a.name || a.username || "?"} size={36} />}
          label={
            <span className="flex flex-wrap items-center gap-2">
              <span className="font-semibold">{a.name}</span>
              {a.banned && <span className={`${badge} text-danger`}>{t("site.staff.suspended")}</span>}
              {a.role === "staff" && <span className={`${badge} text-accent-text`}>{t("site.staff.staff")}</span>}
              {managed && <span className={`${badge} text-text-secondary`}>{t("site.staff.managed")}</span>}
            </span>
          }
          detail={[a.username && `@${a.username}`, managed ? undefined : a.email].filter(Boolean).join(" · ")}
        />
      </button>
      {open && <AccountDetails account={a} />}
    </div>
  );
}

/** An account's Pass, servers, Home and devices, with suspending, restoring and setting its Pass. */
function AccountDetails({ account: a }: { account: Account }) {
  const t = useT();
  const locale = useLocale();
  const qc = useQueryClient();
  const details = useQuery({ queryKey: ["staff", "account", a.id], queryFn: () => call<Details>("/staff/account", { query: { accountId: a.id } }) });
  const done = () => qc.invalidateQueries({ queryKey: ["staff"] });
  const suspend = useMutation({
    mutationFn: async (reason: string) => {
      const res = await authClient.admin.banUser({ userId: a.id, banReason: reason || undefined });
      if (res.error) throw res.error;
    },
    onSuccess: done,
  });
  const restore = useMutation({
    mutationFn: async () => {
      const res = await authClient.admin.unbanUser({ userId: a.id });
      if (res.error) throw res.error;
    },
    onSuccess: done,
  });
  const setPass = useMutation({
    mutationFn: (expiresAt: string | null) => call("/staff/pass", { body: { accountId: a.id, expiresAt } }),
    onSuccess: done,
  });
  const [confirming, setConfirming] = useState(false);
  const date = (d: string | Date) => new Date(d).toLocaleDateString(locale, { dateStyle: "long" });
  const d = details.data;
  return (
    <Block>
      {!d ? (
        details.error ? <ErrorText error={describe(t, details.error)} /> : <Loading />
      ) : (
        <ul className="flex flex-col gap-1 type-callout text-text-secondary">
          <li>{t("site.staff.joined", { date: date(a.createdAt) })}</li>
          <li>{d.passExpiresAt ? t("site.pass.until", { date: date(d.passExpiresAt) }) : d.effectivePass ? t("site.pass.shared") : t("site.staff.noPass")}</li>
          {d.subscription.status && <li>{t("site.staff.subscription", { status: d.subscription.status })}</li>}
          <li>{t("site.staff.servers", { count: d.servers.length })}</li>
          {d.home && <li>{t(d.home.administratorId === a.id ? "site.staff.homeAdmin" : "site.staff.homeMember")}</li>}
          <li>{t("site.staff.devices", { count: d.deviceSessions })}</li>
          {a.banned && a.banReason && <li className="text-danger">{t("site.staff.reason", { reason: a.banReason })}</li>}
        </ul>
      )}
      <form
        className="flex flex-wrap items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          const v = String(new FormData(e.currentTarget).get("until"));
          if (v) setPass.mutate(new Date(`${v}T23:59:59Z`).toISOString());
        }}
      >
        <Field label={t("site.staff.passUntil")} name="until" type="date" defaultValue={d?.passExpiresAt?.slice(0, 10)} />
        <Button type="submit" variant="secondary" disabled={setPass.isPending}>
          {t("site.action.set")}
        </Button>
        {d?.passExpiresAt && (
          <Button variant="danger" disabled={setPass.isPending} onClick={() => setPass.mutate(null)}>
            {t("site.staff.endPass")}
          </Button>
        )}
      </form>
      {a.role !== "staff" &&
        (a.banned ? (
          <Actions>
            <Button variant="secondary" disabled={restore.isPending} onClick={() => restore.mutate()}>
              {t("site.staff.restore")}
            </Button>
          </Actions>
        ) : confirming ? (
          <form
            className="flex flex-col gap-3 rounded-[10px] border border-border p-3"
            onSubmit={(e) => {
              e.preventDefault();
              suspend.mutate(String(new FormData(e.currentTarget).get("reason")).trim());
            }}
          >
            <h3 className="type-headline">{t("site.staff.suspendTitle", { name: a.name })}</h3>
            <Muted>{t("site.staff.suspendBody")}</Muted>
            <Field label={t("site.staff.suspendReason")} name="reason" maxLength={200} />
            <Actions>
              <Button type="submit" variant="danger" disabled={suspend.isPending}>
                {t("site.staff.suspend")}
              </Button>
              <Button variant="secondary" onClick={() => setConfirming(false)}>
                {t("site.action.cancel")}
              </Button>
            </Actions>
          </form>
        ) : (
          <Actions>
            <Button variant="danger" onClick={() => setConfirming(true)}>
              {t("site.staff.suspend")}
            </Button>
          </Actions>
        ))}
      <ErrorText error={describe(t, suspend.error ?? restore.error ?? setPass.error)} />
    </Block>
  );
}

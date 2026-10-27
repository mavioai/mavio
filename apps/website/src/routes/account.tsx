import { timestampDate } from "@bufbuild/protobuf/wkt";
import { AccountKind } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { accounts, homes } from "../lib/api.ts";
import { authClient } from "../lib/auth-client.ts";
import { describe, useLocale, useT } from "../lib/locale.tsx";
import { useAccount } from "../lib/session.ts";
import { Actions, Avatar, Block, Button, ErrorText, Field, Group, Icon, Loading, Muted, Page, Row, useConfirm } from "../ui.tsx";

export const Route = createFileRoute("/account")({ component: AccountPage });

type Me = NonNullable<ReturnType<typeof useAccount>> & { emailVerified?: boolean; twoFactorEnabled?: boolean | null };

function AccountPage() {
  const t = useT();
  const me = useAccount() as Me | undefined;
  const account = useQuery({ queryKey: ["account"], queryFn: () => accounts.getAccount({}), enabled: !!me });
  if (!me || !account.data) return <Loading />;
  const a = account.data;
  const pass = <Pass expires={a.passExpireTime && timestampDate(a.passExpireTime)} effective={a.effectivePass} />;
  if (a.account?.kind === AccountKind.MANAGED) {
    return (
      <Page title={t("site.nav.account")}>
        <Group>
          <Row leading={<Avatar name={a.account.displayName || "?"} size={44} />} label={<span className="type-headline">{a.account.displayName}</span>} detail={`@${a.account.userName}`} />
          <Block>
            <Muted>{t("site.account.managed")}</Muted>
          </Block>
        </Group>
        {pass}
      </Page>
    );
  }
  return (
    <Page title={t("site.nav.account")}>
      <Profile name={a.account?.displayName ?? ""} userName={a.account?.userName ?? ""} email={a.email} verified={!!me.emailVerified} />
      {pass}
      <Security id={me.id} hasPin={a.hasPin} twoFactor={!!me.twoFactorEnabled} />
      <Passkeys />
      <Danger />
    </Page>
  );
}

function Profile({ name, userName, email, verified }: { name: string; userName: string; email: string; verified: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const [error, setError] = useState<string>();
  const [saved, setSaved] = useState(false);
  return (
    <Group title={t("site.account.profile")}>
      <Row leading={<Avatar name={name || userName || "?"} size={44} />} label={<span className="type-headline">{name}</span>} detail={`@${userName}`} />
      <Row label={t("site.field.email")} detail={email}>
        <span className={`inline-flex items-center gap-1 type-footnote font-semibold ${verified ? "text-success" : "text-warning"}`}>
          <Icon name={verified ? "check" : "info"} size={16} />
          {t(verified ? "site.account.verified" : "site.account.unverified")}
        </span>
      </Row>
      <Block>
        <form
          className="flex flex-col gap-3 sm:flex-row sm:items-end"
          onSubmit={async (e) => {
            e.preventDefault();
            const res = await authClient.updateUser({ name: String(new FormData(e.currentTarget).get("name")).trim() });
            setError(describe(t, res.error));
            setSaved(!res.error);
            await qc.invalidateQueries({ queryKey: ["account"] });
          }}
        >
          <Field label={t("site.field.displayName")} name="name" defaultValue={name} required maxLength={64} onChange={() => setSaved(false)} />
          <Button type="submit" className="min-h-11" disabled={saved}>
            {saved ? <Icon name="check" size={18} /> : null}
            {t("site.action.save")}
          </Button>
        </form>
        <ErrorText error={error} />
      </Block>
    </Group>
  );
}

function Pass({ expires, effective }: { expires?: Date; effective: boolean }) {
  const t = useT();
  const locale = useLocale();
  return (
    <Group title={t("site.pass.title")}>
      <Row
        leading={<Icon name="key" size={24} className={expires || effective ? "text-accent" : "text-text-tertiary"} />}
        label={expires ? t("site.pass.until", { date: expires.toLocaleDateString(locale, { dateStyle: "long" }) }) : effective ? t("site.pass.shared") : t("site.pass.none")}
      >
        <Link to="/pass" className="inline-flex min-h-9 items-center rounded-md bg-fill px-4 type-headline text-on-background hover:bg-fill-selected">
          {t("site.nav.pass")}
        </Link>
      </Row>
    </Group>
  );
}

/** A row opening a form under it. */
function Opening({ label, detail, action, open, onToggle, children }: { label: ReactNode; detail?: ReactNode; action: string; open: boolean; onToggle: () => void; children: ReactNode }) {
  const t = useT();
  return (
    <div>
      <Row label={label} detail={detail}>
        <Button variant="secondary" onClick={onToggle}>
          {open ? t("site.action.cancel") : action}
        </Button>
      </Row>
      {open && <div className="flex flex-col gap-3 px-4 pb-4">{children}</div>}
    </div>
  );
}

function Security({ id, hasPin, twoFactor }: { id: string; hasPin: boolean; twoFactor: boolean }) {
  const t = useT();
  const [open, setOpen] = useState<"password" | "pin" | "twoFactor">();
  const toggle = (what: typeof open) => setOpen(open === what ? undefined : what);
  return (
    <Group title={t("site.account.security")}>
      <Opening label={t("site.field.password")} action={t("site.account.changePassword")} open={open === "password"} onToggle={() => toggle("password")}>
        <ChangePassword />
      </Opening>
      <Opening label={t("site.field.pin")} detail={t(hasPin ? "site.account.pinSet" : "site.account.pinNone")} action={t("site.action.set")} open={open === "pin"} onToggle={() => toggle("pin")}>
        <Pin id={id} has={hasPin} onDone={() => setOpen(undefined)} />
      </Opening>
      <Opening
        label={
          <span className="flex items-center gap-2">
            {t("site.account.twoFactor")}
            <span className={`type-footnote font-semibold ${twoFactor ? "text-success" : "text-text-tertiary"}`}>{t(twoFactor ? "site.account.on" : "site.account.off")}</span>
          </span>
        }
        detail={t(twoFactor ? "site.account.twoFactorOn" : "site.account.twoFactorOff")}
        action={t(twoFactor ? "site.account.turnOff" : "site.account.setUp")}
        open={open === "twoFactor"}
        onToggle={() => toggle("twoFactor")}
      >
        <TwoFactor enabled={twoFactor} />
      </Opening>
    </Group>
  );
}

function ChangePassword() {
  const t = useT();
  const [error, setError] = useState<string>();
  const [done, setDone] = useState(false);
  return (
    <form
      className="flex flex-col gap-3"
      onSubmit={async (e) => {
        e.preventDefault();
        const form = e.currentTarget;
        const f = new FormData(form);
        const res = await authClient.changePassword({ currentPassword: String(f.get("current")), newPassword: String(f.get("password")), revokeOtherSessions: true });
        setError(describe(t, res.error));
        setDone(!res.error);
        if (!res.error) form.reset();
      }}
    >
      <div className="flex flex-col gap-3 sm:flex-row">
        <Field label={t("site.field.currentPassword")} name="current" type="password" autoComplete="current-password" required />
        <Field label={t("site.field.newPassword")} name="password" type="password" autoComplete="new-password" placeholder={t("site.field.passwordHint")} minLength={8} required />
      </div>
      <ErrorText error={error} />
      {done && <p className="type-callout text-success">{t("site.account.passwordChanged")}</p>}
      <Actions>
        <Button type="submit">{t("site.account.changePassword")}</Button>
      </Actions>
    </form>
  );
}

function Pin({ id, has, onDone }: { id: string; has: boolean; onDone: () => void }) {
  const t = useT();
  const qc = useQueryClient();
  const set = useMutation({
    mutationFn: (pin: string) => homes.setPin({ accountId: id, pin }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["account"] });
      onDone();
    },
  });
  return (
    <>
      <form
        className="flex flex-col gap-3 sm:flex-row sm:items-end"
        onSubmit={(e) => {
          e.preventDefault();
          set.mutate(String(new FormData(e.currentTarget).get("pin")));
        }}
      >
        <Field label={t(has ? "site.field.newPin" : "site.field.pin")} name="pin" type="password" inputMode="numeric" pattern="[0-9]{4}" maxLength={4} autoFocus required />
        <Button type="submit" className="min-h-11">
          {t("site.action.set")}
        </Button>
        {has && (
          <Button variant="danger" className="min-h-11" onClick={() => set.mutate("")}>
            {t("site.action.remove")}
          </Button>
        )}
      </form>
      <ErrorText error={describe(t, set.error)} />
    </>
  );
}

function TwoFactor({ enabled }: { enabled: boolean }) {
  const t = useT();
  const [uri, setUri] = useState<string>();
  const [error, setError] = useState<string>();
  if (uri) {
    const secret = new URL(uri).searchParams.get("secret") ?? uri;
    return (
      <form
        className="flex flex-col gap-3"
        onSubmit={async (e) => {
          e.preventDefault();
          const res = await authClient.twoFactor.verifyTotp({ code: String(new FormData(e.currentTarget).get("code")).trim() });
          if (res.error) setError(describe(t, res.error));
          else window.location.reload();
        }}
      >
        <Muted>{t("site.account.addKey")}</Muted>
        <a href={uri} className="rounded-[10px] bg-fill px-3 py-2.5 text-center font-mono type-callout tracking-wider break-all select-all">
          {secret}
        </a>
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
          <Field label={t("site.field.code")} name="code" inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6} required />
          <Button type="submit" className="min-h-11">
            {t("site.account.turnOn")}
          </Button>
        </div>
        <ErrorText error={error} />
      </form>
    );
  }
  return (
    <form
      className="flex flex-col gap-3"
      onSubmit={async (e) => {
        e.preventDefault();
        const password = String(new FormData(e.currentTarget).get("password"));
        const res = enabled ? await authClient.twoFactor.disable({ password }) : await authClient.twoFactor.enable({ password });
        if (res.error) setError(describe(t, res.error));
        else if (res.data && "totpURI" in res.data) setUri(res.data.totpURI);
        else window.location.reload();
      }}
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <Field label={t("site.field.password")} name="password" type="password" autoComplete="current-password" autoFocus required />
        <Button type="submit" variant={enabled ? "danger" : "primary"} className="min-h-11">
          {t(enabled ? "site.account.turnOff" : "site.action.continue")}
        </Button>
      </div>
      <ErrorText error={error} />
    </form>
  );
}

function Passkeys() {
  const t = useT();
  const list = authClient.useListPasskeys();
  const [error, setError] = useState<string>();
  return (
    <Group title={t("site.account.passkeys")} footer={<ErrorText error={error} />}>
      {list.data?.length === 0 && (
        <Block>
          <Muted>{t("site.account.noPasskeys")}</Muted>
        </Block>
      )}
      {list.data?.map((p) => (
        <Row key={p.id} leading={<Icon name="key" size={22} className="text-text-secondary" />} label={p.name || t("site.account.passkey")} detail={new Date(p.createdAt).toLocaleDateString()}>
          <Button
            variant="danger"
            onClick={async () => {
              const res = await authClient.passkey.deletePasskey({ id: p.id });
              setError(describe(t, res.error));
            }}
          >
            {t("site.action.remove")}
          </Button>
        </Row>
      ))}
      <Block>
        <Actions>
          <Button
            variant="secondary"
            onClick={async () => {
              const res = await authClient.passkey.addPasskey();
              setError(describe(t, res?.error));
            }}
          >
            <Icon name="add" size={18} />
            {t("site.account.addPasskey")}
          </Button>
        </Actions>
      </Block>
    </Group>
  );
}

function Danger() {
  const t = useT();
  const confirm = useConfirm();
  const report = useMutation({
    mutationFn: () => accounts.reportCompromised({}),
    onSuccess: () => {
      window.location.href = "/sign-in";
    },
  });
  const [error, setError] = useState<string>();
  const [deleting, setDeleting] = useState(false);
  return (
    <Group title={t("site.account.danger")}>
      <Row label={t("site.account.compromised")} detail={t("site.account.compromisedBody")}>
        <Button
          variant="danger"
          onClick={async () => {
            if (await confirm({ title: t("site.account.compromisedConfirm"), message: t("site.account.compromisedBody"), action: t("site.account.compromised"), destructive: true })) report.mutate();
          }}
        >
          {t("site.account.compromised")}
        </Button>
      </Row>
      <div>
        <Row label={t("site.account.delete")} detail={t("site.account.deleteBody")}>
          <Button variant="danger" onClick={() => setDeleting(!deleting)}>
            {deleting ? t("site.action.cancel") : t("site.account.delete")}
          </Button>
        </Row>
        {deleting && (
          <form
            className="flex flex-col gap-3 px-4 pb-4 sm:flex-row sm:items-end"
            onSubmit={async (e) => {
              e.preventDefault();
              const password = String(new FormData(e.currentTarget).get("password"));
              if (!(await confirm({ title: t("site.account.deleteConfirm"), message: t("site.account.deleteBody"), action: t("site.account.delete"), destructive: true }))) return;
              const res = await authClient.deleteUser({ password });
              if (res.error) setError(describe(t, res.error));
              else window.location.href = "/sign-up";
            }}
          >
            <Field label={t("site.field.password")} name="password" type="password" autoComplete="current-password" autoFocus required />
            <Button type="submit" variant="danger" className="min-h-11">
              {t("site.account.delete")}
            </Button>
          </form>
        )}
      </div>
      {(error || report.error) && (
        <Block>
          <ErrorText error={error ?? describe(t, report.error)} />
        </Block>
      )}
    </Group>
  );
}

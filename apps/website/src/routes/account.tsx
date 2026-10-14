import { timestampDate } from "@bufbuild/protobuf/wkt";
import { AccountKind } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";

import { accounts, homes, message } from "../lib/api.ts";
import { authClient } from "../lib/auth-client.ts";
import { useAccount } from "../lib/session.ts";
import { Button, Card, ErrorText, Field, Muted, Page } from "../ui.tsx";

export const Route = createFileRoute("/account")({ component: AccountPage });

function AccountPage() {
  const me = useAccount();
  const account = useQuery({ queryKey: ["account"], queryFn: () => accounts.getAccount({}), enabled: !!me });
  if (!me || !account.data) return null;
  const a = account.data;
  if (a.account?.kind === AccountKind.MANAGED) {
    return (
      <Page title="Account">
        <Card title={a.account.displayName}>
          <Muted>@{a.account.userName} · a managed account, which your Home's administrator manages.</Muted>
        </Card>
        <Pass expires={a.passExpireTime && timestampDate(a.passExpireTime)} effective={a.effectivePass} />
      </Page>
    );
  }
  return (
    <Page title="Account">
      <Profile name={a.account?.displayName ?? ""} userName={a.account?.userName ?? ""} email={a.email} />
      <Pass expires={a.passExpireTime && timestampDate(a.passExpireTime)} effective={a.effectivePass} />
      <Pin id={me.id} has={a.hasPin} />
      <Passkeys />
      <TwoFactor enabled={!!(me as { twoFactorEnabled?: boolean | null }).twoFactorEnabled} />
      <Danger />
    </Page>
  );
}

function Pass({ expires, effective }: { expires?: Date; effective: boolean }) {
  return (
    <Card title="Mavio Pass">
      <Muted>
        {expires
          ? `Your Mavio Pass runs until ${expires.toLocaleDateString()}.`
          : effective
            ? "You share your Home administrator's Mavio Pass."
            : "You have no Mavio Pass. It pays for what mavio.dev provides, such as the relay; nothing your servers do depends on it."}
      </Muted>
    </Card>
  );
}

function Pin({ id, has }: { id: string; has: boolean }) {
  const qc = useQueryClient();
  const set = useMutation({
    mutationFn: (pin: string) => homes.setPin({ accountId: id, pin }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["account"] }),
  });
  return (
    <Card title="PIN">
      <Muted>
        {has
          ? "Switching to you in your Home needs your PIN."
          : "Without a PIN, anyone in your Home can switch to you on a shared device."}
      </Muted>
      <form
        className="flex items-end gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          const form = e.currentTarget;
          set.mutate(String(new FormData(form).get("pin")), { onSuccess: () => form.reset() });
        }}
      >
        <Field label={has ? "New PIN" : "PIN"} name="pin" inputMode="numeric" pattern="[0-9]{4}" maxLength={4} required />
        <Button type="submit">Set</Button>
        {has && (
          <Button kind="secondary" onClick={() => set.mutate("")}>
            Remove
          </Button>
        )}
      </form>
      <ErrorText error={set.error ? message(set.error) : undefined} />
    </Card>
  );
}

function Profile({ name, userName, email }: { name: string; userName: string; email: string }) {
  const qc = useQueryClient();
  const [error, setError] = useState<string>();
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const res = await authClient.updateUser({ name: String(new FormData(e.currentTarget).get("name")) });
    setError(res.error?.message);
    await qc.invalidateQueries({ queryKey: ["account"] });
  }
  return (
    <Card title="Profile">
      <Muted>
        @{userName} · {email}
      </Muted>
      <form className="flex flex-col gap-3" onSubmit={submit}>
        <Field label="Display name" name="name" defaultValue={name} required />
        <ErrorText error={error} />
        <Button type="submit">Save</Button>
      </form>
    </Card>
  );
}

function Passkeys() {
  const list = authClient.useListPasskeys();
  const [error, setError] = useState<string>();
  return (
    <Card title="Passkeys">
      <ul className="flex flex-col gap-2">
        {list.data?.map((p) => (
          <li key={p.id} className="flex items-center gap-3">
            <span className="type-body flex-1">{p.name || "Passkey"}</span>
            <Button
              kind="danger"
              onClick={async () => {
                const res = await authClient.passkey.deletePasskey({ id: p.id });
                setError(res.error?.message);
              }}
            >
              Remove
            </Button>
          </li>
        ))}
      </ul>
      <ErrorText error={error} />
      <Button
        kind="secondary"
        onClick={async () => {
          const res = await authClient.passkey.addPasskey();
          setError(res?.error?.message);
        }}
      >
        Add a passkey
      </Button>
    </Card>
  );
}

function TwoFactor({ enabled }: { enabled: boolean }) {
  const [uri, setUri] = useState<string>();
  const [error, setError] = useState<string>();
  async function start(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const password = String(new FormData(e.currentTarget).get("password"));
    const res = enabled ? await authClient.twoFactor.disable({ password }) : await authClient.twoFactor.enable({ password });
    if (res.error) setError(res.error.message);
    else if (res.data && "totpURI" in res.data) setUri(res.data.totpURI);
    else window.location.reload();
  }
  async function confirmCode(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const res = await authClient.twoFactor.verifyTotp({ code: String(new FormData(e.currentTarget).get("code")) });
    if (res.error) setError(res.error.message);
    else window.location.reload();
  }
  return (
    <Card title="Two-factor authentication">
      <Muted>{enabled ? "Signing in with a password also asks for a code of your authenticator app." : "Ask for a code of an authenticator app after your password."}</Muted>
      {uri ? (
        <form className="flex flex-col gap-3" onSubmit={confirmCode}>
          <Muted>Add this key to your authenticator app, then enter the code it shows:</Muted>
          <code className="type-footnote break-all">{uri}</code>
          <Field label="Code" name="code" inputMode="numeric" autoComplete="one-time-code" required />
          <Button type="submit">Turn on</Button>
        </form>
      ) : (
        <form className="flex flex-col gap-3" onSubmit={start}>
          <Field label="Password" name="password" type="password" autoComplete="current-password" required />
          <Button type="submit" kind={enabled ? "danger" : "primary"}>
            {enabled ? "Turn off" : "Set up"}
          </Button>
        </form>
      )}
      <ErrorText error={error} />
    </Card>
  );
}

function Danger() {
  const report = useMutation({
    mutationFn: () => accounts.reportCompromised({}),
    onSuccess: () => {
      window.location.href = "/sign-in";
    },
  });
  const [error, setError] = useState<string>();
  async function remove(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!confirm("Delete your Mavio account? Servers delete your user and its data.")) return;
    const res = await authClient.deleteUser({ password: String(new FormData(e.currentTarget).get("password")) });
    if (res.error) setError(res.error.message);
    else window.location.href = "/sign-up";
  }
  return (
    <Card title="Security">
      <Muted>If someone else may have signed in to your account, sign out everywhere: your servers end your sessions too. Then change your password.</Muted>
      <Button kind="danger" onClick={() => confirm("Sign out everywhere?") && report.mutate()}>
        Report compromised
      </Button>
      <ErrorText error={report.error ? message(report.error) : undefined} />
      <form className="flex flex-col gap-3 border-t border-border pt-4" onSubmit={remove}>
        <Muted>Deleting the account removes it from every server you have access to, with your watch history there.</Muted>
        <Field label="Password" name="password" type="password" autoComplete="current-password" required />
        <ErrorText error={error} />
        <Button type="submit" kind="danger">
          Delete account
        </Button>
      </form>
    </Card>
  );
}

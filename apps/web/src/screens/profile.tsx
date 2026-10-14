// The profile: the servers the user signed in to, in their order, which
// orders Home; adding, signing out and signing in again; switching users
// and the user's passkeys, authenticator app and PIN; and authorizing a
// device with Quick Connect.
import { errorMessage, type ServerEntry, useServers, useServersStore, useSignedIn } from "@mavio/client-ts";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { Button, Field, ScreenTitle } from "../ui.tsx";
import { Security, SwitchUser } from "./security.tsx";

export function ProfileScreen() {
  const servers = useServersStore();
  const list = useServers();
  const navigate = useNavigate();
  const move = (i: number, by: number) => {
    const ids = list.map((s) => s.id);
    [ids[i], ids[i + by]] = [ids[i + by], ids[i]];
    servers.order(ids);
  };
  return (
    <div className="max-w-3xl">
      <ScreenTitle>{t("nav.profile")}</ScreenTitle>
      <section className="px-margin py-4">
        <div className="flex items-center pb-3">
          <h2 className="type-title3">{t("auth.servers")}</h2>
          <Link
            to="/sign-in"
            search={{ add: true }}
            className="ml-auto inline-flex min-h-11 items-center gap-2 rounded-md px-3 type-headline text-accent hover:bg-surface-raised"
          >
            <Icon name="add" />
            {t("auth.addServer")}
          </Link>
        </div>
        <ul className="flex flex-col divide-y divide-border rounded-lg bg-surface">
          {list.map((s, i) => (
            <li key={s.id} className="flex flex-wrap items-center gap-3 p-4">
              <div className="min-w-0 flex-1">
                <div className="type-headline">{s.name}</div>
                <div className="truncate type-footnote text-text-secondary">
                  {s.token ? t("auth.signedInAs", { name: s.userName }) : t("auth.signedOut")} · {s.url}
                </div>
              </div>
              <div className="flex items-center gap-1">
                <Button variant="plain" aria-label={t("auth.moveUp")} title={t("auth.moveUp")} disabled={i === 0} onClick={() => move(i, -1)}>
                  <Icon name="expand" className="rotate-180" size={20} />
                </Button>
                <Button
                  variant="plain"
                  aria-label={t("auth.moveDown")}
                  title={t("auth.moveDown")}
                  disabled={i === list.length - 1}
                  onClick={() => move(i, 1)}
                >
                  <Icon name="expand" size={20} />
                </Button>
                {!s.token && (
                  <Button variant="secondary" onClick={() => void navigate({ to: "/sign-in", search: { server: s.url, add: true } })}>
                    {t("auth.signIn")}
                  </Button>
                )}
                <Button
                  variant="plain"
                  className="text-danger"
                  // Signing out of the last server leads to sign-in.
                  onClick={() => void servers.remove(s.id)}
                >
                  {t("auth.signOut")}
                </Button>
              </div>
            </li>
          ))}
        </ul>
      </section>
      <Accounts />
      <AuthorizeDevice />
    </div>
  );
}

/** Each standalone server's users to switch to and the signed-in user's credentials. */
function Accounts() {
  const signedIn = useSignedIn().filter((s) => s.mode !== "mavioDev");
  return signedIn.map((s) => (
    <section key={s.id} className="flex flex-col gap-3 px-margin py-4">
      <h2 className="type-title3">
        {t("auth.security")}
        {signedIn.length > 1 && <span className="text-text-secondary"> · {s.name}</span>}
      </h2>
      <SwitchUser server={s} />
      <Security server={s} />
    </section>
  ));
}

/** Signs in another device showing a Quick Connect code, as the user of one of the servers. */
function AuthorizeDevice() {
  const servers = useServersStore();
  const signedIn = useSignedIn();
  const [serverId, setServerId] = useState(signedIn[0]?.id);
  const [code, setCode] = useState("");
  const [state, setState] = useState<{ ok?: boolean; error?: string }>({});
  const server: ServerEntry | undefined = signedIn.find((s) => s.id === serverId) ?? signedIn[0];
  if (!server) return null;
  return (
    <section className="px-margin py-4">
      <h2 className="pb-3 type-title3">{t("auth.authorizeDevice")}</h2>
      <form
        className="flex flex-wrap items-end gap-3 rounded-lg bg-surface p-4"
        onSubmit={(e) => {
          e.preventDefault();
          setState({});
          servers
            .authorizeQuickConnect(server, code)
            .then(() => (setState({ ok: true }), setCode("")))
            .catch((err: unknown) => setState({ error: errorMessage(err) }));
        }}
      >
        {signedIn.length > 1 && (
          <label className="flex flex-col gap-1.5">
            <span className="type-callout text-text-secondary">{t("auth.server")}</span>
            <select
              value={server.id}
              onChange={(e) => setServerId(e.target.value)}
              className="min-h-11 rounded-md border border-border bg-surface px-3 type-body"
            >
              {signedIn.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
          </label>
        )}
        <div className="w-40">
          <Field
            label="Quick Connect"
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
            inputMode="numeric"
            placeholder="000000"
            className="font-mono"
          />
        </div>
        <Button type="submit" disabled={code.length < 6}>
          {t("auth.authorize")}
        </Button>
        {state.ok && <p className="w-full type-callout text-text-secondary">{t("auth.authorized")}</p>}
        {state.error && <p className="w-full type-callout text-danger">{state.error}</p>}
      </form>
    </section>
  );
}

// Sign-in to the serving server or another one: the address, then a
// password, the first user of a new server, or Quick Connect.
import { errorMessage, isUnauthenticated, type QuickConnect, type ServerInfo, useServersStore } from "@mavio/client-ts";
import { getRouteApi, useNavigate } from "@tanstack/react-router";
import { type FormEvent, useEffect, useState } from "react";

import { t } from "../i18n.ts";
import { Button, Field } from "../ui.tsx";

const route = getRouteApi("/sign-in");

export function SignInScreen() {
  const servers = useServersStore();
  const navigate = useNavigate();
  const search = route.useSearch();
  // Fixed on arrival: signing in must not turn the first sign-in into adding.
  const [adding] = useState(() => !!search.add || servers.signedIn().length > 0);
  const [address, setAddress] = useState(search.server ?? (adding ? "" : servers.home));
  const [info, setInfo] = useState<ServerInfo>();
  const [qc, setQc] = useState<QuickConnect>();
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  const done = () => navigate({ to: adding ? "/profile" : "/" });

  const run = async (f: () => Promise<void>) => {
    setBusy(true);
    setError(undefined);
    try {
      await f();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  const probe = (a: string) =>
    run(async () => {
      try {
        setInfo(await servers.probe(a));
      } catch (err) {
        throw /not a Mavio server/.test(String(err)) ? new Error(t("auth.notMavio")) : err;
      }
    });

  // The serving server or the one to sign in to again is asked at once.
  useEffect(() => {
    if (address && !adding) void probe(address);
    else if (search.server) void probe(search.server);
    // Only on arrival.
  }, []);

  // Quick Connect waits for the code to be entered on another device.
  useEffect(() => {
    if (!qc) return;
    let stop = false;
    const tick = async () => {
      if (stop) return;
      if (Date.now() > qc.expires.getTime()) {
        setQc(undefined);
        return;
      }
      try {
        if (await servers.pollQuickConnect(qc)) {
          done();
          return;
        }
      } catch (err) {
        setError(errorMessage(err));
        setQc(undefined);
        return;
      }
      setTimeout(tick, 3000);
    };
    const first = setTimeout(tick, 3000);
    return () => {
      stop = true;
      clearTimeout(first);
    };
  }, [qc]);

  return (
    <div className="flex min-h-screen items-center justify-center p-margin">
      <div className="w-full max-w-sm rounded-xl bg-surface p-6 shadow-level2">
        <div className="mb-1 type-title2 text-accent">Mavio</div>
        {!info ? (
          <form
            className="flex flex-col gap-4"
            onSubmit={(e: FormEvent) => {
              e.preventDefault();
              void probe(address);
            }}
          >
            <h1 className="type-title1">{adding ? t("auth.addServer") : t("auth.welcome")}</h1>
            <Field
              label={t("auth.server")}
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              placeholder="https://media.example.com"
              autoFocus
              required
              inputMode="url"
              autoCapitalize="off"
              autoCorrect="off"
            />
            {error && <p className="type-callout text-danger">{error}</p>}
            <div className="flex gap-2">
              {adding && (
                <Button variant="secondary" onClick={() => navigate({ to: "/profile" })}>
                  {t("action.cancel")}
                </Button>
              )}
              <Button type="submit" disabled={busy || !address.trim()} className="flex-1">
                {t("auth.signIn")}
              </Button>
            </div>
          </form>
        ) : qc ? (
          <div className="flex flex-col gap-4">
            <h1 className="type-title1">{info.name}</h1>
            <p className="type-body text-text-secondary">{t("auth.quickConnectCode", { code: qc.code })}</p>
            <div className="rounded-lg bg-surface-raised py-4 text-center font-mono type-display tracking-[0.3em]">
              {qc.code}
            </div>
            <Button variant="secondary" onClick={() => setQc(undefined)}>
              {t("action.cancel")}
            </Button>
          </div>
        ) : (
          <Credentials
            info={info}
            busy={busy}
            error={error}
            onBack={() => (setInfo(undefined), setError(undefined))}
            onSubmit={(name, password) =>
              run(async () => {
                try {
                  if (info.setupRequired) await servers.createFirstUser(info.url, name, password);
                  else await servers.signIn(info.url, name, password);
                } catch (err) {
                  throw isUnauthenticated(err) ? new Error(t("auth.wrongPassword")) : err;
                }
                done();
              })
            }
            onQuickConnect={() => run(async () => setQc(await servers.startQuickConnect(info.url)))}
          />
        )}
      </div>
    </div>
  );
}

function Credentials({
  info,
  busy,
  error,
  onBack,
  onSubmit,
  onQuickConnect,
}: {
  info: ServerInfo;
  busy: boolean;
  error?: string;
  onBack: () => void;
  onSubmit: (name: string, password: string) => void;
  onQuickConnect: () => void;
}) {
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(name.trim(), password);
      }}
    >
      <div>
        <h1 className="type-title1">{info.setupRequired ? t("auth.setUp", { server: info.name }) : info.name}</h1>
        <button type="button" onClick={onBack} className="type-footnote text-text-secondary hover:underline">
          {info.url}
        </button>
      </div>
      {info.setupRequired && <p className="type-body text-text-secondary">{t("auth.setUpMessage")}</p>}
      <Field
        label={t("auth.username")}
        value={name}
        onChange={(e) => setName(e.target.value)}
        autoComplete="username"
        autoCapitalize="off"
        autoFocus
        required
      />
      <Field
        label={t("auth.password")}
        type="password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        autoComplete={info.setupRequired ? "new-password" : "current-password"}
        required={info.setupRequired}
      />
      {error && <p className="type-callout text-danger">{error}</p>}
      <Button type="submit" disabled={busy || !name.trim()}>
        {info.setupRequired ? t("auth.createAccount") : t("auth.signIn")}
      </Button>
      {!info.setupRequired && (
        <Button variant="plain" onClick={onQuickConnect} disabled={busy}>
          {t("auth.quickConnect")}
        </Button>
      )}
    </form>
  );
}

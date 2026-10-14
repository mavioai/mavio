// Sign-in to the serving server or another one: the address, then a
// password with the code of the user's authenticator app when it has one,
// a passkey or Quick Connect; on a server in mavio.dev mode, a Mavio
// account through mavio.dev (docs/accounts.md §6). A new server's setup
// chooses its mode: claiming it for a Mavio account, or creating its
// first user.
import {
  errorMessage,
  isTotpRequired,
  isUnauthenticated,
  type QuickConnect,
  type ServerInfo,
  useServersStore,
} from "@mavio/client-ts";
import { getRouteApi, Link, useNavigate } from "@tanstack/react-router";
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
  const [needsCode, setNeedsCode] = useState(false);
  // The mode a new server's setup chose.
  const [setupMode, setSetupMode] = useState<"standalone" | "mavioDev">();

  const done = () => navigate({ to: adding ? "/profile" : "/" });

  // Goes to mavio.dev, which comes back to /sign-in/mavio-dev.
  const toMavioDev = async (url: string) => {
    window.location.href = await servers.startMavioDevSignIn(url, `${location.origin}/sign-in/mavio-dev`);
  };

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
        ) : info.setupRequired && !info.mode && !setupMode ? (
          <ChooseMode info={info} onBack={() => setInfo(undefined)} onChoose={setSetupMode} />
        ) : info.mode === "mavioDev" || setupMode === "mavioDev" ? (
          info.setupRequired ? (
            <Claim
              info={info}
              busy={busy}
              error={error}
              onBack={() => (setSetupMode(undefined), setError(undefined))}
              onClaim={(code) =>
                run(async () => {
                  await servers.claimServer(info.url, code);
                  await toMavioDev(info.url);
                })
              }
            />
          ) : (
            <div className="flex flex-col gap-4">
              <ServerHeading info={info} title={info.name} onBack={() => (setInfo(undefined), setError(undefined))} />
              <p className="type-body text-text-secondary">{t("auth.mavioDevMessage")}</p>
              {error && <p className="type-callout text-danger">{error}</p>}
              <Button disabled={busy} onClick={() => run(() => toMavioDev(info.url))}>
                {t("auth.signInWithMavio")}
              </Button>
            </div>
          )
        ) : (
          <Credentials
            info={info}
            busy={busy}
            error={error}
            onBack={() =>
              info.setupRequired && !info.mode
                ? (setSetupMode(undefined), setError(undefined))
                : (setInfo(undefined), setError(undefined), setNeedsCode(false))
            }
            onSubmit={(name, password, code) =>
              run(async () => {
                try {
                  if (info.setupRequired) await servers.createFirstUser(info.url, name, password);
                  else await servers.signIn(info.url, name, password, code);
                } catch (err) {
                  if (!code && isTotpRequired(err)) {
                    setNeedsCode(true);
                    return;
                  }
                  throw isUnauthenticated(err) ? new Error(t(code ? "auth.wrongCode" : "auth.wrongPassword")) : err;
                }
                done();
              })
            }
            needsCode={needsCode}
            onPasskey={() =>
              run(async () => {
                await servers.signInWithPasskey(info.url);
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

/** Where mavio.dev comes back to with the code of a Mavio account's sign-in. */
export function MavioDevCallbackScreen() {
  const servers = useServersStore();
  const navigate = useNavigate();
  const [error, setError] = useState<string>();
  useEffect(() => {
    servers.finishMavioDevSignIn(new URLSearchParams(location.search)).then(
      () => void navigate({ to: "/", replace: true }),
      (err: unknown) => setError(errorMessage(err)),
    );
    // Once: the code is spent.
  }, []);
  return (
    <div className="flex min-h-screen items-center justify-center p-margin">
      <div className="flex w-full max-w-sm flex-col gap-4 rounded-xl bg-surface p-6 shadow-level2">
        <div className="type-title2 text-accent">Mavio</div>
        {error ? (
          <>
            <p className="type-callout text-danger">{error}</p>
            <Link to="/sign-in" className="type-headline text-accent hover:underline">
              {t("auth.tryAgain")}
            </Link>
          </>
        ) : (
          <p className="type-body text-text-secondary">{t("auth.signingIn")}</p>
        )}
      </div>
    </div>
  );
}

function ServerHeading({ info, title, onBack }: { info: ServerInfo; title: string; onBack: () => void }) {
  return (
    <div>
      <h1 className="type-title1">{title}</h1>
      <button type="button" onClick={onBack} className="type-footnote text-text-secondary hover:underline">
        {info.url}
      </button>
    </div>
  );
}

/** A new server's choice of mode (docs/accounts.md §2). */
function ChooseMode({
  info,
  onBack,
  onChoose,
}: {
  info: ServerInfo;
  onBack: () => void;
  onChoose: (mode: "standalone" | "mavioDev") => void;
}) {
  const option = (mode: "standalone" | "mavioDev", title: string, message: string) => (
    <button
      type="button"
      onClick={() => onChoose(mode)}
      className="flex flex-col gap-1 rounded-lg border border-border p-4 text-left hover:border-accent hover:bg-surface-raised"
    >
      <span className="type-headline">{title}</span>
      <span className="type-callout text-text-secondary">{message}</span>
    </button>
  );
  return (
    <div className="flex flex-col gap-4">
      <ServerHeading info={info} title={t("auth.chooseMode", { server: info.name })} onBack={onBack} />
      {option("mavioDev", t("auth.modeMavioDev"), t("auth.modeMavioDevMessage"))}
      {option("standalone", t("auth.modeStandalone"), t("auth.modeStandaloneMessage"))}
    </div>
  );
}

/** Claims a new server for the Mavio account whose claim code is entered. */
function Claim({
  info,
  busy,
  error,
  onBack,
  onClaim,
}: {
  info: ServerInfo;
  busy: boolean;
  error?: string;
  onBack: () => void;
  onClaim: (code: string) => void;
}) {
  const [code, setCode] = useState("");
  const site = info.mavioDevUrl?.replace(/\/api\/?$/, "");
  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        onClaim(code);
      }}
    >
      <ServerHeading info={info} title={t("auth.claim", { server: info.name })} onBack={onBack} />
      <p className="type-body text-text-secondary">{t("auth.claimMessage")}</p>
      {site && (
        <a href={`${site}/claim`} target="_blank" rel="noopener" className="type-headline text-accent hover:underline">
          {t("auth.getClaimCode")}
        </a>
      )}
      <Field
        label={t("auth.claimCode")}
        value={code}
        onChange={(e) => setCode(e.target.value.toUpperCase())}
        placeholder="XXXX-XXXX"
        autoComplete="off"
        autoCapitalize="characters"
        className="font-mono"
        autoFocus
        required
      />
      {error && <p className="type-callout text-danger">{error}</p>}
      <Button type="submit" disabled={busy || !code.trim()}>
        {t("auth.claimServer")}
      </Button>
    </form>
  );
}

/**
 * Whether this page can use passkeys of the server at a URL: a passkey
 * belongs to the host the browser reached the server at, so only the
 * serving server's, at a host name over HTTPS or on localhost.
 */
export function passkeysHere(url: string): boolean {
  if (typeof PublicKeyCredential === "undefined" || !window.isSecureContext) return false;
  const u = new URL(url);
  return u.origin === location.origin && !/^[\d.]+$|^\[/.test(u.hostname);
}

function Credentials({
  info,
  busy,
  error,
  needsCode,
  onBack,
  onSubmit,
  onPasskey,
  onQuickConnect,
}: {
  info: ServerInfo;
  busy: boolean;
  error?: string;
  needsCode: boolean;
  onBack: () => void;
  onSubmit: (name: string, password: string, code?: string) => void;
  onPasskey: () => void;
  onQuickConnect: () => void;
}) {
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(name.trim(), password, needsCode ? code : undefined);
      }}
    >
      <ServerHeading info={info} title={info.setupRequired ? t("auth.setUp", { server: info.name }) : info.name} onBack={onBack} />
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
      {needsCode && (
        <>
          <p className="type-body text-text-secondary">{t("auth.totpMessage")}</p>
          <Field
            label={t("auth.totpCode")}
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
            autoComplete="one-time-code"
            inputMode="numeric"
            autoFocus
            required
          />
        </>
      )}
      {error && <p className="type-callout text-danger">{error}</p>}
      <Button type="submit" disabled={busy || !name.trim() || (needsCode && code.length !== 6)}>
        {info.setupRequired ? t("auth.createAccount") : t("auth.signIn")}
      </Button>
      {!info.setupRequired && passkeysHere(info.url) && (
        <Button variant="secondary" onClick={onPasskey} disabled={busy}>
          {t("auth.signInWithPasskey")}
        </Button>
      )}
      {!info.setupRequired && (
        <Button variant="plain" onClick={onQuickConnect} disabled={busy}>
          {t("auth.quickConnect")}
        </Button>
      )}
    </form>
  );
}

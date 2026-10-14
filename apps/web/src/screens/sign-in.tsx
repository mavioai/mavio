// Sign-in (docs/clients.md §6.3, docs/accounts.md §6): with a Mavio account
// through mavio.dev, the last one used here offered first, or to a server
// by its address, those signed in to before listed first; on a server, a
// password with the code of the user's authenticator app when it has one,
// a passkey or Quick Connect. A new server's setup chooses its mode:
// claiming it for a Mavio account, or creating its first user. On a
// television the two ways are tabs on the left.
import {
  errorMessage,
  isTotpRequired,
  isUnauthenticated,
  type QuickConnect,
  type ServerEntry,
  type ServerInfo,
  signInKey,
  useServers,
  useServersStore,
} from "@mavio/client-ts";
import { useQuery } from "@tanstack/react-query";
import { getRouteApi, Link, useNavigate } from "@tanstack/react-router";
import { type ReactNode, useEffect, useState } from "react";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { Wordmark } from "../logo.tsx";
import { usePrefs } from "../prefs.ts";
import { useSpatialNavigation } from "../shell/spatial.ts";
import { Avatar, Button, Field } from "../ui.tsx";

const route = getRouteApi("/sign-in");

type Step = "welcome" | "mavio" | "servers" | "new" | "server";

export function SignInScreen() {
  const servers = useServersStore();
  const list = useServers();
  const navigate = useNavigate();
  const search = route.useSearch();
  const { tv } = usePrefs();
  // Fixed on arrival: signing in must not turn the first sign-in into adding.
  const [adding] = useState(() => !!search.add || servers.signedIn().length > 0);
  const [step, setStep] = useState<Step>(search.server ? "server" : tv ? "mavio" : "welcome");
  // The serving server is offered until it is kept.
  const [address, setAddress] = useState(search.server ?? (list.some((s) => s.url === servers.home) ? "" : servers.home));
  const [info, setInfo] = useState<ServerInfo>();
  const [qc, setQc] = useState<QuickConnect>();
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const [needsCode, setNeedsCode] = useState(false);
  // The mode a new server's setup chose.
  const [setupMode, setSetupMode] = useState<"standalone" | "mavioDev">();

  // The serving server: a new one is set up first; one in mavio.dev mode
  // signs Mavio accounts in.
  const home = useQuery({ queryKey: ["authInfo", servers.home], queryFn: () => servers.probe(servers.home), retry: false });
  useEffect(() => {
    if (home.data?.setupRequired && !adding && !search.server) {
      setInfo(home.data);
      setStep("server");
    }
  }, [home.data]);

  const lastAccount = list.find((s) => s.mode === "mavioDev" && s.account);
  // The server that signs a Mavio account in: the serving one, else the last one's.
  const mavioServer = home.data?.mode === "mavioDev" ? home.data.url : lastAccount?.url;
  const standalone = list.filter((s) => s.mode !== "mavioDev");

  // A television starts on the Mavio tab only where a server signs Mavio accounts in.
  useEffect(() => {
    if (tv && step === "mavio" && home.isFetched && !mavioServer) setStep(standalone.length ? "servers" : "new");
  }, [home.isFetched]);

  const done = () => navigate({ to: adding ? "/settings" : "/" });

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
        setStep("server");
      } catch (err) {
        throw /not a Mavio server/.test(String(err)) ? new Error(t("auth.notMavio")) : err;
      }
    });

  // The server to sign in to again is asked at once.
  useEffect(() => {
    if (search.server) void probe(search.server);
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

  const go = (to: Step) => {
    // With no server signed in to before, connecting starts at the address.
    const s = to === "servers" && standalone.length === 0 ? "new" : to;
    setError(undefined);
    setNeedsCode(false);
    setSetupMode(undefined);
    setQc(undefined);
    if (s !== "server") setInfo(undefined);
    setStep(s);
  };

  /** Opens a server signed in to before: shows it when still signed in, else signs in again. */
  const openServer = (s: ServerEntry) => {
    if (s.token) {
      servers.show(signInKey(s));
      void navigate({ to: "/" });
    } else {
      setAddress(s.url);
      void probe(s.url);
    }
  };

  let content: ReactNode;
  if (step === "welcome") {
    content = (
      <div className="flex flex-col items-center gap-3 text-center">
        <Wordmark height={40} className="mb-4" />
        <h1 className="type-title1">{t("auth.welcome")}</h1>
        <p className="mb-4 type-body text-text-secondary">{t("auth.welcomeMessage")}</p>
        {(mavioServer || home.isPending) && (
          <Button className="w-full" disabled={!mavioServer} onClick={() => go("mavio")}>
            {t("auth.signInWithMavio")}
          </Button>
        )}
        <Button variant="secondary" className="w-full" onClick={() => go("servers")}>
          {t("auth.connectToServer")}
        </Button>
      </div>
    );
  } else if (step === "mavio") {
    content = (
      <Steps title={t("auth.signInWithMavio")} onBack={tv ? undefined : () => go("welcome")}>
        {lastAccount && (
          <>
            <p className="type-callout text-text-secondary">{t("auth.lastAccount")}</p>
            <div className="flex items-center gap-3 rounded-[14px] bg-fill p-3">
              <Avatar name={lastAccount.account!.name} size={44} />
              <div className="min-w-0 flex-1">
                <div className="truncate type-headline">{lastAccount.account!.name}</div>
                <div className="truncate type-footnote text-text-secondary">{t("auth.mavioAccount")}</div>
              </div>
              <Button disabled={busy} onClick={() => run(() => toMavioDev(lastAccount.url))}>
                {t("auth.continue")}
              </Button>
            </div>
          </>
        )}
        {!mavioServer && !home.isPending && <p className="type-callout text-text-secondary">{t("auth.noMavioServer")}</p>}
        <Button
          variant={lastAccount ? "secondary" : "primary"}
          disabled={busy || !mavioServer}
          onClick={() => mavioServer && run(() => toMavioDev(mavioServer))}
        >
          {lastAccount ? t("auth.otherAccount") : t("auth.signInWithMavio")}
        </Button>
        {error && <p className="type-callout text-danger">{error}</p>}
      </Steps>
    );
  } else if (step === "servers") {
    content = (
      <Steps title={t("auth.connectToServer")} onBack={tv ? undefined : () => go("welcome")}>
        <p className="type-callout text-text-secondary">{t("auth.recentServers")}</p>
        <div className="flex flex-col divide-y divide-border overflow-hidden rounded-[14px] bg-fill">
          {standalone.map((s) => (
            <button
              key={s.id}
              type="button"
              disabled={busy}
              onClick={() => openServer(s)}
              className="flex items-center gap-3 p-3 text-left hover:bg-fill-selected"
            >
              <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] bg-surface-raised type-headline">
                {[...s.name][0]?.toUpperCase()}
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate type-headline">{s.name}</span>
                <span className="block truncate type-footnote text-text-secondary">
                  {s.userName} · {s.token ? t("auth.shown") : t("auth.signedOut")}
                </span>
              </span>
              <Icon name="forward" size={16} className="text-text-tertiary" />
            </button>
          ))}
        </div>
        {error && <p className="type-callout text-danger">{error}</p>}
        <Button variant="secondary" onClick={() => go("new")}>
          <Icon name="add" size={20} />
          {t("auth.newServer")}
        </Button>
      </Steps>
    );
  } else if (step === "new") {
    content = (
      <Steps
        title={t("auth.newServer")}
        onBack={standalone.length ? () => go("servers") : tv ? undefined : () => go("welcome")}
      >
        <form
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            void probe(address);
          }}
        >
          <Field
            label={t("auth.server")}
            value={address}
            onChange={(e) => setAddress(e.target.value)}
            placeholder={t("auth.serverAddress")}
            autoFocus
            required
            inputMode="url"
            autoCapitalize="off"
            autoCorrect="off"
          />
          {error && <p className="type-callout text-danger">{error}</p>}
          <Button type="submit" disabled={busy || !address.trim()}>
            {t("auth.connect")}
          </Button>
        </form>
      </Steps>
    );
  } else if (!info) {
    content = (
      <Steps title={t("auth.connectToServer")} onBack={() => go("new")}>
        {error ? <p className="type-callout text-danger">{error}</p> : <p className="type-body text-text-secondary">{t("state.loading")}</p>}
      </Steps>
    );
  } else if (qc) {
    content = (
      <Steps title={t("auth.quickConnectTitle")} onBack={() => setQc(undefined)}>
        <p className="type-body text-text-secondary">{t("auth.quickConnectCode", { code: qc.code })}</p>
        <div className="flex justify-center gap-2" aria-label={qc.code}>
          {[...qc.code].map((d, i) => (
            <span key={i} className="flex h-14 w-11 items-center justify-center rounded-[10px] bg-fill font-mono type-title1">
              {d}
            </span>
          ))}
        </div>
        <p className="flex items-center justify-center gap-2 type-callout text-text-secondary">
          <span className="h-3 w-3 animate-spin rounded-full border-2 border-text-tertiary border-t-transparent" />
          {t("auth.waiting")}
        </p>
      </Steps>
    );
  } else if (info.setupRequired && !info.mode && !setupMode) {
    content = <ChooseMode info={info} onBack={() => go("new")} onChoose={setSetupMode} />;
  } else if (info.mode === "mavioDev" || setupMode === "mavioDev") {
    content = info.setupRequired ? (
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
      <Steps title={info.name} subtitle={info.url} onBack={() => go("new")}>
        <p className="type-body text-text-secondary">{t("auth.mavioDevMessage")}</p>
        {error && <p className="type-callout text-danger">{error}</p>}
        <Button disabled={busy} onClick={() => run(() => toMavioDev(info.url))}>
          {t("auth.signInWithMavio")}
        </Button>
      </Steps>
    );
  } else {
    content = (
      <Credentials
        info={info}
        busy={busy}
        error={error}
        onBack={() =>
          info.setupRequired && !info.mode ? (setSetupMode(undefined), setError(undefined)) : go("servers")
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
    );
  }

  if (tv) {
    const mavioTab = step === "mavio";
    return (
      <Backdrop>
        <SpatialNavigation />
        <div className="flex w-full max-w-[1400px] gap-16 px-margin">
          <div className="flex w-[420px] shrink-0 flex-col gap-3">
            <Wordmark height={48} className="mb-8" />
            {(
              [
                ["mavio", t("auth.signInWithMavio"), mavioTab],
                ["servers", t("auth.connectToServer"), !mavioTab],
              ] as const
            ).map(([s, label, on]) => (
              <button
                key={s}
                type="button"
                role="tab"
                aria-selected={on}
                onFocus={() => on || go(s)}
                onClick={() => go(s)}
                className={`rounded-[14px] px-6 py-4 text-left type-headline ${on ? "bg-fill-selected" : "text-text-secondary"}`}
              >
                {label}
              </button>
            ))}
          </div>
          <div className="min-w-0 flex-1 rounded-[28px] bg-surface/80 p-12 backdrop-blur-2xl">{content}</div>
        </div>
      </Backdrop>
    );
  }

  return (
    <Backdrop>
      <div className="relative w-full max-w-[420px] rounded-[22px] border border-border bg-surface/80 p-8 shadow-level3 backdrop-blur-2xl max-sm:min-h-screen max-sm:max-w-none max-sm:rounded-none max-sm:border-0 max-sm:px-margin max-sm:pt-16">
        {adding && step === "welcome" && (
          <Link to="/settings" aria-label={t("action.close")} className="absolute top-4 right-4 rounded-full p-1.5 text-text-secondary hover:bg-fill">
            <Icon name="close" size={20} />
          </Link>
        )}
        {content}
      </div>
    </Backdrop>
  );
}

function SpatialNavigation() {
  useSpatialNavigation();
  return null;
}

/** The screen behind sign-in: the logo's colors, blurred. */
function Backdrop({ children }: { children: ReactNode }) {
  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background sm:p-margin">
      <div aria-hidden className="pointer-events-none absolute inset-0 opacity-50 blur-[120px]">
        <div className="absolute -top-1/4 -left-1/4 h-3/4 w-3/4 rounded-full bg-[#FF9A2E]/60" />
        <div className="absolute -right-1/4 -bottom-1/4 h-3/4 w-3/4 rounded-full bg-[#F0412F]/50" />
        <div className="absolute top-1/3 left-1/2 h-1/3 w-1/3 rounded-full bg-[#FFD34E]/40" />
      </div>
      {children}
    </div>
  );
}

/** A step's back link, title and the line under it, above its content. */
function Steps({
  title,
  subtitle,
  onBack,
  children,
}: {
  title: string;
  subtitle?: string;
  onBack?: () => void;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-4">
      {onBack && (
        <button type="button" onClick={onBack} className="-ml-1 inline-flex items-center gap-1 self-start type-body text-accent-text">
          <Icon name="back" size={18} />
          {t("action.back")}
        </button>
      )}
      <div>
        <h1 className="type-title1">{title}</h1>
        {subtitle && <p className="mt-1 truncate type-footnote text-text-secondary">{subtitle}</p>}
      </div>
      {children}
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
    <Backdrop>
      <div className="flex w-full max-w-[420px] flex-col items-center gap-4 rounded-[22px] border border-border bg-surface/80 p-8 text-center shadow-level3 backdrop-blur-2xl">
        <Wordmark height={32} />
        {error ? (
          <>
            <p className="type-callout text-danger">{error}</p>
            <Link to="/sign-in" className="type-headline text-accent-text hover:underline">
              {t("auth.tryAgain")}
            </Link>
          </>
        ) : (
          <p className="type-body text-text-secondary">{t("auth.signingIn")}</p>
        )}
      </div>
    </Backdrop>
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
      className="flex flex-col gap-1 rounded-[14px] bg-fill p-4 text-left hover:bg-fill-selected"
    >
      <span className="type-headline">{title}</span>
      <span className="type-callout text-text-secondary">{message}</span>
    </button>
  );
  return (
    <Steps title={t("auth.chooseMode", { server: info.name })} subtitle={info.url} onBack={onBack}>
      {option("mavioDev", t("auth.modeMavioDev"), t("auth.modeMavioDevMessage"))}
      {option("standalone", t("auth.modeStandalone"), t("auth.modeStandaloneMessage"))}
    </Steps>
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
    <Steps title={t("auth.claim", { server: info.name })} subtitle={info.url} onBack={onBack}>
      <form
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          onClaim(code);
        }}
      >
        <p className="type-body text-text-secondary">{t("auth.claimMessage")}</p>
        {site && (
          <a href={`${site}/claim`} target="_blank" rel="noopener" className="type-headline text-accent-text hover:underline">
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
    </Steps>
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
    <Steps
      title={info.setupRequired ? t("auth.setUp", { server: info.name }) : t("auth.signInTo", { server: info.name })}
      subtitle={`${t("auth.standaloneServer")} · ${info.url}`}
      onBack={onBack}
    >
      <form
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          onSubmit(name.trim(), password, needsCode ? code : undefined);
        }}
      >
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
        {!info.setupRequired && (
          <>
            <div className="flex items-center gap-3 type-footnote text-text-tertiary">
              <span className="h-px flex-1 bg-border" />
              {t("auth.or")}
              <span className="h-px flex-1 bg-border" />
            </div>
            <div className="flex gap-2">
              {passkeysHere(info.url) && (
                <Button variant="secondary" className="flex-1" onClick={onPasskey} disabled={busy}>
                  <Icon name="key" size={18} />
                  {t("auth.passkey")}
                </Button>
              )}
              <Button variant="secondary" className="flex-1" onClick={onQuickConnect} disabled={busy}>
                <Icon name="devices" size={18} />
                {t("auth.quickConnect")}
              </Button>
            </div>
          </>
        )}
      </form>
    </Steps>
  );
}

// Sign-in and setup (docs/clients.md §4.1, docs/accounts.md §6). A server
// in mavio.dev mode signs in its administrator with their Mavio account at
// mavio.dev, and app.mavio.dev any Mavio account; a standalone server signs
// in its users with a password, the code of their authenticator app when
// they have one, a passkey or Quick Connect. A television signs a Mavio
// account in with a code approved at mavio.dev/link. A new server's setup
// chooses its mode: claiming it for a Mavio account at mavio.dev, or
// creating its administrator; a television asks for a computer or phone.
//
// Each step is in the address, so that the browser's Back, Escape and a
// remote's Back go to the step before. A card on a wall of posters on a
// desktop and a television, the whole screen on a phone.
import {
  errorMessage,
  isTotpRequired,
  isUnauthenticated,
  type LinkCode,
  type QuickConnect,
  type ServerInfo,
  useServersStore,
  useSignIn,
} from "@mavio/client-ts";
import { useQuery } from "@tanstack/react-query";
import { getRouteApi, Link, Navigate, useCanGoBack, useNavigate } from "@tanstack/react-router";
import { createContext, type ReactNode, useContext, useEffect, useRef, useState } from "react";

import type { IconName } from "../icon.tsx";
import { Icon } from "../icon.tsx";
import { t } from "../i18n.ts";
import { Wordmark } from "../logo.tsx";
import { usePrefs, useWidthClass } from "../prefs.ts";
import { useSpatialNavigation } from "../shell/spatial.ts";

const signInRoute = getRouteApi("/sign-in");
const setupRoute = getRouteApi("/setup");

/** The steps of signing in to a standalone server after the password. */
export const signInSteps = ["totp", "quick"] as const;

/** The search of /sign-in. */
export interface SignInSearch {
  step?: (typeof signInSteps)[number];
}

/** The steps of setting a new server up. */
export const setupSteps = ["mode", "admin"] as const;

/** The search of /setup. */
export interface SetupSearch {
  step?: (typeof setupSteps)[number];
}

type Layout = "card" | "phone" | "tv";

/** The sizes of each layout, after the sign-in boards of libs/design. */
const looks = {
  card: {
    stack: "gap-[22px]",
    title: "text-[28px] leading-[34px] font-bold tracking-[-0.01em]",
    subtitle: "text-[14px] leading-5 text-text-tertiary",
    body: "text-[15px] leading-[22px] text-text-secondary",
    back: "text-[15px]",
    button: "min-h-12 rounded-[12px] px-5 text-[16px] font-semibold",
    label: "text-[13px] font-semibold text-text-secondary",
    field: "h-11 rounded-[10px] px-3.5 text-[15px]",
    section: "text-[13px] font-semibold text-text-tertiary",
    group: "rounded-[12px]",
    rowTitle: "text-[15px] font-semibold",
    digit: "h-[60px] w-12 rounded-[12px] text-[30px]",
    icon: 18,
  },
  phone: {
    stack: "gap-[22px]",
    title: "text-[30px] leading-9 font-bold tracking-[-0.01em]",
    subtitle: "text-[15px] leading-5 text-text-tertiary",
    body: "text-[16px] leading-[22px] text-text-secondary",
    back: "text-[17px]",
    button: "min-h-[50px] rounded-[14px] px-5 text-[17px] font-semibold",
    label: "text-[16px] text-on-background",
    field: "h-12 px-4 text-[17px]",
    section: "px-4 text-[13px] font-semibold text-text-tertiary uppercase",
    group: "rounded-[12px]",
    rowTitle: "text-[17px] font-semibold",
    digit: "h-[52px] w-10 rounded-[10px] text-[26px]",
    icon: 20,
  },
  tv: {
    stack: "gap-8",
    title: "text-[54px] leading-[62px] font-bold tracking-[-0.01em]",
    subtitle: "text-[26px] leading-8 text-text-tertiary",
    body: "text-[26px] leading-9 text-text-secondary",
    back: "text-[26px]",
    button: "min-h-[84px] rounded-[22px] px-9 text-[28px] font-semibold transition-transform focus:scale-[1.04]",
    label: "text-[22px] font-semibold text-text-secondary",
    field: "h-[84px] rounded-[22px] px-7 text-[28px]",
    section: "text-[22px] font-semibold text-text-tertiary",
    group: "",
    rowTitle: "text-[30px] font-semibold",
    digit: "h-[110px] w-[88px] rounded-[22px] text-[56px]",
    icon: 30,
  },
} satisfies Record<Layout, Record<string, string | number>>;

type Look = (typeof looks)[Layout] & { layout: Layout };

const LookContext = createContext<Look>({ ...looks.card, layout: "card" });
const useLook = () => useContext(LookContext);

/** The look of this window: a television's, a phone's or a card's. */
function useLayoutLook(): Look {
  const { tv } = usePrefs();
  const width = useWidthClass();
  const layout: Layout = tv ? "tv" : width === "compact" ? "phone" : "card";
  return { ...looks[layout], layout };
}

/**
 * The frame of a step in this window's layout: on a phone, the wall of
 * posters above the first step of signing in, and the whole screen for
 * the others.
 */
function Screen({ wall = false, children }: { wall?: boolean; children: ReactNode }) {
  const look = useLayoutLook();
  let frame: ReactNode;
  if (look.layout === "tv") {
    frame = (
      <div className="relative flex h-dvh items-center justify-center overflow-hidden bg-background px-[120px] py-[90px]">
        <SpatialNavigation />
        <PosterWall layout="tv" />
        <Wordmark height={60} className="absolute top-[90px] left-[120px]" />
        <div className="relative max-h-full w-full max-w-[1040px] overflow-y-auto rounded-[44px] border border-(--signin-separator) bg-(--signin-card) px-[90px] py-20 shadow-(--signin-shadow) backdrop-blur-[50px] backdrop-saturate-150">
          {children}
        </div>
      </div>
    );
  } else if (look.layout === "phone") {
    frame = wall ? (
      <div className="relative flex min-h-dvh flex-col overflow-hidden bg-background">
        <PosterWall layout="phone" />
        <div className="relative mt-auto px-6 pb-14">{children}</div>
      </div>
    ) : (
      <div className="min-h-dvh bg-background px-5 pt-14 pb-10">{children}</div>
    );
  } else {
    frame = <Frame>{children}</Frame>;
  }
  return <LookContext value={look}>{frame}</LookContext>;
}

function SpatialNavigation() {
  useSpatialNavigation();
  return null;
}

/** Escape, and Backspace outside a field, go back a step, as a remote's Back does. */
function useBackKey(back: (() => void) | undefined) {
  const ref = useRef(back);
  ref.current = back;
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey || !ref.current) return;
      const typing = e.target instanceof Element && e.target.matches("input, textarea, select");
      if (e.key !== "Escape" && (e.key !== "Backspace" || typing)) return;
      e.preventDefault();
      ref.current();
    };
    document.addEventListener("keydown", key, true);
    return () => document.removeEventListener("keydown", key, true);
  }, []);
}

/** Runs the actions of a step one at a time, keeping the error of the last. */
function useRun() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
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
  return { busy, error, setError, run };
}

/** What the server serving the web UI tells before anyone signs in; nothing on app.mavio.dev. */
function useHomeInfo() {
  const servers = useServersStore();
  return useQuery({
    queryKey: ["authInfo", servers.home],
    queryFn: () => servers.probe(servers.home!),
    enabled: !servers.app,
    retry: false,
  });
}

/** Signing in (docs/accounts.md §6): the way the serving server signs people in, or app.mavio.dev's. */
export function SignInScreen() {
  const servers = useServersStore();
  const signIn = useSignIn();
  const { tv } = usePrefs();
  const home = useHomeInfo();
  if (signIn) return <Navigate to="/" replace />;
  if (servers.app) {
    return tv ? (
      <Screen>
        <LinkSignIn />
      </Screen>
    ) : (
      <Screen wall>
        <MavioSignIn />
      </Screen>
    );
  }
  const info = home.data;
  if (!info) {
    return (
      <Screen>
        <Pending error={home.error} onRetry={() => void home.refetch()} />
      </Screen>
    );
  }
  if (info.setupRequired) {
    return tv ? (
      <Screen>
        <SetUpElsewhere info={info} />
      </Screen>
    ) : (
      <Navigate to="/setup" replace />
    );
  }
  if (info.mode === "mavioDev") {
    return tv ? (
      <Screen>
        <LinkSignIn info={info} />
      </Screen>
    ) : (
      <Screen wall>
        <MavioSignIn info={info} />
      </Screen>
    );
  }
  return (
    <Screen>
      <StandaloneSignIn info={info} />
    </Screen>
  );
}

/** Waiting for the serving server to answer, or why it did not. */
function Pending({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const look = useLook();
  return (
    <div className={`flex flex-col items-center text-center ${look.stack}`}>
      <Wordmark height={look.layout === "tv" ? 48 : 32} />
      {error ? (
        <>
          <p className="text-[15px] text-danger">{errorMessage(error)}</p>
          <Action variant="secondary" onClick={onRetry}>
            {t("auth.tryAgain")}
          </Action>
        </>
      ) : (
        <p className={look.body}>{t("state.loading")}</p>
      )}
    </div>
  );
}

/**
 * Signing a Mavio account in at mavio.dev: the administrator of the
 * serving server, whose other accounts watch at app.mavio.dev, or any
 * account on app.mavio.dev.
 */
function MavioSignIn({ info }: { info?: ServerInfo }) {
  const servers = useServersStore();
  const look = useLook();
  const { busy, error, run } = useRun();
  const app = useQuery({ queryKey: ["appUrl", info?.id], queryFn: () => servers.appUrl(), enabled: !!info, staleTime: Infinity });
  const phone = look.layout === "phone";
  return (
    <div className={`flex flex-col ${phone ? "items-start gap-3" : "items-center gap-3 text-center"}`}>
      <Wordmark height={phone ? 40 : 36} className={phone ? "mb-1" : "mb-3"} />
      <h1 className={`mb-4 ${look.title}`}>{t("auth.welcome")}</h1>
      {error && <p className="text-[15px] text-danger">{error}</p>}
      <Action
        disabled={busy}
        onClick={() =>
          run(async () => {
            window.location.href = await servers.startSignIn("/");
          })
        }
      >
        {t("auth.signInWithMavio")}
      </Action>
      {app.data && (
        <p className={`mt-2 ${look.subtitle}`}>
          <a href={app.data} className="hover:underline">
            {t("auth.othersUseApp", { host: new URL(app.data).host })}
          </a>
        </p>
      )}
    </div>
  );
}

/**
 * A television's sign-in with a Mavio account: a code to approve at
 * mavio.dev/link, polled for until approved, and replaced as it expires.
 */
function LinkSignIn({ info }: { info?: ServerInfo }) {
  const servers = useServersStore();
  const look = useLook();
  const [link, setLink] = useState<LinkCode>();
  const [error, setError] = useState<string>();
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let stop = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const poll = (l: LinkCode) => {
      timer = setTimeout(async () => {
        if (stop) return;
        if (Date.now() > l.expires.getTime()) return void start();
        try {
          // Signed in, the sign-in screen leaves for Home.
          if (await servers.pollLink(l)) return;
        } catch (err) {
          if (!stop) setError(errorMessage(err));
          return;
        }
        poll(l);
      }, l.interval);
    };
    const start = async () => {
      setError(undefined);
      try {
        const l = await servers.startLink();
        if (stop) return;
        setLink(l);
        poll(l);
      } catch (err) {
        if (!stop) setError(errorMessage(err));
      }
    };
    void start();
    return () => {
      stop = true;
      clearTimeout(timer);
    };
  }, [servers, attempt]);
  const where = link && link.verificationUri.replace(/^https?:\/\//, "");
  return (
    <Steps title={t("auth.signInWithMavio")} subtitle={info?.name}>
      {link && <p className={look.body}>{t("auth.linkMessage", { url: where! })}</p>}
      {link && <Code code={link.userCode} />}
      {error ? (
        <>
          <p className="text-[15px] text-danger">{error}</p>
          <Action variant="secondary" onClick={() => setAttempt(attempt + 1)}>
            {t("auth.tryAgain")}
          </Action>
        </>
      ) : (
        <Waiting />
      )}
    </Steps>
  );
}

/** A code to enter on another device, a character to a box. */
function Code({ code }: { code: string }) {
  const look = useLook();
  return (
    <div className="flex items-center justify-center gap-2.5" aria-label={code}>
      {[...code].map((c, i) =>
        c === "-" ? (
          <span key={i} className="w-3 text-center text-text-tertiary">
            –
          </span>
        ) : (
          <span key={i} className={`flex items-center justify-center bg-fill font-mono font-semibold ${look.digit}`}>
            {c}
          </span>
        ),
      )}
    </div>
  );
}

function Waiting() {
  const look = useLook();
  return (
    <p className={`flex items-center justify-center gap-2 ${look.body}`}>
      <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-text-tertiary border-t-transparent" />
      {t("auth.waiting")}
    </p>
  );
}

/** A television's screen of a server not set up yet, which is set up in a browser. */
function SetUpElsewhere({ info }: { info: ServerInfo }) {
  const look = useLook();
  return (
    <Steps eyebrow={t("auth.newServerSetup")} title={t("auth.setUpElsewhere", { server: info.name })}>
      <p className={look.body}>{t("auth.setUpElsewhereMessage")}</p>
      <p className={`font-mono font-semibold [overflow-wrap:anywhere] ${look.title}`}>{info.url.replace(/^https?:\/\//, "")}</p>
    </Steps>
  );
}

/**
 * Signing in to a standalone server: a name and password, then the code of
 * the user's authenticator app when they have one; a passkey; or Quick
 * Connect. The password waits in memory for the code's step, which a
 * reload sends back to the first.
 */
function StandaloneSignIn({ info }: { info: ServerInfo }) {
  const servers = useServersStore();
  const navigate = useNavigate();
  const canGoBack = useCanGoBack();
  const { step } = signInRoute.useSearch();
  const { busy, error, setError, run } = useRun();
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const look = useLook();
  const first = step === undefined || (step === "totp" && !name);
  const back = first ? undefined : () => (canGoBack ? history.back() : void navigate({ to: "/sign-in", replace: true }));
  useBackKey(back);
  useEffect(() => setError(undefined), [step]);

  const signIn = (totp?: string) =>
    run(async () => {
      try {
        await servers.signInWithPassword(name.trim(), password, totp);
      } catch (err) {
        if (!totp && isTotpRequired(err)) {
          setCode("");
          void navigate({ to: "/sign-in", search: { step: "totp" } });
          return;
        }
        throw isUnauthenticated(err) ? new Error(t(totp ? "auth.wrongCode" : "auth.wrongPassword")) : err;
      }
    });

  const subtitle = `${t("auth.standaloneServer")} · ${info.url}`;
  if (step === "quick") return <QuickConnectSignIn info={info} onBack={back} />;
  if (step === "totp" && name) {
    return (
      <Steps title={t("auth.totpCode")} subtitle={subtitle} onBack={back}>
        <form
          className={`flex flex-col ${look.stack}`}
          onSubmit={(e) => {
            e.preventDefault();
            void signIn(code);
          }}
        >
          <p className={look.body}>{t("auth.totpMessage")}</p>
          <Fields
            fields={[
              {
                label: t("auth.totpCode"),
                value: code,
                onChange: (e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6)),
                autoComplete: "one-time-code",
                inputMode: "numeric",
                autoFocus: true,
                required: true,
              },
            ]}
          />
          {error && <p className="text-[15px] text-danger">{error}</p>}
          <Action type="submit" disabled={busy || code.length !== 6}>
            {t("auth.signIn")}
          </Action>
        </form>
      </Steps>
    );
  }
  const passkeys = passkeysHere(info.url);
  return (
    <Steps title={t("auth.signInTo", { server: info.name })} subtitle={subtitle}>
      <form
        className={`flex flex-col ${look.stack}`}
        onSubmit={(e) => {
          e.preventDefault();
          void signIn();
        }}
      >
        <Credentials name={name} password={password} setName={setName} setPassword={setPassword} />
        {error && <p className="text-[15px] text-danger">{error}</p>}
        <Action type="submit" disabled={busy || !name.trim()}>
          {t("auth.signIn")}
        </Action>
        <Or />
        <div className={`grid gap-2.5 ${passkeys ? "grid-cols-2" : "grid-cols-1"}`}>
          {passkeys && (
            <Action variant="secondary" icon="key" onClick={() => run(async () => void (await servers.signInWithPasskey()))} disabled={busy}>
              {t("auth.passkey")}
            </Action>
          )}
          <Action variant="secondary" icon="devices" onClick={() => void navigate({ to: "/sign-in", search: { step: "quick" } })}>
            {t("auth.quickConnect")}
          </Action>
        </div>
      </form>
    </Steps>
  );
}

/** A name and a password. */
function Credentials({
  name,
  password,
  setName,
  setPassword,
  creating = false,
}: {
  name: string;
  password: string;
  setName: (s: string) => void;
  setPassword: (s: string) => void;
  creating?: boolean;
}) {
  const look = useLook();
  return (
    <Fields
      fields={[
        {
          label: t("auth.username"),
          value: name,
          onChange: (e) => setName(e.target.value),
          autoComplete: "username",
          autoCapitalize: "off",
          autoFocus: look.layout !== "tv",
          required: true,
        },
        {
          label: t("auth.password"),
          type: "password",
          value: password,
          onChange: (e) => setPassword(e.target.value),
          autoComplete: creating ? "new-password" : "current-password",
          required: creating,
        },
      ]}
    />
  );
}

/** Quick Connect: a code entered on a device signed in to the server, waited for until it is. */
function QuickConnectSignIn({ info, onBack }: { info: ServerInfo; onBack?: () => void }) {
  const servers = useServersStore();
  const look = useLook();
  const [qc, setQc] = useState<QuickConnect>();
  const [error, setError] = useState<string>();
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let stop = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const poll = (q: QuickConnect) => {
      timer = setTimeout(async () => {
        if (stop) return;
        if (Date.now() > q.expires.getTime()) return void start();
        try {
          // Signed in, the sign-in screen leaves for Home.
          if (await servers.pollQuickConnect(q)) return;
        } catch (err) {
          if (!stop) setError(errorMessage(err));
          return;
        }
        poll(q);
      }, 3000);
    };
    const start = async () => {
      setError(undefined);
      try {
        const q = await servers.startQuickConnect();
        if (stop) return;
        setQc(q);
        poll(q);
      } catch (err) {
        if (!stop) setError(errorMessage(err));
      }
    };
    void start();
    return () => {
      stop = true;
      clearTimeout(timer);
    };
  }, [servers, attempt]);
  return (
    <Steps title={t("auth.quickConnectTitle")} subtitle={info.name} onBack={onBack}>
      {qc && <p className={look.body}>{t("auth.quickConnectCode", { code: qc.code })}</p>}
      {qc && <Code code={qc.code} />}
      {error ? (
        <>
          <p className="text-[15px] text-danger">{error}</p>
          <Action variant="secondary" onClick={() => setAttempt(attempt + 1)}>
            {t("auth.tryAgain")}
          </Action>
        </>
      ) : (
        <Waiting />
      )}
    </Steps>
  );
}

/**
 * Setting the serving server up (docs/accounts.md §2, §3.2): choosing its
 * mode, then claiming it at mavio.dev or creating its administrator.
 */
export function SetupScreen() {
  const servers = useServersStore();
  const signIn = useSignIn();
  const { tv } = usePrefs();
  const home = useHomeInfo();
  if (servers.app || signIn) return <Navigate to="/" replace />;
  const info = home.data;
  if (!info) {
    return (
      <Screen>
        <Pending error={home.error} onRetry={() => void home.refetch()} />
      </Screen>
    );
  }
  if (!info.setupRequired) return <Navigate to="/sign-in" replace />;
  return <Screen>{tv ? <SetUpElsewhere info={info} /> : <Setup info={info} />}</Screen>;
}

function Setup({ info }: { info: ServerInfo }) {
  const servers = useServersStore();
  const navigate = useNavigate();
  const canGoBack = useCanGoBack();
  const { step } = setupRoute.useSearch();
  const { busy, error, setError, run } = useRun();
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const look = useLook();
  // A mode the server was started with skips the choice.
  const admin = info.mode === "standalone" || (!info.mode && step === "admin");
  const back =
    admin && !info.mode ? () => (canGoBack ? history.back() : void navigate({ to: "/setup", search: { step: "mode" }, replace: true })) : undefined;
  useBackKey(back);
  useEffect(() => setError(undefined), [step]);

  const claim = () =>
    run(async () => {
      window.location.href = await servers.startClaim("/");
    });

  if (admin) {
    return (
      <Steps
        eyebrow={t("auth.newServerSetup")}
        title={t("auth.setUp", { server: info.name })}
        subtitle={`${t("auth.standaloneServer")} · ${info.url}`}
        onBack={back}
      >
        <form
          className={`flex flex-col ${look.stack}`}
          onSubmit={(e) => {
            e.preventDefault();
            void run(async () => void (await servers.createFirstUser(name.trim(), password)));
          }}
        >
          <p className={look.body}>{t("auth.setUpMessage")}</p>
          <Credentials name={name} password={password} setName={setName} setPassword={setPassword} creating />
          {error && <p className="text-[15px] text-danger">{error}</p>}
          <Action type="submit" disabled={busy || !name.trim()}>
            {t("auth.createAccount")}
          </Action>
        </form>
      </Steps>
    );
  }
  if (info.mode === "mavioDev") {
    return (
      <Steps eyebrow={t("auth.newServerSetup")} title={t("auth.claim", { server: info.name })} subtitle={info.url}>
        <p className={look.body}>{t("auth.claimMessage")}</p>
        {error && <p className="text-[15px] text-danger">{error}</p>}
        <Action disabled={busy} onClick={claim}>
          {t("auth.claimServer")}
        </Action>
      </Steps>
    );
  }
  return (
    <ChooseMode
      info={info}
      busy={busy}
      error={error}
      onChoose={(mode) => (mode === "mavioDev" ? void claim() : void navigate({ to: "/setup", search: { step: "admin" } }))}
    />
  );
}

/** Where mavio.dev comes back to with the code of a sign-in or a claim. */
export function CallbackScreen() {
  const servers = useServersStore();
  const navigate = useNavigate();
  const [error, setError] = useState<string>();
  useEffect(() => {
    servers.finishCallback(new URLSearchParams(location.search)).then(
      (to) => void navigate({ to, replace: true }),
      (err: unknown) => setError(errorMessage(err)),
    );
    // Once: the code is spent.
  }, []);
  return (
    <Screen>
      <div className="flex flex-col items-center gap-4 text-center">
        <Wordmark height={32} />
        {error ? (
          <>
            <p className="text-[15px] text-danger">{error}</p>
            <Link to="/sign-in" replace className="text-[15px] font-semibold text-accent-text hover:underline">
              {t("auth.tryAgain")}
            </Link>
          </>
        ) : (
          <p className="text-[15px] text-text-secondary">{t("auth.signingIn")}</p>
        )}
      </div>
    </Screen>
  );
}

/** The card of a wide window, on the wall of posters. */
function Frame({ children }: { children: ReactNode }) {
  return (
    <div className="relative flex min-h-dvh items-center justify-center overflow-hidden bg-background p-margin">
      <PosterWall layout="card" />
      <div className="relative w-full max-w-[460px] rounded-[26px] border border-(--signin-separator) bg-(--signin-card) px-10 pt-10 pb-[34px] shadow-(--signin-shadow) backdrop-blur-[40px] backdrop-saturate-150">
        {children}
      </div>
    </div>
  );
}

/** The colors of the posters, top to bottom: warm, cool and dusk, as films are. */
const tones = [
  ["#FF9F0A", "#6B2400"],
  ["#2A6F97", "#0A1A2E"],
  ["#8E5CF7", "#24104D"],
  ["#F0412F", "#3A0A0A"],
  ["#30B07A", "#0A2E20"],
  ["#FFD34E", "#7A3E00"],
  ["#5AC8FA", "#10325A"],
  ["#FF6482", "#420D26"],
  ["#C7A17A", "#33241A"],
  ["#64D2FF", "#1A2A66"],
  ["#E07A5F", "#2E1A3A"],
];

/**
 * The screen behind sign-in: a wall of posters drawn from gradients,
 * tilted and blurred under a veil, darker or lighter with the theme.
 */
function PosterWall({ layout }: { layout: Layout }) {
  const { columns, blur, rotate } = { card: { columns: 8, blur: 2, rotate: -8 }, phone: { columns: 6, blur: 0, rotate: -8 }, tv: { columns: 10, blur: 30, rotate: -6 } }[
    layout
  ];
  const phone = layout === "phone";
  return (
    <div aria-hidden className={`pointer-events-none absolute inset-x-0 top-0 overflow-hidden ${phone ? "h-[68%]" : "bottom-0"}`}>
      <div
        className="absolute -inset-[25%] flex gap-[1%]"
        style={{ transform: `rotate(${rotate}deg) scale(1.15)`, filter: blur ? `blur(${blur}px)` : undefined, opacity: "var(--signin-wall)" }}
      >
        {Array.from({ length: columns }, (_, c) => (
          <div key={c} className="poster-column flex flex-1 flex-col gap-[1%]" style={{ marginTop: c % 2 ? "-6%" : "0" }}>
            {Array.from({ length: 9 }, (_, r) => {
              const [a, b] = tones[(c * 4 + r * 7) % tones.length];
              return (
                <div
                  key={r}
                  className="aspect-[2/3] w-full shrink-0 rounded-[8px]"
                  style={{
                    background: `radial-gradient(120% 70% at 25% 12%, rgb(255 255 255 / 0.28), transparent 55%), linear-gradient(165deg, ${a}, ${b})`,
                  }}
                />
              );
            })}
          </div>
        ))}
      </div>
      <div
        className="absolute inset-0"
        style={{ background: phone ? "linear-gradient(to bottom, transparent 25%, var(--mavio-color-background) 96%)" : "var(--signin-veil)" }}
      />
    </div>
  );
}

/** A step's back link, title and the line under it, above its content. */
function Steps({
  title,
  eyebrow,
  subtitle,
  onBack,
  children,
}: {
  title: string;
  eyebrow?: string;
  subtitle?: string;
  onBack?: () => void;
  children: ReactNode;
}) {
  const look = useLook();
  return (
    <div className={`flex flex-col ${look.stack}`}>
      {onBack && look.layout !== "tv" && (
        <button
          type="button"
          onClick={onBack}
          className={`-ml-1 inline-flex items-center gap-0.5 self-start text-accent-text hover:opacity-80 ${look.back}`}
        >
          <Icon name="back" size={look.layout === "phone" ? 22 : 18} />
          {t("action.back")}
        </button>
      )}
      <div className="flex flex-col gap-1.5">
        {eyebrow && <p className={`${look.section} px-0 normal-case`}>{eyebrow}</p>}
        <h1 className={`[overflow-wrap:anywhere] ${look.title}`}>{title}</h1>
        {subtitle && <p className={`truncate ${look.subtitle}`}>{subtitle}</p>}
      </div>
      {children}
    </div>
  );
}

/** A line between ways of going on, broken by "or". */
function Or() {
  return (
    <div className="flex w-full items-center gap-3 text-[13px] text-text-tertiary">
      <span className="h-px flex-1 bg-(--signin-separator)" />
      {t("auth.or")}
      <span className="h-px flex-1 bg-(--signin-separator)" />
    </div>
  );
}

/** A button of sign-in, as wide as its step. */
function Action({
  variant = "primary",
  icon,
  className,
  children,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "secondary"; icon?: IconName }) {
  const look = useLook();
  const styles = variant === "primary" ? "bg-button text-on-button hover:opacity-90" : "bg-fill text-on-background hover:bg-fill-selected";
  return (
    <button
      type="button"
      {...props}
      className={`inline-flex w-full items-center justify-center gap-2 transition active:opacity-80 disabled:opacity-50 ${look.button} ${styles} ${className ?? ""}`}
    >
      {icon && <Icon name={icon} size={look.icon} />}
      {children}
    </button>
  );
}

type FieldProps = React.InputHTMLAttributes<HTMLInputElement> & { label: string };

/**
 * Text fields: labeled above in a card or on a television, and as the
 * rows of an inset group on a phone.
 */
function Fields({ fields }: { fields: FieldProps[] }) {
  const look = useLook();
  if (look.layout === "phone") {
    return (
      <div className={`flex flex-col divide-y divide-(--signin-separator) overflow-hidden bg-(--signin-group) ${look.group}`}>
        {fields.map(({ label, className, ...props }) => (
          <label key={label} className="flex items-center">
            <span className={`w-[104px] shrink-0 pl-4 ${look.label}`}>{label}</span>
            <input
              {...props}
              className={`min-w-0 flex-1 bg-transparent text-on-background placeholder:text-text-tertiary focus:outline-none focus-visible:outline-none ${look.field} pl-0 ${className ?? ""}`}
            />
          </label>
        ))}
      </div>
    );
  }
  return (
    <>
      {fields.map(({ label, className, ...props }) => (
        <label key={label} className="flex flex-col gap-2">
          <span className={look.label}>{label}</span>
          <input
            {...props}
            className={`border border-(--signin-separator) bg-(--signin-field) text-on-background placeholder:text-text-tertiary focus:border-accent focus:ring-1 focus:ring-accent focus:outline-none focus-visible:outline-none ${look.field} ${className ?? ""}`}
          />
        </label>
      ))}
    </>
  );
}

/** A new server's choice of mode (docs/accounts.md §2). */
function ChooseMode({
  info,
  busy,
  error,
  onChoose,
}: {
  info: ServerInfo;
  busy: boolean;
  error?: string;
  onChoose: (mode: "standalone" | "mavioDev") => void;
}) {
  const look = useLook();
  const phone = look.layout === "phone";
  const option = (m: "standalone" | "mavioDev", title: string, message: string) => (
    <div className="flex w-full flex-col gap-2">
      <Action variant={m === "mavioDev" ? "primary" : "secondary"} disabled={busy} onClick={() => onChoose(m)}>
        {title}
      </Action>
      <p className={look.subtitle}>{message}</p>
    </div>
  );
  return (
    <div className={`flex flex-col ${phone ? "items-start gap-3" : "items-center gap-3 text-center"}`}>
      <Wordmark height={phone ? 40 : 36} className={phone ? "mb-1" : "mb-3"} />
      <h1 className={look.title}>{t("auth.welcome")}</h1>
      <div className="mb-4 flex w-full flex-col gap-0.5">
        <p className={`[overflow-wrap:anywhere] ${look.body}`}>{t("auth.setUpNewServer", { server: info.name })}</p>
        <p className={`truncate ${look.subtitle}`}>{info.url}</p>
      </div>
      {error && <p className="text-[15px] text-danger">{error}</p>}
      <div className={`flex w-full flex-col ${look.stack}`}>
        {option("mavioDev", t("auth.modeMavioDev"), t("auth.modeMavioDevMessage"))}
        <Or />
        {option("standalone", t("auth.modeStandalone"), t("auth.modeStandaloneMessage"))}
      </div>
    </div>
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

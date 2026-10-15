// Sign-in (docs/clients.md §4.1, docs/accounts.md §6): with a Mavio account
// through mavio.dev, the last one used here offered first, or to a server
// by its address, those signed in to before listed first; on a server, a
// password with the code of the user's authenticator app when it has one,
// a passkey or Quick Connect. A new server's setup chooses its mode:
// claiming it for a Mavio account, or creating its first user. A server
// can also be added to a kept sign-in, a Mavio account's among them.
//
// Each step is in the address, so that the browser's Back, Escape and a
// remote's Back go to the step before. A card on a wall of posters on a
// desktop, the whole screen on a phone, and tabs on the left on a
// television.
import {
  errorMessage,
  isTotpRequired,
  isUnauthenticated,
  normalizeUrl,
  type QuickConnect,
  type ServerEntry,
  type ServerInfo,
  signInKey,
  useServers,
  useServersStore,
  useSignIns,
} from "@mavio/client-ts";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { getRouteApi, Link, useCanGoBack, useNavigate } from "@tanstack/react-router";
import { createContext, type ReactNode, useContext, useEffect, useRef, useState } from "react";

import type { IconName } from "../icon.tsx";
import { Icon } from "../icon.tsx";
import { t } from "../i18n.ts";
import { Wordmark } from "../logo.tsx";
import { usePrefs, useWidthClass } from "../prefs.ts";
import { useSpatialNavigation } from "../shell/spatial.ts";
import { Avatar } from "../ui.tsx";

const route = getRouteApi("/sign-in");

/** The steps of signing in, each with its own address. */
export const steps = ["welcome", "mavio", "servers", "new", "server"] as const;
export type Step = (typeof steps)[number];

/** The search of /sign-in. */
export interface SignInSearch {
  step?: Step;
  /** The address of the server signed in to at the server step. */
  server?: string;
  /** Adds a sign-in to those kept, from settings. */
  add?: boolean;
  /** The key of the kept sign-in a server is added to. */
  to?: string;
  /** How many steps were taken here, which Back retraces. */
  d?: number;
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
    row: "min-h-14 gap-3 px-3.5",
    rowTitle: "text-[15px] font-semibold",
    rowDetail: "text-[12px] text-text-tertiary",
    avatar: 32,
    account: "gap-3.5 rounded-[14px] p-3.5",
    accountAvatar: 44,
    accountName: "text-[16px] font-semibold",
    accountDetail: "text-[13px] text-text-tertiary",
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
    row: "min-h-[60px] gap-3 px-4",
    rowTitle: "text-[17px] font-semibold",
    rowDetail: "text-[13px] text-text-tertiary",
    avatar: 36,
    account: "gap-3.5 rounded-[14px] p-4",
    accountAvatar: 48,
    accountName: "text-[17px] font-semibold",
    accountDetail: "text-[14px] text-text-tertiary",
    digit: "h-[60px] w-12 rounded-[12px] text-[30px]",
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
    row: "min-h-[104px] gap-6 rounded-[22px] bg-fill px-8 transition-transform focus:scale-[1.04]",
    rowTitle: "text-[30px] font-semibold",
    rowDetail: "text-[22px] opacity-60",
    avatar: 56,
    account: "gap-6 rounded-[22px] bg-fill px-8 py-6 transition-transform focus:scale-[1.04]",
    accountAvatar: 76,
    accountName: "text-[32px] font-semibold",
    accountDetail: "text-[22px] opacity-60",
    digit: "h-[110px] w-[88px] rounded-[22px] text-[56px]",
    icon: 30,
  },
} satisfies Record<Layout, Record<string, string | number>>;

type Look = (typeof looks)[Layout] & { layout: Layout };

const LookContext = createContext<Look>({ ...looks.card, layout: "card" });
const useLook = () => useContext(LookContext);

export function SignInScreen() {
  const servers = useServersStore();
  const list = useServers();
  const { all } = useSignIns();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const canGoBack = useCanGoBack();
  const search = route.useSearch();
  const { tv } = usePrefs();
  const width = useWidthClass();
  const layout: Layout = tv ? "tv" : width === "compact" ? "phone" : "card";
  const look: Look = { ...looks[layout], layout };
  const d = search.d ?? 0;
  // Fixed on arrival: signing in must not turn the first sign-in into adding.
  const [adding] = useState(() => !!search.add || !!search.to || servers.signedIn().length > 0);
  const [address, setAddress] = useState(list.some((s) => s.url === servers.home) ? "" : servers.home);
  const [qc, setQc] = useState<QuickConnect>();
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const [needsCode, setNeedsCode] = useState(false);
  // The mode a new server's setup chose.
  const [setupMode, setSetupMode] = useState<"standalone" | "mavioDev">();

  // The serving server: a new one is set up first; one in mavio.dev mode
  // signs Mavio accounts in.
  const home = useQuery({ queryKey: ["authInfo", servers.home], queryFn: () => servers.probe(servers.home), retry: false });
  const lastAccount = list.find((s) => s.mode === "mavioDev" && s.account);
  // The server that signs a Mavio account in: the serving one, else the last one's.
  const mavioServer = home.data?.mode === "mavioDev" ? home.data.url : lastAccount?.url;
  const target = search.to ? all.find((x) => x.key === search.to) : undefined;
  // The servers signed in to before, but those already in the sign-in added to.
  const known = list.filter((s) => s.mode !== "mavioDev" && !(search.to && s.token && signInKey(s) === search.to));

  let step: Step = search.step ?? (search.server ? "server" : search.to ? "servers" : tv ? "mavio" : "welcome");
  // A television starts on the Mavio tab only where a server signs Mavio accounts in.
  if (tv && !search.step && step === "mavio" && home.isFetched && !mavioServer) step = "servers";
  if (search.to && (step === "welcome" || step === "mavio")) step = "servers";
  // With no server signed in to before, connecting starts at the address.
  if (step === "servers" && known.length === 0) step = "new";

  let url: string | undefined;
  try {
    url = search.server ? normalizeUrl(search.server) : undefined;
  } catch {
    url = undefined;
  }
  const probe = useQuery({
    queryKey: ["authInfo", url],
    queryFn: () => servers.probe(url!),
    enabled: step === "server" && !!url,
    retry: false,
  });
  const info = step === "server" ? probe.data : undefined;
  // The serving server's first setup, which has no step before it.
  const firstRun = d === 0 && !adding && !!info?.setupRequired && info.url === home.data?.url;

  // A new serving server is set up at once.
  useEffect(() => {
    if (home.data?.setupRequired && !adding && !search.server && !search.step) {
      void navigate({ to: "/sign-in", search: { step: "server", server: home.data.url }, replace: true });
    }
  }, [home.data]);

  // A step starts afresh.
  useEffect(() => {
    setError(undefined);
    setNeedsCode(false);
    setSetupMode(undefined);
    setQc(undefined);
  }, [step, url]);

  const keep = { add: search.add, to: search.to };
  /** Goes on to a step, which Back returns from. */
  const go = (to: Step, server?: string) =>
    void navigate({ to: "/sign-in", search: { ...keep, step: to, server, d: d + 1 } });
  /** Leaves sign-in for where it was opened from. */
  const leave = () => (canGoBack ? history.back() : void navigate({ to: adding ? "/settings" : "/", replace: true }));

  // The step before this one, or leaving; none at the first step of a first sign-in.
  const leaving = adding ? ("leave" as const) : undefined;
  let parent: Step | "leave" | undefined;
  switch (step) {
    case "welcome":
      parent = leaving;
      break;
    case "mavio":
      parent = tv ? leaving : "welcome";
      break;
    case "servers":
      parent = search.to || tv ? leaving : "welcome";
      break;
    case "new":
      parent = known.length ? "servers" : search.to || tv ? leaving : "welcome";
      break;
    case "server":
      if (firstRun) parent = undefined;
      else if (d === 0 && adding && search.server && !search.step) parent = "leave";
      else parent = known.some((s) => s.url === url) ? "servers" : "new";
  }
  const inner = !!qc || !!setupMode;
  const back =
    inner || parent
      ? () => {
          if (qc) return setQc(undefined);
          if (setupMode) {
            setSetupMode(undefined);
            setError(undefined);
            return;
          }
          if (d > 0) return history.back();
          if (parent === "leave") return leave();
          if (parent) void navigate({ to: "/sign-in", search: { ...keep, step: parent }, replace: true });
        }
      : undefined;

  // Escape, and Backspace outside a field, go back a step before the
  // television's going back leaves the screen.
  const backRef = useRef(back);
  backRef.current = back;
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey) return;
      const typing = e.target instanceof Element && e.target.matches("input, textarea, select");
      if (e.key !== "Escape" && (e.key !== "Backspace" || typing)) return;
      e.preventDefault();
      backRef.current?.();
    };
    document.addEventListener("keydown", key, true);
    return () => document.removeEventListener("keydown", key, true);
  }, []);

  const done = (entry?: ServerEntry) => {
    if (entry && search.to) servers.addTo(entry.id, search.to);
    void navigate({ to: adding && (search.to || search.server) ? "/settings" : "/", replace: true });
  };

  // Goes to mavio.dev, which comes back to /sign-in/mavio-dev.
  const toMavioDev = async (u: string) => {
    window.location.href = await servers.startMavioDevSignIn(u, `${location.origin}/sign-in/mavio-dev`);
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

  const connect = (a: string) =>
    run(async () => {
      let i: ServerInfo;
      try {
        i = await servers.probe(a);
      } catch (err) {
        throw /not a Mavio server/.test(String(err)) ? new Error(t("auth.notMavio")) : err;
      }
      queryClient.setQueryData(["authInfo", i.url], i);
      go("server", i.url);
    });

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
        const entry = await servers.pollQuickConnect(qc);
        if (entry) {
          done(entry);
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

  /** Opens a server signed in to before: shows it when still signed in, else signs in again. */
  const openServer = (s: ServerEntry) => {
    if (!s.token) return go("server", s.url);
    if (search.to) servers.addTo(s.id, search.to);
    else servers.show(signInKey(s));
    done();
  };

  const errorLine = error && <p className="text-[15px] text-danger">{error}</p>;
  const addTo = target && t("auth.addToMessage", { name: target.name });

  let content: ReactNode;
  if (step === "welcome") {
    content = (
      <Welcome
        mavio={!!mavioServer || home.isPending}
        mavioReady={!!mavioServer}
        onMavio={() => go("mavio")}
        onServers={() => go("servers")}
      />
    );
  } else if (step === "mavio") {
    content = (
      <Steps
        title={t("auth.signInWithMavio")}
        subtitle={lastAccount ? t("auth.lastAccount") : undefined}
        onBack={tv ? undefined : back}
      >
        {lastAccount && (
          <AccountCard entry={lastAccount} disabled={busy} onClick={() => run(() => toMavioDev(lastAccount.url))} />
        )}
        {!mavioServer && !home.isPending && <p className={look.body}>{t("auth.noMavioServer")}</p>}
        {errorLine}
        <Action
          variant={lastAccount ? "secondary" : "primary"}
          disabled={busy || !mavioServer}
          onClick={() => mavioServer && run(() => toMavioDev(mavioServer))}
        >
          {lastAccount ? t("auth.otherAccount") : t("auth.signInWithMavio")}
        </Action>
      </Steps>
    );
  } else if (step === "servers") {
    content = (
      <Steps
        title={target ? t("auth.addServer") : t("auth.connectToServer")}
        subtitle={addTo}
        onBack={tv && !search.to ? undefined : back}
      >
        <div className="flex flex-col gap-2">
          <p className={look.section}>{t("auth.recentServers")}</p>
          <Rows>
            {known.map((s) => (
              <Row
                key={s.id}
                name={s.name}
                detail={s.token ? t("auth.signedInAs", { name: s.userName }) : `${s.userName} · ${t("auth.signedOut")}`}
                disabled={busy}
                onClick={() => openServer(s)}
              />
            ))}
          </Rows>
        </div>
        {errorLine}
        <Action variant="secondary" icon="add" onClick={() => go("new")}>
          {t("auth.newServer")}
        </Action>
      </Steps>
    );
  } else if (step === "new") {
    content = (
      <Steps title={t("auth.newServer")} subtitle={addTo} onBack={tv && !parent ? undefined : back}>
        <form
          className={`flex flex-col ${look.stack}`}
          onSubmit={(e) => {
            e.preventDefault();
            void connect(address);
          }}
        >
          <Fields
            fields={[
              {
                label: t("auth.server"),
                value: address,
                onChange: (e) => setAddress(e.target.value),
                placeholder: t("auth.serverAddress"),
                autoFocus: !tv,
                required: true,
                inputMode: "url",
                autoCapitalize: "off",
                autoCorrect: "off",
              },
            ]}
          />
          {errorLine}
          <Action type="submit" disabled={busy || !address.trim()}>
            {t("auth.connect")}
          </Action>
        </form>
      </Steps>
    );
  } else if (!info) {
    content = (
      <Steps title={t("auth.connectToServer")} subtitle={url} onBack={back}>
        {probe.error || !url ? (
          <p className="text-[15px] text-danger">
            {!url || /not a Mavio server/.test(String(probe.error)) ? t("auth.notMavio") : errorMessage(probe.error)}
          </p>
        ) : (
          <p className={look.body}>{t("state.loading")}</p>
        )}
      </Steps>
    );
  } else if (qc) {
    content = (
      <Steps title={t("auth.quickConnectTitle")} subtitle={info.name} onBack={back}>
        <p className={look.body}>{t("auth.quickConnectCode", { code: qc.code })}</p>
        <div className="flex justify-center gap-2.5" aria-label={qc.code}>
          {[...qc.code].map((digit, i) => (
            <span key={i} className={`flex items-center justify-center bg-fill font-mono font-semibold ${look.digit}`}>
              {digit}
            </span>
          ))}
        </div>
        <p className={`flex items-center justify-center gap-2 ${look.body}`}>
          <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-text-tertiary border-t-transparent" />
          {t("auth.waiting")}
        </p>
      </Steps>
    );
  } else if (info.setupRequired && !info.mode && !setupMode) {
    content = <ChooseMode info={info} onBack={back} onChoose={setSetupMode} />;
  } else if (info.mode === "mavioDev" || setupMode === "mavioDev") {
    content = info.setupRequired ? (
      <Claim
        info={info}
        busy={busy}
        error={error}
        onBack={back}
        onClaim={(code) =>
          run(async () => {
            queryClient.setQueryData(["authInfo", info.url], await servers.claimServer(info.url, code));
            await toMavioDev(info.url);
          })
        }
      />
    ) : (
      <Steps title={info.name} subtitle={info.url} onBack={back}>
        <p className={look.body}>{t("auth.mavioDevMessage")}</p>
        {errorLine}
        <Action disabled={busy} onClick={() => run(() => toMavioDev(info.url))}>
          {t("auth.signInWithMavio")}
        </Action>
      </Steps>
    );
  } else {
    content = (
      <Credentials
        info={info}
        busy={busy}
        error={error}
        onBack={back}
        onSubmit={(name, password, code) =>
          run(async () => {
            let entry: ServerEntry;
            try {
              entry = info.setupRequired
                ? await servers.createFirstUser(info.url, name, password)
                : await servers.signIn(info.url, name, password, code);
            } catch (err) {
              if (!code && isTotpRequired(err)) {
                setNeedsCode(true);
                return;
              }
              throw isUnauthenticated(err) ? new Error(t(code ? "auth.wrongCode" : "auth.wrongPassword")) : err;
            }
            done(entry);
          })
        }
        needsCode={needsCode}
        onPasskey={() => run(async () => done(await servers.signInWithPasskey(info.url)))}
        onQuickConnect={() => run(async () => setQc(await servers.startQuickConnect(info.url)))}
      />
    );
  }

  if (layout === "tv") {
    const mavioTab = step === "mavio";
    const tabs: [Step, string, IconName, boolean][] = [
      ...(search.to ? [] : [["mavio", t("auth.signInWithMavio"), "profile", mavioTab] as [Step, string, IconName, boolean]]),
      ["servers", t("auth.connectToServer"), "server", !mavioTab],
    ];
    // Moving focus onto a tab shows it, in place of the step shown.
    const show = (s: Step) => void navigate({ to: "/sign-in", search: { ...keep, step: s, d: search.d }, replace: true });
    return (
      <LookContext value={look}>
        <SpatialNavigation />
        <div className="relative flex h-dvh overflow-hidden bg-background">
          <PosterWall layout="tv" />
          <div className="relative flex w-full gap-20 px-[120px] py-[90px]">
            <aside className="flex w-[520px] shrink-0 flex-col">
              <Wordmark height={60} className="mb-16" />
              <div role="tablist" aria-orientation="vertical" className="flex flex-col gap-4">
                {tabs.map(([s, label, icon, on]) => (
                  <button
                    key={s}
                    type="button"
                    role="tab"
                    aria-selected={on}
                    onFocus={() => on || show(s)}
                    onClick={() => on || show(s)}
                    className={`flex h-[92px] items-center gap-5 rounded-[22px] px-8 text-left text-[32px] font-semibold transition-transform focus:scale-[1.06] focus:shadow-[0_18px_40px_rgb(0_0_0/0.4)] ${on ? "bg-fill" : "text-text-secondary"}`}
                  >
                    <Icon name={icon} size={36} />
                    {label}
                  </button>
                ))}
              </div>
              <p className="mt-auto text-[20px] text-text-tertiary">{t("auth.tvHint")}</p>
            </aside>
            <div
              role="tabpanel"
              className="min-w-0 flex-1 self-center overflow-y-auto rounded-[44px] border border-(--signin-separator) bg-(--signin-card) px-[90px] py-20 shadow-(--signin-shadow) backdrop-blur-[50px] backdrop-saturate-150"
            >
              {content}
            </div>
          </div>
        </div>
      </LookContext>
    );
  }

  if (layout === "phone") {
    return (
      <LookContext value={look}>
        {step === "welcome" ? (
          <div className="relative flex min-h-dvh flex-col overflow-hidden bg-background">
            <PosterWall layout="phone" />
            {adding && <Close />}
            <div className="relative mt-auto px-6 pb-14">{content}</div>
          </div>
        ) : (
          <div className="min-h-dvh bg-background px-5 pt-14 pb-10">{content}</div>
        )}
      </LookContext>
    );
  }

  return (
    <LookContext value={look}>
      <Frame>
        {adding && step === "welcome" && <Close />}
        {content}
      </Frame>
    </LookContext>
  );
}

function SpatialNavigation() {
  useSpatialNavigation();
  return null;
}

/** Closes sign-in that was opened to add a sign-in. */
function Close() {
  return (
    <Link
      to="/settings"
      aria-label={t("action.close")}
      className="absolute top-4 right-4 z-10 rounded-full bg-fill p-1.5 text-text-secondary hover:bg-fill-selected"
    >
      <Icon name="close" size={18} />
    </Link>
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

/** The first step: the two ways of signing in. */
function Welcome({
  mavio,
  mavioReady,
  onMavio,
  onServers,
}: {
  mavio: boolean;
  mavioReady: boolean;
  onMavio: () => void;
  onServers: () => void;
}) {
  const look = useLook();
  const phone = look.layout === "phone";
  return (
    <div className={`flex flex-col ${phone ? "items-start gap-3" : "items-center gap-3 text-center"}`}>
      <Wordmark height={phone ? 40 : 36} className={phone ? "mb-1" : "mb-3"} />
      {!phone && <h1 className={look.title}>{t("auth.welcome")}</h1>}
      <p className={`mb-4 ${phone ? "text-[16px] leading-[22px] text-text-secondary" : look.body}`}>{t("auth.welcomeMessage")}</p>
      {mavio && (
        <Action disabled={!mavioReady} onClick={onMavio}>
          {t("auth.signInWithMavio")}
        </Action>
      )}
      <Action variant="secondary" onClick={onServers}>
        {t("auth.connectToServer")}
      </Action>
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

/** Rows grouped together, or apart on a television. */
function Rows({ children }: { children: ReactNode }) {
  const look = useLook();
  return look.layout === "tv" ? (
    <div className="flex flex-col gap-4">{children}</div>
  ) : (
    <div className={`flex flex-col divide-y divide-(--signin-separator) overflow-hidden bg-(--signin-group) ${look.group}`}>{children}</div>
  );
}

/** A server's row: its initial in a circle, its name and who is signed in. */
function Row({ name, detail, disabled, onClick }: { name: string; detail: string; disabled?: boolean; onClick: () => void }) {
  const look = useLook();
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={`flex w-full items-center text-left hover:bg-fill ${look.row}`}
    >
      <Avatar name={name} size={look.avatar} />
      <span className="min-w-0 flex-1">
        <span className={`block truncate ${look.rowTitle}`}>{name}</span>
        <span className={`block truncate ${look.rowDetail}`}>{detail}</span>
      </span>
      <Icon name="forward" size={look.icon} className="shrink-0 opacity-50" />
    </button>
  );
}

/** The Mavio account last signed in with, which continues as it. */
function AccountCard({ entry, disabled, onClick }: { entry: ServerEntry; disabled: boolean; onClick: () => void }) {
  const look = useLook();
  const name = entry.account?.name ?? entry.userName;
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={`flex w-full items-center text-left hover:bg-fill ${look.layout === "tv" ? "" : "bg-(--signin-group)"} ${look.account}`}
    >
      <Avatar name={name} size={look.accountAvatar} />
      <span className="min-w-0 flex-1">
        <span className={`block truncate ${look.accountName}`}>{name}</span>
        <span className={`block truncate ${look.accountDetail}`}>{t("auth.mavioAccountOf", { server: entry.name })}</span>
      </span>
      <span className={`shrink-0 font-semibold text-accent-text ${look.layout === "tv" ? "text-[28px]" : "text-[15px]"}`}>
        {t("auth.continue")}
      </span>
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
    <Frame>
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
    </Frame>
  );
}

/** A new server's choice of mode (docs/accounts.md §2). */
function ChooseMode({
  info,
  onBack,
  onChoose,
}: {
  info: ServerInfo;
  onBack?: () => void;
  onChoose: (mode: "standalone" | "mavioDev") => void;
}) {
  const look = useLook();
  const [mode, setMode] = useState<"standalone" | "mavioDev">("mavioDev");
  const tv = look.layout === "tv";
  const option = (m: "standalone" | "mavioDev", title: string, message: string) => (
    <button
      type="button"
      role="radio"
      aria-checked={mode === m}
      onClick={() => setMode(m)}
      className={`flex items-start gap-3.5 text-left ${tv ? "rounded-[22px] bg-fill p-7 transition-transform focus:scale-[1.04]" : "rounded-[14px] bg-(--signin-group) p-4"} ${mode === m ? "ring-2 ring-accent" : ""}`}
    >
      <span
        className={`mt-0.5 flex shrink-0 items-center justify-center rounded-full border-2 ${tv ? "h-8 w-8" : "h-[22px] w-[22px]"} ${mode === m ? "border-accent" : "border-text-tertiary"}`}
      >
        {mode === m && <span className={`rounded-full bg-accent ${tv ? "h-4 w-4" : "h-3 w-3"}`} />}
      </span>
      <span className="flex flex-col gap-1">
        <span className={look.rowTitle}>{title}</span>
        <span className={tv ? "text-[22px] leading-7 opacity-70" : "text-[14px] leading-5 text-text-secondary"}>{message}</span>
      </span>
    </button>
  );
  return (
    <Steps
      eyebrow={t("auth.newServerSetup")}
      title={t("auth.chooseMode", { server: info.name })}
      subtitle={info.url}
      onBack={onBack}
    >
      <div role="radiogroup" className="flex flex-col gap-3">
        {option("mavioDev", t("auth.modeMavioDev"), t("auth.modeMavioDevMessage"))}
        {option("standalone", t("auth.modeStandalone"), t("auth.modeStandaloneMessage"))}
      </div>
      <Action onClick={() => onChoose(mode)}>{t("auth.continue")}</Action>
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
  onBack?: () => void;
  onClaim: (code: string) => void;
}) {
  const look = useLook();
  const [code, setCode] = useState("");
  const site = info.mavioDevUrl?.replace(/\/api\/?$/, "");
  return (
    <Steps eyebrow={t("auth.newServerSetup")} title={t("auth.claim", { server: info.name })} subtitle={info.url} onBack={onBack}>
      <form
        className={`flex flex-col ${look.stack}`}
        onSubmit={(e) => {
          e.preventDefault();
          onClaim(code);
        }}
      >
        <p className={look.body}>{t("auth.claimMessage")}</p>
        {site && (
          <a href={`${site}/claim`} target="_blank" rel="noopener" className="self-start font-semibold text-accent-text hover:underline">
            {t("auth.getClaimCode")}
          </a>
        )}
        <Fields
          fields={[
            {
              label: t("auth.claimCode"),
              value: code,
              onChange: (e) => setCode(e.target.value.toUpperCase()),
              placeholder: "XXXX-XXXX",
              autoComplete: "off",
              autoCapitalize: "characters",
              className: "font-mono",
              autoFocus: look.layout !== "tv",
              required: true,
            },
          ]}
        />
        {error && <p className="text-[15px] text-danger">{error}</p>}
        <Action type="submit" disabled={busy || !code.trim()}>
          {t("auth.claimServer")}
        </Action>
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
  onBack?: () => void;
  onSubmit: (name: string, password: string, code?: string) => void;
  onPasskey: () => void;
  onQuickConnect: () => void;
}) {
  const look = useLook();
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const passkeys = passkeysHere(info.url);
  return (
    <Steps
      eyebrow={info.setupRequired ? t("auth.newServerSetup") : undefined}
      title={info.setupRequired ? t("auth.setUp", { server: info.name }) : t("auth.signInTo", { server: info.name })}
      subtitle={`${t("auth.standaloneServer")} · ${info.url}`}
      onBack={onBack}
    >
      <form
        className={`flex flex-col ${look.stack}`}
        onSubmit={(e) => {
          e.preventDefault();
          onSubmit(name.trim(), password, needsCode ? code : undefined);
        }}
      >
        {info.setupRequired && <p className={look.body}>{t("auth.setUpMessage")}</p>}
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
              autoComplete: info.setupRequired ? "new-password" : "current-password",
              required: info.setupRequired,
            },
          ]}
        />
        {needsCode && (
          <>
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
          </>
        )}
        {error && <p className="text-[15px] text-danger">{error}</p>}
        <Action type="submit" disabled={busy || !name.trim() || (needsCode && code.length !== 6)}>
          {info.setupRequired ? t("auth.createAccount") : t("auth.signIn")}
        </Action>
        {!info.setupRequired && (
          <>
            <div className="flex items-center gap-3 text-[13px] text-text-tertiary">
              <span className="h-px flex-1 bg-(--signin-separator)" />
              {t("auth.or")}
              <span className="h-px flex-1 bg-(--signin-separator)" />
            </div>
            <div className={`grid gap-2.5 ${passkeys ? "grid-cols-2" : "grid-cols-1"}`}>
              {passkeys && (
                <Action variant="secondary" icon="key" onClick={onPasskey} disabled={busy}>
                  {t("auth.passkey")}
                </Action>
              )}
              <Action variant="secondary" icon="devices" onClick={onQuickConnect} disabled={busy}>
                {t("auth.quickConnect")}
              </Action>
            </div>
          </>
        )}
      </form>
    </Steps>
  );
}

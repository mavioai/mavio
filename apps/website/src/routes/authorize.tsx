import { useMutation, useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect } from "react";

import { Action, actionClass, AuthScreen, AuthText, Notice, SignedInAs } from "../auth-ui.tsx";
import { accounts, config } from "../lib/api.ts";
import type { MessageKey } from "../lib/i18n.ts";
import { describe, useRich, useT } from "../lib/locale.tsx";
import { expectedOrigin } from "../lib/origins.ts";
import { useAccount } from "../lib/session.ts";
import { Loading, Name } from "../ui.tsx";

// A web UI signs the account in (docs/accounts.md §6): an authorization
// request with PKCE, carrying the key of the device whose session it
// gets. A server pushes its web UI's request, signed with its identity
// key, and the page names it by request_uri; approving it signs in only
// the server's administrator, or claims a server that is not claimed
// yet (docs/accounts.md §3.2). app.mavio.dev sends its request here
// itself, with the client_id app, and signs in at once. One of Mavio's
// apps opens the page in the browser with its reverse-DNS client ID and
// a loopback redirect URI or one of its own URI scheme (RFC 8252), and
// the account approves it.
type Search = {
  request_uri?: string;
  client_id?: string;
  redirect_uri?: string;
  state?: string;
  code_challenge?: string;
  code_challenge_method?: string;
  device_key?: string;
};

const str = (v: unknown) => (typeof v === "string" ? v : undefined);

export const Route = createFileRoute("/authorize")({
  validateSearch: (s: Record<string, unknown>): Search => ({
    request_uri: str(s.request_uri),
    client_id: str(s.client_id),
    redirect_uri: str(s.redirect_uri),
    state: str(s.state),
    code_challenge: str(s.code_challenge),
    code_challenge_method: str(s.code_challenge_method),
    device_key: str(s.device_key),
  }),
  component: Authorize,
});

function fromBase64url(s: string): Uint8Array | undefined {
  try {
    return Uint8Array.from(atob(s.replace(/-/g, "+").replace(/_/g, "/")), (c) => c.charCodeAt(0));
  } catch {
    return undefined;
  }
}

function redirectTo(uri: string, params: Record<string, string | undefined>) {
  const u = new URL(uri);
  for (const [k, v] of Object.entries(params)) if (v !== undefined) u.searchParams.set(k, v);
  window.location.href = u.toString();
}

type Me = { name: string; email: string; username?: string | null };

function Authorize() {
  const me = useAccount();
  const q = Route.useSearch();
  if (!me) return <Loading />;
  if (q.request_uri) return <ServerRequest request={q.request_uri} me={me} />;
  if (q.client_id && q.client_id !== "app") return <NativeRequest q={q} me={me} />;
  return <AppRequest q={q} />;
}

/** app.mavio.dev's request, which is mavio.dev's own: it signs in without asking. */
function AppRequest({ q }: { q: Search }) {
  const t = useT();
  const site = useQuery({ queryKey: ["config"], queryFn: config });
  const approve = useMutation({
    mutationFn: async () => {
      const { code } = await accounts.createAuthorizationCode({
        redirectUri: q.redirect_uri ?? "",
        codeChallenge: q.code_challenge ?? "",
        deviceKey: fromBase64url(q.device_key ?? ""),
      });
      redirectTo(q.redirect_uri ?? "", { code, state: q.state });
    },
  });
  const appOrigin = site.data && new URL(site.data.appURL).origin;
  const key = fromBase64url(q.device_key ?? "");
  const complete = q.client_id === "app" && q.code_challenge_method === "S256" && !!q.code_challenge && key?.length === 32;
  const toApp = complete && !!appOrigin && !!q.redirect_uri && URL.canParse(q.redirect_uri) && new URL(q.redirect_uri).origin === appOrigin;
  const { mutate, isIdle } = approve;
  useEffect(() => {
    if (toApp && isIdle) mutate();
  }, [toApp, isIdle, mutate]);
  const error = !complete ? t("site.authorize.incomplete") : site.error || approve.error ? describe(t, site.error ?? approve.error) : undefined;
  return (
    <AuthScreen title={t("site.signIn.title")}>
      {error ? (
        <Notice>{error}</Notice>
      ) : site.isPending || toApp ? (
        <AuthText>{t("site.authorize.signingIn")}</AuthText>
      ) : (
        <Notice>{t("site.authorize.notWeb")}</Notice>
      )}
    </AuthScreen>
  );
}

/** The names of Mavio's apps by their client IDs. */
const appNames: Record<string, MessageKey> = {
  "dev.mavio.desktop": "site.app.desktop",
  "dev.mavio.app": "site.app.android",
  "dev.mavio.ios": "site.app.ios",
  "dev.mavio.macos": "site.app.macos",
  "dev.mavio.tvos": "site.app.tvos",
};

/**
 * Whether a redirect URI is an app's on this device: http://127.0.0.1 or http://[::1] with any port (RFC 8252 §7.3),
 * or a URI of the scheme named after the app's client ID, such as dev.mavio.app:/callback (RFC 8252 §7.1).
 */
function appRedirect(uri: string | undefined, clientId: string | undefined): boolean {
  if (!uri || !clientId || !URL.canParse(uri)) return false;
  const u = new URL(uri);
  if (u.protocol === `${clientId}:`) return u.host === "";
  return u.protocol === "http:" && (u.hostname === "127.0.0.1" || u.hostname === "[::1]");
}

/** Approving or cancelling a request. */
function Answer({ onAnswer, pending }: { onAnswer: (approve: boolean) => void; pending: boolean }) {
  const t = useT();
  return (
    <div className="flex flex-col gap-3">
      <Action onClick={() => onAnswer(true)} disabled={pending}>
        {t("site.action.signIn")}
      </Action>
      <Action variant="secondary" onClick={() => onAnswer(false)} disabled={pending}>
        {t("site.action.cancel")}
      </Action>
    </div>
  );
}

/**
 * One of Mavio's apps signing the account in, from the browser it opened:
 * the account approves it, and the code goes back to the app on its
 * loopback address or through its own URI scheme.
 */
function NativeRequest({ q, me }: { q: Search; me: Me }) {
  const t = useT();
  const answer = useMutation({
    mutationFn: async (approve: boolean) => {
      if (!approve) return redirectTo(q.redirect_uri ?? "", { error: "access_denied", state: q.state });
      const { code } = await accounts.createAuthorizationCode({
        clientId: q.client_id,
        redirectUri: q.redirect_uri ?? "",
        codeChallenge: q.code_challenge ?? "",
        deviceKey: fromBase64url(q.device_key ?? ""),
      });
      redirectTo(q.redirect_uri ?? "", { code, state: q.state });
    },
  });
  const key = fromBase64url(q.device_key ?? "");
  const name = appNames[q.client_id ?? ""];
  const app = name ? t(name) : (q.client_id ?? "");
  const complete =
    /^dev\.mavio\.[a-z0-9.-]+$/.test(q.client_id ?? "") &&
    q.code_challenge_method === "S256" &&
    !!q.code_challenge &&
    key?.length === 32 &&
    appRedirect(q.redirect_uri, q.client_id);
  if (!complete) {
    return (
      <AuthScreen title={t("site.signIn.title")}>
        <Notice>{t("site.authorize.incomplete")}</Notice>
      </AuthScreen>
    );
  }
  return (
    <AuthScreen title={t("site.authorize.appTitle", { app })}>
      <AuthText>{t("site.authorize.appBody")}</AuthText>
      <SignedInAs me={me} />
      <Answer onAnswer={(approve) => answer.mutate(approve)} pending={answer.isPending} />
      <Notice>{describe(t, answer.error)}</Notice>
    </AuthScreen>
  );
}

/** A server's web UI signing in its administrator, or claiming the server. */
function ServerRequest({ request, me }: { request: string; me: Me }) {
  const t = useT();
  const rich = useRich();
  const site = useQuery({ queryKey: ["config"], queryFn: config });
  const r = useQuery({
    queryKey: ["authorizationRequest", request],
    queryFn: () => accounts.getAuthorizationRequest({ authorizationRequest: request }),
    retry: false,
  });
  const answer = useMutation({
    mutationFn: async (approve: boolean) => {
      const { redirectUrl } = await accounts.answerAuthorizationRequest({ authorizationRequest: request, approve });
      window.location.href = redirectUrl;
    },
  });
  const c = r.data;
  const server = <Name>{c?.serverName || t("site.authorize.thisServer")}</Name>;
  const address = <Name>{c && URL.canParse(c.redirectUri) ? new URL(c.redirectUri).host : ""}</Name>;
  const appOrigin = site.data && new URL(site.data.appURL).origin;
  return (
    <AuthScreen title={t("site.authorize.webTitle")}>
      {r.isPending && <AuthText>{t("site.authorize.signingIn")}</AuthText>}
      {c &&
        (c.claim || c.administrator ? (
          <>
            <AuthText>{rich(c.claim ? "site.authorize.claim" : "site.authorize.web", { server, address })}</AuthText>
            {c.claim && c.claimed && !c.administrator && <Notice>{t("site.authorize.claimed")}</Notice>}
            {!expectedOrigin(c.redirectUri, c.serverId) && <Notice>{t("site.authorize.elsewhere")}</Notice>}
            <SignedInAs me={me} />
            <Answer onAnswer={(approve) => answer.mutate(approve)} pending={answer.isPending} />
          </>
        ) : (
          <>
            <AuthText>{rich("site.authorize.adminOnly", { server })}</AuthText>
            <SignedInAs me={me} />
            {appOrigin && (
              <a className={actionClass()} href={appOrigin}>
                {t("site.authorize.openApp")}
              </a>
            )}
          </>
        ))}
      <Notice>{r.error ? t("site.authorize.unknown") : describe(t, answer.error)}</Notice>
    </AuthScreen>
  );
}

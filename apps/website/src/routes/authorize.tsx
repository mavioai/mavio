import { useMutation, useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { type ReactNode, useEffect } from "react";

import { accounts, config, message } from "../lib/api.ts";
import { authClient } from "../lib/auth-client.ts";
import { expectedOrigin } from "../lib/origins.ts";
import { useAccount } from "../lib/session.ts";
import { Wordmark } from "../logo.tsx";
import { Button, ErrorText, Muted } from "../ui.tsx";

// A web UI signs the account in (docs/accounts.md §6): an authorization
// request with PKCE, carrying the key of the device whose session it
// gets. A server pushes its web UI's request, signed with its identity
// key, and the page names it by request_uri; approving it signs in only
// the server's administrator, or claims a server that is not claimed
// yet (docs/accounts.md §3.2). app.mavio.dev sends its request here
// itself, with the client_id app, and signs in at once.
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
  if (!me) return null;
  if (q.request_uri) return <ServerRequest request={q.request_uri} me={me} />;
  return <AppRequest q={q} />;
}

/** The page of its own that asks the account, centered under the logo. */
function Frame({ title, children }: { title: string; children: ReactNode }) {
  return (
    <main className="mx-auto flex min-h-dvh max-w-sm flex-col items-center justify-center gap-6 px-4 py-10 text-center">
      <Wordmark height={36} />
      <h1 className="type-title2">{title}</h1>
      {children}
    </main>
  );
}

/** app.mavio.dev's request, which is mavio.dev's own: it signs in without asking. */
function AppRequest({ q }: { q: Search }) {
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
  if (!complete) {
    return (
      <Frame title="Sign in to Mavio">
        <ErrorText error="This sign-in request is incomplete. Start signing in again." />
      </Frame>
    );
  }
  return (
    <Frame title="Sign in to Mavio">
      {site.isPending || toApp ? <Muted>Signing in…</Muted> : <ErrorText error="This sign-in request does not come from Mavio on the web." />}
      <ErrorText error={site.error ? message(site.error) : approve.error ? message(approve.error) : undefined} />
    </Frame>
  );
}

/** The account's avatar, colored from its name as in every client (libs/design/components.md, Avatar). */
function Avatar({ name }: { name: string }) {
  let hue = 0;
  for (const c of name) hue = (hue * 31 + c.codePointAt(0)!) % 360;
  return (
    <span
      aria-hidden
      className="flex size-9 shrink-0 items-center justify-center rounded-full font-semibold text-white"
      style={{ fontSize: 36 * 0.42, background: `hsl(${hue} 42% 44%)` }}
    >
      {[...name][0]?.toUpperCase()}
    </span>
  );
}

/** A server's web UI signing in its administrator, or claiming the server. */
function ServerRequest({ request, me }: { request: string; me: Me }) {
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
  // Another account signs in on mavio.dev and comes back here.
  const switchAccount = async () => {
    const back = window.location.pathname + window.location.search;
    await authClient.signOut();
    window.location.href = `/sign-in?${new URLSearchParams({ next: back })}`;
  };
  const c = r.data;
  const server = <span className="font-mono font-semibold text-on-background">{c?.serverName || "this server"}</span>;
  const origin = c && URL.canParse(c.redirectUri) ? new URL(c.redirectUri).origin : undefined;
  const at = <span className="font-mono font-semibold text-on-background">{origin}</span>;
  const appOrigin = site.data && new URL(site.data.appURL).origin;
  const elsewhere = c && !expectedOrigin(c.redirectUri, c.serverId) && (
    <ErrorText error="This address is neither one of the server's mavio.site names nor on your local network. Continue only if you reach the server there." />
  );
  // A managed account's e-mail address is a placeholder, never shown.
  const email = me.email.startsWith("managed:") ? undefined : me.email;
  const detail = me.username && me.username !== me.name ? `@${me.username}` : email;
  const signedInAs = (
    <div className="flex w-full items-center gap-3 rounded-lg border border-border bg-surface px-4 py-3 text-left">
      <Avatar name={me.name || me.username || "?"} />
      <span className="flex min-w-0 flex-col">
        <span className="type-callout truncate font-semibold">{me.name || me.username}</span>
        {detail && <span className="type-footnote truncate text-text-secondary">{detail}</span>}
      </span>
    </div>
  );
  const differentAccount = (
    <p className="type-footnote text-text-secondary">
      Or{" "}
      <button type="button" className="text-accent hover:underline" onClick={() => void switchAccount()}>
        use a different account
      </button>
      .
    </p>
  );
  const buttons = (
    <div className="flex w-full flex-col gap-3">
      <Button onClick={() => answer.mutate(true)} disabled={answer.isPending}>
        Sign in
      </Button>
      <Button kind="secondary" onClick={() => answer.mutate(false)} disabled={answer.isPending}>
        Cancel
      </Button>
    </div>
  );
  return (
    <Frame title="Mavio Web would like to sign in to your Mavio account">
      {c &&
        (c.claim || c.administrator ? (
          <>
            <Muted>
              {c.claim ? (
                <>
                  You become the administrator of {server} at {at}, and your account reaches it everywhere.
                </>
              ) : (
                <>
                  You sign in to the web UI of {server} at {at}, which reaches your account's servers.
                </>
              )}
            </Muted>
            {c.claim && c.claimed && !c.administrator && (
              <ErrorText error="Another account claimed this server before. Signing in takes it over." />
            )}
            {elsewhere}
            {signedInAs}
            {buttons}
            {differentAccount}
          </>
        ) : (
          <>
            <Muted>
              Only the administrator of {server} signs in to its web UI with a Mavio account. Watch the servers your account reaches on
              Mavio on the web.
            </Muted>
            {signedInAs}
            {appOrigin && (
              <a className="type-callout font-semibold text-accent" href={appOrigin}>
                Open Mavio on the web
              </a>
            )}
            {differentAccount}
          </>
        ))}
      <ErrorText
        error={
          r.error ? "This sign-in request is unknown or expired. Start signing in again from the server." : answer.error ? message(answer.error) : undefined
        }
      />
    </Frame>
  );
}

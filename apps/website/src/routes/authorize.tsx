import { useMutation, useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect } from "react";

import { accounts, config, message } from "../lib/api.ts";
import { expectedOrigin } from "../lib/origins.ts";
import { useAccount } from "../lib/session.ts";
import { Button, Card, ErrorText, Muted, Page } from "../ui.tsx";

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

function Authorize() {
  const me = useAccount();
  const q = Route.useSearch();
  if (!me) return null;
  if (q.request_uri) return <ServerRequest request={q.request_uri} name={me.name} />;
  return <AppRequest q={q} />;
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
      <Page title="Sign in to a server">
        <Card>
          <ErrorText error="This sign-in request is incomplete. Start signing in again." />
        </Card>
      </Page>
    );
  }
  return (
    <Page title="Sign in to Mavio">
      <Card>
        {site.isPending || toApp ? <Muted>Signing in…</Muted> : <ErrorText error="This sign-in request does not come from Mavio on the web." />}
        <ErrorText error={site.error ? message(site.error) : approve.error ? message(approve.error) : undefined} />
      </Card>
    </Page>
  );
}

/** A server's web UI signing in its administrator, or claiming the server. */
function ServerRequest({ request, name }: { request: string; name: string }) {
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
  const server = c?.serverName || "this server";
  const origin = c && URL.canParse(c.redirectUri) ? new URL(c.redirectUri).origin : undefined;
  const appOrigin = site.data && new URL(site.data.appURL).origin;
  const buttons = (approve: string) => (
    <div className="flex gap-3">
      <Button onClick={() => answer.mutate(true)} disabled={answer.isPending}>
        {approve}
      </Button>
      <Button kind="secondary" onClick={() => answer.mutate(false)} disabled={answer.isPending}>
        Cancel
      </Button>
    </div>
  );
  const at = <span className="font-mono text-text-primary">{origin}</span>;
  const elsewhere = c && !expectedOrigin(c.redirectUri, c.serverId) && (
    <ErrorText error="This address is neither one of the server's mavio.site names nor on your local network. Continue only if you reach the server there." />
  );
  return (
    <Page title={c ? `Sign in to ${server}` : "Sign in to a server"}>
      <Card>
        {c &&
          (c.claim ? (
            <>
              <Muted>
                Sign in as {name} in the web UI of {server} at {at}? You become its administrator, and your account reaches
                it everywhere.
              </Muted>
              {c.claimed && !c.administrator && (
                <ErrorText error="Another account claimed this server before. Signing in takes it over." />
              )}
              {elsewhere}
              {buttons("Sign in")}
            </>
          ) : !c.administrator ? (
            <>
              <Muted>
                Only the administrator of {server} signs in to its web UI with a Mavio account. Watch the servers your account
                reaches on Mavio on the web.
              </Muted>
              {appOrigin && (
                <a className="type-callout text-accent" href={appOrigin}>
                  Open Mavio on the web
                </a>
              )}
            </>
          ) : (
            <>
              <Muted>
                Sign in as {name} in the web UI of {server} at {at}? It reaches your account's servers.
              </Muted>
              {elsewhere}
              {buttons("Sign in")}
            </>
          ))}
        <ErrorText
          error={
            r.error ? "This sign-in request is unknown or expired. Start signing in again from the server." : answer.error ? message(answer.error) : undefined
          }
        />
      </Card>
    </Page>
  );
}

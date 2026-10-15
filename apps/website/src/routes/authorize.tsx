import { useMutation, useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect } from "react";

import { accounts, config, message } from "../lib/api.ts";
import { expectedOrigin } from "../lib/origins.ts";
import { useAccount } from "../lib/session.ts";
import { Button, Card, ErrorText, Muted, Page } from "../ui.tsx";

// A web UI signs the account in (docs/accounts.md §6): an authorization
// request with PKCE, carrying the key of the device whose session it
// gets. Its client is app.mavio.dev, which signs in at once, or a server,
// whose web UI signs in only the server's administrator.
type Search = {
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
  const app = q.client_id === "app";
  const site = useQuery({ queryKey: ["config"], queryFn: config });
  const servers = useQuery({ queryKey: ["servers"], queryFn: () => accounts.listServers({}), enabled: !!me && !app });
  const approve = useMutation({
    mutationFn: async () => {
      const { code } = await accounts.createAuthorizationCode({
        serverId: app ? "" : (q.client_id ?? ""),
        redirectUri: q.redirect_uri ?? "",
        codeChallenge: q.code_challenge ?? "",
        deviceKey: fromBase64url(q.device_key ?? ""),
      });
      redirectTo(q.redirect_uri ?? "", { code, state: q.state });
    },
  });
  const appOrigin = site.data && new URL(site.data.appURL).origin;
  const toApp = app && !!appOrigin && !!q.redirect_uri && URL.canParse(q.redirect_uri) && new URL(q.redirect_uri).origin === appOrigin;
  // app.mavio.dev is mavio.dev's own: it signs in without asking.
  const { mutate, isIdle } = approve;
  useEffect(() => {
    if (me && toApp && isIdle) mutate();
  }, [me, toApp, isIdle, mutate]);
  if (!me) return null;

  let origin: string | undefined;
  try {
    const u = q.redirect_uri ? new URL(q.redirect_uri) : undefined;
    if (u && (u.protocol === "http:" || u.protocol === "https:")) origin = u.origin;
  } catch {
    // Refused below.
  }
  const key = fromBase64url(q.device_key ?? "");
  if (!q.client_id || !origin || q.code_challenge_method !== "S256" || !q.code_challenge || key?.length !== 32) {
    return (
      <Page title="Sign in to a server">
        <Card>
          <ErrorText error="This sign-in request is incomplete. Start signing in again." />
        </Card>
      </Page>
    );
  }
  if (app) {
    return (
      <Page title="Sign in to Mavio">
        <Card>
          {site.isPending || toApp ? (
            <Muted>Signing in…</Muted>
          ) : (
            <ErrorText error="This sign-in request does not come from Mavio on the web." />
          )}
          <ErrorText error={site.error ? message(site.error) : approve.error ? message(approve.error) : undefined} />
        </Card>
      </Page>
    );
  }
  const server = servers.data?.servers.find((s) => s.id === q.client_id && s.administrator?.id === me.id);
  const expected = expectedOrigin(q.redirect_uri ?? "", q.client_id);
  return (
    <Page title={server ? `Sign in to ${server.name}` : "Sign in to a server"}>
      <Card>
        {servers.isPending ? null : !server ? (
          <>
            <Muted>
              Only the administrator of this server signs in to its web UI with a Mavio account. Watch the servers your
              account reaches on Mavio on the web.
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
              Sign in as {me.name} in the web UI of {server.name} at <span className="font-mono text-text-primary">{origin}</span>?
              It reaches your account's servers.
            </Muted>
            {!expected && (
              <ErrorText error="This address is neither one of the server's mavio.site names nor on your local network. Continue only if you reach the server there." />
            )}
            <div className="flex gap-3">
              <Button onClick={() => approve.mutate()} disabled={approve.isPending}>
                Sign in
              </Button>
              <Button kind="secondary" onClick={() => redirectTo(q.redirect_uri ?? "", { error: "access_denied", state: q.state })}>
                Cancel
              </Button>
            </div>
          </>
        )}
        <ErrorText error={servers.error ? message(servers.error) : approve.error ? message(approve.error) : undefined} />
      </Card>
    </Page>
  );
}

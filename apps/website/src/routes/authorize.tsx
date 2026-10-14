import { useMutation, useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { accounts, message } from "../lib/api.ts";
import { expectedOrigin } from "../lib/origins.ts";
import { useAccount } from "../lib/session.ts";
import { Button, Card, ErrorText, Muted, Page } from "../ui.tsx";

// A server's web interface signs the account in to the server
// (docs/accounts.md §6): an authorization request with PKCE whose client
// is the server, carrying the device key the assertion is for.
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
  const servers = useQuery({ queryKey: ["servers"], queryFn: () => accounts.listServers({}), enabled: !!me });
  const approve = useMutation({
    mutationFn: async () => {
      const { code } = await accounts.createAuthorizationCode({
        serverId: q.client_id ?? "",
        redirectUri: q.redirect_uri ?? "",
        codeChallenge: q.code_challenge ?? "",
        deviceKey: fromBase64url(q.device_key ?? ""),
      });
      redirectTo(q.redirect_uri ?? "", { code, state: q.state });
    },
  });
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
          <ErrorText error="This sign-in request is incomplete. Start signing in again from the server." />
        </Card>
      </Page>
    );
  }
  const server = servers.data?.servers.find((s) => s.id === q.client_id);
  const expected = expectedOrigin(q.redirect_uri ?? "", q.client_id);
  return (
    <Page title={server ? `Sign in to ${server.name}` : "Sign in to a server"}>
      <Card>
        {servers.isPending ? null : !server ? (
          <ErrorText error="Your account has no access to this server." />
        ) : (
          <>
            <Muted>
              Sign in to {server.name} as {me.name} in the web interface at{" "}
              <span className="font-mono text-text-primary">{origin}</span>? It reaches this server only.
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

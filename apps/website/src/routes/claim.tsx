import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useMutation, useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { accounts, message } from "../lib/api.ts";
import { useAccount } from "../lib/session.ts";
import { Button, Card, ErrorText, Muted, Page } from "../ui.tsx";

// Claiming a server (docs/accounts.md §3.2): a server's web UI sends the
// browser here with the claim request the server made, and the account
// confirms it, which sends the browser back signed in. Without a request,
// the page gives a claim code for a server started without a browser at
// hand, in MAVIO_CLAIM_TOKEN.
export const Route = createFileRoute("/claim")({
  validateSearch: (s: Record<string, unknown>): { request?: string } => ({
    request: typeof s.request === "string" ? s.request : undefined,
  }),
  component: Claim,
});

function Claim() {
  const me = useAccount();
  const { request } = Route.useSearch();
  if (!me) return null;
  return request ? <ClaimRequest request={request} name={me.name} /> : <ClaimCode />;
}

function ClaimRequest({ request, name }: { request: string; name: string }) {
  const claim = useQuery({ queryKey: ["claim", request], queryFn: () => accounts.getClaimRequest({ claimRequest: request }), retry: false });
  const answer = useMutation({
    mutationFn: async (confirm: boolean) => {
      const { redirectUrl } = await accounts.answerClaimRequest({ claimRequest: request, confirm });
      window.location.href = redirectUrl;
    },
  });
  const c = claim.data;
  const origin = c && URL.canParse(c.redirectUri) ? new URL(c.redirectUri).origin : undefined;
  return (
    <Page title={c ? `Claim ${c.serverName || "a server"}` : "Claim a server"}>
      <Card>
        {c && (
          <>
            <Muted>
              Become the administrator of {c.serverName || "this server"} as {name}? Its web UI at{" "}
              <span className="font-mono text-text-primary">{origin}</span> signs you in, and your account reaches it
              everywhere.
            </Muted>
            {c.claimed && <ErrorText error="An account claimed this server before. Claiming it again takes it over." />}
            <div className="flex gap-3">
              <Button onClick={() => answer.mutate(true)} disabled={answer.isPending}>
                Claim
              </Button>
              <Button kind="secondary" onClick={() => answer.mutate(false)} disabled={answer.isPending}>
                Cancel
              </Button>
            </div>
          </>
        )}
        <ErrorText
          error={claim.error ? "This claim request is unknown or expired. Start claiming again from the server." : answer.error ? message(answer.error) : undefined}
        />
      </Card>
    </Page>
  );
}

function ClaimCode() {
  const code = useMutation({ mutationFn: () => accounts.createClaimCode({}) });
  const expires = code.data?.expireTime && timestampDate(code.data.expireTime);
  return (
    <Page title="Claim a server">
      <Card>
        <Muted>
          A claim code makes you the administrator of a server started without a browser at hand: start it with the code in
          MAVIO_CLAIM_TOKEN. A server you set up in a browser asks you to claim it there.
        </Muted>
        {code.data ? (
          <>
            <p className="type-display text-center font-mono tracking-widest">{code.data.claimCode}</p>
            <Muted>Valid until {expires?.toLocaleTimeString()}; it claims one server.</Muted>
          </>
        ) : (
          <Button onClick={() => code.mutate()} disabled={code.isPending}>
            Get a claim code
          </Button>
        )}
        <ErrorText error={code.error ? message(code.error) : undefined} />
      </Card>
    </Page>
  );
}

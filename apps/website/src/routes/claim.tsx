import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useMutation } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { accounts, message } from "../lib/api.ts";
import { useAccount } from "../lib/session.ts";
import { Button, Card, ErrorText, Muted, Page } from "../ui.tsx";

// A claim code (docs/accounts.md §3.2) for a server started without a
// browser at hand, in MAVIO_CLAIM_TOKEN; a server set up in a browser is
// claimed by signing in to it (/authorize).
export const Route = createFileRoute("/claim")({
  component: Claim,
});

function Claim() {
  const me = useAccount();
  if (!me) return null;
  return <ClaimCode />;
}

function ClaimCode() {
  const code = useMutation({ mutationFn: () => accounts.createClaimCode({}) });
  const expires = code.data?.expireTime && timestampDate(code.data.expireTime);
  return (
    <Page title="Claim a server">
      <Card>
        <Muted>
          A claim code makes you the administrator of a server started without a browser at hand: start it with the code in
          MAVIO_CLAIM_TOKEN. A server you set up in a browser is claimed when you sign in to it there.
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

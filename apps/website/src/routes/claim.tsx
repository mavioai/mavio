import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useMutation } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { accounts, message } from "../lib/api.ts";
import { useAccount } from "../lib/session.ts";
import { Button, Card, ErrorText, Muted, Page } from "../ui.tsx";

export const Route = createFileRoute("/claim")({ component: Claim });

function Claim() {
  const me = useAccount();
  const code = useMutation({ mutationFn: () => accounts.createClaimCode({}) });
  if (!me) return null;
  const expires = code.data?.expireTime && timestampDate(code.data.expireTime);
  return (
    <Page title="Claim a server">
      <Card>
        <Muted>
          A claim code makes you the administrator of a server you run. Enter it in the server's setup, or start a server
          without a browser at hand with the code in MAVIO_CLAIM_TOKEN.
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

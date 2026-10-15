import { useMutation } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";

import { accounts, message } from "../lib/api.ts";
import { useAccount } from "../lib/session.ts";
import { Button, Card, ErrorText, Field, Muted, Page } from "../ui.tsx";

// A television shows a code and the person signs it in here (RFC 8628):
// Mavio's apps and app.mavio.dev for any account, and a server's web UI
// for its administrator.
export const Route = createFileRoute("/link")({
  validateSearch: (s: Record<string, unknown>): { code?: string } => ({
    code: typeof s.code === "string" ? s.code : undefined,
  }),
  component: LinkDevice,
});

function LinkDevice() {
  const me = useAccount();
  const search = Route.useSearch();
  const [code, setCode] = useState<string>();
  const [done, setDone] = useState<"approved" | "denied">();
  const check = useMutation({
    mutationFn: async (userCode: string) => {
      const res = await accounts.getLinkCode({ userCode });
      setCode(userCode);
      return res;
    },
  });
  const answer = useMutation({
    mutationFn: async (approve: boolean) => {
      await accounts.answerLinkCode({ userCode: code ?? "", approve });
      setDone(approve ? "approved" : "denied");
    },
  });
  if (!me) return null;

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    check.mutate(String(new FormData(e.currentTarget).get("code")));
  }

  const target = check.data?.serverName ? `the web UI of ${check.data.serverName}` : "Mavio";
  return (
    <Page title="Sign in a television">
      <Card>
        {done ? (
          <Muted>{done === "approved" ? "The television is signed in to your account." : "The television was not signed in."}</Muted>
        ) : code && check.data ? (
          <>
            <Muted>
              Sign in {target} on the television showing {code.toUpperCase()} as {me.name}? It reaches your account's
              servers.
            </Muted>
            <div className="flex gap-3">
              <Button onClick={() => answer.mutate(true)} disabled={answer.isPending}>
                Sign in
              </Button>
              <Button kind="secondary" onClick={() => answer.mutate(false)} disabled={answer.isPending}>
                Cancel
              </Button>
            </div>
          </>
        ) : (
          <form className="flex flex-col gap-4" onSubmit={submit}>
            <Field label="The code on the television" name="code" defaultValue={search.code} autoComplete="off" required />
            <Button type="submit" disabled={check.isPending}>
              Continue
            </Button>
          </form>
        )}
        <ErrorText error={check.error ? message(check.error) : answer.error ? message(answer.error) : undefined} />
      </Card>
    </Page>
  );
}

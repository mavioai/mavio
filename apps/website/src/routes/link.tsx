import { createFileRoute } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";

import { authClient } from "../lib/auth-client.ts";
import { useAccount } from "../lib/session.ts";
import { Button, Card, ErrorText, Field, Muted, Page } from "../ui.tsx";

// A television shows a code and the person signs it in here (RFC 8628).
export const Route = createFileRoute("/link")({
  validateSearch: (s: Record<string, unknown>): { user_code?: string } => ({
    user_code: typeof s.user_code === "string" ? s.user_code : undefined,
  }),
  component: LinkDevice,
});

function LinkDevice() {
  const me = useAccount();
  const { user_code } = Route.useSearch();
  const [code, setCode] = useState<string>();
  const [error, setError] = useState<string>();
  const [done, setDone] = useState<"approved" | "denied">();
  if (!me) return null;

  async function check(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const userCode = String(new FormData(e.currentTarget).get("code")).toUpperCase().replace(/[\s-]/g, "");
    const res = await authClient.device({ query: { user_code: userCode } });
    if (res.error) setError(res.error.error_description);
    else {
      setError(undefined);
      setCode(userCode);
    }
  }

  async function answer(approve: boolean) {
    if (!code) return;
    const res = approve ? await authClient.device.approve({ userCode: code }) : await authClient.device.deny({ userCode: code });
    if (res.error) setError(res.error.error_description);
    else setDone(approve ? "approved" : "denied");
  }

  return (
    <Page title="Sign in a television">
      <Card>
        {done ? (
          <Muted>{done === "approved" ? "The television is signed in to your account." : "The television was not signed in."}</Muted>
        ) : code ? (
          <>
            <Muted>
              Sign in the television showing {code} as {me.name}? It reaches every server your account reaches.
            </Muted>
            <div className="flex gap-3">
              <Button onClick={() => answer(true)}>Sign in</Button>
              <Button kind="secondary" onClick={() => answer(false)}>
                Cancel
              </Button>
            </div>
          </>
        ) : (
          <form className="flex flex-col gap-4" onSubmit={check}>
            <Field label="The code on the television" name="code" defaultValue={user_code} autoComplete="off" required />
            <Button type="submit">Continue</Button>
          </form>
        )}
        <ErrorText error={error} />
      </Card>
    </Page>
  );
}

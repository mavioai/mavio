import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";

import { authClient } from "../lib/auth-client.ts";
import { Button, Card, ErrorText, Field, Muted, Page } from "../ui.tsx";

type Search = { next?: string; step?: "totp" };

export const Route = createFileRoute("/sign-in")({
  validateSearch: (s: Record<string, unknown>): Search => ({
    next: typeof s.next === "string" && s.next.startsWith("/") ? s.next : undefined,
    step: s.step === "totp" ? "totp" : undefined,
  }),
  component: SignIn,
});

function SignIn() {
  const { next, step } = Route.useSearch();
  const navigate = useNavigate();
  const [error, setError] = useState<string>();
  const done = () => {
    window.location.href = next ?? "/";
  };

  if (step === "totp") return <Totp onDone={done} />;

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const name = String(f.get("name"));
    const password = String(f.get("password"));
    const res = name.includes("@")
      ? await authClient.signIn.email({ email: name, password })
      : await authClient.signIn.username({ username: name, password });
    if (res.error) setError(res.error.message);
    else if (res.data && "twoFactorRedirect" in res.data && res.data.twoFactorRedirect) {
      await navigate({ to: "/sign-in", search: { next, step: "totp" } });
    } else done();
  }

  return (
    <Page title="Sign in to Mavio">
      <Card>
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <Field label="E-mail address or user name" name="name" autoComplete="username webauthn" required />
          <Field label="Password" name="password" type="password" autoComplete="current-password" required />
          <ErrorText error={error} />
          <Button type="submit">Sign in</Button>
        </form>
        <Button
          kind="secondary"
          onClick={async () => {
            const res = await authClient.signIn.passkey();
            if (res?.error) setError(res.error.message);
            else done();
          }}
        >
          Sign in with a passkey
        </Button>
      </Card>
      <Muted>
        No account yet?{" "}
        <Link to="/sign-up" search={{ next }} className="text-accent">
          Create one
        </Link>
      </Muted>
    </Page>
  );
}

function Totp({ onDone }: { onDone: () => void }) {
  const [error, setError] = useState<string>();
  return (
    <Page title="Two-factor authentication">
      <Card>
        <form
          className="flex flex-col gap-4"
          onSubmit={async (e) => {
            e.preventDefault();
            const code = String(new FormData(e.currentTarget).get("code"));
            const res = await authClient.twoFactor.verifyTotp({ code, trustDevice: true });
            if (res.error) setError(res.error.message);
            else onDone();
          }}
        >
          <Field label="The code of your authenticator app" name="code" inputMode="numeric" autoComplete="one-time-code" required />
          <ErrorText error={error} />
          <Button type="submit">Verify</Button>
        </form>
      </Card>
    </Page>
  );
}

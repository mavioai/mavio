import { createFileRoute, Link } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";

import { authClient } from "../lib/auth-client.ts";
import { Button, Card, ErrorText, Field, Muted, Page } from "../ui.tsx";

export const Route = createFileRoute("/sign-up")({
  validateSearch: (s: Record<string, unknown>): { next?: string } => ({
    next: typeof s.next === "string" && s.next.startsWith("/") ? s.next : undefined,
  }),
  component: SignUp,
});

function SignUp() {
  const { next } = Route.useSearch();
  const [error, setError] = useState<string>();

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const res = await authClient.signUp.email({
      email: String(f.get("email")),
      username: String(f.get("username")),
      name: String(f.get("name")) || String(f.get("username")),
      password: String(f.get("password")),
    });
    if (res.error) setError(res.error.message);
    else window.location.href = next ?? "/";
  }

  return (
    <Page title="Create a Mavio account">
      <Card>
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <Field label="E-mail address" name="email" type="email" autoComplete="email" required />
          <Field label="User name" name="username" autoComplete="username" minLength={3} maxLength={32} required />
          <Field label="Display name" name="name" autoComplete="name" />
          <Field label="Password" name="password" type="password" autoComplete="new-password" minLength={8} required />
          <ErrorText error={error} />
          <Button type="submit">Create account</Button>
        </form>
      </Card>
      <Muted>
        Have an account?{" "}
        <Link to="/sign-in" search={{ next }} className="text-accent">
          Sign in
        </Link>
      </Muted>
    </Page>
  );
}

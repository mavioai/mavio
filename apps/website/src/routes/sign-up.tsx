import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";

import { Action, actionClass, AuthField, AuthFoot, AuthForm, AuthScreen, authLink, AuthText, Notice } from "../auth-ui.tsx";
import { authClient } from "../lib/auth-client.ts";
import { describe, useRich, useT } from "../lib/locale.tsx";
import { Name } from "../ui.tsx";
import { nextOf } from "./sign-in.tsx";

export const Route = createFileRoute("/sign-up")({
  validateSearch: (s: Record<string, unknown>): { next?: string } => ({ next: nextOf(s.next) }),
  component: SignUp,
});

function SignUp() {
  const t = useT();
  const rich = useRich();
  const { next } = Route.useSearch();
  const [error, setError] = useState<string>();
  const [pending, setPending] = useState(false);
  // The address the link went to: the account signs in once it is verified.
  const [sentTo, setSentTo] = useState<string>();

  async function submit(f: FormData) {
    setPending(true);
    const email = String(f.get("email")).trim();
    const res = await authClient.signUp.email({
      email,
      username: String(f.get("username")).trim(),
      name: String(f.get("name")).trim() || String(f.get("username")).trim(),
      password: String(f.get("password")),
      callbackURL: `/verified${next ? `?${new URLSearchParams({ next })}` : ""}`,
    });
    setPending(false);
    if (res.error) setError(describe(t, res.error));
    else setSentTo(email);
  }

  if (sentTo) {
    return (
      <AuthScreen title={t("site.checkMail.title")}>
        <AuthText>{rich("site.checkMail.body", { email: <Name>{sentTo}</Name> })}</AuthText>
        <AuthText>{t("site.checkMail.hint")}</AuthText>
        <Link to="/sign-in" search={{ next }} className={actionClass("secondary")}>
          {t("site.forgot.backToSignIn")}
        </Link>
      </AuthScreen>
    );
  }

  return (
    <AuthScreen title={t("site.signUp.title")} subtitle={t("site.signUp.subtitle")}>
      <AuthForm onSubmit={(f) => void submit(f)}>
        <AuthField label={t("site.field.email")} name="email" type="email" autoComplete="email" required />
        <AuthField
          label={t("site.field.userName")}
          name="username"
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          minLength={3}
          maxLength={32}
          pattern="[A-Za-z0-9_.]+"
          required
        />
        <AuthField label={t("site.field.displayName")} name="name" autoComplete="name" maxLength={64} />
        <AuthField
          label={t("site.field.password")}
          name="password"
          type="password"
          autoComplete="new-password"
          placeholder={t("site.field.passwordHint")}
          minLength={8}
          required
        />
        <Notice>{error}</Notice>
        <Action type="submit" disabled={pending}>
          {t("site.signUp.create")}
        </Action>
      </AuthForm>
      <AuthFoot>
        {t("site.signUp.haveAccount")}{" "}
        <Link to="/sign-in" search={{ next }} className={authLink}>
          {t("site.signUp.signIn")}
        </Link>
      </AuthFoot>
    </AuthScreen>
  );
}

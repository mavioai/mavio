import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Action, actionClass, AuthField, AuthForm, AuthScreen, AuthText, Notice } from "../auth-ui.tsx";
import { authClient } from "../lib/auth-client.ts";
import { describe, useT } from "../lib/locale.tsx";

// The e-mailed link's page (docs/mavio-dev.md §2): better-auth checks the
// link and comes here with its token, or with error=INVALID_TOKEN.
type Search = { token?: string; error?: string };

export const Route = createFileRoute("/reset-password")({
  validateSearch: (s: Record<string, unknown>): Search => ({
    token: typeof s.token === "string" ? s.token : undefined,
    error: typeof s.error === "string" ? s.error : undefined,
  }),
  component: ResetPassword,
});

function ResetPassword() {
  const t = useT();
  const { token, error: linkError } = Route.useSearch();
  const navigate = useNavigate();
  const [error, setError] = useState<string>();
  const [pending, setPending] = useState(false);
  const [done, setDone] = useState(false);
  const toSignIn = () => void navigate({ to: "/sign-in" });

  if (!token || linkError) {
    return (
      <AuthScreen title={t("site.verified.invalidTitle")}>
        <AuthText>{t("site.reset.invalid")}</AuthText>
        <Link to="/forgot-password" className={actionClass()}>
          {t("site.forgot.send")}
        </Link>
      </AuthScreen>
    );
  }
  if (done) {
    return (
      <AuthScreen title={t("site.reset.title")}>
        <Notice tone="success">{t("site.reset.done")}</Notice>
        <Action onClick={toSignIn}>{t("site.action.signIn")}</Action>
      </AuthScreen>
    );
  }
  return (
    <AuthScreen title={t("site.reset.title")} subtitle={t("site.reset.subtitle")}>
      <AuthForm
        onSubmit={async (f) => {
          setPending(true);
          const res = await authClient.resetPassword({ token, newPassword: String(f.get("password")) });
          setPending(false);
          if (res.error) setError(describe(t, res.error));
          else setDone(true);
        }}
      >
        <AuthField
          label={t("site.field.newPassword")}
          name="password"
          type="password"
          autoComplete="new-password"
          placeholder={t("site.field.passwordHint")}
          minLength={8}
          autoFocus
          required
        />
        <Notice>{error}</Notice>
        <Action type="submit" disabled={pending}>
          {t("site.reset.submit")}
        </Action>
      </AuthForm>
    </AuthScreen>
  );
}

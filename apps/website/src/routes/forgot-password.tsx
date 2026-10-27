import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Action, AuthField, AuthForm, AuthScreen, AuthText, Notice } from "../auth-ui.tsx";
import { authClient } from "../lib/auth-client.ts";
import { describe, useRich, useT } from "../lib/locale.tsx";
import { Name } from "../ui.tsx";

// Asking for the link that sets a new password (docs/mavio-dev.md §2). It
// answers alike whether an account uses the address or not.
export const Route = createFileRoute("/forgot-password")({ component: ForgotPassword });

function ForgotPassword() {
  const t = useT();
  const rich = useRich();
  const navigate = useNavigate();
  const [error, setError] = useState<string>();
  const [pending, setPending] = useState(false);
  const [sentTo, setSentTo] = useState<string>();
  const back = () => void navigate({ to: "/sign-in" });

  if (sentTo) {
    return (
      <AuthScreen title={t("site.checkMail.title")} onBack={back}>
        <AuthText>{rich("site.forgot.sent", { email: <Name>{sentTo}</Name> })}</AuthText>
        <Action variant="secondary" onClick={back}>
          {t("site.forgot.backToSignIn")}
        </Action>
      </AuthScreen>
    );
  }
  return (
    <AuthScreen title={t("site.forgot.title")} subtitle={t("site.forgot.subtitle")} onBack={back}>
      <AuthForm
        onSubmit={async (f) => {
          setPending(true);
          const email = String(f.get("email")).trim();
          const res = await authClient.requestPasswordReset({ email, redirectTo: `${window.location.origin}/reset-password` });
          setPending(false);
          if (res.error) setError(describe(t, res.error));
          else setSentTo(email);
        }}
      >
        <AuthField label={t("site.field.email")} name="email" type="email" autoComplete="email" autoFocus required />
        <Notice>{error}</Notice>
        <Action type="submit" disabled={pending}>
          {t("site.forgot.send")}
        </Action>
      </AuthForm>
    </AuthScreen>
  );
}

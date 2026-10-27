import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Action, AuthField, AuthFoot, AuthForm, AuthScreen, authLink, Notice, Or } from "../auth-ui.tsx";
import { authClient } from "../lib/auth-client.ts";
import { describe, useT } from "../lib/locale.tsx";

type Search = { next?: string; step?: "totp" };

/** next as a path of this site, else undefined. */
export function nextOf(v: unknown): string | undefined {
  return typeof v === "string" && v.startsWith("/") && !v.startsWith("//") ? v : undefined;
}

export const Route = createFileRoute("/sign-in")({
  validateSearch: (s: Record<string, unknown>): Search => ({
    next: nextOf(s.next),
    step: s.step === "totp" ? "totp" : undefined,
  }),
  component: SignIn,
});

function SignIn() {
  const t = useT();
  const { next, step } = Route.useSearch();
  const navigate = useNavigate();
  const [error, setError] = useState<string>();
  const [pending, setPending] = useState(false);
  const done = () => {
    window.location.href = next ?? "/";
  };

  if (step === "totp") return <Totp onDone={done} onBack={() => void navigate({ to: "/sign-in", search: { next } })} />;

  async function submit(f: FormData) {
    setPending(true);
    const name = String(f.get("name")).trim();
    const password = String(f.get("password"));
    // An address not verified yet gets the link again (sendOnSignIn).
    const callbackURL = `/verified${next ? `?${new URLSearchParams({ next })}` : ""}`;
    const res = name.includes("@")
      ? await authClient.signIn.email({ email: name, password, callbackURL })
      : await authClient.signIn.username({ username: name, password });
    setPending(false);
    if (res.error) setError(describe(t, res.error));
    else if (res.data && "twoFactorRedirect" in res.data && res.data.twoFactorRedirect) {
      await navigate({ to: "/sign-in", search: { next, step: "totp" } });
    } else done();
  }

  return (
    <AuthScreen title={t("site.signIn.title")} subtitle={t("site.signIn.subtitle")}>
      <AuthForm onSubmit={(f) => void submit(f)}>
        <AuthField label={t("site.field.nameOrEmail")} name="name" autoComplete="username webauthn" autoCapitalize="none" spellCheck={false} required />
        <AuthField
          label={t("site.field.password")}
          name="password"
          type="password"
          autoComplete="current-password"
          required
          aside={
            <Link to="/forgot-password" className="font-semibold text-accent-text hover:underline">
              {t("site.signIn.forgot")}
            </Link>
          }
        />
        <Notice>{error}</Notice>
        <Action type="submit" disabled={pending}>
          {t("site.action.signIn")}
        </Action>
      </AuthForm>
      <Or />
      <Action
        variant="secondary"
        icon="key"
        onClick={async () => {
          const res = await authClient.signIn.passkey();
          if (res?.error) setError(describe(t, res.error));
          else done();
        }}
      >
        {t("site.signIn.passkey")}
      </Action>
      <AuthFoot>
        {t("site.signIn.noAccount")}{" "}
        <Link to="/sign-up" search={{ next }} className={authLink}>
          {t("site.signIn.createOne")}
        </Link>
      </AuthFoot>
    </AuthScreen>
  );
}

function Totp({ onDone, onBack }: { onDone: () => void; onBack: () => void }) {
  const t = useT();
  const [error, setError] = useState<string>();
  return (
    <AuthScreen title={t("site.totp.title")} subtitle={t("site.totp.subtitle")} onBack={onBack}>
      <AuthForm
        onSubmit={async (f) => {
          const res = await authClient.twoFactor.verifyTotp({ code: String(f.get("code")).trim(), trustDevice: true });
          if (res.error) setError(describe(t, res.error));
          else onDone();
        }}
      >
        <AuthField
          label={t("site.field.code")}
          name="code"
          inputMode="numeric"
          autoComplete="one-time-code"
          pattern="[0-9]{6}"
          maxLength={6}
          autoFocus
          required
          className="tracking-[0.3em]"
        />
        <Notice>{error}</Notice>
        <Action type="submit">{t("site.totp.verify")}</Action>
      </AuthForm>
    </AuthScreen>
  );
}

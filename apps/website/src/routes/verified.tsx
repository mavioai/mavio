import { createFileRoute, Link } from "@tanstack/react-router";

import { Action, actionClass, AuthScreen, AuthText, Notice } from "../auth-ui.tsx";
import { useT } from "../lib/locale.tsx";
import { nextOf } from "./sign-in.tsx";

// Where the verification link of an e-mail leads (docs/mavio-dev.md §2):
// better-auth verified the address and signed the account in, or comes
// here with error=invalid_token for a link used or expired.
type Search = { next?: string; error?: string };

export const Route = createFileRoute("/verified")({
  validateSearch: (s: Record<string, unknown>): Search => ({
    next: nextOf(s.next),
    error: typeof s.error === "string" ? s.error : undefined,
  }),
  component: Verified,
});

function Verified() {
  const t = useT();
  const { next, error } = Route.useSearch();
  if (error) {
    return (
      <AuthScreen title={t("site.verified.invalidTitle")}>
        <AuthText>{t("site.verified.invalidBody")}</AuthText>
        <Link to="/sign-in" search={{ next }} className={actionClass()}>
          {t("site.action.signIn")}
        </Link>
      </AuthScreen>
    );
  }
  return (
    <AuthScreen title={t("site.verified.title")}>
      <Notice tone="success">{t("site.verified.body")}</Notice>
      <Action onClick={() => (window.location.href = next ?? "/")}>{t("site.action.continue")}</Action>
    </AuthScreen>
  );
}

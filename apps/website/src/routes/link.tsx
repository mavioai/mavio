import { useMutation } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Action, AuthField, AuthForm, AuthScreen, AuthText, Notice, SignedInAs } from "../auth-ui.tsx";
import { accounts } from "../lib/api.ts";
import { describe, useRich, useT } from "../lib/locale.tsx";
import { useAccount } from "../lib/session.ts";
import { Loading, Name } from "../ui.tsx";

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
  const t = useT();
  const rich = useRich();
  const me = useAccount();
  const search = Route.useSearch();
  const navigate = useNavigate();
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
  if (!me) return <Loading />;

  if (done) {
    return (
      <AuthScreen title={t("site.link.title")}>
        <Notice tone={done === "approved" ? "success" : "danger"}>{t(done === "approved" ? "site.link.approved" : "site.link.denied")}</Notice>
        <Action variant="secondary" onClick={() => void navigate({ to: "/" })}>
          {t("site.nav.servers")}
        </Action>
      </AuthScreen>
    );
  }
  if (code && check.data) {
    const shown = <Name>{code.toUpperCase()}</Name>;
    return (
      <AuthScreen title={t("site.link.title")} onBack={() => setCode(undefined)}>
        <AuthText>
          {check.data.serverName
            ? rich("site.link.confirmServer", { server: <Name>{check.data.serverName}</Name>, code: shown })
            : rich("site.link.confirmApp", { code: shown })}
        </AuthText>
        <SignedInAs me={me} />
        <div className="flex flex-col gap-3">
          <Action onClick={() => answer.mutate(true)} disabled={answer.isPending}>
            {t("site.action.signIn")}
          </Action>
          <Action variant="secondary" onClick={() => answer.mutate(false)} disabled={answer.isPending}>
            {t("site.action.cancel")}
          </Action>
        </div>
        <Notice>{describe(t, answer.error)}</Notice>
      </AuthScreen>
    );
  }
  return (
    <AuthScreen title={t("site.link.title")} subtitle={t("site.link.subtitle")}>
      <AuthForm onSubmit={(f) => check.mutate(String(f.get("code")).trim())}>
        <AuthField
          label={t("site.field.code")}
          name="code"
          defaultValue={search.code}
          autoComplete="off"
          autoCapitalize="characters"
          spellCheck={false}
          autoFocus
          required
          className="text-center text-[22px] font-semibold tracking-[0.25em] uppercase"
        />
        <Notice>{describe(t, check.error)}</Notice>
        <Action type="submit" disabled={check.isPending}>
          {t("site.action.continue")}
        </Action>
      </AuthForm>
    </AuthScreen>
  );
}

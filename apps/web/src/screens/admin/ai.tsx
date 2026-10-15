// The settings' AI (docs/clients.md §6.3.13): the providers of the AI
// gateway plugin, each with where its credential comes from; a provider's
// API key, its OAuth login followed step by step, signing out, and its
// models.
import {
  errorMessage,
  isNoAiGateway,
  type ServerEntry,
  useAiActions,
  useAiLogin,
  useAiModels,
  useAiProviders,
} from "@mavio/client-ts";
import { AiCredential, AiLoginPromptKind, type AiLoginEvent, AiLoginState, type AiModel, type AiProvider } from "@mavio/proto/mavio/plugin/v1/ai_pb";
import { useSearch } from "@tanstack/react-router";
import { useState } from "react";

import { locale, t } from "../../i18n.ts";
import { Button, Group, Row, Select } from "../../ui.tsx";
import type { SettingsSearch } from "../settings.tsx";
import { BackLink, Confirm, input, Page, PageLink } from "./common.tsx";

/** The section of the server chosen in the settings. */
export function AiSection({ server }: { server: ServerEntry }) {
  const { id } = useSearch({ strict: false }) as SettingsSearch;
  return id ? <ProviderPage key={id} server={server} id={id} /> : <ProvidersList server={server} />;
}

/** Where a provider's credential comes from, or that it has none. */
function credentialOf(p: AiProvider) {
  if (!p.available) return t("ai.notSignedIn");
  if (p.credential === AiCredential.OAUTH) return t("ai.signedIn", { label: p.oauthLabel || p.name });
  if (p.credential === AiCredential.API_KEY) return t("ai.apiKeyStored");
  return t("ai.fromSource", { source: p.source });
}

function ProvidersList({ server }: { server: ServerEntry }) {
  const providers = useAiProviders(server);
  const models = useAiModels(server, true);
  const sorted = (providers.data ?? []).toSorted((a, b) => Number(b.available) - Number(a.available) || a.name.localeCompare(b.name, locale));
  const count = (p: AiProvider) => (models.data ?? []).filter((m) => m.provider === p.id).length;
  if (providers.isError && isNoAiGateway(providers.error))
    return (
      <Page title={t("settings.ai")} message={t("ai.message")}>
        <Group footer={t("ai.noGatewayFooter")}>
          <PageLink search={{}} section="plugins" icon="plugins" color="#AF52DE" label={t("ai.noGateway")} detail={t("ai.noGatewayDetail")} />
        </Group>
      </Page>
    );
  return (
    <Page title={t("settings.ai")} message={t("ai.message")}>
      <Group title={t("ai.providers")} footer={t("ai.providersFooter")}>
        {sorted.map((p) => (
          <PageLink
            key={p.id}
            search={{ id: p.id }}
            icon="key"
            color={p.available ? "#34C759" : "#8E8E93"}
            label={p.name}
            detail={[credentialOf(p), p.available && t("ai.modelCount", { count: count(p) }), p.custom && t("ai.custom")].filter(Boolean).join(" · ")}
          />
        ))}
        {providers.isSuccess && sorted.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("ai.noProviders")}</p>}
        {providers.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(providers.error)}</p>}
      </Group>
    </Page>
  );
}

function ProviderPage({ server, id }: { server: ServerEntry; id: string }) {
  const providers = useAiProviders(server);
  const p = providers.data?.find((x) => x.id === id);
  const back = <BackLink section="ai" label={t("settings.ai")} />;
  if (!p)
    return (
      <Page title={id} back={back}>
        {providers.isError && <p className="type-callout text-danger">{errorMessage(providers.error)}</p>}
        {providers.isSuccess && <p className="type-callout text-text-secondary">{t("ai.providerGone")}</p>}
      </Page>
    );
  return (
    <Page title={p.name} message={credentialOf(p)} back={back}>
      {p.oauth && <OAuthLogin server={server} provider={p} />}
      {p.apiKey && <ApiKey server={server} provider={p} />}
      {p.credential !== AiCredential.UNSPECIFIED && <SignOut server={server} provider={p} />}
      <Models server={server} provider={p} />
    </Page>
  );
}

function ApiKey({ server, provider }: { server: ServerEntry; provider: AiProvider }) {
  const { setApiKey } = useAiActions(server);
  const [key, setKey] = useState("");
  const [saved, setSaved] = useState(false);
  return (
    <Group title={t("ai.apiKey")} footer={provider.credential === AiCredential.API_KEY ? t("ai.apiKeyReplaceFooter") : t("ai.apiKeyFooter")}>
      <Row label={t("ai.apiKey")}>
        <input
          type="password"
          autoComplete="off"
          className={`${input} sm:w-72`}
          aria-label={t("ai.apiKey")}
          value={key}
          placeholder={provider.credential === AiCredential.API_KEY ? "••••••••" : ""}
          onChange={(e) => (setKey(e.target.value), setSaved(false))}
        />
      </Row>
      <div className="flex flex-wrap items-center gap-3 px-4 py-2.5">
        <Button
          disabled={!key.trim() || setApiKey.isPending}
          onClick={() => setApiKey.mutate({ provider: provider.id, key: key.trim() }, { onSuccess: () => (setKey(""), setSaved(true)) })}
        >
          {setApiKey.isPending ? "…" : t("action.save")}
        </Button>
        {saved && <span className="type-callout text-text-secondary">{t("ai.apiKeySaved")}</span>}
        {setApiKey.isError && <span className="type-callout break-words text-danger">{errorMessage(setApiKey.error)}</span>}
      </div>
    </Group>
  );
}

function SignOut({ server, provider }: { server: ServerEntry; provider: AiProvider }) {
  const { logout } = useAiActions(server);
  const [confirming, setConfirming] = useState(false);
  const action = provider.credential === AiCredential.OAUTH ? t("ai.signOut") : t("ai.removeKey");
  if (confirming)
    return (
      <Confirm
        title={action}
        message={logout.isError ? errorMessage(logout.error) : t("ai.signOutFooter")}
        action={action}
        busy={logout.isPending}
        onCancel={() => setConfirming(false)}
        onConfirm={() => logout.mutate(provider.id, { onSuccess: () => setConfirming(false) })}
      />
    );
  return (
    <div>
      <Button variant="plain" className="text-danger" onClick={() => setConfirming(true)}>
        {action}
      </Button>
    </div>
  );
}

/** A provider's OAuth login: started here, followed as it shows pages and codes and asks questions, until it ends. */
function OAuthLogin({ server, provider }: { server: ServerEntry; provider: AiProvider }) {
  const actions = useAiActions(server);
  const [loginId, setLoginId] = useState<string>();
  const { login, error } = useAiLogin(server, loginId);
  const running = !!loginId && (!login || login.state === AiLoginState.RUNNING) && !error;
  const label = provider.oauthLabel || provider.name;
  const start = () => (actions.answer.reset(), actions.startLogin.mutate(provider.id, { onSuccess: setLoginId }));
  return (
    <Group title={t("ai.oauth")} footer={provider.subscription ? t("ai.subscriptionFooter") : undefined}>
      {!running && (
        <Row label={t("ai.signInWith", { label })} detail={provider.credential === AiCredential.OAUTH ? t("ai.signInAgain") : undefined}>
          <Button disabled={actions.startLogin.isPending} onClick={start}>
            {actions.startLogin.isPending ? "…" : t("ai.signIn")}
          </Button>
        </Row>
      )}
      {loginId && login && login.events.map((e, i) => <LoginEvent key={i} event={e} />)}
      {running && login?.prompt && (
        <LoginPrompt
          key={login.prompt.id}
          prompt={login.prompt}
          busy={actions.answer.isPending}
          onAnswer={(value) => login.prompt && actions.answer.mutate({ loginId, promptId: login.prompt.id, value })}
        />
      )}
      {running && (
        <div className="flex flex-wrap items-center gap-3 px-4 py-2.5">
          <span className="type-callout text-text-secondary">{t("ai.waiting")}</span>
          <Button variant="plain" onClick={() => loginId && actions.cancel.mutate(loginId, { onSuccess: () => setLoginId(undefined) })}>
            {t("action.cancel")}
          </Button>
        </div>
      )}
      {login?.state === AiLoginState.SUCCEEDED && <p className="px-4 py-3 type-callout text-text-secondary">{t("ai.loginSucceeded")}</p>}
      {login?.state === AiLoginState.FAILED && <p className="px-4 py-3 type-callout break-words text-danger">{login.error || t("ai.loginFailed")}</p>}
      {login?.state === AiLoginState.CANCELED && <p className="px-4 py-3 type-callout text-text-secondary">{t("ai.loginCanceled")}</p>}
      {[error, actions.startLogin.error, actions.answer.error, actions.cancel.error]
        .filter((e) => e != null)
        .map((e, i) => (
          <p key={i} className="px-4 py-3 type-callout break-words text-danger">
            {errorMessage(e)}
          </p>
        ))}
    </Group>
  );
}

const link = "break-all text-accent-text underline-offset-2 hover:underline";

function LoginEvent({ event }: { event: AiLoginEvent }) {
  const e = event.event;
  switch (e.case) {
    case "info":
      return (
        <div className="flex flex-col gap-1 px-4 py-2.5 type-callout">
          <p className="break-words whitespace-pre-wrap">{e.value.message}</p>
          {e.value.links.map((l) => (
            <a key={l.url} href={l.url} target="_blank" rel="noreferrer" className={link}>
              {l.label || l.url}
            </a>
          ))}
        </div>
      );
    case "url":
      return (
        <div className="flex flex-col gap-1.5 px-4 py-2.5 type-callout">
          {e.value.instructions && <p className="break-words whitespace-pre-wrap">{e.value.instructions}</p>}
          <a
            href={e.value.url}
            target="_blank"
            rel="noreferrer"
            className="inline-flex min-h-11 items-center self-start rounded-md bg-fill px-4 type-headline text-on-background transition hover:bg-fill-selected"
          >
            {t("ai.openPage")}
          </a>
          <span className="type-footnote break-all text-text-secondary">{e.value.url}</span>
        </div>
      );
    case "deviceCode":
      return (
        <div className="flex flex-col gap-1.5 px-4 py-2.5 type-callout">
          <p>{t("ai.enterCode")}</p>
          <code className="self-start rounded-md bg-surface-raised px-3 py-1.5 font-mono type-title3 tracking-widest select-all">{e.value.userCode}</code>
          <a href={e.value.verificationUri} target="_blank" rel="noreferrer" className={link}>
            {e.value.verificationUri}
          </a>
        </div>
      );
    case "progress":
      return <p className="px-4 py-2.5 type-callout text-text-secondary">{e.value}</p>;
  }
  return null;
}

function LoginPrompt({
  prompt,
  busy,
  onAnswer,
}: {
  prompt: { kind: AiLoginPromptKind; message: string; placeholder: string; options: { id: string; label: string; description: string }[] };
  busy: boolean;
  onAnswer: (value: string) => void;
}) {
  const [value, setValue] = useState(prompt.kind === AiLoginPromptKind.SELECT ? (prompt.options[0]?.id ?? "") : "");
  const label = prompt.message || t("ai.answer");
  return (
    <div className="flex flex-col gap-2 px-4 py-2.5">
      <p className="type-body break-words whitespace-pre-wrap">{label}</p>
      {prompt.kind === AiLoginPromptKind.SELECT ? (
        <Select value={value} label={label} className="self-start" options={prompt.options.map((o) => ({ value: o.id, label: o.description ? `${o.label} — ${o.description}` : o.label }))} onChange={setValue} />
      ) : (
        <input
          type={prompt.kind === AiLoginPromptKind.SECRET ? "password" : "text"}
          autoComplete="off"
          className={input}
          aria-label={label}
          placeholder={prompt.placeholder}
          value={value}
          onChange={(e) => setValue(e.target.value)}
        />
      )}
      {prompt.kind === AiLoginPromptKind.MANUAL_CODE && <p className="type-footnote text-text-secondary">{t("ai.manualCode")}</p>}
      <Button className="self-start" disabled={!value.trim() || busy} onClick={() => onAnswer(value.trim())}>
        {busy ? "…" : t("ai.submit")}
      </Button>
    </div>
  );
}

const tokens = new Intl.NumberFormat([locale], { notation: "compact" });
const dollars = new Intl.NumberFormat([locale], { style: "currency", currency: "USD", maximumFractionDigits: 2 });

function modelDetail(m: AiModel) {
  return [
    m.id !== m.name && m.id,
    m.contextWindow > 0n && t("ai.contextWindow", { tokens: tokens.format(m.contextWindow) }),
    m.reasoning && t("ai.reasoning"),
    m.images && t("ai.images"),
    m.cost && (m.cost.input > 0 || m.cost.output > 0) && t("ai.cost", { input: dollars.format(m.cost.input), output: dollars.format(m.cost.output) }),
  ]
    .filter(Boolean)
    .join(" · ");
}

function Models({ server, provider }: { server: ServerEntry; provider: AiProvider }) {
  const models = useAiModels(server, false, provider.id);
  return (
    <Group title={t("ai.models")} footer={provider.available ? undefined : t("ai.modelsFooter")}>
      {(models.data ?? []).map((m) => (
        <div key={m.id} className="px-4 py-2.5">
          <div className="type-body">{m.name || m.id}</div>
          <div className="type-footnote break-words text-text-secondary">{modelDetail(m)}</div>
        </div>
      ))}
      {models.isSuccess && models.data.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("ai.noModels")}</p>}
      {models.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(models.error)}</p>}
    </Group>
  );
}

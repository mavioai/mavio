// The settings' AI Conversations (docs/clients.md §6.3.26): the
// administrators' conversations with the models of the AI providers, a new
// one started with a model, instructions and its first message, and a
// conversation's messages with the answer streamed as the model writes it.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  isNoAiGateway,
  type ServerEntry,
  useAiConversation,
  useAiConversationActions,
  useAiConversations,
  useAiModels,
  useAiProviders,
  useAiSend,
} from "@mavio/client-ts";
import { type AgentSession, type AiModel, AiRole, type AiUsage } from "@mavio/proto/mavio/plugin/v1/ai_pb";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { type KeyboardEvent, useEffect, useRef, useState } from "react";

import { locale, t } from "../../i18n.ts";
import { modelDetail } from "../../ai-model.ts";
import { Button, Group, Row } from "../../ui.tsx";
import { ModelLabel, type ModelOption, ModelPicker } from "../../controls/model-picker.tsx";
import type { SettingsSearch } from "../settings.tsx";
import { BackLink, fromNow, Page, PageLink } from "./common.tsx";
import { input } from "../../controls/styles.ts";
import { Confirm } from "../../controls/dialog.tsx";

/** A message to send once its new conversation shows. */
interface First {
  id: string;
  text: string;
}

/** The section of the server chosen in the settings. */
export function ConversationsSection({ server }: { server: ServerEntry }) {
  const { id } = useSearch({ strict: false }) as SettingsSearch;
  const [first, setFirst] = useState<First>();
  return id ? (
    <ConversationPage key={id} server={server} id={id} first={first?.id === id ? first.text : undefined} onFirstSent={() => setFirst(undefined)} />
  ) : (
    <ConversationsList server={server} onStarted={setFirst} />
  );
}

const dollars = new Intl.NumberFormat(locale, { style: "currency", currency: "USD", maximumSignificantDigits: 3 });
const count = new Intl.NumberFormat(locale);

function usageOf(u: AiUsage | undefined): string {
  return t("conversations.usage", {
    input: count.format(u?.inputTokens ?? 0n),
    output: count.format(u?.outputTokens ?? 0n),
    cost: dollars.format(u?.cost ?? 0),
  });
}

function ConversationsList({ server, onStarted }: { server: ServerEntry; onStarted: (f: First) => void }) {
  const conversations = useAiConversations(server);
  if (conversations.isError && isNoAiGateway(conversations.error))
    return (
      <Page title={t("settings.conversations")} message={t("conversations.message")}>
        <Group footer={t("conversations.noPluginFooter")}>
          <PageLink search={{}} section="plugins" icon="plugins" color="#AF52DE" label={t("conversations.noPlugin")} detail={t("ai.noGatewayDetail")} />
        </Group>
      </Page>
    );
  return (
    <Page title={t("settings.conversations")} message={t("conversations.message")}>
      <NewConversation server={server} onStarted={onStarted} />
      <Group title={t("conversations.list")}>
        {(conversations.data ?? []).map((c) => (
          <PageLink
            key={c.id}
            search={{ id: c.id }}
            label={c.title || t("conversations.untitled")}
            detail={[c.model, c.updated && fromNow(timestampDate(c.updated)), usageOf(c.usage)].filter(Boolean).join(" · ")}
          />
        ))}
        {conversations.isSuccess && conversations.data.length === 0 && (
          <p className="px-4 py-3 type-callout text-text-secondary">{t("conversations.none")}</p>
        )}
        {conversations.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(conversations.error)}</p>}
      </Group>
    </Page>
  );
}

/** Sends a field's text with Enter; Shift+Enter starts a new line. */
function sendOnEnter(send: () => void) {
  return (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key !== "Enter" || e.shiftKey || e.nativeEvent.isComposing) return;
    e.preventDefault();
    send();
  };
}

/** The models of a server's providers as the model picker lists them, by `provider/model`. */
function useModelOptions(server: ServerEntry): { options: ModelOption[]; loaded: boolean } {
  const models = useAiModels(server, true);
  const providers = useAiProviders(server);
  const names = new Map((providers.data ?? []).map((p) => [p.id, p.name]));
  const options = (models.data ?? []).map((m: AiModel) => ({
    value: `${m.provider}/${m.id}`,
    label: m.name || m.id,
    detail: modelDetail(m),
    provider: m.provider,
    providerName: names.get(m.provider) ?? m.provider,
  }));
  return { options, loaded: models.isSuccess };
}

/** A field for a message, its button sending it at its bottom end. */
function Composer({
  value,
  onChange,
  onSend,
  label,
  action,
  busy,
  disabled,
}: {
  value: string;
  onChange: (text: string) => void;
  onSend: () => void;
  label: string;
  action: string;
  busy: boolean;
  disabled?: boolean;
}) {
  return (
    <div className="flex flex-col rounded-md border border-border bg-surface focus-within:border-focus">
      <textarea
        className="min-h-16 w-full min-w-0 resize-none bg-transparent px-3 pt-2 type-callout text-on-background placeholder:text-text-tertiary focus:outline-none"
        rows={2}
        aria-label={label}
        placeholder={label}
        maxLength={100000}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={sendOnEnter(onSend)}
      />
      <div className="flex justify-end px-1.5 pb-1.5">
        <button
          type="button"
          disabled={disabled || !value.trim() || busy}
          onClick={onSend}
          className="min-h-8 rounded-full bg-button px-3.5 type-footnote font-semibold text-on-button hover:opacity-85 disabled:opacity-40 desk:min-h-7"
        >
          {busy ? "…" : action}
        </button>
      </div>
    </div>
  );
}

function NewConversation({ server, onStarted }: { server: ServerEntry; onStarted: (f: First) => void }) {
  const { options, loaded } = useModelOptions(server);
  const { create } = useAiConversationActions(server);
  const navigate = useNavigate();
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const [model, setModel] = useState("");
  const [instructions, setInstructions] = useState("");
  const [text, setText] = useState("");
  const chosen = model || options[0]?.value || "";
  const placeholder = t("conversations.placeholder", { model: options.find((o) => o.value === chosen)?.label ?? "" });
  const start = () => {
    const message = text.trim();
    if (!chosen || !message || create.isPending) return;
    const at = chosen.indexOf("/");
    create.mutate(
      { provider: chosen.slice(0, at), model: chosen.slice(at + 1), instructions: instructions.trim(), title: message.split("\n")[0].slice(0, 80) },
      {
        onSuccess: (c) => {
          onStarted({ id: c.id, text: message });
          void navigate({ replace: true, to: "/settings/$section", params: { section: "conversations" }, search: { server: cur.server, id: c.id } });
        },
      },
    );
  };
  if (loaded && options.length === 0)
    return (
      <Group title={t("conversations.new")}>
        <p className="px-4 py-3 type-callout text-text-secondary">{t("conversations.noModels")}</p>
      </Group>
    );
  return (
    <Group title={t("conversations.new")} footer={t("conversations.newFooter")}>
      <Row label={t("conversations.model")}>
        <ModelPicker
          value={chosen}
          label={t("conversations.model")}
          options={options}
          className="w-full sm:w-72"
          labels={{ placeholder: t("conversations.model"), search: t("nav.search"), empty: t("schema.noMatches") }}
          onChange={setModel}
        />
      </Row>
      <div className="flex flex-col gap-3 px-4 py-2.5">
        <textarea
          className={`${input} min-h-16 py-2`}
          aria-label={t("conversations.instructions")}
          placeholder={`${t("conversations.instructions")} (${t("conversations.instructionsPlaceholder")})`}
          maxLength={20000}
          value={instructions}
          onChange={(e) => setInstructions(e.target.value)}
        />
        <Composer
          value={text}
          onChange={setText}
          onSend={start}
          label={placeholder}
          action={t("conversations.start")}
          busy={create.isPending}
          disabled={!chosen}
        />
        {create.isError && <p className="type-callout break-words text-danger">{errorMessage(create.error)}</p>}
      </div>
    </Group>
  );
}

/**
 * A message: the user's on the right, the model's on the left under the
 * model as the model picker's rows show it; each short of the other side,
 * so that the two stand apart.
 */
function Bubble({ who, text, pending }: { who?: ModelOption; text: string; pending?: boolean }) {
  const user = !who;
  return (
    <li className={`flex flex-col gap-1 ${user ? "items-end ps-[15%]" : "items-start pe-[15%]"}`}>
      {who ? (
        <span className="flex min-w-0 max-w-full items-center gap-2 px-1 type-callout">
          <ModelLabel option={who} />
        </span>
      ) : (
        <span className="px-1 type-caption text-text-tertiary">{t("conversations.you")}</span>
      )}
      <div
        className={`max-w-[min(100%,42rem)] rounded-lg px-3 py-2 type-callout break-words whitespace-pre-wrap ${user ? "bg-button text-on-button" : "bg-surface"}`}
      >
        {text || (pending ? "…" : "")}
      </div>
    </li>
  );
}

function ConversationPage({ server, id, first, onFirstSent }: { server: ServerEntry; id: string; first?: string; onFirstSent: () => void }) {
  const conversation = useAiConversation(server, id);
  const { remove } = useAiConversationActions(server);
  const { exchange, send, retry, sending } = useAiSend(server, id);
  const { options } = useModelOptions(server);
  const navigate = useNavigate();
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const [text, setText] = useState("");
  const [confirming, setConfirming] = useState(false);
  const end = useRef<HTMLDivElement>(null);

  // The new conversation's first message, sent once.
  const sent = useRef(false);
  useEffect(() => {
    if (!first || sent.current) return;
    sent.current = true;
    onFirstSent();
    void send(first);
  }, [first, onFirstSent, send]);

  const messages = conversation.data?.messages ?? [];
  useEffect(() => {
    end.current?.scrollIntoView({ block: "end" });
  }, [messages.length, exchange?.answer, exchange?.text]);

  const back = <BackLink section="conversations" label={t("settings.conversations")} />;
  const c: AgentSession | undefined = conversation.data?.conversation;
  if (!c)
    return (
      <Page title={t("settings.conversations")} back={back}>
        {conversation.isError && <p className="type-callout break-words text-danger">{errorMessage(conversation.error)}</p>}
        {conversation.isSuccess && <p className="type-callout text-text-secondary">{t("conversations.gone")}</p>}
      </Page>
    );
  // The conversation's model as the picker lists it, else by its ID.
  const model: ModelOption = options.find((o) => o.value === `${c.provider}/${c.model}`) ?? {
    value: `${c.provider}/${c.model}`,
    label: c.model,
    provider: c.provider,
    providerName: c.provider,
  };
  const submit = () => {
    const message = text.trim();
    if (!message || sending) return;
    setText("");
    void send(message);
  };
  return (
    <Page
      title={c.title || t("conversations.untitled")}
      message={`${c.model} (${c.provider}) · ${usageOf(c.usage)}`}
      back={back}
    >
      {confirming && (
        <Confirm
          title={t("conversations.deleteConfirm")}
          message={t("conversations.deleteMessage")}
          action={t("conversations.delete")}
          busy={remove.isPending}
          onCancel={() => setConfirming(false)}
          onConfirm={() =>
            remove.mutate(id, {
              onSuccess: () => void navigate({ replace: true, to: "/settings/$section", params: { section: "conversations" }, search: { server: cur.server } }),
            })
          }
        />
      )}
      {remove.isError && <p className="type-callout break-words text-danger">{errorMessage(remove.error)}</p>}
      <ul className="flex flex-col gap-4" aria-live="polite">
        {messages.map((m, i) => (
          <Bubble key={i} who={m.role === AiRole.USER ? undefined : model} text={m.text} />
        ))}
        {/* A failed message shows as the conversation kept it. */}
        {exchange && exchange.error === undefined && (
          <>
            <Bubble text={exchange.text} />
            <Bubble who={model} text={exchange.answer} pending />
          </>
        )}
      </ul>
      {exchange?.error !== undefined && (
        <div className="flex flex-wrap items-center gap-3">
          <p className="type-callout break-words text-danger">{errorMessage(exchange.error)}</p>
          <Button variant="secondary" onClick={() => void retry()}>
            {t("conversations.retry")}
          </Button>
        </div>
      )}
      <div ref={end}>
        <Composer
          value={text}
          onChange={setText}
          onSend={submit}
          label={t("conversations.placeholder", { model: model.label })}
          action={t("conversations.send")}
          busy={sending}
        />
      </div>
      <div>
        <Button variant="secondary" className="text-danger" onClick={() => setConfirming(true)}>
          {t("conversations.delete")}
        </Button>
      </div>
    </Page>
  );
}

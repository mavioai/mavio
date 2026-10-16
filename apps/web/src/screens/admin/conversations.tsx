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
  useAiSend,
} from "@mavio/client-ts";
import { type AgentSession, AiRole, type AiUsage } from "@mavio/proto/mavio/plugin/v1/ai_pb";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { type KeyboardEvent, useEffect, useRef, useState } from "react";

import { locale, t } from "../../i18n.ts";
import { Button, Group, Row } from "../../ui.tsx";
import { Select } from "../../controls/select.tsx";
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

function NewConversation({ server, onStarted }: { server: ServerEntry; onStarted: (f: First) => void }) {
  const models = useAiModels(server, true);
  const { create } = useAiConversationActions(server);
  const navigate = useNavigate();
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const [model, setModel] = useState("");
  const [instructions, setInstructions] = useState("");
  const [text, setText] = useState("");
  const options = (models.data ?? []).map((m) => ({ value: `${m.provider}/${m.id}`, label: `${m.name || m.id} (${m.provider})` }));
  const chosen = model || options[0]?.value || "";
  const name = (models.data ?? []).find((m) => `${m.provider}/${m.id}` === chosen);
  const placeholder = t("conversations.placeholder", { model: name?.name || name?.id || "" });
  const start = () => {
    const message = text.trim();
    if (!chosen || !message || create.isPending) return;
    const at = chosen.indexOf("/");
    create.mutate(
      { provider: chosen.slice(0, at), model: chosen.slice(at + 1), instructions: instructions.trim(), title: message.split("\n")[0].slice(0, 80) },
      {
        onSuccess: (c) => {
          onStarted({ id: c.id, text: message });
          void navigate({ to: "/settings/$section", params: { section: "conversations" }, search: { server: cur.server, id: c.id } });
        },
      },
    );
  };
  if (models.isSuccess && options.length === 0)
    return (
      <Group title={t("conversations.new")}>
        <p className="px-4 py-3 type-callout text-text-secondary">{t("conversations.noModels")}</p>
      </Group>
    );
  return (
    <Group title={t("conversations.new")} footer={t("conversations.newFooter")}>
      <Row label={t("conversations.model")}>
        <Select value={chosen} label={t("conversations.model")} options={options} onChange={setModel} />
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
        <textarea
          className={`${input} min-h-20 py-2`}
          aria-label={placeholder}
          placeholder={placeholder}
          maxLength={100000}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={sendOnEnter(start)}
        />
        <div className="flex flex-wrap items-center gap-3">
          <Button disabled={!chosen || !text.trim() || create.isPending} onClick={start}>
            {create.isPending ? "…" : t("conversations.start")}
          </Button>
          {create.isError && <span className="type-callout break-words text-danger">{errorMessage(create.error)}</span>}
        </div>
      </div>
    </Group>
  );
}

/** A message: the user's on the right, the model's on the left. */
function Bubble({ user, who, text, pending }: { user: boolean; who: string; text: string; pending?: boolean }) {
  return (
    <li className={`flex flex-col gap-1 ${user ? "items-end" : "items-start"}`}>
      <span className="px-1 type-caption1 text-text-secondary">{who}</span>
      <div
        className={`max-w-[min(100%,42rem)] rounded-lg px-3.5 py-2.5 type-body break-words whitespace-pre-wrap ${user ? "bg-accent text-on-accent" : "bg-surface"}`}
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
      actions={
        <Button variant="plain" className="text-danger" onClick={() => setConfirming(true)}>
          {t("conversations.delete")}
        </Button>
      }
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
              onSuccess: () => void navigate({ to: "/settings/$section", params: { section: "conversations" }, search: { server: cur.server } }),
            })
          }
        />
      )}
      {remove.isError && <p className="type-callout break-words text-danger">{errorMessage(remove.error)}</p>}
      <ul className="flex flex-col gap-4" aria-live="polite">
        {messages.map((m, i) => (
          <Bubble key={i} user={m.role === AiRole.USER} who={m.role === AiRole.USER ? t("conversations.you") : c.model} text={m.text} />
        ))}
        {/* A failed message shows as the conversation kept it. */}
        {exchange && exchange.error === undefined && (
          <>
            <Bubble user who={t("conversations.you")} text={exchange.text} />
            <Bubble user={false} who={c.model} text={exchange.answer} pending />
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
      <div ref={end} className="flex items-end gap-2">
        <textarea
          className={`${input} min-h-11 flex-1 py-2`}
          rows={2}
          aria-label={t("conversations.placeholder", { model: c.model })}
          placeholder={t("conversations.placeholder", { model: c.model })}
          maxLength={100000}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={sendOnEnter(submit)}
        />
        <Button disabled={!text.trim() || sending} onClick={submit}>
          {sending ? "…" : t("conversations.send")}
        </Button>
      </div>
    </Page>
  );
}

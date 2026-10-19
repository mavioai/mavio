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
import { useWindowVirtualizer } from "@tanstack/react-virtual";
import { type KeyboardEvent, useEffect, useLayoutEffect, useRef, useState } from "react";

import { locale, t } from "../../i18n.ts";
import { modelDetail } from "../../ai-model.ts";
import { Button, Group, Row, Spinner } from "../../ui.tsx";
import { ModelLabel, type ModelOption, ModelPicker } from "../../controls/model-picker.tsx";
import type { SettingsSearch } from "../settings.tsx";
import { BackLink, fromNow, Page, PageLink } from "./common.tsx";
import { input } from "../../controls/styles.ts";
import { Confirm } from "../../controls/dialog.tsx";
import { Tip } from "../../controls/tooltip.tsx";
import { Icon } from "../../icon.tsx";
import { useDictation } from "../../assistant/dictation.ts";

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
  const { options } = useModelOptions(server);
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
            detail={[modelName(modelOf(options, c)), c.updated && fromNow(timestampDate(c.updated)), usageOf(c.usage)].filter(Boolean).join(" · ")}
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

/** A conversation's model as the picker lists it, else by its IDs, as for a model its provider no longer has. */
function modelOf(options: ModelOption[], c: AgentSession): ModelOption {
  const value = `${c.provider}/${c.model}`;
  return options.find((o) => o.value === value) ?? { value, label: c.model, provider: c.provider, providerName: c.provider };
}

/** A model's name and its provider's, on one line. */
const modelName = (m: ModelOption) => `${m.label} (${m.providerName})`;

/** Text said joined to what was typed, a space between words of scripts that space them. */
function joined(typed: string, said: string): string {
  if (!typed) return said;
  return /[\p{L}\p{N}]$/u.test(typed) && /^[\p{Script=Latin}\p{Script=Cyrillic}\p{Script=Greek}\p{N}]/u.test(said) ? `${typed} ${said}` : typed + said;
}

/**
 * A field for a message, its buttons in a column of their own at its end, at
 * the bottom: dictation over sending. The field grows and shrinks with its
 * text, and from its highest scrolls, the scroll bar at the edge beyond the
 * buttons; while it is dictated to, it is not typed in, and what is heard
 * shows in it as it is said.
 */
function Composer({
  server,
  value,
  onChange,
  onSend,
  label,
  action,
  busy,
  disabled,
}: {
  server: ServerEntry;
  value: string;
  onChange: (text: string) => void;
  onSend: () => void;
  label: string;
  action: string;
  busy: boolean;
  disabled?: boolean;
}) {
  const field = useRef<HTMLTextAreaElement>(null);
  const typed = useRef(value);
  typed.current = value;
  const dictation = useDictation(server, (said) => onChange(joined(typed.current, said)));
  const shown = dictation.listening ? joined(value, dictation.heard) : value;
  // As tall as its text, which the box around it scrolls.
  useLayoutEffect(() => {
    const el = field.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${el.scrollHeight}px`;
  }, [shown]);
  return (
    <div className="flex flex-col gap-1.5">
      <div className="max-h-60 overflow-y-auto rounded-md border border-border bg-surface focus-within:border-focus">
        <div className="flex items-end gap-2 p-1.5 ps-0">
          <textarea
            ref={field}
            className="min-h-16 min-w-0 flex-1 resize-none overflow-hidden bg-transparent ps-3 pt-0.5 type-callout text-on-background placeholder:text-text-tertiary read-only:cursor-default focus:outline-none"
            rows={2}
            aria-label={label}
            placeholder={dictation.listening ? t("assistant.listening") : label}
            maxLength={100000}
            readOnly={dictation.listening}
            value={shown}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={sendOnEnter(() => !dictation.listening && onSend())}
          />
          <div className="sticky bottom-1.5 flex shrink-0 flex-col items-center gap-1.5">
            <Tip label={dictation.listening ? t("assistant.stopListening") : t("conversations.dictate")}>
              <button
                type="button"
                aria-label={dictation.listening ? t("assistant.stopListening") : t("conversations.dictate")}
                aria-pressed={dictation.listening}
                disabled={disabled || busy}
                onClick={dictation.toggle}
                className={`flex size-8 items-center justify-center rounded-full disabled:opacity-40 desk:size-7 ${dictation.listening ? "bg-danger text-white" : "text-text-secondary hover:bg-fill"}`}
              >
                {dictation.listening ? <Voice /> : <Icon name="microphone" size={18} />}
              </button>
            </Tip>
            <Tip label={action}>
              <button
                type="button"
                aria-label={action}
                disabled={disabled || !value.trim() || busy || dictation.listening}
                onClick={onSend}
                className="flex size-8 items-center justify-center rounded-full bg-button text-on-button hover:opacity-85 disabled:opacity-40 desk:size-7"
              >
                {busy ? <Spinner size={16} className="text-on-button" /> : <Icon name="send" size={18} />}
              </button>
            </Tip>
          </div>
        </div>
      </div>
      {dictation.error && (
        <div className="flex items-start gap-1.5">
          <p className="min-w-0 flex-1 type-footnote break-words text-danger">{dictation.error}</p>
          <button
            type="button"
            aria-label={t("remote.dismiss")}
            onClick={dictation.dismiss}
            className="-my-0.5 flex size-5 shrink-0 items-center justify-center rounded-full text-text-secondary hover:bg-fill"
          >
            <Icon name="close" size={14} />
          </button>
        </div>
      )}
    </div>
  );
}

/** Bars rising and falling one after another, while dictation listens. */
function Voice() {
  return (
    <span aria-hidden className="flex h-3.5 items-center gap-[2px]">
      {[0, 0.15, 0.3, 0.45].map((delay) => (
        <span key={delay} className="voice-bar h-full w-[2.5px] rounded-full bg-current" style={{ animationDelay: `${delay}s` }} />
      ))}
    </span>
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
          server={server}
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
function Bubble({
  who,
  text,
  pending,
  index,
  measure,
  top,
}: {
  who?: ModelOption;
  text: string;
  pending?: boolean;
  index: number;
  measure: (el: HTMLElement | null) => void;
  top: number;
}) {
  const user = !who;
  return (
    <li
      ref={measure}
      data-index={index}
      className={`absolute start-0 top-0 flex w-full flex-col gap-1 ${user ? "items-end ps-[15%]" : "items-start pe-[15%]"}`}
      style={{ transform: `translateY(${top}px)` }}
    >
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

/** A message of a conversation, or of the exchange under way. */
interface Said {
  key: string;
  who?: ModelOption;
  text: string;
  pending?: boolean;
}

/** The messages, only those scrolled into view drawn (TanStack Virtual), the page scrolling them. */
function Messages({ said }: { said: Said[] }) {
  const list = useRef<HTMLUListElement>(null);
  // Where the list starts on the page, from which its rows are placed.
  const [margin, setMargin] = useState(0);
  useLayoutEffect(() => {
    const el = list.current;
    if (el) setMargin(el.getBoundingClientRect().top + window.scrollY);
  });
  const virtualizer = useWindowVirtualizer({
    count: said.length,
    estimateSize: () => 96,
    gap: 16,
    overscan: 6,
    scrollMargin: margin,
    getItemKey: (i) => said[i].key,
  });
  return (
    <ul ref={list} className="relative" style={{ height: virtualizer.getTotalSize() }} aria-live="polite">
      {virtualizer.getVirtualItems().map((row) => {
        const m = said[row.index];
        return (
          <Bubble
            key={row.key}
            index={row.index}
            measure={virtualizer.measureElement}
            top={row.start - virtualizer.options.scrollMargin}
            who={m.who}
            text={m.text}
            pending={m.pending}
          />
        );
      })}
    </ul>
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
  const model = modelOf(options, c);
  const submit = () => {
    const message = text.trim();
    if (!message || sending) return;
    setText("");
    void send(message);
  };
  return (
    <Page
      title={c.title || t("conversations.untitled")}
      documentTitle={`${t("settings.conversations")} · ${c.title || t("conversations.untitled")}`}
      message={`${modelName(model)} · ${usageOf(c.usage)}`}
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
      <Messages
        said={[
          ...messages.map((m, i) => ({ key: String(i), who: m.role === AiRole.USER ? undefined : model, text: m.text })),
          // A failed message shows as the conversation kept it.
          ...(exchange && exchange.error === undefined
            ? [
                { key: "sending", text: exchange.text },
                { key: "answer", who: model, text: exchange.answer, pending: true },
              ]
            : []),
        ]}
      />
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
          server={server}
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

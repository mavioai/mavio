// The assistant's settings (docs/assistant.md §9.1, §10): this browser's —
// whether answers are spoken, the voice and which recognizer goes first —
// with the user's memories and conversations on the server; and, in the
// AI settings, the server's for its administrators.
import { clone, create, equals } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  type ServerEntry,
  useAiModels,
  useAssistant,
  useAssistantActions,
  useAssistantConversation,
  useAssistantConversations,
  useAssistantMemories,
  useAssistantSettings,
  useUpdateAssistantSettings,
  useUsers,
} from "@mavio/client-ts";
import {
  AssistantAccess,
  type AssistantSettings,
  AssistantSettingsSchema,
  type AssistantVoiceChoice,
  AssistantVoiceChoiceSchema,
} from "@mavio/proto/mavio/assistant/v1/assistant_pb";
import { useState } from "react";

import { Checkbox, NumberField, Switch } from "../controls/inputs.tsx";
import { Confirm } from "../controls/dialog.tsx";
import { Select } from "../controls/select.tsx";
import { input } from "../controls/styles.ts";
import { useUnsavedGuard } from "../controls/unsaved.tsx";
import { locale, t } from "../i18n.ts";
import { setPrefs, usePrefs } from "../prefs.ts";
import { Button, Group, Row } from "../ui.tsx";
import { browserRecognizes } from "../assistant/listen.ts";
import { patched } from "./admin/common.tsx";

const when = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" });

/** This browser's assistant settings, and the user's memories and conversations on a server. */
export function AssistantPrefs({ server }: { server?: ServerEntry }) {
  const prefs = usePrefs();
  const info = useAssistant(server, locale);
  const voices = info.data?.voices ?? [];
  const speech = (info.data?.recognizers.length ?? 0) > 0 || browserRecognizes();
  return (
    <>
      <Group>
        <Row label={t("assistant.speakAnswers")} detail={t("assistant.speakAnswersDetail")}>
          <Switch on={prefs.assistantSpeak} label={t("assistant.speakAnswers")} onChange={(assistantSpeak) => setPrefs({ assistantSpeak })} />
        </Row>
        <Row label={t("assistant.voice")}>
          <Select
            value={voices.some((v) => v.id === prefs.assistantVoice) ? prefs.assistantVoice : ""}
            label={t("assistant.voice")}
            disabled={!prefs.assistantSpeak}
            options={[{ value: "", label: t("assistant.deviceVoice") }, ...voices.map((v) => ({ value: v.id, label: v.name || v.id }))]}
            onChange={(assistantVoice) => setPrefs({ assistantVoice })}
          />
        </Row>
      </Group>
      <Group footer={t("assistant.recognitionFooter")}>
        {speech ? (
          <Row label={t("assistant.recognition")}>
            <Select
              value={prefs.assistantRecognition}
              label={t("assistant.recognition")}
              options={[
                { value: "server" as const, label: t("assistant.serverFirst") },
                { value: "device" as const, label: t("assistant.deviceFirst") },
              ]}
              onChange={(assistantRecognition) => setPrefs({ assistantRecognition })}
            />
          </Row>
        ) : (
          <Row label={t("assistant.recognition")} detail={t("assistant.noSpeech")} wrap />
        )}
      </Group>
      {server && info.data?.enabled && (
        <>
          <Memories key={`m${server.id}`} server={server} on={info.data.memory} />
          {info.data.ai && <Conversations key={`c${server.id}`} server={server} />}
        </>
      )}
    </>
  );
}

function Memories({ server, on }: { server: ServerEntry; on: boolean }) {
  const memories = useAssistantMemories(server);
  const actions = useAssistantActions(server);
  const [text, setText] = useState("");
  const [asking, setAsking] = useState(false);
  const list = memories.data ?? [];
  const failure = actions.updateMemory.error ?? actions.deleteMemories.error ?? memories.error;
  if (!on)
    return (
      <Group title={t("assistant.memory")}>
        <Row label={t("assistant.memoryOff")} />
      </Group>
    );
  return (
    <Group title={t("assistant.memory")} footer={t("assistant.memoryFooter")}>
      {asking && (
        <Confirm
          title={t("assistant.forgetAllConfirm")}
          action={t("assistant.forgetAll")}
          busy={actions.deleteMemories.isPending}
          onCancel={() => setAsking(false)}
          onConfirm={() => actions.deleteMemories.mutate([], { onSettled: () => setAsking(false) })}
        />
      )}
      {list.map((m) => (
        <Row key={m.id} label={m.text} detail={m.updateTime && when.format(timestampDate(m.updateTime))} wrap>
          <Button variant="plain" className="text-danger" onClick={() => actions.deleteMemories.mutate([m.id])}>
            {t("assistant.forget")}
          </Button>
        </Row>
      ))}
      {memories.isSuccess && list.length === 0 && <Row label={t("assistant.noMemories")} />}
      <form
        className="flex flex-wrap items-center gap-2 px-4 py-2.5"
        onSubmit={(e) => {
          e.preventDefault();
          if (text.trim()) actions.updateMemory.mutate({ id: "", text: text.trim() }, { onSuccess: () => setText("") });
        }}
      >
        <input
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder={t("assistant.newMemory")}
          aria-label={t("assistant.newMemory")}
          maxLength={1000}
          className={`${input} min-w-0 flex-1`}
        />
        <Button type="submit" variant="secondary" disabled={!text.trim() || actions.updateMemory.isPending}>
          {t("assistant.remember")}
        </Button>
        {list.length > 0 && (
          <Button variant="plain" className="text-danger" onClick={() => setAsking(true)}>
            {t("assistant.forgetAll")}
          </Button>
        )}
      </form>
      {failure && <p className="px-4 py-2 type-callout text-danger">{errorMessage(failure)}</p>}
    </Group>
  );
}

function Conversations({ server }: { server: ServerEntry }) {
  const conversations = useAssistantConversations(server);
  const actions = useAssistantActions(server);
  const [shown, setShown] = useState<string>();
  const list = conversations.data ?? [];
  return (
    <Group title={t("assistant.conversations")} footer={t("assistant.conversationsFooter")}>
      {list.map((c) => (
        <div key={c.id} className="flex flex-col">
          <button type="button" className="text-start hover:tint" onClick={() => setShown(shown === c.id ? undefined : c.id)} aria-expanded={shown === c.id}>
            <Row
              label={c.title || "…"}
              detail={[c.deviceName, c.updateTime && when.format(timestampDate(c.updateTime))].filter(Boolean).join(" · ")}
            />
          </button>
          {shown === c.id && (
            <Messages server={server} id={c.id} onDelete={() => actions.deleteConversation.mutate(c.id, { onSuccess: () => setShown(undefined) })} />
          )}
        </div>
      ))}
      {conversations.isSuccess && list.length === 0 && <Row label={t("assistant.noConversations")} />}
      {conversations.isError && <p className="px-4 py-2 type-callout text-danger">{errorMessage(conversations.error)}</p>}
    </Group>
  );
}

function Messages({ server, id, onDelete }: { server: ServerEntry; id: string; onDelete: () => void }) {
  const conversation = useAssistantConversation(server, id);
  return (
    <div className="flex flex-col gap-2 px-4 pb-3">
      {conversation.data?.messages.map((m, i) => (
        <p key={i} className={`max-w-[85%] rounded-[14px] px-3 py-2 type-callout whitespace-pre-wrap ${m.user ? "self-end bg-accent text-on-accent" : "self-start bg-fill"}`}>
          {m.text}
        </p>
      ))}
      {conversation.isError && <p className="type-callout text-danger">{errorMessage(conversation.error)}</p>}
      <Button variant="plain" className="self-start text-danger" onClick={onDelete}>
        {t("assistant.deleteConversation")}
      </Button>
    </div>
  );
}

/** The server's assistant settings, in its AI settings. */
export function AssistantServerSettings({ server }: { server: ServerEntry }) {
  const settings = useAssistantSettings(server);
  if (settings.isError) return <p className="type-callout text-danger">{errorMessage(settings.error)}</p>;
  return settings.data ? <ServerForm server={server} settings={settings.data} /> : null;
}

function ServerForm({ server, settings }: { server: ServerEntry; settings: AssistantSettings }) {
  const update = useUpdateAssistantSettings(server);
  const models = useAiModels(server, true);
  const users = useUsers(server);
  const info = useAssistant(server, locale);
  const [draft, setDraft] = useState(() => clone(AssistantSettingsSchema, settings));
  const [notice, setNotice] = useState<string>();
  const set = (patch: Partial<AssistantSettings>) => (setDraft((d) => patched(AssistantSettingsSchema, d, patch)), setNotice(undefined));
  const changed = !equals(AssistantSettingsSchema, draft, settings);
  const guard = useUnsavedGuard(changed);
  const save = () => update.mutate(draft, { onSuccess: () => setNotice(t("assistant.saved")) });
  const voices = info.data?.voices ?? [];
  const setVoice = (i: number, v: Partial<AssistantVoiceChoice>) =>
    set({ voices: draft.voices.map((x, j) => (j === i ? create(AssistantVoiceChoiceSchema, { ...x, ...v }) : x)) });
  const off = !draft.enabled;
  return (
    <>
      {guard.dialog}
      <Group title={t("assistant.server")}>
        <Row label={t("assistant.enabled")} detail={t("assistant.enabledDetail")}>
          <Switch on={draft.enabled} label={t("assistant.enabled")} onChange={(enabled) => set({ enabled })} />
        </Row>
        <Row label={t("assistant.model")}>
          <Select
            value={draft.model}
            label={t("assistant.model")}
            disabled={off}
            options={[
              { value: "", label: t("assistant.noModel") },
              ...(models.data ?? []).map((m) => ({ value: `ai/${m.provider}/${m.id}`, label: `${m.name || m.id} (${m.provider})` })),
            ]}
            onChange={(model) => set({ model })}
          />
        </Row>
        <Row label={t("assistant.codeMode")} detail={t("assistant.codeModeDetail")}>
          <Switch on={draft.codeMode} label={t("assistant.codeMode")} disabled={off || !draft.model} onChange={(codeMode) => set({ codeMode })} />
        </Row>
        <Row label={t("assistant.aiAccess")}>
          <Select
            value={String(draft.aiAccess || AssistantAccess.ALL)}
            label={t("assistant.aiAccess")}
            disabled={off || !draft.model}
            options={[
              { value: String(AssistantAccess.ALL), label: t("assistant.accessAll") },
              { value: String(AssistantAccess.ADMINISTRATORS), label: t("assistant.accessAdmins") },
              { value: String(AssistantAccess.SELECTED), label: t("assistant.accessSelected") },
            ]}
            onChange={(v) => set({ aiAccess: Number(v) as AssistantAccess })}
          />
        </Row>
        {draft.aiAccess === AssistantAccess.SELECTED && (
          <div className="flex flex-wrap gap-x-5 gap-y-2 px-4 py-3">
            {(users.data ?? []).map((u) => (
              <Checkbox
                key={u.id}
                checked={draft.aiUserIds.includes(u.id)}
                onChange={(on) => set({ aiUserIds: on ? [...draft.aiUserIds, u.id] : draft.aiUserIds.filter((id) => id !== u.id) })}
              >
                {u.name}
              </Checkbox>
            ))}
          </div>
        )}
        <Row label={t("assistant.idleMinutes")}>
          <NumberField value={draft.idleMinutes || undefined} placeholder="30" min={1} max={1440} label={t("assistant.idleMinutes")} onChange={(v) => set({ idleMinutes: v ?? 0 })} />
        </Row>
        <Row label={t("assistant.retentionDays")}>
          <NumberField value={draft.retentionDays || undefined} placeholder="30" min={1} max={3650} label={t("assistant.retentionDays")} onChange={(v) => set({ retentionDays: v ?? 0 })} />
        </Row>
        <Row label={t("assistant.serverMemory")} detail={t("assistant.serverMemoryDetail")}>
          <Switch on={draft.memory} label={t("assistant.serverMemory")} disabled={off} onChange={(memory) => set({ memory })} />
        </Row>
        <Row label={t("assistant.recognizer")}>
          <Select
            value={draft.recognizer}
            label={t("assistant.recognizer")}
            disabled={off}
            options={[{ value: "", label: t("assistant.firstReady") }, ...(info.data?.recognizers ?? []).map((r) => ({ value: r.id, label: r.name || r.id }))]}
            onChange={(recognizer) => set({ recognizer })}
          />
        </Row>
        <Row label={t("assistant.hints")} detail={t("assistant.hintsDetail")}>
          <Switch on={draft.hints} label={t("assistant.hints")} disabled={off} onChange={(hints) => set({ hints })} />
        </Row>
      </Group>
      {voices.length > 0 && (
        <Group title={t("assistant.defaultVoices")} footer={t("assistant.defaultVoicesFooter")}>
          {draft.voices.map((v, i) => (
            <div key={i} className="flex flex-wrap items-center gap-2 px-4 py-2.5">
              <input
                value={v.language}
                onChange={(e) => setVoice(i, { language: e.target.value.trim() })}
                placeholder="en"
                aria-label={t("assistant.language")}
                className={`${input} w-24`}
              />
              <Select
                value={v.voice}
                label={t("assistant.voice")}
                options={voices.map((x) => ({ value: x.id, label: x.name || x.id }))}
                onChange={(voice) => setVoice(i, { voice })}
                className="min-w-0 flex-1"
              />
              <Button variant="plain" className="text-danger" onClick={() => set({ voices: draft.voices.filter((_, j) => j !== i) })}>
                {t("action.delete")}
              </Button>
            </div>
          ))}
          <div className="px-4 py-2.5">
            <Button
              variant="secondary"
              onClick={() => set({ voices: [...draft.voices, create(AssistantVoiceChoiceSchema, { language: locale.split("-")[0], voice: voices[0].id })] })}
            >
              {t("assistant.addVoice")}
            </Button>
          </div>
        </Group>
      )}
      <div className="flex flex-wrap items-center gap-3">
        <Button disabled={!changed || update.isPending} onClick={save}>
          {update.isPending ? "…" : t("action.save")}
        </Button>
        {changed && (
          <Button variant="plain" onClick={() => setDraft(clone(AssistantSettingsSchema, settings))}>
            {t("action.cancel")}
          </Button>
        )}
        {notice && !changed && <span className="type-callout text-text-secondary">{notice}</span>}
        {update.isError && <span className="type-callout break-words text-danger">{errorMessage(update.error)}</span>}
      </div>
    </>
  );
}

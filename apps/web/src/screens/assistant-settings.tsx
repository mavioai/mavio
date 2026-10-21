// The assistant's settings (docs/assistant.md §9.1, §10): the user's
// memories and conversations on the server; and, in the AI settings, the
// server's for its administrators, with the recognizer and voice of each
// language and the timing of turns.
import { clone, create, equals } from "@bufbuild/protobuf";
import { type Duration, durationFromMs, durationMs, timestampDate } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  type ServerEntry,
  useAiModels,
  useAiProviders,
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
  type AssistantSpeech,
  AssistantSpeechSchema,
  type VoiceEngine,
} from "@mavio/proto/mavio/assistant/v1/assistant_pb";
import { useCallback, useEffect, useState } from "react";

import { Checkbox, NumberField, Switch } from "../controls/inputs.tsx";
import { Confirm } from "../controls/dialog.tsx";
import { Select } from "../controls/select.tsx";
import { ModelPicker } from "../controls/model-picker.tsx";
import { modelDetail } from "../ai-model.ts";
import { input } from "../controls/styles.ts";
import { useUnsavedGuard } from "../controls/unsaved.tsx";
import { locale, t } from "../i18n.ts";
import { Button, Group, Row } from "../ui.tsx";
import { useKeptSpeech } from "../assistant/kept-speech.ts";
import { MessageBar } from "../assistant/message-bar.tsx";
import { chooseMicrophone, chosenMicrophone, microphoneAllowed } from "../assistant/voice.ts";
import { patched } from "./admin/common.tsx";

const when = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" });

/** The user's memories and conversations on a server. */
export function AssistantPrefs({ server }: { server?: ServerEntry }) {
  const info = useAssistant(server, locale);
  if (!server || !info.data?.enabled) return null;
  return (
    <>
      {info.data.voice && microphoneAllowed() && <Microphone />}
      <Memories key={`m${server.id}`} server={server} on={info.data.memory} />
      {info.data.ai && <Conversations key={`c${server.id}`} server={server} />}
    </>
  );
}

// The browser's own entries for the system's microphone, which "System
// default" stands for.
const systemIds = new Set(["default", "communications"]);

/** The microphone voice sessions listen with in this browser. */
function Microphone() {
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([]);
  const [chosen, setChosen] = useState(chosenMicrophone);
  const [failure, setFailure] = useState<string>();
  const list = useCallback(async () => {
    try {
      setDevices((await navigator.mediaDevices.enumerateDevices()).filter((d) => d.kind === "audioinput"));
    } catch (err) {
      setFailure(errorMessage(err));
    }
  }, []);
  useEffect(() => {
    void list();
    navigator.mediaDevices.addEventListener("devicechange", list);
    return () => navigator.mediaDevices.removeEventListener("devicechange", list);
  }, [list]);
  // Browsers name their microphones only once the page may use one.
  const named = devices.some((d) => d.label);
  const allow = async () => {
    try {
      (await navigator.mediaDevices.getUserMedia({ audio: true })).getTracks().forEach((tr) => tr.stop());
      await list();
    } catch (err) {
      setFailure(errorMessage(err));
    }
  };
  const others = devices.filter((d) => !systemIds.has(d.deviceId));
  const value = chosen && others.some((d) => d.deviceId === chosen) ? chosen : "";
  return (
    <Group footer={t("assistant.microphoneFooter")}>
      <Row label={t("assistant.microphoneChoice")}>
        {named ? (
          <Select
            value={value}
            label={t("assistant.microphoneChoice")}
            options={[
              { value: "", label: t("assistant.systemMicrophone") },
              ...others.map((d, i) => ({ value: d.deviceId, label: d.label || t("assistant.unnamedMicrophone", { n: i + 1 }) })),
            ]}
            onChange={(id) => {
              // The system's own, where the browser lists it, is asked for
              // exactly: a browser can rank its own choice above it.
              const next = id || (devices.some((d) => d.deviceId === "default") ? "default" : undefined);
              chooseMicrophone(next);
              setChosen(next);
            }}
          />
        ) : (
          <Button variant="secondary" onClick={() => void allow()}>
            {t("assistant.showMicrophones")}
          </Button>
        )}
      </Row>
      {failure && <p className="px-4 py-2 type-callout text-danger">{failure}</p>}
    </Group>
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
            {/* The assistant's conversations have no titles: they go by when they were, and where. */}
            <Row
              label={c.title || (c.updateTime && when.format(timestampDate(c.updateTime))) || ""}
              detail={[c.deviceName, c.title && c.updateTime && when.format(timestampDate(c.updateTime))].filter(Boolean).join(" · ")}
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
  const playing = useKeptSpeech(server);
  return (
    <div className="flex flex-col gap-4 px-4 pb-3">
      {conversation.data?.messages.map((m, i) => (
        <div key={i} className={`flex max-w-[85%] flex-col gap-0.5 ${m.user ? "items-end self-end" : "items-start self-start"}`}>
          <p className={`rounded-[14px] px-3 py-2 type-callout whitespace-pre-wrap ${m.user ? "bg-button text-on-button" : "bg-fill"}`}>{m.text}</p>
          <MessageBar
            text={m.text}
            at={m.time && timestampDate(m.time)}
            playing={playing.at === String(i)}
            onPlay={m.speechUrl ? () => playing.toggle(String(i), m.speechUrl) : undefined}
          />
        </div>
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
  const d = settings.data;
  return d?.settings ? <ServerForm server={server} settings={d.settings} recognizers={d.recognizers} voices={d.voices} /> : null;
}

/** Seconds of a duration, for a field; undefined when unset. */
const seconds = (d: Duration | undefined) => (d ? durationMs(d) / 1000 : undefined);

/** A duration of seconds from a field; unset when empty, for the default. */
const ofSeconds = (v: number | undefined) => (v === undefined ? undefined : durationFromMs(Math.round(v * 1000)));

function ServerForm({
  server,
  settings,
  recognizers,
  voices,
}: {
  server: ServerEntry;
  settings: AssistantSettings;
  recognizers: VoiceEngine[];
  voices: VoiceEngine[];
}) {
  const update = useUpdateAssistantSettings(server);
  const models = useAiModels(server, true);
  const providers = useAiProviders(server);
  const providerNames = new Map((providers.data ?? []).map((p) => [p.id, p.name]));
  const users = useUsers(server);
  const [draft, setDraft] = useState(() => clone(AssistantSettingsSchema, settings));
  const [notice, setNotice] = useState<string>();
  const set = (patch: Partial<AssistantSettings>) => (setDraft((d) => patched(AssistantSettingsSchema, d, patch)), setNotice(undefined));
  const changed = !equals(AssistantSettingsSchema, draft, settings);
  const guard = useUnsavedGuard(changed);
  const save = () => update.mutate(draft, { onSuccess: () => setNotice(t("assistant.saved")) });
  const setSpeech = (i: number, v: Partial<AssistantSpeech>) =>
    set({ speech: draft.speech.map((x, j) => (j === i ? create(AssistantSpeechSchema, { ...x, ...v }) : x)) });
  const choices = (engines: VoiceEngine[]) => [
    { value: "", label: t("assistant.firstReady") },
    ...engines.map((e) => ({ value: e.id, label: e.name || e.id })),
  ];
  const off = !draft.enabled;
  return (
    <>
      {guard.dialog}
      <Group title={t("assistant.server")}>
        <Row label={t("assistant.enabled")} detail={t("assistant.enabledDetail")}>
          <Switch on={draft.enabled} label={t("assistant.enabled")} onChange={(enabled) => set({ enabled })} />
        </Row>
        <Row label={t("assistant.model")}>
          <ModelPicker
            value={draft.model}
            label={t("assistant.model")}
            disabled={off}
            className="w-full sm:w-72"
            options={[
              { value: "", label: t("assistant.noModel") },
              ...(models.data ?? []).map((m) => ({
                value: `ai/${m.provider}/${m.id}`,
                label: m.name || m.id,
                detail: modelDetail(m),
                provider: m.provider,
                providerName: providerNames.get(m.provider) ?? m.provider,
              })),
            ]}
            labels={{ placeholder: t("assistant.noModel"), search: t("nav.search"), empty: t("schema.noMatches") }}
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
        <Row label={t("assistant.hints")} detail={t("assistant.hintsDetail")}>
          <Switch on={draft.hints} label={t("assistant.hints")} disabled={off} onChange={(hints) => set({ hints })} />
        </Row>
      </Group>
      <Group title={t("assistant.speech")} footer={t("assistant.speechFooter")}>
        {draft.speech.map((v, i) => (
          <div key={i} className="flex flex-wrap items-center gap-2 px-4 py-2.5">
            <input
              value={v.language}
              onChange={(e) => setSpeech(i, { language: e.target.value.trim() })}
              placeholder="en"
              aria-label={t("assistant.language")}
              disabled={off}
              className={`${input} w-24`}
            />
            <Select
              value={v.recognizer}
              label={t("assistant.recognizer")}
              disabled={off}
              options={choices(recognizers)}
              onChange={(recognizer) => setSpeech(i, { recognizer })}
              className="min-w-0 flex-1"
            />
            <Select
              value={v.voice}
              label={t("assistant.voice")}
              disabled={off}
              options={choices(voices)}
              onChange={(voice) => setSpeech(i, { voice })}
              className="min-w-0 flex-1"
            />
            <Button variant="plain" className="text-danger" onClick={() => set({ speech: draft.speech.filter((_, j) => j !== i) })}>
              {t("action.delete")}
            </Button>
          </div>
        ))}
        <div className="px-4 py-2.5">
          <Button
            variant="secondary"
            disabled={off}
            onClick={() => set({ speech: [...draft.speech, create(AssistantSpeechSchema, { language: locale.split("-")[0] })] })}
          >
            {t("assistant.addLanguage")}
          </Button>
        </div>
        <Row label={t("assistant.longestPause")}>
          <NumberField
            value={seconds(draft.longestPause)}
            placeholder="3"
            min={0.5}
            max={10}
            step={0.5}
            integer={false}
            disabled={off}
            label={t("assistant.longestPause")}
            onChange={(v) => set({ longestPause: ofSeconds(v) })}
          />
        </Row>
        <Row label={t("assistant.commitDelay")}>
          <NumberField
            value={seconds(draft.commitDelay)}
            placeholder="0.7"
            min={0}
            max={3}
            step={0.1}
            integer={false}
            disabled={off}
            label={t("assistant.commitDelay")}
            onChange={(v) => set({ commitDelay: ofSeconds(v) })}
          />
        </Row>
        <Row label={t("assistant.sessionIdle")}>
          <NumberField
            value={seconds(draft.sessionIdle)}
            placeholder="8"
            min={3}
            max={120}
            disabled={off}
            label={t("assistant.sessionIdle")}
            onChange={(v) => set({ sessionIdle: ofSeconds(v) })}
          />
        </Row>
      </Group>
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

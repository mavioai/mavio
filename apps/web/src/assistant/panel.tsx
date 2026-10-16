// The assistant's panel (docs/assistant.md §9.1, §9.2): over any page, the
// player's included, it shows the text heard as it grows, the answer,
// results to choose from and a pending confirmation, and takes typed
// requests. The assistant button and a key held talk; a request goes
// through the fast path (fast.ts) and else to the server's Ask, whose
// client actions the page carries out. It closes on its own a few seconds
// after an answer with nothing to choose.
import { create, isFieldSet, type MessageInitShape } from "@bufbuild/protobuf";
import { durationFromMs, durationMs } from "@bufbuild/protobuf/wkt";
import {
  artwork,
  ask,
  errorMessage,
  Sentences,
  type ServerEntry,
  startListening,
  useAssistant,
  useServersStore,
  useSignedIn,
} from "@mavio/client-ts";
import {
  AskContextSchema,
  type AskRequestSchema,
  type ClientAction,
  ControlCommand,
  NavigateTarget,
  PlayActionSchema,
  StreamsActionSchema,
} from "@mavio/proto/mavio/assistant/v1/assistant_pb";
import type { Intent } from "@mavio/proto/mavio/intent/v1/intent_pb";
import { ImageKind, type Item } from "@mavio/proto/mavio/library/v1/item_pb";
import { useLocation, useParams, useRouter } from "@tanstack/react-router";
import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";

import { locale, t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { usePrefs } from "../prefs.ts";
import { useShortcutsShown } from "../keys.ts";
import { Artwork, subtitleOf } from "../ui.tsx";
import { grammar, runLocal } from "./fast.ts";
import { browserRecognizes, denied, isDenied, type Listening, listenInBrowser, listenOnServer, microphoneAllowed } from "./listen.ts";
import { openPlayer } from "./player.ts";
import { Speaker } from "./speak.ts";

/** The key held to talk, outside text fields. */
export const talkKey = "v";

/** The language spoken to the assistant: the browser's of the UI's language, else the UI's. */
const speechLanguage = (typeof navigator === "undefined" ? [] : navigator.languages).find((l) => l.split("-")[0] === locale.split("-")[0]) ?? locale;

/** How long a held button or key must be held to end the speech as it is let go. */
const holdMs = 400;

/** How long an answer with nothing to choose stays before the panel closes. */
const lingerMs = 6000;

interface State {
  open: boolean;
  /** What the user said or typed; partial while listening. */
  heard: string;
  listening: boolean;
  working: boolean;
  /** The summaries of the tools running or run. */
  activity: { id: string; summary: string; done: boolean; failed: boolean }[];
  answer: string;
  items: Item[];
  choose: boolean;
  confirmation?: { id: string; text: string };
  error?: string;
}

const idle: State = { open: false, heard: "", listening: false, working: false, activity: [], answer: "", items: [], choose: false };

interface Assistant {
  /** The server whose assistant answers, when it is on. */
  server?: ServerEntry;
  state: State;
  open(): void;
  close(): void;
  /** Starts listening, as the button or key goes down. */
  press(): void;
  /** The button or key went up. */
  release(): void;
  send(text: string): void;
}

const Context = createContext<Assistant | undefined>(undefined);

/** The assistant of the page, if one is set up. */
export function useAssistantPanel(): Assistant | undefined {
  return useContext(Context);
}

/** The server the assistant asks: the page's, else the one serving the web UI, else the first signed in to. */
function useAssistantServer(): ServerEntry | undefined {
  const servers = useServersStore();
  const signedIn = useSignedIn();
  const { serverId } = useParams({ strict: false }) as { serverId?: string };
  const home = servers.homeServer();
  return signedIn.find((s) => s.id === serverId) ?? signedIn.find((s) => s.id === home?.id) ?? signedIn[0];
}

/** The screen a path shows, as AskContext names it. */
function screenOf(pathname: string): string {
  if (pathname === "/") return "home";
  const m = /^\/s\/[^/]+\/(item|library|play|person|discover|title)\b/.exec(pathname);
  if (m) return { item: "item", library: "library", play: "player", person: "person", discover: "discover", title: "discover" }[m[1]]!;
  if (pathname.startsWith("/search")) return "search";
  if (pathname.startsWith("/settings")) return "settings";
  return pathname.split("/")[1] ?? "";
}

/** Provides the assistant to the page, with its panel and the key held to talk. */
export function AssistantProvider({ children }: { children: ReactNode }) {
  const servers = useServersStore();
  const router = useRouter();
  const { pathname } = useLocation();
  const params = useParams({ strict: false }) as { itemId?: string; libraryId?: string };
  const server = useAssistantServer();
  const info = useAssistant(server, locale);
  const prefs = usePrefs();
  const enabled = !!info.data?.enabled;
  const [state, setState] = useState<State>(idle);
  const set = useCallback((patch: Partial<State> | ((s: State) => Partial<State>)) => setState((s) => ({ ...s, ...(typeof patch === "function" ? patch(s) : patch) })), []);

  // What the next request needs: the latest state and context, the actions
  // the last answer could not carry out, and what runs now.
  const latest = useRef({ state, pathname, params, server, prefs, info: info.data });
  latest.current = { state, pathname, params, server, prefs, info: info.data };
  const failed = useRef<string[]>([]);
  const running = useRef<AbortController>(undefined);
  const listening = useRef<Listening>(undefined);
  const speaker = useRef<Speaker>(undefined);
  const pressedAt = useRef(0);
  const linger = useRef<ReturnType<typeof setTimeout>>(undefined);

  const stopSpeaking = useCallback(() => {
    speaker.current?.stop();
    speaker.current = undefined;
  }, []);

  const close = useCallback(() => {
    clearTimeout(linger.current);
    running.current?.abort();
    listening.current?.cancel();
    listening.current = undefined;
    stopSpeaking();
    setState(idle);
  }, [stopSpeaking]);

  /** Carries out a client action; false when the page cannot. */
  const perform = useCallback(
    (a: ClientAction, srv: ServerEntry): boolean => {
      const k = a.kind;
      const p = openPlayer();
      switch (k.case) {
        case "play": {
          const id = k.value.itemIds[k.value.startIndex] ?? k.value.itemIds[0];
          if (!id) return false;
          void router.navigate({
            to: "/s/$serverId/play/$itemId",
            params: { serverId: srv.id, itemId: id },
            search: {
              start: k.value.startPosition ? durationMs(k.value.startPosition) : undefined,
              audio: isFieldSet(k.value, PlayActionSchema.field.audioStreamIndex) ? k.value.audioStreamIndex : undefined,
              subtitle: isFieldSet(k.value, PlayActionSchema.field.subtitleStreamIndex) ? k.value.subtitleStreamIndex : undefined,
            },
          });
          return true;
        }
        case "control": {
          if (!p) return false;
          const v = k.value.value;
          const st = p.state();
          switch (k.value.command) {
            case ControlCommand.PAUSE:
              return (p.pause(), true);
            case ControlCommand.RESUME:
              return (p.resume(), true);
            case ControlCommand.STOP:
              return (p.stop(), true);
            case ControlCommand.SEEK:
              return (p.seek(v), true);
            case ControlCommand.SEEK_BY:
              return (p.seek(st.position + v), true);
            case ControlCommand.NEXT:
              return p.next();
            case ControlCommand.PREVIOUS:
              return (p.previous(), true);
            case ControlCommand.SPEED:
              return (p.speed(v), true);
            case ControlCommand.VOLUME:
              return (p.volume(v), true);
            case ControlCommand.MUTE:
              return (p.mute(true), true);
            case ControlCommand.UNMUTE:
              return (p.mute(false), true);
            case ControlCommand.FULLSCREEN:
              return (p.fullScreen(), true);
          }
          return false;
        }
        case "streams": {
          // The player shifts no subtitles.
          if (!p || k.value.subtitleOffset !== 0) return false;
          p.streams(
            isFieldSet(k.value, StreamsActionSchema.field.audioStreamIndex) ? k.value.audioStreamIndex : undefined,
            isFieldSet(k.value, StreamsActionSchema.field.subtitleStreamIndex) ? k.value.subtitleStreamIndex : undefined,
          );
          return true;
        }
        case "navigate": {
          const { target, id, query } = k.value;
          if (target === NavigateTarget.HOME) void router.navigate({ to: "/" });
          else if (target === NavigateTarget.BACK) router.history.back();
          else if (target === NavigateTarget.ITEM && id) void router.navigate({ to: "/s/$serverId/item/$itemId", params: { serverId: srv.id, itemId: id } });
          else if (target === NavigateTarget.LIBRARY && id) void router.navigate({ to: "/s/$serverId/library/$libraryId", params: { serverId: srv.id, libraryId: id } });
          else if (target === NavigateTarget.SEARCH) void router.navigate({ to: "/search", search: { q: query || undefined } });
          else if (target === NavigateTarget.SETTINGS) void router.navigate({ to: "/settings" });
          else return false;
          return true;
        }
        case "showItems":
          set({ items: k.value.items, choose: k.value.choose });
          return true;
      }
      return false;
    },
    [router, set],
  );

  /** Closes the panel a while after an answer with nothing left to do. */
  const lingerThenClose = useCallback(() => {
    clearTimeout(linger.current);
    linger.current = setTimeout(() => {
      const s = latest.current.state;
      if (!s.listening && !s.working && !s.confirmation && !s.choose && !s.error) setState(idle);
    }, lingerMs);
  }, []);

  /** Speaks sentences of an answer, when the request was spoken and the user lets answers be spoken. */
  const speakerFor = useCallback(
    (spoken: boolean): Speaker | undefined => {
      const { prefs: pr, server: srv, info: inf } = latest.current;
      if (!spoken || !pr.assistantSpeak) return undefined;
      stopSpeaking();
      speaker.current = new Speaker(
        pr.assistantVoice,
        srv && inf ? { servers, server: srv, languages: inf.voices.flatMap((v) => v.languages) } : undefined,
      );
      return speaker.current;
    },
    [servers, stopSpeaking],
  );

  const request = useCallback(
    async (req: MessageInitShape<typeof AskRequestSchema>, spoken: boolean, heardLanguage = "") => {
      const { state: s, pathname: path, params: ps, server: srv } = latest.current;
      if (!srv) return;
      clearTimeout(linger.current);
      running.current?.abort();
      const abort = new AbortController();
      running.current = abort;
      const text = req.text?.trim() ?? "";
      set({ open: true, heard: text || s.heard, working: true, activity: [], answer: "", error: undefined, confirmation: undefined, ...(req.choice || text ? { items: [], choose: false } : {}) });
      const voice = speakerFor(spoken);
      const lang = heardLanguage || speechLanguage;

      // The fast path: a command for this page is carried out here.
      let intent: Intent | undefined;
      const g = text ? await grammar() : undefined;
      if (g && text) {
        intent = g.parse({
          text,
          languages: [lang, locale],
          state: { confirmation: !!s.confirmation, results: s.choose ? s.items.length : 0, playing: !!openPlayer() },
        })[0];
        const reply = intent && runLocal(g, intent, {
          home: () => void router.navigate({ to: "/" }),
          back: () => router.history.back(),
          settings: () => void router.navigate({ to: "/settings" }),
        });
        if (intent && reply) {
          const answer = g.reply(intent.language || lang, reply.key, reply.args);
          set({ working: false, answer });
          voice?.say(answer, intent.language || lang);
          lingerThenClose();
          return;
        }
      }

      const p = openPlayer()?.state();
      const context = create(AskContextSchema, {
        languages: [...new Set([lang, speechLanguage, locale])],
        screen: screenOf(path),
        itemId: ps.itemId ?? "",
        libraryId: ps.libraryId ?? "",
        timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        playing: p && {
          itemId: p.itemId,
          position: durationFromMs(Math.round(p.position * 1000)),
          paused: p.paused,
          speed: p.speed,
          audioStreamIndex: p.audioStreamIndex,
          subtitleStreamIndex: p.subtitleStreamIndex,
          volume: p.volume,
          muted: p.muted,
        },
      });
      const sentences = new Sentences();
      let answerLang = lang;
      // Whether the answer waits for the user: a confirmation, or results to choose from.
      let waits = false;
      try {
        const events = ask(servers, srv, { ...req, text, intent, context, spoken, failedActions: failed.current.splice(0) }, abort.signal);
        for await (const e of events) {
          const ev = e.event;
          switch (ev.case) {
            case "understood":
              if (ev.value.text) set({ heard: ev.value.text });
              break;
            case "tool": {
              const tool = ev.value;
              set((st) => ({
                activity: [...st.activity.filter((x) => x.id !== tool.id), { id: tool.id, summary: tool.summary, done: tool.done, failed: tool.failed }],
              }));
              break;
            }
            case "action":
              if (!perform(ev.value, srv)) failed.current.push(ev.value.id);
              if (ev.value.kind.case === "showItems" && ev.value.kind.value.choose) waits = true;
              break;
            case "confirmation":
              waits = true;
              set({ confirmation: { id: ev.value.id, text: ev.value.text } });
              voice?.say(ev.value.text, answerLang);
              break;
            case "answer":
              set((st) => ({ answer: st.answer + ev.value }));
              for (const sentence of sentences.push(ev.value)) voice?.say(sentence, answerLang);
              break;
            case "done":
              answerLang = ev.value.language || answerLang;
              for (const sentence of sentences.end()) voice?.say(sentence, answerLang);
              set((st) => ({ answer: ev.value.answer || st.answer }));
              break;
          }
        }
        set((st) => ({ working: false, error: failed.current.length > 0 && !st.answer ? t("assistant.cantDo") : undefined }));
        if (!waits) lingerThenClose();
      } catch (err) {
        if (abort.signal.aborted) return;
        set({ working: false, error: errorMessage(err) });
      }
    },
    [servers, router, set, perform, speakerFor, lingerThenClose],
  );

  const send = useCallback((text: string) => void request({ text }, false), [request]);

  const listen = useCallback(async () => {
    const { server: srv, info: inf, prefs: pr } = latest.current;
    if (!srv || listening.current) return;
    clearTimeout(linger.current);
    stopSpeaking();
    running.current?.abort();
    if (!microphoneAllowed()) {
      set({ open: true, error: t("assistant.insecure") });
      return;
    }
    const onServer = !!inf?.recognizers.length;
    const order = (pr.assistantRecognition === "device" ? ["device", "server"] : ["server", "device"]).filter((r) =>
      r === "server" ? onServer : browserRecognizes(),
    );
    if (order.length === 0) {
      set({ open: true, error: t("assistant.noRecognizer") });
      return;
    }
    set({ open: true, listening: true, heard: "", answer: "", activity: [], error: undefined, ...(latest.current.state.choose ? {} : { items: [] }) });
    const hearing = {
      partial: (text: string) => set({ heard: text }),
      final: (text: string, language: string) => {
        listening.current = undefined;
        set({ listening: false, heard: text });
        if (text) void request({ text }, true, language);
        else lingerThenClose();
      },
      error: (message: string) => {
        listening.current = undefined;
        set({ listening: false, error: message === denied ? t("assistant.microphoneDenied") : message });
      },
    };
    for (const r of order) {
      try {
        listening.current =
          r === "server"
            ? await listenOnServer(async () => (await startListening(servers, srv, speechLanguage)).url, hearing)
            : listenInBrowser(speechLanguage, hearing);
        return;
      } catch (err) {
        // A recognizer that fails falls back to the other.
        if (r === order.at(-1))
          set({ listening: false, error: isDenied(err) ? t("assistant.microphoneDenied") : t("assistant.noMicrophone", { error: errorMessage(err) }) });
      }
    }
  }, [servers, set, stopSpeaking, request, lingerThenClose]);

  const press = useCallback(() => {
    // Pressing again while listening ends the speech; while speaking, stops it.
    if (listening.current) {
      listening.current.stop();
      return;
    }
    pressedAt.current = Date.now();
    void listen();
  }, [listen]);

  const release = useCallback(() => {
    // A tap listens until the speech ends; a hold until it is let go.
    if (pressedAt.current && Date.now() - pressedAt.current >= holdMs) listening.current?.stop();
    pressedAt.current = 0;
  }, []);

  // The key held to talk, outside text fields.
  useEffect(() => {
    if (!enabled) return;
    const typing = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement | null;
      return !!el && (el.isContentEditable || el instanceof HTMLTextAreaElement || (el instanceof HTMLInputElement && el.type !== "range" && el.type !== "checkbox"));
    };
    const down = (e: KeyboardEvent) => {
      if (e.key !== talkKey || e.repeat || e.metaKey || e.ctrlKey || e.altKey || typing(e)) return;
      e.preventDefault();
      press();
    };
    const up = (e: KeyboardEvent) => {
      if (e.key !== talkKey || typing(e)) return;
      release();
    };
    addEventListener("keydown", down);
    addEventListener("keyup", up);
    return () => {
      removeEventListener("keydown", down);
      removeEventListener("keyup", up);
    };
  }, [enabled, press, release]);

  useEffect(() => () => close(), [close]);

  const value = useMemo<Assistant>(
    () => ({
      server: enabled ? server : undefined,
      state,
      open: () => {
        clearTimeout(linger.current);
        set({ open: true });
      },
      close,
      press,
      release,
      send,
    }),
    [enabled, server, state, set, close, press, release, send],
  );

  const answerConfirmation = (confirmed: boolean) => {
    const c = state.confirmation;
    if (c) void request({ answer: { id: c.id, confirmed } }, false);
  };
  const choose = (item: Item) => {
    if (state.choose) void request({ choice: item.id }, false);
    else if (server) void router.navigate({ to: "/s/$serverId/item/$itemId", params: { serverId: server.id, itemId: item.id } });
  };

  return (
    <Context.Provider value={value}>
      {children}
      {state.open && server && (
        <Panel
          server={server}
          state={state}
          examples={info.data?.examples ?? []}
          off={info.isSuccess && !enabled}
          onClose={close}
          onSend={send}
          onPress={press}
          onRelease={release}
          onConfirm={answerConfirmation}
          onChoose={choose}
          onTouch={() => clearTimeout(linger.current)}
        />
      )}
    </Context.Provider>
  );
}

function Panel({
  server,
  state,
  examples,
  off,
  onClose,
  onSend,
  onPress,
  onRelease,
  onConfirm,
  onChoose,
  onTouch,
}: {
  server: ServerEntry;
  state: State;
  examples: string[];
  off: boolean;
  onClose: () => void;
  onSend: (text: string) => void;
  onPress: () => void;
  onRelease: () => void;
  onConfirm: (confirmed: boolean) => void;
  onChoose: (item: Item) => void;
  onTouch: () => void;
}) {
  const [text, setText] = useState("");
  const keyboard = useShortcutsShown();
  const fresh = !state.heard && !state.answer && !state.listening && !state.working && !state.error && state.items.length === 0;
  const status = state.listening ? t("assistant.listening") : state.working && !state.answer ? t("assistant.working") : undefined;
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  }, [onClose]);
  return (
    <div
      role="dialog"
      aria-label={t("assistant.title")}
      data-assistant
      onPointerDown={onTouch}
      className="fixed inset-x-3 bottom-[calc(72px+env(safe-area-inset-bottom))] z-50 mx-auto flex max-h-[70vh] max-w-xl flex-col overflow-hidden rounded-[20px] border border-border bg-surface text-on-background shadow-level3 sm:bottom-6"
    >
      <header className="flex items-center gap-2 px-4 pt-3">
        <Icon name="microphone" fill size={18} className="text-accent-text" />
        <h2 className="flex-1 type-headline">{t("assistant.title")}</h2>
        <button
          type="button"
          aria-label={t("action.close")}
          onClick={onClose}
          className="flex size-8 items-center justify-center rounded-full bg-fill hover:bg-fill-selected"
        >
          <Icon name="close" size={16} />
        </button>
      </header>
      <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-4 py-3" aria-live="polite">
        {off && <p className="type-callout text-text-secondary">{t("assistant.off")}</p>}
        {state.heard && <p className={`type-title3 ${state.listening ? "text-text-secondary" : ""}`}>{state.heard}</p>}
        {status && <p className="type-callout text-text-secondary">{status}</p>}
        {state.activity.length > 0 && (
          <ul className="flex flex-col gap-1">
            {state.activity.map((a) => (
              <li key={a.id} className={`flex items-center gap-2 type-footnote ${a.failed ? "text-danger" : "text-text-secondary"}`}>
                <Icon name={a.failed ? "error" : a.done ? "check" : "pending"} size={14} />
                {a.summary}
              </li>
            ))}
          </ul>
        )}
        {state.answer && <p className="type-body whitespace-pre-wrap">{state.answer}</p>}
        {state.items.length > 0 && (
          <ul className="flex flex-col divide-y divide-border overflow-hidden rounded-[14px] bg-background">
            {state.items.map((item, i) => (
              <li key={item.id}>
                <button type="button" onClick={() => onChoose(item)} className="flex w-full items-center gap-3 px-3 py-2 text-start hover:tint">
                  {state.choose && <span className="w-5 shrink-0 text-center type-headline text-text-secondary">{i + 1}</span>}
                  <Artwork src={artwork(server, item, ImageKind.PRIMARY, 80)?.url} alt="" className="aspect-[2/3] w-10 shrink-0 rounded-[6px]" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate type-headline">{item.name}</span>
                    {subtitleOf(item) && <span className="block truncate type-footnote text-text-secondary">{subtitleOf(item)}</span>}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
        {state.confirmation && (
          <div className="flex flex-col gap-3 rounded-[14px] bg-background p-3">
            <p className="type-body">{state.confirmation.text}</p>
            <div className="flex gap-2">
              <button type="button" onClick={() => onConfirm(true)} className="min-h-9 rounded-md bg-button px-4 type-headline text-on-button hover:opacity-85">
                {t("assistant.confirm")}
              </button>
              <button type="button" onClick={() => onConfirm(false)} className="min-h-9 rounded-md bg-fill px-4 type-headline hover:bg-fill-selected">
                {t("action.cancel")}
              </button>
            </div>
          </div>
        )}
        {state.error && <p className="type-callout break-words text-danger">{state.error}</p>}
        {fresh && examples.length > 0 && (
          <div className="flex flex-col gap-2">
            <p className="type-footnote text-text-secondary">{t("assistant.trySaying")}</p>
            <div className="flex flex-wrap gap-2">
              {examples.map((ex) => (
                <button key={ex} type="button" onClick={() => onSend(ex)} className="rounded-full bg-fill px-3 py-1.5 type-callout hover:bg-fill-selected">
                  {ex}
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
      <form
        className="flex items-center gap-2 border-t border-border px-3 py-2.5"
        onSubmit={(e) => {
          e.preventDefault();
          if (!text.trim()) return;
          onSend(text);
          setText("");
        }}
      >
        <TalkButton listening={state.listening} onPress={onPress} onRelease={onRelease} />
        <input
          value={text}
          onChange={(e) => (setText(e.target.value), onTouch())}
          placeholder={keyboard ? `${t("assistant.placeholder")} · ${t("assistant.shortcut", { key: talkKey.toUpperCase() })}` : t("assistant.placeholder")}
          aria-label={t("assistant.placeholder")}
          maxLength={2000}
          className="min-h-10 min-w-0 flex-1 rounded-full bg-fill px-4 type-body placeholder:text-text-tertiary focus:outline-none"
        />
        <button
          type="submit"
          disabled={!text.trim()}
          className="min-h-10 rounded-full bg-button px-4 type-headline text-on-button hover:opacity-85 disabled:opacity-40"
        >
          {t("assistant.send")}
        </button>
      </form>
    </div>
  );
}

/** The microphone button: held to talk, or tapped to talk until the speech ends. */
function TalkButton({ listening, onPress, onRelease }: { listening: boolean; onPress: () => void; onRelease: () => void }) {
  return (
    <button
      type="button"
      aria-label={listening ? t("assistant.stopListening") : t("assistant.talk")}
      aria-pressed={listening}
      onPointerDown={(e) => {
        e.preventDefault();
        onPress();
      }}
      onPointerUp={onRelease}
      onPointerCancel={onRelease}
      onKeyDown={(e) => {
        if ((e.key === " " || e.key === "Enter") && !e.repeat) {
          e.preventDefault();
          onPress();
        }
      }}
      onKeyUp={(e) => (e.key === " " || e.key === "Enter") && onRelease()}
      className={`flex size-10 shrink-0 items-center justify-center rounded-full transition ${listening ? "animate-pulse bg-danger text-white" : "bg-fill hover:bg-fill-selected"}`}
    >
      <Icon name="microphone" fill={listening} size={20} />
    </button>
  );
}

/**
 * The assistant button of the page's chrome, shown when the server's
 * assistant is on: it opens the panel and listens, held or tapped.
 */
export function AssistantButton({ className, size = 22 }: { className?: string; size?: number }) {
  const a = useAssistantPanel();
  if (!a?.server) return null;
  return (
    <button
      type="button"
      aria-label={t("assistant.title")}
      title={t("assistant.title")}
      onPointerDown={(e) => {
        if (e.button !== 0) return;
        e.preventDefault();
        a.press();
      }}
      onPointerUp={a.release}
      onPointerCancel={a.release}
      onClick={(e) => {
        // A keyboard's or a television remote's press opens the panel to type or talk.
        if (e.detail === 0) a.open();
      }}
      className={className}
    >
      <Icon name="microphone" fill={a.state.listening} size={size} />
    </button>
  );
}

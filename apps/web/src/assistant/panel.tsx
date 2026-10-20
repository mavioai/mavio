// The assistant's panel (docs/assistant.md §9.1, §9.2): over any page, the
// player's included, it shows as the AI Conversations do the text heard as
// it grows and the answer, each with buttons copying it and its time,
// results to choose from and a pending confirmation, and takes typed
// requests. The assistant button tapped starts a voice session (voice.ts)
// and tapped again ends it; held, or a key held, it talks while held. A
// typed request goes through the fast path (fast.ts) and else to the
// server's Ask, or to the voice session when one runs; the page carries
// out their client actions. It closes on its own a few seconds after an
// answer with nothing to choose, or after the session ends.
import { create, fromJson, isFieldSet, type JsonValue, type MessageInitShape, toJson } from "@bufbuild/protobuf";
import { durationFromMs, durationMs } from "@bufbuild/protobuf/wkt";
import {
  artwork,
  ask,
  errorMessage,
  type ServerEntry,
  startVoiceSession,
  useAssistant,
  useServersStore,
  useSignedIn,
} from "@mavio/client-ts";
import {
  type AskContext,
  AskContextSchema,
  type AskRequestSchema,
  type ClientAction,
  ClientActionSchema,
  ControlCommand,
  NavigateTarget,
  PlayActionSchema,
  StreamsActionSchema,
} from "@mavio/proto/mavio/assistant/v1/assistant_pb";
import type { Intent } from "@mavio/proto/mavio/intent/v1/intent_pb";
import { ImageKind, type Item } from "@mavio/proto/mavio/library/v1/item_pb";
import { VoiceMode } from "@mavio/proto/mavio/plugin/v1/voice_pb";
import { useLocation, useParams, useRouter } from "@tanstack/react-router";
import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";

import { locale, t } from "../i18n.ts";
import { Tip } from "../controls/tooltip.tsx";
import { Icon } from "../icon.tsx";
import { useShortcutsShown } from "../keys.ts";
import { isTabPlace } from "../shell/places.ts";
import { Artwork, subtitleOf } from "../ui.tsx";
import { grammar, runLocal } from "./fast.ts";
import { openPlayer } from "./player.ts";
import { bubble, MessageBar } from "./message-bar.tsx";
import { isDenied, microphoneAllowed, type VoiceEvent, VoiceSession } from "./voice.ts";

/** The key held to talk, outside text fields. */
export const talkKey = "v";

/** The language spoken to the assistant: the browser's of the UI's language, else the UI's. */
const speechLanguage = (typeof navigator === "undefined" ? [] : navigator.languages).find((l) => l.split("-")[0] === locale.split("-")[0]) ?? locale;

/** How long the button or key must be held to talk while held, rather than start a conversation. */
const holdMs = 400;

/** How long an answer with nothing to choose stays before the panel closes. */
const lingerMs = 6000;

interface State {
  open: boolean;
  /** What the user said or typed; partial while listening. */
  heard: string;
  /** When it was said, once said. */
  heardAt?: Date;
  /** A voice session runs. */
  listening: boolean;
  working: boolean;
  /** The summaries of the tools running or run. */
  activity: { id: string; summary: string; done: boolean; failed: boolean }[];
  answer: string;
  /** When the answer ended. */
  answerAt?: Date;
  /** Whether the answer is being spoken. */
  speaking: boolean;
  items: Item[];
  choose: boolean;
  confirmation?: { id: string; text: string };
  error?: string;
}

const idle: State = { open: false, heard: "", listening: false, working: false, activity: [], answer: "", speaking: false, items: [], choose: false };

interface Assistant {
  /** The server whose assistant answers, when it is on. */
  server?: ServerEntry;
  state: State;
  open(): void;
  close(): void;
  /** The button or key went down. */
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
  const enabled = !!info.data?.enabled;
  const [state, setState] = useState<State>(idle);
  const set = useCallback((patch: Partial<State> | ((s: State) => Partial<State>)) => setState((s) => ({ ...s, ...(typeof patch === "function" ? patch(s) : patch) })), []);

  // What the next request needs: the latest state and context, the actions
  // the last answer could not carry out, and what runs now.
  const latest = useRef({ state, pathname, params, server, info: info.data });
  latest.current = { state, pathname, params, server, info: info.data };
  const failed = useRef<string[]>([]);
  const running = useRef<AbortController>(undefined);
  const session = useRef<VoiceSession>(undefined);
  // The button or key held: a timer that turns the press into talking
  // while held, and whether it did.
  const hold = useRef<{ timer?: ReturnType<typeof setTimeout>; held: boolean }>(undefined);
  // The final transcripts of the user's turn, until its run starts.
  const turn = useRef("");
  const linger = useRef<ReturnType<typeof setTimeout>>(undefined);

  const close = useCallback(() => {
    clearTimeout(linger.current);
    running.current?.abort();
    session.current?.close();
    session.current = undefined;
    setState(idle);
  }, []);

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

  /** What the page shows and plays, as the requests' context. */
  const askContext = useCallback((lang: string): AskContext => {
    const { pathname: path, params: ps } = latest.current;
    const p = openPlayer()?.state();
    return create(AskContextSchema, {
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
  }, []);

  const request = useCallback(
    async (req: MessageInitShape<typeof AskRequestSchema>) => {
      const { state: s, server: srv } = latest.current;
      if (!srv) return;
      clearTimeout(linger.current);
      running.current?.abort();
      const abort = new AbortController();
      running.current = abort;
      const text = req.text?.trim() ?? "";
      set({
        open: true,
        heard: text || s.heard,
        heardAt: text ? new Date() : s.heardAt,
        working: true,
        activity: [],
        answer: "",
        answerAt: undefined,
        error: undefined,
        confirmation: undefined,
        ...(req.choice || text ? { items: [], choose: false } : {}),
      });
      const lang = speechLanguage;

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
          set({ working: false, answer, answerAt: new Date() });
          lingerThenClose();
          return;
        }
      }

      const context = askContext(lang);
      // Whether the answer waits for the user: a confirmation, or results to choose from.
      let waits = false;
      try {
        const events = ask(
          servers,
          srv,
          { ...req, text, intent, context, failedActions: failed.current.splice(0) },
          abort.signal,
        );
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
              break;
            case "answer":
              set((st) => ({ answer: st.answer + ev.value }));
              break;
            case "done":
              set((st) => ({ answer: ev.value.answer || st.answer }));
              break;
          }
        }
        set((st) => ({ working: false, answerAt: new Date(), error: failed.current.length > 0 && !st.answer ? t("assistant.cantDo") : undefined }));
        if (!waits) lingerThenClose();
      } catch (err) {
        if (abort.signal.aborted) return;
        set({ working: false, error: errorMessage(err) });
      }
    },
    [servers, router, set, perform, askContext, lingerThenClose],
  );

  /** A typed request: to the voice session when one runs, so that its answer is spoken, else to Ask. */
  const send = useCallback(
    (text: string) => {
      const v = session.current;
      if (!v) return void request({ text });
      turn.current = text;
      set({ heard: text, heardAt: new Date() });
      v.send({ type: "text", text });
    },
    [request, set],
  );

  /** Carries out what the voice session tells (§8.6). */
  const onVoice = useCallback(
    (e: VoiceEvent) => {
      const srv = latest.current.server;
      const text = typeof e.text === "string" ? e.text : "";
      switch (e.type) {
        case "user_started":
          clearTimeout(linger.current);
          break;
        case "transcript":
          if (e.final) turn.current = [turn.current, text.trim()].filter(Boolean).join(" ");
          set({ heard: e.final ? turn.current : [turn.current, text.trim()].filter(Boolean).join(" "), heardAt: undefined });
          break;
        case "run":
          turn.current = "";
          set((s) => ({
            heard: text || s.heard,
            heardAt: new Date(),
            working: true,
            activity: [],
            answer: "",
            answerAt: undefined,
            error: undefined,
            confirmation: undefined,
            ...(s.choose ? {} : { items: [] }),
          }));
          break;
        case "intent":
          if (text) set({ heard: text });
          break;
        case "tool": {
          const tool = { id: String(e.id ?? ""), summary: String(e.summary ?? ""), done: !!e.done, failed: !!e.failed };
          set((st) => ({ activity: [...st.activity.filter((x) => x.id !== tool.id), tool] }));
          break;
        }
        case "action": {
          let a: ClientAction;
          try {
            a = fromJson(ClientActionSchema, e.action as JsonValue, { ignoreUnknownFields: true });
          } catch {
            break;
          }
          if (srv) perform(a, srv);
          break;
        }
        case "confirm":
          set({ confirmation: { id: String(e.id ?? ""), text } });
          break;
        case "answer":
          set((st) => ({ answer: st.answer + text }));
          break;
        case "end":
          set((st) => ({
            working: false,
            answer: typeof e.answer === "string" && e.answer ? e.answer : st.answer,
            answerAt: new Date(),
            error: e.status === "failed" && !st.answer ? t("assistant.cantDo") : st.error,
          }));
          break;
        case "bot_started":
          set({ speaking: true });
          break;
        case "bot_stopped":
          set({ speaking: false });
          break;
        case "error":
          set({ working: false, error: typeof e.message === "string" && e.message ? e.message : String(e.code ?? "") });
          break;
      }
    },
    [set, perform],
  );

  /** Starts the session on the server: a conversation, or talking while held. */
  const connect = useCallback(
    (pushToTalk: boolean) => {
      const v = session.current;
      const srv = latest.current.server;
      if (!v || !srv) return;
      v.connect(
        startVoiceSession(servers, srv, {
          language: speechLanguage,
          mode: pushToTalk ? VoiceMode.PUSH_TO_TALK : VoiceMode.CONVERSATION,
          context: askContext(speechLanguage),
        }).then((r) => r.url),
        pushToTalk,
      );
      if (pushToTalk) v.talk(true);
    },
    [servers, askContext],
  );

  /** Opens the microphone, and the voice session once it is known whether the user holds to talk. */
  const capture = useCallback(async () => {
    const { server: srv, info: inf } = latest.current;
    if (!srv) return;
    clearTimeout(linger.current);
    running.current?.abort();
    if (!inf?.voice) {
      set({ open: true, error: t("assistant.noVoice") });
      return;
    }
    if (!microphoneAllowed()) {
      set({ open: true, error: t("assistant.insecure") });
      return;
    }
    turn.current = "";
    set({
      open: true,
      listening: true,
      heard: "",
      heardAt: undefined,
      answer: "",
      answerAt: undefined,
      activity: [],
      error: undefined,
      ...(latest.current.state.choose ? {} : { items: [] }),
    });
    try {
      const v = await VoiceSession.capture({
        event: onVoice,
        closed: (error) => {
          if (session.current !== v) return;
          session.current = undefined;
          set({ listening: false, speaking: false, working: false, ...(error ? { error } : {}) });
          if (!error) lingerThenClose();
        },
      });
      session.current = v;
      // Let go before the microphone opened: a tap.
      if (!hold.current) connect(false);
      else if (hold.current.held) connect(true);
    } catch (err) {
      set({ listening: false, error: isDenied(err) ? t("assistant.microphoneDenied") : t("assistant.noMicrophone", { error: errorMessage(err) }) });
    }
  }, [set, onVoice, lingerThenClose, connect]);

  const press = useCallback(() => {
    const v = session.current;
    // A conversation ends when pressed again.
    if (v && !v.holdsToTalk) {
      close();
      return;
    }
    const h: { timer?: ReturnType<typeof setTimeout>; held: boolean } = { held: false };
    hold.current = h;
    h.timer = setTimeout(() => {
      h.held = true;
      const s = session.current;
      if (s?.holdsToTalk) s.talk(true);
      else if (s) connect(true);
    }, holdMs);
    if (!v) void capture();
  }, [close, capture, connect]);

  const release = useCallback(() => {
    const h = hold.current;
    hold.current = undefined;
    if (!h) return;
    clearTimeout(h.timer);
    const v = session.current;
    if (h.held) v?.talk(false);
    // A tap: a conversation starts, or a session held to talk ends.
    else if (v?.holdsToTalk) close();
    else if (v) connect(false);
  }, [close, connect]);

  // The page's context goes to the session as it changes.
  useEffect(() => {
    session.current?.send({ type: "context", context: toJson(AskContextSchema, askContext(speechLanguage)) });
  }, [pathname, params.itemId, params.libraryId, askContext]);

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
    if (c) void request({ answer: { id: c.id, confirmed } });
  };
  const choose = (item: Item) => {
    if (state.choose) void request({ choice: item.id });
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
  const status = state.speaking
    ? t("assistant.speaking")
    : state.working && !state.answer
      ? t("assistant.working")
      : state.listening
        ? t("assistant.listening")
        : undefined;
  // Above the phone's tab bar where it shows.
  const tabs = isTabPlace(useLocation().pathname);
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
      className={`fixed inset-x-3 ${tabs ? "bottom-[calc(72px+env(safe-area-inset-bottom))]" : "bottom-[calc(12px+env(safe-area-inset-bottom))]"} z-50 mx-auto flex max-h-[70vh] max-w-xl flex-col overflow-hidden rounded-[20px] border border-border bg-surface text-on-background shadow-level3 sm:bottom-6`}
    >
      <header className="flex items-center gap-2 px-4 pt-3">
        <Icon name="microphone" fill size={18} />
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
      <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-4 py-3" aria-live="polite">
        {off && <p className="type-callout text-text-secondary">{t("assistant.off")}</p>}
        {state.heard && (
          <div className="flex flex-col items-end gap-1 ps-[15%]">
            <span className="px-1 type-caption text-text-tertiary">{t("conversations.you")}</span>
            <p className={`${bubble(true, "bg-background")} ${state.listening ? "text-text-secondary" : ""}`}>{state.heard}</p>
            {!state.listening && <MessageBar text={state.heard} at={state.heardAt} />}
          </div>
        )}
        {(status || state.activity.length > 0 || state.answer) && (
          <div className="flex flex-col items-start gap-1 pe-[15%]">
            <span className="px-1 type-callout">{t("assistant.title")}</span>
            {status && <p className="px-1 type-callout text-text-secondary">{status}</p>}
            {state.activity.length > 0 && (
              <ul className="flex flex-col gap-1 px-1">
                {state.activity.map((a) => (
                  <li key={a.id} className={`flex items-center gap-2 type-footnote ${a.failed ? "text-danger" : "text-text-secondary"}`}>
                    <Icon name={a.failed ? "error" : a.done ? "check" : "pending"} size={14} />
                    {a.summary}
                  </li>
                ))}
              </ul>
            )}
            {state.answer && <p className={bubble(false, "bg-background")}>{state.answer}</p>}
            {state.answer && state.answerAt && (
              <MessageBar text={state.answer} at={state.answerAt} />
            )}
          </div>
        )}
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
              <button type="button" onClick={() => onConfirm(true)} className="min-h-9 rounded-md bg-button px-4 type-headline desk:min-h-7 desk:px-3 text-on-button hover:opacity-85">
                {t("assistant.confirm")}
              </button>
              <button type="button" onClick={() => onConfirm(false)} className="min-h-9 rounded-md bg-fill px-4 type-headline desk:min-h-7 desk:px-3 hover:bg-fill-selected">
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
          placeholder={keyboard ? `${t("assistant.messagePlaceholder")} · ${t("assistant.shortcut", { key: talkKey.toUpperCase() })}` : t("assistant.messagePlaceholder")}
          aria-label={t("assistant.messagePlaceholder")}
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

/** The microphone button: held to talk while held, or tapped to start a conversation and again to end it. */
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
 * assistant is on: it opens the panel and starts a voice session, held or tapped.
 */
export function AssistantButton({
  className,
  size = 22,
  tip,
}: {
  className?: string;
  size?: number;
  /** The side of a tooltip naming it, in place of the browser's title. */
  tip?: "top" | "bottom" | "left" | "right";
}) {
  const a = useAssistantPanel();
  if (!a?.server) return null;
  const button = (
    <button
      type="button"
      aria-label={t("assistant.title")}
      title={tip ? undefined : t("assistant.title")}
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
  return tip ? (
    <Tip label={t("assistant.title")} side={tip}>
      {button}
    </Tip>
  ) : (
    button
  );
}

// The assistant over the page (docs/assistant.md §9.1, §9.2;
// libs/design/components.md, Assistant): an orb floating on its own shows
// the voice session's state, and cards beside it what the user said, the
// answer with its words lit as they are spoken, the tools run, results to
// choose from and a pending confirmation. The assistant button, or the orb,
// tapped starts a voice session (voice.ts) and tapped again ends it; held,
// or a key held, it talks while held. The page carries out the session's
// client actions. It goes away on its own a few seconds after an answer
// with nothing to choose, or after the session ends; and by a click outside
// it, Escape, or the remote's Back.
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
import { isTabPlace } from "../shell/places.ts";
import { Artwork, subtitleOf } from "../ui.tsx";
import { grammar, runLocal } from "./fast.ts";
import { Orb, type OrbMode, orbSize } from "./orb.tsx";
import { openPlayer } from "./player.ts";
import { isDenied, microphoneAllowed, type VoiceEvent, VoiceSession } from "./voice.ts";

/** The key held to talk, outside text fields. */
export const talkKey = "v";

/** The language spoken to the assistant: the browser's of the UI's language, else the UI's. */
const speechLanguage = (typeof navigator === "undefined" ? [] : navigator.languages).find((l) => l.split("-")[0] === locale.split("-")[0]) ?? locale;

/** How long the button or key must be held to talk while held, rather than start a conversation. */
const holdMs = 400;

/** How long an answer with nothing to choose stays before the assistant goes away. */
const lingerMs = 6000;

/** How long the orb and cards take to fade out. */
const leaveMs = 400;

interface State {
  open: boolean;
  /** What the user said or typed; partial while listening. */
  heard: string;
  /** When it was said, once said. */
  heardAt?: Date;
  /** A voice session runs. */
  listening: boolean;
  /** The microphone's audio has started coming, which takes it a moment to open. */
  capturing: boolean;
  working: boolean;
  /** The summaries of the tools running or run. */
  activity: { id: string; summary: string; done: boolean; failed: boolean }[];
  answer: string;
  /** When the answer ended. */
  answerAt?: Date;
  /** Whether the answer is being spoken. */
  speaking: boolean;
  /** Whether the answer comes with speech, whose playing lights its words. */
  voiced: boolean;
  /** The user speaks: from user_started to user_stopped. */
  userSpeaking: boolean;
  /** The session listens only while held, and whether it is held. */
  pushToTalk: boolean;
  talking: boolean;
  /** How many times the user barged in. */
  interrupts: number;
  /** The orb and cards fade out. */
  leaving: boolean;
  items: Item[];
  choose: boolean;
  confirmation?: { id: string; text: string };
  error?: string;
}

const idle: State = {
  open: false,
  heard: "",
  listening: false,
  capturing: false,
  working: false,
  activity: [],
  answer: "",
  speaking: false,
  voiced: false,
  userSpeaking: false,
  pushToTalk: false,
  talking: false,
  interrupts: 0,
  leaving: false,
  items: [],
  choose: false,
};

interface Assistant {
  /** The server whose assistant answers, when it is on. */
  server?: ServerEntry;
  state: State;
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

/** Provides the assistant to the page, with its orb and cards and the key held to talk. */
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
  const leave = useRef<ReturnType<typeof setTimeout>>(undefined);

  /** Ends the session, and the orb and cards fade out. */
  const close = useCallback(() => {
    clearTimeout(linger.current);
    running.current?.abort();
    session.current?.close();
    session.current = undefined;
    if (!latest.current.state.open) return;
    setState((s) => ({ ...s, leaving: true, listening: false, speaking: false, userSpeaking: false }));
    clearTimeout(leave.current);
    leave.current = setTimeout(() => setState(idle), leaveMs);
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

  /** Goes away a while after an answer with nothing left to do. */
  const lingerThenClose = useCallback(() => {
    clearTimeout(linger.current);
    linger.current = setTimeout(() => {
      const s = latest.current.state;
      if (!s.listening && !s.working && !s.confirmation && !s.choose && !s.error) close();
    }, lingerMs);
  }, [close]);

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
      clearTimeout(leave.current);
      running.current?.abort();
      const abort = new AbortController();
      running.current = abort;
      const text = req.text?.trim() ?? "";
      set({
        open: true,
        leaving: false,
        voiced: !!session.current,
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
          set({ userSpeaking: true });
          break;
        case "user_stopped":
          set({ userSpeaking: false });
          break;
        case "interrupt":
          set((s) => ({ interrupts: s.interrupts + 1, speaking: false, userSpeaking: true }));
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
            userSpeaking: false,
            voiced: false,
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
          set({ speaking: true, voiced: true });
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
      set({ pushToTalk, talking: pushToTalk });
      if (pushToTalk) v.talk(true);
    },
    [servers, askContext, set],
  );

  /** Opens the microphone, and the voice session once it is known whether the user holds to talk. */
  const capture = useCallback(async () => {
    const { server: srv, info: inf } = latest.current;
    if (!srv) return;
    clearTimeout(linger.current);
    clearTimeout(leave.current);
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
      leaving: false,
      listening: true,
      capturing: false,
      userSpeaking: false,
      speaking: false,
      voiced: false,
      pushToTalk: false,
      talking: false,
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
          set({ listening: false, capturing: false, speaking: false, userSpeaking: false, working: false, ...(error ? { error } : {}) });
          if (!error) lingerThenClose();
        },
        capturing: () => set({ capturing: true }),
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
      if (s?.holdsToTalk) {
        s.talk(true);
        set({ talking: true });
      } else if (s) connect(true);
    }, holdMs);
    if (!v) void capture();
  }, [close, capture, connect, set]);

  const release = useCallback(() => {
    const h = hold.current;
    hold.current = undefined;
    if (!h) return;
    clearTimeout(h.timer);
    const v = session.current;
    if (h.held) {
      v?.talk(false);
      set({ talking: false });
    }
    // A tap: a conversation starts, or a session held to talk ends.
    else if (v?.holdsToTalk) close();
    else if (v) connect(false);
  }, [close, connect, set]);

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
  // The audio the orb follows: the playback's while the answer is spoken, else the microphone's.
  const level = useCallback(() => {
    const v = session.current;
    if (!v) return 0;
    return latest.current.state.speaking ? v.outputLevel() : v.inputLevel();
  }, []);
  const speech = useCallback(() => session.current?.speech(), []);

  return (
    <Context.Provider value={value}>
      {children}
      {state.open && server && (
        <Overlay
          server={server}
          state={state}
          off={info.isSuccess && !enabled}
          level={level}
          speech={speech}
          onClose={close}
          onPress={press}
          onRelease={release}
          onConfirm={answerConfirmation}
          onChoose={choose}
          onHold={() => clearTimeout(linger.current)}
          onLetGo={() => {
            if (!session.current && !latest.current.state.working) lingerThenClose();
          }}
        />
      )}
    </Context.Provider>
  );
}

/** The orb's state, from the session's (libs/design/components.md, Assistant). */
function orbMode(s: State): OrbMode {
  if (s.leaving) return "ended";
  if (s.error) return "error";
  if (s.speaking) return "speaking";
  if (s.userSpeaking) return "hearing";
  if (s.working) return "thinking";
  if (s.listening) {
    if (!s.capturing) return "opening";
    return s.pushToTalk && !s.talking ? "resting" : "idle";
  }
  return "resting";
}

function Overlay({
  server,
  state,
  off,
  level,
  speech,
  onClose,
  onPress,
  onRelease,
  onConfirm,
  onChoose,
  onHold,
  onLetGo,
}: {
  server: ServerEntry;
  state: State;
  off: boolean;
  level: () => number;
  speech: () => { played: number; total: number } | undefined;
  onClose: () => void;
  onPress: () => void;
  onRelease: () => void;
  onConfirm: (confirmed: boolean) => void;
  onChoose: (item: Item) => void;
  onHold: () => void;
  onLetGo: () => void;
}) {
  const mode = orbMode(state);
  const status =
    mode === "opening"
      ? t("state.loading")
      : mode === "idle" && !state.heard
        ? t("assistant.listening")
        : mode === "resting" && state.pushToTalk
          ? t("assistant.shortcut", { key: talkKey.toUpperCase() })
          : undefined;
  // Above the phone's tab bar where it shows.
  const tabs = isTabPlace(useLocation().pathname);
  const lift = tabs ? 64 : 0;

  // A click or tap outside the orb and cards, Escape, or the remote's Back
  // (Backspace in TV mode outside a field) dismisses them.
  useEffect(() => {
    const down = (e: PointerEvent) => {
      const el = e.target as Element | null;
      if (el?.closest("[data-assistant], [data-assistant-button]")) return;
      // The press only dismisses: its click does not reach the page.
      const swallow = (c: MouseEvent) => {
        c.preventDefault();
        c.stopPropagation();
      };
      addEventListener("click", swallow, { capture: true, once: true });
      setTimeout(() => removeEventListener("click", swallow, { capture: true }), 600);
      onClose();
    };
    const key = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement | null;
      const typing = !!el && (el.isContentEditable || el instanceof HTMLTextAreaElement || el instanceof HTMLInputElement);
      const back = e.key === "Escape" || (e.key === "Backspace" && "tv" in document.documentElement.dataset && !typing);
      if (!back) return;
      e.preventDefault();
      e.stopPropagation();
      onClose();
    };
    addEventListener("pointerdown", down, true);
    addEventListener("keydown", key, true);
    return () => {
      removeEventListener("pointerdown", down, true);
      removeEventListener("keydown", key, true);
    };
  }, [onClose]);

  const card =
    "pointer-events-auto rounded-xl border border-border bg-[var(--assistant-card)] shadow-level3 backdrop-blur-[28px] backdrop-saturate-[1.6] animate-[assistant-rise_400ms_cubic-bezier(0.2,0.9,0.3,1.2)_both]";
  return (
    <div
      role="dialog"
      aria-label={t("assistant.title")}
      className={`pointer-events-none fixed inset-0 z-50 transition-opacity duration-400 ${state.leaving ? "opacity-0" : ""}`}
    >
      <div
        data-assistant
        className="pointer-events-auto absolute bottom-[calc(var(--lift)+16px+env(safe-area-inset-bottom))] left-1/2 flex -translate-x-1/2 flex-col-reverse items-center gap-2 sm:top-3 sm:right-3 sm:bottom-auto sm:left-auto sm:translate-x-0 sm:flex-row-reverse"
        style={{ "--lift": `${lift}px` } as React.CSSProperties}
      >
        <button
          type="button"
          aria-label={state.listening ? t("assistant.stopListening") : t("assistant.talk")}
          onPointerDown={(e) => {
            if (e.button !== 0) return;
            e.preventDefault();
            onPress();
          }}
          onPointerUp={onRelease}
          onPointerCancel={onRelease}
          onClick={(e) => {
            // A keyboard's or a remote's press taps it.
            if (e.detail === 0) (onPress(), onRelease());
          }}
          className={`rounded-full transition-transform duration-400 ${state.leaving ? "scale-40" : ""}`}
        >
          <Orb mode={mode} level={level} interrupts={state.interrupts} />
        </button>
        {status && (
          <span className="rounded-full border border-border bg-[var(--assistant-card)] px-3 py-1 type-footnote whitespace-nowrap backdrop-blur-xl">{status}</span>
        )}
      </div>
      <div
        data-assistant
        aria-live="polite"
        onPointerDown={onHold}
        onPointerEnter={onHold}
        onPointerLeave={onLetGo}
        className="pointer-events-none absolute inset-x-1 bottom-[calc(var(--lift)+144px+env(safe-area-inset-bottom))] flex p-1 max-h-[calc(100dvh-200px)] flex-col-reverse gap-2 overflow-y-auto sm:top-[104px] sm:right-4 sm:bottom-auto sm:left-auto sm:max-h-[calc(100dvh-120px)] sm:w-[380px] sm:flex-col"
        style={{ "--lift": `${lift}px` } as React.CSSProperties}
      >
        {off && <p className={`${card} px-4 py-3.5 type-body text-text-secondary`}>{t("assistant.off")}</p>}
        {(state.heard || state.answer) && (
          <div className={`${card} px-4 py-3.5 type-body`}>
            {state.heard && (
              <p className="text-text-secondary">
                {state.heard}
                {!state.userSpeaking && state.heardAt && <span className="ms-1 text-text-tertiary">›</span>}
              </p>
            )}
            {state.answer && (
              <Spoken
                text={state.answer}
                className={state.heard ? "mt-2.5" : ""}
                live={state.voiced && (state.speaking || state.working || !state.answerAt)}
                speech={speech}
              />
            )}
          </div>
        )}
        {state.activity.length > 0 && (
          <ul className={`${card} flex flex-col gap-2 px-3.5 py-3`}>
            {state.activity.map((a) => (
              <li key={a.id} className="flex items-center gap-2.5 type-footnote text-text-secondary">
                <span className="flex size-[26px] shrink-0 items-center justify-center rounded-sm bg-accent/16 text-accent">
                  <Icon name="search" size={14} />
                </span>
                <span className="min-w-0 flex-1">{a.summary}</span>
                {a.failed ? (
                  <span className="text-danger">
                    <Icon name="close" size={16} />
                  </span>
                ) : a.done ? (
                  <span className="text-success">
                    <Icon name="check" size={16} />
                  </span>
                ) : (
                  <span className="size-4 animate-spin rounded-full border-2 border-fill border-t-accent" />
                )}
              </li>
            ))}
          </ul>
        )}
        {state.items.length > 0 && (
          <ul className={`${card} flex flex-col gap-0.5 p-2.5`}>
            {state.items.map((item, i) => (
              <li key={item.id}>
                <button type="button" onClick={() => onChoose(item)} className="flex w-full items-center gap-2.5 rounded-md p-1.5 text-start hover:bg-fill">
                  {state.choose && <span className="w-4 shrink-0 text-center type-footnote font-semibold text-text-tertiary">{i + 1}</span>}
                  <Artwork src={artwork(server, item, ImageKind.PRIMARY, 80)?.url} alt="" className="aspect-[2/3] w-[34px] shrink-0 rounded-[5px]" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate type-callout font-medium">{item.name}</span>
                    {subtitleOf(item) && <span className="block truncate type-caption text-text-secondary">{subtitleOf(item)}</span>}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
        {state.confirmation && (
          <div className={`${card} flex flex-col gap-3 px-4 py-3.5`}>
            <p className="type-body">{state.confirmation.text}</p>
            <div className="flex gap-2">
              <button type="button" onClick={() => onConfirm(false)} className="min-h-9 flex-1 rounded-md bg-fill type-callout hover:bg-fill-selected">
                {t("action.cancel")}
              </button>
              <button type="button" onClick={() => onConfirm(true)} className="min-h-9 flex-1 rounded-md bg-accent type-callout text-on-accent hover:opacity-90">
                {t("assistant.confirm")}
              </button>
            </div>
          </div>
        )}
        {state.error && <p className={`${card} px-4 py-3.5 type-body break-words text-danger`}>{state.error}</p>}
      </div>
    </div>
  );
}

// How long a character takes to say, roughly: the answer's words are lit by
// these against how much of its speech has played.
const cjk = /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/u;
function sayMs(segment: string): number {
  let ms = 0;
  for (const ch of segment) ms += cjk.test(ch) ? 230 : /[\p{L}\p{N}]/u.test(ch) ? 62 : /[,.;:!?，。；：！？、]/u.test(ch) ? 180 : 0;
  return ms;
}

/** The answer, its words lit as its speech plays; all lit when it is not spoken. */
function Spoken({
  text,
  className,
  live,
  speech,
}: {
  text: string;
  className: string;
  live: boolean;
  speech: () => { played: number; total: number } | undefined;
}) {
  const segments = useMemo(() => {
    let at = 0;
    return [...new Intl.Segmenter(locale, { granularity: "word" }).segment(text)].map((x) => ({ w: x.segment, at: (at += sayMs(x.segment)) }));
  }, [text]);
  const [played, setPlayed] = useState(0);
  useEffect(() => {
    if (!live) return;
    let frame = 0;
    const tick = () => {
      // Lit words stay lit.
      setPlayed((p) => Math.max(p, speech()?.played ?? 0));
      frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [live, speech]);
  return (
    <p className={className}>
      {segments.map((x, i) => (
        <span key={i} className={`transition-colors duration-200 ${!live || x.at <= played + 120 ? "text-text-primary" : "text-text-tertiary"}`}>
          {x.w}
        </span>
      ))}
    </p>
  );
}

/**
 * The assistant button of the page's chrome, shown when the server's
 * assistant is on: it starts a voice session, held or tapped.
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
      data-assistant-button
      title={tip ? undefined : t("assistant.title")}
      onPointerDown={(e) => {
        if (e.button !== 0) return;
        e.preventDefault();
        a.press();
      }}
      onPointerUp={a.release}
      onPointerCancel={a.release}
      onClick={(e) => {
        // A keyboard's or a remote's press taps it, starting a conversation.
        if (e.detail === 0) (a.press(), a.release());
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

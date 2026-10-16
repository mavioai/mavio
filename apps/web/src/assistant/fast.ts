// The web UI's fast path (docs/assistant.md §3, §4): the grammar of
// libs/intent, bundled as its WASM module, reads a request first; what it
// reads as a command for this page's player or a plain move of the page is
// carried out here, with no request to the server.
import { Grammar, slotKeyword, slotNumber } from "@mavio/client-ts";
import { type Intent, IntentMatch } from "@mavio/proto/mavio/intent/v1/intent_pb";
import intentUrl from "@mavio/intent/intent.wasm?url";

import { openPlayer } from "./player.ts";

let loading: Promise<Grammar | undefined> | undefined;

/** The grammar, loaded as it is first needed; undefined when it cannot load, and the server reads requests alone. */
export function grammar(): Promise<Grammar | undefined> {
  loading ??= Grammar.load(fetch(intentUrl)).catch(() => undefined);
  return loading;
}

/** A reply of the grammar: its key and arguments. */
export interface Reply {
  key: string;
  args?: Record<string, string>;
}

/** The moves of the page the fast path makes. */
export interface Moves {
  home(): void;
  back(): void;
  settings(): void;
}

/** A number of seconds said with the grammar's units, as the server says it. */
export function spokenDuration(g: Grammar, lang: string, secs: number): string {
  const s = Math.round(Math.abs(secs));
  const parts = [
    [Math.floor(s / 3600), "hours"],
    [Math.floor(s / 60) % 60, "minutes"],
    [s % 60, "seconds"],
  ] as const;
  const said = parts.filter(([n]) => n > 0).map(([n, key]) => g.reply(lang, key, { n: String(n) }));
  return said.length > 0 ? said.join(" ") : g.reply(lang, "seconds", { n: "0" });
}

/** A position as h:mm:ss or m:ss. */
export function clock(secs: number): string {
  const s = Math.round(secs);
  const [h, m, sec] = [Math.floor(s / 3600), Math.floor(s / 60) % 60, s % 60];
  const two = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${two(m)}:${two(sec)}` : `${m}:${two(sec)}`;
}

/**
 * Carries out an intent about this page — its player's controls without a
 * device named, or going home, back or to the settings — and returns its
 * reply; undefined leaves the intent to the server.
 */
export function runLocal(g: Grammar, intent: Intent, moves: Moves): Reply | undefined {
  if (intent.match !== IntentMatch.FULL || intent.slots.some((s) => s.name === "device")) return undefined;
  switch (intent.name) {
    case "go_home":
      moves.home();
      return { key: "home" };
    case "go_back":
      moves.back();
      return { key: "back" };
    case "open_settings":
      moves.settings();
      return { key: "settings" };
  }
  const p = openPlayer();
  if (!p) return undefined;
  const st = p.state();
  const lang = intent.language;
  switch (intent.name) {
    case "pause":
      p.pause();
      return { key: "paused" };
    case "resume":
      p.resume();
      return { key: "resumed" };
    case "stop":
      p.stop();
      return { key: "stopped" };
    case "mute":
      p.mute(true);
      return { key: "muted" };
    case "unmute":
      p.mute(false);
      return { key: "unmuted" };
    case "fullscreen":
      p.fullScreen();
      return { key: "fullscreen" };
    case "seek_back":
    case "seek_forward": {
      const secs = slotNumber(intent, "seconds") ?? 10;
      p.seek(st.position + (intent.name === "seek_back" ? -secs : secs));
      return { key: intent.name === "seek_back" ? "seeked_back" : "seeked_forward", args: { duration: spokenDuration(g, lang, secs) } };
    }
    case "seek_to": {
      const pos = slotNumber(intent, "position") ?? 0;
      p.seek(pos);
      return { key: "seeked_to", args: { position: clock(pos) } };
    }
    case "restart":
      p.seek(0);
      return { key: "restarted" };
    case "speed": {
      let speed = slotNumber(intent, "speed");
      if (speed === undefined) {
        const change = slotKeyword(intent, "change");
        speed = change === "up" ? Math.min(st.speed + 0.25, 3) : change === "down" ? Math.max(st.speed - 0.25, 0.25) : 1;
      }
      p.speed(speed);
      return { key: "speed", args: { speed: String(speed) } };
    }
    case "volume": {
      let level = slotNumber(intent, "level");
      if (level === undefined) level = st.volume + (slotKeyword(intent, "change") === "down" ? -10 : 10);
      level = Math.min(Math.max(Math.round(level), 0), 100);
      p.volume(level);
      return { key: "volume", args: { level: String(level) } };
    }
    case "subtitles_off":
      p.streams(undefined, -1);
      return { key: "subtitles_off" };
  }
  return undefined;
}

// Dictation (docs/clients.md §6.3.26): a field's text said rather than
// typed, heard by a voice session in dictation mode (docs/assistant.md
// §8.3), which sends the final transcripts of its turns and asks and
// speaks nothing.
import { errorMessage, type ServerEntry, startVoiceSession, useAssistant, useServersStore } from "@mavio/client-ts";
import { VoiceMode } from "@mavio/proto/mavio/plugin/v1/voice_pb";
import { useCallback, useEffect, useRef, useState } from "react";

import { locale, t } from "../i18n.ts";
import { isDenied, microphoneAllowed, VoiceSession } from "./voice.ts";

/** The language spoken: the browser's of the interface's language, else the interface's. */
const speechLanguage = (typeof navigator === "undefined" ? [] : navigator.languages).find((l) => l.split("-")[0] === locale.split("-")[0]) ?? locale;

/** A dictation: whether it listens, what it heard so far, and why it could not. */
export interface Dictation {
  listening: boolean;
  heard: string;
  error?: string;
  /** Listens, or, while listening, stops. */
  toggle(): void;
  /** Clears the error. */
  dismiss(): void;
}

/** Listens on a server, giving each turn recognized to onText. */
export function useDictation(server: ServerEntry | undefined, onText: (text: string) => void): Dictation {
  const servers = useServersStore();
  const info = useAssistant(server, locale);
  const [state, setState] = useState<{ listening: boolean; heard: string; error?: string }>({ listening: false, heard: "" });
  const current = useRef<VoiceSession | undefined>(undefined);
  const latest = useRef(onText);
  latest.current = onText;
  // Leaving drops what is being said.
  useEffect(() => () => current.current?.close(), []);

  const listen = useCallback(async () => {
    if (!microphoneAllowed()) {
      setState({ listening: false, heard: "", error: t("assistant.insecure") });
      return;
    }
    if (!server || !info.data?.voice) {
      setState({ listening: false, heard: "", error: t("assistant.noVoice") });
      return;
    }
    setState({ listening: true, heard: "" });
    try {
      const v = await VoiceSession.capture({
        event(e) {
          if (e.type !== "transcript" || typeof e.text !== "string") return;
          if (!e.final) setState((s) => ({ ...s, heard: e.text as string }));
          else {
            setState((s) => ({ ...s, heard: "" }));
            if (e.text.trim()) latest.current(e.text.trim());
          }
        },
        closed(error) {
          if (current.current === v) current.current = undefined;
          setState({ listening: false, heard: "", error });
        },
        silent(device) {
          setState((s) => ({ ...s, error: t("assistant.silentMicrophone", { device: device || t("assistant.microphone") }) }));
        },
      });
      current.current = v;
      v.connect(
        startVoiceSession(servers, server, { language: speechLanguage, mode: VoiceMode.DICTATION }).then((r) => r.url),
        false,
      );
    } catch (err) {
      setState({
        listening: false,
        heard: "",
        error: isDenied(err) ? t("assistant.microphoneDenied") : t("assistant.noMicrophone", { error: errorMessage(err) }),
      });
    }
  }, [servers, server, info.data]);

  const toggle = useCallback(() => {
    if (current.current) current.current.close();
    else if (!state.listening) void listen();
  }, [listen, state.listening]);

  const dismiss = useCallback(() => setState((s) => ({ ...s, error: undefined })), []);

  return { ...state, toggle, dismiss };
}

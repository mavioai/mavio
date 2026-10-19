// Dictation (docs/clients.md §6.3.26): a field's text said rather than
// typed, heard by the recognizers the assistant listens with — the server's,
// else the browser's, in the order the assistant's settings prefer.
import { errorMessage, type ServerEntry, startListening, useAssistant, useServersStore } from "@mavio/client-ts";
import { useCallback, useEffect, useRef, useState } from "react";

import { locale, t } from "../i18n.ts";
import { usePrefs } from "../prefs.ts";
import { browserRecognizes, denied, isDenied, type Listening, listenInBrowser, listenOnServer, microphoneAllowed } from "./listen.ts";

/** The language spoken: the browser's of the interface's language, else the interface's. */
const speechLanguage = (typeof navigator === "undefined" ? [] : navigator.languages).find((l) => l.split("-")[0] === locale.split("-")[0]) ?? locale;

/** A dictation: whether it listens, what it heard so far, and why it could not. */
export interface Dictation {
  listening: boolean;
  heard: string;
  error?: string;
  /** Listens, or, while listening, ends the speech, which is then recognized. */
  toggle(): void;
}

/** Listens on a server or in the browser, giving each utterance recognized to onText. */
export function useDictation(server: ServerEntry | undefined, onText: (text: string) => void): Dictation {
  const servers = useServersStore();
  const info = useAssistant(server, locale);
  const { assistantRecognition } = usePrefs();
  const [state, setState] = useState<{ listening: boolean; heard: string; error?: string }>({ listening: false, heard: "" });
  const current = useRef<Listening | undefined>(undefined);
  const latest = useRef(onText);
  latest.current = onText;
  // Leaving drops what is being said.
  useEffect(() => () => current.current?.cancel(), []);

  const listen = useCallback(async () => {
    if (!microphoneAllowed()) {
      setState({ listening: false, heard: "", error: t("assistant.insecure") });
      return;
    }
    const onServer = !!server && !!info.data?.recognizers.length;
    const order = (assistantRecognition === "device" ? ["device", "server"] : ["server", "device"]).filter((r) =>
      r === "server" ? onServer : browserRecognizes(),
    );
    if (order.length === 0) {
      setState({ listening: false, heard: "", error: t("assistant.noRecognizer") });
      return;
    }
    setState({ listening: true, heard: "" });
    const hearing = {
      partial: (text: string) => setState((s) => ({ ...s, heard: text })),
      final: (text: string) => {
        current.current = undefined;
        setState({ listening: false, heard: "" });
        if (text) latest.current(text);
      },
      error: (message: string) => {
        current.current = undefined;
        setState({ listening: false, heard: "", error: message === denied ? t("assistant.microphoneDenied") : message });
      },
    };
    for (const r of order) {
      try {
        current.current =
          r === "server"
            ? await listenOnServer(async () => (await startListening(servers, server!, speechLanguage)).url, hearing)
            : listenInBrowser(speechLanguage, hearing);
        return;
      } catch (err) {
        // A recognizer that fails falls back to the other.
        if (r === order.at(-1))
          setState({
            listening: false,
            heard: "",
            error: isDenied(err) ? t("assistant.microphoneDenied") : t("assistant.noMicrophone", { error: errorMessage(err) }),
          });
      }
    }
  }, [servers, server, info.data, assistantRecognition]);

  const toggle = useCallback(() => {
    if (current.current) current.current.stop();
    else if (!state.listening) void listen();
  }, [listen, state.listening]);

  return { ...state, toggle };
}

// Playing the speech the server kept of the assistant's answers
// (docs/assistant.md §8.8): only those files, never a voice speaking again.
import { keptSpeech, type ServerEntry, useServersStore } from "@mavio/client-ts";
import { useEffect, useRef, useState } from "react";

/** Plays the kept speech of an answer, one answer at a time. */
export function useKeptSpeech(server: ServerEntry) {
  const servers = useServersStore();
  const [at, setAt] = useState<string>();
  const run = useRef<{ abort: AbortController; audio?: HTMLAudioElement }>(undefined);
  const stop = () => {
    run.current?.abort.abort();
    run.current?.audio?.pause();
    run.current = undefined;
    setAt(undefined);
  };
  useEffect(() => stop, []);
  /** Plays the speech at url as the answer key, or stops it when it plays. */
  const toggle = (key: string, url: string) => {
    const again = at === key;
    stop();
    if (again) return;
    const r = { abort: new AbortController() } as { abort: AbortController; audio?: HTMLAudioElement };
    run.current = r;
    setAt(key);
    void (async () => {
      const blob = await keptSpeech(servers, server, url, r.abort.signal).catch(() => undefined);
      if (blob && !r.abort.signal.aborted) {
        const src = URL.createObjectURL(blob);
        const audio = new Audio(src);
        r.audio = audio;
        await new Promise<void>((resolve) => {
          audio.onended = audio.onerror = audio.onpause = () => resolve();
          audio.play().catch(() => resolve());
        });
        URL.revokeObjectURL(src);
      }
      if (run.current === r) {
        run.current = undefined;
        setAt(undefined);
      }
    })();
  };
  return { at, toggle };
}

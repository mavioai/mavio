// Playing the speech the server kept of the assistant's answers
// (docs/assistant.md §8.3): only those files, never a voice speaking again.
import { keptSpeech, type ServerEntry, useServersStore } from "@mavio/client-ts";
import { useEffect, useRef, useState } from "react";

/** Plays the kept speech of an answer, a sentence after another, one answer at a time. */
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
  const toggle = (key: string, urls: string[]) => {
    const again = at === key;
    stop();
    if (again) return;
    const r = { abort: new AbortController() } as { abort: AbortController; audio?: HTMLAudioElement };
    run.current = r;
    setAt(key);
    void (async () => {
      // Each sentence is fetched while the one before plays.
      let next = keptSpeech(servers, server, urls[0], r.abort.signal);
      for (let j = 0; j < urls.length && !r.abort.signal.aborted; j++) {
        const blob = await next.catch(() => undefined);
        if (j + 1 < urls.length) next = keptSpeech(servers, server, urls[j + 1], r.abort.signal);
        if (!blob || r.abort.signal.aborted) continue;
        const url = URL.createObjectURL(blob);
        const audio = new Audio(url);
        r.audio = audio;
        await new Promise<void>((resolve) => {
          audio.onended = audio.onerror = audio.onpause = () => resolve();
          audio.play().catch(() => resolve());
        });
        URL.revokeObjectURL(url);
      }
      if (run.current === r) {
        run.current = undefined;
        setAt(undefined);
      }
    })();
  };
  return { at, toggle };
}

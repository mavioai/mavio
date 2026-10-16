// Speaking answers (docs/assistant.md §8.3): a sentence at a time as the
// answer arrives, with the browser's voice for the language, or a server
// voice when the user chose one or the browser has none. Speaking stops
// when the user talks or presses the button again.
import { type ServerEntry, type Servers, speakWithServer } from "@mavio/client-ts";

/** The browser's voice for a language, if it has one. */
function deviceVoice(language: string): SpeechSynthesisVoice | undefined {
  if (typeof speechSynthesis === "undefined") return undefined;
  const voices = speechSynthesis.getVoices();
  const lang = language.toLowerCase();
  const base = lang.split("-")[0];
  return (
    voices.find((v) => v.lang.toLowerCase() === lang) ??
    voices.find((v) => v.lang.toLowerCase().split(/[-_]/)[0] === base && v.localService) ??
    voices.find((v) => v.lang.toLowerCase().split(/[-_]/)[0] === base)
  );
}

/** Where a speaker's server voices come from. */
export interface ServerVoices {
  servers: Servers;
  server: ServerEntry;
  /** The server's voices' languages, to tell whether one speaks a language. */
  languages: string[];
}

/** Speaks sentences in order, until stopped. */
export class Speaker {
  // Each sentence's audio from the server, fetched as it is said so that
  // the next is ready when one ends.
  private queue: Promise<Blob | undefined>[] = [];
  private playing = false;
  private audio?: HTMLAudioElement;
  private abort = new AbortController();

  /** voice is a server voice's ID, or "" for the browser's. */
  constructor(
    private readonly voice: string,
    private readonly server?: ServerVoices,
  ) {}

  say(sentence: string, language: string) {
    const browser = this.voice === "" ? deviceVoice(language) : undefined;
    if (browser || (this.voice === "" && !this.serverSpeaks(language))) {
      if (typeof speechSynthesis === "undefined") return;
      const u = new SpeechSynthesisUtterance(sentence);
      u.lang = language;
      if (browser) u.voice = browser;
      speechSynthesis.speak(u);
      return;
    }
    const s = this.server;
    if (!s) return;
    this.queue.push(
      speakWithServer(s.servers, s.server, { text: sentence, language, voice: this.voice }, this.abort.signal).catch(() => undefined),
    );
    void this.drain();
  }

  stop() {
    this.abort.abort();
    this.abort = new AbortController();
    this.queue = [];
    this.audio?.pause();
    this.audio = undefined;
    if (typeof speechSynthesis !== "undefined") speechSynthesis.cancel();
  }

  private serverSpeaks(language: string): boolean {
    const base = language.split("-")[0].toLowerCase();
    return !!this.server?.languages.some((l) => l.toLowerCase().split("-")[0] === base);
  }

  private async drain() {
    if (this.playing) return;
    this.playing = true;
    try {
      while (this.queue.length > 0) {
        const signal = this.abort.signal;
        const blob = await this.queue.shift()!;
        if (!blob || signal.aborted) continue;
        const url = URL.createObjectURL(blob);
        const audio = new Audio(url);
        this.audio = audio;
        await new Promise<void>((resolve) => {
          audio.onended = audio.onerror = audio.onpause = () => resolve();
          audio.play().catch(() => resolve());
        });
        URL.revokeObjectURL(url);
      }
    } finally {
      this.playing = false;
    }
  }
}

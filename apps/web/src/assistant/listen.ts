// Listening (docs/assistant.md §8.1, §9.2): the microphone streamed to the
// server's recognizer as 16 kHz mono 16-bit PCM, which an AudioWorklet
// resamples, or the browser's own SpeechRecognition. Browsers give the
// microphone only to secure pages.
import type { Heard } from "@mavio/client-ts";

/** What a recognizer tells as it listens. */
export interface Hearing {
  /** The text heard so far. */
  partial(text: string): void;
  /** The utterance ended; empty when nothing was said. */
  final(text: string, language: string): void;
  error(message: string): void;
}

/** An utterance being listened to. */
export interface Listening {
  /** The user let go: the speech ends and is recognized. */
  stop(): void;
  /** Drops the utterance. */
  cancel(): void;
}

/** Whether the page may open the microphone. */
export function microphoneAllowed(): boolean {
  return window.isSecureContext && !!navigator.mediaDevices?.getUserMedia;
}

// The worklet averages the input over each output sample's span, which
// keeps what is above 8 kHz from folding into speech, and posts about a
// tenth of a second at a time.
const worklet = `
class Capture extends AudioWorkletProcessor {
  constructor() {
    super();
    this.ratio = sampleRate / 16000;
    this.acc = 0; this.sum = 0; this.count = 0;
    this.out = new Int16Array(1600); this.n = 0;
  }
  process(inputs) {
    const ch = inputs[0] && inputs[0][0];
    if (!ch) return true;
    for (let i = 0; i < ch.length; i++) {
      this.sum += ch[i]; this.count++; this.acc += 1;
      if (this.acc < this.ratio) continue;
      this.acc -= this.ratio;
      const s = Math.max(-1, Math.min(1, this.sum / this.count));
      this.sum = 0; this.count = 0;
      this.out[this.n++] = s < 0 ? s * 0x8000 : s * 0x7fff;
      if (this.n === this.out.length) {
        this.port.postMessage(this.out.buffer, [this.out.buffer]);
        this.out = new Int16Array(1600); this.n = 0;
      }
    }
    return true;
  }
}
registerProcessor("mavio-capture", Capture);
`;

/**
 * Listens through the server's recognizer: open starts listening on the
 * server and returns its WebSocket URL. Audio captured before the socket
 * opens is kept and sent when it does.
 */
export async function listenOnServer(open: () => Promise<string>, h: Hearing): Promise<Listening> {
  const media = await navigator.mediaDevices.getUserMedia({ audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true } });
  const ctx = new AudioContext();
  let ws: WebSocket | undefined;
  let done = false;
  const pending: ArrayBuffer[] = [];
  const release = () => {
    media.getTracks().forEach((t) => t.stop());
    void ctx.close().catch(() => {});
  };
  const finish = () => {
    done = true;
    release();
    ws?.close();
  };
  try {
    const url = URL.createObjectURL(new Blob([worklet], { type: "text/javascript" }));
    await ctx.audioWorklet.addModule(url);
    URL.revokeObjectURL(url);
    const node = new AudioWorkletNode(ctx, "mavio-capture");
    node.port.onmessage = (e: MessageEvent<ArrayBuffer>) => {
      if (ws?.readyState === WebSocket.OPEN) ws.send(e.data);
      else pending.push(e.data);
    };
    ctx.createMediaStreamSource(media).connect(node);
  } catch (err) {
    release();
    throw err;
  }
  open()
    .then((url) => {
      if (done) return;
      ws = new WebSocket(url);
      ws.binaryType = "arraybuffer";
      ws.onopen = () => {
        for (const b of pending.splice(0)) ws!.send(b);
        // The user let go before the socket opened.
        if (ctx.state === "closed") ws!.send("end");
      };
      ws.onmessage = (e) => {
        const heard = JSON.parse(String(e.data)) as Heard;
        if (heard.error) {
          finish();
          h.error(heard.error);
        } else if (heard.final) {
          finish();
          h.final(heard.text ?? "", heard.language ?? "");
        } else if (heard.text) h.partial(heard.text);
      };
      ws.onclose = () => {
        if (done) return;
        done = true;
        release();
        h.error("the connection closed");
      };
    })
    .catch((err: unknown) => {
      if (done) return;
      finish();
      h.error(err instanceof Error ? err.message : String(err));
    });
  return {
    stop() {
      if (done) return;
      release();
      if (ws?.readyState === WebSocket.OPEN) ws.send("end");
    },
    cancel: finish,
  };
}

// SpeechRecognition, which TypeScript's DOM types lack.
interface Recognition {
  lang: string;
  interimResults: boolean;
  continuous: boolean;
  onresult: ((e: { results: ArrayLike<ArrayLike<{ transcript: string }> & { isFinal: boolean }> }) => void) | null;
  onerror: ((e: { error: string }) => void) | null;
  onend: (() => void) | null;
  start(): void;
  stop(): void;
  abort(): void;
}

type RecognitionClass = new () => Recognition;

function recognitionClass(): RecognitionClass | undefined {
  const w = window as unknown as { SpeechRecognition?: RecognitionClass; webkitSpeechRecognition?: RecognitionClass };
  return w.SpeechRecognition ?? w.webkitSpeechRecognition;
}

/** Whether the browser has a recognizer of its own. */
export function browserRecognizes(): boolean {
  return !!recognitionClass();
}

/** What Hearing.error is told when the microphone is refused or missing. */
export const denied = "microphone denied";

/** Reports whether an error opening the microphone is its refusal. */
export function isDenied(err: unknown): boolean {
  return err instanceof DOMException && (err.name === "NotAllowedError" || err.name === "NotFoundError");
}

/** Listens through the browser's SpeechRecognition, in a language. */
export function listenInBrowser(language: string, h: Hearing): Listening {
  const Class = recognitionClass();
  if (!Class) throw new Error("no SpeechRecognition");
  const rec = new Class();
  rec.lang = language;
  rec.interimResults = true;
  rec.continuous = false;
  let text = "";
  let ended = false;
  rec.onresult = (e) => {
    text = Array.from(e.results, (r) => r[0]?.transcript ?? "").join("");
    h.partial(text);
  };
  rec.onerror = (e) => {
    if (ended) return;
    ended = true;
    // Nothing said is an empty utterance, not a failure.
    if (e.error === "no-speech" || e.error === "aborted") h.final("", language);
    else if (e.error === "not-allowed" || e.error === "service-not-allowed" || e.error === "audio-capture") h.error(denied);
    else h.error(e.error);
  };
  rec.onend = () => {
    if (ended) return;
    ended = true;
    h.final(text.trim(), language);
  };
  rec.start();
  return {
    stop: () => rec.stop(),
    cancel() {
      ended = true;
      rec.abort();
    },
  };
}

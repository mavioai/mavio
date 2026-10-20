// Voice sessions (docs/assistant.md §8, §9.2): the echo-cancelled
// microphone resampled to 16 kHz mono 16-bit PCM by an AudioWorklet and
// sent in 20 ms frames on the session's WebSocket, and the answer's PCM
// played through the same AudioContext, whose output is what the browser's
// echo cancellation removes. Browsers give the microphone only to secure
// pages.

/** An event of the session's protocol (§8.6). */
export interface VoiceEvent {
  type: string;
  [field: string]: unknown;
}

/** What a session tells as it goes. */
export interface VoiceHandlers {
  /** Every event the session sends. */
  event(e: VoiceEvent): void;
  /** The session ended; error says why when it failed. */
  closed(error?: string): void;
  /** The microphone's first audio came: what is said from now on is heard. */
  capturing?(): void;
}

/** Whether the page may open the microphone. */
export function microphoneAllowed(): boolean {
  return window.isSecureContext && !!navigator.mediaDevices?.getUserMedia;
}

/** Reports whether an error opening the microphone is its refusal. */
export function isDenied(err: unknown): boolean {
  return err instanceof DOMException && (err.name === "NotAllowedError" || err.name === "NotFoundError");
}

// The worklet averages the input over each output sample's span, which
// keeps what is above 8 kHz from folding into speech, and posts 20 ms at a
// time.
const worklet = `
class Capture extends AudioWorkletProcessor {
  constructor() {
    super();
    this.ratio = sampleRate / 16000;
    this.acc = 0; this.sum = 0; this.count = 0;
    this.out = new Int16Array(320); this.n = 0;
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
        this.out = new Int16Array(320); this.n = 0;
      }
    }
    return true;
  }
}
registerProcessor("mavio-capture", Capture);
`;

/** How loud an analyser's signal is now, from 0 to about 1. */
function loudness(a: AnalyserNode, buf: Float32Array<ArrayBuffer>): number {
  a.getFloatTimeDomainData(buf);
  let sum = 0;
  for (const v of buf) sum += v * v;
  // Speech's RMS spans roughly 0.01 to 0.3: its square root spreads it out.
  return Math.min(1, Math.sqrt(Math.sqrt(sum / buf.length)) * 1.5);
}

/** The answers' speech, played as it arrives, which an interrupt drops. */
class Playback {
  /** The rate of the speech, from the session's ready event. */
  rate = 24000;
  private next = 0;
  private sources: { node: AudioBufferSourceNode; start: number; duration: number }[] = [];
  /** Where the speech plays, through an analyser of its level. */
  readonly out: GainNode;
  readonly analyser: AnalyserNode;

  constructor(private readonly ctx: AudioContext) {
    this.out = ctx.createGain();
    this.out.connect(ctx.destination);
    this.analyser = ctx.createAnalyser();
    this.analyser.fftSize = 512;
    this.out.connect(this.analyser);
  }

  /** Plays 16-bit PCM after what was queued. */
  push(pcm: ArrayBuffer) {
    const samples = new Int16Array(pcm);
    if (samples.length === 0) return;
    const buffer = this.ctx.createBuffer(1, samples.length, this.rate);
    const ch = buffer.getChannelData(0);
    for (let i = 0; i < samples.length; i++) ch[i] = samples[i] / 0x8000;
    const node = this.ctx.createBufferSource();
    node.buffer = buffer;
    node.connect(this.out);
    const start = Math.max(this.next, this.ctx.currentTime);
    node.start(start);
    this.next = start + buffer.duration;
    this.sources.push({ node, start, duration: buffer.duration });
  }

  /** How much of the run's speech has played and has come, in milliseconds. */
  progress(): { played: number; total: number } {
    const now = this.ctx.currentTime;
    let played = 0;
    let total = 0;
    for (const s of this.sources) {
      played += Math.min(s.duration, Math.max(0, now - s.start));
      total += s.duration;
    }
    return { played: played * 1000, total: total * 1000 };
  }

  /** A run starts: what it plays is counted from here. */
  run() {
    const now = this.ctx.currentTime;
    this.sources = this.sources.filter((s) => s.start + s.duration > now);
  }

  /** Drops what was not played, returning how much of the run's speech was, in milliseconds. */
  drop(): number {
    const now = this.ctx.currentTime;
    let played = 0;
    for (const s of this.sources) {
      played += Math.min(s.duration, Math.max(0, now - s.start));
      try {
        s.node.stop();
      } catch {
        // Not started yet.
      }
    }
    this.sources = [];
    this.next = 0;
    return Math.round(played * 1000);
  }
}

/**
 * A voice session from the client's side: capture starts at once and is
 * held until the session's WebSocket opens.
 */
export class VoiceSession {
  private ws?: WebSocket;
  private pending: ArrayBuffer[] = [];
  private ended = false;
  private captured = false;
  // Whether the session said it is ready: one closed before failed.
  private ready = false;
  // Push to talk: whether the user talks, and what the socket was told.
  private talking = false;
  private told = false;
  private readonly playback: Playback;
  private input?: AnalyserNode;
  private readonly buf = new Float32Array(512);

  private constructor(
    private readonly media: MediaStream,
    private readonly ctx: AudioContext,
    private readonly h: VoiceHandlers,
    /** The session listens only while the user holds to talk. */
    private pushToTalk = false,
  ) {
    this.playback = new Playback(ctx);
  }

  /** Opens the microphone; fails when it is refused or missing. */
  static async capture(h: VoiceHandlers): Promise<VoiceSession> {
    // The audio context is made in the press that opens the session, which
    // lets it play, and readied while the microphone opens.
    const ctx = new AudioContext();
    const url = URL.createObjectURL(new Blob([worklet], { type: "text/javascript" }));
    const loaded = ctx.audioWorklet.addModule(url);
    let media: MediaStream;
    try {
      [media] = await Promise.all([
        navigator.mediaDevices.getUserMedia({ audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true } }),
        loaded,
      ]);
    } catch (err) {
      void ctx.close().catch(() => {});
      throw err;
    } finally {
      URL.revokeObjectURL(url);
    }
    const v = new VoiceSession(media, ctx, h);
    try {
      const node = new AudioWorkletNode(ctx, "mavio-capture");
      node.port.onmessage = (e: MessageEvent<ArrayBuffer>) => v.audio(e.data);
      const source = ctx.createMediaStreamSource(media);
      source.connect(node);
      v.input = ctx.createAnalyser();
      v.input.fftSize = 512;
      source.connect(v.input);
    } catch (err) {
      v.release();
      throw err;
    }
    return v;
  }

  /** Whether the session listens only while the user holds to talk. */
  get holdsToTalk(): boolean {
    return this.pushToTalk;
  }

  /** How loud the microphone is now, from 0 to about 1. */
  inputLevel(): number {
    return this.input && !this.ended ? loudness(this.input, this.buf) : 0;
  }

  /** How loud the answer's speech is now, from 0 to about 1. */
  outputLevel(): number {
    return this.ended ? 0 : loudness(this.playback.analyser, this.buf);
  }

  /** How much of the answer's speech has played and has come, in milliseconds. */
  speech(): { played: number; total: number } {
    return this.playback.progress();
  }

  /** Connects to the session's WebSocket, once its URL is known. */
  connect(url: Promise<string>, pushToTalk: boolean) {
    this.pushToTalk = pushToTalk;
    url
      .then((u) => {
        if (this.ended) return;
        const ws = new WebSocket(u);
        this.ws = ws;
        ws.binaryType = "arraybuffer";
        ws.onopen = () => {
          this.tell();
          for (const b of this.pending.splice(0)) ws.send(b);
          // The user let go before the socket opened.
          this.tell();
        };
        ws.onmessage = (e) => this.message(e.data as ArrayBuffer | string);
        ws.onclose = () => this.end(this.ready ? undefined : "the voice session could not be reached");
      })
      .catch((err: unknown) => this.end(err instanceof Error ? err.message : String(err)));
  }

  /** Push to talk: the user holds to talk, or lets go. */
  talk(on: boolean) {
    this.talking = on;
    if (on) this.playback.drop();
    if (this.ws?.readyState === WebSocket.OPEN) this.tell();
  }

  /** Sends an event, once the socket is open. */
  send(e: VoiceEvent) {
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(e));
  }

  /** Ends the session. */
  close() {
    this.send({ type: "close" });
    this.ready = true;
    this.end();
  }

  private tell() {
    if (!this.pushToTalk || this.talking === this.told) return;
    this.told = this.talking;
    this.ws!.send(JSON.stringify({ type: this.talking ? "ptt_start" : "ptt_end" }));
  }

  private audio(pcm: ArrayBuffer) {
    if (this.ended) return;
    if (!this.captured) {
      this.captured = true;
      this.h.capturing?.();
    }
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(pcm);
    // The socket opens once the plugin has made the session's services,
    // which can take seconds: what the user says meanwhile is kept.
    else if (!this.ws || this.ws.readyState === WebSocket.CONNECTING) this.pending.push(pcm);
  }

  private message(data: ArrayBuffer | string) {
    if (typeof data !== "string") {
      this.playback.push(data);
      return;
    }
    let e: VoiceEvent;
    try {
      e = JSON.parse(data) as VoiceEvent;
    } catch {
      return;
    }
    switch (e.type) {
      case "ready":
        this.ready = true;
        if (typeof e.output_sample_rate === "number" && e.output_sample_rate > 0) this.playback.rate = e.output_sample_rate;
        break;
      case "run":
        this.playback.run();
        break;
      case "interrupt":
        // The relay keeps the answer's speech as far as it was played (§8.8).
        this.send({ type: "played", ms: this.playback.drop() });
        break;
    }
    this.h.event(e);
  }

  private release() {
    this.media.getTracks().forEach((t) => t.stop());
    void this.ctx.close().catch(() => {});
  }

  private end(error?: string) {
    if (this.ended) return;
    this.ended = true;
    this.release();
    this.ws?.close();
    this.h.closed(error);
  }
}

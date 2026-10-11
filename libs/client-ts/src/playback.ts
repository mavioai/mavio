// Playback in the browser (docs/clients.md §3, §4.1): the capabilities the
// browser declares, the server's decision, and progress reported every 10
// seconds and on pause, seek and stop.
import { create } from "@bufbuild/protobuf";
import { durationFromMs, durationMs } from "@bufbuild/protobuf/wkt";
import {
  type ClientCapabilities,
  ClientCapabilitiesSchema,
  MediaKind,
  PlaybackService,
  Protocol,
  type StartPlaybackResponse,
  SubtitleMethod,
} from "@mavio/proto/mavio/playback/v1/playback_pb";

import { urlOf } from "./server.ts";
import type { ServerEntry, Servers } from "./servers.ts";

/** What the browser can decode, by the codec strings of MSE and the media element. */
export interface Decoders {
  /** Whether a codec string plays through Media Source Extensions, as hls.js plays. */
  mse(type: string): boolean;
  /** Whether the media element plays a type itself, as Safari's native HLS does. */
  native(type: string): boolean;
}

/** The decoders of this browser. */
export function browserDecoders(): Decoders {
  const video = typeof document === "undefined" ? undefined : document.createElement("video");
  return {
    mse: (type) => typeof MediaSource !== "undefined" && MediaSource.isTypeSupported(type),
    native: (type) => !!video && video.canPlayType(type) !== "",
  };
}

/** Whether the browser plays HLS itself rather than through hls.js: Safari, or no MSE. */
export function nativeHls(d: Decoders = browserDecoders()): boolean {
  const ua = typeof navigator === "undefined" ? "" : navigator.userAgent;
  const safari = /^((?!chrome|chromium|android).)*safari/i.test(ua);
  return (safari || !d.mse('video/mp4; codecs="avc1.640028"')) && d.native("application/vnd.apple.mpegurl");
}

const videoTypes: [codec: string, type: string][] = [
  ["h264", 'video/mp4; codecs="avc1.640028"'],
  ["hevc", 'video/mp4; codecs="hvc1.1.6.L120.90"'],
  ["av1", 'video/mp4; codecs="av01.0.08M.08"'],
  ["vp9", 'video/mp4; codecs="vp09.00.40.08"'],
];

const audioTypes: [codec: string, type: string][] = [
  ["aac", 'audio/mp4; codecs="mp4a.40.2"'],
  ["mp3", 'audio/mp4; codecs="mp4a.40.34"'],
  ["opus", 'audio/mp4; codecs="opus"'],
  ["flac", 'audio/mp4; codecs="flac"'],
  ["ac3", 'audio/mp4; codecs="ac-3"'],
  ["eac3", 'audio/mp4; codecs="ec-3"'],
];

/**
 * The capabilities of a browser: the codecs it decodes in MP4 and WebM
 * files, and HLS with fMP4 segments for everything else, through hls.js or
 * the native player; subtitles as WebVTT.
 */
export function capabilities(d: Decoders = browserDecoders(), maxBitrate = 40_000_000): ClientCapabilities {
  const native = nativeHls(d);
  const plays = (type: string) => (native ? d.native(type) || d.mse(type) : d.mse(type));
  const video = videoTypes.filter(([, t]) => plays(t)).map(([c]) => c);
  if (!video.includes("h264")) video.unshift("h264");
  const audio = audioTypes.filter(([, t]) => plays(t)).map(([c]) => c);
  if (!audio.includes("aac")) audio.unshift("aac");
  const webm = d.native('video/webm; codecs="vp9"');
  return create(ClientCapabilitiesSchema, {
    name: native ? "Mavio Web (native HLS)" : "Mavio Web",
    maxStreamingBitrate: BigInt(maxBitrate),
    directPlay: [
      { kind: MediaKind.VIDEO, container: "mp4,m4v,mov", videoCodec: video.join(","), audioCodec: audio.join(",") },
      ...(webm ? [{ kind: MediaKind.VIDEO, container: "webm", videoCodec: "vp8,vp9,av1", audioCodec: "opus,vorbis" }] : []),
      { kind: MediaKind.AUDIO, container: "mp3", audioCodec: "mp3" },
      { kind: MediaKind.AUDIO, container: "m4a,mp4", audioCodec: audio.join(",") },
      { kind: MediaKind.AUDIO, container: "flac", audioCodec: "flac" },
    ],
    transcoding: [
      {
        kind: MediaKind.VIDEO,
        protocol: Protocol.HLS,
        container: "mp4",
        videoCodec: video.join(","),
        audioCodec: audio.join(","),
        maxAudioChannels: 6,
        enableSubtitlesInManifest: true,
        segmentLengthSeconds: 6,
      },
      { kind: MediaKind.AUDIO, protocol: Protocol.HLS, container: "mp4", audioCodec: "aac", maxAudioChannels: 2 },
    ],
    subtitles: [
      { format: "vtt", method: SubtitleMethod.HLS },
      { format: "vtt", method: SubtitleMethod.EXTERNAL },
    ],
  });
}

/** What to play. */
export interface PlayOptions {
  mediaSourceId?: string;
  audioStreamIndex?: number;
  /** -1 turns subtitles off. */
  subtitleStreamIndex?: number;
  /** Where to begin, in milliseconds. */
  startMs?: number;
  capabilities?: ClientCapabilities;
}

/** A playback in progress, reporting its progress to its server. */
export class Playback {
  private timer?: ReturnType<typeof setInterval>;
  private stopped = false;

  private constructor(
    private servers: Servers,
    readonly server: ServerEntry,
    readonly info: StartPlaybackResponse,
    /** The current position in milliseconds and whether playback is paused. */
    private state: () => { positionMs: number; paused: boolean },
  ) {
    this.timer = setInterval(() => void this.report(), 10_000);
  }

  /** Asks the server how to play an item and starts reporting its progress. */
  static async start(
    servers: Servers,
    server: ServerEntry,
    itemId: string,
    options: PlayOptions,
    state: () => { positionMs: number; paused: boolean },
  ): Promise<Playback> {
    const info = await servers.client(PlaybackService, server).startPlayback({
      itemId,
      mediaSourceId: options.mediaSourceId,
      audioStreamIndex: options.audioStreamIndex,
      subtitleStreamIndex: options.subtitleStreamIndex,
      startPosition: options.startMs ? durationFromMs(options.startMs) : undefined,
      capabilities: options.capabilities ?? capabilities(),
    });
    return new Playback(servers, server, info, state);
  }

  /** The media's absolute URL. */
  get url(): string {
    return urlOf(this.server, this.info.url);
  }

  /** Where the media begins, in milliseconds. */
  get startMs(): number {
    return this.info.startPosition ? durationMs(this.info.startPosition) : 0;
  }

  /** Whether the URL is an HLS playlist rather than a file. */
  get hls(): boolean {
    return /\.m3u8(\?|$)/.test(this.info.url);
  }

  /** Reports progress now, as on pause and seek. */
  async report() {
    if (this.stopped) return;
    const { positionMs, paused } = this.state();
    try {
      await this.servers.client(PlaybackService, this.server).reportProgress({
        playbackId: this.info.playbackId,
        position: durationFromMs(Math.max(0, Math.round(positionMs))),
        paused,
      });
    } catch {
      // The next report carries the position.
    }
  }

  /** Ends the playback at the current position. */
  async stop() {
    if (this.stopped) return;
    const { positionMs } = this.state();
    this.stopped = true;
    clearInterval(this.timer);
    try {
      await this.servers.client(PlaybackService, this.server).stopPlayback({
        playbackId: this.info.playbackId,
        position: durationFromMs(Math.max(0, Math.round(positionMs))),
      });
    } catch {
      // The server ends a playback five minutes after its last report.
    }
  }

  /** Ends the playback as the page goes away, with a request that outlives it. */
  stopOnUnload() {
    if (this.stopped) return;
    const { positionMs } = this.state();
    this.stopped = true;
    clearInterval(this.timer);
    void fetch(urlOf(this.server, "mavio.playback.v1.PlaybackService/StopPlayback"), {
      method: "POST",
      keepalive: true,
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${this.server.token ?? ""}` },
      body: JSON.stringify({ playbackId: this.info.playbackId, position: `${(positionMs / 1000).toFixed(3)}s` }),
    });
  }
}

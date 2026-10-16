// The player open in the page, as the assistant sees it (docs/assistant.md
// §5.2): what it plays, for the requests' context, and the controls the
// client actions and the fast path call. The player registers itself while
// it is mounted.

/** What the open player plays. */
export interface PlayingState {
  serverId: string;
  itemId: string;
  /** Seconds. */
  position: number;
  duration: number;
  paused: boolean;
  speed: number;
  /** 0–100. */
  volume: number;
  muted: boolean;
  audioStreamIndex: number;
  subtitleStreamIndex: number;
}

/** What the assistant can ask of the open player. */
export interface PlayerControls {
  state(): PlayingState;
  pause(): void;
  resume(): void;
  stop(): void;
  /** Seconds from the start. */
  seek(seconds: number): void;
  speed(rate: number): void;
  /** 0–100. */
  volume(level: number): void;
  mute(muted: boolean): void;
  fullScreen(): void;
  next(): boolean;
  previous(): void;
  /** Picks streams by their index in the media, -1 for no subtitles; undefined keeps one. */
  streams(audio?: number, subtitle?: number): void;
}

let current: PlayerControls | undefined;

/** Registers the open player until the returned function is called. */
export function registerPlayer(p: PlayerControls): () => void {
  current = p;
  return () => {
    if (current === p) current = undefined;
  };
}

/** The open player, if any. */
export function openPlayer(): PlayerControls | undefined {
  return current;
}

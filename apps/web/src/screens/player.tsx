// The player (docs/clients.md §4.1, §6.5): the server decides how the
// browser plays an item; hls.js plays HLS where Media Source Extensions
// exist, Safari plays it natively. Progress is reported every 10 seconds
// and on pause, seek and stop. As the Mac app's, the controls' row has a
// menu each of the speed, subtitles, audio and quality and of how the item
// plays; on the television a side panel has them. An episode's end starts
// the next.
// Other devices of the user control it remotely (§6.3.23), and it follows
// the SyncPlay group playing its item (§6.3.24). A track shows its lyrics
// (§6.3.27). The assistant controls it (docs/assistant.md §5.2).
import {
  capabilities,
  errorMessage,
  nativeHls,
  Playback,
  type ServerEntry,
  urlOf,
  useItem,
  useLyrics,
  useNextEpisode,
  useServer,
  useServersStore,
} from "@mavio/client-ts";
import { ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { type MediaStream, StreamKind } from "@mavio/proto/mavio/library/v1/media_pb";
import { durationMs } from "@bufbuild/protobuf/wkt";
import { PlayMethod, SubtitleMethod } from "@mavio/proto/mavio/playback/v1/playback_pb";
import { PlayStateCommand } from "@mavio/proto/mavio/session/v1/session_pb";
import { useQueryClient } from "@tanstack/react-query";
import { getRouteApi, useRouter } from "@tanstack/react-router";
import type Hls from "hls.js";
import { type ReactNode, useCallback, useEffect, useRef, useState } from "react";

import { AssistantButton } from "../assistant/panel.tsx";
import { registerPlayer } from "../assistant/player.ts";
import { Slider, Tabs } from "../controls/inputs.tsx";
import { Menu, MenuChoice, MenuFact, MenuItem, MenuLabel, MenuOption, MenuSeparator } from "../controls/menu.tsx";
import { Tip } from "../controls/tooltip.tsx";
import { useServerEvents } from "../events.tsx";
import { locale, t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { prefs, type Quality, qualities, qualityBitrates, setPrefs, usePrefs } from "../prefs.ts";
import { useGroupPlayer } from "../syncplay.tsx";
import { Spinner, StatusView } from "../ui.tsx";
import { LyricsView } from "./lyrics.tsx";
import { PictureInPictureControls, usePictureInPicture } from "./pip.tsx";
import { type ChapterMark, clock, SeekBar } from "./seekbar.tsx";
import { GroupChip } from "./syncplay.tsx";

const route = getRouteApi("/s/$serverId/play/$itemId");

const speeds = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2];

/** Whether the browser lets a video's sound go to a device it picks. */
const sinkable = typeof HTMLMediaElement !== "undefined" && "setSinkId" in HTMLMediaElement.prototype;

/** How long after a click a second one makes it a double click, in milliseconds. */
const doubleClickMs = 300;

/** An offset in seconds, as "+0.3 s". */
function offsetText(s: number): string {
  return s === 0 ? "0 s" : `${s > 0 ? "+" : "−"}${Math.abs(s).toFixed(1)} s`;
}

const languageNames = new Intl.DisplayNames([locale], { type: "language" });

function languageOf(code: string): string {
  try {
    return code && code !== "und" ? (languageNames.of(code) ?? code) : "";
  } catch {
    return code;
  }
}

/** A track's name, and its codec and flags. */
function trackLabel(s: MediaStream): [string, string] {
  const details = [
    s.codec.toUpperCase(),
    s.channels ? (s.channels === 6 ? "5.1" : s.channels === 8 ? "7.1" : `${s.channels} ch`) : "",
    s.forced ? "Forced" : "",
    s.hearingImpaired ? "SDH" : "",
  ]
    .filter(Boolean)
    .join(" · ");
  return [s.title || languageOf(s.language) || `#${s.index}`, details];
}

export function PlayerScreen() {
  const { serverId, itemId } = route.useParams();
  const server = useServer(serverId);
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  return <Player key={itemId} server={server} itemId={itemId} />;
}

/** The streams and quality the user picked, which restart playback when they change. */
interface Selection {
  audio?: number;
  subtitle?: number;
  quality: Quality;
}

type Tab = "audio" | "subtitles" | "quality" | "info";

/** A menu of the desktop's row. */
type RowMenu = Tab | "speed";

function Player({ server, itemId }: { server: ServerEntry; itemId: string }) {
  const servers = useServersStore();
  const router = useRouter();
  const qc = useQueryClient();
  const search = route.useSearch();
  const item = useItem(server, itemId);
  const next = useNextEpisode(server, item.data?.item);
  const video = useRef<HTMLVideoElement>(null);
  const shell = useRef<HTMLDivElement>(null);
  const playback = useRef<Playback>(undefined);
  const [info, setInfo] = useState<Playback>();
  const [error, setError] = useState<string>();
  const [selection, setSelection] = useState<Selection>(() => ({ quality: prefs().quality, audio: search.audio, subtitle: search.subtitle }));
  // Where the next start begins, in milliseconds: the resume position, then where a track change happened.
  const startAt = useRef(search.start ?? 0);
  const [paused, setPaused] = useState(true);
  // Until the video plays, and while it waits for data.
  const [loading, setLoading] = useState(true);
  const [time, setTime] = useState(0);
  // A seek: for a beat nothing changes, so that one within what is buffered
  // does not flash, and after it the old frame is hidden and the activity
  // indicator shown together until the video is at the new time. The
  // subtitles hide from the seek on, so that they never run ahead of the
  // frame shown.
  const [seek, setSeek] = useState<"pending" | "shown">();
  const seekTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(seekTimer.current), []);
  const seekFrame = useRef<number>(undefined);
  const landTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(landTimer.current), []);
  const landed = () => {
    clearTimeout(seekTimer.current);
    clearTimeout(landTimer.current);
    setSeek(undefined);
  };
  // The seek ends with the first frame shown at the new time, where the
  // browser reports frames shown; else when the video has the time's data,
  // and a second after that where no frame is reported, as in a window
  // the browser does not draw frames of.
  const framed = (v: HTMLVideoElement) => "requestVideoFrameCallback" in v;
  const awaitFrame = (v: HTMLVideoElement) => {
    if (seekFrame.current !== undefined) v.cancelVideoFrameCallback(seekFrame.current);
    const check = (_: number, frame: VideoFrameCallbackMetadata) => {
      seekFrame.current = undefined;
      if (!v.seeking && Math.abs(frame.mediaTime - v.currentTime) < 1) landed();
      else seekFrame.current = v.requestVideoFrameCallback(check);
    };
    seekFrame.current = v.requestVideoFrameCallback(check);
  };
  // The video has the new time's data: the seek ends, at once where no
  // frame is reported, else a second on unless a frame ended it.
  const landing = (v: HTMLVideoElement) => {
    if (v.seeking || v.readyState < HTMLMediaElement.HAVE_CURRENT_DATA) return;
    clearTimeout(landTimer.current);
    if (!framed(v)) landed();
    else landTimer.current = setTimeout(landed, 1000);
  };
  const [duration, setDuration] = useState(0);
  const [muted, setMuted] = useState(false);
  const [volume, setVolume] = useState(() => prefs().volume);
  // Seconds the subtitles show later, or earlier.
  const [subtitleOffset, setSubtitleOffset] = useState(0);
  const [sink, setSink] = useState(() => prefs().audioOutput ?? "");
  const [outputs, setOutputs] = useState<MediaDeviceInfo[]>([]);
  // The brief notice of the volume, the speed or a seek; its key starts it again.
  const [notice, setNotice] = useState<{ icon: IconName; text: string; level?: number; key: number }>();
  const [speed, setSpeed] = useState(1);
  const [full, setFull] = useState(false);
  const [controls, setControls] = useState(true);
  const [panel, setPanel] = useState<Tab>();
  const [menu, setMenu] = useState<RowMenu>();
  const pip = usePictureInPicture(video);
  const lyrics = useLyrics(server, itemId, item.data?.item?.kind === ItemKind.TRACK);
  const [showLyrics, setShowLyrics] = useState(true);

  // Leaves for where the player was opened from, else the item's page in
  // the player's place.
  const back = useCallback(() => {
    if (router.history.canGoBack()) router.history.back();
    else void router.navigate({ to: "/s/$serverId/item/$itemId", params: { serverId: server.id, itemId }, replace: true });
  }, [router, server.id, itemId]);

  // The SyncPlay group playing the item, which the controls ask instead of the video.
  const group = useGroupPlayer(server, itemId, video, back);
  const groupRef = useRef(group);
  useEffect(() => {
    groupRef.current = group;
  });

  const playNext = useCallback(() => {
    if (group) return group.next(next.data ? [next.data.id] : []);
    if (!next.data) return;
    void router.navigate({
      to: "/s/$serverId/play/$itemId",
      params: { serverId: server.id, itemId: next.data.id },
      replace: true,
    });
  }, [router, server.id, next.data, group]);

  // Starts playback, again whenever the user picks other streams.
  useEffect(() => {
    const v = video.current;
    if (!v) return;
    setLoading(true);
    let hls: Hls | undefined;
    let onMediaError: (() => void) | undefined;
    let cancelled = false;
    const state = () => ({ positionMs: v.currentTime * 1000, paused: v.paused });
    // Started a tick later, so that an effect undone at once, as React's
    // development mode does, starts no playback on the server.
    const timer = setTimeout(async () => {
      try {
        const p = await Playback.start(
          servers,
          server,
          itemId,
          {
            mediaSourceId: search.source,
            audioStreamIndex: selection.audio,
            subtitleStreamIndex: selection.subtitle,
            startMs: startAt.current,
            capabilities: capabilities(undefined, qualityBitrates[selection.quality]),
          },
          state,
        );
        if (cancelled) {
          void p.stop();
          return;
        }
        playback.current = p;
        setInfo(p);
        setError(undefined);
        const start = p.startMs / 1000;
        if (p.hls && !nativeHls()) {
          const { default: HlsJs } = await import("hls.js");
          if (cancelled) return;
          hls = new HlsJs({ startPosition: start });
          // A network error that hls.js gives up on, as when the computer's
          // network changes under it, is retried for about twenty seconds,
          // twice as long apart each time, before it is shown.
          const delays = [1000, 2000, 4000, 8000, 8000];
          let retries = 0;
          // A media error, as when the browser's hardware decoder fails on a
          // frame, rebuilds the media pipeline where the video is and plays
          // on; three times within ten seconds of the video are shown.
          let recoveries = 0;
          let recoveredAt = -Infinity;
          const recover = () => {
            if (v.currentTime - recoveredAt > 10) recoveries = 0;
            if (recoveries >= 3) return false;
            recoveries++;
            recoveredAt = v.currentTime;
            hls?.recoverMediaError();
            v.play().catch(() => setPaused(true));
            return true;
          };
          onMediaError = () => {
            if (cancelled || !v.error || v.error.code === MediaError.MEDIA_ERR_ABORTED) return;
            if (!recover()) setError(v.error.message || `media error ${v.error.code}`);
          };
          v.addEventListener("error", onMediaError);
          hls.on(HlsJs.Events.ERROR, (_, data) => {
            if (!data.fatal) return;
            if (data.type === HlsJs.ErrorTypes.NETWORK_ERROR && retries < delays.length) {
              setTimeout(() => !cancelled && hls?.startLoad(), delays[retries++]);
              return;
            }
            if (data.type === HlsJs.ErrorTypes.MEDIA_ERROR && recover()) return;
            setError(`${data.type}: ${data.details}`);
          });
          hls.on(HlsJs.Events.FRAG_LOADED, () => {
            retries = 0;
          });
          hls.on(HlsJs.Events.SUBTITLE_TRACKS_UPDATED, () => {
            if (!hls) return;
            hls.subtitleDisplay = p.info.subtitleStreamIndex >= 0;
            hls.subtitleTrack = p.info.subtitleStreamIndex >= 0 ? 0 : -1;
          });
          hls.loadSource(p.url);
          hls.attachMedia(v);
        } else {
          v.src = p.url;
          v.addEventListener("loadedmetadata", () => start > 0 && (v.currentTime = start), { once: true });
        }
        // A group's member waits for the group to play.
        if (groupRef.current) groupRef.current.restarted();
        else v.play().catch(() => setPaused(true));
      } catch (err) {
        if (!cancelled) setError(errorMessage(err));
      }
    });
    return () => {
      cancelled = true;
      clearTimeout(timer);
      startAt.current = v.currentTime > 0 ? Math.round(v.currentTime * 1000) : startAt.current;
      // Stopped before the video is torn down, which takes its position.
      const p = playback.current;
      playback.current = undefined;
      if (p) void p.stop().then(() => qc.invalidateQueries({ queryKey: ["mavio", server.id] }));
      if (onMediaError) v.removeEventListener("error", onMediaError);
      hls?.destroy();
      v.pause();
      v.removeAttribute("src");
      v.load();
    };
  }, [servers, server, itemId, search.source, selection, qc]);

  // The page going away ends the playback with a request that outlives it.
  useEffect(() => {
    const onHide = () => playback.current?.stopOnUnload();
    addEventListener("pagehide", onHide);
    return () => removeEventListener("pagehide", onHide);
  }, []);

  useEffect(() => {
    const onChange = () => setFull(!!document.fullscreenElement);
    document.addEventListener("fullscreenchange", onChange);
    return () => document.removeEventListener("fullscreenchange", onChange);
  }, []);

  const tell = useCallback((icon: IconName, text: string, level?: number) => setNotice({ icon, text, level, key: performance.now() }), []);
  const tellVolume = useCallback(() => {
    const v = video.current;
    if (!v) return;
    const level = v.muted ? 0 : v.volume;
    tell(level === 0 ? "mute" : "volume", `${Math.round(level * 100)}%`, level);
  }, [tell]);
  const changeVolume = useCallback(
    (by: number) => {
      const v = video.current;
      if (!v) return;
      v.volume = Math.min(Math.max((v.muted ? 0 : v.volume) + by, 0), 1);
      v.muted = false;
      tellVolume();
    },
    [tellVolume],
  );

  // The volume of the last video, and the sound's device as chosen.
  useEffect(() => {
    const v = video.current;
    if (!v) return;
    v.volume = prefs().volume;
    const id = prefs().audioOutput;
    if (sinkable && id) v.setSinkId(id).catch(() => (setPrefs({ audioOutput: undefined }), setSink("")));
  }, []);

  // The devices the sound can play to, as the audio's panel opens; the
  // browser names them only once the page may use a microphone.
  useEffect(() => {
    if (!sinkable || (panel !== "audio" && menu !== "audio")) return;
    void navigator.mediaDevices
      ?.enumerateDevices()
      .then((ds) => setOutputs(ds.filter((d) => d.kind === "audiooutput" && d.deviceId !== "default" && d.label)))
      .catch(() => setOutputs([]));
  }, [panel, menu]);
  const pickOutput = (id: string) => {
    void video.current
      ?.setSinkId(id)
      .then(() => {
        setSink(id);
        setPrefs({ audioOutput: id || undefined });
      })
      .catch(() => undefined);
  };

  // The subtitles' cues shifted by the offset from the times they came
  // with, which cues arriving later take as they come.
  const cueTimes = useRef(new WeakMap<TextTrackCue, [number, number]>());
  const shifted = useRef(false);
  useEffect(() => {
    const v = video.current;
    if (!v) return;
    const apply = () => {
      if (subtitleOffset === 0 && !shifted.current) return;
      for (const track of v.textTracks) {
        for (const cue of track.cues ?? []) {
          let times = cueTimes.current.get(cue);
          if (!times) {
            times = [cue.startTime, cue.endTime];
            cueTimes.current.set(cue, times);
          }
          if (cue.startTime !== times[0] + subtitleOffset) {
            cue.startTime = Math.max(times[0] + subtitleOffset, 0);
            cue.endTime = Math.max(times[1] + subtitleOffset, 0);
          }
        }
      }
      shifted.current = subtitleOffset !== 0;
    };
    apply();
    v.addEventListener("timeupdate", apply);
    return () => v.removeEventListener("timeupdate", apply);
  }, [subtitleOffset]);
  const offsetSubtitles = (s: number) => {
    const o = Math.round(s * 10) / 10;
    setSubtitleOffset(o);
    tell("subtitles", `${t("player.subtitleOffset")} ${offsetText(o)}`);
  };

  // The controls hide after 3 seconds without input while playing.
  const hideTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const poke = useCallback(() => {
    setControls(true);
    clearTimeout(hideTimer.current);
    hideTimer.current = setTimeout(() => {
      if (!video.current?.paused) setControls(false);
    }, 3000);
  }, []);
  const hide = useCallback(() => {
    clearTimeout(hideTimer.current);
    setControls(false);
  }, []);
  useEffect(() => () => clearTimeout(hideTimer.current), []);
  // An open panel or menu keeps the controls up.
  const shown = controls || panel !== undefined || menu !== undefined;
  const tv = usePrefs().tv;

  // On the television (docs/tv.md §7), focus leaves the controls' row as
  // they hide, so that the arrows seek again.
  const row = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (tv && !shown && shell.current?.contains(document.activeElement)) (document.activeElement as HTMLElement).blur();
  }, [tv, shown]);

  // Focus enters the panel as it opens, on its current choice, and returns
  // to the button that opened it as it closes.
  const opener = useRef<HTMLElement>(null);
  const panelRef = useRef<HTMLElement>(null);
  const open = panel !== undefined;
  useEffect(() => {
    if (open) {
      const p = panelRef.current;
      if (p && !p.contains(document.activeElement)) (p.querySelector<HTMLElement>("[aria-checked=true]") ?? p.querySelector<HTMLElement>("button"))?.focus();
    } else if (opener.current?.isConnected && (document.activeElement === document.body || !document.activeElement)) {
      opener.current.focus();
      opener.current = null;
    }
  }, [open]);

  // Subtitles rise above the controls while they show.
  useEffect(() => {
    const v = video.current;
    if (!v) return;
    const tracks = v.textTracks;
    const lift = () => {
      for (const track of tracks) {
        for (const cue of track.activeCues ?? []) if (cue instanceof VTTCue) cue.line = shown ? -4 : "auto";
      }
    };
    const watch = (e: TrackEvent) => e.track?.addEventListener("cuechange", lift);
    lift();
    for (const track of tracks) track.addEventListener("cuechange", lift);
    tracks.addEventListener("addtrack", watch);
    return () => {
      for (const track of tracks) track.removeEventListener("cuechange", lift);
      tracks.removeEventListener("addtrack", watch);
    };
  }, [shown]);

  const play = useCallback(() => (group ? group.unpause() : void video.current?.play()), [group]);
  const pause = useCallback(() => (group ? group.pause() : video.current?.pause()), [group]);
  const toggle = useCallback(() => (video.current?.paused ? play() : pause()), [play, pause]);
  const seekTo = useCallback(
    (s: number) => {
      const v = video.current;
      if (!v) return;
      const to = Math.max(0, Math.min(v.duration || Infinity, s));
      if (group) group.seek(to * 1000);
      else v.currentTime = to;
    },
    [group],
  );
  const seekBy = useCallback(
    (s: number) => {
      const v = video.current;
      if (!v) return;
      const to = Math.max(0, Math.min(v.duration || Infinity, v.currentTime + s));
      seekTo(to);
      tell(s < 0 ? "seekBack" : "seekForward", `${clock(to)} / ${clock(v.duration)}`, v.duration ? to / v.duration : undefined);
    },
    [seekTo, tell],
  );
  const fullScreen = useCallback(() => {
    if (document.fullscreenElement) void document.exitFullscreen();
    else void shell.current?.requestFullscreen();
  }, []);

  // Dragging the seek bar pauses, and releasing it plays on.
  const resumeAfterScrub = useRef(false);
  const scrubStart = () => {
    const v = video.current;
    resumeAfterScrub.current = !group && !!v && !v.paused;
    if (resumeAfterScrub.current) v?.pause();
  };
  const scrubEnd = (s: number) => {
    seekTo(s);
    if (resumeAfterScrub.current) play();
    resumeAfterScrub.current = false;
  };

  // The wheel and trackpad: sideways seeks and up and down sets the
  // volume, which of them taken at a gesture's start, and over the seek bar
  // always seeking. A seek pauses until the gesture ends, then goes to
  // where it points and plays on.
  const scroll = useRef<{ axis?: "seek" | "volume"; last: number; target?: number; resume: boolean; timer?: ReturnType<typeof setTimeout> }>({ last: 0, resume: false });
  const wheelDeps = useRef({ seekTo, play, changeVolume, tell, group });
  wheelDeps.current = { seekTo, play, changeVolume, tell, group };
  useEffect(() => {
    const el = shell.current;
    if (!el || tv) return;
    const onWheel = (e: WheelEvent) => {
      const v = video.current;
      if (!v || (e.target instanceof Element && e.target.closest("aside"))) return;
      e.preventDefault();
      const d = wheelDeps.current;
      const w = scroll.current;
      // Pixels, from lines or pages where the wheel gives those.
      const unit = e.deltaMode === 1 ? 40 : e.deltaMode === 2 ? 800 : 1;
      const right = e.deltaX * unit;
      const up = -e.deltaY * unit;
      if (right === 0 && up === 0) return;
      const overBar = e.target instanceof Element && !!e.target.closest("[data-seekbar]");
      const now = performance.now();
      // A gesture is the scrolls less than a fifth of a second apart.
      if (!w.axis || now - w.last > 200) w.axis = overBar || Math.abs(right) > Math.abs(up) ? "seek" : "volume";
      w.last = now;
      if (w.axis === "volume") {
        d.changeVolume(up * 0.001);
        return;
      }
      const delta = overBar && Math.abs(up) > Math.abs(right) ? up : right;
      if (w.target === undefined) {
        w.resume = !d.group && !v.paused;
        if (w.resume) v.pause();
      }
      const total = v.duration || Infinity;
      const target = Math.min(Math.max((w.target ?? v.currentTime) + delta * 0.1, 0), total);
      w.target = target;
      d.tell(delta >= 0 ? "seekForward" : "seekBack", `${clock(target)} / ${clock(v.duration)}`, Number.isFinite(total) ? target / total : undefined);
      clearTimeout(w.timer);
      w.timer = setTimeout(() => {
        const to = w.target;
        w.target = undefined;
        w.axis = undefined;
        if (to !== undefined) wheelDeps.current.seekTo(to);
        if (w.resume) wheelDeps.current.play();
        w.resume = false;
      }, 300);
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => {
      el.removeEventListener("wheel", onWheel);
      clearTimeout(scroll.current.timer);
    };
  }, [tv]);

  // A click plays or pauses once it is not a double click, which goes to
  // full screen alone.
  const clickTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(clickTimer.current), []);

  // The assistant's controls, which read the latest of what they call.
  const live = useRef({ play, pause, back, seekTo, fullScreen, playNext, hasNext: false, info, selection, group });
  live.current = { play, pause, back, seekTo, fullScreen, playNext, hasNext: !!next.data || !!group, info, selection, group };
  useEffect(
    () =>
      registerPlayer({
        state: () => {
          const v = video.current;
          const l = live.current;
          return {
            serverId: server.id,
            itemId,
            position: v?.currentTime ?? 0,
            duration: v?.duration || 0,
            paused: v?.paused ?? true,
            speed: v?.playbackRate ?? 1,
            volume: Math.round((v?.volume ?? 1) * 100),
            muted: v?.muted ?? false,
            audioStreamIndex: l.info?.info.audioStreamIndex ?? -1,
            subtitleStreamIndex: l.info?.info.subtitleStreamIndex ?? -1,
          };
        },
        pause: () => live.current.pause(),
        resume: () => live.current.play(),
        stop: () => live.current.back(),
        seek: (s) => live.current.seekTo(s),
        speed: (r) => video.current && !live.current.group && (video.current.playbackRate = r),
        volume: (level) => {
          if (!video.current) return;
          video.current.volume = Math.min(Math.max(level, 0), 100) / 100;
          video.current.muted = false;
        },
        mute: (m) => video.current && (video.current.muted = m),
        fullScreen: () => live.current.fullScreen(),
        next: () => {
          if (!live.current.hasNext) return false;
          live.current.playNext();
          return true;
        },
        previous: () => live.current.seekTo(0),
        streams: (audio, subtitle) =>
          setSelection((s) => ({ ...s, ...(audio !== undefined ? { audio } : {}), ...(subtitle !== undefined ? { subtitle } : {}) })),
      }),
    [server.id, itemId],
  );

  // What other devices of the user ask of the playback.
  useServerEvents((s, e) => {
    const v = video.current;
    if (s.id !== server.id || e.kind.case !== "command" || !v) return;
    const c = e.kind.value.command?.kind;
    if (c?.case === "playState") {
      const commands: Partial<Record<PlayStateCommand, () => void>> = {
        [PlayStateCommand.PAUSE]: pause,
        [PlayStateCommand.UNPAUSE]: play,
        [PlayStateCommand.STOP]: back,
        [PlayStateCommand.NEXT]: playNext,
        [PlayStateCommand.PREVIOUS]: () => (v.currentTime = 0),
      };
      commands[c.value.command]?.();
    } else if (c?.case === "seek" && c.value.position) {
      seekTo(durationMs(c.value.position) / 1000);
    } else if (c?.case === "volume") {
      if (c.value.level !== undefined) v.volume = c.value.level / 100;
      if (c.value.muted !== undefined) v.muted = c.value.muted;
    } else return;
    poke();
  });

  // Keyboard shortcuts: space and k, arrows and j/l, f, m; Escape closes
  // the panel, then leaves. On the television (docs/tv.md §7) Backspace is
  // Back too, which hides the controls before it leaves; down shows the
  // controls, and again moves focus into their row, which up leaves; left
  // and right seek, and select pauses, keeping the controls up, or plays,
  // hiding them.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target;
      if (target instanceof HTMLInputElement && target.type !== "range") return;
      const v = video.current;
      if (!v || e.metaKey || e.ctrlKey || e.altKey) return;
      if (e.key === "Escape" || (tv && e.key === "Backspace")) {
        e.preventDefault();
        if (panel !== undefined) {
          setPanel(undefined);
          poke();
        } else if (tv && shown) {
          (document.activeElement as HTMLElement | null)?.blur();
          hide();
        } else if (tv || !document.fullscreenElement) back();
        return;
      }
      if (tv) {
        // The panel's choices take the keys while it is open.
        if (panel !== undefined) {
          poke();
          return;
        }
        if (row.current?.contains(document.activeElement)) {
          if (e.key === "ArrowUp") {
            e.preventDefault();
            (document.activeElement as HTMLElement).blur();
          }
          poke();
          return;
        }
        if (e.key === "ArrowDown") {
          e.preventDefault();
          if (shown) row.current?.querySelector<HTMLElement>("button")?.focus();
          poke();
        } else if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
          e.preventDefault();
          seekBy(e.key === "ArrowLeft" ? -10 : 10);
          poke();
        } else if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          if (v.paused) {
            play();
            hide();
          } else {
            pause();
            poke();
          }
        }
        return;
      }
      const focused = shell.current?.contains(document.activeElement) ?? false;
      // A slider's own arrows.
      if (e.defaultPrevented || (target instanceof HTMLInputElement && e.key.startsWith("Arrow"))) return;
      if (panel !== undefined || menu !== undefined) return;
      // A focused button presses with space and Return.
      if (focused && target instanceof HTMLButtonElement && (e.key === " " || e.key === "Enter")) {
        poke();
        return;
      }
      const keys: Record<string, () => void> = {
        " ": toggle,
        k: toggle,
        ArrowLeft: () => seekBy(-10),
        j: () => seekBy(-10),
        ArrowRight: () => seekBy(10),
        l: () => seekBy(10),
        ArrowUp: () => changeVolume(0.05),
        ArrowDown: () => changeVolume(-0.05),
        f: fullScreen,
        m: () => ((v.muted = !v.muted), tellVolume()),
      };
      const f = keys[e.key];
      if (!f) return;
      e.preventDefault();
      f();
      poke();
    };
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  }, [toggle, seekBy, fullScreen, back, poke, hide, play, pause, panel, menu, tv, shown, changeVolume, tellVolume]);

  const current = item.data?.item;
  const source = item.data?.mediaSources.find((m) => m.id === info?.info.mediaSourceId);
  const audioTracks = source?.streams.filter((s) => s.kind === StreamKind.AUDIO) ?? [];
  const subtitleTracks = info?.info.subtitles ?? [];
  const external = subtitleTracks.filter((s) => s.method === SubtitleMethod.EXTERNAL && s.url);
  const total = duration || (current?.runtime ? Number(current.runtime.seconds) : 0);
  const episode =
    current?.kind === ItemKind.EPISODE && current.indexNumber > 0
      ? t("item.episodeNumber", { season: current.parentIndexNumber, episode: current.indexNumber })
      : undefined;
  const subtitlesOn = (info?.info.subtitleStreamIndex ?? -1) >= 0;
  const chapters: ChapterMark[] = (source?.chapters ?? []).map((c) => ({ start: c.start ? durationMs(c.start) / 1000 : 0, title: c.title }));
  const pick = (s: Partial<Selection>) => setSelection({ ...selection, ...s });
  const openPanel = (tab: Tab) => {
    if (document.activeElement instanceof HTMLElement) opener.current = document.activeElement;
    setPanel(panel === tab ? undefined : tab);
  };
  // A menu of the desktop's row, above its button, opening in the player
  // so that it shows in full screen.
  const rowMenu = (which: RowMenu, label: string, icon: ReactNode, items: ReactNode) => (
    <Menu
      side="top"
      align="center"
      width="w-72"
      label={label}
      container={shell}
      glass
      gutter
      onOpenChange={(o) => {
        setMenu((m) => (o ? which : m === which ? undefined : m));
        if (!o) poke();
      }}
      trigger={
        <Tip label={label}>
          <button type="button" aria-label={label} className={`${playerButton} ${menu === which ? "bg-white/20" : ""}`}>
            {icon}
          </button>
        </Tip>
      }
    >
      {items}
    </Menu>
  );

  // Dark in either theme, as the Mac app's player, its menus with it.
  return (
    <div
      ref={shell}
      data-theme="dark"
      data-tv-player={tv || undefined}
      className={`fixed inset-0 bg-black text-white select-none ${shown ? "" : "cursor-none"}`}
      onMouseMove={poke}
      onPointerDown={poke}
    >
      <video
        ref={video}
        className={`h-full w-full ${seek === "shown" ? "opacity-0" : ""}`}
        data-hide-cues={seek !== undefined || undefined}
        playsInline
        crossOrigin="anonymous"
        onClick={(e) => {
          clearTimeout(clickTimer.current);
          if (panel !== undefined) setPanel(undefined);
          else if (e.detail <= 1) clickTimer.current = setTimeout(toggle, doubleClickMs);
        }}
        onDoubleClick={() => {
          clearTimeout(clickTimer.current);
          fullScreen();
        }}
        // On the television playing hides the controls.
        onPlay={() => (setPaused(false), tv ? hide() : poke())}
        onPause={() => {
          setPaused(true);
          setControls(true);
          void playback.current?.report();
        }}
        onSeeking={(e) => {
          // The seek bar shows where the video goes at once.
          setTime(e.currentTarget.currentTime);
          // A seek while the old frame is hidden keeps it hidden.
          if (seek !== "shown") {
            clearTimeout(seekTimer.current);
            setSeek("pending");
            seekTimer.current = setTimeout(() => setSeek("shown"), 200);
          }
          clearTimeout(landTimer.current);
          if (framed(e.currentTarget)) awaitFrame(e.currentTarget);
        }}
        onSeeked={(e) => {
          landing(e.currentTarget);
          void playback.current?.report();
        }}
        onEmptied={landed}
        onWaiting={() => setLoading(true)}
        onPlaying={(e) => {
          setLoading(false);
          landing(e.currentTarget);
        }}
        // Paused with enough to play, as a group's member waiting for the group.
        onCanPlay={(e) => {
          if (e.currentTarget.paused) setLoading(false);
          landing(e.currentTarget);
        }}
        onTimeUpdate={(e) => setTime(e.currentTarget.currentTime)}
        onDurationChange={(e) => Number.isFinite(e.currentTarget.duration) && setDuration(e.currentTarget.duration)}
        onVolumeChange={(e) => {
          const v = e.currentTarget;
          setMuted(v.muted);
          setVolume(v.volume);
          if (v.volume !== prefs().volume) setPrefs({ volume: v.volume });
        }}
        onRateChange={(e) => !group && setSpeed(e.currentTarget.playbackRate)}
        onEnded={() => (group ? group.ended() : prefs().autoplayNext && next.data ? playNext() : back())}
      >
        {external.map((s) => (
          <track
            key={s.streamIndex}
            kind="subtitles"
            src={urlOf(server, s.url)}
            label={s.title || s.language || `#${s.streamIndex}`}
            srcLang={s.language || undefined}
            default={s.streamIndex === info?.info.subtitleStreamIndex}
          />
        ))}
      </video>

      {pip.host && (
        <>
          <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-3 text-white/60">
            <Icon name="pictureInPicture" size={48} />
            <span className="type-callout">{t("player.pictureInPicture")}</span>
          </div>
          <PictureInPictureControls
            host={pip.host}
            paused={paused}
            time={time}
            total={total}
            onToggle={toggle}
            onClose={() => {
              // Closed once the pause is told: the video's events still
              // queued as it moves back are lost, and hls.js would play
              // it again.
              const v = video.current;
              if (!v || v.paused) return pip.close();
              v.addEventListener("pause", () => setTimeout(pip.close), { once: true });
              pause();
            }}
            onRestore={pip.close}
            onScrubStart={scrubStart}
            onScrubEnd={scrubEnd}
          />
        </>
      )}

      {lyrics.data && showLyrics && (
        <LyricsView lyrics={lyrics.data} time={time} onSeek={seekTo} onBackground={() => (panel !== undefined ? setPanel(undefined) : toggle())} />
      )}

      {notice && (
        <div
          key={notice.key}
          role="status"
          className="pointer-events-none absolute top-20 left-5 flex animate-[notice_1.5s_ease-out_forwards] items-center gap-3 rounded-[10px] bg-black/60 py-2 pr-4 pl-3.5"
        >
          <Icon name={notice.icon} fill size={18} />
          <span className="type-headline font-semibold tabular-nums">{notice.text}</span>
          {notice.level !== undefined && (
            <span className="h-1.5 w-[100px] overflow-hidden rounded-full bg-white/30">
              <span className="block h-full bg-white" style={{ width: `${Math.min(Math.max(notice.level, 0), 1) * 100}%` }} />
            </span>
          )}
        </div>
      )}

      {((loading && seek !== "pending") || seek === "shown") && !error && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <Spinner size={48} className="text-white/70!" />
        </div>
      )}

      {error && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/70">
          <div className="max-w-md p-6 text-center">
            <Icon name="error" size={48} className="mx-auto mb-3 opacity-70" />
            <p className="mb-4 type-body">{error}</p>
            <button type="button" onClick={back} className="rounded-[10px] bg-white/15 px-4 py-2 type-headline hover:bg-white/25">
              {t("player.back")}
            </button>
          </div>
        </div>
      )}

      <div className={`pointer-events-none absolute inset-0 transition-opacity duration-300 ${shown ? "opacity-100" : "opacity-0"}`}>
        <div className="pointer-events-auto absolute inset-x-0 top-0 flex items-center gap-3 bg-gradient-to-b from-black/75 to-transparent px-5 pt-4 pb-12">
          {!tv && (
            <PlayerButton label={t("player.back")} onClick={back}>
              <Icon name="back" />
            </PlayerButton>
          )}
          <div className="min-w-0">
            {current?.seriesName && (
              <div className="truncate type-footnote text-white/70">
                {[current.seriesName, episode].filter(Boolean).join(" · ")}
              </div>
            )}
            <div className="truncate type-title3 font-semibold">{current?.name ?? ""}</div>
          </div>
          {group && !tv && <GroupChip server={server} group={group.group} />}
        </div>

        <div className="pointer-events-auto absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/85 via-black/40 to-transparent px-5 pt-16 pb-4">
          {tv ? (
            <Timeline time={time} total={total} />
          ) : (
            // The time played and the time left on either side of the bar, as the Mac app's.
            <div className="flex items-center gap-3 type-footnote text-white/85 tabular-nums">
              <span>{clock(time)}</span>
              <div className="min-w-0 flex-1">
                <SeekBar
                  time={time}
                  total={total}
                  chapters={chapters}
                  trickplay={item.data?.trickplay ?? []}
                  sheetUrl={(path) => urlOf(server, path)}
                  onScrubStart={scrubStart}
                  onScrubEnd={scrubEnd}
                />
              </div>
              <span>-{clock(Math.max(total - time, 0))}</span>
            </div>
          )}
          {tv ? (
            // The television's row: the remote's keys play, pause and seek.
            <div className="mt-3 flex items-center gap-3">
              <span className="type-callout text-white/85 tabular-nums">
                {clock(time)} / {clock(total)}
              </span>
              <div ref={row} data-tv-row className="ml-auto flex min-w-0 items-center gap-2">
                {group && <GroupChip server={server} group={group.group} tv />}
                {(next.data || (group && group.group.currentIndex < group.group.queue.length - 1)) && (
                  <button
                    type="button"
                    onClick={playNext}
                    className="flex h-12 items-center gap-2 rounded-full bg-white/15 px-5 type-callout font-semibold backdrop-blur-xl"
                  >
                    <Icon name="next" fill size={20} />
                    {t("player.nextEpisode")}
                  </button>
                )}
                {subtitleTracks.length > 0 && (
                  <PlayerButton label={t("player.subtitles")} onClick={() => openPanel("subtitles")} active={panel === "subtitles"}>
                    <Icon name="subtitles" fill={subtitlesOn} />
                  </PlayerButton>
                )}
                <PlayerButton label={t("player.audio")} onClick={() => openPanel("audio")} active={panel !== undefined && panel !== "subtitles"}>
                  <Icon name="audio" />
                </PlayerButton>
              </div>
            </div>
          ) : (
          // The desktop's row, as the Mac app's: playing and the sound on
          // the leading side, the menus and the windows on the trailing one.
          <div className="mt-2 flex items-center gap-1">
            <PlayerButton label={paused ? t("player.play") : t("player.pause")} onClick={toggle} large>
              <Icon name={paused ? "play" : "pause"} fill size={30} />
            </PlayerButton>
            <PlayerButton
              label={muted || volume === 0 ? t("player.unmute") : t("player.mute")}
              onClick={() => {
                const v = video.current;
                if (!v) return;
                // Unmuting a silent video brings its sound back.
                if (v.muted || v.volume === 0) {
                  v.muted = false;
                  if (v.volume === 0) v.volume = 0.5;
                } else v.muted = true;
                tellVolume();
              }}
            >
              <Icon name={muted || volume === 0 ? "mute" : "volume"} />
            </PlayerButton>
            <Slider
              light
              min={0}
              max={1}
              step={0.01}
              value={muted ? 0 : volume}
              label={t("player.volume")}
              valueText={(x) => `${Math.round(x * 100)}%`}
              onChange={(x) => {
                const v = video.current;
                if (!v) return;
                v.volume = x;
                v.muted = false;
                tellVolume();
              }}
              className="w-20"
            />
            {(next.data || (group && group.group.currentIndex < group.group.queue.length - 1)) && (
              <PlayerButton label={t("player.nextEpisode")} onClick={playNext}>
                <Icon name="next" fill />
              </PlayerButton>
            )}
            <div className="ml-auto flex items-center gap-1">
              {lyrics.data && (
                <PlayerButton label={t("player.lyrics")} onClick={() => setShowLyrics(!showLyrics)} active={showLyrics}>
                  <Icon name="music" fill={showLyrics} />
                </PlayerButton>
              )}
              <AssistantButton className="flex size-11 items-center justify-center rounded-full hover:bg-white/15" />
              {rowMenu("speed", t("player.speed"), <Icon name="speed" />, (
                <>
                  <MenuLabel>{t("player.speed")}</MenuLabel>
                  <MenuChoice
                    value={String(speed)}
                    onChange={(r) => {
                      if (!video.current || group) return;
                      video.current.playbackRate = Number(r);
                      tell("speed", `${r}×`);
                    }}
                  >
                    {speeds.map((r) => (
                      <MenuOption key={r} value={String(r)}>
                        {r}×
                      </MenuOption>
                    ))}
                  </MenuChoice>
                </>
              ))}
              {subtitleTracks.length > 0 &&
                rowMenu("subtitles", t("player.subtitles"), <Icon name="subtitles" fill={subtitlesOn} />, (
                  <>
                    <MenuLabel>{t("player.subtitles")}</MenuLabel>
                    <MenuChoice value={String(info?.info.subtitleStreamIndex ?? -1)} onChange={(i) => pick({ subtitle: Number(i) })}>
                      <MenuOption value="-1">{t("player.subtitlesOff")}</MenuOption>
                      {subtitleTracks.map((s) => {
                        const stream = source?.streams.find((x) => x.index === s.streamIndex);
                        return (
                          <MenuOption key={s.streamIndex} value={String(s.streamIndex)}>
                            <TrackName
                              label={stream ? trackLabel(stream) : [s.title || languageOf(s.language) || `#${s.streamIndex}`, s.format.toUpperCase()]}
                            />
                          </MenuOption>
                        );
                      })}
                    </MenuChoice>
                    {subtitlesOn && (
                      <>
                        <MenuSeparator />
                        <MenuLabel>
                          {t("player.subtitleOffset")}: {offsetText(subtitleOffset)}
                        </MenuLabel>
                        <MenuItem closeOnClick={false} onClick={() => offsetSubtitles(subtitleOffset - 0.1)}>
                          {t("player.offsetEarlier")}
                        </MenuItem>
                        <MenuItem closeOnClick={false} onClick={() => offsetSubtitles(subtitleOffset + 0.1)}>
                          {t("player.offsetLater")}
                        </MenuItem>
                        <MenuItem closeOnClick={false} disabled={subtitleOffset === 0} onClick={() => offsetSubtitles(0)}>
                          {t("player.offsetReset")}
                        </MenuItem>
                      </>
                    )}
                  </>
                ))}
              {rowMenu("audio", t("player.audio"), <Icon name="audio" />, (
                <>
                  <MenuLabel>{t("player.audio")}</MenuLabel>
                  <MenuChoice value={String(info?.info.audioStreamIndex ?? -1)} onChange={(i) => pick({ audio: Number(i) })}>
                    {audioTracks.map((s) => (
                      <MenuOption key={s.index} value={String(s.index)}>
                        <TrackName label={trackLabel(s)} />
                      </MenuOption>
                    ))}
                  </MenuChoice>
                  {sinkable && outputs.length > 0 && (
                    <>
                      <MenuSeparator />
                      <MenuLabel>{t("player.audioDevice")}</MenuLabel>
                      <MenuChoice value={sink} onChange={pickOutput}>
                        <MenuOption value="">{t("player.audioDeviceSystem")}</MenuOption>
                        {outputs.map((d) => (
                          <MenuOption key={d.deviceId} value={d.deviceId}>
                            {d.label}
                          </MenuOption>
                        ))}
                      </MenuChoice>
                    </>
                  )}
                </>
              ))}
              {rowMenu("quality", t("player.quality"), <Icon name="quality" />, (
                <>
                  <MenuLabel>{t("player.quality")}</MenuLabel>
                  <MenuChoice value={selection.quality} onChange={(q) => pick({ quality: q })}>
                    {qualities.map((q) => (
                      <MenuOption key={q.value} value={q.value}>
                        {q.label()}
                      </MenuOption>
                    ))}
                  </MenuChoice>
                </>
              ))}
              {info &&
                rowMenu("info", t("player.info"), <Icon name="info" />, (
                  <>
                    <MenuLabel>{t("player.method")}</MenuLabel>
                    <MenuFact>{methodText(info.info.method)}</MenuFact>
                    {source && (
                      <>
                        <MenuLabel>{t("player.source")}</MenuLabel>
                        <MenuFact>{[source.name, source.container.toUpperCase()].filter(Boolean).join(" · ")}</MenuFact>
                      </>
                    )}
                    <MenuSeparator />
                    <MenuItem onClick={() => window.open(info.url, "_blank")}>
                      {t("player.openExternal")}
                    </MenuItem>
                    <MenuItem onClick={() => void navigator.clipboard?.writeText(info.url)}>
                      {t("player.copyLink")}
                    </MenuItem>
                  </>
                ))}
              {pip.supported && (
                <PlayerButton label={t("player.pictureInPicture")} onClick={() => (pip.active ? pip.close() : void pip.open())} active={pip.active}>
                  <Icon name="pictureInPicture" />
                </PlayerButton>
              )}
              <PlayerButton label={t("player.fullScreen")} onClick={fullScreen}>
                <Icon name={full ? "exitFullScreen" : "fullScreen"} />
              </PlayerButton>
            </div>
          </div>
          )}
        </div>
      </div>

      {panel && (
        <aside
          ref={panelRef}
          data-focus-region
          aria-label={t("player.options")}
          className="absolute right-4 bottom-24 flex max-h-[calc(100%-7rem)] w-[min(360px,calc(100vw-32px))] flex-col overflow-hidden rounded-[18px] border border-white/10 bg-neutral-900/75 shadow-level3 backdrop-blur-2xl"
        >
          <Tabs
            value={panel}
            label={t("player.options")}
            look="dark"
            className="m-3"
            tabs={[
              { value: "audio" as const, label: t("player.audio") },
              { value: "subtitles" as const, label: t("player.subtitles") },
              { value: "quality" as const, label: t("player.quality") },
              { value: "info" as const, label: t("player.info") },
            ]}
            onChange={setPanel}
          />
          <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-3">
            {panel === "audio" && (
              <>
                {audioTracks.map((s) => (
                  <Choice
                    key={s.index}
                    checked={s.index === info?.info.audioStreamIndex}
                    onClick={() => pick({ audio: s.index })}
                    label={trackLabel(s)}
                  />
                ))}
                <Heading>{t("player.speed")}</Heading>
                <div className="flex flex-wrap gap-1.5 px-2.5">
                  {speeds.map((r) => (
                    <button
                      key={r}
                      type="button"
                      aria-pressed={r === speed}
                      disabled={!!group}
                      onClick={() => {
                        if (!video.current) return;
                        video.current.playbackRate = r;
                        tell("speed", `${r}×`);
                      }}
                      className={`h-8 rounded-full px-3 type-footnote font-semibold tabular-nums disabled:opacity-40 ${r === speed ? "bg-white/30 text-white" : "bg-white/10 hover:bg-white/20"}`}
                    >
                      {r}×
                    </button>
                  ))}
                </div>
                {sinkable && outputs.length > 0 && (
                  <>
                    <Heading>{t("player.audioDevice")}</Heading>
                    <Choice checked={sink === ""} onClick={() => pickOutput("")} label={[t("player.audioDeviceSystem"), ""]} />
                    {outputs.map((d) => (
                      <Choice key={d.deviceId} checked={sink === d.deviceId} onClick={() => pickOutput(d.deviceId)} label={[d.label, ""]} />
                    ))}
                  </>
                )}
              </>
            )}
            {panel === "subtitles" && (
              <>
                <Choice checked={!subtitlesOn} onClick={() => pick({ subtitle: -1 })} label={[t("player.subtitlesOff"), ""]} />
                {subtitleTracks.map((s) => {
                  const stream = source?.streams.find((x) => x.index === s.streamIndex);
                  return (
                    <Choice
                      key={s.streamIndex}
                      checked={s.streamIndex === info?.info.subtitleStreamIndex}
                      onClick={() => pick({ subtitle: s.streamIndex })}
                      label={stream ? trackLabel(stream) : [s.title || languageOf(s.language) || `#${s.streamIndex}`, s.format.toUpperCase()]}
                    />
                  );
                })}
                {subtitlesOn && (
                  <>
                    <Heading>{t("player.subtitleOffset")}</Heading>
                    <div className="flex items-center gap-1.5 px-2.5">
                      <OffsetButton onClick={() => offsetSubtitles(subtitleOffset - 0.1)}>{t("player.offsetEarlier")}</OffsetButton>
                      <span className="min-w-16 text-center type-callout tabular-nums">{offsetText(subtitleOffset)}</span>
                      <OffsetButton onClick={() => offsetSubtitles(subtitleOffset + 0.1)}>{t("player.offsetLater")}</OffsetButton>
                      <OffsetButton disabled={subtitleOffset === 0} onClick={() => offsetSubtitles(0)}>
                        {t("player.offsetReset")}
                      </OffsetButton>
                    </div>
                  </>
                )}
              </>
            )}
            {panel === "quality" &&
              qualities.map((q) => (
                <Choice key={q.value} checked={q.value === selection.quality} onClick={() => pick({ quality: q.value })} label={[q.label(), ""]} />
              ))}
            {panel === "info" && info && (
              <div className="flex flex-col gap-4 px-2.5 pt-1">
                <Fact label={t("player.method")}>{methodText(info.info.method)}</Fact>
                {source && (
                  <Fact label={t("player.source")}>
                    {[source.name, source.container.toUpperCase()].filter(Boolean).join(" · ")}
                  </Fact>
                )}
                <div className="flex flex-col gap-1">
                  <PanelAction icon="external" onClick={() => window.open(info.url, "_blank")}>
                    {t("player.openExternal")}
                  </PanelAction>
                  <PanelAction onClick={() => void navigator.clipboard?.writeText(info.url)}>
                    {t("player.copyLink")}
                  </PanelAction>
                </div>
              </div>
            )}
          </div>
        </aside>
      )}
    </div>
  );
}

function methodText(m: PlayMethod): string {
  switch (m) {
    case PlayMethod.DIRECT_PLAY:
      return t("player.directPlay");
    case PlayMethod.DIRECT_STREAM:
      return t("player.directStream");
    default:
      return t("player.transcode");
  }
}

/** The television's timeline, which the remote's left and right seek along. */
function Timeline({ time, total }: { time: number; total: number }) {
  return (
    <div className="h-1.5 overflow-hidden rounded-full bg-white/25">
      <div className="h-full bg-white" style={{ width: `${total ? Math.min(100, (time / total) * 100) : 0}%` }} />
    </div>
  );
}

const playerButton = "flex h-11 w-11 items-center justify-center rounded-full hover:bg-white/15";

/** A track's name, and after it its codec and flags. */
function TrackName({ label: [name, details] }: { label: [string, string] }) {
  return (
    <span className="flex min-w-0 items-baseline gap-2">
      <span className="truncate">{name}</span>
      {details && <span className="truncate type-caption text-text-secondary">{details}</span>}
    </span>
  );
}

function PlayerButton({
  label,
  onClick,
  children,
  active,
  large,
}: {
  label: string;
  onClick: () => void;
  children: ReactNode;
  active?: boolean;
  large?: boolean;
}) {
  return (
    <Tip label={label}>
      <button
        type="button"
        aria-label={label}
        aria-pressed={active}
        onClick={onClick}
        className={`${large ? "flex h-12 w-12 items-center justify-center rounded-full hover:bg-white/15" : playerButton} ${active ? "bg-white/20" : ""}`}
      >
        {children}
      </button>
    </Tip>
  );
}

function OffsetButton({ onClick, disabled, children }: { onClick: () => void; disabled?: boolean; children: ReactNode }) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className="h-8 rounded-full bg-white/10 px-3 type-footnote font-semibold hover:bg-white/20 disabled:opacity-40"
    >
      {children}
    </button>
  );
}

function Heading({ children }: { children: ReactNode }) {
  return <h3 className="px-2.5 pt-4 pb-2 type-footnote font-semibold text-white/60">{children}</h3>;
}

function Choice({ checked, onClick, label: [name, details] }: { checked?: boolean; onClick: () => void; label: [string, string] }) {
  return (
    <button
      type="button"
      role="menuitemradio"
      aria-checked={checked}
      onClick={onClick}
      className="flex w-full items-center gap-3 rounded-[10px] px-2.5 py-2 text-left hover:bg-white/10"
    >
      <span className="w-4 shrink-0 text-accent">{checked && <Icon name="check" size={16} />}</span>
      <span className="min-w-0 flex-1">
        <span className="block truncate type-callout">{name}</span>
        {details && <span className="block truncate type-caption text-white/60">{details}</span>}
      </span>
    </button>
  );
}

function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <div className="type-footnote font-semibold text-white/60">{label}</div>
      <div className="mt-0.5 type-callout">{children}</div>
    </div>
  );
}

function PanelAction({ icon, onClick, children }: { icon?: IconName; onClick: () => void; children: ReactNode }) {
  return (
    <button type="button" onClick={onClick} className="-mx-2.5 flex items-center gap-3 rounded-[10px] px-2.5 py-2 text-left type-callout hover:bg-white/10">
      {icon ? <Icon name={icon} size={18} className="text-white/70" /> : <span className="w-[18px]" />}
      {children}
    </button>
  );
}

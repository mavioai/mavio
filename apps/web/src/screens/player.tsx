// The player (docs/clients.md §4.1, §6.5): the server decides how the
// browser plays an item; hls.js plays HLS where Media Source Extensions
// exist, Safari plays it natively. Progress is reported every 10 seconds
// and on pause, seek and stop. A side panel picks the audio, subtitles and
// quality and tells how the item plays; an episode's end starts the next.
// Other devices of the user control it remotely (§6.3.23), and it follows
// the SyncPlay group playing its item (§6.3.24). A track shows its lyrics
// (§6.3.27).
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

import { Slider, Tabs } from "../controls/inputs.tsx";
import { Tip } from "../controls/tooltip.tsx";
import { useServerEvents } from "../events.tsx";
import { locale, t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { prefs, type Quality, qualities, qualityBitrates } from "../prefs.ts";
import { useGroupPlayer } from "../syncplay.tsx";
import { StatusView } from "../ui.tsx";
import { LyricsView } from "./lyrics.tsx";
import { GroupChip } from "./syncplay.tsx";

const route = getRouteApi("/s/$serverId/play/$itemId");

const speeds = [0.5, 0.75, 1, 1.25, 1.5, 2];

/** Seconds as "1:02:03" or "2:03". */
function clock(s: number): string {
  if (!Number.isFinite(s) || s < 0) s = 0;
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = Math.floor(s % 60);
  const mm = h > 0 ? String(m).padStart(2, "0") : String(m);
  return `${h > 0 ? `${h}:` : ""}${mm}:${String(sec).padStart(2, "0")}`;
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
  const [selection, setSelection] = useState<Selection>(() => ({ quality: prefs().quality }));
  // Where the next start begins, in milliseconds: the resume position, then where a track change happened.
  const startAt = useRef(search.start ?? 0);
  const [paused, setPaused] = useState(true);
  const [time, setTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [muted, setMuted] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [full, setFull] = useState(false);
  const [controls, setControls] = useState(true);
  const [panel, setPanel] = useState<Tab>();
  const lyrics = useLyrics(server, itemId, item.data?.item?.kind === ItemKind.TRACK);
  const [showLyrics, setShowLyrics] = useState(true);

  const back = useCallback(() => {
    if (router.history.length > 1) router.history.back();
    else void router.navigate({ to: "/s/$serverId/item/$itemId", params: { serverId: server.id, itemId } });
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
    let hls: Hls | undefined;
    let cancelled = false;
    const state = () => ({ positionMs: v.currentTime * 1000, paused: v.paused });
    (async () => {
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
          hls.on(HlsJs.Events.ERROR, (_, data) => {
            if (data.fatal) setError(`${data.type}: ${data.details}`);
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
    })();
    return () => {
      cancelled = true;
      startAt.current = v.currentTime > 0 ? Math.round(v.currentTime * 1000) : startAt.current;
      // Stopped before the video is torn down, which takes its position.
      const p = playback.current;
      playback.current = undefined;
      if (p) void p.stop().then(() => qc.invalidateQueries({ queryKey: ["mavio", server.id] }));
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

  // The controls hide after 3 seconds without input while playing.
  const hideTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const poke = useCallback(() => {
    setControls(true);
    clearTimeout(hideTimer.current);
    hideTimer.current = setTimeout(() => {
      if (!video.current?.paused) setControls(false);
    }, 3000);
  }, []);
  useEffect(() => () => clearTimeout(hideTimer.current), []);
  // An open panel keeps the controls up.
  const shown = controls || panel !== undefined;

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
  const seekBy = useCallback((s: number) => seekTo((video.current?.currentTime ?? 0) + s), [seekTo]);
  const fullScreen = useCallback(() => {
    if (document.fullscreenElement) void document.exitFullscreen();
    else void shell.current?.requestFullscreen();
  }, []);

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

  // Keyboard shortcuts: space and k, arrows and j/l, f, m; Escape closes the panel, then leaves.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement && e.target.type !== "range") return;
      const v = video.current;
      if (!v) return;
      if (panel !== undefined && e.key !== "Escape") return;
      const keys: Record<string, () => void> = {
        " ": toggle,
        k: toggle,
        ArrowLeft: () => seekBy(-10),
        j: () => seekBy(-10),
        ArrowRight: () => seekBy(10),
        l: () => seekBy(10),
        f: fullScreen,
        m: () => (v.muted = !v.muted),
        Escape: () => (panel !== undefined ? setPanel(undefined) : document.fullscreenElement ? undefined : back()),
      };
      const f = keys[e.key];
      if (!f) return;
      e.preventDefault();
      f();
      poke();
    };
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  }, [toggle, seekBy, fullScreen, back, poke, panel]);

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
  const pick = (s: Partial<Selection>) => setSelection({ ...selection, ...s });
  const openPanel = (tab: Tab) => setPanel(panel === tab ? undefined : tab);

  return (
    <div
      ref={shell}
      className={`fixed inset-0 bg-black text-white select-none ${shown ? "" : "cursor-none"}`}
      onMouseMove={poke}
      onPointerDown={poke}
    >
      <video
        ref={video}
        className="h-full w-full"
        playsInline
        crossOrigin="anonymous"
        onClick={() => (panel !== undefined ? setPanel(undefined) : toggle())}
        onDoubleClick={fullScreen}
        onPlay={() => (setPaused(false), poke())}
        onPause={() => {
          setPaused(true);
          setControls(true);
          void playback.current?.report();
        }}
        onSeeked={() => void playback.current?.report()}
        onTimeUpdate={(e) => setTime(e.currentTarget.currentTime)}
        onDurationChange={(e) => Number.isFinite(e.currentTarget.duration) && setDuration(e.currentTarget.duration)}
        onVolumeChange={(e) => setMuted(e.currentTarget.muted)}
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

      {lyrics.data && showLyrics && (
        <LyricsView lyrics={lyrics.data} time={time} onSeek={seekTo} onBackground={() => (panel !== undefined ? setPanel(undefined) : toggle())} />
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
          <PlayerButton label={t("player.back")} onClick={back}>
            <Icon name="back" />
          </PlayerButton>
          <div className="min-w-0">
            {current?.seriesName && (
              <div className="truncate type-footnote text-white/70">
                {[current.seriesName, episode].filter(Boolean).join(" · ")}
              </div>
            )}
            <div className="truncate type-title3 font-semibold">{current?.name ?? ""}</div>
          </div>
          {group && <GroupChip server={server} group={group.group} />}
        </div>

        <div className="pointer-events-auto absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/85 via-black/40 to-transparent px-5 pt-16 pb-4">
          <Scrubber time={time} total={total} onSeek={seekTo} />
          <div className="mt-2 flex items-center gap-1">
            <PlayerButton label={paused ? t("player.play") : t("player.pause")} onClick={toggle} large>
              <Icon name={paused ? "play" : "pause"} fill size={30} />
            </PlayerButton>
            <PlayerButton label={t("player.seekBack", { count: 10 })} onClick={() => seekBy(-10)}>
              <Icon name="seekBack" />
            </PlayerButton>
            <PlayerButton label={t("player.seekForward", { count: 10 })} onClick={() => seekBy(10)}>
              <Icon name="seekForward" />
            </PlayerButton>
            <PlayerButton
              label={muted ? t("player.unmute") : t("player.mute")}
              onClick={() => video.current && (video.current.muted = !video.current.muted)}
            >
              <Icon name={muted ? "mute" : "volume"} />
            </PlayerButton>
            <span className="ml-2 type-footnote text-white/85 tabular-nums">
              {clock(time)} / {clock(total)}
            </span>
            <div className="ml-auto flex items-center gap-1">
              {(next.data || (group && group.group.currentIndex < group.group.queue.length - 1)) && (
                <button
                  type="button"
                  onClick={playNext}
                  className="mr-1 flex h-9 items-center gap-1.5 rounded-full bg-white/15 px-3.5 type-footnote font-semibold backdrop-blur-xl hover:bg-white/25"
                >
                  <Icon name="next" fill size={16} />
                  {t("player.nextEpisode")}
                </button>
              )}
              {lyrics.data && (
                <PlayerButton label={t("player.lyrics")} onClick={() => setShowLyrics(!showLyrics)} active={showLyrics}>
                  <Icon name="music" fill={showLyrics} />
                </PlayerButton>
              )}
              {subtitleTracks.length > 0 && (
                <PlayerButton label={t("player.subtitles")} onClick={() => openPanel("subtitles")} active={panel === "subtitles"}>
                  <Icon name="subtitles" fill={subtitlesOn} />
                </PlayerButton>
              )}
              <PlayerButton label={t("player.options")} onClick={() => openPanel("audio")} active={panel !== undefined && panel !== "subtitles"}>
                <Icon name="settings" />
              </PlayerButton>
              {document.pictureInPictureEnabled && (
                <PlayerButton
                  label={t("player.pictureInPicture")}
                  onClick={() =>
                    document.pictureInPictureElement
                      ? void document.exitPictureInPicture()
                      : void video.current?.requestPictureInPicture()
                  }
                >
                  <Icon name="pictureInPicture" />
                </PlayerButton>
              )}
              <PlayerButton label={t("player.fullScreen")} onClick={fullScreen}>
                <Icon name={full ? "exitFullScreen" : "fullScreen"} />
              </PlayerButton>
            </div>
          </div>
        </div>
      </div>

      {panel && (
        <aside
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
                      onClick={() => video.current && (video.current.playbackRate = r)}
                      className={`h-8 rounded-full px-3 type-footnote font-semibold tabular-nums disabled:opacity-40 ${r === speed ? "bg-white text-black" : "bg-white/10 hover:bg-white/20"}`}
                    >
                      {r}×
                    </button>
                  ))}
                </div>
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

/** The timeline: a slim bar that thickens under the pointer. */
function Scrubber({ time, total, onSeek }: { time: number; total: number; onSeek: (s: number) => void }) {
  return <Slider light min={0} max={total || 1} value={Math.min(time, total || 1)} label={t("player.position")} valueText={clock} onChange={onSeek} />;
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
        className={`flex items-center justify-center rounded-full hover:bg-white/15 ${large ? "h-12 w-12" : "h-11 w-11"} ${active ? "bg-white/20" : ""}`}
      >
        {children}
      </button>
    </Tip>
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

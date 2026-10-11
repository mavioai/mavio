// The player (docs/clients.md §4.1, §6.5): the server decides how the
// browser plays an item; hls.js plays HLS where Media Source Extensions
// exist, Safari plays it natively. Progress is reported every 10 seconds
// and on pause, seek and stop.
import {
  errorMessage,
  nativeHls,
  Playback,
  type ServerEntry,
  urlOf,
  useItem,
  useServer,
  useServersStore,
} from "@mavio/client-ts";
import { type MediaStream, StreamKind } from "@mavio/proto/mavio/library/v1/media_pb";
import { SubtitleMethod } from "@mavio/proto/mavio/playback/v1/playback_pb";
import { useQueryClient } from "@tanstack/react-query";
import { getRouteApi, useRouter } from "@tanstack/react-router";
import type Hls from "hls.js";
import { type ReactNode, useCallback, useEffect, useRef, useState } from "react";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { StatusView } from "../ui.tsx";

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

function trackLabel(s: MediaStream): string {
  const lang = s.language ? (new Intl.DisplayNames([navigator.language], { type: "language" }).of(s.language) ?? s.language) : "";
  const details = [s.codec.toUpperCase(), s.channels ? `${s.channels} ch` : "", s.forced ? "Forced" : "", s.hearingImpaired ? "SDH" : ""]
    .filter(Boolean)
    .join(" · ");
  return [s.title || lang || `#${s.index}`, details].filter(Boolean).join(" — ");
}

export function PlayerScreen() {
  const { serverId, itemId } = route.useParams();
  const server = useServer(serverId);
  if (!server?.token) return <StatusView icon="error" title={t("auth.signedOut")} />;
  return <Player key={itemId} server={server} itemId={itemId} />;
}

/** The streams the user picked, which restart playback when they change. */
interface Selection {
  audio?: number;
  subtitle?: number;
}

function Player({ server, itemId }: { server: ServerEntry; itemId: string }) {
  const servers = useServersStore();
  const router = useRouter();
  const qc = useQueryClient();
  const search = route.useSearch();
  const item = useItem(server, itemId);
  const video = useRef<HTMLVideoElement>(null);
  const shell = useRef<HTMLDivElement>(null);
  const playback = useRef<Playback>(undefined);
  const [info, setInfo] = useState<Playback>();
  const [error, setError] = useState<string>();
  const [selection, setSelection] = useState<Selection>({});
  // Where the next start begins, in milliseconds: the resume position, then where a track change happened.
  const startAt = useRef(search.start ?? 0);
  const [paused, setPaused] = useState(true);
  const [time, setTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [muted, setMuted] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [controls, setControls] = useState(true);
  const [menu, setMenu] = useState<"audio" | "subtitles" | "speed" | "external">();

  const back = useCallback(() => {
    if (router.history.length > 1) router.history.back();
    else void router.navigate({ to: "/s/$serverId/item/$itemId", params: { serverId: server.id, itemId } });
  }, [router, server.id, itemId]);

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
        v.play().catch(() => setPaused(true));
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
  // An open menu keeps the controls up.
  const shown = controls || menu !== undefined;

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

  const toggle = useCallback(() => {
    const v = video.current;
    if (!v) return;
    if (v.paused) void v.play();
    else v.pause();
  }, []);
  const seekBy = useCallback((s: number) => {
    const v = video.current;
    if (v) v.currentTime = Math.max(0, Math.min(v.duration || Infinity, v.currentTime + s));
  }, []);
  const fullScreen = useCallback(() => {
    if (document.fullscreenElement) void document.exitFullscreen();
    else void shell.current?.requestFullscreen();
  }, []);

  // Keyboard shortcuts: space and k, arrows and j/l, f, m.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement && e.target.type !== "range") return;
      const v = video.current;
      if (!v) return;
      const keys: Record<string, () => void> = {
        " ": toggle,
        k: toggle,
        ArrowLeft: () => seekBy(-10),
        j: () => seekBy(-10),
        ArrowRight: () => seekBy(10),
        l: () => seekBy(10),
        f: fullScreen,
        m: () => (v.muted = !v.muted),
        Escape: () => (document.fullscreenElement ? undefined : back()),
      };
      const f = keys[e.key];
      if (!f) return;
      e.preventDefault();
      f();
      poke();
    };
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  }, [toggle, seekBy, fullScreen, back, poke]);

  const source = item.data?.mediaSources.find((m) => m.id === info?.info.mediaSourceId);
  const audioTracks = source?.streams.filter((s) => s.kind === StreamKind.AUDIO) ?? [];
  const subtitleTracks = info?.info.subtitles ?? [];
  const external = subtitleTracks.filter((s) => s.method === SubtitleMethod.EXTERNAL && s.url);
  const total = duration || (item.data?.item?.runtime ? Number(item.data.item.runtime.seconds) : 0);
  const name = item.data?.item?.name ?? "";
  const series = item.data?.item?.seriesName;

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
        onClick={toggle}
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
        onRateChange={(e) => setSpeed(e.currentTarget.playbackRate)}
        onEnded={back}
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

      {error && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/70">
          <div className="max-w-md p-6 text-center">
            <Icon name="error" size={48} className="mx-auto mb-3 opacity-70" />
            <p className="mb-4 type-body">{error}</p>
            <button type="button" onClick={back} className="rounded-md bg-white/15 px-4 py-2 type-headline">
              {t("player.back")}
            </button>
          </div>
        </div>
      )}

      <div className={`pointer-events-none absolute inset-0 transition-opacity duration-300 ${shown ? "opacity-100" : "opacity-0"}`}>
        <div className="pointer-events-auto absolute inset-x-0 top-0 flex items-center gap-3 bg-gradient-to-b from-black/70 to-transparent p-4">
          <PlayerButton label={t("player.back")} onClick={back}>
            <Icon name="back" />
          </PlayerButton>
          <div className="min-w-0">
            {series && <div className="truncate type-footnote opacity-80">{series}</div>}
            <div className="truncate type-headline">{name}</div>
          </div>
        </div>

        <div className="pointer-events-auto absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent px-4 pt-10 pb-4">
          <input
            type="range"
            min={0}
            max={total || 1}
            step={1}
            value={Math.min(time, total || 1)}
            onChange={(e) => {
              if (video.current) video.current.currentTime = Number(e.target.value);
            }}
            aria-label={clock(time)}
            className="w-full accent-[var(--mavio-color-progress)]"
          />
          <div className="mt-1 flex items-center gap-1">
            <PlayerButton label={t("player.seekBack", { count: 10 })} onClick={() => seekBy(-10)}>
              <Icon name="seekBack" />
            </PlayerButton>
            <PlayerButton label={paused ? t("player.play") : t("player.pause")} onClick={toggle}>
              <Icon name={paused ? "play" : "pause"} fill size={28} />
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
            <span className="ml-2 type-footnote tabular-nums opacity-90">
              {clock(time)} / {clock(total)}
            </span>
            <div className="relative ml-auto flex items-center gap-1">
              {audioTracks.length > 1 && (
                <PlayerButton label={t("player.audio")} onClick={() => setMenu(menu === "audio" ? undefined : "audio")}>
                  <Icon name="audio" />
                </PlayerButton>
              )}
              {subtitleTracks.length > 0 && (
                <PlayerButton
                  label={t("player.subtitles")}
                  onClick={() => setMenu(menu === "subtitles" ? undefined : "subtitles")}
                >
                  <Icon name="subtitles" fill={(info?.info.subtitleStreamIndex ?? -1) >= 0} />
                </PlayerButton>
              )}
              <PlayerButton label={t("player.speed")} onClick={() => setMenu(menu === "speed" ? undefined : "speed")}>
                <Icon name="speed" />
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
              <PlayerButton label={t("player.openExternal")} onClick={() => setMenu(menu === "external" ? undefined : "external")}>
                <Icon name="more" />
              </PlayerButton>
              <PlayerButton label={t("player.fullScreen")} onClick={fullScreen}>
                <Icon name="fullScreen" />
              </PlayerButton>

              {menu && (
                <div className="absolute right-0 bottom-14 max-h-[60vh] w-72 overflow-y-auto rounded-lg bg-neutral-900/95 p-1 shadow-level3">
                  {menu === "audio" &&
                    audioTracks.map((s) => (
                      <MenuItem
                        key={s.index}
                        checked={s.index === info?.info.audioStreamIndex}
                        onClick={() => (setMenu(undefined), setSelection({ ...selection, audio: s.index }))}
                      >
                        {trackLabel(s)}
                      </MenuItem>
                    ))}
                  {menu === "subtitles" && (
                    <>
                      <MenuItem
                        checked={(info?.info.subtitleStreamIndex ?? -1) < 0}
                        onClick={() => (setMenu(undefined), setSelection({ ...selection, subtitle: -1 }))}
                      >
                        {t("player.subtitlesOff")}
                      </MenuItem>
                      {subtitleTracks.map((s) => {
                        const stream = source?.streams.find((x) => x.index === s.streamIndex);
                        return (
                          <MenuItem
                            key={s.streamIndex}
                            checked={s.streamIndex === info?.info.subtitleStreamIndex}
                            onClick={() => (setMenu(undefined), setSelection({ ...selection, subtitle: s.streamIndex }))}
                          >
                            {stream ? trackLabel(stream) : s.title || s.language || `#${s.streamIndex}`}
                          </MenuItem>
                        );
                      })}
                    </>
                  )}
                  {menu === "speed" &&
                    speeds.map((r) => (
                      <MenuItem
                        key={r}
                        checked={r === speed}
                        onClick={() => {
                          setMenu(undefined);
                          if (video.current) video.current.playbackRate = r;
                        }}
                      >
                        {r}×
                      </MenuItem>
                    ))}
                  {menu === "external" && info && (
                    <>
                      <MenuItem onClick={() => (setMenu(undefined), window.open(info.url, "_blank"))}>
                        {t("player.openExternal")}
                      </MenuItem>
                      <MenuItem onClick={() => (setMenu(undefined), void navigator.clipboard?.writeText(info.url))}>
                        {t("player.copyLink")}
                      </MenuItem>
                    </>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function PlayerButton({ label, onClick, children }: { label: string; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      className="flex h-11 w-11 items-center justify-center rounded-full hover:bg-white/15"
    >
      {children}
    </button>
  );
}

function MenuItem({ checked, onClick, children }: { checked?: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      role="menuitemradio"
      aria-checked={checked}
      onClick={onClick}
      className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left type-callout hover:bg-white/10"
    >
      <span className="w-4">{checked && <Icon name="played" fill size={16} />}</span>
      <span className="min-w-0 flex-1">{children}</span>
    </button>
  );
}

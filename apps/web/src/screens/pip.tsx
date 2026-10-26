// Picture in Picture (docs/clients.md §6.5): where the browser opens a
// document in its Picture in Picture window, the player's video moves into
// it, with its subtitles, under controls of the Mac's: close at the top
// leading corner, back to the player at the top trailing one, play and
// pause in the middle and the seek bar along the bottom. Elsewhere the
// browser's own Picture in Picture of the video.
import { type RefObject, useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { Spinner } from "../ui.tsx";
import { clock, SeekBar } from "./seekbar.tsx";

interface DocumentPictureInPicture {
  requestWindow(options?: { width?: number; height?: number }): Promise<Window>;
}

/**
 * The browser's Picture in Picture of documents, where it opens their
 * window: a browser of its own brand, as Chrome or Edge. One embedded in
 * an app, Chromium's brand alone, has the API but no window, and asking
 * for it uses up the click the video's own Picture in Picture needs.
 */
function documentPip(): DocumentPictureInPicture | undefined {
  const brands = (navigator as Navigator & { userAgentData?: { brands: { brand: string }[] } }).userAgentData?.brands ?? [];
  if (!brands.some((b) => b.brand !== "Chromium" && !/not.?a.?brand/i.test(b.brand))) return undefined;
  return (window as Window & { documentPictureInPicture?: DocumentPictureInPicture }).documentPictureInPicture;
}

/** The window's width as it opens, in CSS pixels. */
const openWidth = 480;

/** Gives d the page's styles, its root's classes and attributes, and its base for their URLs. */
function copyPage(d: Document) {
  const base = d.createElement("base");
  base.href = document.baseURI;
  d.head.append(base);
  for (const sheet of document.styleSheets) {
    try {
      const style = d.createElement("style");
      style.textContent = [...sheet.cssRules].map((r) => r.cssText).join("\n");
      d.head.append(style);
    } catch {
      // A sheet of another origin, whose rules cannot be read.
      if (!sheet.href) continue;
      const link = d.createElement("link");
      link.rel = "stylesheet";
      link.href = sheet.href;
      d.head.append(link);
    }
  }
  for (const a of document.documentElement.attributes) d.documentElement.setAttribute(a.name, a.value);
}

/**
 * Picture in Picture of video: whether the browser has it, the element of
 * its window the controls go in while it is open, and opening and closing
 * it. It closes as the player goes.
 */
export function usePictureInPicture(video: RefObject<HTMLVideoElement | null>) {
  const [host, setHost] = useState<HTMLElement>();
  // The browser's own Picture in Picture of the video.
  const [native, setNative] = useState(false);
  const win = useRef<Window>(undefined);
  // Set once the browser fails to open a window, which leaves the video's own.
  const noWindow = useRef(false);
  const supported = !!documentPip() || (typeof document !== "undefined" && document.pictureInPictureEnabled);

  useEffect(() => {
    const v = video.current;
    if (!v) return;
    const on = () => setNative(true);
    const off = () => setNative(false);
    v.addEventListener("enterpictureinpicture", on);
    v.addEventListener("leavepictureinpicture", off);
    return () => {
      v.removeEventListener("enterpictureinpicture", on);
      v.removeEventListener("leavepictureinpicture", off);
    };
  }, [video]);
  useEffect(() => () => win.current?.close(), []);

  const open = useCallback(async () => {
    const v = video.current;
    if (!v || win.current) return;
    const aspect = v.videoWidth > 0 && v.videoHeight > 0 ? v.videoWidth / v.videoHeight : 16 / 9;
    const pip = noWindow.current ? undefined : documentPip();
    if (!pip) {
      await v.requestPictureInPicture().catch(() => undefined);
      return;
    }
    const w = await pip.requestWindow({ width: openWidth, height: Math.round(openWidth / aspect) }).catch(() => undefined);
    // The click is used up: the next one opens the video's own.
    if (!w) {
      noWindow.current = true;
      return;
    }
    win.current = w;
    const d = w.document;
    copyPage(d);
    // The page's own rules for the body come before the utilities.
    d.body.style.cssText = "margin: 0; overflow: hidden; background: #000";
    const box = d.createElement("div");
    box.className = "relative h-screen w-screen overflow-hidden bg-black text-white select-none";
    d.body.append(box);
    // The video keeps playing as it moves, and comes back where it was.
    const mark = document.createComment("picture in picture");
    v.before(mark);
    box.append(v);
    w.addEventListener(
      "pagehide",
      () => {
        mark.replaceWith(v);
        win.current = undefined;
        setHost(undefined);
      },
      { once: true },
    );
    setHost(box);
  }, [video]);

  const close = useCallback(() => {
    if (win.current) win.current.close();
    else if (document.pictureInPictureElement) void document.exitPictureInPicture();
  }, []);

  return { supported, active: !!host || native, host, open, close };
}

/**
 * The controls of the Picture in Picture window, in host: shown while the
 * pointer moves over it or the video is paused; while the video loads,
 * its spinner in place of play and pause, shown either way.
 */
export function PictureInPictureControls({
  host,
  paused,
  loading,
  time,
  total,
  onToggle,
  onClose,
  onRestore,
  onScrubStart,
  onScrubEnd,
}: {
  host: HTMLElement;
  paused: boolean;
  loading: boolean;
  time: number;
  total: number;
  onToggle: () => void;
  /** Closes the window, the video paused. */
  onClose: () => void;
  /** Brings the video back to the player, playing on. */
  onRestore: () => void;
  onScrubStart: () => void;
  onScrubEnd: (s: number) => void;
}) {
  const [moved, setMoved] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  const poke = () => {
    setMoved(true);
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setMoved(false), 2000);
  };

  // Space plays and pauses in the window too.
  const toggle = useRef(onToggle);
  toggle.current = onToggle;
  useEffect(() => {
    const w = host.ownerDocument.defaultView;
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== " " && e.key !== "k") return;
      e.preventDefault();
      toggle.current();
    };
    w?.addEventListener("keydown", onKey);
    return () => w?.removeEventListener("keydown", onKey);
  }, [host]);

  const shown = moved || paused;
  const button = "flex items-center justify-center rounded-full hover:bg-white/15";
  return createPortal(
    <>
      {loading && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <Spinner size={40} className="text-white/70!" />
        </div>
      )}
      <div
        className={`absolute inset-0 transition-opacity duration-200 ${shown ? "opacity-100" : "cursor-none opacity-0"}`}
        onPointerMove={poke}
        onPointerDown={poke}
        onPointerLeave={() => (clearTimeout(timer.current), setMoved(false))}
      >
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-black/50 via-transparent via-40% to-black/70" />
        <button type="button" aria-label={t("action.close")} title={t("action.close")} onClick={onClose} className={`absolute top-2 left-2 size-8 ${button}`}>
          <Icon name="close" size={18} />
        </button>
        <button
          type="button"
          aria-label={t("player.pictureInPicture")}
          title={t("player.pictureInPicture")}
          onClick={onRestore}
          className={`absolute top-2 right-2 size-8 ${button}`}
        >
          <Icon name="pictureInPicture" size={18} />
        </button>
        {!loading && (
          <button
            type="button"
            aria-label={paused ? t("player.play") : t("player.pause")}
            onClick={onToggle}
            className={`absolute top-1/2 left-1/2 size-14 -translate-x-1/2 -translate-y-1/2 ${button}`}
          >
            <Icon name={paused ? "play" : "pause"} fill size={36} />
          </button>
        )}
        <div className="absolute inset-x-3 bottom-2 flex items-center gap-2 type-caption text-white/85 tabular-nums">
          <span>{clock(time)}</span>
          <div className="min-w-0 flex-1">
            <SeekBar time={time} total={total} chapters={[]} trickplay={[]} sheetUrl={(p) => p} onScrubStart={onScrubStart} onScrubEnd={onScrubEnd} />
          </div>
          <span>-{clock(Math.max(total - time, 0))}</span>
        </div>
      </div>
    </>,
    host,
  );
}

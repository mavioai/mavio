// The desktop player's seek bar (docs/clients.md §6.5): the track with what
// has played filled, gaps where chapters start, and over the time under the
// pointer the trickplay sheets' thumbnail with the chapter's name, below the
// bar where it does not fit above. Dragging pauses and releasing plays on.
import type { Trickplay } from "@mavio/proto/mavio/library/v1/item_pb";
import { type CSSProperties, useLayoutEffect, useRef, useState } from "react";

import { t } from "../i18n.ts";

/** A chapter's start, in seconds, and its name. */
export interface ChapterMark {
  start: number;
  title: string;
}

/** The sheets' thumbnails as the preview shows them, in CSS pixels. */
const previewWidth = 176;

/** The sheets of a width at least wide, else the widest. */
function chooseSheets(sheets: readonly Trickplay[], wide: number): Trickplay | undefined {
  const wider = sheets.filter((s) => s.width >= wide).sort((a, b) => a.width - b.width);
  return wider[0] ?? [...sheets].sort((a, b) => b.width - a.width)[0];
}

/** Seconds as "1:02:03" or "2:03". */
export function clock(s: number): string {
  if (!Number.isFinite(s) || s < 0) s = 0;
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = Math.floor(s % 60);
  const mm = h > 0 ? String(m).padStart(2, "0") : String(m);
  return `${h > 0 ? `${h}:` : ""}${mm}:${String(sec).padStart(2, "0")}`;
}

export function SeekBar({
  time,
  total,
  chapters,
  trickplay,
  sheetUrl,
  onScrubStart,
  onScrubEnd,
}: {
  time: number;
  total: number;
  chapters: readonly ChapterMark[];
  trickplay: readonly Trickplay[];
  /** The URL of a sheet's path the server gave. */
  sheetUrl: (path: string) => string;
  /** A drag began. */
  onScrubStart: () => void;
  /** A drag ended at a time. */
  onScrubEnd: (s: number) => void;
}) {
  const bar = useRef<HTMLDivElement>(null);
  // The time under the pointer and where it is across the bar.
  const [hover, setHover] = useState<{ time: number; x: number }>();
  // Where a drag has the knob, until the video gets there.
  const [scrub, setScrub] = useState<number>();
  const dragging = useRef(false);

  const track = useRef<HTMLDivElement>(null);

  // The time at a point, from the track, and where the point is across the bar.
  const at = (clientX: number) => {
    const b = bar.current!.getBoundingClientRect();
    const r = track.current!.getBoundingClientRect();
    const x = Math.min(Math.max(clientX, r.left), r.right);
    return { time: total > 0 ? ((x - r.left) / Math.max(r.width, 1)) * total : 0, x: x - b.left };
  };

  const position = scrub ?? time;
  const fraction = (s: number) => (total > 0 ? Math.min(Math.max(s / total, 0), 1) : 0);
  // A 2 pixel gap where each chapter after the first starts, as the Mac app's.
  const gaps = chapters
    .map((c) => c.start)
    .filter((s) => s > 0 && s < total)
    .flatMap((s) => {
      const p = `${fraction(s) * 100}%`;
      return [`#000 calc(${p} - 1px)`, `transparent calc(${p} - 1px)`, `transparent calc(${p} + 1px)`, `#000 calc(${p} + 1px)`];
    });
  const active = hover !== undefined || scrub !== undefined;

  return (
    <div className="relative">
      <div
        ref={bar}
        role="slider"
        data-seekbar
        tabIndex={0}
        aria-label={t("player.position")}
        aria-valuemin={0}
        aria-valuemax={Math.round(total)}
        aria-valuenow={Math.round(position)}
        aria-valuetext={clock(position)}
        className="relative flex h-5 cursor-pointer touch-none items-center outline-none focus-visible:ring-2 focus-visible:ring-white/60"
        onPointerMove={(e) => {
          if (total <= 0) return;
          const p = at(e.clientX);
          setHover(p);
          if (dragging.current) setScrub(p.time);
        }}
        onPointerLeave={() => !dragging.current && setHover(undefined)}
        onPointerDown={(e) => {
          if (total <= 0 || e.button !== 0) return;
          e.currentTarget.setPointerCapture(e.pointerId);
          dragging.current = true;
          const p = at(e.clientX);
          setHover(p);
          setScrub(p.time);
          onScrubStart();
        }}
        onPointerUp={(e) => {
          if (!dragging.current) return;
          dragging.current = false;
          const p = at(e.clientX);
          setScrub(undefined);
          const r = e.currentTarget.getBoundingClientRect();
          if (e.clientY < r.top || e.clientY > r.bottom || e.clientX < r.left || e.clientX > r.right) setHover(undefined);
          onScrubEnd(p.time);
        }}
        onPointerCancel={() => {
          if (!dragging.current) return;
          dragging.current = false;
          setScrub(undefined);
          setHover(undefined);
          onScrubEnd(position);
        }}
      >
        {/* The track is inset by half the knob, so the knob stays inside the bar at either end. */}
        <div ref={track} className="pointer-events-none absolute inset-x-1.5 flex h-full items-center">
          <div
            className={`relative w-full overflow-hidden rounded-full bg-white/30 transition-[height] ${active ? "h-1.5" : "h-1"}`}
            // The chapters' gaps are cut out of the track and its fill alike.
            style={gaps.length > 0 ? { maskImage: `linear-gradient(to right, ${["#000 0", ...gaps, "#000 100%"].join(", ")})` } : undefined}
          >
            <div className="h-full bg-white" style={{ width: `${fraction(position) * 100}%` }} />
          </div>
          {active && total > 0 && (
            <div className="absolute size-3 -translate-x-1/2 rounded-full bg-white" style={{ left: `${fraction(position) * 100}%` }} />
          )}
        </div>
      </div>
      {hover && total > 0 && (
        <Preview
          time={hover.time}
          x={hover.x}
          barWidth={bar.current?.clientWidth ?? 0}
          chapter={[...chapters].reverse().find((c) => c.start <= hover.time)?.title}
          sheets={chooseSheets(trickplay, previewWidth * (window.devicePixelRatio || 1))}
          sheetUrl={sheetUrl}
        />
      )}
    </div>
  );
}

/** The thumbnail of a time, the chapter's name and the time, above the bar or below it. */
function Preview({
  time,
  x,
  barWidth,
  chapter,
  sheets,
  sheetUrl,
}: {
  time: number;
  x: number;
  barWidth: number;
  chapter?: string;
  sheets?: Trickplay;
  sheetUrl: (path: string) => string;
}) {
  const box = useRef<HTMLDivElement>(null);
  const [below, setBelow] = useState(false);
  useLayoutEffect(() => {
    const b = box.current;
    if (!b) return;
    // Above the bar unless the window's top cuts it.
    setBelow(b.parentElement!.getBoundingClientRect().top - b.offsetHeight - 12 < 0);
  });

  let thumbnail: CSSProperties | undefined;
  if (sheets && sheets.tileWidth > 0 && sheets.tileHeight > 0 && sheets.thumbnailCount > 0 && sheets.interval && sheets.url) {
    const interval = Number(sheets.interval.seconds) + sheets.interval.nanos / 1e9;
    const perSheet = sheets.tileWidth * sheets.tileHeight;
    const index = Math.min(Math.max(Math.floor(time / Math.max(interval, 1e-3)), 0), sheets.thumbnailCount - 1);
    const tile = index % perSheet;
    const scale = previewWidth / Math.max(sheets.width, 1);
    thumbnail = {
      height: Math.round(sheets.height * scale),
      backgroundImage: `url("${sheetUrl(`${sheets.url}/${Math.floor(index / perSheet)}.jpg`)}")`,
      backgroundSize: `${sheets.tileWidth * previewWidth}px auto`,
      backgroundPosition: `-${(tile % sheets.tileWidth) * previewWidth}px -${Math.floor(tile / sheets.tileWidth) * sheets.height * scale}px`,
    };
  }
  const half = previewWidth / 2 + 4;
  const left = Math.min(Math.max(x, half), Math.max(barWidth - half, half));

  return (
    <div
      ref={box}
      className={`pointer-events-none absolute z-10 flex -translate-x-1/2 flex-col items-center gap-1 rounded-[8px] bg-black/75 px-1 pt-1 pb-1.5 ${below ? "top-7" : "bottom-7"}`}
      style={{ left }}
    >
      {thumbnail && <div className="rounded-[6px] bg-no-repeat" style={{ width: previewWidth, ...thumbnail }} />}
      <div className="truncate text-center type-footnote font-medium tabular-nums" style={{ width: previewWidth }}>
        {[chapter, clock(time)].filter(Boolean).join(" · ")}
      </div>
    </div>
  );
}

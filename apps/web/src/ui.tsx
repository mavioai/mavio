// The core components of libs/design/components.md for the web: cards,
// shelves, grids and status views.
import { durationMs, timestampDate } from "@bufbuild/protobuf/wkt";
import type { Item } from "@mavio/proto/mavio/library/v1/item_pb";
import { ImageKind, ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { LibraryKind } from "@mavio/proto/mavio/library/v1/library_pb";
import { artwork, type Server, thumbOf } from "@mavio/client-ts";
import { Link } from "@tanstack/react-router";
import { type ReactNode, type RefObject, useEffect, useRef, useState } from "react";

import { t } from "./i18n.ts";
import { Icon, type IconName } from "./icon.tsx";
import { usePrefs } from "./prefs.ts";

/** The shape of an item's card. */
export type Shape = "poster" | "thumb" | "square";

/** The card of an item of a kind: thumbnails for episodes and videos, squares for music. */
export function shapeOf(kind: ItemKind): Shape {
  switch (kind) {
    case ItemKind.EPISODE:
    case ItemKind.VIDEO:
    case ItemKind.MUSIC_VIDEO:
      return "thumb";
    case ItemKind.MUSIC_ALBUM:
    case ItemKind.MUSIC_ARTIST:
    case ItemKind.TRACK:
    case ItemKind.PHOTO:
    case ItemKind.PHOTO_ALBUM:
      return "square";
    default:
      return "poster";
  }
}

const aspects: Record<Shape, string> = { poster: "aspect-[2/3]", thumb: "aspect-video", square: "aspect-square" };
const widths: Record<Shape, string> = { poster: "w-poster", thumb: "w-thumbnail", square: "w-poster" };

/** The line under a card's title. */
export function subtitleOf(item: Item): string | undefined {
  switch (item.kind) {
    case ItemKind.EPISODE: {
      const n =
        item.parentIndexNumber && item.indexNumber
          ? `S${item.parentIndexNumber}:E${item.indexNumber}`
          : item.indexNumber
            ? `E${item.indexNumber}`
            : "";
      return [item.seriesName, n].filter(Boolean).join(" · ") || undefined;
    }
    case ItemKind.SEASON:
      return item.seriesName || undefined;
    case ItemKind.MUSIC_ALBUM:
    case ItemKind.TRACK:
      return (item.albumArtists.length ? item.albumArtists : item.artists).join(", ") || undefined;
    default:
      return item.productionYear ? String(item.productionYear) : undefined;
  }
}

/** How far the user watched an item, from 0 to 1; undefined when not begun or played. */
export function progressOf(item: Item): number | undefined {
  const pos = item.userData?.position;
  if (!pos || !item.runtime || item.userData?.played) return undefined;
  const p = durationMs(pos) / durationMs(item.runtime);
  return p > 0 && p < 1 ? p : undefined;
}

/** The minutes left of a partly watched item. */
export function minutesLeft(item: Item): number | undefined {
  const pos = item.userData?.position;
  if (!pos || !item.runtime) return undefined;
  return Math.max(1, Math.round((durationMs(item.runtime) - durationMs(pos)) / 60_000));
}

/** An item's runtime as "1 h 52 min". */
export function runtimeOf(item: Item): string | undefined {
  if (!item.runtime) return undefined;
  const m = Math.round(durationMs(item.runtime) / 60_000);
  if (m <= 0) return undefined;
  return m >= 60 ? `${Math.floor(m / 60)} h ${m % 60} min` : `${m} min`;
}

/** The year an item came out. */
export function yearOf(item: Item): number | undefined {
  return item.productionYear || (item.premiereDate ? timestampDate(item.premiereDate).getFullYear() : undefined);
}

/** An image that fades in, over a placeholder of its final size. */
export function Artwork({ src, alt, className }: { src?: string; alt: string; className?: string }) {
  const [loaded, setLoaded] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <div data-art className={`relative overflow-hidden bg-surface-raised ${className ?? ""}`}>
      {src && !failed ? (
        <img
          src={src}
          alt=""
          loading="lazy"
          decoding="async"
          onLoad={() => setLoaded(true)}
          onError={() => setFailed(true)}
          className={`absolute inset-0 h-full w-full object-cover transition-opacity duration-300 ${loaded ? "opacity-100" : "opacity-0"}`}
        />
      ) : (
        <div className="absolute inset-0 flex items-center justify-center p-3 text-center type-title3 text-text-secondary">
          {alt}
        </div>
      )}
    </div>
  );
}

/** The card of an item: poster, thumbnail or square, with watched progress and the played check. */
export function ItemCard({
  server,
  serverId,
  item,
  shape,
  fluid = false,
}: {
  server: Server;
  serverId: string;
  item: Item;
  shape?: Shape;
  /** Fills the width of a grid cell rather than the shelf's fixed width. */
  fluid?: boolean;
}) {
  const s = shape ?? shapeOf(item.kind);
  const width = s === "thumb" ? 320 : 200;
  const image =
    s === "thumb" ? thumbOf(server, item, width) : artwork(server, item, ImageKind.PRIMARY, width);
  const progress = progressOf(item);
  const subtitle = subtitleOf(item);
  return (
    <Link
      to="/s/$serverId/item/$itemId"
      params={{ serverId, itemId: item.id }}
      className={`group block shrink-0 snap-start ${fluid ? "w-full" : widths[s]}`}
    >
      <div className="relative">
        <Artwork
          src={image?.url}
          alt={item.name}
          className={`${aspects[s]} rounded-md transition group-hover:brightness-110 group-active:opacity-80`}
        />
        {progress !== undefined && (
          <div className="absolute inset-x-2.5 bottom-2.5 h-1 overflow-hidden rounded-full bg-white/35 shadow-level1">
            <div className="h-full rounded-full bg-white" style={{ width: `${progress * 100}%` }} />
          </div>
        )}
        {item.userData?.played && (
          <span className="absolute top-1.5 right-1.5 rounded-full bg-scrim p-0.5 text-on-background">
            <Icon name="played" fill size={18} />
          </span>
        )}
      </div>
      <div className="mt-2 truncate type-headline">{item.name}</div>
      {subtitle && <div className="truncate type-footnote text-text-secondary">{subtitle}</div>}
    </Link>
  );
}

/** A placeholder card, while a shelf loads. */
export function CardPlaceholder({ shape = "poster" }: { shape?: Shape }) {
  return (
    <div className={`shrink-0 ${widths[shape]}`} aria-hidden>
      <div className={`${aspects[shape]} rounded-md bg-surface-raised`} />
      <div className="mt-2 h-4 w-3/4 rounded-xs bg-surface-raised" />
    </div>
  );
}

/**
 * A titled row of cards that scrolls sideways, with arrows at the end of
 * its title where a mouse points; a remote moves through it by focus.
 */
export function Shelf({
  title,
  badge,
  more,
  children,
}: {
  title: ReactNode;
  badge?: ReactNode;
  more?: ReactNode;
  children: ReactNode;
}) {
  const { tv } = usePrefs();
  const row = useRef<HTMLDivElement>(null);
  const [ends, setEnds] = useState({ start: true, end: true });
  useEffect(() => {
    const el = row.current;
    if (!el || tv) return;
    const update = () =>
      setEnds({ start: el.scrollLeft <= 1, end: el.scrollLeft + el.clientWidth >= el.scrollWidth - 1 });
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    el.addEventListener("scroll", update, { passive: true });
    return () => (ro.disconnect(), el.removeEventListener("scroll", update));
  }, [tv]);
  const scroll = (dir: number) => row.current?.scrollBy({ left: dir * row.current.clientWidth * 0.8, behavior: "smooth" });
  const arrows = !tv && !(ends.start && ends.end);
  return (
    <section className="py-3">
      <div className="mb-2 flex items-center gap-3 px-margin">
        <h2 className="truncate type-title3">{title}</h2>
        {badge}
        <div className="ml-auto flex shrink-0 items-center gap-3">
          {more && <div className="type-callout text-accent-text hover:opacity-80">{more}</div>}
          {arrows && (
            <div className="hidden gap-1 pointer-fine:flex">
              {(["back", "forward"] as const).map((dir) => (
                <button
                  key={dir}
                  type="button"
                  tabIndex={-1}
                  aria-label={dir === "back" ? t("action.scrollBack") : t("action.scrollForward")}
                  disabled={dir === "back" ? ends.start : ends.end}
                  onClick={() => scroll(dir === "back" ? -1 : 1)}
                  className="flex size-7 items-center justify-center rounded-full text-text-secondary hover:bg-fill hover:text-on-background disabled:text-text-disabled disabled:hover:bg-transparent"
                >
                  <Icon name={dir} size={18} />
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
      <div ref={row} className={`scroll-row flex overflow-x-auto px-margin pb-1 ${tv ? "gap-gutter" : gap}`}>
        {children}
      </div>
    </section>
  );
}

/** The space between cards side by side, wider than the gutter outside TV mode. */
const gap = "gap-x-[calc(var(--mavio-layout-gutter)*1.5)]";

/** A grid of cards whose columns are at least a poster wide. */
export function Grid({ shape = "poster", children }: { shape?: Shape; children: ReactNode }) {
  const { tv } = usePrefs();
  const min = shape === "thumb" ? "var(--mavio-layout-thumbnail)" : "var(--mavio-layout-poster)";
  return (
    <div className={`grid gap-y-6 px-margin ${tv ? "gap-x-gutter" : gap}`} style={{ gridTemplateColumns: `repeat(auto-fill, minmax(${min}, 1fr))` }}>
      {children}
    </div>
  );
}

/** Calls onVisible when the element scrolls into view, to load the next page. */
export function LoadMore({ onVisible, active }: { onVisible: () => void; active: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el || !active) return;
    const io = new IntersectionObserver((entries) => entries.some((e) => e.isIntersecting) && onVisible(), {
      rootMargin: "600px",
    });
    io.observe(el);
    return () => io.disconnect();
  }, [onVisible, active]);
  return <div ref={ref} className="h-px" />;
}

/** The empty, error, offline and loading states of a screen or list. */
export function StatusView({
  icon,
  title,
  message,
  action,
}: {
  icon?: IconName;
  title: string;
  message?: string;
  action?: { label: string; onClick: () => void };
}) {
  return (
    <div className="flex flex-col items-center gap-3 px-margin py-16 text-center">
      {icon && <Icon name={icon} size={48} className="text-text-tertiary" />}
      <h2 className="type-title3">{title}</h2>
      {message && <p className="max-w-md type-body text-text-secondary">{message}</p>}
      {action && (
        <Button variant="secondary" onClick={action.onClick}>
          {action.label}
        </Button>
      )}
    </div>
  );
}

/** A button of the primary, secondary or plain kind. */
export function Button({
  variant = "primary",
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "secondary" | "plain" }) {
  const styles = {
    primary: "bg-button text-on-button hover:opacity-85",
    secondary: "bg-fill text-on-background hover:bg-fill-selected",
    plain: "text-on-background hover:bg-surface-raised",
  }[variant];
  return (
    <button
      type="button"
      {...props}
      className={`inline-flex min-h-11 items-center justify-center gap-2 rounded-md px-4 type-headline transition active:opacity-80 disabled:text-text-disabled disabled:opacity-60 ${styles} ${className ?? ""}`}
    />
  );
}

/** A text field with its label. */
export function Field({
  label,
  ...props
}: React.InputHTMLAttributes<HTMLInputElement> & { label: string }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="type-callout text-text-secondary">{label}</span>
      <input
        {...props}
        className="min-h-11 rounded-md border border-border bg-surface px-3 type-body text-on-background placeholder:text-text-tertiary focus:border-focus focus:outline-none"
      />
    </label>
  );
}

/** The screen's title, with the margin of the layout. */
export function ScreenTitle({ children }: { children: ReactNode }) {
  return <h1 className="px-margin pt-6 pb-2 type-title1">{children}</h1>;
}

/** The loading state of a screen. */
export function Loading() {
  return <div className="px-margin py-16 text-center type-body text-text-secondary">{t("state.loading")}</div>;
}

/** The icon of a library of a kind. */
export function libraryIcon(kind: LibraryKind): IconName {
  switch (kind) {
    case LibraryKind.MOVIES:
      return "movies";
    case LibraryKind.SHOWS:
      return "shows";
    case LibraryKind.MUSIC:
    case LibraryKind.MUSIC_VIDEOS:
      return "music";
    case LibraryKind.HOME_VIDEOS:
      return "homeVideos";
    case LibraryKind.BOOKS:
      return "books";
    case LibraryKind.PHOTOS:
      return "photos";
    case LibraryKind.COLLECTIONS:
      return "collections";
    case LibraryKind.PLAYLISTS:
      return "playlists";
    default:
      return "folder";
  }
}

/** A person's initial on a color their name picks (libs/design/components.md, Avatar). */
export function Avatar({ name, size = 32, className }: { name: string; size?: number; className?: string }) {
  let hue = 0;
  for (const c of name) hue = (hue * 31 + c.codePointAt(0)!) % 360;
  return (
    <span
      aria-hidden
      className={`inline-flex shrink-0 items-center justify-center rounded-full font-semibold text-white ${className ?? ""}`}
      style={{ width: size, height: size, fontSize: size * 0.42, background: `hsl(${hue} 42% 44%)` }}
    >
      {[...name][0]?.toUpperCase()}
    </span>
  );
}

/** Calls onClose on Escape and on a press outside ref, while open. */
export function useDismiss(ref: RefObject<HTMLElement | null>, open: boolean, onClose: () => void) {
  useEffect(() => {
    if (!open) return;
    const down = (e: PointerEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    // Before other handlers of Escape, such as going back on a television.
    const key = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      e.preventDefault();
      onClose();
    };
    document.addEventListener("pointerdown", down);
    document.addEventListener("keydown", key, true);
    return () => {
      document.removeEventListener("pointerdown", down);
      document.removeEventListener("keydown", key, true);
    };
  }, [ref, open, onClose]);
}

/** A group of rows on a raised surface, as in the settings. */
export function Group({ title, footer, children }: { title?: ReactNode; footer?: ReactNode; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-2">
      {title && <h2 className="px-1 type-headline">{title}</h2>}
      <div className="flex flex-col divide-y divide-border overflow-hidden rounded-[14px] bg-surface">{children}</div>
      {footer && <p className="px-1 type-footnote text-text-secondary">{footer}</p>}
    </section>
  );
}

/** A row of a group: a label, a line under it and its control or value. */
export function Row({
  label,
  detail,
  leading,
  children,
}: {
  label: ReactNode;
  detail?: ReactNode;
  leading?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className="flex min-h-13 flex-wrap items-center gap-3 px-4 py-2.5">
      {leading}
      <div className="min-w-0 flex-1">
        <div className="type-body">{label}</div>
        {detail && <div className="truncate type-footnote text-text-secondary">{detail}</div>}
      </div>
      {children}
    </div>
  );
}

/** A switch, green when on as Apple's are. */
export function Toggle({ on, onChange, label }: { on: boolean; onChange: (on: boolean) => void; label: string }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      aria-label={label}
      onClick={() => onChange(!on)}
      className={`relative h-[31px] w-[51px] shrink-0 rounded-full transition-colors ${on ? "bg-[#34C759]" : "bg-fill-selected"}`}
    >
      <span
        className={`absolute top-[2px] left-[2px] h-[27px] w-[27px] rounded-full bg-white shadow-level1 transition-transform ${on ? "translate-x-5" : ""}`}
      />
    </button>
  );
}

/** A choice among options, drawn as a native select. */
export function Select<T extends string>({
  value,
  options,
  onChange,
  label,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (v: T) => void;
  label: string;
}) {
  return (
    <select
      value={value}
      aria-label={label}
      onChange={(e) => onChange(e.target.value as T)}
      className="min-h-9 max-w-full rounded-md bg-fill px-2 type-callout text-on-background"
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  );
}

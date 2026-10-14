// The hero at the top of Home (docs/clients.md §6.3): the latest films and
// shows with backdrops, one after another, each with its facts, overview
// and the buttons to play it and open its details.
import { artwork, type Section, type ServerEntry } from "@mavio/client-ts";
import { ImageKind, type Item, ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { HubKind } from "@mavio/proto/mavio/library/v1/library_pb";
import { Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { usePrefs, useWidthClass } from "../prefs.ts";
import { runtimeOf, yearOf } from "../ui.tsx";
import { sectionTitle } from "./home.tsx";

/** A title the hero shows, and the line above it. */
export interface HeroEntry {
  server: ServerEntry;
  item: Item;
  eyebrow: string;
}

const heroKinds = new Set([ItemKind.MOVIE, ItemKind.SERIES, ItemKind.VIDEO, ItemKind.MUSIC_VIDEO]);

/** Up to five titles with backdrops from Home's sections, Continue Watching aside. */
export function heroEntries(sections: Section[], servers: ServerEntry[]): HeroEntry[] {
  const out: HeroEntry[] = [];
  const seen = new Set<string>();
  for (const s of sections) {
    if (s.kind === HubKind.CONTINUE_WATCHING || s.content.type !== "items") continue;
    for (const e of s.content.entries) {
      const server = servers.find((x) => x.id === e.serverId);
      const key = `${e.serverId}/${e.item.id}`;
      if (!server || seen.has(key) || !heroKinds.has(e.item.kind)) continue;
      if (!e.item.images.some((i) => i.kind === ImageKind.BACKDROP && i.url)) continue;
      seen.add(key);
      out.push({ server, item: e.item, eyebrow: sectionTitle(s) });
      if (out.length === 5) return out;
    }
  }
  return out;
}

/** How long each title shows before the next. */
const interval = 9000;

export function Hero({ entries }: { entries: HeroEntry[] }) {
  const [at, setAt] = useState(0);
  const [paused, setPaused] = useState(false);
  const width = useWidthClass();
  const { tv } = usePrefs();
  useEffect(() => {
    if (paused || entries.length < 2) return;
    const h = setInterval(() => setAt((i) => (i + 1) % entries.length), interval);
    return () => clearInterval(h);
  }, [paused, entries.length]);
  if (entries.length === 0) return null;
  const { server, item, eyebrow } = entries[at % entries.length];
  const backdrop = artwork(server, item, ImageKind.BACKDROP, tv ? 1920 : 1600);
  const logo = artwork(server, item, ImageKind.LOGO, 600);
  const phone = width === "compact" && !tv;
  const facts = [yearOf(item), runtimeOf(item), ...item.genres.slice(0, 2)].filter(Boolean);
  const playable = item.kind !== ItemKind.SERIES;
  const frame = tv
    ? "h-[78vh]"
    : phone
      ? "h-[min(72vh,540px)]"
      : "mx-2.5 mt-2.5 h-[470px] rounded-[18px]";
  return (
    <section
      aria-roledescription="carousel"
      aria-label={eyebrow}
      onPointerEnter={() => setPaused(true)}
      onPointerLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
      className={`relative overflow-hidden bg-surface ${frame}`}
    >
      {backdrop && (
        <img key={backdrop.url} src={backdrop.url} alt="" className="absolute inset-0 h-full w-full animate-[fade_600ms_ease-out] object-cover" />
      )}
      <div
        className={`absolute inset-0 ${
          phone
            ? "bg-gradient-to-t from-black/90 via-black/40 to-transparent"
            : "bg-[linear-gradient(90deg,rgb(0_0_0/0.78)_0%,rgb(0_0_0/0.35)_45%,transparent_75%),linear-gradient(0deg,rgb(0_0_0/0.55)_0%,transparent_45%)]"
        }`}
      />
      <div
        className={`absolute flex flex-col gap-3 text-white ${
          tv ? "bottom-[14vh] left-margin max-w-[860px]" : phone ? "inset-x-margin bottom-8" : "bottom-10 left-10 max-w-[560px]"
        }`}
      >
        <div className="type-footnote font-semibold tracking-wide text-white/75 uppercase">{eyebrow}</div>
        {logo ? (
          <img src={logo.url} alt={item.name} className={`object-contain object-left ${tv ? "max-h-40" : "max-h-24"} max-w-[80%]`} />
        ) : (
          <h2 className={`font-bold leading-[1.05] ${tv ? "text-[72px]" : phone ? "text-[34px]" : "text-[52px]"}`}>{item.name}</h2>
        )}
        <div className="flex flex-wrap items-center gap-2 type-callout text-white/80">
          {facts.map((f, i) => (
            <span key={i} className="flex items-center gap-2">
              {i > 0 && <span aria-hidden>·</span>}
              {f}
            </span>
          ))}
          {item.officialRating && (
            <span className="rounded-[4px] border border-white/50 px-1.5 type-caption text-white/90">{item.officialRating}</span>
          )}
        </div>
        {item.overview && !phone && <p className={`line-clamp-3 text-white/85 ${tv ? "type-body" : "type-body"}`}>{item.overview}</p>}
        <div className={`mt-2 flex gap-3 ${phone ? "[&>*]:flex-1" : ""}`}>
          {playable ? (
            <Link
              to="/s/$serverId/play/$itemId"
              params={{ serverId: server.id, itemId: item.id }}
              data-tv-button
              className={`inline-flex items-center justify-center gap-2 rounded-[10px] bg-white font-semibold text-black hover:bg-white/90 ${tv ? "h-16 px-10 type-headline" : "h-11 px-6 type-headline"}`}
            >
              <Icon name="play" fill size={tv ? 28 : 20} />
              {t("player.play")}
            </Link>
          ) : null}
          <Link
            to="/s/$serverId/item/$itemId"
            params={{ serverId: server.id, itemId: item.id }}
            data-tv-button
            className={`inline-flex items-center justify-center gap-2 rounded-[10px] bg-white/20 font-semibold text-white backdrop-blur-xl hover:bg-white/30 ${tv ? "h-16 px-10 type-headline" : "h-11 px-6 type-headline"}`}
          >
            <Icon name="info" size={tv ? 28 : 20} />
            {t("item.details")}
          </Link>
        </div>
      </div>
      {entries.length > 1 && (
        <div className={`absolute flex gap-1.5 ${tv ? "right-margin bottom-[14vh]" : phone ? "top-4 right-margin" : "right-8 bottom-8"}`}>
          {entries.map((e, i) => (
            <button
              key={`${e.server.id}/${e.item.id}`}
              type="button"
              tabIndex={tv ? -1 : 0}
              aria-label={e.item.name}
              aria-current={i === at}
              onClick={() => setAt(i)}
              className={`h-1.5 rounded-full transition-all ${i === at ? "w-5 bg-white" : "w-1.5 bg-white/45 hover:bg-white/70"}`}
            />
          ))}
        </div>
      )}
    </section>
  );
}

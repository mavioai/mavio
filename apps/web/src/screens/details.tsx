// An item's details: the header with its actions, its versions on this and
// the user's other servers, its seasons, episodes or tracks, its cast and
// items like it (libs/design/components.md).
import { durationMs, timestampDate } from "@bufbuild/protobuf/wkt";
import {
  artwork,
  type Copy,
  type ServerEntry,
  thumbOf,
  useChildren,
  useItem,
  useOtherCopies,
  useServer,
  useSignedIn,
  useSimilar,
  useUserData,
} from "@mavio/client-ts";
import { CreditKind, type Credit, ImageKind, type Item, ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { type MediaSource, type MediaStream, StreamKind, VideoRange } from "@mavio/proto/mavio/library/v1/media_pb";
import { getRouteApi, Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { locale, t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { usePrefs, useWidthClass } from "../prefs.ts";
import {
  Artwork,
  Button,
  Grid,
  ItemCard,
  Loading,
  minutesLeft,
  progressOf,
  runtimeOf,
  Shelf,
  StatusView,
  yearOf,
} from "../ui.tsx";

const route = getRouteApi("/s/$serverId/item/$itemId");

export function Details() {
  const { serverId, itemId } = route.useParams();
  const server = useServer(serverId);
  const q = useItem(server?.token ? server : undefined, itemId);
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  if (q.isPending) return <Loading />;
  if (q.isError || !q.data.item) {
    return (
      <StatusView
        icon="error"
        title={t("state.error")}
        action={{ label: t("action.retry"), onClick: () => void q.refetch() }}
      />
    );
  }
  return (
    <ItemDetails
      key={itemId}
      server={server}
      item={q.data.item}
      sources={q.data.mediaSources}
      credits={q.data.credits}
    />
  );
}

/** The resolution of a source's first video stream, as "4K" or "1080p". */
function resolutionOf(source?: MediaSource): string | undefined {
  const v = source?.streams.find((s) => s.kind === StreamKind.VIDEO);
  if (!v?.height) return undefined;
  if (v.width >= 3200 || v.height >= 2000) return "4K";
  if (v.width >= 1800 || v.height >= 1000) return "1080p";
  if (v.width >= 1200 || v.height >= 700) return "720p";
  return `${v.height}p`;
}

function sizeOf(bytes: bigint): string {
  const n = Number(bytes);
  return n >= 1e9 ? `${(n / 1e9).toFixed(1)} GB` : `${Math.round(n / 1e6)} MB`;
}

/** A position as 47:12 or 1:05:30. */
function clock(ms: number): string {
  const total = Math.floor(ms / 1000);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const sec = String(total % 60).padStart(2, "0");
  return h > 0 ? `${h}:${String(m).padStart(2, "0")}:${sec}` : `${m}:${sec}`;
}

/** A length of time as "1 h 21 min". */
function minutesText(m: number): string {
  return m >= 60 ? `${Math.floor(m / 60)} h ${m % 60} min` : `${m} min`;
}

const rangeNames: Partial<Record<VideoRange, string>> = {
  [VideoRange.HDR10]: "HDR10",
  [VideoRange.HDR10_PLUS]: "HDR10+",
  [VideoRange.HLG]: "HLG",
  [VideoRange.DOLBY_VISION]: "Dolby Vision",
};

/** The dynamic range of a source's video when it is HDR. */
function rangeOf(source?: MediaSource): string | undefined {
  const v = source?.streams.find((s) => s.kind === StreamKind.VIDEO);
  return v ? rangeNames[v.range] : undefined;
}

const languageNames = new Intl.DisplayNames([locale], { type: "language", fallback: "none" });

/** A stream's language or title, codec and channels, as the facts list them. */
function streamText(s: MediaStream): string {
  let lang: string | undefined;
  try {
    lang = s.language && s.language !== "und" ? languageNames.of(s.language) : undefined;
  } catch {
    lang = s.language;
  }
  const channels = s.channels > 0 ? (s.channels === 6 ? "5.1" : s.channels === 8 ? "7.1" : `${s.channels} ch`) : "";
  return [s.title || lang, s.codec.toUpperCase(), channels].filter(Boolean).join(" · ");
}

function ItemDetails({
  server,
  item,
  sources,
  credits,
}: {
  server: ServerEntry;
  item: Item;
  sources: MediaSource[];
  credits: Credit[];
}) {
  const several = useSignedIn().length > 1;
  const children = useChildren(server, item);
  const similar = useSimilar(server, item.id);
  const copies = useOtherCopies(server.id, item);
  const userData = useUserData(server);
  const [source, setSource] = useState(sources[0]?.id);
  const width = useWidthClass();
  const { tv } = usePrefs();
  const phone = width === "compact" && !tv;
  const wide = width === "expanded" || tv;

  const backdrop = artwork(server, item, ImageKind.BACKDROP, tv ? 1920 : 1600);
  const logo = artwork(server, item, ImageKind.LOGO, 600);
  const played = !!item.userData?.played;
  const favorite = !!item.userData?.favorite;
  const position = item.userData?.position ? durationMs(item.userData.position) : 0;
  const progress = progressOf(item);
  const resumable = progress !== undefined;
  const playable = sources.length > 0;
  const firstUnplayed = item.kind === ItemKind.SEASON ? children.data?.find((c) => !c.userData?.played) : undefined;
  const selected = sources.find((s) => s.id === source);
  const versions = sources.length > 1 || copies.length > 0;

  const facts = [
    yearOf(item),
    runtimeOf(item),
    item.communityRating > 0 ? `★ ${item.communityRating.toFixed(1)}` : undefined,
    ...item.genres.slice(0, 3),
  ].filter(Boolean);
  const badges = [item.officialRating, resolutionOf(selected), rangeOf(selected)].filter(Boolean);

  const icon = tv ? 28 : 20;
  const circle = `inline-flex items-center justify-center rounded-full bg-white/20 text-white backdrop-blur-xl hover:bg-white/30 ${tv ? "h-16 w-16" : "h-11 w-11"}`;
  const primary = `inline-flex items-center justify-center gap-2 rounded-[10px] bg-white font-semibold text-black hover:bg-white/90 type-headline ${
    tv ? "h-16 px-10" : phone ? "h-11 w-full px-5" : "h-11 px-5"
  }`;

  return (
    <div className="pb-6">
      <header
        className={`relative overflow-hidden bg-surface ${
          tv ? "h-[86vh]" : phone ? "h-[64vh] min-h-96" : "mx-2.5 mt-2.5 h-[clamp(480px,64vh,640px)] rounded-[18px]"
        }`}
      >
        {backdrop && <img src={backdrop.url} alt="" className="absolute inset-0 h-full w-full object-cover" />}
        <div
          className={`absolute inset-0 ${
            phone
              ? "bg-gradient-to-t from-black via-black/50 to-transparent"
              : "bg-[linear-gradient(90deg,rgb(0_0_0/0.8)_0%,rgb(0_0_0/0.4)_45%,transparent_75%),linear-gradient(0deg,rgb(0_0_0/0.7)_0%,transparent_50%)]"
          }`}
        />
        <div className={`absolute inset-x-0 bottom-0 flex items-end gap-8 ${phone ? "px-margin pb-6" : tv ? "px-margin pb-[10vh]" : "p-10"}`}>
          <div className={`flex min-w-0 flex-col gap-3 text-white ${tv ? "max-w-[900px]" : "max-w-[620px]"}`}>
            {(item.kind === ItemKind.EPISODE || item.kind === ItemKind.SEASON) && item.seriesId && (
              <Link
                to="/s/$serverId/item/$itemId"
                params={{ serverId: server.id, itemId: item.seriesId }}
                className="type-headline text-white/80 hover:underline"
              >
                {item.seriesName}
              </Link>
            )}
            {logo ? (
              <img src={logo.url} alt={item.name} className={`max-w-[80%] object-contain object-left ${tv ? "max-h-40" : "max-h-28"}`} />
            ) : (
              <h1 className={`font-bold leading-[1.05] ${tv ? "text-[72px]" : phone ? "text-[34px]" : "text-[52px]"}`}>{item.name}</h1>
            )}
            {item.kind === ItemKind.EPISODE && item.indexNumber > 0 && (
              <div className="type-headline">{t("item.episodeNumber", { season: item.parentIndexNumber, episode: item.indexNumber })}</div>
            )}
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1 type-callout text-white/80">
              {facts.map((f, i) => (
                <span key={i} className="flex items-center gap-2">
                  {i > 0 && <span aria-hidden>·</span>}
                  {f}
                </span>
              ))}
              {badges.map((b) => (
                <span key={b} className="rounded-[4px] border border-white/50 px-1.5 type-caption text-white/90">
                  {b}
                </span>
              ))}
              {several && <span className="rounded-[4px] bg-white/20 px-1.5 type-caption">{server.name}</span>}
            </div>
            {item.overview && !phone && <p className="line-clamp-3 type-body text-white/85">{item.overview}</p>}
            <div className="mt-1 flex flex-wrap items-center gap-3">
              {playable && (
                <Link
                  to="/s/$serverId/play/$itemId"
                  params={{ serverId: server.id, itemId: item.id }}
                  search={{ start: resumable ? position : undefined, source }}
                  data-tv-button
                  className={primary}
                >
                  <Icon name="play" fill size={icon} />
                  {resumable ? t("item.resumeAt", { time: clock(position) }) : t("item.play")}
                </Link>
              )}
              {firstUnplayed && (
                <Link
                  to="/s/$serverId/play/$itemId"
                  params={{ serverId: server.id, itemId: firstUnplayed.id }}
                  data-tv-button
                  className={primary}
                >
                  <Icon name="play" fill size={icon} />
                  {t("item.play")}
                </Link>
              )}
              {resumable && (
                <Link
                  to="/s/$serverId/play/$itemId"
                  params={{ serverId: server.id, itemId: item.id }}
                  search={{ source }}
                  aria-label={t("item.play")}
                  title={t("item.play")}
                  data-tv-button
                  className={circle}
                >
                  <Icon name="previous" size={icon} />
                </Link>
              )}
              <button
                type="button"
                aria-pressed={favorite}
                aria-label={t("item.favorite")}
                title={t("item.favorite")}
                onClick={() => userData.mutate({ itemId: item.id, favorite: !favorite })}
                className={circle}
              >
                <Icon name="favorite" fill={favorite} size={icon} className={favorite ? "text-[#FF453A]" : undefined} />
              </button>
              <button
                type="button"
                aria-pressed={played}
                aria-label={played ? t("item.markUnplayed") : t("item.markPlayed")}
                title={played ? t("item.markUnplayed") : t("item.markPlayed")}
                onClick={() => userData.mutate({ itemId: item.id, played: !played })}
                className={circle}
              >
                <Icon name="played" fill={played} size={icon} />
              </button>
            </div>
            {resumable && (
              <div className="flex items-center gap-3 type-footnote text-white/80">
                <div className="h-1 w-56 overflow-hidden rounded-full bg-white/35">
                  <div className="h-full rounded-full bg-white" style={{ width: `${progress * 100}%` }} />
                </div>
                {t("item.timeLeft", { time: minutesText(minutesLeft(item) ?? 0) })}
              </div>
            )}
          </div>
          {wide && versions && (
            <VersionsPanel
              server={server}
              sources={sources}
              selected={source}
              onSelect={setSource}
              copies={copies}
              className="ml-auto w-[320px] shrink-0"
            />
          )}
        </div>
      </header>

      {phone && item.overview && (
        <div className="px-margin pt-4">
          <Overview text={item.overview} />
        </div>
      )}
      {!wide && versions && (
        <div className="px-margin pt-4">
          <VersionsPanel server={server} sources={sources} selected={source} onSelect={setSource} copies={copies} />
        </div>
      )}
      <div className="pt-4">
        <Children server={server} item={item} items={children.data ?? []} />
        <Cast server={server} credits={credits} />
        {(similar.data?.length ?? 0) > 0 && (
          <Shelf title={t("item.similar")}>
            {similar.data!.map((i) => (
              <ItemCard key={i.id} server={server} serverId={server.id} item={i} />
            ))}
          </Shelf>
        )}
        <Facts item={item} credits={credits} source={selected} />
      </div>
    </div>
  );
}

function Overview({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  return (
    <button type="button" onClick={() => setOpen(!open)} className={`text-left type-body ${open ? "" : "line-clamp-3"}`}>
      {text}
    </button>
  );
}

/** The media files of an item to choose from, and its copies on the user's other servers. */
function VersionsPanel({
  server,
  sources,
  selected,
  onSelect,
  copies,
  className,
}: {
  server: ServerEntry;
  sources: MediaSource[];
  selected?: string;
  onSelect: (id: string) => void;
  copies: Copy[];
  className?: string;
}) {
  return (
    <section className={`rounded-[16px] border border-white/15 bg-black/45 p-2 text-white backdrop-blur-2xl ${className ?? ""}`}>
      <h2 className="px-2.5 pt-1.5 pb-1 type-footnote font-semibold text-white/70">{t("item.versions")}</h2>
      <div role="radiogroup" aria-label={t("item.versions")} className="flex flex-col">
        {sources.map((s) => (
          <button
            key={s.id}
            type="button"
            role="radio"
            aria-checked={s.id === selected}
            onClick={() => onSelect(s.id)}
            className="flex items-center gap-3 rounded-[10px] px-2.5 py-2 text-left hover:bg-white/10"
          >
            <span
              className={`flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-full border-2 ${
                s.id === selected ? "border-accent" : "border-white/50"
              }`}
            >
              {s.id === selected && <span className="h-2 w-2 rounded-full bg-accent" />}
            </span>
            <span className="min-w-0">
              <span className="block truncate type-callout font-semibold">
                {s.name || [resolutionOf(s), rangeOf(s)].filter(Boolean).join(" ") || s.container.toUpperCase()}
              </span>
              <span className="block truncate type-caption text-white/65">
                {[s.container.toUpperCase(), s.sizeBytes > 0n ? sizeOf(s.sizeBytes) : undefined, server.name].filter(Boolean).join(" · ")}
              </span>
            </span>
          </button>
        ))}
        {copies.map((c) => (
          <Link
            key={c.server.id}
            to="/s/$serverId/item/$itemId"
            params={{ serverId: c.server.id, itemId: c.item.id }}
            className="flex items-center gap-3 rounded-[10px] px-2.5 py-2 hover:bg-white/10"
          >
            <Icon name="server" size={18} className="shrink-0 text-white/70" />
            <span className="min-w-0 flex-1 truncate type-callout">{t("item.onServer", { server: c.server.name })}</span>
            <Icon name="forward" size={14} className="text-white/50" />
          </Link>
        ))}
      </div>
    </section>
  );
}

/** Who made an item, and what its file holds. */
function Facts({ item, credits, source }: { item: Item; credits: Credit[]; source?: MediaSource }) {
  const names = (kind: CreditKind) =>
    credits
      .filter((c) => c.kind === kind && c.person)
      .map((c) => c.person!.name)
      .join(", ");
  const streams = (kind: StreamKind) => source?.streams.filter((s) => s.kind === kind).map(streamText) ?? [];
  const rows: [string, ReactNode][] = [
    [t("item.director"), names(CreditKind.DIRECTOR)],
    [t("item.writers"), names(CreditKind.WRITER)],
    [t("item.studio"), item.studios.join(", ")],
    [t("item.audioTracks"), streams(StreamKind.AUDIO).join("\n")],
    [t("item.subtitleTracks"), streams(StreamKind.SUBTITLE).join(", ")],
    [
      t("item.file"),
      source
        ? [source.path.split("/").pop(), source.sizeBytes > 0n ? sizeOf(source.sizeBytes) : undefined].filter(Boolean).join(" · ")
        : "",
    ],
  ];
  const shown = rows.filter(([, v]) => v);
  if (shown.length === 0) return null;
  return (
    <dl className="grid gap-x-8 gap-y-5 px-margin pt-6 pb-4" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(240px, 1fr))" }}>
      {shown.map(([k, v]) => (
        <div key={k} className="min-w-0">
          <dt className="type-footnote font-semibold text-text-secondary">{k}</dt>
          <dd className="mt-1 type-callout break-words whitespace-pre-line">{v}</dd>
        </div>
      ))}
    </dl>
  );
}

/** A series' seasons, a season's episodes, an album's tracks or a collection's items. */
function Children({ server, item, items }: { server: ServerEntry; item: Item; items: Item[] }) {
  if (items.length === 0) return null;
  if (item.kind === ItemKind.SEASON) {
    return (
      <section className="py-4">
        <h2 className="px-margin pb-2 type-title3">{t("item.episodes")}</h2>
        <div className="flex flex-col">
          {items.map((e) => (
            <EpisodeRow key={e.id} server={server} episode={e} />
          ))}
        </div>
      </section>
    );
  }
  if (item.kind === ItemKind.MUSIC_ALBUM) {
    return (
      <section className="px-margin py-4">
        <h2 className="pb-2 type-title3">{t("item.tracks")}</h2>
        <ol className="flex flex-col">
          {items.map((track) => (
            <li key={track.id}>
              <Link
                to="/s/$serverId/play/$itemId"
                params={{ serverId: server.id, itemId: track.id }}
                className="flex items-center gap-4 rounded-md px-2 py-2 hover:bg-surface-raised"
              >
                <span className="w-6 text-right type-callout text-text-tertiary">{track.indexNumber || ""}</span>
                <span className="flex-1 truncate type-body">{track.name}</span>
                <span className="type-footnote text-text-secondary">{runtimeOf(track)}</span>
              </Link>
            </li>
          ))}
        </ol>
      </section>
    );
  }
  if (item.kind === ItemKind.SERIES) {
    return (
      <Shelf title={t("item.seasons")}>
        {items.map((s) => (
          <ItemCard key={s.id} server={server} serverId={server.id} item={s} />
        ))}
      </Shelf>
    );
  }
  return (
    <section className="py-4">
      <Grid>
        {items.map((i) => (
          <ItemCard key={i.id} server={server} serverId={server.id} item={i} fluid />
        ))}
      </Grid>
    </section>
  );
}

function EpisodeRow({ server, episode: e }: { server: ServerEntry; episode: Item }) {
  const thumb = thumbOf(server, e, 320);
  const progress = progressOf(e);
  const aired = e.premiereDate ? timestampDate(e.premiereDate).toLocaleDateString() : undefined;
  return (
    <Link
      to="/s/$serverId/item/$itemId"
      params={{ serverId: server.id, itemId: e.id }}
      className="flex gap-4 px-margin py-3 hover:bg-surface"
    >
      <div className="relative w-[calc(var(--mavio-layout-thumbnail)/2)] shrink-0 md:w-thumbnail">
        <Artwork src={thumb?.url} alt={e.name} className="aspect-video rounded-md" />
        {progress !== undefined && (
          <div className="absolute inset-x-2 bottom-2 h-1 overflow-hidden rounded-full bg-white/35">
            <div className="h-full rounded-full bg-white" style={{ width: `${progress * 100}%` }} />
          </div>
        )}
        {e.userData?.played && (
          <span className="absolute top-1.5 right-1.5 rounded-full bg-scrim p-0.5">
            <Icon name="played" fill size={16} />
          </span>
        )}
      </div>
      <div className="min-w-0 flex-1">
        <div className="type-headline">
          {e.indexNumber > 0 && `${e.indexNumber}. `}
          {e.name}
        </div>
        <div className="type-footnote text-text-secondary">{[runtimeOf(e), aired].filter(Boolean).join(" · ")}</div>
        {e.overview && <p className="mt-1 line-clamp-2 type-callout text-text-secondary">{e.overview}</p>}
      </div>
    </Link>
  );
}

function Cast({ server, credits }: { server: ServerEntry; credits: Credit[] }) {
  if (credits.length === 0) return null;
  const isCast = (c: Credit) => c.kind === CreditKind.ACTOR || c.kind === CreditKind.GUEST_STAR;
  const sorted = [...credits].sort((a, b) => Number(!isCast(a)) - Number(!isCast(b)) || a.order - b.order);
  return (
    <Shelf title={t("item.cast")}>
      {sorted.map((c, i) => {
        const p = c.person;
        if (!p) return null;
        const photo = artwork(server, p, ImageKind.PRIMARY, 160);
        const initials = p.name
          .split(/\s+/)
          .map((w) => w[0])
          .slice(0, 2)
          .join("");
        return (
          <div key={`${p.id}-${i}`} className="w-28 shrink-0 snap-start text-center">
            <div className="relative mx-auto aspect-square w-24 overflow-hidden rounded-full bg-surface-raised">
              {photo ? (
                <img src={photo.url} alt="" loading="lazy" className="h-full w-full object-cover" />
              ) : (
                <span className="absolute inset-0 flex items-center justify-center type-title3 text-text-secondary">
                  {initials}
                </span>
              )}
            </div>
            <div className="mt-2 line-clamp-2 type-callout">{p.name}</div>
            {c.role && <div className="line-clamp-2 type-footnote text-text-secondary">{c.role}</div>}
          </div>
        );
      })}
    </Shelf>
  );
}

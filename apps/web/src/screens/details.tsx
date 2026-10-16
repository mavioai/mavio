// An item's details (libs/design/components.md, Details Screen): the header
// with what to watch and how, a series' episodes by season, an episode's
// season, the cast, items like it, and its details and media.
import { durationMs, timestampDate } from "@bufbuild/protobuf/wkt";
import {
  artwork,
  type Copy,
  type ItemValue,
  type ServerEntry,
  seriesNextUp,
  thumbOf,
  useChildren,
  useCollections,
  useDiscovery,
  useEpisodes,
  useExtras,
  useItem,
  useOtherCopies,
  useServer,
  useSignedIn,
  useSimilar,
  useUserData,
} from "@mavio/client-ts";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { CreditKind, type Credit, ExtraKind, ImageKind, type Item, ItemKind, SeriesStatus } from "@mavio/proto/mavio/library/v1/item_pb";
import { CatalogKind } from "@mavio/proto/mavio/plugin/v1/discovery_pb";
import { type MediaSource, type MediaStream, StreamKind, VideoRange } from "@mavio/proto/mavio/library/v1/media_pb";
import { getRouteApi, Link } from "@tanstack/react-router";
import { Fragment, type ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";

import type { MessageKey } from "@mavio/design/strings";
import { discoverLanguage, locale, t } from "../i18n.ts";
import { Acquisition } from "./acquire.tsx";
import { CatalogShelf } from "./discover.tsx";
import { ItemIssues } from "./issues.tsx";
import { ItemSubtitles } from "./item-subtitles.tsx";
import { CollectionMembers, MoreMenu, PlaylistEntries } from "./lists.tsx";
import { initialsOf } from "./person.tsx";
import { Icon } from "../icon.tsx";
import { usePrefs, useWidthClass } from "../prefs.ts";
import { Artwork, Grid, ItemCard, Loading, minutesLeft, progressOf, runtimeOf, Shelf, StatusView, yearOf } from "../ui.tsx";
import { Dismiss, Sheet } from "../controls/dialog.tsx";
import { Tabs, ToggleButton } from "../controls/inputs.tsx";
import { Tip } from "../controls/tooltip.tsx";
import { Menu, MenuChoice, MenuLabel, MenuLink, MenuOption } from "../controls/menu.tsx";

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
    <ItemDetails key={itemId} server={server} item={q.data.item} sources={q.data.mediaSources} credits={q.data.credits} />
  );
}

/** The resolution of a source's first video stream, as "4K" or "1080p". */
function resolutionOf(source?: MediaSource): string | undefined {
  const v = videoOf(source);
  if (!v?.height) return undefined;
  if (v.width >= 3200 || v.height >= 2000) return "4K";
  if (v.width >= 1800 || v.height >= 1000) return "1080p";
  if (v.width >= 1200 || v.height >= 700) return "720p";
  return `${v.height}p`;
}

function videoOf(source?: MediaSource): MediaStream | undefined {
  return source?.streams.find((s) => s.kind === StreamKind.VIDEO);
}

function sizeOf(bytes: bigint): string {
  const n = Number(bytes);
  return n >= 1e9 ? `${(n / 1e9).toFixed(1)} GB` : `${Math.round(n / 1e6)} MB`;
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
  const v = videoOf(source);
  return v ? rangeNames[v.range] : undefined;
}

/** The most channels of a source's audio, as "5.1" or "7.1", when it is surround. */
function surroundOf(source?: MediaSource): string | undefined {
  const most = Math.max(0, ...(source?.streams.filter((s) => s.kind === StreamKind.AUDIO).map((s) => s.channels) ?? []));
  return most >= 8 ? "7.1" : most >= 6 ? "5.1" : undefined;
}

/** A file's container, by its extension, else the first format the probe named. */
function containerOf(s: MediaSource): string {
  const ext = /\.([a-z0-9]+)$/i.exec(s.path)?.[1];
  return (ext ?? s.container.split(",")[0]).toUpperCase();
}

/** A version's name: its own, else its resolution and range, else its container. */
function versionName(s: MediaSource): string {
  return s.name || [resolutionOf(s), rangeOf(s)].filter(Boolean).join(" ") || containerOf(s);
}

const languageNames = new Intl.DisplayNames([locale], { type: "language", fallback: "none" });
const dateFormat = new Intl.DateTimeFormat(locale, { dateStyle: "medium" });
const timeFormat = new Intl.DateTimeFormat(locale, { hour: "numeric", minute: "2-digit" });

/** A stream's language or title, codec and channels, as the details list them. */
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

/** An episode's short numbers, as "S2 E3". */
function episodeShort(e: Item): string {
  return t("item.episodeShort", { season: e.parentIndexNumber, episode: e.indexNumber });
}

/** The years a series ran, as "2023–" while it continues or "2019–2022". */
function yearsOf(item: Item): string | undefined {
  const from = yearOf(item);
  if (!from) return undefined;
  const to = item.endDate ? timestampDate(item.endDate).getFullYear() : undefined;
  if (to && to !== from) return `${from}–${to}`;
  return item.seriesStatus === SeriesStatus.CONTINUING ? `${from}–` : String(from);
}

/** The time of day an item would end if played now, from its position. */
function endsAt(item: Item): string | undefined {
  if (!item.runtime) return undefined;
  const left = durationMs(item.runtime) - (item.userData?.position && !item.userData.played ? durationMs(item.userData.position) : 0);
  return left > 0 ? t("item.endsAt", { time: timeFormat.format(Date.now() + left) }) : undefined;
}

function ItemDetails({ server, item, sources, credits }: { server: ServerEntry; item: Item; sources: MediaSource[]; credits: Credit[] }) {
  const several = useSignedIn().length > 1;
  const inSeries = item.kind === ItemKind.EPISODE || item.kind === ItemKind.SEASON;
  // An episode or season wears its series' artwork where it has none.
  const series = useItem(inSeries && item.seriesId ? server : undefined, item.seriesId).data?.item;
  const children = useChildren(server, item);
  const episodes = useEpisodes(server, item.kind === ItemKind.SERIES ? item.id : inSeries ? item.seriesId : undefined).data;
  const similar = useSimilar(server, item.id);
  // Theme songs and videos play behind the screen elsewhere; the other extras are watched.
  const extras = (useExtras(server, item.id).data ?? []).filter((e) => e.extra !== ExtraKind.THEME_SONG && e.extra !== ExtraKind.THEME_VIDEO);
  const trailer = extras.find((e) => e.extra === ExtraKind.TRAILER);
  const copies = useOtherCopies(server.id, item);
  const [source, setSource] = useState(sources[0]?.id);
  const width = useWidthClass();
  const { tv } = usePrefs();
  const phone = width === "compact" && !tv;

  const selected = sources.find((s) => s.id === source);
  const seasonEpisodes = (id: string) => episodes?.filter((e) => e.seasonId === id) ?? [];
  // What the primary action plays: the item itself, or a series' or
  // season's episode to watch next.
  const next =
    item.kind === ItemKind.SERIES
      ? seriesNextUp(episodes ?? [])
      : item.kind === ItemKind.SEASON
        ? seriesNextUp(seasonEpisodes(item.id))
        : sources.length > 0
          ? item
          : undefined;

  const backdrop = artwork(server, item, ImageKind.BACKDROP, tv ? 1920 : 1600) ?? (series && artwork(server, series, ImageKind.BACKDROP, tv ? 1920 : 1600));
  const still = item.kind === ItemKind.EPISODE ? thumbOf(server, item, 1280) : undefined;
  const poster = artwork(server, item, ImageKind.PRIMARY, 400);
  const logo = inSeries ? undefined : artwork(server, item, ImageKind.LOGO, 800);
  const seriesLogo = series && artwork(server, series, ImageKind.LOGO, 400);

  const facts: ReactNode[] = [
    item.kind === ItemKind.SERIES ? yearsOf(item) : item.kind === ItemKind.EPISODE && item.premiereDate ? dateFormat.format(timestampDate(item.premiereDate)) : yearOf(item),
    item.kind === ItemKind.SERIES && children.data?.length ? t("item.seasonCount", { count: children.data.length }) : undefined,
    item.kind === ItemKind.SEASON && children.data?.length ? t("item.episodeCount", { count: children.data.length }) : undefined,
    item.kind !== ItemKind.SERIES ? runtimeOf(item) : undefined,
    item.kind === ItemKind.EPISODE ? undefined : item.genres.slice(0, 3).join(" / ") || undefined,
  ].filter(Boolean);
  const ratings: ReactNode[] = [
    item.communityRating > 0 && (
      <span key="c" className="inline-flex items-center gap-1">
        <Icon name="rating" fill size={14} className="text-[#FFD60A]" />
        {item.communityRating.toFixed(1)}
      </span>
    ),
    item.criticRating > 0 && <span key="r">{t("item.critics", { score: Math.round(item.criticRating) })}</span>,
  ].filter(Boolean);
  const subtitles = selected?.streams.some((s) => s.kind === StreamKind.SUBTITLE);
  const badges = [item.officialRating, resolutionOf(selected), rangeOf(selected), surroundOf(selected), subtitles ? "CC" : undefined].filter(
    (b): b is string => !!b,
  );

  const header = (
    <Header
      server={server}
      item={item}
      series={series}
      seriesLogo={seriesLogo?.url}
      logo={logo?.url}
      image={(backdrop ?? still)?.url}
      poster={backdrop || still ? undefined : poster?.url}
      facts={facts}
      ratings={ratings}
      badges={badges}
      serverName={several ? server.name : undefined}
      phone={phone}
      tv={tv}
    >
      {!phone && (
        <Actions
          server={server}
          item={item}
          next={next}
          sources={sources}
          source={source}
          onSource={setSource}
          copies={copies}
          trailer={trailer}
          tv={tv}
          phone={false}
        />
      )}
    </Header>
  );

  return (
    <div className="pb-10">
      {header}
      {phone && (
        <div className="flex flex-col gap-4 px-margin pt-4">
          <Actions
            server={server}
            item={item}
            next={next}
            sources={sources}
            source={source}
            onSource={setSource}
            copies={copies}
            trailer={trailer}
            tv={false}
            phone
          />
          {item.overview && <Overview item={item} inline />}
        </div>
      )}
      <div className="pt-6">
        {item.kind === ItemKind.SERIES && (
          <Seasons server={server} seasons={children.data ?? []} episodesOf={seasonEpisodes} current={next} tv={tv} />
        )}
        {item.kind === ItemKind.SEASON && <EpisodeList server={server} items={children.data ?? []} current={next?.id} />}
        {item.kind === ItemKind.EPISODE && item.seasonId && (
          <SeasonShelf server={server} episode={item} items={seasonEpisodes(item.seasonId)} />
        )}
        {item.kind === ItemKind.PLAYLIST && <PlaylistEntries server={server} playlist={item} />}
        {item.kind === ItemKind.COLLECTION && children.isSuccess && <CollectionMembers server={server} collection={item} items={children.data} />}
        {item.kind !== ItemKind.SERIES && item.kind !== ItemKind.SEASON && item.kind !== ItemKind.PLAYLIST && item.kind !== ItemKind.COLLECTION && (
          <Children server={server} item={item} items={children.data ?? []} />
        )}
        <Cast server={server} credits={credits} />
        <Extras server={server} items={extras} fallback={artwork(server, item, ImageKind.BACKDROP, 640)?.url} />
        {item.collectionName && <CollectionBanner server={server} item={item} fallback={backdrop?.url} />}
        {(similar.data?.length ?? 0) > 0 && (
          <Shelf title={t("item.similar")}>
            {similar.data!.map((i) => (
              <ItemCard key={i.id} server={server} serverId={server.id} item={i} />
            ))}
          </Shelf>
        )}
        <Recommendations item={item} />
        <About server={server} item={item} series={series} credits={credits} source={selected} overview={!phone} />
        <Acquisition server={server} item={item} />
        <ItemSubtitles server={server} item={item} />
        <ItemIssues server={server} item={item} />
      </div>
    </div>
  );
}

/**
 * The top of the screen: the artwork, and over its lower left the series
 * an episode or season belongs to, the logo or title, the facts and the
 * actions.
 */
function Header({
  server,
  item,
  series,
  seriesLogo,
  logo,
  image,
  poster,
  facts,
  ratings,
  badges,
  serverName,
  phone,
  tv,
  children,
}: {
  server: ServerEntry;
  item: Item;
  series?: Item;
  seriesLogo?: string;
  logo?: string;
  image?: string;
  poster?: string;
  facts: ReactNode[];
  ratings: ReactNode[];
  badges: string[];
  serverName?: string;
  phone: boolean;
  tv: boolean;
  children: ReactNode;
}) {
  const title = `font-bold leading-[1.05] text-balance ${tv ? "text-[72px]" : phone ? "text-[34px]" : "text-[52px]"}`;
  return (
    <header
      className={`relative overflow-hidden bg-surface ${
        tv ? "h-[86vh]" : phone ? "h-[64vh] min-h-96" : "mx-2.5 mt-2.5 h-[clamp(520px,70vh,700px)] rounded-[18px]"
      }`}
    >
      {image ? (
        <img src={image} alt="" className="absolute inset-0 h-full w-full animate-[fade_600ms_ease-out] object-cover" />
      ) : (
        // Without a backdrop the poster, blurred, colors the header.
        poster && <img src={poster} alt="" className="absolute inset-0 h-full w-full scale-125 object-cover opacity-70 blur-3xl" />
      )}
      <div
        className={`absolute inset-0 ${
          phone
            ? "bg-gradient-to-t from-black via-black/50 to-transparent"
            : "bg-[linear-gradient(90deg,rgb(0_0_0/0.82)_0%,rgb(0_0_0/0.45)_45%,transparent_75%),linear-gradient(0deg,rgb(0_0_0/0.75)_0%,transparent_55%)]"
        }`}
      />
      <div className={`absolute inset-x-0 bottom-0 flex items-end gap-10 ${phone ? "px-margin pb-6" : tv ? "px-margin pb-[10vh]" : "p-10"}`}>
        {poster && !phone && (
          <img src={poster} alt="" className={`shrink-0 rounded-lg shadow-level3 ${tv ? "w-[300px]" : "w-[200px]"}`} />
        )}
        <div className={`flex min-w-0 flex-col gap-3 text-white ${tv ? "max-w-[960px]" : "max-w-[640px]"}`}>
          {series && (
            <Link
              to="/s/$serverId/item/$itemId"
              params={{ serverId: server.id, itemId: series.id }}
              className="w-fit type-headline text-white/85 hover:text-white"
            >
              {seriesLogo ? (
                <img src={seriesLogo} alt={series.name} className={`object-contain object-left ${tv ? "max-h-20" : "max-h-12"}`} />
              ) : (
                item.seriesName
              )}
            </Link>
          )}
          {item.kind === ItemKind.EPISODE && item.indexNumber > 0 && (
            <div className="type-headline text-white/85">
              {t("item.episodeNumber", { season: item.parentIndexNumber, episode: item.indexNumber })}
            </div>
          )}
          {logo ? (
            <img src={logo} alt={item.name} className={`max-w-[80%] object-contain object-left ${tv ? "max-h-40" : "max-h-28"}`} />
          ) : (
            <h1 className={title}>{item.name}</h1>
          )}
          {/* The facts, then the ratings and badges, on a row of their own. */}
          {facts.length > 0 && (
            // Each separator keeps to the fact before it, so that a line ends with one rather than starts with it.
            <p className="type-callout text-white/80">
              {facts.map((f, i) => (
                <Fragment key={i}>
                  <span className="whitespace-nowrap">
                    {f}
                    {i < facts.length - 1 && <span aria-hidden>{"\u00a0·"}</span>}
                  </span>{" "}
                </Fragment>
              ))}
            </p>
          )}
          {ratings.length + badges.length > 0 || serverName ? (
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 type-callout text-white/80">
              {ratings}
              {badges.map((b) => (
                <span key={b} className="rounded-[4px] border border-white/50 px-1.5 type-caption font-semibold text-white/90">
                  {b}
                </span>
              ))}
              {serverName && <span className="rounded-[4px] bg-white/20 px-1.5 type-caption">{serverName}</span>}
            </div>
          ) : null}
          {/* The television keeps to two lines of overview, under the logo, with room for the actions. */}
          {!phone && !tv && item.tagline && <p className="type-headline text-white/90">{item.tagline}</p>}
          {!phone && item.overview && <Overview item={item} lines={tv ? 2 : 3} />}
          {children}
        </div>
      </div>
    </header>
  );
}

/**
 * The overview, three lines long (two on the television) with More when it is
 * longer: More opens it whole in a dialog, or inline on a phone.
 */
function Overview({ item, inline = false, lines = 3 }: { item: Item; inline?: boolean; lines?: number }) {
  const ref = useRef<HTMLParagraphElement>(null);
  const [long, setLong] = useState(false);
  const [open, setOpen] = useState(false);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = () => setLong(el.scrollHeight > el.clientHeight + 1);
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  const expanded = inline && open;
  return (
    <div className={inline ? "" : "text-white/85"}>
      <p ref={ref} className={`type-body ${expanded ? "" : lines === 2 ? "line-clamp-2" : "line-clamp-3"}`}>
        {item.overview}
      </p>
      {(long || expanded) && (
        <button
          type="button"
          onClick={() => setOpen(!open)}
          className={`mt-1 type-callout font-semibold ${inline ? "text-accent-text" : "text-white hover:text-white/80"}`}
        >
          {expanded ? t("action.showLess") : t("item.more")}
        </button>
      )}
      {!inline && open && <OverviewDialog item={item} onClose={() => setOpen(false)} />}
    </div>
  );
}

/** The whole overview and tagline, over the screen. */
function OverviewDialog({ item, onClose }: { item: Item; onClose: () => void }) {
  return (
    <Sheet label={item.name} width="medium" className="overflow-y-auto" onClose={onClose}>
      <div className="flex flex-col gap-3 p-7">
        <div className="flex items-start gap-4">
          <h2 className="min-w-0 flex-1 type-title2">{item.name}</h2>
          <Dismiss />
        </div>
        {item.tagline && <p className="type-headline text-text-secondary">{item.tagline}</p>}
        <p className="type-body whitespace-pre-line">{item.overview}</p>
      </div>
    </Sheet>
  );
}

/**
 * Play or Resume, of the item or of the episode to watch next, with the
 * episode's name and progress; from the beginning, favorite, played, and
 * the version to play.
 */
function Actions({
  server,
  item,
  next,
  sources,
  source,
  onSource,
  copies,
  trailer,
  tv,
  phone,
}: {
  server: ServerEntry;
  item: Item;
  next?: Item;
  sources: MediaSource[];
  source?: string;
  onSource: (id: string) => void;
  copies: Copy[];
  trailer?: Item;
  tv: boolean;
  phone: boolean;
}) {
  const userData = useUserData(server);
  // On the television focus starts on Play, as on tvOS.
  const play = useRef<HTMLAnchorElement>(null);
  const hasNext = !!next;
  useEffect(() => {
    if (tv && hasNext && document.activeElement === document.body) play.current?.focus({ preventScroll: true });
  }, [tv, hasNext, item.id]);
  const played = !!item.userData?.played;
  const favorite = !!item.userData?.favorite;
  const own = next?.id === item.id;
  const progress = next ? progressOf(next) : undefined;
  const resumable = progress !== undefined;
  const position = next?.userData?.position ? durationMs(next.userData.position) : 0;
  const episode = next && !own ? episodeShort(next) : undefined;

  const icon = tv ? 28 : 20;
  // Over the artwork the buttons are white; on a phone they are below it, on the page.
  const circle = `inline-flex shrink-0 items-center justify-center rounded-full ${tv ? "size-16" : "size-11"} ${
    phone ? "bg-fill text-on-background hover:bg-fill-selected" : "bg-white/20 text-white backdrop-blur-xl hover:bg-white/30"
  }`;
  const primary = `inline-flex items-center justify-center gap-2 rounded-[10px] font-semibold type-headline ${
    tv ? "h-16 px-10" : phone ? "h-12 flex-1 px-5" : "h-11 px-6"
  } ${
    // On a television white is focus's, so the first button is glass like
    // the others until it has focus.
    phone ? "bg-button text-on-button hover:opacity-90" : tv ? "bg-white/20 text-white backdrop-blur-xl" : "bg-white text-black hover:bg-white/90"
  }`;
  const label = episode
    ? t(resumable ? "item.resumeEpisode" : "item.playEpisode", { episode })
    : t(resumable ? "item.resume" : "item.play");
  const status = next && (
    <div className={`flex flex-wrap items-center gap-x-3 gap-y-1 type-footnote ${phone ? "text-text-secondary" : "text-white/80"}`}>
      {resumable && (
        <div className={`h-1 w-40 overflow-hidden rounded-full ${phone ? "bg-fill-selected" : "bg-white/35"}`}>
          <div className={`h-full rounded-full ${phone ? "bg-progress" : "bg-white"}`} style={{ width: `${progress * 100}%` }} />
        </div>
      )}
      {[episode && next.name, resumable && t("item.timeLeft", { time: minutesText(minutesLeft(next) ?? 0) }), endsAt(next)]
        .filter(Boolean)
        .join(" · ")}
    </div>
  );
  return (
    <div className="mt-1 flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-3">
        {next && (
          <Link
            to="/s/$serverId/play/$itemId"
            params={{ serverId: server.id, itemId: next.id }}
            search={{ start: resumable ? position : undefined, source: own ? source : undefined }}
            ref={play}
            data-tv-button
            className={primary}
          >
            <Icon name="play" fill size={icon} />
            {label}
          </Link>
        )}
        {resumable && next && (
          <Tip label={t("item.fromStart")}>
            <Link
              to="/s/$serverId/play/$itemId"
              params={{ serverId: server.id, itemId: next.id }}
              search={{ source: own ? source : undefined }}
              aria-label={t("item.fromStart")}
              data-tv-button
              className={circle}
            >
              <Icon name="previous" size={icon} />
            </Link>
          </Tip>
        )}
        {trailer && (
          <Tip label={t("item.trailer")}>
            <Link
              to="/s/$serverId/play/$itemId"
              params={{ serverId: server.id, itemId: trailer.id }}
              aria-label={t("item.trailer")}
              data-tv-button
              className={circle}
            >
              <Icon name="movies" size={icon} />
            </Link>
          </Tip>
        )}
        <Tip label={t("item.favorite")}>
          <ToggleButton on={favorite} label={t("item.favorite")} onChange={(on) => userData.mutate({ itemId: item.id, favorite: on })} className={circle}>
            <Icon name="favorite" fill={favorite} size={icon} className={favorite ? "text-[#FF453A]" : undefined} />
          </ToggleButton>
        </Tip>
        <Tip label={played ? t("item.markUnplayed") : t("item.markPlayed")}>
          <ToggleButton
            on={played}
            label={played ? t("item.markUnplayed") : t("item.markPlayed")}
            onChange={(on) => userData.mutate({ itemId: item.id, played: on })}
            className={circle}
          >
            <Icon name="played" fill={played} size={icon} />
          </ToggleButton>
        </Tip>
        {(sources.length > 1 || copies.length > 0) && (
          <VersionMenu server={server} sources={sources} selected={source} onSelect={onSource} copies={copies} tv={tv} phone={phone} />
        )}
        <MoreMenu server={server} item={item} className={circle} iconSize={icon} phone={phone} />
      </div>
      {status}
    </div>
  );
}

/** The version to play, and the same title on the user's other servers. */
function VersionMenu({
  server,
  sources,
  selected,
  onSelect,
  copies,
  tv,
  phone,
}: {
  server: ServerEntry;
  sources: MediaSource[];
  selected?: string;
  onSelect: (id: string) => void;
  copies: Copy[];
  tv: boolean;
  phone: boolean;
}) {
  const current = sources.find((s) => s.id === selected);
  return (
    <Menu
      side="top"
      align={phone ? "start" : "end"}
      width={tv ? "w-[440px]" : "w-[320px]"}
      trigger={
        <button
          type="button"
          className={`inline-flex items-center gap-2 rounded-full px-4 font-semibold backdrop-blur-xl ${tv ? "h-16 px-6 type-headline" : "h-11 type-callout"} ${
            phone ? "bg-fill hover:bg-fill-selected data-popup-open:bg-fill-selected" : "bg-white/20 text-white hover:bg-white/30 data-popup-open:bg-white/30"
          }`}
        >
          <span className={phone ? "text-text-secondary" : "text-white/70"}>{t("item.version")}</span>
          {current ? versionName(current) : server.name}
          <Icon name="expand" size={14} />
        </button>
      }
    >
      <MenuLabel>{t("item.versions")}</MenuLabel>
      <MenuChoice value={selected ?? ""} onChange={onSelect}>
        {sources.map((s) => (
          <MenuOption key={s.id} value={s.id}>
            <span className="block truncate type-callout font-semibold">{versionName(s)}</span>
            <span className="block truncate type-caption text-text-secondary">
              {[resolutionOf(s), rangeOf(s), surroundOf(s), containerOf(s), s.sizeBytes > 0n ? sizeOf(s.sizeBytes) : undefined].filter(Boolean).join(" · ")}
            </span>
          </MenuOption>
        ))}
      </MenuChoice>
      {copies.map((c) => (
        <MenuLink
          key={c.server.id}
          icon="server"
          to="/s/$serverId/item/$itemId"
          params={{ serverId: c.server.id, itemId: c.item.id }}
          end={<Icon name="forward" size={14} className="text-text-tertiary" />}
        >
          <span className="block truncate type-callout">{t("item.onServer", { server: c.server.name })}</span>
        </MenuLink>
      ))}
    </Menu>
  );
}

/** A series' episodes by season: the seasons to choose from, and the chosen one's episodes in a row. */
function Seasons({
  server,
  seasons,
  episodesOf,
  current,
  tv,
}: {
  server: ServerEntry;
  seasons: Item[];
  episodesOf: (seasonId: string) => Item[];
  current?: Item;
  tv: boolean;
}) {
  // The season of the episode to watch next comes first.
  const [chosen, setChosen] = useState<string>();
  const season = seasons.find((s) => s.id === chosen) ?? seasons.find((s) => s.id === current?.seasonId) ?? seasons[0];
  if (!season) return null;
  const items = episodesOf(season.id);
  return (
    <section className="py-3">
      <div className="mb-3 flex items-center gap-4 px-margin">
        <h2 className="type-title3">{t("item.episodes")}</h2>
        {seasons.length > 1 && (
          <Tabs
            value={season.id}
            label={t("item.seasons")}
            look="pills"
            className="scroll-row -my-1 min-w-0 overflow-x-auto py-1"
            tabs={seasons.map((s) => ({ value: s.id, label: s.name }))}
            onChange={setChosen}
          />
        )}
        {seasons.length === 1 && <span className="type-callout text-text-secondary">{season.name}</span>}
        <Link
          to="/s/$serverId/item/$itemId"
          params={{ serverId: server.id, itemId: season.id }}
          className="ml-auto shrink-0 type-callout text-accent-text hover:opacity-80"
        >
          {t("home.more")}
        </Link>
      </div>
      <EpisodeRow server={server} items={items} current={current?.id} />
    </section>
  );
}

/** An episode's season, the episode marked where it is. */
function SeasonShelf({ server, episode, items }: { server: ServerEntry; episode: Item; items: Item[] }) {
  if (items.length < 2) return null;
  return (
    <section className="py-3">
      <h2 className="mb-3 px-margin type-title3">{t("item.moreFrom", { season: episode.seasonName || `${episode.parentIndexNumber}` })}</h2>
      <EpisodeRow server={server} items={items} current={episode.id} scrollTo={episode.id} />
    </section>
  );
}

/** Episodes side by side, each with its still, number, name, length and overview. */
function EpisodeRow({ server, items, current, scrollTo }: { server: ServerEntry; items: Item[]; current?: string; scrollTo?: string }) {
  const row = useRef<HTMLDivElement>(null);
  // The row starts at the episode shown, as a season's list would.
  useEffect(() => {
    const el = scrollTo && row.current?.querySelector<HTMLElement>(`[data-episode="${scrollTo}"]`);
    if (el && row.current) row.current.scrollLeft = el.offsetLeft - row.current.offsetLeft - parseFloat(getComputedStyle(row.current).paddingLeft);
  }, [scrollTo, items.length]);
  return (
    <div ref={row} className="scroll-row -mt-1 flex gap-x-[calc(var(--mavio-layout-gutter)*1.5)] overflow-x-auto px-margin pt-1 pb-1 [[data-tv]_&]:gap-gutter">
      {items.map((e) => (
        <EpisodeCard key={e.id} server={server} episode={e} current={e.id === current} />
      ))}
    </div>
  );
}

function EpisodeCard({ server, episode: e, current }: { server: ServerEntry; episode: Item; current: boolean }) {
  const thumb = thumbOf(server, e, 640);
  const progress = progressOf(e);
  const aired = e.premiereDate ? dateFormat.format(timestampDate(e.premiereDate)) : undefined;
  return (
    <Link
      to="/s/$serverId/item/$itemId"
      params={{ serverId: server.id, itemId: e.id }}
      data-card
      data-episode={e.id}
      aria-current={current || undefined}
      className="group block w-thumbnail shrink-0 snap-start"
    >
      <div className="relative">
        <Artwork src={thumb?.url} alt={e.name} className="aspect-video rounded-md" />
        {progress !== undefined && (
          <div className="absolute inset-x-2.5 bottom-2.5 h-1 overflow-hidden rounded-full bg-white/35 shadow-level1">
            <div className="h-full rounded-full bg-white" style={{ width: `${progress * 100}%` }} />
          </div>
        )}
        {e.userData?.played && (
          <span className="absolute top-1.5 right-1.5 rounded-full bg-scrim p-0.5 text-white">
            <Icon name="played" fill size={18} />
          </span>
        )}
      </div>
      <div className="mt-2 flex items-baseline gap-2">
        <span className={`type-footnote font-semibold ${current ? "text-accent-text" : "text-text-secondary"}`}>
          {e.indexNumber > 0 ? episodeShort(e) : ""}
        </span>
        <span className="truncate type-footnote text-text-tertiary">{[runtimeOf(e), aired].filter(Boolean).join(" · ")}</span>
      </div>
      <div className="truncate type-headline">{e.name}</div>
      {e.overview && <p className="mt-0.5 line-clamp-3 type-footnote text-text-secondary">{e.overview}</p>}
    </Link>
  );
}

/** A season's episodes, one under another. */
function EpisodeList({ server, items, current }: { server: ServerEntry; items: Item[]; current?: string }) {
  if (items.length === 0) return null;
  return (
    <section className="py-3">
      <h2 className="px-margin pb-2 type-title3">{t("item.episodes")}</h2>
      <div className="flex flex-col">
        {items.map((e) => {
          const thumb = thumbOf(server, e, 640);
          const progress = progressOf(e);
          const aired = e.premiereDate ? dateFormat.format(timestampDate(e.premiereDate)) : undefined;
          return (
            <Link
              key={e.id}
              to="/s/$serverId/item/$itemId"
              params={{ serverId: server.id, itemId: e.id }}
              data-card
              aria-current={e.id === current || undefined}
              className="group flex gap-5 px-margin py-3 hover:bg-surface"
            >
              <div className="relative w-[clamp(140px,32%,var(--mavio-layout-thumbnail))] shrink-0">
                <Artwork src={thumb?.url} alt={e.name} className="aspect-video rounded-md" />
                {progress !== undefined && (
                  <div className="absolute inset-x-2 bottom-2 h-1 overflow-hidden rounded-full bg-white/35">
                    <div className="h-full rounded-full bg-white" style={{ width: `${progress * 100}%` }} />
                  </div>
                )}
                {e.userData?.played && (
                  <span className="absolute top-1.5 right-1.5 rounded-full bg-scrim p-0.5 text-white">
                    <Icon name="played" fill size={16} />
                  </span>
                )}
              </div>
              <div className="min-w-0 flex-1 py-1">
                <div className={`type-footnote font-semibold ${e.id === current ? "text-accent-text" : "text-text-secondary"}`}>
                  {e.indexNumber > 0 ? episodeShort(e) : ""}
                </div>
                <div className="type-headline">{e.name}</div>
                <div className="type-footnote text-text-tertiary">{[runtimeOf(e), aired].filter(Boolean).join(" · ")}</div>
                {e.overview && <p className="mt-1.5 line-clamp-3 max-w-[760px] type-callout text-text-secondary">{e.overview}</p>}
              </div>
            </Link>
          );
        })}
      </div>
    </section>
  );
}

/** An album's tracks or a folder's items. */
function Children({ server, item, items }: { server: ServerEntry; item: Item; items: Item[] }) {
  if (items.length === 0) return null;
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

const crewLabels: Partial<Record<CreditKind, () => string>> = {
  [CreditKind.CREATOR]: () => t("item.creators"),
  [CreditKind.DIRECTOR]: () => t("item.director"),
  [CreditKind.WRITER]: () => t("item.writers"),
};

/** The cast in billing order, then the creators, directors and writers. */
function Cast({ server, credits }: { server: ServerEntry; credits: Credit[] }) {
  const isCast = (c: Credit) => c.kind === CreditKind.ACTOR || c.kind === CreditKind.GUEST_STAR;
  const shown = credits.filter((c) => c.person && (isCast(c) || crewLabels[c.kind]));
  if (shown.length === 0) return null;
  const sorted = shown.toSorted((a, b) => Number(!isCast(a)) - Number(!isCast(b)) || a.order - b.order);
  return (
    <Shelf title={t("item.cast")}>
      {sorted.map((c, i) => {
        const p = c.person!;
        const photo = artwork(server, p, ImageKind.PRIMARY, 240);
        return (
          <Link
            key={`${p.id}-${i}`}
            to="/s/$serverId/person/$personId"
            params={{ serverId: server.id, personId: p.id }}
            className="group w-28 shrink-0 snap-start rounded-lg text-center [[data-tv]_&]:w-40"
          >
            <div className="relative mx-auto aspect-square w-24 overflow-hidden rounded-full bg-fill [[data-tv]_&]:w-36">
              {photo ? (
                <img src={photo.url} alt="" loading="lazy" className="h-full w-full object-cover" />
              ) : (
                <span className="absolute inset-0 flex items-center justify-center type-title3 text-text-secondary">{initialsOf(p.name)}</span>
              )}
            </div>
            <div className="mt-2 line-clamp-2 type-callout group-hover:underline">{p.name}</div>
            <div className="line-clamp-2 type-footnote text-text-secondary">{isCast(c) ? c.role : crewLabels[c.kind]!()}</div>
          </Link>
        );
      })}
    </Shelf>
  );
}

const extraLabels: Partial<Record<ExtraKind, MessageKey>> = {
  [ExtraKind.TRAILER]: "extra.trailer",
  [ExtraKind.CLIP]: "extra.clip",
  [ExtraKind.BEHIND_THE_SCENES]: "extra.behindTheScenes",
  [ExtraKind.DELETED_SCENE]: "extra.deletedScene",
  [ExtraKind.INTERVIEW]: "extra.interview",
  [ExtraKind.SCENE]: "extra.scene",
  [ExtraKind.SAMPLE]: "extra.sample",
  [ExtraKind.FEATURETTE]: "extra.featurette",
  [ExtraKind.SHORT]: "extra.short",
};

/** An item's trailers, featurettes and other extras, each played by itself; those without a thumbnail wear the item's backdrop. */
function Extras({ server, items, fallback }: { server: ServerEntry; items: Item[]; fallback?: string }) {
  if (items.length === 0) return null;
  return (
    <Shelf title={t("item.extras")}>
      {items.map((e) => (
        <Link
          key={e.id}
          to="/s/$serverId/play/$itemId"
          params={{ serverId: server.id, itemId: e.id }}
          data-card
          className="group block w-thumbnail shrink-0 snap-start"
        >
          <div className="relative">
            <Artwork src={thumbOf(server, e, 640)?.url ?? fallback} alt={e.name} className="aspect-video rounded-md" />
            <span className="absolute bottom-2 left-2 rounded-full bg-scrim p-1.5 text-white">
              <Icon name="play" fill size={14} />
            </span>
          </div>
          <div className="mt-2 truncate type-headline">{e.name}</div>
          <div className="truncate type-footnote text-text-secondary">
            {[t(extraLabels[e.extra] ?? "extra.other"), runtimeOf(e)].filter(Boolean).join(" · ")}
          </div>
        </Link>
      ))}
    </Shelf>
  );
}

/**
 * The collection a movie belongs to, as the providers name it, over its
 * artwork, else the movie's.
 */
function CollectionBanner({ server, item, fallback }: { server: ServerEntry; item: Item; fallback?: string }) {
  const collections = useCollections(server);
  const id = item.externalIds.tmdb_collection;
  const name = item.collectionName.toLowerCase();
  const c = collections.data?.find((c) => (id ? c.externalIds.tmdb_collection === id : c.name.toLowerCase() === name));
  if (!c) return null;
  const image = artwork(server, c, ImageKind.BACKDROP, 1280)?.url ?? fallback;
  return (
    <section className="px-margin py-3">
      <Link
        to="/s/$serverId/item/$itemId"
        params={{ serverId: server.id, itemId: c.id }}
        data-card
        className="group relative flex min-h-28 items-center gap-4 overflow-hidden rounded-[14px] bg-surface-raised px-6 py-5 text-white [[data-tv]_&]:min-h-40 [[data-tv]_&]:px-10"
      >
        {image && <img src={image} alt="" loading="lazy" className="absolute inset-0 h-full w-full object-cover transition-transform duration-500 group-hover:scale-105" />}
        <div className="absolute inset-0 bg-[linear-gradient(90deg,rgb(0_0_0/0.8)_0%,rgb(0_0_0/0.55)_60%,rgb(0_0_0/0.3)_100%)]" />
        <div className="relative min-w-0 flex-1">
          <div className="type-footnote font-semibold text-white/70">{t("item.collection")}</div>
          <div className="truncate type-title3 [[data-tv]_&]:type-title1">{c.name}</div>
        </div>
        <span className="relative inline-flex h-9 shrink-0 items-center gap-1 rounded-full bg-white/20 px-4 type-callout font-semibold backdrop-blur-xl group-hover:bg-white/30">
          {t("item.viewCollection")}
          <Icon name="forward" size={14} />
        </span>
      </Link>
    </section>
  );
}

/**
 * The titles the discovery providers recommend to those who liked a movie
 * or series, those in a library opening their details there.
 */
function Recommendations({ item }: { item: Item }) {
  const { server, catalogs } = useDiscovery(discoverLanguage);
  const tmdb = Number(item.externalIds.tmdb);
  const kind = item.kind === ItemKind.MOVIE ? MediaKind.MOVIE : item.kind === ItemKind.SERIES ? MediaKind.SERIES : undefined;
  const catalog = kind && catalogs.find((c) => c.kind === CatalogKind.RECOMMENDATIONS && c.mediaKinds.includes(kind));
  if (!server || !kind || !catalog || !(tmdb > 0)) return null;
  return <CatalogShelf server={server} row={{ catalog, mediaKind: kind }} tmdbId={tmdb} title={t("discover.recommendations")} />;
}

/**
 * The details: the whole overview with the tagline, keywords and links
 * beside the ratings and facts, and under them the media of the version
 * chosen. Genres, keywords and studios open their items in the library.
 */
function About({
  server,
  item,
  series,
  credits,
  source,
  overview,
}: {
  server: ServerEntry;
  item: Item;
  series?: Item;
  credits: Credit[];
  source?: MediaSource;
  overview: boolean;
}) {
  const names = (kind: CreditKind) =>
    credits
      .filter((c) => c.kind === kind && c.person)
      .map((c) => c.person!.name)
      .join(", ");
  const date = item.premiereDate ? dateFormat.format(timestampDate(item.premiereDate)) : undefined;
  // An episode or season shows its series' genres and studios, whose library it shares.
  const genres = item.genres.length ? item.genres : (series?.genres ?? []);
  const studios = item.studios.length ? item.studios : (series?.studios ?? []);
  const tags = item.kind === ItemKind.EPISODE || item.kind === ItemKind.SEASON ? [] : item.tags;
  const values = (kind: ItemValue["kind"], list: string[]) =>
    list.length > 0 && (
      <>
        {list.map((v, i) => (
          <Fragment key={v}>
            {i > 0 && ", "}
            <Link
              to="/s/$serverId/library/$libraryId"
              params={{ serverId: server.id, libraryId: item.libraryId }}
              search={{ tab: "all", [kind]: v }}
              className="hover:underline"
            >
              {v}
            </Link>
          </Fragment>
        ))}
      </>
    );
  const facts: [string, ReactNode][] = [
    [item.kind === ItemKind.SERIES ? t("item.firstAired") : item.kind === ItemKind.EPISODE ? t("item.aired") : t("item.released"), date],
    [
      t("item.status"),
      item.kind === ItemKind.SERIES && item.seriesStatus === SeriesStatus.CONTINUING
        ? t("item.continuing")
        : item.kind === ItemKind.SERIES && item.seriesStatus === SeriesStatus.ENDED
          ? t("item.ended")
          : undefined,
    ],
    [t("item.runtime"), runtimeOf(item)],
    [t("item.rated"), item.officialRating],
    [t("item.genres"), values("genre", genres)],
    [t("item.creators"), names(CreditKind.CREATOR)],
    [t("item.director"), names(CreditKind.DIRECTOR)],
    [t("item.writers"), names(CreditKind.WRITER)],
    [t("item.producers"), names(CreditKind.PRODUCER)],
    [t("item.music"), names(CreditKind.COMPOSER)],
    [t("item.studio"), values("studio", studios)],
    [t("item.country"), item.productionLocations.join(", ")],
    [t("item.originalTitle"), item.originalTitle && item.originalTitle !== item.name ? item.originalTitle : undefined],
    [t("item.dateAdded"), item.dateAdded && !item.virtual ? dateFormat.format(timestampDate(item.dateAdded)) : undefined],
  ];
  const streams = (kind: StreamKind) => source?.streams.filter((s) => s.kind === kind).map(streamText) ?? [];
  const video = videoOf(source);
  const media: [string, ReactNode][] = [
    [t("item.video"), video ? [resolutionOf(source), video.codec.toUpperCase(), rangeOf(source)].filter(Boolean).join(" · ") : undefined],
    [t("item.audioTracks"), streams(StreamKind.AUDIO).join("\n")],
    [t("item.subtitleTracks"), streams(StreamKind.SUBTITLE).join("\n")],
    [
      t("item.file"),
      source ? [source.path.split("/").pop(), source.sizeBytes > 0n ? sizeOf(source.sizeBytes) : undefined].filter(Boolean).join("\n") : undefined,
    ],
  ];
  const shownFacts = facts.filter(([, v]) => v);
  const shownMedia = media.filter(([, v]) => v);
  const text = overview && item.overview;
  const links = item.externalUrls;
  const ratings = item.communityRating > 0 || item.criticRating > 0;
  if (!text && !ratings && tags.length === 0 && shownFacts.length === 0 && shownMedia.length === 0 && links.length === 0) return null;
  return (
    <section className="px-margin pt-8 pb-4">
      <h2 className="mb-4 type-title3">{t("item.details")}</h2>
      <div className="flex flex-col gap-x-16 gap-y-8 lg:flex-row">
        {(text || tags.length > 0 || links.length > 0) && (
          <div className="flex min-w-0 flex-col gap-5 lg:w-[44%] lg:shrink-0">
            {text && (
              <div className="flex flex-col gap-2">
                {item.tagline && <p className="type-headline">{item.tagline}</p>}
                <p className="type-body whitespace-pre-line text-text-secondary">{item.overview}</p>
              </div>
            )}
            {tags.length > 0 && (
              <div className="flex flex-col gap-2">
                <h3 className="type-footnote font-semibold text-text-secondary">{t("item.tags")}</h3>
                <div className="flex flex-wrap gap-2">
                  {tags.map((tag) => (
                    <Link
                      key={tag}
                      to="/s/$serverId/library/$libraryId"
                      params={{ serverId: server.id, libraryId: item.libraryId }}
                      search={{ tab: "all", tag }}
                      className="inline-flex items-center rounded-full border border-border px-3 py-1 type-footnote hover:bg-fill"
                    >
                      {tag}
                    </Link>
                  ))}
                </div>
              </div>
            )}
            {links.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {links.map((l) => (
                  <a
                    key={l.url}
                    href={l.url}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1.5 rounded-full bg-fill px-3.5 py-1.5 type-footnote font-semibold hover:bg-fill-selected"
                  >
                    {l.name}
                    <Icon name="external" size={14} className="text-text-secondary" />
                  </a>
                ))}
              </div>
            )}
          </div>
        )}
        <div className="flex min-w-0 flex-1 flex-col gap-8">
          {ratings && <Ratings item={item} />}
          <FactList rows={shownFacts} />
          {shownMedia.length > 0 && <FactList rows={shownMedia} />}
        </div>
      </div>
    </section>
  );
}

/** The audience's rating out of 10 and the critics' score out of 100, large. */
function Ratings({ item }: { item: Item }) {
  const rows: [string, ReactNode, string][] = [];
  if (item.communityRating > 0) {
    rows.push([
      t("item.communityRating"),
      <span className="inline-flex items-center gap-1.5">
        <Icon name="rating" fill size={22} className="text-[#FFD60A]" />
        {item.communityRating.toFixed(1)}
      </span>,
      "/ 10",
    ]);
  }
  if (item.criticRating > 0) rows.push([t("item.criticRating"), `${Math.round(item.criticRating)}%`, ""]);
  return (
    <div role="group" aria-label={t("item.ratings")} className="flex flex-wrap gap-3">
      {rows.map(([label, value, scale]) => (
        <div key={label} className="flex min-w-36 flex-col gap-1 rounded-[12px] bg-fill px-4 py-3">
          <span className="type-footnote font-semibold text-text-secondary">{label}</span>
          <span className="type-title2 font-bold tabular-nums">
            {value}
            {scale && <span className="ml-1 type-callout font-normal text-text-tertiary">{scale}</span>}
          </span>
        </div>
      ))}
    </div>
  );
}

function FactList({ rows }: { rows: [string, ReactNode][] }) {
  if (rows.length === 0) return null;
  return (
    <dl className="grid gap-x-8 gap-y-5" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))" }}>
      {rows.map(([k, v]) => (
        <div key={k} className="min-w-0">
          <dt className="type-footnote font-semibold text-text-secondary">{k}</dt>
          <dd className="mt-1 type-callout break-words whitespace-pre-line">{v}</dd>
        </div>
      ))}
    </dl>
  );
}

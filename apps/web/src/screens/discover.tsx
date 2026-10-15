// Discover (docs/clients.md §6.3.2): the catalogs of the first server that
// discovers, a catalog's titles, and a title with its seasons, to watch
// when a library has it and to request when none does.
import {
  type CatalogRow,
  errorMessage,
  rowsOf,
  type ServerEntry,
  titlesOf,
  useCatalog,
  useDiscovery,
  useRequestTitle,
  useServer,
  useTitle,
} from "@mavio/client-ts";
import { type DiscoveredTitle, MediaStatus, type SeasonStatus, type Title } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { CatalogKind } from "@mavio/proto/mavio/plugin/v1/discovery_pb";
import { getRouteApi, Link } from "@tanstack/react-router";
import { type KeyboardEvent, type ReactNode, useEffect, useRef, useState } from "react";

import { discoverLanguage, t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { usePrefs, useWidthClass } from "../prefs.ts";
import { Artwork, Button, CardPlaceholder, Grid, LoadMore, Loading, ScreenTitle, Shelf, StatusView } from "../ui.tsx";

/** A title's kind in addresses. */
export type KindParam = "movie" | "series";

export function kindParam(kind: MediaKind): KindParam {
  return kind === MediaKind.SERIES ? "series" : "movie";
}

function mediaKindOf(p: KindParam | undefined): MediaKind {
  return p === "series" ? MediaKind.SERIES : p === "movie" ? MediaKind.MOVIE : MediaKind.UNSPECIFIED;
}

/** The same TMDB image at a width, rather than its original size. */
export function sized(url: string, width: number): string | undefined {
  if (!url) return undefined;
  return url.replace(/(image\.tmdb\.org\/t\/p\/)original\//, `$1w${width}/`);
}

/** The name of a row of Discover, by its catalog's kind and its kind of title. */
function rowTitle({ catalog, mediaKind }: CatalogRow): string {
  const series = mediaKind === MediaKind.SERIES;
  switch (catalog.kind) {
    case CatalogKind.TRENDING:
      return mediaKind === MediaKind.UNSPECIFIED ? t("discover.trending") : catalog.name;
    case CatalogKind.POPULAR:
      return series ? t("discover.popularSeries") : t("discover.popularMovies");
    case CatalogKind.UPCOMING:
      return series ? t("discover.upcomingSeries") : t("discover.upcomingMovies");
    case CatalogKind.TOP_RATED:
      return series ? t("discover.topRatedSeries") : t("discover.topRatedMovies");
    default:
      return catalog.name;
  }
}

function tmdbOf(title: DiscoveredTitle): number {
  return Number(title.externalIds.tmdb ?? 0);
}

/** The status a title's card and header show, if any. */
function statusLabel(status: MediaStatus): string | undefined {
  switch (status) {
    case MediaStatus.AVAILABLE:
      return t("discover.available");
    case MediaStatus.PARTIALLY_AVAILABLE:
      return t("discover.partiallyAvailable");
    case MediaStatus.REQUESTED:
      return t("discover.requested");
    case MediaStatus.WANTED:
      return t("discover.processing");
    case MediaStatus.BLOCKLISTED:
      return t("discover.blocklisted");
    default:
      return undefined;
  }
}

/** The mark of a card's status: a check once available, a clock while requested or processing. */
function StatusMark({ status }: { status: MediaStatus }) {
  const available = status === MediaStatus.AVAILABLE || status === MediaStatus.PARTIALLY_AVAILABLE;
  const waiting = status === MediaStatus.REQUESTED || status === MediaStatus.WANTED;
  if (!available && !waiting) return null;
  return (
    <span
      title={statusLabel(status)}
      className={`absolute top-1.5 right-1.5 flex rounded-full p-0.5 text-white ${available ? "bg-accent" : "bg-scrim"} ${
        status === MediaStatus.PARTIALLY_AVAILABLE ? "opacity-70" : ""
      }`}
    >
      <Icon name={available ? "check" : "pending"} size={16} />
    </span>
  );
}

/** The card of a discovered title, which opens it. */
export function TitleCard({ serverId, title, fluid = false }: { serverId: string; title: DiscoveredTitle; fluid?: boolean }) {
  const kind = title.mediaKind === MediaKind.SERIES ? t("discover.series") : t("discover.movie");
  return (
    <Link
      to="/s/$serverId/title/$kind/$tmdbId"
      params={{ serverId, kind: kindParam(title.mediaKind), tmdbId: String(tmdbOf(title)) }}
      data-card
      className={`group block shrink-0 snap-start ${fluid ? "w-full" : "w-poster"}`}
    >
      <div className="relative">
        <Artwork src={sized(title.posterUrl, 342)} alt={title.name} className="aspect-[2/3] rounded-md transition group-active:opacity-80" />
        <StatusMark status={title.status} />
      </div>
      <div className="mt-2 truncate type-headline">{title.name}</div>
      <div className="truncate type-footnote text-text-secondary">{[title.year || undefined, kind].filter(Boolean).join(" · ")}</div>
    </Link>
  );
}

/** Discover: a row of each catalog of the first server that discovers. */
export function DiscoverScreen() {
  const { server, catalogs, pending } = useDiscovery(discoverLanguage);
  const { tv } = usePrefs();
  if (pending) return <Loading />;
  if (!server) return <StatusView icon="discover" title={t("discover.notSetUp")} message={t("discover.notSetUpMessage")} />;
  return (
    <div className={tv ? "" : "pt-2"}>
      <div className="flex items-end justify-between gap-3 pr-margin">
        {!tv && <ScreenTitle>{t("nav.discover")}</ScreenTitle>}
        <Link
          to="/s/$serverId/requests"
          params={{ serverId: server.id }}
          search={{}}
          className={`inline-flex min-h-9 shrink-0 items-center gap-1.5 rounded-full bg-fill px-4 type-callout font-semibold hover:bg-fill-selected ${tv ? "mt-4 ml-margin" : "mb-2"}`}
        >
          <Icon name="pending" size={16} />
          {t("requests.title")}
        </Link>
      </div>
      {rowsOf(catalogs).map((row) => (
        <CatalogShelf key={`${row.catalog.id}/${row.mediaKind}`} server={server} row={row} title={rowTitle(row)} />
      ))}
    </div>
  );
}

/**
 * A row of a catalog's first page, with See All: at its title, or on the
 * television as the row's last card, where focus moving along the row
 * reaches it.
 */
function CatalogShelf({ server, row, title, tmdbId = 0 }: { server: ServerEntry; row: CatalogRow; title: string; tmdbId?: number }) {
  const { tv } = usePrefs();
  const q = useCatalog(server, row.catalog.id, row.mediaKind, discoverLanguage, tmdbId);
  const titles = q.data?.pages[0]?.titles ?? [];
  if (q.isError || (q.isSuccess && titles.length === 0)) return null;
  const kind = row.mediaKind === MediaKind.UNSPECIFIED ? undefined : kindParam(row.mediaKind);
  const all = (className?: string, children: ReactNode = t("home.more")) =>
    q.data?.pages[0]?.next ? (
      <Link
        to="/s/$serverId/discover/$catalog"
        params={{ serverId: server.id, catalog: row.catalog.id }}
        search={{ kind, tmdb: tmdbId || undefined }}
        data-card={tv || undefined}
        className={className}
      >
        {children}
      </Link>
    ) : undefined;
  return (
    <Shelf title={title} more={tv ? undefined : all()}>
      {q.isPending
        ? Array.from({ length: 8 }, (_, i) => <CardPlaceholder key={i} />)
        : titles.map((title) => <TitleCard key={`${title.mediaKind}/${tmdbOf(title)}`} serverId={server.id} title={title} />)}
      {tv &&
        all(
          "block w-poster shrink-0 snap-start",
          <div data-art className="flex aspect-[2/3] flex-col items-center justify-center gap-3 rounded-md bg-surface-raised type-headline">
            <Icon name="forward" size={40} />
            {t("home.more")}
          </div>,
        )}
    </Shelf>
  );
}

const catalogRoute = getRouteApi("/s/$serverId/discover/$catalog");

/** All the titles of a catalog, page by page. */
export function CatalogScreen() {
  const { serverId, catalog } = catalogRoute.useParams();
  const { kind, tmdb } = catalogRoute.useSearch();
  const server = useServer(serverId);
  const { catalogs } = useDiscovery(discoverLanguage);
  const q = useCatalog(server?.token ? server : undefined, catalog, mediaKindOf(kind), discoverLanguage, tmdb ?? 0);
  // On the television focus starts on the first title, as the page has no tabs.
  const { tv } = usePrefs();
  const grid = useRef<HTMLDivElement>(null);
  const loaded = q.isSuccess;
  useEffect(() => {
    if (tv && loaded && document.activeElement === document.body) grid.current?.querySelector<HTMLElement>("a")?.focus({ preventScroll: true });
  }, [tv, loaded]);
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  const found = catalogs.find((c) => c.id === catalog);
  const title = found ? (tmdb ? found.name : rowTitle({ catalog: found, mediaKind: mediaKindOf(kind) })) : "";
  return (
    <div>
      <ScreenTitle>{title || " "}</ScreenTitle>
      {q.isPending ? (
        <Loading />
      ) : q.isError ? (
        <StatusView icon="error" title={t("state.error")} action={{ label: t("action.retry"), onClick: () => void q.refetch() }} />
      ) : (
        <div ref={grid} className="pt-2">
          <Grid>
            {titlesOf(q.data.pages).map((title) => (
              <TitleCard key={`${title.mediaKind}/${tmdbOf(title)}`} serverId={server.id} title={title} fluid />
            ))}
          </Grid>
          <LoadMore active={q.hasNextPage && !q.isFetchingNextPage} onVisible={() => void q.fetchNextPage()} />
        </div>
      )}
    </div>
  );
}

const titleRoute = getRouteApi("/s/$serverId/title/$kind/$tmdbId");

/** A discovered title. */
export function TitleScreen() {
  const { serverId, kind, tmdbId } = titleRoute.useParams();
  const server = useServer(serverId);
  const q = useTitle(server?.token ? server : undefined, mediaKindOf(kind as KindParam), Number(tmdbId));
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  if (q.isPending) return <Loading />;
  if (q.isError || !q.data) {
    return <StatusView icon="error" title={t("state.error")} message={q.error ? errorMessage(q.error) : undefined} />;
  }
  return <TitleDetails key={`${kind}/${tmdbId}`} server={server} title={q.data} />;
}

function TitleDetails({ server, title }: { server: ServerEntry; title: Title }) {
  const { tv } = usePrefs();
  const phone = useWidthClass() === "compact";
  const { catalogs } = useDiscovery(discoverLanguage);
  const [asking, setAsking] = useState(false);
  const request = useRequestTitle(server);
  const series = title.mediaKind === MediaKind.SERIES;
  // A series with seasons requested or available already requests more.
  const some = title.seasons.some((s) => s.status !== MediaStatus.UNSPECIFIED && s.status !== MediaStatus.DELETED);
  const facts = [
    title.year || undefined,
    series ? t("discover.series") : t("discover.movie"),
    title.officialRating || undefined,
    title.genres.slice(0, 3).join(" / ") || undefined,
  ].filter(Boolean);
  const status = statusLabel(title.status);
  const onRequest = () => (series ? setAsking(true) : request.mutate({ mediaKind: title.mediaKind, tmdbId: title.tmdbId }));
  // On the television focus starts on the first action, as on an item's details.
  const first = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (tv && document.activeElement === document.body) first.current?.querySelector<HTMLElement>("a, button")?.focus({ preventScroll: true });
  }, [tv]);
  const primary = `inline-flex items-center justify-center gap-2 rounded-[10px] font-semibold type-headline ${
    tv ? "h-16 bg-white/20 px-10 text-white backdrop-blur-xl" : phone ? "h-12 flex-1 bg-white px-5 text-black" : "h-11 bg-white px-6 text-black hover:bg-white/90"
  }`;
  const secondary = `inline-flex items-center justify-center gap-2 rounded-[10px] bg-white/20 font-semibold text-white backdrop-blur-xl type-headline hover:bg-white/30 ${
    tv ? "h-16 px-10" : phone ? "h-12 flex-1 px-5" : "h-11 px-6"
  }`;
  return (
    <div className="pb-8">
      <header
        className={`relative overflow-hidden bg-surface ${
          tv ? "h-[86vh]" : phone ? "h-[64vh] min-h-96" : "mx-2.5 mt-2.5 h-[clamp(520px,70vh,700px)] rounded-[18px]"
        }`}
      >
        {title.backdropUrl ? (
          <img src={sized(title.backdropUrl, 1280)} alt="" className="absolute inset-0 h-full w-full animate-[fade_600ms_ease-out] object-cover" />
        ) : (
          title.posterUrl && (
            <img src={sized(title.posterUrl, 342)} alt="" className="absolute inset-0 h-full w-full scale-125 object-cover opacity-70 blur-3xl" />
          )
        )}
        <div
          className={`absolute inset-0 ${
            phone
              ? "bg-gradient-to-t from-black via-black/50 to-transparent"
              : "bg-[linear-gradient(90deg,rgb(0_0_0/0.82)_0%,rgb(0_0_0/0.45)_45%,transparent_75%),linear-gradient(0deg,rgb(0_0_0/0.75)_0%,transparent_55%)]"
          }`}
        />
        <div className={`absolute inset-x-0 bottom-0 flex items-end gap-10 ${phone ? "px-margin pb-6" : tv ? "px-margin pb-[10vh]" : "p-10"}`}>
          {title.posterUrl && !phone && (
            <img src={sized(title.posterUrl, 500)} alt="" className={`shrink-0 rounded-lg shadow-level3 ${tv ? "w-[300px]" : "w-[200px]"}`} />
          )}
          <div className={`flex min-w-0 flex-col gap-3 text-white ${tv ? "max-w-[960px]" : "max-w-[640px]"}`}>
            <h1 className={`font-bold leading-[1.05] text-balance break-words ${tv ? "text-[72px]" : phone ? "text-[34px]" : "text-[52px]"}`}>{title.name}</h1>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 type-callout text-white/80">
              {facts.length > 0 && <span>{facts.join(" · ")}</span>}
              {title.communityRating > 0 && (
                <span className="inline-flex items-center gap-1">
                  <Icon name="rating" fill size={14} className="text-[#FFD60A]" />
                  {title.communityRating.toFixed(1)}
                </span>
              )}
              {status && <span className="rounded-[4px] bg-white/20 px-1.5 type-caption font-semibold text-white">{status}</span>}
            </div>
            {title.overview && <p className={`type-body text-white/85 ${tv ? "line-clamp-2" : "line-clamp-4"}`}>{title.overview}</p>}
            <div ref={first} className={`mt-2 flex flex-wrap gap-3 ${phone ? "[&>*]:flex-1" : ""}`}>
              {title.itemId && (
                <Link to="/s/$serverId/item/$itemId" params={{ serverId: server.id, itemId: title.itemId }} data-tv-button className={primary}>
                  <Icon name="play" fill size={tv ? 28 : 20} />
                  {t("discover.watch")}
                </Link>
              )}
              {title.requestable && (
                <button
                  type="button"
                  disabled={request.isPending}
                  onClick={onRequest}
                  className={title.itemId ? secondary : primary}
                >
                  <Icon name="add" size={tv ? 28 : 20} />
                  {series && some ? t("discover.requestMore") : t("discover.request")}
                </button>
              )}
            </div>
            {request.isError && <p className="type-callout text-white">{t("discover.requestFailed", { reason: errorMessage(request.error) })}</p>}
          </div>
        </div>
      </header>
      {phone && title.overview && <p className="px-margin pt-4 type-body">{title.overview}</p>}
      {series && title.seasons.length > 0 && <Seasons seasons={title.seasons} />}
      {catalogs
        .filter((c) => (c.kind === CatalogKind.RECOMMENDATIONS || c.kind === CatalogKind.SIMILAR) && c.mediaKinds.includes(title.mediaKind))
        .map((c) => (
          <CatalogShelf
            key={c.id}
            server={server}
            row={{ catalog: c, mediaKind: title.mediaKind }}
            tmdbId={title.tmdbId}
            title={c.kind === CatalogKind.RECOMMENDATIONS ? t("discover.recommendations") : t("discover.similar")}
          />
        ))}
      {asking && (
        <RequestDialog
          title={title}
          busy={request.isPending}
          onCancel={() => setAsking(false)}
          onRequest={(seasons) =>
            request.mutate({ mediaKind: title.mediaKind, tmdbId: title.tmdbId, seasons }, { onSettled: () => setAsking(false) })
          }
        />
      )}
    </div>
  );
}

function seasonName(s: SeasonStatus): string {
  return s.seasonNumber === 0 ? t("discover.specials") : t("discover.season", { number: s.seasonNumber });
}

/** A series' seasons with what the server has of each. */
function Seasons({ seasons }: { seasons: SeasonStatus[] }) {
  return (
    <section className="px-margin py-3">
      <h2 className="mb-2 type-title3">{t("item.seasons")}</h2>
      <div className="flex flex-col divide-y divide-border rounded-lg bg-surface-raised">
        {seasons.map((s) => {
          const status = statusLabel(s.status);
          return (
            <div key={s.seasonNumber} className="flex min-h-12 items-center gap-3 px-4 py-2">
              <span className="min-w-0 flex-1 truncate type-headline">{seasonName(s)}</span>
              {s.episodeCount > 0 && <span className="type-callout text-text-secondary">{t("item.episodeCount", { count: s.episodeCount })}</span>}
              {status && <span className="rounded-full bg-fill px-2.5 py-0.5 type-footnote font-semibold">{status}</span>}
            </div>
          );
        })}
      </div>
    </section>
  );
}

/**
 * The dialog choosing the seasons of a series to request: all those not
 * requested yet, or some; or, given a pending request's seasons, changing
 * them.
 */
export function RequestDialog({
  title,
  requested,
  busy,
  onCancel,
  onRequest,
}: {
  title: Title;
  requested?: number[];
  busy: boolean;
  onCancel: () => void;
  onRequest: (seasons?: number[]) => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const choosable = (s: SeasonStatus) => s.requestable || !!requested?.includes(s.seasonNumber);
  const open = title.seasons.filter(choosable);
  const [chosen, setChosen] = useState<Set<number>>(() => new Set(requested ?? open.map((s) => s.seasonNumber)));
  const all = chosen.size === open.length;
  // Focus returns to Request when the dialog goes.
  const [opener] = useState(() => (document.activeElement instanceof HTMLElement ? document.activeElement : undefined));
  useEffect(() => {
    if (!ref.current?.open) ref.current?.showModal();
    return () => opener?.focus({ preventScroll: true });
  }, [opener]);
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key !== "Escape" && e.key !== "Backspace") return;
    e.preventDefault();
    e.stopPropagation();
    onCancel();
  };
  const toggle = (n: number) =>
    setChosen((c) => {
      const next = new Set(c);
      if (next.has(n)) next.delete(n);
      else next.add(n);
      return next;
    });
  const check = (on: boolean) => (
    <span className={`flex size-6 shrink-0 items-center justify-center rounded-md ${on ? "bg-accent text-on-accent" : "border-2 border-border"}`}>
      {on && <Icon name="check" size={16} />}
    </span>
  );
  const row = "flex min-h-12 w-full items-center gap-3 rounded-lg px-3 text-left type-body hover:bg-fill";
  return (
    <dialog
      ref={ref}
      aria-modal
      aria-labelledby="request-title"
      onClose={onCancel}
      onKeyDown={onKeyDown}
      onClick={(e) => e.target === ref.current && onCancel()}
      className="m-auto max-h-[80vh] w-[min(480px,calc(100vw-32px))] rounded-[18px] border border-border bg-surface-overlay p-0 text-on-background shadow-level3 backdrop:bg-black/60 backdrop:backdrop-blur-sm"
    >
      <div className="flex max-h-[80vh] flex-col gap-3 p-6">
        <h2 id="request-title" className="type-title2">
          {requested ? t("requests.editTitle", { name: title.name }) : t("discover.requestTitle", { name: title.name })}
        </h2>
        <div className="-mx-3 flex min-h-0 flex-col overflow-y-auto">
          <button type="button" role="checkbox" aria-checked={all} onClick={() => setChosen(all ? new Set() : new Set(open.map((s) => s.seasonNumber)))} className={row}>
            {check(all)}
            <span className="flex-1 type-headline">{t("discover.allSeasons")}</span>
          </button>
          {title.seasons.map((s) => {
            const status = statusLabel(s.status);
            return (
              <button
                key={s.seasonNumber}
                type="button"
                role="checkbox"
                aria-checked={chosen.has(s.seasonNumber)}
                disabled={!choosable(s)}
                onClick={() => toggle(s.seasonNumber)}
                className={`${row} disabled:opacity-50 disabled:hover:bg-transparent`}
              >
                {check(chosen.has(s.seasonNumber))}
                <span className="min-w-0 flex-1 truncate">{seasonName(s)}</span>
                {s.episodeCount > 0 && <span className="type-callout text-text-secondary">{t("item.episodeCount", { count: s.episodeCount })}</span>}
                {status && <span className="type-footnote text-text-secondary">{status}</span>}
              </button>
            );
          })}
        </div>
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="secondary" onClick={onCancel}>
            {t("action.cancel")}
          </Button>
          <Button
            disabled={busy || chosen.size === 0}
            onClick={() => onRequest(all && !requested ? undefined : [...chosen].sort((a, b) => a - b))}
          >
            {requested ? t("action.save") : t("discover.requestSeasons", { count: chosen.size })}
          </Button>
        </div>
      </div>
    </dialog>
  );
}

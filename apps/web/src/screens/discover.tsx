// Discover (docs/clients.md §6.3.2): the catalogs of the first server that
// discovers, a catalog's titles, and a title with its seasons, to watch
// when a library has it and to request when none does.
import {
  type CatalogFilters,
  type CatalogRow,
  errorMessage,
  filterCount,
  rowsOf,
  type ServerEntry,
  titlesOf,
  useAdminServers,
  useCatalog,
  useDiscovery,
  useIssues,
  useItem,
  useQueue,
  useRequestActions,
  useRequestTitle,
  useSeason,
  useServer,
  useTitle,
  useUsers,
} from "@mavio/client-ts";
import { type DiscoveredTitle, MediaStatus, type SeasonStatus, type Title } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { DownloadState } from "@mavio/proto/mavio/acquisition/v1/download_pb";
import { IssueStatus } from "@mavio/proto/mavio/acquisition/v1/issue_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { RequestStatus } from "@mavio/proto/mavio/acquisition/v1/request_pb";
import { CreditKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { CatalogKind } from "@mavio/proto/mavio/plugin/v1/discovery_pb";
import { getRouteApi, Link } from "@tanstack/react-router";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { discoverLanguage, t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { usePrefs, useWidthClass } from "../prefs.ts";
import { FiltersDialog, parseFilters } from "./discover-filters.tsx";
import { Artwork, Button, CardPlaceholder, Grid, Group, LoadMore, Loading, minutesText, ScreenTitle, Shelf, StatusView } from "../ui.tsx";
import { CastShelf, titleCredits } from "./credits.tsx";
import { dayText, FactList, languageText, moneyText, WatchProviders } from "./facts.tsx";
import { Modal } from "../controls/dialog.tsx";
import { Checkbox } from "../controls/inputs.tsx";
import { QueueRow } from "./admin/downloads.tsx";
import { IssueDialog, IssueRow, ReportDialog, useMayReport } from "./issues.tsx";
import { RequestRow } from "./requests.tsx";

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

/** A title's TMDB ID. */
export function tmdbOf(title: DiscoveredTitle): number {
  return Number(title.externalIds.tmdb ?? 0);
}

/** The status a title's card and header show, if any. */
export function statusLabel(status: MediaStatus): string | undefined {
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
      role="img"
      aria-label={statusLabel(status)}
      className={`absolute top-1.5 right-1.5 flex rounded-full p-0.5 text-white ${available ? "bg-accent" : "bg-scrim"} ${
        status === MediaStatus.PARTIALLY_AVAILABLE ? "opacity-70" : ""
      }`}
    >
      <Icon name={available ? "check" : "pending"} size={16} />
    </span>
  );
}

/** The card of a discovered title, which opens it; under its year and kind, a person's role in it. */
export function TitleCard({ serverId, title, fluid = false, role }: { serverId: string; title: DiscoveredTitle; fluid?: boolean; role?: string }) {
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
      {role && <div className="truncate type-footnote text-text-secondary">{role}</div>}
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
export function CatalogShelf({
  server,
  row,
  title,
  tmdbId = 0,
  filters,
  more,
}: {
  server: ServerEntry;
  row: CatalogRow;
  title: string;
  tmdbId?: number;
  /** Filters of the row, which then has no See All. */
  filters?: CatalogFilters;
  /** A link of the row's own, in place of See All. */
  more?: ReactNode;
}) {
  const { tv } = usePrefs();
  const q = useCatalog(server, row.catalog.id, row.mediaKind, discoverLanguage, tmdbId, filters);
  const titles = q.data?.pages[0]?.titles ?? [];
  if (q.isError || (q.isSuccess && titles.length === 0)) return null;
  const kind = row.mediaKind === MediaKind.UNSPECIFIED ? undefined : kindParam(row.mediaKind);
  const all = (className?: string, children: ReactNode = t("home.more")) =>
    q.data?.pages[0]?.next && !filters ? (
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
    <Shelf title={title} more={tv ? undefined : (more ?? all())}>
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
  const { kind, tmdb, f = noFilters } = catalogRoute.useSearch();
  const navigate = catalogRoute.useNavigate();
  const server = useServer(serverId);
  const { catalogs } = useDiscovery(discoverLanguage);
  const [filtering, setFiltering] = useState(false);
  const q = useCatalog(server?.token ? server : undefined, catalog, mediaKindOf(kind), discoverLanguage, tmdb ?? 0, f);
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
  // Recommendations and similar titles take no filters.
  const filterable = !!found && !tmdb && (found.filters.length > 0 || found.sorts.length > 0);
  const n = filterCount(f);
  const titles = q.isSuccess ? titlesOf(q.data.pages) : [];
  return (
    <div>
      <div className="flex items-end justify-between gap-3 pe-margin">
        <ScreenTitle>{title || " "}</ScreenTitle>
        {filterable && (
          <Button variant={n > 0 || f.sort ? "primary" : "secondary"} onClick={() => setFiltering(true)} className="mb-2 shrink-0">
            <Icon name="filter" size={16} />
            {n > 0 ? `${t("discover.filters")} (${n})` : t("discover.filters")}
          </Button>
        )}
      </div>
      {filtering && found && (
        <FiltersDialog
          server={server}
          catalog={found}
          mediaKind={mediaKindOf(kind)}
          value={f}
          onClose={() => setFiltering(false)}
          onApply={(next) => {
            setFiltering(false);
            void navigate({ search: (cur) => ({ ...cur, f: parseFilters(next) }), replace: true });
          }}
        />
      )}
      {q.isPending ? (
        <Loading />
      ) : q.isError ? (
        <StatusView icon="error" title={t("state.error")} action={{ label: t("action.retry"), onClick: () => void q.refetch() }} />
      ) : titles.length === 0 && (n > 0 || !!f.sort) ? (
        <StatusView
          icon="search"
          title={t("discover.noMatches")}
          action={{ label: t("discover.reset"), onClick: () => void navigate({ search: (cur) => ({ ...cur, f: undefined }), replace: true }) }}
        />
      ) : (
        <div ref={grid} className="pt-2">
          <Grid>
            {titles.map((title) => (
              <TitleCard key={`${title.mediaKind}/${tmdbOf(title)}`} serverId={server.id} title={title} fluid />
            ))}
          </Grid>
          <LoadMore active={q.hasNextPage && !q.isFetchingNextPage} onVisible={() => void q.fetchNextPage()} />
        </div>
      )}
    </div>
  );
}

const noFilters: CatalogFilters = {};

const titleRoute = getRouteApi("/s/$serverId/title/$kind/$tmdbId");

/** A discovered title. */
export function TitleScreen() {
  const { serverId, kind, tmdbId } = titleRoute.useParams();
  const server = useServer(serverId);
  const q = useTitle(server?.token ? server : undefined, mediaKindOf(kind as KindParam), Number(tmdbId), discoverLanguage);
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
  const admin = useAdminServers().some((s) => s.id === server.id);
  const actions = useRequestActions(server);
  const [dialog, setDialog] = useState<"manage" | "report">();
  // An issue is reported of what a library has, as on Seerr once available.
  const available = title.status === MediaStatus.AVAILABLE || title.status === MediaStatus.PARTIALLY_AVAILABLE;
  const mayReport = useMayReport(server);
  const item = useItem(available && title.itemId && mayReport ? server : undefined, title.itemId);
  const pending = title.requests.filter((r) => r.status === RequestStatus.PENDING);
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
  // The other parts of a movie's collection, as the collection catalog lists them.
  const collection = title.externalIds.tmdb_collection
    ? catalogs.find((c) => c.kind === CatalogKind.COLLECTION && c.mediaKinds.includes(title.mediaKind))
    : undefined;
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
              {admin && title.itemId && <DownloadBadge server={server} itemId={title.itemId} />}
            </div>
            {title.overview && <p className={`type-body text-white/85 ${tv ? "line-clamp-2" : "line-clamp-4"}`}>{title.overview}</p>}
            <div ref={first} className={`mt-2 flex flex-wrap gap-3 ${phone ? "[&>*]:flex-1" : ""}`}>
              {title.itemId && (
                <Link to="/s/$serverId/item/$itemId" params={{ serverId: server.id, itemId: title.itemId }} data-tv-button className={primary}>
                  <Icon name="play" fill size={tv ? 28 : 20} />
                  {t("discover.watch")}
                </Link>
              )}
              {title.trailerUrls[0] && (
                <a href={title.trailerUrls[0]} target="_blank" rel="noreferrer" data-tv-button className={secondary}>
                  <Icon name="movies" size={tv ? 28 : 20} />
                  {t("item.trailer")}
                </a>
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
              {admin && pending.length > 0 && (
                <>
                  <button
                    type="button"
                    disabled={actions.approve.isPending || actions.decline.isPending}
                    onClick={() => pending.forEach((r) => actions.approve.mutate(r.id))}
                    className={secondary}
                  >
                    <Icon name="check" size={tv ? 28 : 20} />
                    {t("requests.approve")}
                  </button>
                  <button
                    type="button"
                    disabled={actions.approve.isPending || actions.decline.isPending}
                    onClick={() => pending.forEach((r) => actions.decline.mutate(r.id))}
                    className={secondary}
                  >
                    <Icon name="close" size={tv ? 28 : 20} />
                    {t("requests.decline")}
                  </button>
                </>
              )}
              {item.data?.item && (
                <button type="button" onClick={() => setDialog("report")} className={secondary}>
                  <Icon name="error" size={tv ? 28 : 20} />
                  {t("discover.reportIssue")}
                </button>
              )}
              {admin && (
                <button type="button" onClick={() => setDialog("manage")} className={secondary}>
                  <Icon name="settings" size={tv ? 28 : 20} />
                  {t("discover.manage")}
                </button>
              )}
            </div>
            {request.isError && <p className="type-callout text-white">{t("discover.requestFailed", { reason: errorMessage(request.error) })}</p>}
            {[actions.approve, actions.decline].map(
              (m, k) => m.isError && <p key={k} className="type-callout break-words text-white">{errorMessage(m.error)}</p>,
            )}
          </div>
        </div>
      </header>
      {phone && title.overview && <p className="px-margin pt-4 type-body">{title.overview}</p>}
      {series && title.seasons.length > 0 && <Seasons server={server} tmdbId={title.tmdbId} seasons={title.seasons} />}
      <CastShelf
        serverId={server.id}
        credits={titleCredits(title.credits)}
        all={
          <Link
            to="/s/$serverId/title/$kind/$tmdbId/credits"
            params={{ serverId: server.id, kind: kindParam(title.mediaKind), tmdbId: String(title.tmdbId) }}
          >
            {t("credits.all")}
          </Link>
        }
      />
      {collection && (
        <CatalogShelf
          server={server}
          row={{ catalog: collection, mediaKind: title.mediaKind }}
          filters={{ collectionId: Number(title.externalIds.tmdb_collection) }}
          title={title.collectionName || t("item.collection")}
          more={
            <Link
              to="/s/$serverId/discover/collection/$tmdbId"
              params={{ serverId: server.id, tmdbId: title.externalIds.tmdb_collection }}
            >
              {t("collection.view")}
            </Link>
          }
        />
      )}
      <TitleFacts title={title} />
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
      {dialog === "report" && item.data?.item && <ReportDialog server={server} item={item.data.item} onClose={() => setDialog(undefined)} />}
      {dialog === "manage" && <ManageDialog server={server} title={title} onClose={() => setDialog(undefined)} />}
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

/** How far the title's downloads have come, for administrators, as Seerr's status badge. */
function DownloadBadge({ server, itemId }: { server: ServerEntry; itemId: string }) {
  const queue = useQueue(server, itemId);
  const downloading = (queue.data?.pages.flatMap((p) => p.items) ?? []).filter((d) => d.state === DownloadState.DOWNLOADING && Number(d.size) > 0);
  if (downloading.length === 0) return null;
  const size = downloading.reduce((n, d) => n + Number(d.size), 0);
  const left = downloading.reduce((n, d) => n + Number(d.remainingSize), 0);
  const percent = Math.round(Math.max(0, Math.min(1, 1 - left / size)) * 100);
  return <span className="rounded-[4px] bg-accent px-1.5 type-caption font-semibold text-white">{t("discover.downloading", { percent })}</span>;
}

/**
 * What an administrator manages of a title, as Seerr's manage panel: its
 * requests, downloads and open issues, and the blocklist.
 */
function ManageDialog({ server, title, onClose }: { server: ServerEntry; title: Title; onClose: () => void }) {
  const actions = useRequestActions(server);
  const users = useUsers(server, true);
  const target = { mediaKind: title.mediaKind, tmdbId: title.tmdbId };
  const blocked = title.status === MediaStatus.BLOCKLISTED;
  const blockable = title.status === MediaStatus.UNSPECIFIED && title.requests.length === 0;
  const error = [actions.blockTitle, actions.unblockTitle].find((m) => m.isError)?.error;
  return (
    <Modal title={t("discover.manageTitle", { name: title.name })} wide onClose={onClose}>
      <div className="-mx-6 flex min-h-0 flex-col gap-6 overflow-y-auto px-6 pb-1">
        <Group title={t("requests.title")}>
          {title.requests.length === 0 ? (
            <p className="px-4 py-3 type-callout text-text-secondary">{t("discover.noRequests")}</p>
          ) : (
            title.requests.map((r) => (
              <RequestRow key={r.id} server={server} request={r} admin userName={users.data?.find((u) => u.id === r.requestedBy)?.name ?? ""} />
            ))
          )}
        </Group>
        {title.itemId && <ManageDownloads server={server} itemId={title.itemId} />}
        {title.itemId && <ManageIssues server={server} itemId={title.itemId} />}
        {title.itemId && (
          <Link
            to="/s/$serverId/item/$itemId"
            params={{ serverId: server.id, itemId: title.itemId }}
            onClick={onClose}
            className="type-callout font-semibold text-accent hover:underline"
          >
            {t("discover.openItem")}
          </Link>
        )}
        {(blocked || blockable) && (
          <Group footer={blocked ? undefined : t("discover.blockFooter")}>
            <div className="flex flex-wrap items-center gap-3 px-4 py-3">
              {blocked ? (
                <Button variant="secondary" disabled={actions.unblockTitle.isPending} onClick={() => actions.unblockTitle.mutate(target)}>
                  {t("collection.unblock")}
                </Button>
              ) : (
                <Button variant="secondary" className="text-danger" disabled={actions.blockTitle.isPending} onClick={() => actions.blockTitle.mutate(target)}>
                  {t("collection.block")}
                </Button>
              )}
            </div>
            {error != null && <p className="px-4 pb-3 type-footnote break-words text-danger">{errorMessage(error)}</p>}
          </Group>
        )}
      </div>
    </Modal>
  );
}

function ManageDownloads({ server, itemId }: { server: ServerEntry; itemId: string }) {
  const queue = useQueue(server, itemId);
  const downloads = queue.data?.pages.flatMap((p) => p.items) ?? [];
  if (downloads.length === 0) return null;
  return (
    <Group title={t("downloads.queue")}>
      {downloads.map((d) => (
        <QueueRow key={d.id} server={server} item={d} />
      ))}
    </Group>
  );
}

function ManageIssues({ server, itemId }: { server: ServerEntry; itemId: string }) {
  const list = useIssues(server, [IssueStatus.OPEN], itemId);
  const [open, setOpen] = useState<string>();
  const issues = list.data?.pages.flatMap((p) => p.issues) ?? [];
  if (issues.length === 0) return null;
  return (
    <Group title={t("issues.itemTitle")}>
      {issues.map((i) => (
        <IssueRow key={i.id} server={server} issue={i} item={false} onOpen={() => setOpen(i.id)} />
      ))}
      {open && <IssueDialog server={server} id={open} onClose={() => setOpen(undefined)} />}
    </Group>
  );
}

/** A title's facts, as Seerr's movie and series pages show them, and the services streaming it. */
function TitleFacts({ title }: { title: Title }) {
  const series = title.mediaKind === MediaKind.SERIES;
  const names = (kind: CreditKind) =>
    title.credits
      .filter((c) => c.kind === kind)
      .map((c) => c.name)
      .join(", ");
  const rows: [string, ReactNode][] = [
    [t("item.creators"), names(CreditKind.CREATOR)],
    [t("item.director"), names(CreditKind.DIRECTOR)],
    [t("item.writers"), names(CreditKind.WRITER)],
    [t("item.producers"), names(CreditKind.PRODUCER)],
    [t("item.music"), names(CreditKind.COMPOSER)],
    [series ? t("item.firstAired") : t("item.released"), dayText(title.premiereDate)],
    [t("item.digitalRelease"), dayText(title.digitalReleaseDate)],
    [t("item.physicalRelease"), dayText(title.physicalReleaseDate)],
    [t("item.lastAired"), dayText(title.endDate)],
    [t("item.nextAirDate"), dayText(title.nextAirDate)],
    [t("item.runtime"), title.runtimeMinutes > 0 ? minutesText(title.runtimeMinutes) : undefined],
    [t("item.rated"), title.officialRating],
    [t("item.genres"), title.genres.join(", ")],
    [series ? t("item.networks") : t("item.studio"), title.studios.join(", ")],
    [t("item.country"), title.productionLocations.join(", ")],
    [t("item.originalLanguage"), languageText(title.originalLanguage)],
    [t("item.originalTitle"), title.originalTitle && title.originalTitle !== title.name ? title.originalTitle : undefined],
    [t("item.budget"), moneyText(title.budget)],
    [t("item.revenue"), moneyText(title.revenue)],
  ];
  const shown = rows.filter(([, v]) => v);
  if (shown.length === 0 && title.watchProviders.length === 0 && !title.tagline && title.keywords.length === 0) return null;
  return (
    <section className="flex flex-col gap-6 px-margin pt-6 pb-3">
      <h2 className="type-title3">{t("item.details")}</h2>
      {title.tagline && <p className="type-title3 font-normal text-text-secondary italic">{title.tagline}</p>}
      <FactList rows={shown} />
      <WatchProviders providers={title.watchProviders} />
      {title.keywords.length > 0 && (
        <ul className="flex flex-wrap gap-2">
          {title.keywords.map((k) => (
            <li key={k} className="rounded-full bg-fill px-3 py-1 type-footnote">
              {k}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function seasonName(s: SeasonStatus): string {
  return s.seasonNumber === 0 ? t("discover.specials") : t("discover.season", { number: s.seasonNumber });
}

/**
 * A series' seasons with what the server has of each, as Seerr lists them:
 * the latest first, specials last, each expanding to its episodes.
 */
function Seasons({ server, tmdbId, seasons }: { server: ServerEntry; tmdbId: number; seasons: SeasonStatus[] }) {
  const ordered = seasons.toSorted((a, b) => Number(a.seasonNumber === 0) - Number(b.seasonNumber === 0) || b.seasonNumber - a.seasonNumber);
  return (
    <section className="px-margin py-3">
      <h2 className="mb-2 type-title3">{t("item.seasons")}</h2>
      <div className="flex flex-col divide-y divide-border rounded-lg bg-surface-raised">
        {ordered.map((s) => (
          <Season key={s.seasonNumber} server={server} tmdbId={tmdbId} season={s} />
        ))}
      </div>
    </section>
  );
}

function Season({ server, tmdbId, season: s }: { server: ServerEntry; tmdbId: number; season: SeasonStatus }) {
  const [open, setOpen] = useState(false);
  const status = statusLabel(s.status);
  return (
    <div>
      <button type="button" aria-expanded={open} onClick={() => setOpen(!open)} className="flex min-h-12 w-full items-center gap-3 px-4 py-2 text-left">
        <span className="min-w-0 flex-1 truncate type-headline">{seasonName(s)}</span>
        {s.episodeCount > 0 && <span className="type-callout text-text-secondary">{t("item.episodeCount", { count: s.episodeCount })}</span>}
        {status && <span className="rounded-full bg-fill px-2.5 py-0.5 type-footnote font-semibold">{status}</span>}
        <Icon name="expand" size={16} className={`text-text-secondary transition-transform ${open ? "rotate-180" : ""}`} />
      </button>
      {open && <SeasonEpisodes server={server} tmdbId={tmdbId} seasonNumber={s.seasonNumber} />}
    </div>
  );
}

/** The episodes of an expanded season, the latest first, as Seerr's. */
function SeasonEpisodes({ server, tmdbId, seasonNumber }: { server: ServerEntry; tmdbId: number; seasonNumber: number }) {
  const q = useSeason(server, tmdbId, seasonNumber, discoverLanguage);
  if (q.isPending) return <div className="px-4 pb-4 type-callout text-text-secondary">{t("state.loading")}</div>;
  if (q.isError || q.data.length === 0) {
    return <p className="px-4 pb-4 type-callout text-text-secondary">{q.isError ? t("state.error") : t("discover.noEpisodes")}</p>;
  }
  return (
    <ul className="mx-4 divide-y divide-border border-t border-border">
      {q.data.toReversed().map((e) => (
        <li key={e.episodeNumber} className="flex flex-col gap-3 py-4 sm:flex-row sm:items-start">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
              <h3 className="type-headline">{t("discover.episodeTitle", { number: e.episodeNumber, name: e.name })}</h3>
              {e.airDate && <span className="rounded-full bg-fill px-2.5 py-0.5 type-footnote">{dayText(e.airDate)}</span>}
            </div>
            {e.overview && <p className="mt-1 type-callout text-text-secondary">{e.overview}</p>}
          </div>
          {e.stillUrl && <img src={sized(e.stillUrl, 300)} alt="" loading="lazy" className="aspect-video w-full shrink-0 rounded-lg object-cover sm:w-56" />}
        </li>
      ))}
    </ul>
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
  const choosable = (s: SeasonStatus) => s.requestable || !!requested?.includes(s.seasonNumber);
  const open = title.seasons.filter(choosable);
  const [chosen, setChosen] = useState<Set<number>>(() => new Set(requested ?? open.map((s) => s.seasonNumber)));
  const all = chosen.size === open.length;
  const toggle = (n: number) =>
    setChosen((c) => {
      const next = new Set(c);
      if (next.has(n)) next.delete(n);
      else next.add(n);
      return next;
    });
  const row = "min-h-12 rounded-lg px-3 hover:bg-fill";
  return (
    <Modal title={requested ? t("requests.editTitle", { name: title.name }) : t("discover.requestTitle", { name: title.name })} onClose={onCancel}>
        <div className="-mx-3 flex min-h-0 flex-col overflow-y-auto">
          <Checkbox checked={all} onChange={(on) => setChosen(on ? new Set(open.map((s) => s.seasonNumber)) : new Set())} className={row}>
            <span className="type-headline">{t("discover.allSeasons")}</span>
          </Checkbox>
          {title.seasons.map((s) => {
            const status = statusLabel(s.status);
            return (
              <Checkbox
                key={s.seasonNumber}
                checked={chosen.has(s.seasonNumber)}
                disabled={!choosable(s)}
                onChange={() => toggle(s.seasonNumber)}
                className={row}
              >
                <span className="min-w-0 flex-1 truncate">{seasonName(s)}</span>
                {s.episodeCount > 0 && <span className="type-callout text-text-secondary">{t("item.episodeCount", { count: s.episodeCount })}</span>}
                {status && <span className="type-footnote text-text-secondary">{status}</span>}
              </Checkbox>
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
    </Modal>
  );
}

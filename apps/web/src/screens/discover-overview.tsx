// A discovered title's overview (docs/clients.md §6.3.2), laid out as
// Seerr's movie and series pages: the tagline, overview, leading crew and
// keywords beside its collection, ratings and facts, each genre, keyword,
// studio, network and language opening the popular titles alike.
import { type CatalogFilters, type ServerEntry, useCollection, useDiscovery, useTitleRatings } from "@mavio/client-ts";
import type { Title } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { CreditKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { CatalogKind } from "@mavio/proto/mavio/plugin/v1/discovery_pb";
import { ProductionStatus } from "@mavio/proto/mavio/plugin/v1/metadata_pb";
import { type Rating, RatingSource } from "@mavio/proto/mavio/plugin/v1/ratings_pb";
import { Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import imdbLogo from "../assets/services/imdb.svg";
import letterboxdLogo from "../assets/services/letterboxd.svg";
import rtAudienceFresh from "../assets/services/rt-aud-fresh.svg";
import rtAudienceRotten from "../assets/services/rt-aud-rotten.svg";
import rtFresh from "../assets/services/rt-fresh.svg";
import rtRotten from "../assets/services/rt-rotten.svg";
import rtLogo from "../assets/services/rt.svg";
import simklLogo from "../assets/services/simkl.svg";
import tmdbLogo from "../assets/services/tmdb.svg";
import tmdbWideLogo from "../assets/services/tmdb-wide.svg";
import traktLogo from "../assets/services/trakt.svg";
import tvdbLogo from "../assets/services/tvdb.svg";
import { discoverLanguage, locale, t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { Wordmark } from "../logo.tsx";
import { minutesText } from "../ui.tsx";
import { type CreditEntry, crewLabels, isCast, titleCredits } from "./credits.tsx";
import { kindParam, sized } from "./discover.tsx";
import { dayText, languageText, moneyText } from "./facts.tsx";
import { useEntryState } from "../shell/entry-state.ts";

const regions = new Intl.DisplayNames(locale, { type: "region" });

/** The name of an ISO 3166-1 country in the UI's language, after its flag. */
export function countryText(code: string): string {
  const flag = /^[A-Z]{2}$/.test(code) ? String.fromCodePoint(...[...code].map((c) => 0x1f1a5 + c.charCodeAt(0))) : "";
  let name = code;
  try {
    name = regions.of(code) ?? code;
  } catch {
    // An unknown code names itself.
  }
  return flag ? `${flag} ${name}` : name;
}

export const statusNames: Partial<Record<ProductionStatus, () => string>> = {
  [ProductionStatus.RUMORED]: () => t("discover.statusRumored"),
  [ProductionStatus.PLANNED]: () => t("discover.statusPlanned"),
  [ProductionStatus.IN_PRODUCTION]: () => t("discover.statusInProduction"),
  [ProductionStatus.POST_PRODUCTION]: () => t("discover.statusPostProduction"),
  [ProductionStatus.RELEASED]: () => t("discover.statusReleased"),
  [ProductionStatus.CANCELED]: () => t("discover.statusCanceled"),
  [ProductionStatus.PILOT]: () => t("discover.statusPilot"),
  [ProductionStatus.RETURNING]: () => t("discover.statusReturning"),
  [ProductionStatus.ENDED]: () => t("discover.statusEnded"),
};

/** The jobs Seerr shows first of a title's crew, in its order, after a series' creators. */
const crewJobs = ["Director", "Creator", "Screenplay", "Writer", "Original Music Composer", "Composer", "Editor", "Producer", "Co-Producer", "Executive Producer", "Animation"];
/** The job of a credit without a more specific one. */
const kindJobs: Partial<Record<CreditKind, string>> = {
  [CreditKind.DIRECTOR]: "Director",
  [CreditKind.CREATOR]: "Creator",
  [CreditKind.WRITER]: "Writer",
  [CreditKind.COMPOSER]: "Composer",
  [CreditKind.PRODUCER]: "Producer",
};
export const crewRank = (c: { role: string; kind: CreditKind }) => {
  if (c.kind === CreditKind.CREATOR) return -1;
  const i = crewJobs.indexOf(c.role || (kindJobs[c.kind] ?? ""));
  return i < 0 ? crewJobs.length : i;
};

/** Links to the popular titles of a kind filtered alike; plain text without such a catalog. */
function useAlike(server: ServerEntry, kind: MediaKind) {
  const { catalogs } = useDiscovery(discoverLanguage);
  const popular = catalogs.find((c) => c.kind === CatalogKind.POPULAR && c.mediaKinds.includes(kind));
  return (f: CatalogFilters, children: ReactNode, className = "hover:underline") =>
    popular ? (
      <Link
        to="/s/$serverId/discover/$catalog"
        params={{ serverId: server.id, catalog: popular.id }}
        search={{ kind: kindParam(kind), f }}
        className={className}
      >
        {children}
      </Link>
    ) : (
      <span className={className}>{children}</span>
    );
}

/** The genres of a title's header, each opening the popular titles of the genre. */
export function GenreLinks({ server, title }: { server: ServerEntry; title: Title }) {
  const alike = useAlike(server, title.mediaKind);
  const genres = title.genreRefs.length > 0 ? title.genreRefs : title.genres.map((name) => ({ id: "", name }));
  return genres.slice(0, 3).map((g, i) => (
    <span key={g.id || g.name}>
      {i > 0 && " / "}
      {g.id ? alike({ genres: [g.id] }, g.name) : g.name}
    </span>
  ));
}

/** The overview and facts of a discovered title, in two columns where they fit. */
export function TitleOverview({ server, title }: { server: ServerEntry; title: Title }) {
  return (
    <div className="grid gap-8 px-margin pt-6 pb-3 lg:grid-cols-[minmax(0,1fr)_minmax(320px,380px)]">
      <OverviewLeft server={server} title={title} />
      <div className="flex min-w-0 flex-col gap-6">
        <CollectionCard server={server} title={title} />
        <Facts server={server} title={title} />
      </div>
    </div>
  );
}

function OverviewLeft({ server, title }: { server: ServerEntry; title: Title }) {
  const alike = useAlike(server, title.mediaKind);
  const keywords = title.keywordRefs.length > 0 ? title.keywordRefs : title.keywords.map((name) => ({ id: "", name }));
  return (
    <div className="flex min-w-0 flex-col gap-4">
      {title.tagline && <p className="type-title3 font-normal text-text-secondary italic">{title.tagline}</p>}
      <h2 className="type-title3">{t("discover.overview")}</h2>
      <p className="type-body whitespace-pre-line">{title.overview || t("discover.noOverview")}</p>
      <CrewList
        serverId={server.id}
        credits={titleCredits(title.credits)}
        more={
          <Link
            to="/s/$serverId/title/$kind/$tmdbId/crew"
            params={{ serverId: server.id, kind: kindParam(title.mediaKind), tmdbId: String(title.tmdbId) }}
            className={crewMoreClass}
          >
            {t("discover.fullCrew")}
            <Icon name="forward" size={16} />
          </Link>
        }
      />
      {keywords.length > 0 && (
        <ul className="flex flex-wrap gap-2 pt-2">
          {keywords.map((k) => (
            <li key={k.id || k.name}>
              {k.id ? (
                alike({ keywords: [{ id: k.id, name: k.name }] }, k.name, "inline-flex rounded-full bg-fill px-3 py-1 type-footnote hover:bg-fill-selected")
              ) : (
                <span className="inline-flex rounded-full bg-fill px-3 py-1 type-footnote">{k.name}</span>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** The class of the link under the leading crew opening the whole crew. */
export const crewMoreClass = "inline-flex items-center gap-1 self-end type-callout text-text-secondary hover:text-on-background";

/**
 * The leading crew, as Seerr's: six of them, a series' creators first,
 * then by the jobs Seerr shows first, each opening their page; and under
 * them the link to the whole crew.
 */
export function CrewList({ serverId, credits, more }: { serverId: string; credits: CreditEntry[]; more: ReactNode }) {
  const crew = credits
    .filter((c) => !isCast(c) && crewLabels.some(([k]) => k === c.kind))
    .map((c, i) => ({ c, i }))
    .toSorted((a, b) => crewRank(a.c) - crewRank(b.c) || a.i - b.i)
    .slice(0, 6)
    .map(({ c }) => c);
  if (crew.length === 0) return null;
  const name = "truncate type-callout text-text-secondary";
  return (
    <>
      <ul className="grid grid-cols-2 gap-x-6 gap-y-4 pt-2 sm:grid-cols-3">
        {crew.map((c, i) => (
          <li key={`${c.personId ?? c.name}-${i}`} className="flex min-w-0 flex-col">
            <span className="type-headline">{c.kind === CreditKind.OTHER && c.role ? c.role : t(crewLabels.find(([k]) => k === c.kind)![1])}</span>
            {c.personId ? (
              <Link to="/s/$serverId/person/$personId" params={{ serverId, personId: c.personId }} className={`${name} hover:text-on-background hover:underline`}>
                {c.name}
              </Link>
            ) : c.tmdbId ? (
              <Link
                to="/s/$serverId/discover/person/$tmdbId"
                params={{ serverId, tmdbId: c.tmdbId }}
                className={`${name} hover:text-on-background hover:underline`}
              >
                {c.name}
              </Link>
            ) : (
              <span className={name}>{c.name}</span>
            )}
          </li>
        ))}
      </ul>
      {more}
    </>
  );
}

/** A movie's collection as Seerr's card: its backdrop and name, opening its page. */
function CollectionCard({ server, title }: { server: ServerEntry; title: Title }) {
  const id = Number(title.externalIds.tmdb_collection ?? 0);
  const collection = useCollection(id > 0 ? server : undefined, id, discoverLanguage).data;
  if (id <= 0) return null;
  const backdrop = collection?.backdropUrl || collection?.posterUrl;
  return (
    <Link
      to="/s/$serverId/discover/collection/$tmdbId"
      params={{ serverId: server.id, tmdbId: String(id) }}
      className="group relative flex min-h-20 items-center justify-between gap-3 overflow-hidden rounded-[14px] bg-surface-raised p-4 text-white"
    >
      {backdrop && <img src={sized(backdrop, 780)} alt="" className="absolute inset-0 h-full w-full object-cover transition-transform duration-300 group-hover:scale-105" />}
      <span className="absolute inset-0 bg-gradient-to-b from-black/45 to-black/80" />
      <span className="relative min-w-0 truncate type-headline">{collection?.name || title.collectionName || t("item.collection")}</span>
      <span className="relative shrink-0 rounded-full bg-white/20 px-3 py-1 type-footnote font-semibold backdrop-blur-xl">{t("collection.view")}</span>
    </Link>
  );
}

function Fact({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 px-4 py-2.5">
      <dt className="shrink-0 type-callout font-semibold">{label}</dt>
      <dd className="flex min-w-0 flex-col items-end text-right type-callout text-text-secondary">{children}</dd>
    </div>
  );
}

/** A date with the icon of how it was released, named on hover. */
export function Dated({ icon, label, date }: { icon: IconName; label: string; date: string }) {
  return (
    <span className="inline-flex items-center gap-1.5" title={label}>
      <Icon name={icon} size={14} label={label} />
      {dayText(date)}
    </span>
  );
}

/** The ratings shown beside TMDB's score, in Seerr's order. */
const ratingOrder = [RatingSource.ROTTEN_TOMATOES_CRITICS, RatingSource.ROTTEN_TOMATOES_AUDIENCE, RatingSource.IMDB];

/** Rotten Tomatoes' least fresh score, and least upright audience score. */
const freshScore = 60;

/** A rating linking to its page, with its logo, named on hover. */
function RatingLink({ rating: r }: { rating: Rating }) {
  const fresh = r.score >= freshScore;
  const [logo, label, score] =
    r.source === RatingSource.IMDB
      ? [
          imdbLogo,
          t("discover.imdbScore", { votes: new Intl.NumberFormat(locale, { notation: "compact", maximumFractionDigits: 1 }).format(Number(r.votes)) }),
          r.score.toFixed(1),
        ]
      : r.source === RatingSource.ROTTEN_TOMATOES_CRITICS
        ? [fresh ? rtFresh : rtRotten, r.certified ? t("discover.tomatometerCertified") : t("discover.tomatometer"), `${Math.round(r.score)}%`]
        : [fresh ? rtAudienceFresh : rtAudienceRotten, t("discover.audienceScore"), `${Math.round(r.score)}%`];
  return (
    <a href={r.url} target="_blank" rel="noreferrer" title={label} className="inline-flex items-center gap-1.5 type-callout font-semibold hover:underline">
      <img src={logo} alt={label} className="h-5 w-auto" />
      {score}
    </a>
  );
}

/** A title's page on TMDB, in the UI's language. */
function tmdbUrl(kind: MediaKind, tmdbId: number): string {
  return `https://www.themoviedb.org/${kind === MediaKind.SERIES ? "tv" : "movie"}/${tmdbId}?language=${locale}`;
}

/**
 * A title's ratings as Seerr's: Rotten Tomatoes' and IMDb's, then TMDB's
 * score when given, out of 10, each linking to its page; else the fallback
 * once there are none.
 */
export function TitleRatings({
  server,
  kind,
  tmdbId,
  score,
  fallback = null,
  className,
}: {
  server: ServerEntry;
  kind: MediaKind;
  tmdbId: number;
  score?: number;
  fallback?: ReactNode;
  className: string;
}) {
  const q = useTitleRatings(server, kind, tmdbId);
  const shown = ratingOrder.flatMap((source) => q.data?.find((r) => r.source === source) ?? []);
  if (shown.length === 0 && score === undefined) return q.isPending ? null : fallback;
  return (
    <div className={`flex flex-wrap gap-x-4 gap-y-1 ${className}`}>
      {shown.map((r) => (
        <RatingLink key={r.source} rating={r} />
      ))}
      {score !== undefined && (
        <a
          href={tmdbUrl(kind, tmdbId)}
          target="_blank"
          rel="noreferrer"
          title={t("discover.tmdbScore")}
          className="inline-flex items-center gap-1.5 type-callout font-semibold hover:underline"
        >
          <img src={tmdbWideLogo} alt="TMDB" className="h-4 w-auto" />
          {Math.round(score * 10)}%
        </a>
      )}
    </div>
  );
}

/** The logos linking a title elsewhere, as Seerr's, Rotten Tomatoes' once its ratings tell its page. */
export function useTitleLinks(server: ServerEntry, kind: MediaKind, tmdbId: number, imdb: string, tvdb: string): ExternalLink[] {
  const series = kind === MediaKind.SERIES;
  const rt = useTitleRatings(server, kind, tmdbId).data?.find((r) => r.source !== RatingSource.IMDB && r.url)?.url;
  return [
    { name: "TMDB", url: tmdbUrl(kind, tmdbId), logo: tmdbLogo, width: "w-8" },
    series && tvdb && { name: "TVDB", url: `https://www.thetvdb.com/dereferrer/series/${tvdb}`, logo: tvdbLogo, width: "w-9", mono: true },
    imdb && { name: "IMDb", url: `https://www.imdb.com/title/${imdb}`, logo: imdbLogo, width: "w-8" },
    rt && { name: "Rotten Tomatoes", url: rt, logo: rtLogo, width: "w-14" },
    imdb && { name: "Trakt", url: `https://trakt.tv/${series ? "shows" : "movies"}/${imdb}`, logo: traktLogo, width: "w-8" },
    imdb && { name: "Simkl", url: `https://api.simkl.com/redirect?to=Simkl&imdb=${encodeURIComponent(imdb)}`, logo: simklLogo, width: "w-8", mono: true },
    !series && { name: "Letterboxd", url: `https://letterboxd.com/tmdb/${tmdbId}`, logo: letterboxdLogo, width: "w-8" },
  ].filter((l) => !!l);
}

/** A link to a title or person elsewhere, with the site's logo. */
export interface ExternalLink {
  name: string;
  url: string;
  logo: string;
  /** The logo's width, as Seerr's. */
  width: string;
  /** Whether the logo is white alone, drawn in the text's color instead. */
  mono?: boolean;
}

/**
 * The logos linking a title or person elsewhere, as Seerr's, faint until
 * hovered; first, Mavio's opening its item when the server has it, where
 * Seerr links its media server.
 */
export function ExternalLinks({
  links,
  item,
  className = "justify-center",
}: {
  links: ExternalLink[];
  item?: { serverId: string; itemId: string };
  className?: string;
}) {
  const faint = "flex items-center opacity-50 transition-opacity hover:opacity-100 focus-visible:opacity-100";
  return (
    <div className={`flex flex-wrap items-center gap-x-5 gap-y-3 ${className}`}>
      {item && (
        <Link to="/s/$serverId/item/$itemId" params={item} className={faint} title={t("discover.openItem")}>
          <Wordmark height={14} />
        </Link>
      )}
      {links.map((l) => (
        <a key={l.name} href={l.url} target="_blank" rel="noreferrer" title={l.name} className={`${faint} ${l.width}`}>
          {l.mono ? (
            <span
              role="img"
              aria-label={l.name}
              className="block h-6 w-full bg-on-background"
              style={{ mask: `url("${l.logo}") center / contain no-repeat`, WebkitMask: `url("${l.logo}") center / contain no-repeat` }}
            />
          ) : (
            <img src={l.logo} alt={l.name} className="w-full" />
          )}
        </a>
      ))}
    </div>
  );
}

/** The studios shown before Show More. */
export const minStudios = 5;

function Facts({ server, title }: { server: ServerEntry; title: Title }) {
  const series = title.mediaKind === MediaKind.SERIES;
  const alike = useAlike(server, title.mediaKind);
  const [allStudios, setAllStudios] = useEntryState("studios", false);
  const status = statusNames[title.productionStatus]?.();
  const releases = series
    ? []
    : [
        title.theatricalReleaseDate && <Dated key="t" icon="movies" label={t("discover.theatricalRelease")} date={title.theatricalReleaseDate} />,
        title.digitalReleaseDate && <Dated key="d" icon="devices" label={t("item.digitalRelease")} date={title.digitalReleaseDate} />,
        title.physicalReleaseDate && <Dated key="p" icon="collections" label={t("item.physicalRelease")} date={title.physicalReleaseDate} />,
      ].filter(Boolean);
  const studios = title.studioRefs.length > 0 ? title.studioRefs : title.studios.map((name) => ({ id: "", name }));
  const shownStudios = allStudios ? studios : studios.slice(0, minStudios);
  const countries = title.productionCountries.length > 0 ? title.productionCountries.map(countryText) : title.productionLocations;
  const links = useTitleLinks(server, title.mediaKind, title.tmdbId, title.externalIds.imdb ?? "", title.externalIds.tvdb || (title.tvdbId > 0 ? String(title.tvdbId) : ""));
  return (
    <dl className="flex flex-col divide-y divide-border overflow-hidden rounded-[14px] border border-border">
      <TitleRatings
        server={server}
        kind={title.mediaKind}
        tmdbId={title.tmdbId}
        score={title.voteCount > 0 ? title.communityRating : undefined}
        className="justify-center px-4 py-2.5"
      />
      {title.originalTitle && title.originalLanguage !== discoverLanguage && (
        <Fact label={t("item.originalTitle")}>{title.originalTitle}</Fact>
      )}
      {status && <Fact label={t("item.status")}>{status}</Fact>}
      {releases.length > 0 ? (
        <Fact label={t(releases.length === 1 ? "discover.releaseDate" : "discover.releaseDates")}>{releases}</Fact>
      ) : (
        title.premiereDate && <Fact label={series ? t("item.firstAired") : t("item.released")}>{dayText(title.premiereDate)}</Fact>
      )}
      {series && title.endDate && title.productionStatus === ProductionStatus.ENDED && (
        <Fact label={t("item.lastAired")}>{dayText(title.endDate)}</Fact>
      )}
      {series && title.nextAirDate && <Fact label={t("item.nextAirDate")}>{dayText(title.nextAirDate)}</Fact>}
      {title.runtimeMinutes > 0 && (
        <Fact label={series ? t("discover.episodeRuntime") : t("item.runtime")}>{minutesText(title.runtimeMinutes)}</Fact>
      )}
      {moneyText(title.revenue) && <Fact label={t("item.revenue")}>{moneyText(title.revenue)}</Fact>}
      {moneyText(title.budget) && <Fact label={t("item.budget")}>{moneyText(title.budget)}</Fact>}
      {title.originalLanguage && (
        <Fact label={t("item.originalLanguage")}>{alike({ originalLanguage: title.originalLanguage }, languageText(title.originalLanguage))}</Fact>
      )}
      {countries.length > 0 && (
        <Fact label={t("item.country")}>
          {countries.map((c) => (
            <span key={c}>{c}</span>
          ))}
        </Fact>
      )}
      {studios.length > 0 && (
        <Fact label={series ? t("item.networks") : t("item.studio")}>
          {shownStudios.map((s) => (
            <span key={s.id || s.name}>{s.id ? alike(series ? { networks: [{ id: s.id, name: s.name }] } : { studios: [{ id: s.id, name: s.name }] }, s.name) : s.name}</span>
          ))}
          {studios.length > minStudios && (
            <button type="button" onClick={() => setAllStudios(!allStudios)} className="inline-flex items-center gap-1 hover:text-on-background">
              {allStudios ? t("action.showLess") : t("discover.showMore")}
              <Icon name="expand" size={14} className={allStudios ? "rotate-180" : ""} />
            </button>
          )}
        </Fact>
      )}
      {title.watchProviders.length > 0 && (
        <div className="flex flex-col gap-2 px-4 py-2.5">
          <dt className="type-callout font-semibold">{t("item.streaming")}</dt>
          <dd className="flex flex-wrap gap-3">
            {title.watchProviders.map((p) =>
              p.logoUrl ? (
                <img key={p.name} src={p.logoUrl} alt={p.name} title={p.name} loading="lazy" className="size-8 rounded-md opacity-70 transition-opacity hover:opacity-100" />
              ) : (
                <span key={p.name} className="type-footnote">
                  {p.name}
                </span>
              ),
            )}
          </dd>
        </div>
      )}
      <div className="px-4 py-3">
        <ExternalLinks links={links} item={title.itemId ? { serverId: server.id, itemId: title.itemId } : undefined} />
      </div>
    </dl>
  );
}

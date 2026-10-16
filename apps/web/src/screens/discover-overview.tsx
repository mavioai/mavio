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

import imdbLogo from "../assets/ratings/imdb.svg";
import rtAudienceFresh from "../assets/ratings/rt-aud-fresh.svg";
import rtAudienceRotten from "../assets/ratings/rt-aud-rotten.svg";
import rtFresh from "../assets/ratings/rt-fresh.svg";
import rtRotten from "../assets/ratings/rt-rotten.svg";
import { discoverLanguage, locale, t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { minutesText } from "../ui.tsx";
import { crewLabels, isCast } from "./credits.tsx";
import { kindParam, sized } from "./discover.tsx";
import { dayText, languageText, moneyText } from "./facts.tsx";

const regions = new Intl.DisplayNames(locale, { type: "region" });

/** The name of an ISO 3166-1 country in the UI's language, after its flag. */
function countryText(code: string): string {
  const flag = /^[A-Z]{2}$/.test(code) ? String.fromCodePoint(...[...code].map((c) => 0x1f1a5 + c.charCodeAt(0))) : "";
  let name = code;
  try {
    name = regions.of(code) ?? code;
  } catch {
    // An unknown code names itself.
  }
  return flag ? `${flag} ${name}` : name;
}

const statusNames: Partial<Record<ProductionStatus, () => string>> = {
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
const crewRank = (c: { role: string; kind: CreditKind }) => {
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
  const crew = title.credits
    .filter((c) => !isCast(c) && crewLabels.some(([k]) => k === c.kind))
    .map((c, i) => ({ c, i }))
    .toSorted((a, b) => crewRank(a.c) - crewRank(b.c) || a.i - b.i)
    .slice(0, 6)
    .map(({ c }) => c);
  const keywords = title.keywordRefs.length > 0 ? title.keywordRefs : title.keywords.map((name) => ({ id: "", name }));
  return (
    <div className="flex min-w-0 flex-col gap-4">
      {title.tagline && <p className="type-title3 font-normal text-text-secondary italic">{title.tagline}</p>}
      <h2 className="type-title3">{t("discover.overview")}</h2>
      <p className="type-body whitespace-pre-line">{title.overview || t("discover.noOverview")}</p>
      {crew.length > 0 && (
        <>
          <ul className="grid grid-cols-2 gap-x-6 gap-y-4 pt-2 sm:grid-cols-3">
            {crew.map((c, i) => (
              <li key={`${c.name}-${i}`} className="flex min-w-0 flex-col">
                <span className="type-headline">{c.kind === CreditKind.OTHER && c.role ? c.role : t(crewLabels.find(([k]) => k === c.kind)![1])}</span>
                {c.externalIds.tmdb ? (
                  <Link
                    to="/s/$serverId/discover/person/$tmdbId"
                    params={{ serverId: server.id, tmdbId: c.externalIds.tmdb }}
                    className="truncate type-callout text-text-secondary hover:text-on-background hover:underline"
                  >
                    {c.name}
                  </Link>
                ) : (
                  <span className="truncate type-callout text-text-secondary">{c.name}</span>
                )}
              </li>
            ))}
          </ul>
          <Link
            to="/s/$serverId/title/$kind/$tmdbId/credits"
            params={{ serverId: server.id, kind: kindParam(title.mediaKind), tmdbId: String(title.tmdbId) }}
            className="inline-flex items-center gap-1 self-end type-callout text-text-secondary hover:text-on-background"
          >
            {t("discover.fullCrew")}
            <Icon name="forward" size={16} />
          </Link>
        </>
      )}
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
function Dated({ icon, label, date }: { icon: IconName; label: string; date: string }) {
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

const minStudios = 5;

function Facts({ server, title }: { server: ServerEntry; title: Title }) {
  const series = title.mediaKind === MediaKind.SERIES;
  const alike = useAlike(server, title.mediaKind);
  const [allStudios, setAllStudios] = useState(false);
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
  const tmdbPath = `https://www.themoviedb.org/${series ? "tv" : "movie"}/${title.tmdbId}`;
  const ratings = useTitleRatings(server, title.mediaKind, title.tmdbId).data ?? [];
  const shownRatings = ratingOrder.flatMap((source) => ratings.find((r) => r.source === source) ?? []);
  const links = [
    { name: "TMDB", url: tmdbPath },
    title.externalIds.imdb && { name: "IMDb", url: `https://www.imdb.com/title/${title.externalIds.imdb}` },
    (title.externalIds.tvdb || title.tvdbId > 0) && {
      name: "TVDB",
      url: `https://www.thetvdb.com/dereferrer/series/${title.externalIds.tvdb || title.tvdbId}`,
    },
  ].filter((l) => !!l);
  return (
    <dl className="flex flex-col divide-y divide-border overflow-hidden rounded-[14px] border border-border">
      {(title.voteCount > 0 || shownRatings.length > 0) && (
        <div className="flex flex-wrap justify-center gap-x-4 gap-y-1 px-4 py-2.5">
          {shownRatings.map((r) => (
            <RatingLink key={r.source} rating={r} />
          ))}
          {title.voteCount > 0 && (
            <a
              href={`${tmdbPath}?language=${locale}`}
              target="_blank"
              rel="noreferrer"
              title={t("discover.tmdbScore")}
              className="inline-flex items-center gap-1.5 type-callout font-semibold hover:underline"
            >
              <span className="rounded-[4px] bg-[#0d253f] px-1.5 type-caption font-bold text-[#01b4e4]">TMDB</span>
              {Math.round(title.communityRating * 10)}%
            </a>
          )}
        </div>
      )}
      {title.originalTitle && title.originalLanguage !== discoverLanguage && (
        <Fact label={t("item.originalTitle")}>{title.originalTitle}</Fact>
      )}
      {status && <Fact label={t("item.status")}>{status}</Fact>}
      {releases.length > 0 ? (
        <Fact label={t("discover.releaseDates", { count: releases.length })}>{releases}</Fact>
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
      <div className="flex flex-wrap justify-center gap-2 px-4 py-2.5">
        {links.map((l) => (
          <a key={l.name} href={l.url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 rounded-full bg-fill px-3 py-1 type-footnote font-semibold hover:bg-fill-selected">
            {l.name}
            <Icon name="external" size={12} />
          </a>
        ))}
      </div>
    </dl>
  );
}

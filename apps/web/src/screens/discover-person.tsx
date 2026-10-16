// A discovered person's page (docs/clients.md §6.3.2), as Seerr's, opened
// from the cast and crew of a discovered title: their photo, dates, other
// names, biography and links, then the movies and series they play in and
// work on, marked with what the server has of each, the department they
// are known for first.
import { errorMessage, isNotFound, type ServerEntry, useDiscoveredPerson, useServer } from "@mavio/client-ts";
import type { Person, PersonTitle } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { getRouteApi } from "@tanstack/react-router";
import { useState } from "react";

import imdbLogo from "../assets/services/imdb.svg";
import tmdbLogo from "../assets/services/tmdb.svg";
import { discoverLanguage, locale, t } from "../i18n.ts";
import { Grid, Loading, NotFoundView, StatusView } from "../ui.tsx";
import { Segmented } from "../controls/inputs.tsx";
import { sized, TitleCard } from "./discover.tsx";
import { type ExternalLink, ExternalLinks } from "./discover-overview.tsx";
import { initialsOf } from "./person.tsx";
import { useEntryState } from "../shell/entry-state.ts";

const route = getRouteApi("/s/$serverId/discover/person/$tmdbId");

const dateFormat = new Intl.DateTimeFormat(locale, { dateStyle: "long", timeZone: "UTC" });

type Kinds = "all" | "movie" | "series";

export function DiscoveredPersonScreen() {
  const { serverId, tmdbId } = route.useParams();
  const server = useServer(serverId);
  const q = useDiscoveredPerson(server?.token ? server : undefined, Number(tmdbId), discoverLanguage);
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  if (q.isPending) return <Loading />;
  if ((q.isError && isNotFound(q.error)) || (q.isSuccess && !q.data)) return <NotFoundView />;
  if (q.isError || !q.data) {
    return <StatusView icon="error" title={t("state.error")} message={q.error ? errorMessage(q.error) : undefined} />;
  }
  return <PersonPage key={tmdbId} server={server} person={q.data} />;
}

/** A date of the calendar, as TMDB gives it. */
function day(iso: string): string | undefined {
  return iso ? dateFormat.format(new Date(`${iso}T00:00:00Z`)) : undefined;
}

/** Whole years between two calendar dates. */
function age(from: string, to: string): number {
  const [a, b] = [new Date(`${from}T00:00:00Z`), to ? new Date(`${to}T00:00:00Z`) : new Date()];
  const years = b.getUTCFullYear() - a.getUTCFullYear();
  const before = b.getUTCMonth() < a.getUTCMonth() || (b.getUTCMonth() === a.getUTCMonth() && b.getUTCDate() < a.getUTCDate());
  return before ? years - 1 : years;
}

function PersonPage({ server, person: p }: { server: ServerEntry; person: Person }) {
  // The kind shown is kept in the address, which Back and a reload return to.
  const kinds: Kinds = route.useSearch().kind ?? "all";
  const navigate = route.useNavigate();
  const setKinds = (k: Kinds) => void navigate({ search: { kind: k === "all" ? undefined : k }, replace: true, resetScroll: false });
  const [more, setMore] = useEntryState("biography", false);
  const of = (ts: PersonTitle[]) =>
    ts.filter((pt) => kinds === "all" || (pt.title?.mediaKind === MediaKind.SERIES) === (kinds === "series"));
  const facts = [
    p.birthday && t("person.born", { date: day(p.birthday)!, age: age(p.birthday, p.deathday) }),
    p.placeOfBirth,
    p.deathday && t("person.died", { date: day(p.deathday)! }),
  ].filter(Boolean);
  const links: ExternalLink[] = [
    p.externalIds.tmdb && { name: "TMDB", url: `https://www.themoviedb.org/person/${p.externalIds.tmdb}?language=${locale}`, logo: tmdbLogo, width: "w-8" },
    p.externalIds.imdb && { name: "IMDb", url: `https://www.imdb.com/name/${p.externalIds.imdb}`, logo: imdbLogo, width: "w-8" },
  ].filter((l) => !!l);
  const cast = <Titles key="cast" server={server} title={t("person.appearances")} titles={of(p.cast)} />;
  const crew = <Titles key="crew" server={server} title={t("person.crew")} titles={of(p.crew)} />;
  return (
    <div className="pb-10">
      <header className="flex flex-col items-center gap-6 px-margin pt-8 sm:flex-row sm:items-start">
        <div className="relative aspect-square w-36 shrink-0 overflow-hidden rounded-full bg-fill sm:w-44">
          {p.imageUrl ? (
            <img src={sized(p.imageUrl, 342)} alt="" className="h-full w-full object-cover" />
          ) : (
            <span className="absolute inset-0 flex items-center justify-center type-display text-text-secondary">{initialsOf(p.name)}</span>
          )}
        </div>
        <div className="flex w-full min-w-0 max-w-3xl flex-col gap-3">
          <h1 className="type-title1">{p.name}</h1>
          {facts.length > 0 && <p className="type-callout text-text-secondary">{facts.join(" | ")}</p>}
          {p.alsoKnownAs.length > 0 && (
            <p className="type-callout text-text-secondary">{t("person.alsoKnownAs", { names: p.alsoKnownAs.join(", ") })}</p>
          )}
          {links.length > 0 && <ExternalLinks links={links} className="justify-center sm:justify-start" />}
          {p.biography && (
            <div className="flex flex-col items-start gap-1">
              <p className={`type-body whitespace-pre-line ${more ? "" : "line-clamp-6"}`}>{p.biography}</p>
              <button type="button" className="type-callout font-semibold text-accent-text hover:underline" onClick={() => setMore(!more)}>
                {more ? t("person.less") : t("person.more")}
              </button>
            </div>
          )}
          <Segmented
            value={kinds}
            onChange={setKinds}
            label={t("person.kinds")}
            className="self-start"
            options={[
              { value: "all", label: t("person.allKinds") },
              { value: "movie", label: t("person.movies") },
              { value: "series", label: t("person.series") },
            ]}
          />
        </div>
      </header>
      {p.knownForDepartment === "Acting" || !p.knownForDepartment ? [cast, crew] : [crew, cast]}
    </div>
  );
}

function Titles({ server, title, titles }: { server: ServerEntry; title: string; titles: PersonTitle[] }) {
  if (titles.length === 0) return null;
  return (
    <section className="pt-8">
      <h2 className="px-margin pb-3 type-title3">
        {title} <span className="type-callout text-text-secondary">{titles.length}</span>
      </h2>
      <Grid>
        {titles.map((pt, i) => pt.title && <TitleCard key={i} serverId={server.id} title={pt.title} role={pt.role} fluid />)}
      </Grid>
    </section>
  );
}

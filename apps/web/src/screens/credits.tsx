// The cast and crew of an item or a discovered title (libs/design/
// components.md, Credits Screen): the shelf on their details, and the page
// of the whole cast and crew it opens, the cast in billing order, then the
// crew by job.
import { artwork, errorMessage, type ServerEntry, useItem, useServer, useTitle } from "@mavio/client-ts";
import type { TitleCredit } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { CreditKind, type Credit, ImageKind } from "@mavio/proto/mavio/library/v1/item_pb";
import type { MessageKey } from "@mavio/design/strings";
import { getRouteApi, Link } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { discoverLanguage, t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { Loading, Shelf, StatusView } from "../ui.tsx";
import { initialsOf } from "./person.tsx";

const itemRoute = getRouteApi("/s/$serverId/item/$itemId/credits");
const titleRoute = getRouteApi("/s/$serverId/title/$kind/$tmdbId/credits");

/** A credited person: of the server's people when personId is set, else as a metadata provider tells them. */
export interface CreditEntry {
  name: string;
  photo?: string;
  kind: CreditKind;
  role: string;
  order: number;
  personId?: string;
  /** Their TMDB ID, opening their discovered page when they are not of the server. */
  tmdbId?: string;
}

/** An item's credits, of people of the server. */
export function libraryCredits(server: ServerEntry, credits: Credit[]): CreditEntry[] {
  return credits
    .filter((c) => c.person)
    .map((c) => ({
      name: c.person!.name,
      photo: artwork(server, c.person!, ImageKind.PRIMARY, 240)?.url,
      kind: c.kind,
      role: c.role,
      order: c.order,
      personId: c.person!.id,
    }));
}

/**
 * A discovered title's credits, their photos at TMDB's 185-pixel width.
 * An actor playing several characters is listed once, as on Seerr, as the
 * first of them, the one they play in the most episodes.
 */
export function titleCredits(credits: TitleCredit[]): CreditEntry[] {
  const seen = new Set<string>();
  const once = credits.filter((c) => {
    if (!isCast(c)) return true;
    const key = c.externalIds.tmdb || c.name;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
  return once.map((c) => ({
    name: c.name,
    photo: c.imageUrl ? c.imageUrl.replace(/(image\.tmdb\.org\/t\/p\/)original\//, "$1w185/") : undefined,
    kind: c.kind,
    role: c.role,
    order: c.order,
    tmdbId: c.externalIds.tmdb,
  }));
}

/** Whether a credit is of the cast rather than the crew. */
export const isCast = (c: { kind: CreditKind }) => c.kind === CreditKind.ACTOR || c.kind === CreditKind.GUEST_STAR;

/** The crew's jobs, in the order the page lists them. */
export const crewLabels: [CreditKind, MessageKey][] = [
  [CreditKind.CREATOR, "item.creators"],
  [CreditKind.DIRECTOR, "item.director"],
  [CreditKind.WRITER, "item.writers"],
  [CreditKind.AUTHOR, "credits.authors"],
  [CreditKind.PRODUCER, "item.producers"],
  [CreditKind.COMPOSER, "item.music"],
  [CreditKind.CONDUCTOR, "credits.conductors"],
  [CreditKind.LYRICIST, "credits.lyricists"],
  [CreditKind.ARTIST, "credits.artists"],
  [CreditKind.NARRATOR, "credits.narrators"],
  [CreditKind.OTHER, "credits.other"],
];

const shelfCrew = new Set([CreditKind.CREATOR, CreditKind.DIRECTOR, CreditKind.WRITER]);

/** The cast in billing order, then the creators, directors and writers, with See All opening the whole cast and crew. */
export function CastShelf({ serverId, credits, all }: { serverId: string; credits: CreditEntry[]; all: ReactNode }) {
  const shown = credits.filter((c) => isCast(c) || shelfCrew.has(c.kind));
  if (shown.length === 0) return null;
  const sorted = shown.toSorted((a, b) => Number(!isCast(a)) - Number(!isCast(b)) || a.order - b.order);
  const label = (c: CreditEntry) => (isCast(c) ? c.role : t(crewLabels.find(([k]) => k === c.kind)![1]));
  return (
    <Shelf title={t("item.cast")} more={all}>
      {sorted.map((c, i) => (
        <PersonTile
          key={`${c.personId ?? c.name}-${i}`}
          serverId={serverId}
          credit={c}
          label={label(c)}
          className="w-28 shrink-0 snap-start [[data-tv]_&]:w-40"
        />
      ))}
    </Shelf>
  );
}

export function CreditsScreen() {
  const { serverId, itemId } = itemRoute.useParams();
  const server = useServer(serverId);
  const q = useItem(server?.token ? server : undefined, itemId);
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  if (q.isPending) return <Loading />;
  if (q.isError || !q.data.item) {
    return <StatusView icon="error" title={t("state.error")} action={{ label: t("action.retry"), onClick: () => void q.refetch() }} />;
  }
  return (
    <CreditsPage
      serverId={serverId}
      credits={libraryCredits(server, q.data.credits)}
      back={
        <Link to="/s/$serverId/item/$itemId" params={{ serverId, itemId }} className={backClass}>
          <Icon name="back" size={16} />
          {q.data.item.name}
        </Link>
      }
    />
  );
}

export function TitleCreditsScreen() {
  const { serverId, kind, tmdbId } = titleRoute.useParams();
  const server = useServer(serverId);
  const q = useTitle(server?.token ? server : undefined, kind === "series" ? MediaKind.SERIES : MediaKind.MOVIE, Number(tmdbId), discoverLanguage);
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  if (q.isPending) return <Loading />;
  if (q.isError || !q.data) {
    return <StatusView icon="error" title={t("state.error")} message={q.error ? errorMessage(q.error) : undefined} />;
  }
  return (
    <CreditsPage
      serverId={serverId}
      credits={titleCredits(q.data.credits)}
      back={
        <Link to="/s/$serverId/title/$kind/$tmdbId" params={{ serverId, kind, tmdbId }} className={backClass}>
          <Icon name="back" size={16} />
          {q.data.name}
        </Link>
      }
    />
  );
}

const backClass = "inline-flex items-center gap-1 type-callout text-text-secondary hover:text-on-background";

function CreditsPage({ serverId, credits, back }: { serverId: string; credits: CreditEntry[]; back: ReactNode }) {
  const of = (kind: CreditKind) => credits.filter((c) => c.kind === kind).toSorted((a, b) => a.order - b.order);
  return (
    <div className="pb-10">
      <div className="px-margin pt-6 pb-2">
        {back}
        <h1 className="mt-1 type-title1">{t("item.cast")}</h1>
      </div>
      {credits.length === 0 && <p className="px-margin pt-6 type-body text-text-secondary">{t("credits.none")}</p>}
      <People serverId={serverId} title={t("credits.cast")} credits={of(CreditKind.ACTOR)} />
      <People serverId={serverId} title={t("credits.guestStars")} credits={of(CreditKind.GUEST_STAR)} />
      {crewLabels.map(([kind, label]) => (
        <People key={kind} serverId={serverId} title={t(label)} credits={of(kind)} />
      ))}
    </div>
  );
}

/** A heading over its people, in a grid. */
function People({ serverId, title, credits }: { serverId: string; title: string; credits: CreditEntry[] }) {
  if (credits.length === 0) return null;
  return (
    <section className="pt-6">
      <h2 className="px-margin pb-3 type-title3">{title}</h2>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(7rem,1fr))] gap-x-4 gap-y-6 px-margin [[data-tv]_&]:grid-cols-[repeat(auto-fill,minmax(10rem,1fr))]">
        {credits.map((c, i) => (
          <PersonTile key={`${c.personId ?? c.name}-${i}`} serverId={serverId} credit={c} label={c.role} />
        ))}
      </div>
    </section>
  );
}

/**
 * A credited person: their photo, else their initials, their name, and
 * under it a label, such as the character they play or their job. People
 * of the server open their page, others their discovered page.
 */
function PersonTile({ serverId, credit: c, label, className = "" }: { serverId: string; credit: CreditEntry; label: string; className?: string }) {
  const body = (
    <>
      <div className="relative mx-auto aspect-square w-24 overflow-hidden rounded-full bg-fill [[data-tv]_&]:w-36">
        {c.photo ? (
          <img src={c.photo} alt="" loading="lazy" className="h-full w-full object-cover" />
        ) : (
          <span className="absolute inset-0 flex items-center justify-center type-title3 text-text-secondary">{initialsOf(c.name)}</span>
        )}
      </div>
      <div className="mt-2 line-clamp-2 type-callout group-hover:underline">{c.name}</div>
      <div className="line-clamp-2 type-footnote text-text-secondary">{label}</div>
    </>
  );
  if (!c.personId && c.tmdbId) {
    return (
      <Link to="/s/$serverId/discover/person/$tmdbId" params={{ serverId, tmdbId: c.tmdbId }} className={`group rounded-lg text-center ${className}`}>
        {body}
      </Link>
    );
  }
  if (!c.personId) return <div className={`text-center ${className}`}>{body}</div>;
  return (
    <Link to="/s/$serverId/person/$personId" params={{ serverId, personId: c.personId }} className={`group rounded-lg text-center ${className}`}>
      {body}
    </Link>
  );
}

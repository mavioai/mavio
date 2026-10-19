// The cast and crew of an item or a discovered title (libs/design/
// components.md, Credits Screen): the shelf on their details, and the page
// of the whole cast and crew it opens, the cast in billing order, then the
// crew by job; and an item's or a discovered title's crew alone, by job.
import { artwork, errorMessage, isNotFound, type ServerEntry, useItem, useServer, useTitle } from "@mavio/client-ts";
import type { TitleCredit } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { CreditKind, type Credit, ImageKind } from "@mavio/proto/mavio/library/v1/item_pb";
import type { MessageKey } from "@mavio/design/strings";
import { getRouteApi, Link } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { discoverLanguage, t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { usePrefs } from "../prefs.ts";
import { Artwork, Loading, NotFoundView, Shelf, StatusView } from "../ui.tsx";

const itemRoute = getRouteApi("/s/$serverId/item/$itemId/credits");
const itemCrewRoute = getRouteApi("/s/$serverId/item/$itemId/crew");
const titleRoute = getRouteApi("/s/$serverId/title/$kind/$tmdbId/credits");
const crewRoute = getRouteApi("/s/$serverId/title/$kind/$tmdbId/crew");

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

/**
 * An item's credits, of people of the server. An actor credited for
 * several characters is listed once, as a discovered title's, as the first
 * of them, the one the metadata providers tell they play the longest.
 */
export function libraryCredits(server: ServerEntry, credits: Credit[]): CreditEntry[] {
  const seen = new Set<string>();
  return credits
    .filter((c) => {
      if (!c.person) return false;
      if (!isCast(c)) return true;
      if (seen.has(c.person.id)) return false;
      seen.add(c.person.id);
      return true;
    })
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

/**
 * The cast in billing order, then the creators, directors and writers,
 * with See All opening the whole cast and crew: all draws its link around
 * what it is given, beside the title, or on the television as the row's
 * last card, as the other rows' See All.
 */
export function CastShelf({
  serverId,
  credits,
  all,
}: {
  serverId: string;
  credits: CreditEntry[];
  all: (className: string | undefined, children: ReactNode, card?: boolean) => ReactNode;
}) {
  const { tv } = usePrefs();
  const shown = credits.filter((c) => isCast(c) || shelfCrew.has(c.kind));
  if (shown.length === 0) return null;
  const sorted = shown.toSorted((a, b) => Number(!isCast(a)) - Number(!isCast(b)) || a.order - b.order);
  const label = (c: CreditEntry) => (isCast(c) ? c.role : t(crewLabels.find(([k]) => k === c.kind)![1]));
  return (
    <Shelf title={t("item.cast")} more={tv ? undefined : all(undefined, t("credits.all"))}>
      {sorted.map((c, i) => (
        <PersonTile
          key={`${c.personId ?? c.name}-${i}`}
          serverId={serverId}
          credit={c}
          label={label(c)}
          className="w-28 shrink-0 snap-start [[data-tv]_&]:w-40"
        />
      ))}
      {tv &&
        all(
          "group w-40 shrink-0 snap-start rounded-lg text-center",
          <>
            <div data-art className="mx-auto flex aspect-square w-36 items-center justify-center rounded-full bg-fill">
              <Icon name="forward" size={40} />
            </div>
            <div className="mt-2 type-callout">{t("credits.all")}</div>
          </>,
          true,
        )}
    </Shelf>
  );
}

export function CreditsScreen() {
  return <ItemCredits params={itemRoute.useParams()} />;
}

/** An item's crew alone, which View Full Crew opens. */
export function ItemCrewScreen() {
  return <ItemCredits params={itemCrewRoute.useParams()} crew />;
}

function ItemCredits({ params: { serverId, itemId }, crew = false }: { params: { serverId: string; itemId: string }; crew?: boolean }) {
  const server = useServer(serverId);
  const q = useItem(server?.token ? server : undefined, itemId);
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  if (q.isPending) return <Loading />;
  if ((q.isError && isNotFound(q.error)) || (q.isSuccess && !q.data.item)) return <NotFoundView />;
  if (q.isError || !q.data.item) {
    return <StatusView icon="error" title={t("state.error")} action={{ label: t("action.retry"), onClick: () => void q.refetch() }} />;
  }
  const credits = libraryCredits(server, q.data.credits);
  return (
    <CreditsPage
      serverId={serverId}
      title={crew ? t("credits.fullCrew") : undefined}
      credits={crew ? credits.filter((c) => !isCast(c)) : credits}
    />
  );
}

export function TitleCreditsScreen() {
  return <TitleCredits params={titleRoute.useParams()} />;
}

/** A discovered title's crew alone, which View Full Crew opens, as Seerr's crew page. */
export function TitleCrewScreen() {
  return <TitleCredits params={crewRoute.useParams()} crew />;
}

function TitleCredits({ params: { serverId, kind, tmdbId }, crew = false }: { params: { serverId: string; kind: string; tmdbId: string }; crew?: boolean }) {
  const server = useServer(serverId);
  const q = useTitle(server?.token ? server : undefined, kind === "series" ? MediaKind.SERIES : MediaKind.MOVIE, Number(tmdbId), discoverLanguage);
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  if (q.isPending) return <Loading />;
  if ((q.isError && isNotFound(q.error)) || (q.isSuccess && !q.data)) return <NotFoundView />;
  if (q.isError || !q.data) {
    return <StatusView icon="error" title={t("state.error")} message={q.error ? errorMessage(q.error) : undefined} />;
  }
  const credits = titleCredits(q.data.credits);
  return (
    <CreditsPage
      serverId={serverId}
      title={crew ? t("credits.fullCrew") : undefined}
      credits={crew ? credits.filter((c) => !isCast(c)) : credits}
    />
  );
}

function CreditsPage({ serverId, title, credits }: { serverId: string; title?: string; credits: CreditEntry[] }) {
  const of = (kind: CreditKind) => credits.filter((c) => c.kind === kind).toSorted((a, b) => a.order - b.order);
  return (
    <div className="pb-10">
      <div className="px-margin pt-6 pb-2 [main:has([data-back-row])_&]:pt-0">
        <h1 className="type-title1">{title ?? t("item.cast")}</h1>
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
      <Artwork src={c.photo || undefined} alt="" icon="profile" className="mx-auto aspect-square w-24 rounded-full group-focus-visible:outline-2 group-focus-visible:outline-offset-1 group-focus-visible:outline-focus [[data-tv]_&]:w-36" />
      <div className="mt-2 line-clamp-2 type-callout group-hover:underline">{c.name}</div>
      <div className="line-clamp-2 type-footnote text-text-secondary">{label}</div>
    </>
  );
  if (!c.personId && c.tmdbId) {
    return (
      <Link to="/s/$serverId/discover/person/$tmdbId" params={{ serverId, tmdbId: c.tmdbId }} data-card className={`group rounded-lg text-center focus-visible:outline-none ${className}`}>
        {body}
      </Link>
    );
  }
  if (!c.personId) return <div className={`text-center ${className}`}>{body}</div>;
  return (
    <Link to="/s/$serverId/person/$personId" params={{ serverId, personId: c.personId }} data-card className={`group rounded-lg text-center focus-visible:outline-none ${className}`}>
      {body}
    </Link>
  );
}

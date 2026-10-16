// An item's whole cast and crew (libs/design/components.md, Credits
// Screen), opened from the cast on its details: the cast in billing order,
// then the crew by job.
import { artwork, type ServerEntry, useItem, useServer } from "@mavio/client-ts";
import { CreditKind, type Credit, ImageKind } from "@mavio/proto/mavio/library/v1/item_pb";
import type { MessageKey } from "@mavio/design/strings";
import { getRouteApi, Link } from "@tanstack/react-router";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { Loading, StatusView } from "../ui.tsx";
import { initialsOf } from "./person.tsx";

const route = getRouteApi("/s/$serverId/item/$itemId/credits");

/** Whether a credit is of the cast rather than the crew. */
export const isCast = (c: Credit) => c.kind === CreditKind.ACTOR || c.kind === CreditKind.GUEST_STAR;

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

export function CreditsScreen() {
  const { serverId, itemId } = route.useParams();
  const server = useServer(serverId);
  const q = useItem(server?.token ? server : undefined, itemId);
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  if (q.isPending) return <Loading />;
  if (q.isError || !q.data.item) {
    return <StatusView icon="error" title={t("state.error")} action={{ label: t("action.retry"), onClick: () => void q.refetch() }} />;
  }
  const item = q.data.item;
  const credits = q.data.credits.filter((c) => c.person);
  const byOrder = (a: Credit, b: Credit) => a.order - b.order;
  const cast = credits.filter((c) => c.kind === CreditKind.ACTOR).toSorted(byOrder);
  const guests = credits.filter((c) => c.kind === CreditKind.GUEST_STAR).toSorted(byOrder);
  return (
    <div className="pb-10">
      <div className="px-margin pt-6 pb-2">
        <Link
          to="/s/$serverId/item/$itemId"
          params={{ serverId, itemId }}
          className="inline-flex items-center gap-1 type-callout text-text-secondary hover:text-on-background"
        >
          <Icon name="back" size={16} />
          {item.name}
        </Link>
        <h1 className="mt-1 type-title1">{t("item.cast")}</h1>
      </div>
      {credits.length === 0 && <p className="px-margin pt-6 type-body text-text-secondary">{t("credits.none")}</p>}
      <People server={server} title={t("credits.cast")} credits={cast} />
      <People server={server} title={t("credits.guestStars")} credits={guests} />
      {crewLabels.map(([kind, label]) => (
        <People key={kind} server={server} title={t(label)} credits={credits.filter((c) => c.kind === kind).toSorted(byOrder)} />
      ))}
    </div>
  );
}

/** A heading over its people, in a grid. */
function People({ server, title, credits }: { server: ServerEntry; title: string; credits: Credit[] }) {
  if (credits.length === 0) return null;
  return (
    <section className="pt-6">
      <h2 className="px-margin pb-3 type-title3">{title}</h2>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(7rem,1fr))] gap-x-4 gap-y-6 px-margin [[data-tv]_&]:grid-cols-[repeat(auto-fill,minmax(10rem,1fr))]">
        {credits.map((c, i) => (
          <PersonTile key={`${c.person!.id}-${i}`} server={server} credit={c} label={c.role} />
        ))}
      </div>
    </section>
  );
}

/** A credited person: their photo, else their initials, their name, and under it a label, such as the character they play or their job. */
export function PersonTile({ server, credit: c, label, className = "" }: { server: ServerEntry; credit: Credit; label: string; className?: string }) {
  const p = c.person!;
  const photo = artwork(server, p, ImageKind.PRIMARY, 240);
  return (
    <Link
      to="/s/$serverId/person/$personId"
      params={{ serverId: server.id, personId: p.id }}
      className={`group rounded-lg text-center ${className}`}
    >
      <div className="relative mx-auto aspect-square w-24 overflow-hidden rounded-full bg-fill [[data-tv]_&]:w-36">
        {photo ? (
          <img src={photo.url} alt="" loading="lazy" className="h-full w-full object-cover" />
        ) : (
          <span className="absolute inset-0 flex items-center justify-center type-title3 text-text-secondary">{initialsOf(p.name)}</span>
        )}
      </div>
      <div className="mt-2 line-clamp-2 type-callout group-hover:underline">{p.name}</div>
      <div className="line-clamp-2 type-footnote text-text-secondary">{label}</div>
    </Link>
  );
}

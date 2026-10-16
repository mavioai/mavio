// A person's page (docs/clients.md §6.3.19), opened from the cast on an
// item's details: their photo, dates, biography and links, and the movies,
// series and episodes of the server crediting them.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { artwork, errorMessage, type ServerEntry, usePerson, usePersonItems, useServer } from "@mavio/client-ts";
import { ImageKind, type Item, ItemKind, type Person } from "@mavio/proto/mavio/library/v1/item_pb";
import { getRouteApi } from "@tanstack/react-router";
import { useState } from "react";

import { locale, t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { usePrefs } from "../prefs.ts";
import { Grid, ItemCard, Loading, StatusView } from "../ui.tsx";

const route = getRouteApi("/s/$serverId/person/$personId");

const dateFormat = new Intl.DateTimeFormat(locale, { dateStyle: "long" });

export function PersonScreen() {
  const { serverId, personId } = route.useParams();
  const server = useServer(serverId);
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  return <PersonPage key={personId} server={server} personId={personId} />;
}

/** The person's initials, shown without a photo. */
export function initialsOf(name: string): string {
  return name
    .split(/\s+/)
    .map((w) => w[0])
    .slice(0, 2)
    .join("");
}

/** Whole years between two dates. */
function yearsBetween(from: Date, to: Date): number {
  const years = to.getFullYear() - from.getFullYear();
  const before = to.getMonth() < from.getMonth() || (to.getMonth() === from.getMonth() && to.getDate() < from.getDate());
  return before ? years - 1 : years;
}

function PersonPage({ server, personId }: { server: ServerEntry; personId: string }) {
  const person = usePerson(server, personId);
  const items = usePersonItems(server, personId);
  if (person.isPending) return <Loading />;
  if (person.isError || !person.data) {
    return <StatusView icon="error" title={t("state.error")} action={{ label: t("action.retry"), onClick: () => void person.refetch() }} />;
  }
  const all = items.data ?? [];
  const of = (kind: ItemKind) => all.filter((i) => i.kind === kind);
  return (
    <div className="pb-10">
      <Header server={server} person={person.data} />
      <Titles server={server} title={t("person.movies")} items={of(ItemKind.MOVIE)} />
      <Titles server={server} title={t("person.series")} items={of(ItemKind.SERIES)} />
      <Titles server={server} title={t("person.episodes")} items={of(ItemKind.EPISODE)} thumb />
      {items.isError && <p className="px-margin pt-8 type-body break-words text-danger">{errorMessage(items.error)}</p>}
      {items.isSuccess && all.length === 0 && <p className="px-margin pt-8 type-body text-text-secondary">{t("person.none")}</p>}
    </div>
  );
}

function Header({ server, person: p }: { server: ServerEntry; person: Person }) {
  const [more, setMore] = useState(false);
  const { tv } = usePrefs();
  const photo = artwork(server, p, ImageKind.PRIMARY, 480);
  const born = p.birthDate && timestampDate(p.birthDate);
  const died = p.deathDate && timestampDate(p.deathDate);
  const facts = [
    born && t("person.born", { date: dateFormat.format(born), age: yearsBetween(born, died || new Date()) }),
    p.birthPlace,
    died && t("person.died", { date: dateFormat.format(died) }),
  ].filter(Boolean);
  return (
    <header className="flex flex-col gap-6 px-margin pt-8 sm:flex-row sm:items-start">
      <div className="relative aspect-[2/3] w-40 shrink-0 overflow-hidden rounded-[14px] bg-fill sm:w-52">
        {photo ? (
          <img src={photo.url} alt="" className="h-full w-full object-cover" />
        ) : (
          <span className="absolute inset-0 flex items-center justify-center type-display text-text-secondary">{initialsOf(p.name)}</span>
        )}
      </div>
      <div className="flex min-w-0 max-w-3xl flex-col gap-3">
        <h1 className="type-title1">{p.name}</h1>
        {facts.map((f) => (
          <p key={String(f)} className="type-callout text-text-secondary">
            {f}
          </p>
        ))}
        {p.overview && (
          <div className="flex flex-col items-start gap-1">
            <p className={`type-body whitespace-pre-line ${more ? "" : "line-clamp-5"}`}>{p.overview}</p>
            <button type="button" className="type-callout font-semibold text-accent-text hover:underline" onClick={() => setMore(!more)}>
              {more ? t("person.less") : t("person.more")}
            </button>
          </div>
        )}
        {/* A television opens no web pages. */}
        {p.externalUrls.length > 0 && !tv && (
          <div className="flex flex-wrap gap-2 pt-1">
            {p.externalUrls.map((l) => (
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
    </header>
  );
}

function Titles({ server, title, items, thumb = false }: { server: ServerEntry; title: string; items: Item[]; thumb?: boolean }) {
  if (items.length === 0) return null;
  return (
    <section className="pt-8">
      <h2 className="px-margin pb-3 type-title3">
        {title} <span className="type-callout text-text-secondary">{items.length}</span>
      </h2>
      <Grid shape={thumb ? "thumb" : "poster"}>
        {items.map((i) => (
          <ItemCard key={i.id} server={server} serverId={server.id} item={i} shape={thumb ? "thumb" : undefined} fluid />
        ))}
      </Grid>
    </section>
  );
}

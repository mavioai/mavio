// An item's details: the header with its actions, its versions on this and
// the user's other servers, its seasons, episodes or tracks, its cast and
// items like it (libs/design/components.md).
import { durationMs, timestampDate } from "@bufbuild/protobuf/wkt";
import {
  artwork,
  type Copy,
  type ServerEntry,
  thumbOf,
  useChildren,
  useItem,
  useOtherCopies,
  useServer,
  useSignedIn,
  useSimilar,
  useUserData,
} from "@mavio/client-ts";
import { CreditKind, type Credit, ImageKind, type Item, ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { type MediaSource, StreamKind } from "@mavio/proto/mavio/library/v1/media_pb";
import { getRouteApi, Link } from "@tanstack/react-router";
import { useState } from "react";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import {
  Artwork,
  Button,
  Grid,
  ItemCard,
  Loading,
  minutesLeft,
  progressOf,
  runtimeOf,
  Shelf,
  StatusView,
  yearOf,
} from "../ui.tsx";

const route = getRouteApi("/s/$serverId/item/$itemId");

export function Details() {
  const { serverId, itemId } = route.useParams();
  const server = useServer(serverId);
  const q = useItem(server?.token ? server : undefined, itemId);
  if (!server?.token) return <StatusView icon="error" title={t("auth.signedOut")} />;
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
    <ItemDetails
      key={itemId}
      server={server}
      item={q.data.item}
      sources={q.data.mediaSources}
      credits={q.data.credits}
    />
  );
}

/** The resolution of a source's first video stream, as "4K" or "1080p". */
function resolutionOf(source?: MediaSource): string | undefined {
  const v = source?.streams.find((s) => s.kind === StreamKind.VIDEO);
  if (!v?.height) return undefined;
  if (v.width >= 3200 || v.height >= 2000) return "4K";
  if (v.width >= 1800 || v.height >= 1000) return "1080p";
  if (v.width >= 1200 || v.height >= 700) return "720p";
  return `${v.height}p`;
}

function sizeOf(bytes: bigint): string {
  const n = Number(bytes);
  return n >= 1e9 ? `${(n / 1e9).toFixed(1)} GB` : `${Math.round(n / 1e6)} MB`;
}

function ItemDetails({
  server,
  item,
  sources,
  credits,
}: {
  server: ServerEntry;
  item: Item;
  sources: MediaSource[];
  credits: Credit[];
}) {
  const several = useSignedIn().length > 1;
  const children = useChildren(server, item);
  const similar = useSimilar(server, item.id);
  const copies = useOtherCopies(server.id, item);
  const userData = useUserData(server);
  const [source, setSource] = useState(sources[0]?.id);

  const backdrop = artwork(server, item, ImageKind.BACKDROP, 1920);
  const logo = artwork(server, item, ImageKind.LOGO, 600);
  const poster = artwork(server, item, ImageKind.PRIMARY, 400);
  const played = !!item.userData?.played;
  const favorite = !!item.userData?.favorite;
  const position = item.userData?.position ? durationMs(item.userData.position) : 0;
  const resumable = progressOf(item) !== undefined;
  const playable = sources.length > 0;
  const firstUnplayed = item.kind === ItemKind.SEASON ? children.data?.find((c) => !c.userData?.played) : undefined;

  const facts = [
    yearOf(item),
    item.officialRating,
    item.communityRating > 0 ? `★ ${item.communityRating.toFixed(1)}` : undefined,
    runtimeOf(item),
    resolutionOf(sources.find((s) => s.id === source)),
  ].filter(Boolean);

  return (
    <div>
      <header className="relative">
        {backdrop && (
          <div className="relative h-[56vw] max-h-[70vh] min-h-72 w-full overflow-hidden">
            <img src={backdrop.url} alt="" className="h-full w-full object-cover" />
            <div className="absolute inset-0 bg-gradient-to-t from-background via-background/60 to-transparent" />
            <div className="absolute inset-0 bg-gradient-to-r from-background/80 via-transparent to-transparent" />
          </div>
        )}
        <div className={`relative flex gap-6 px-margin ${backdrop ? "-mt-48 md:-mt-64" : "pt-8"}`}>
          {poster && !backdrop && (
            <Artwork src={poster.url} alt={item.name} className="hidden aspect-[2/3] w-48 shrink-0 rounded-md md:block" />
          )}
          <div className="flex max-w-3xl flex-col gap-3">
            {item.kind === ItemKind.EPISODE || item.kind === ItemKind.SEASON ? (
              item.seriesId ? (
                <Link
                  to="/s/$serverId/item/$itemId"
                  params={{ serverId: server.id, itemId: item.seriesId }}
                  className="type-headline text-text-secondary hover:underline"
                >
                  {item.seriesName}
                </Link>
              ) : null
            ) : null}
            {logo ? (
              <img src={logo.url} alt={item.name} className="max-h-32 max-w-sm object-contain object-left" />
            ) : (
              <h1 className="type-display">{item.name}</h1>
            )}
            {item.kind === ItemKind.EPISODE && item.indexNumber > 0 && (
              <div className="type-headline">
                {t("item.episodeNumber", { season: item.parentIndexNumber, episode: item.indexNumber })}
              </div>
            )}
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 type-callout text-text-secondary">
              {facts.map((f) => (
                <span key={String(f)}>{f}</span>
              ))}
              {several && <span className="rounded-sm bg-surface-raised px-1.5 type-caption">{server.name}</span>}
            </div>
            <div className="flex flex-wrap gap-2 pt-1">
              {playable && (
                <Link
                  to="/s/$serverId/play/$itemId"
                  params={{ serverId: server.id, itemId: item.id }}
                  search={{ start: resumable ? position : undefined, source }}
                  className="inline-flex min-h-11 items-center gap-2 rounded-md bg-accent px-5 type-headline text-on-accent hover:bg-accent-pressed"
                >
                  <Icon name="play" fill />
                  {resumable ? t("item.resume") : t("item.play")}
                  {resumable && (
                    <span className="type-footnote opacity-80">
                      {t("item.minutesLeft", { count: minutesLeft(item) ?? 0 })}
                    </span>
                  )}
                </Link>
              )}
              {firstUnplayed && (
                <Link
                  to="/s/$serverId/play/$itemId"
                  params={{ serverId: server.id, itemId: firstUnplayed.id }}
                  className="inline-flex min-h-11 items-center gap-2 rounded-md bg-accent px-5 type-headline text-on-accent hover:bg-accent-pressed"
                >
                  <Icon name="play" fill />
                  {t("item.play")}
                </Link>
              )}
              {resumable && (
                <Link
                  to="/s/$serverId/play/$itemId"
                  params={{ serverId: server.id, itemId: item.id }}
                  search={{ source }}
                  aria-label={t("item.play")}
                  className="inline-flex min-h-11 items-center gap-2 rounded-md bg-surface-raised px-4 type-headline hover:bg-surface-overlay"
                >
                  <Icon name="previous" />
                </Link>
              )}
              <Button
                variant="secondary"
                aria-pressed={played}
                title={played ? t("item.markUnplayed") : t("item.markPlayed")}
                onClick={() => userData.mutate({ itemId: item.id, played: !played })}
              >
                <Icon name="played" fill={played} />
              </Button>
              <Button
                variant="secondary"
                aria-pressed={favorite}
                title={t("item.favorite")}
                onClick={() => userData.mutate({ itemId: item.id, favorite: !favorite })}
              >
                <Icon name="favorite" fill={favorite} className={favorite ? "text-danger" : undefined} />
              </Button>
            </div>
            {item.tagline && <p className="type-headline text-text-secondary italic">{item.tagline}</p>}
            {item.overview && <Overview text={item.overview} />}
            {item.genres.length > 0 && (
              <p className="type-footnote text-text-secondary">{item.genres.join(" · ")}</p>
            )}
          </div>
        </div>
      </header>

      <Versions server={server} sources={sources} selected={source} onSelect={setSource} copies={copies} />
      <Children server={server} item={item} items={children.data ?? []} />
      <Cast server={server} credits={credits} />
      {(similar.data?.length ?? 0) > 0 && (
        <Shelf title={t("item.similar")}>
          {similar.data!.map((i) => (
            <ItemCard key={i.id} server={server} serverId={server.id} item={i} />
          ))}
        </Shelf>
      )}
    </div>
  );
}

function Overview({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  return (
    <button
      type="button"
      onClick={() => setOpen(!open)}
      className={`text-left type-body ${open ? "" : "line-clamp-3 md:line-clamp-4"}`}
    >
      {text}
    </button>
  );
}

/** The media files of an item, and its copies on the user's other servers. */
function Versions({
  server,
  sources,
  selected,
  onSelect,
  copies,
}: {
  server: ServerEntry;
  sources: MediaSource[];
  selected?: string;
  onSelect: (id: string) => void;
  copies: Copy[];
}) {
  if (sources.length < 2 && copies.length === 0) return null;
  return (
    <section className="px-margin py-4">
      <h2 className="pb-2 type-title3">{t("item.versions")}</h2>
      <div className="flex flex-wrap gap-2">
        {sources.length > 1 &&
          sources.map((s) => (
            <button
              key={s.id}
              type="button"
              onClick={() => onSelect(s.id)}
              aria-pressed={s.id === selected}
              className="rounded-md border border-border px-3 py-2 text-left aria-pressed:border-accent"
            >
              <div className="type-headline">{s.name || resolutionOf(s) || s.container}</div>
              <div className="type-footnote text-text-secondary">
                {[resolutionOf(s), s.container, s.sizeBytes > 0n ? sizeOf(s.sizeBytes) : undefined]
                  .filter(Boolean)
                  .join(" · ")}
                {" · "}
                {t("item.onServer", { server: server.name })}
              </div>
            </button>
          ))}
        {copies.map((c) => (
          <Link
            key={c.server.id}
            to="/s/$serverId/item/$itemId"
            params={{ serverId: c.server.id, itemId: c.item.id }}
            className="rounded-md border border-border px-3 py-2 hover:bg-surface-raised"
          >
            <div className="type-headline">{c.item.name}</div>
            <div className="type-footnote text-text-secondary">{t("item.onServer", { server: c.server.name })}</div>
          </Link>
        ))}
      </div>
    </section>
  );
}

/** A series' seasons, a season's episodes, an album's tracks or a collection's items. */
function Children({ server, item, items }: { server: ServerEntry; item: Item; items: Item[] }) {
  if (items.length === 0) return null;
  if (item.kind === ItemKind.SEASON) {
    return (
      <section className="py-4">
        <h2 className="px-margin pb-2 type-title3">{t("item.episodes")}</h2>
        <div className="flex flex-col">
          {items.map((e) => (
            <EpisodeRow key={e.id} server={server} episode={e} />
          ))}
        </div>
      </section>
    );
  }
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
  if (item.kind === ItemKind.SERIES) {
    return (
      <Shelf title={t("item.seasons")}>
        {items.map((s) => (
          <ItemCard key={s.id} server={server} serverId={server.id} item={s} />
        ))}
      </Shelf>
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

function EpisodeRow({ server, episode: e }: { server: ServerEntry; episode: Item }) {
  const thumb = thumbOf(server, e, 320);
  const progress = progressOf(e);
  const aired = e.premiereDate ? timestampDate(e.premiereDate).toLocaleDateString() : undefined;
  return (
    <Link
      to="/s/$serverId/item/$itemId"
      params={{ serverId: server.id, itemId: e.id }}
      className="flex gap-4 px-margin py-3 hover:bg-surface"
    >
      <div className="relative w-[calc(var(--mavio-layout-thumbnail)/2)] shrink-0 md:w-thumbnail">
        <Artwork src={thumb?.url} alt={e.name} className="aspect-video rounded-md" />
        {progress !== undefined && (
          <div className="absolute inset-x-0 bottom-0 h-[3px] bg-scrim">
            <div className="h-full bg-progress" style={{ width: `${progress * 100}%` }} />
          </div>
        )}
        {e.userData?.played && (
          <span className="absolute top-1.5 right-1.5 rounded-full bg-scrim p-0.5">
            <Icon name="played" fill size={16} />
          </span>
        )}
      </div>
      <div className="min-w-0 flex-1">
        <div className="type-headline">
          {e.indexNumber > 0 && `${e.indexNumber}. `}
          {e.name}
        </div>
        <div className="type-footnote text-text-secondary">{[runtimeOf(e), aired].filter(Boolean).join(" · ")}</div>
        {e.overview && <p className="mt-1 line-clamp-2 type-callout text-text-secondary">{e.overview}</p>}
      </div>
    </Link>
  );
}

function Cast({ server, credits }: { server: ServerEntry; credits: Credit[] }) {
  if (credits.length === 0) return null;
  const isCast = (c: Credit) => c.kind === CreditKind.ACTOR || c.kind === CreditKind.GUEST_STAR;
  const sorted = [...credits].sort((a, b) => Number(!isCast(a)) - Number(!isCast(b)) || a.order - b.order);
  return (
    <Shelf title={t("item.cast")}>
      {sorted.map((c, i) => {
        const p = c.person;
        if (!p) return null;
        const photo = artwork(server, p, ImageKind.PRIMARY, 160);
        const initials = p.name
          .split(/\s+/)
          .map((w) => w[0])
          .slice(0, 2)
          .join("");
        return (
          <div key={`${p.id}-${i}`} className="w-28 shrink-0 snap-start text-center">
            <div className="relative mx-auto aspect-square w-24 overflow-hidden rounded-full bg-surface-raised">
              {photo ? (
                <img src={photo.url} alt="" loading="lazy" className="h-full w-full object-cover" />
              ) : (
                <span className="absolute inset-0 flex items-center justify-center type-title3 text-text-secondary">
                  {initials}
                </span>
              )}
            </div>
            <div className="mt-2 line-clamp-2 type-callout">{p.name}</div>
            {c.role && <div className="line-clamp-2 type-footnote text-text-secondary">{c.role}</div>}
          </div>
        );
      })}
    </Shelf>
  );
}

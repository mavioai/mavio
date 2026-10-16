// A discovered collection's page (docs/clients.md §6.3.2), as Seerr's,
// opened from the page of a movie belonging to it: its backdrop, poster,
// name and overview, Request asking for the movies of it chosen, and for
// administrators Blocklist, then its movies marked with what the server
// has of each.
import { errorMessage, isNotFound, type ServerEntry, useAdminServers, useCollection, useRequestActions, useRequestTitle, useServer } from "@mavio/client-ts";
import { type Collection, type DiscoveredTitle, MediaStatus } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { getRouteApi } from "@tanstack/react-router";
import { useState } from "react";

import { discoverLanguage, t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { usePrefs, useWidthClass } from "../prefs.ts";
import { Button, Grid, Loading, NotFoundView, StatusView } from "../ui.tsx";
import { Modal } from "../controls/dialog.tsx";
import { Checkbox } from "../controls/inputs.tsx";
import { sized, statusLabel, TitleCard, tmdbOf } from "./discover.tsx";

const route = getRouteApi("/s/$serverId/discover/collection/$tmdbId");

export function CollectionScreen() {
  const { serverId, tmdbId } = route.useParams();
  const server = useServer(serverId);
  const q = useCollection(server?.token ? server : undefined, Number(tmdbId), discoverLanguage);
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  if (q.isPending) return <Loading />;
  if ((q.isError && isNotFound(q.error)) || (q.isSuccess && !q.data)) return <NotFoundView />;
  if (q.isError || !q.data) {
    return <StatusView icon="error" title={t("state.error")} message={q.error ? errorMessage(q.error) : undefined} />;
  }
  return <CollectionPage key={tmdbId} server={server} tmdbId={Number(tmdbId)} collection={q.data} />;
}

/**
 * What the server has of a collection, as Seerr marks it: available when
 * it has every movie, partially available when some, requested when every
 * other movie is requested, blocklisted when every movie is.
 */
function collectionStatus(parts: DiscoveredTitle[]): MediaStatus {
  if (parts.length === 0) return MediaStatus.UNSPECIFIED;
  const every = (s: MediaStatus) => parts.every((p) => p.status === s);
  const has = (p: DiscoveredTitle) => p.status === MediaStatus.AVAILABLE || p.status === MediaStatus.PARTIALLY_AVAILABLE;
  if (every(MediaStatus.BLOCKLISTED)) return MediaStatus.BLOCKLISTED;
  if (every(MediaStatus.AVAILABLE)) return MediaStatus.AVAILABLE;
  if (parts.some(has)) return MediaStatus.PARTIALLY_AVAILABLE;
  if (parts.every((p) => p.status === MediaStatus.REQUESTED || p.status === MediaStatus.WANTED)) return MediaStatus.REQUESTED;
  return MediaStatus.UNSPECIFIED;
}

function CollectionPage({ server, tmdbId, collection: c }: { server: ServerEntry; tmdbId: number; collection: Collection }) {
  const { tv } = usePrefs();
  const phone = useWidthClass() === "compact" && !tv;
  const admin = useAdminServers().some((s) => s.id === server.id);
  const actions = useRequestActions(server);
  const [asking, setAsking] = useState(false);
  const requestable = c.parts.filter((p) => p.requestable);
  const status = collectionStatus(c.parts);
  const label = statusLabel(status);
  const blocked = status === MediaStatus.BLOCKLISTED;
  const block = blocked ? actions.unblockCollection : actions.blockCollection;
  const failed = actions.blockCollection.error ?? actions.unblockCollection.error;
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
          tv ? "h-[70vh]" : phone ? "h-[56vh] min-h-96" : "mx-2.5 mt-2.5 h-[clamp(440px,60vh,600px)] rounded-[18px]"
        }`}
      >
        {c.backdropUrl && <img src={sized(c.backdropUrl, 1280)} alt="" className="absolute inset-0 h-full w-full animate-[fade_600ms_ease-out] object-cover" />}
        <div
          className={`absolute inset-0 ${
            phone
              ? "bg-gradient-to-t from-black via-black/50 to-transparent"
              : "bg-[linear-gradient(90deg,rgb(0_0_0/0.82)_0%,rgb(0_0_0/0.45)_45%,transparent_75%),linear-gradient(0deg,rgb(0_0_0/0.75)_0%,transparent_55%)]"
          }`}
        />
        <div className={`absolute inset-x-0 bottom-0 flex items-end gap-10 ${phone ? "px-margin pb-6" : tv ? "px-margin pb-[10vh]" : "p-10"}`}>
          {c.posterUrl && !phone && (
            <img src={sized(c.posterUrl, 500)} alt="" className={`shrink-0 rounded-lg shadow-level3 ${tv ? "w-[300px]" : "w-[200px]"}`} />
          )}
          <div className={`flex min-w-0 flex-col gap-3 text-white ${tv ? "max-w-[960px]" : "max-w-[640px]"}`}>
            <h1 className={`font-bold leading-[1.05] text-balance break-words ${tv ? "text-[72px]" : phone ? "text-[34px]" : "text-[52px]"}`}>{c.name}</h1>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 type-callout text-white/80">
              <span>{t("collection.movieCount", { count: c.parts.length })}</span>
              {label && <span className="rounded-[4px] bg-white/20 px-1.5 type-caption font-semibold text-white">{label}</span>}
            </div>
            {c.overview && <p className={`type-body text-white/85 ${tv ? "line-clamp-2" : "line-clamp-4"}`}>{c.overview}</p>}
            <div className={`mt-2 flex flex-wrap gap-3 ${phone ? "[&>*]:flex-1" : ""}`}>
              {requestable.length > 0 && (
                <button type="button" onClick={() => setAsking(true)} data-tv-button className={primary}>
                  <Icon name="add" size={tv ? 28 : 20} />
                  {t("collection.request")}
                </button>
              )}
              {admin && (
                <button type="button" disabled={block.isPending} onClick={() => block.mutate(tmdbId)} data-tv-button className={secondary}>
                  <Icon name={blocked ? "check" : "close"} size={tv ? 28 : 20} />
                  {blocked ? t("collection.unblock") : t("collection.block")}
                </button>
              )}
            </div>
            {failed && <p className="type-callout text-white">{errorMessage(failed)}</p>}
          </div>
        </div>
      </header>
      {phone && c.overview && <p className="px-margin pt-4 type-body">{c.overview}</p>}
      <section className="pt-8">
        <h2 className="px-margin pb-3 type-title3">{t("collection.movies")}</h2>
        <Grid>
          {c.parts.map((p) => (
            <TitleCard key={tmdbOf(p)} serverId={server.id} title={p} fluid />
          ))}
        </Grid>
      </section>
      {asking && <RequestCollectionDialog server={server} collection={c} onClose={() => setAsking(false)} />}
    </div>
  );
}

/** The dialog choosing the movies of a collection to request, all those not requested yet by default. */
function RequestCollectionDialog({ server, collection: c, onClose }: { server: ServerEntry; collection: Collection; onClose: () => void }) {
  const request = useRequestTitle(server);
  const open = c.parts.filter((p) => p.requestable);
  const [chosen, setChosen] = useState(() => new Set(open.map(tmdbOf)));
  const [busy, setBusy] = useState(false);
  const all = chosen.size === open.length;
  const toggle = (id: number) =>
    setChosen((s) => {
      const next = new Set(s);
      if (!next.delete(id)) next.add(id);
      return next;
    });
  const submit = async () => {
    setBusy(true);
    try {
      for (const id of chosen) await request.mutateAsync({ mediaKind: MediaKind.MOVIE, tmdbId: id });
      onClose();
    } catch {
      // The error shows in the dialog.
    } finally {
      setBusy(false);
    }
  };
  const row = "min-h-12 rounded-lg px-3 hover:bg-fill";
  return (
    <Modal title={t("discover.requestTitle", { name: c.name })} onClose={onClose}>
      <div className="-mx-3 flex min-h-0 flex-col overflow-y-auto">
        <Checkbox checked={all} onChange={(on) => setChosen(on ? new Set(open.map(tmdbOf)) : new Set())} className={row}>
          <span className="type-headline">{t("collection.allMovies")}</span>
        </Checkbox>
        {c.parts.map((p) => {
          const status = statusLabel(p.status);
          return (
            <Checkbox key={tmdbOf(p)} checked={chosen.has(tmdbOf(p))} disabled={!p.requestable} onChange={() => toggle(tmdbOf(p))} className={row}>
              <span className="min-w-0 flex-1 truncate">{p.name}</span>
              {p.year > 0 && <span className="type-callout text-text-secondary">{p.year}</span>}
              {status && <span className="type-footnote text-text-secondary">{status}</span>}
            </Checkbox>
          );
        })}
      </div>
      {request.isError && <p className="type-callout text-danger">{t("discover.requestFailed", { reason: errorMessage(request.error) })}</p>}
      <div className="flex justify-end gap-2 pt-2">
        <Button variant="secondary" onClick={onClose}>
          {t("action.cancel")}
        </Button>
        <Button disabled={busy || chosen.size === 0} onClick={() => void submit()}>
          {t("collection.requestMovies", { count: chosen.size })}
        </Button>
      </div>
    </Modal>
  );
}

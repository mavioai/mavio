// The settings' Wanted (docs/clients.md §6.3.16): the monitored movies and
// episodes missing a file or below their profile's cutoff, as Radarr's and
// Sonarr's Wanted, each searched for or unmonitored, all searched at once,
// and an RSS sync.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, useWantedActions, useWantedList } from "@mavio/client-ts";
import { type WantedEpisode, WantedFilter, WantedKind, type WantedMovie } from "@mavio/proto/mavio/acquisition/v1/wanted_pb";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { locale, t } from "../../i18n.ts";
import { Button, Group, LoadMore, Select } from "../../ui.tsx";
import { Page } from "./common.tsx";

const day = new Intl.DateTimeFormat([locale], { dateStyle: "medium" });

/** An episode's numbers, as "S01E02". */
export function episodeCode(season: number, episode: number): string {
  return `S${String(season).padStart(2, "0")}E${String(episode).padStart(2, "0")}`;
}

/** The section of the server chosen in the settings. */
export function WantedSection({ server }: { server: ServerEntry }) {
  const [filter, setFilter] = useState(WantedFilter.MISSING);
  const [kind, setKind] = useState(WantedKind.MOVIE);
  const list = useWantedList(server, filter, kind);
  const actions = useWantedActions(server);
  const [queued, setQueued] = useState<number>();
  const [synced, setSynced] = useState(false);
  const pages = list.data?.pages ?? [];
  const movies = pages.flatMap((p) => p.movies);
  const episodes = pages.flatMap((p) => p.episodes);
  const total = pages[0]?.total ?? 0;
  const searchAll = () => actions.search.mutate({ filter, kind }, { onSuccess: (n) => setQueued(n) });
  return (
    <Page title={t("settings.wanted")} message={t("wanted.message")}>
      <div className="flex flex-wrap items-center gap-2">
        <Select
          value={String(filter)}
          label={t("wanted.filter")}
          options={[
            { value: String(WantedFilter.MISSING), label: t("wanted.missing") },
            { value: String(WantedFilter.CUTOFF_UNMET), label: t("wanted.cutoffUnmet") },
          ]}
          onChange={(v) => (setFilter(Number(v) as WantedFilter), setQueued(undefined))}
        />
        <Select
          value={String(kind)}
          label={t("wanted.kind")}
          options={[
            { value: String(WantedKind.MOVIE), label: t("wanted.movies") },
            { value: String(WantedKind.EPISODE), label: t("wanted.episodes") },
          ]}
          onChange={(v) => (setKind(Number(v) as WantedKind), setQueued(undefined))}
        />
        <span className="flex-1" />
        <Button variant="secondary" disabled={actions.syncRss.isPending} onClick={() => actions.syncRss.mutate(undefined, { onSuccess: () => setSynced(true) })}>
          {actions.syncRss.isPending ? "…" : t("wanted.rssSync")}
        </Button>
        <Button disabled={actions.search.isPending || total === 0} onClick={searchAll}>
          {actions.search.isPending ? "…" : t("wanted.searchAll")}
        </Button>
      </div>
      {(queued !== undefined || synced || actions.search.isError || actions.syncRss.isError) && (
        <p className="type-callout text-text-secondary">
          {[
            queued !== undefined && t("wanted.queued", { count: queued }),
            synced && t("wanted.rssQueued"),
            actions.search.isError && errorMessage(actions.search.error),
            actions.syncRss.isError && errorMessage(actions.syncRss.error),
          ]
            .filter(Boolean)
            .join(" · ")}
        </p>
      )}
      <Group footer={list.isSuccess ? t("wanted.count", { count: total }) : undefined}>
        {kind === WantedKind.MOVIE
          ? movies.map((m) => <MovieRow key={m.itemId} server={server} movie={m} />)
          : episodes.map((e) => <EpisodeRow key={e.itemId} server={server} episode={e} />)}
        {list.isSuccess && total === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("wanted.none")}</p>}
        {list.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(list.error)}</p>}
      </Group>
      <LoadMore active={list.hasNextPage && !list.isFetchingNextPage} onVisible={() => void list.fetchNextPage()} />
    </Page>
  );
}

/** A wanted movie or episode's row: its name linking to its details, what it shows, and searching and unmonitoring it. */
function WantedRow({ server, itemId, movie, label, detail }: { server: ServerEntry; itemId: string; movie: boolean; label: string; detail: string }) {
  const actions = useWantedActions(server);
  const [searched, setSearched] = useState(false);
  const unmonitor = movie ? actions.setWantedMonitored : actions.setMonitored;
  return (
    <div className="flex min-h-13 flex-wrap items-center gap-x-3 gap-y-1 px-4 py-2.5">
      <div className="min-w-0 flex-1">
        <Link to="/s/$serverId/item/$itemId" params={{ serverId: server.id, itemId }} className="type-body hover:underline">
          {label}
        </Link>
        <div className="type-footnote break-words text-text-secondary">{detail}</div>
        {(actions.search.isError || unmonitor.isError) && (
          <div className="type-footnote break-words text-danger">{errorMessage(actions.search.error ?? unmonitor.error)}</div>
        )}
      </div>
      <Button
        variant="plain"
        disabled={unmonitor.isPending}
        onClick={() =>
          movie ? actions.setWantedMonitored.mutate({ itemId, monitored: false }) : actions.setMonitored.mutate({ itemIds: [itemId], monitored: false })
        }
      >
        {t("wanted.unmonitor")}
      </Button>
      <Button variant="secondary" disabled={actions.search.isPending || searched} onClick={() => actions.search.mutate({ itemIds: [itemId] }, { onSuccess: () => setSearched(true) })}>
        {searched ? t("wanted.searching") : t("wanted.search")}
      </Button>
    </div>
  );
}

function MovieRow({ server, movie: m }: { server: ServerEntry; movie: WantedMovie }) {
  const detail = [m.quality && t("wanted.has", { quality: m.quality }), !m.available && t("wanted.notAvailable")].filter(Boolean).join(" · ");
  return <WantedRow server={server} itemId={m.itemId} movie label={m.year > 0 ? `${m.name} (${m.year})` : m.name} detail={detail} />;
}

function EpisodeRow({ server, episode: e }: { server: ServerEntry; episode: WantedEpisode }) {
  const detail = [
    e.airTime && day.format(timestampDate(e.airTime)),
    e.absoluteNumber > 0 && t("wanted.absolute", { number: e.absoluteNumber }),
    e.quality && t("wanted.has", { quality: e.quality }),
  ]
    .filter(Boolean)
    .join(" · ");
  const label = `${e.seriesName} · ${episodeCode(e.seasonNumber, e.episodeNumber)}${e.name ? ` · ${e.name}` : ""}`;
  return <WantedRow server={server} itemId={e.itemId} movie={false} label={label} detail={detail} />;
}

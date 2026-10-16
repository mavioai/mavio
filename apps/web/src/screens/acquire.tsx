// An item's acquisition on its details, for administrators (docs/clients.md
// §6.3.18): whether a movie, series, season or episode is monitored, a
// movie's or series' quality profile, an automatic search, the releases the
// indexers have of it to grab, and its downloads and history.
import { clone } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  type ServerEntry,
  useAdminServers,
  useGrab,
  useHistory,
  useQualityProfiles,
  useQueue,
  useReleases,
  useWanted,
  useWantedActions,
} from "@mavio/client-ts";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { DownloadProtocol, type Release } from "@mavio/proto/mavio/acquisition/v1/release_pb";
import { type Wanted, WantedSchema } from "@mavio/proto/mavio/acquisition/v1/wanted_pb";
import { type Item, ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { useState } from "react";

import { t } from "../i18n.ts";
import { Button, Group } from "../ui.tsx";
import { Sheet } from "../controls/dialog.tsx";
import { Switch } from "../controls/inputs.tsx";
import { Select } from "../controls/select.tsx";
import { fromNow, sizeOf } from "./admin/common.tsx";
import { HistoryRow, QueueRow } from "./admin/downloads.tsx";

const acquirable = new Set([ItemKind.MOVIE, ItemKind.SERIES, ItemKind.SEASON, ItemKind.EPISODE]);

/** The item's acquisition, when the user administers its server and it can be acquired. */
export function Acquisition({ server, item }: { server: ServerEntry; item: Item }) {
  const admin = useAdminServers().some((s) => s.id === server.id);
  if (!admin || !acquirable.has(item.kind)) return null;
  return <AcquisitionPanel server={server} item={item} />;
}

function AcquisitionPanel({ server, item }: { server: ServerEntry; item: Item }) {
  const whole = item.kind === ItemKind.MOVIE || item.kind === ItemKind.SERIES;
  const wanted = useWanted(server, item.id, whole);
  const actions = useWantedActions(server);
  const queue = useQueue(server, item.id, false);
  const history = useHistory(server, item.id);
  const [searched, setSearched] = useState(false);
  const [releases, setReleases] = useState(false);
  const downloads = queue.data?.pages.flatMap((p) => p.items) ?? [];
  const events = history.data?.pages[0]?.events.slice(0, 10) ?? [];
  const monitored = whole ? !!wanted.data?.monitored : item.monitored;
  const notWanted = whole && wanted.isSuccess && !wanted.data;
  const setMonitored = (on: boolean) =>
    whole ? actions.setWantedMonitored.mutate({ itemId: item.id, monitored: on }) : actions.setMonitored.mutate({ itemIds: [item.id], monitored: on });
  const error = [actions.setWantedMonitored, actions.setMonitored, actions.search, actions.update].find((m) => m.isError)?.error;
  return (
    <section className="flex flex-col gap-4 px-margin pt-8">
      <h2 className="type-title3">{t("acquire.title")}</h2>
      <div className="flex max-w-3xl flex-col gap-6">
        {notWanted ? (
          <Group footer={t("acquire.notWantedFooter")}>
            <div className="flex flex-wrap items-center gap-3 px-4 py-2.5">
              <p className="min-w-0 flex-1 type-body text-text-secondary">{t("acquire.notWanted")}</p>
              {/* A release can be grabbed for a movie not wanted, not for a series. */}
              {item.kind === ItemKind.MOVIE && (
                <Button variant="secondary" onClick={() => setReleases(true)}>
                  {t("acquire.interactive")}
                </Button>
              )}
            </div>
          </Group>
        ) : (
          <Group>
            <div className="flex min-h-12 items-center justify-between gap-3 px-4 py-2">
              <span className="type-body">{t("acquire.monitored")}</span>
              <Switch on={monitored} onChange={setMonitored} label={t("acquire.monitored")} />
            </div>
            {whole && wanted.data && <ProfileRow server={server} wanted={wanted.data} kind={item.kind === ItemKind.MOVIE ? MediaKind.MOVIE : MediaKind.SERIES} />}
            {whole && wanted.data?.lastSearchTime && (
              <div className="flex min-h-12 items-center justify-between gap-3 px-4 py-2">
                <span className="type-body">{t("acquire.lastSearch")}</span>
                <span className="type-callout text-text-secondary">{fromNow(timestampDate(wanted.data.lastSearchTime))}</span>
              </div>
            )}
            <div className="flex flex-wrap gap-2 px-4 py-3">
              <Button
                variant="secondary"
                disabled={actions.search.isPending || searched}
                onClick={() => actions.search.mutate({ itemIds: [item.id] }, { onSuccess: () => setSearched(true) })}
              >
                {searched ? t("wanted.searching") : t("acquire.search")}
              </Button>
              <Button variant="secondary" onClick={() => setReleases(true)}>
                {t("acquire.interactive")}
              </Button>
            </div>
            {error && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(error)}</p>}
            {wanted.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(wanted.error)}</p>}
          </Group>
        )}
        {downloads.length > 0 && (
          <Group title={t("downloads.queue")}>
            {downloads.map((d) => (
              <QueueRow key={d.id} server={server} item={d} />
            ))}
          </Group>
        )}
        {events.length > 0 && (
          <Group title={t("downloads.history")}>
            {events.map((e) => (
              <HistoryRow key={e.id} server={server} event={e} />
            ))}
          </Group>
        )}
      </div>
      {releases && <ReleasesDialog server={server} item={item} onClose={() => setReleases(false)} />}
    </section>
  );
}

function ProfileRow({ server, wanted, kind }: { server: ServerEntry; wanted: Wanted; kind: MediaKind }) {
  const profiles = useQualityProfiles(server).list.data?.filter((p) => p.kind === kind) ?? [];
  const { update } = useWantedActions(server);
  const choose = (id: string) => {
    const next = clone(WantedSchema, wanted);
    next.qualityProfileId = id;
    update.mutate(next);
  };
  return (
    <div className="flex min-h-12 items-center justify-between gap-3 px-4 py-2">
      <span className="type-body">{t("acquire.profile")}</span>
      <Select value={wanted.qualityProfileId} label={t("acquire.profile")} options={profiles.map((p) => ({ value: p.id, label: p.name }))} onChange={choose} />
    </div>
  );
}

/** The releases the indexers have of the item, each grabbed in place. */
function ReleasesDialog({ server, item, onClose }: { server: ServerEntry; item: Item; onClose: () => void }) {
  const releases = useReleases(server, item.id, true);
  const list = releases.data?.releases ?? [];
  const failures = releases.data?.failures ?? [];
  return (
    <Sheet label={t("acquire.releasesTitle", { name: item.name })} width="full" onClose={onClose}>
      <div className="flex max-h-[85vh] min-h-0 flex-col gap-3 p-6">
        <div className="flex items-center gap-3">
          <h2 className="min-w-0 flex-1 type-title2">
            {t("acquire.releasesTitle", { name: item.name })}
          </h2>
          <Button variant="secondary" disabled={releases.isFetching} onClick={() => void releases.refetch()}>
            {releases.isFetching ? "…" : t("logs.refresh")}
          </Button>
          <Button variant="plain" onClick={onClose}>
            {t("keys.done")}
          </Button>
        </div>
        {failures.map((f) => (
          <p key={`${f.pluginId}/${f.indexerId}`} className="type-footnote break-words text-danger">
            {t("acquire.indexerFailed", { indexer: f.indexerId, message: f.message })}
          </p>
        ))}
        <div className="-mx-6 min-h-0 flex-1 divide-y divide-border overflow-y-auto border-t border-border">
          {releases.isFetching && list.length === 0 && <p className="px-6 py-4 type-callout text-text-secondary">{t("acquire.searchingIndexers")}</p>}
          {releases.isSuccess && list.length === 0 && <p className="px-6 py-4 type-callout text-text-secondary">{t("acquire.noReleases")}</p>}
          {releases.isError && <p className="px-6 py-4 type-callout break-words text-danger">{errorMessage(releases.error)}</p>}
          {list.map((r) => (
            <ReleaseRow key={`${r.indexerId}/${r.guid}`} server={server} itemId={item.id} release={r} />
          ))}
        </div>
      </div>
    </Sheet>
  );
}

function ReleaseRow({ server, itemId, release: r }: { server: ServerEntry; itemId: string; release: Release }) {
  const grab = useGrab(server);
  const torrent = r.protocol === DownloadProtocol.TORRENT;
  const detail = [
    r.parsed?.quality?.name,
    r.parsed?.languages.join(", "),
    r.size > 0n && sizeOf(r.size),
    r.publishDate && fromNow(timestampDate(r.publishDate)),
    torrent ? t("acquire.peers", { seeders: r.seeders, peers: r.peers }) : t("acquire.usenet"),
    r.indexerName,
  ].filter(Boolean);
  const formats = [...r.customFormats, r.customFormats.length > 0 || r.customFormatScore !== 0 ? t("acquire.score", { score: r.customFormatScore }) : ""].filter(Boolean);
  const rejected = r.rejections.length > 0;
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 px-6 py-3">
      <div className="flex min-w-0 flex-1 basis-80 flex-col gap-0.5">
        <p className={`type-callout break-all ${rejected ? "text-text-secondary" : ""}`}>{r.title}</p>
        <p className="type-footnote text-text-secondary">{detail.join(" · ")}</p>
        {formats.length > 0 && <p className="type-footnote text-text-secondary">{formats.join(" · ")}</p>}
        {r.rejections.map((x, i) => (
          <p key={i} className="type-footnote break-words text-danger">
            {x.message || x.reason}
          </p>
        ))}
        {grab.isError && <p className="type-footnote break-words text-danger">{errorMessage(grab.error)}</p>}
      </div>
      <Button variant={rejected ? "secondary" : "primary"} disabled={grab.isPending || grab.isSuccess} onClick={() => grab.mutate({ itemId, release: r })}>
        {grab.isSuccess ? t("downloads.grabbed") : grab.isPending ? "…" : t("acquire.grab")}
      </Button>
    </div>
  );
}

// The indexer search, an advanced tool (docs/clients.md §6.3.21): a search
// of the indexers by text, as Prowlarr's, whose releases are sent to a
// download client without a library item. With a Mavio account it searches
// every server the user administers, or one chosen, each server's releases
// a group of their own pages.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ReleaseSearch, type ServerEntry, useAdminServers, useIndexers, useReleaseSearch, useSendRelease, useSignIn } from "@mavio/client-ts";
import { DownloadProtocol, type Release, ReleaseSearchType } from "@mavio/proto/mavio/acquisition/v1/release_pb";
import { useParams } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";

import { Tip } from "../controls/tooltip.tsx";
import { Select } from "../controls/select.tsx";
import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { setPrefs, usePrefs, useWidthClass } from "../prefs.ts";
import { Button, Group, NotFoundView } from "../ui.tsx";
import { lengthOf, Page, sizeOf } from "./admin/common.tsx";
import type { ToolId } from "./settings-menus.ts";

const pageSize = 50;

const types: { value: string; type: ReleaseSearchType; label: () => string }[] = [
  { value: "any", type: ReleaseSearchType.UNSPECIFIED, label: () => t("indexers.typeAny") },
  { value: "movie", type: ReleaseSearchType.MOVIE, label: () => t("indexers.typeMovie") },
  { value: "tv", type: ReleaseSearchType.TV, label: () => t("indexers.typeTv") },
];

/** Newznab's top categories, by their first number. */
const categories: Record<number, () => string> = {
  1000: () => t("indexers.categoryConsole"),
  2000: () => t("indexers.categoryMovies"),
  3000: () => t("indexers.categoryAudio"),
  4000: () => t("indexers.categoryPc"),
  5000: () => t("indexers.categoryTv"),
  6000: () => t("indexers.categoryXxx"),
  7000: () => t("indexers.categoryBooks"),
  8000: () => t("indexers.categoryOther"),
};

/** The top categories of a release's categories, by name. */
function categoryNames(list: number[]): string {
  const tops = [...new Set(list.map((c) => Math.floor(c / 1000) * 1000))].filter((c) => categories[c]);
  return tops.map((c) => categories[c]()).join(", ");
}

/** How long ago a release was published: in days from a day, as Prowlarr tells. */
function ageOf(r: Release): string {
  if (!r.publishDate) return "";
  const ms = Date.now() - timestampDate(r.publishDate).getTime();
  return lengthOf(ms >= day ? Math.floor(ms / day) * day : ms);
}

const day = 86_400_000;

/** Whether a tool is pinned to the sidebar, and pinning or unpinning it. */
export function usePinned(id: ToolId): [boolean, () => void] {
  const pinned = usePrefs().pinned;
  const on = pinned.includes(id);
  return [on, () => setPrefs({ pinned: on ? pinned.filter((p) => p !== id) : [...pinned, id] })];
}

/** The pin of a tool's row: an icon pinning it to the sidebar, or unpinning it. */
export function PinButton({ id, className }: { id: ToolId; className?: string }) {
  const [on, toggle] = usePinned(id);
  const label = on ? t("settings.unpin") : t("settings.pin");
  return (
    <Tip label={label}>
      <button
        type="button"
        aria-label={label}
        aria-pressed={on}
        onClick={(e) => {
          e.preventDefault();
          toggle();
        }}
        className={`flex ${on ? "text-accent-text" : "text-text-tertiary hover:text-on-background"} ${className ?? ""}`}
      >
        <Icon name="pin" fill={on} size={16} />
      </button>
    </Tip>
  );
}

type SortKey = "age" | "title" | "indexer" | "size" | "grabs" | "peers";

const sorters: Record<SortKey, (r: Release) => number | string> = {
  age: (r) => -(r.publishDate ? Number(r.publishDate.seconds) : 0),
  title: (r) => r.title.toLowerCase(),
  indexer: (r) => r.indexerName.toLowerCase(),
  size: (r) => Number(r.size),
  grabs: (r) => r.grabs,
  peers: (r) => r.seeders,
};

interface Sort {
  key: SortKey;
  desc: boolean;
}

/** The indexer search: the servers to search, the search, and each server's releases. */
export function IndexerSearch() {
  const admins = useAdminServers();
  const mavioDev = useSignIn()?.kind === "mavioDev";
  // "" searches every server.
  const [serverId, setServerId] = useState("");
  const [query, setQuery] = useState("");
  const [type, setType] = useState("any");
  const [category, setCategory] = useState("all");
  const [indexer, setIndexer] = useState("all");
  const [search, setSearch] = useState<ReleaseSearch>();
  const [sort, setSort] = useState<Sort>();
  const one = mavioDev ? admins.find((s) => s.id === serverId) : admins[0];
  const searched = one ? [one] : admins;
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;
    const [pluginId, indexerId] = indexer === "all" ? [] : indexer.split("/");
    setSearch({
      query: query.trim(),
      type: types.find((x) => x.value === type)!.type,
      categories: category === "all" ? [] : [Number(category)],
      indexers: one && pluginId ? [{ pluginId, indexerId }] : [],
    });
  };
  return (
    <Page
      title={t("indexers.title")}
      message={t("indexers.message")}
    >
      <form role="search" onSubmit={submit} className="flex flex-wrap items-center gap-3">
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("indexers.query")}
          aria-label={t("indexers.query")}
          enterKeyHint="search"
          className="min-h-9 min-w-0 flex-1 basis-56 rounded-md border border-border bg-surface px-3 type-body text-on-background placeholder:text-text-tertiary focus:border-focus focus:outline-none desk:min-h-7 desk:type-callout"
        />
        {mavioDev && admins.length > 0 && (
          <Select
            value={serverId}
            label={t("settings.serverChoice")}
            options={[{ value: "", label: t("indexers.allServers") }, ...admins.map((s) => ({ value: s.id, label: s.name }))]}
            onChange={(v) => (setServerId(v), setIndexer("all"))}
          />
        )}
        <Select value={type} label={t("indexers.type")} options={types.map((x) => ({ value: x.value, label: x.label() }))} onChange={setType} />
        <Select
          value={category}
          label={t("indexers.category")}
          options={[
            { value: "all", label: t("indexers.allCategories") },
            ...Object.entries(categories).map(([c, label]) => ({ value: c, label: label() })),
          ]}
          onChange={setCategory}
        />
        {one && <IndexerChoice server={one} value={indexer} onChange={setIndexer} />}
        <Button type="submit" disabled={!query.trim()}>
          {t("indexers.search")}
        </Button>
      </form>
      {!search ? (
        <p className="type-callout text-text-secondary">{t("indexers.prompt")}</p>
      ) : (
        searched.map((s) => (
          // A new search starts each server again at its first page.
          <ServerReleases key={`${s.id}/${JSON.stringify(search)}`} server={s} search={search} named={!one} sort={sort} onSort={setSort} />
        ))
      )}
    </Page>
  );
}

/** The choice of one of a server's enabled indexers, or all of them. */
function IndexerChoice({ server, value, onChange }: { server: ServerEntry; value: string; onChange: (v: string) => void }) {
  const indexers = (useIndexers(server).data?.indexers ?? []).filter((x) => x.enabled);
  return (
    <Select
      value={value}
      label={t("indexers.indexer")}
      options={[{ value: "all", label: t("indexers.allIndexers") }, ...indexers.map((x) => ({ value: `${x.pluginId}/${x.id}`, label: x.name }))]}
      onChange={onChange}
    />
  );
}

/** A server's releases of a search, page by page, named by the server when every server is searched. */
function ServerReleases({
  server,
  search,
  named,
  sort,
  onSort,
}: {
  server: ServerEntry;
  search: ReleaseSearch;
  named: boolean;
  sort: Sort | undefined;
  onSort: (s: Sort) => void;
}) {
  const result = useReleaseSearch(server, search);
  const [page, setPage] = useState(0);
  const compact = useWidthClass() === "compact";
  const all = result.data?.releases ?? [];
  const list = sort ? sortedBy(all, sort) : all;
  const pages = Math.max(1, Math.ceil(list.length / pageSize));
  const shown = list.slice(page * pageSize, (page + 1) * pageSize);
  const names = new Map(all.map((r) => [`${r.pluginId}/${r.indexerId}`, r.indexerName]));
  const title = named ? (
    <span className="flex items-center gap-2">
      {server.name}
      {result.isSuccess && <span className="font-normal text-text-tertiary">{t("indexers.results", { count: all.length })}</span>}
    </span>
  ) : undefined;
  const footer =
    pages > 1 ? (
      <span className="flex items-center justify-end gap-2">
        <span>{t("indexers.page", { page: page + 1, pages })}</span>
        <PageButton icon="back" label={t("indexers.previous")} disabled={page === 0} onClick={() => setPage(page - 1)} />
        <PageButton icon="forward" label={t("indexers.next")} disabled={page + 1 >= pages} onClick={() => setPage(page + 1)} />
      </span>
    ) : undefined;
  return (
    <Group title={title} footer={footer}>
      {result.data?.failures.map((f) => (
        <p key={`${f.pluginId}/${f.indexerId}`} className="px-4 py-2.5 type-footnote break-words text-danger">
          {t("acquire.indexerFailed", { indexer: names.get(`${f.pluginId}/${f.indexerId}`) || f.indexerId || f.pluginId, message: f.message })}
        </p>
      ))}
      {result.isFetching && all.length === 0 && <p className="px-4 py-2.5 type-callout text-text-secondary">{t("acquire.searchingIndexers")}</p>}
      {result.isError && <p className="px-4 py-2.5 type-callout break-words text-danger">{errorMessage(result.error)}</p>}
      {result.isSuccess && all.length === 0 && <p className="px-4 py-2.5 type-callout text-text-secondary">{t("acquire.noReleases")}</p>}
      {shown.length > 0 &&
        (compact ? (
          <div className="divide-y divide-border">
            {shown.map((r) => (
              <ReleaseItem key={`${r.pluginId}/${r.indexerId}/${r.guid}`} server={server} release={r} />
            ))}
          </div>
        ) : (
          <ReleaseTable server={server} releases={shown} sort={sort} onSort={onSort} />
        ))}
    </Group>
  );
}

function sortedBy(list: Release[], sort: Sort): Release[] {
  const key = sorters[sort.key];
  return [...list].sort((a, b) => {
    const x = key(a);
    const y = key(b);
    const c = typeof x === "string" ? x.localeCompare(y as string) : x - (y as number);
    return sort.desc ? -c : c;
  });
}

function PageButton({ icon, label, disabled, onClick }: { icon: "back" | "forward"; label: string; disabled: boolean; onClick: () => void }) {
  return (
    <Tip label={label}>
      <button
        type="button"
        aria-label={label}
        disabled={disabled}
        onClick={onClick}
        className="flex size-7 items-center justify-center rounded-md text-on-background hover:bg-fill disabled:text-text-disabled disabled:hover:bg-transparent"
      >
        <Icon name={icon} size={14} />
      </button>
    </Tip>
  );
}

/** The columns of the releases' table: sorted by their key, when they have one. */
const columns: { label: () => string; key?: SortKey; className?: string }[] = [
  { label: () => t("indexers.protocol") },
  { label: () => t("indexers.age"), key: "age" },
  { label: () => t("indexers.releaseTitle"), key: "title", className: "w-full min-w-80" },
  { label: () => t("indexers.indexer"), key: "indexer" },
  { label: () => t("indexers.size"), key: "size", className: "text-end" },
  { label: () => t("indexers.grabs"), key: "grabs", className: "text-end" },
  { label: () => t("indexers.peers"), key: "peers", className: "text-end" },
  { label: () => t("indexers.category") },
  { label: () => "" },
];

/** A page of releases as a table, as Prowlarr's search lists them; a column's heading sorts by it. */
function ReleaseTable({ server, releases, sort, onSort }: { server: ServerEntry; releases: Release[]; sort: Sort | undefined; onSort: (s: Sort) => void }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full type-footnote">
        <thead>
          <tr className="border-b border-border text-start text-text-secondary">
            {columns.map((c, i) => (
              <th key={i} scope="col" aria-sort={c.key && sort?.key === c.key ? (sort.desc ? "descending" : "ascending") : undefined} className={`px-2 py-2 font-medium whitespace-nowrap first:ps-4 last:pe-4 ${c.className ?? "text-start"}`}>
                {c.key ? (
                  <button
                    type="button"
                    onClick={() => onSort({ key: c.key!, desc: sort && sort.key === c.key ? !sort.desc : c.key === "size" || c.key === "grabs" || c.key === "peers" })}
                    className="inline-flex items-center gap-1 hover:text-on-background"
                  >
                    {c.label()}
                    {sort?.key === c.key && <Icon name="expand" size={12} className={sort.desc ? "" : "rotate-180"} />}
                  </button>
                ) : (
                  c.label()
                )}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {releases.map((r) => (
            <ReleaseLine key={`${r.pluginId}/${r.indexerId}/${r.guid}`} server={server} release={r} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

const cell = "px-2 py-2 align-middle whitespace-nowrap first:ps-4 last:pe-4";

function ReleaseLine({ server, release: r }: { server: ServerEntry; release: Release }) {
  const torrent = r.protocol === DownloadProtocol.TORRENT;
  return (
    <tr>
      <td className={cell}>
        <span className={`rounded px-1.5 py-0.5 type-caption font-medium ${torrent ? "bg-accent/15 text-accent-text" : "bg-fill text-text-secondary"}`}>
          {torrent ? t("indexers.torrent") : t("acquire.usenet")}
        </span>
      </td>
      <td className={`${cell} text-text-secondary`}>{ageOf(r)}</td>
      <td className="min-w-80 px-2 py-2 align-middle break-all">{r.title}</td>
      <td className={`${cell} text-text-secondary`}>{r.indexerName}</td>
      <td className={`${cell} text-end`}>{r.size > 0n ? sizeOf(r.size) : ""}</td>
      <td className={`${cell} text-end text-text-secondary`}>{r.grabs || ""}</td>
      <td className={`${cell} text-end`}>{torrent ? `${r.seeders} / ${r.peers}` : ""}</td>
      <td className={`${cell} text-text-secondary`}>{categoryNames(r.categories)}</td>
      <td className={cell}>
        <ReleaseActions server={server} release={r} />
      </td>
    </tr>
  );
}

/** A release on a phone: its title, and what the table's columns would tell, in a line under it. */
function ReleaseItem({ server, release: r }: { server: ServerEntry; release: Release }) {
  const torrent = r.protocol === DownloadProtocol.TORRENT;
  const detail = [
    torrent ? t("indexers.torrent") : t("acquire.usenet"),
    ageOf(r),
    r.indexerName,
    r.size > 0n && sizeOf(r.size),
    torrent && t("acquire.peers", { seeders: r.seeders, peers: r.peers }),
    categoryNames(r.categories),
  ].filter(Boolean);
  return (
    <div className="flex items-center gap-3 px-4 py-2.5">
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <p className="type-callout break-all">{r.title}</p>
        <p className="type-footnote text-text-secondary">{detail.join(" · ")}</p>
      </div>
      <ReleaseActions server={server} release={r} />
    </div>
  );
}

/**
 * Sending a release to a download client, copying its magnet link, and its
 * page on the indexer, in a window of its own.
 */
function ReleaseActions({ server, release: r }: { server: ServerEntry; release: Release }) {
  const send = useSendRelease(server);
  const [copied, setCopied] = useState(false);
  const copyLabel = copied ? t("indexers.magnetCopied") : t("indexers.magnet");
  const label = send.isSuccess ? t("indexers.sent", { client: send.data }) : send.isError ? errorMessage(send.error) : t("indexers.send");
  const action = "flex size-7 items-center justify-center rounded-md hover:bg-fill";
  return (
    <div className="flex items-center justify-end gap-0.5">
      {r.infoUrl && (
        <Tip label={t("indexers.info")}>
          <a
            href={r.infoUrl}
            target="_blank"
            rel="noreferrer"
            aria-label={t("indexers.info")}
            // Never in the page's own place, wherever a new tab is not the default.
            onClick={(e) => {
              e.preventDefault();
              window.open(r.infoUrl, "_blank", "noopener,noreferrer");
            }}
            className={`${action} text-text-secondary`}
          >
            <Icon name="external" size={16} />
          </a>
        </Tip>
      )}
      {r.magnetUrl && (
        <Tip label={copyLabel}>
          <button
            type="button"
            aria-label={copyLabel}
            onClick={() =>
              void navigator.clipboard.writeText(r.magnetUrl).then(
                () => setCopied(true),
                () => {},
              )
            }
            className={`${action} ${copied ? "text-success" : "text-text-secondary"}`}
          >
            <Icon name={copied ? "check" : "magnet"} size={16} />
          </button>
        </Tip>
      )}
      <Tip label={label}>
        <button
          type="button"
          aria-label={label}
          disabled={send.isPending || send.isSuccess}
          onClick={() => send.mutate({ release: r })}
          className={`${action} ${send.isError ? "text-danger" : send.isSuccess ? "text-success" : "text-accent-text"} disabled:hover:bg-transparent`}
        >
          <Icon name={send.isSuccess ? "check" : send.isError ? "error" : "downloads"} size={16} className={send.isPending ? "animate-pulse" : undefined} />
        </button>
      </Tip>
    </div>
  );
}

/** A tool pinned to the sidebar, opened in the app's own layout. */
export function ToolScreen() {
  const { tool } = useParams({ strict: false }) as { tool?: string };
  if (tool !== "indexer-search") return <NotFoundView />;
  return (
    <div className="mx-auto max-w-[1200px] px-margin pt-6 pb-12">
      <IndexerSearch />
    </div>
  );
}

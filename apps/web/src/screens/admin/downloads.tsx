// The settings' Downloads (docs/clients.md §6.3.17): the tracked downloads
// with their progress, removed with their release blocklisted or not; the
// history, where a grab is marked failed; the blocklist; and the manual
// import of a download's files or of a folder, as Radarr's Activity.
import { durationMs, timestampDate } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, useBlocklist, useDownloadActions, useHistory, useManualImport, useQueue } from "@mavio/client-ts";
import {
  type BlocklistEntry,
  DownloadState,
  DownloadStatus,
  type HistoryEvent,
  HistoryEventType,
  ImportMode,
  type ManualImportFile,
  type QueueItem,
} from "@mavio/proto/mavio/acquisition/v1/download_pb";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { t } from "../../i18n.ts";
import { Button, Group, LoadMore, Select, Toggle } from "../../ui.tsx";
import { fromNow, input, lengthOf, Page, sizeOf } from "./common.tsx";

type Tab = "queue" | "history" | "blocklist" | "import";

/** The section of the server chosen in the settings. */
export function DownloadsSection({ server }: { server: ServerEntry }) {
  const [tab, setTab] = useState<Tab>("queue");
  const tabs: [Tab, string][] = [
    ["queue", t("downloads.queue")],
    ["history", t("downloads.history")],
    ["blocklist", t("downloads.blocklist")],
    ["import", t("downloads.import")],
  ];
  return (
    <Page title={t("settings.downloads")} message={t("downloads.message")}>
      <div className="flex flex-wrap gap-2" role="tablist">
        {tabs.map(([id, label]) => (
          <button
            key={id}
            type="button"
            role="tab"
            aria-selected={tab === id}
            onClick={() => setTab(id)}
            className={`min-h-9 rounded-full px-4 type-callout font-semibold ${tab === id ? "bg-button text-on-button" : "bg-fill hover:bg-fill-selected"}`}
          >
            {label}
          </button>
        ))}
      </div>
      {tab === "queue" && <Queue server={server} />}
      {tab === "history" && <History server={server} />}
      {tab === "blocklist" && <Blocklist server={server} />}
      {tab === "import" && <FolderImport server={server} />}
    </Page>
  );
}

function stateName(s: DownloadState): string {
  switch (s) {
    case DownloadState.DOWNLOADING:
      return t("downloads.downloading");
    case DownloadState.IMPORT_BLOCKED:
      return t("downloads.importBlocked");
    case DownloadState.IMPORT_PENDING:
      return t("downloads.importPending");
    case DownloadState.IMPORTING:
      return t("downloads.importing");
    case DownloadState.IMPORTED:
      return t("downloads.imported");
    case DownloadState.FAILED_PENDING:
    case DownloadState.FAILED:
      return t("downloads.failed");
    case DownloadState.IGNORED:
      return t("downloads.ignored");
    default:
      return "";
  }
}

function statusName(s: DownloadStatus): string | undefined {
  switch (s) {
    case DownloadStatus.QUEUED:
      return t("downloads.queued");
    case DownloadStatus.PAUSED:
      return t("downloads.paused");
    case DownloadStatus.WARNING:
      return t("downloads.warning");
    default:
      return undefined;
  }
}

/** A link to an item's details, or its name alone. */
function ItemLink({ server, itemId, name }: { server: ServerEntry; itemId: string; name: string }) {
  if (!itemId) return <span>{name}</span>;
  return (
    <Link to="/s/$serverId/item/$itemId" params={{ serverId: server.id, itemId }} className="hover:underline">
      {name || t("downloads.item")}
    </Link>
  );
}

function Queue({ server }: { server: ServerEntry }) {
  const [finished, setFinished] = useState(false);
  const q = useQueue(server, "", finished);
  const items = q.data?.pages.flatMap((p) => p.items) ?? [];
  return (
    <>
      <Group footer={q.isSuccess ? t("downloads.count", { count: q.data.pages[0]?.total ?? 0 }) : undefined}>
        <div className="flex min-h-12 items-center justify-between gap-3 px-4 py-2">
          <span className="type-body">{t("downloads.showFinished")}</span>
          <Toggle on={finished} onChange={setFinished} label={t("downloads.showFinished")} />
        </div>
        {items.map((i) => (
          <QueueRow key={i.id} server={server} item={i} />
        ))}
        {q.isSuccess && items.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("downloads.queueEmpty")}</p>}
        {q.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(q.error)}</p>}
      </Group>
      <LoadMore active={q.hasNextPage && !q.isFetchingNextPage} onVisible={() => void q.fetchNextPage()} />
    </>
  );
}

export function QueueRow({ server, item: i }: { server: ServerEntry; item: QueueItem }) {
  const [panel, setPanel] = useState<"remove" | "import">();
  const size = Number(i.size);
  const progress = size > 0 && i.state === DownloadState.DOWNLOADING ? Math.max(0, Math.min(1, 1 - Number(i.remainingSize) / size)) : undefined;
  const status = statusName(i.status ?? DownloadStatus.UNSPECIFIED);
  const finished = i.state === DownloadState.IMPORTED || i.state === DownloadState.FAILED || i.state === DownloadState.IGNORED;
  const importable = i.state === DownloadState.IMPORT_BLOCKED || i.state === DownloadState.IMPORT_PENDING;
  const detail = [
    i.quality,
    i.languages.join(", "),
    size > 0 && sizeOf(size),
    i.indexerName,
    i.remainingTime && i.state === DownloadState.DOWNLOADING && t("downloads.left", { time: lengthOf(Number(durationMs(i.remainingTime))) }),
    i.grabTime && fromNow(timestampDate(i.grabTime)),
  ].filter(Boolean);
  return (
    <div className="flex flex-col gap-1.5 px-4 py-3">
      <div className="flex flex-wrap items-baseline gap-x-2">
        <span className="type-headline">
          <ItemLink server={server} itemId={i.itemId} name={i.itemName} />
        </span>
        <span className={`rounded-full px-2 py-0.5 type-caption font-semibold ${i.state === DownloadState.IMPORT_BLOCKED || i.state === DownloadState.FAILED || i.state === DownloadState.FAILED_PENDING ? "bg-danger text-white" : "bg-fill"}`}>
          {stateName(i.state)}
        </span>
        {status && <span className="type-footnote text-text-secondary">{status}</span>}
      </div>
      <p className="type-footnote break-all">{i.title}</p>
      <p className="type-footnote text-text-secondary">{detail.join(" · ")}</p>
      {progress !== undefined && (
        <div className="h-1.5 overflow-hidden rounded-full bg-fill" role="progressbar" aria-valuenow={Math.round(progress * 100)} aria-valuemin={0} aria-valuemax={100}>
          <div className="h-full rounded-full bg-progress" style={{ width: `${progress * 100}%` }} />
        </div>
      )}
      {i.messages.map((m, k) => (
        <p key={k} className="type-footnote break-words text-danger">
          {m}
        </p>
      ))}
      {!finished && (
        <div className="flex flex-wrap gap-2 pt-1">
          {importable && (
            <Button variant="secondary" onClick={() => setPanel(panel === "import" ? undefined : "import")}>
              {t("downloads.manualImport")}
            </Button>
          )}
          <Button variant="plain" className="text-danger" onClick={() => setPanel(panel === "remove" ? undefined : "remove")}>
            {t("downloads.remove")}
          </Button>
        </div>
      )}
      {panel === "remove" && <RemovePanel server={server} item={i} onDone={() => setPanel(undefined)} />}
      {panel === "import" && <ImportFiles server={server} target={{ id: i.id }} />}
    </div>
  );
}

function RemovePanel({ server, item, onDone }: { server: ServerEntry; item: QueueItem; onDone: () => void }) {
  const { remove } = useDownloadActions(server);
  const [fromClient, setFromClient] = useState(true);
  const [blocklist, setBlocklist] = useState(false);
  const [search, setSearch] = useState(false);
  const row = "flex min-h-11 items-center justify-between gap-3";
  return (
    <div className="flex flex-col rounded-lg bg-surface-raised px-3 py-2">
      <div className={row}>
        <span className="type-callout">{t("downloads.removeFromClient")}</span>
        <Toggle on={fromClient} onChange={setFromClient} label={t("downloads.removeFromClient")} />
      </div>
      <div className={row}>
        <span className="type-callout">{t("downloads.blocklistRelease")}</span>
        <Toggle on={blocklist} onChange={(v) => (setBlocklist(v), v || setSearch(false))} label={t("downloads.blocklistRelease")} />
      </div>
      {blocklist && (
        <div className={row}>
          <span className="type-callout">{t("downloads.searchAgain")}</span>
          <Toggle on={search} onChange={setSearch} label={t("downloads.searchAgain")} />
        </div>
      )}
      <div className="flex flex-wrap items-center gap-2 pt-1">
        <Button variant="plain" onClick={onDone}>
          {t("action.cancel")}
        </Button>
        <Button
          variant="secondary"
          className="text-danger"
          disabled={remove.isPending}
          onClick={() => remove.mutate({ id: item.id, removeFromClient: fromClient, blocklist, search }, { onSuccess: onDone })}
        >
          {t("downloads.remove")}
        </Button>
        {remove.isError && <span className="type-footnote break-words text-danger">{errorMessage(remove.error)}</span>}
      </div>
    </div>
  );
}

/** The files a manual import would take from a download or a folder, each imported on its own. */
function ImportFiles({ server, target }: { server: ServerEntry; target: { id?: string; folder?: string } }) {
  const files = useManualImport(server, target);
  const [mode, setMode] = useState(ImportMode.UNSPECIFIED);
  return (
    <div className="flex flex-col gap-2 rounded-lg bg-surface-raised px-3 py-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="flex-1 type-callout font-semibold">{t("downloads.files")}</span>
        <Select
          value={String(mode)}
          label={t("downloads.mode")}
          options={[
            { value: String(ImportMode.UNSPECIFIED), label: t("downloads.modeAuto") },
            { value: String(ImportMode.MOVE), label: t("downloads.modeMove") },
            { value: String(ImportMode.COPY), label: t("downloads.modeCopy") },
          ]}
          onChange={(v) => setMode(Number(v) as ImportMode)}
        />
      </div>
      {files.isPending && <p className="type-callout text-text-secondary">{t("state.loading")}</p>}
      {files.isError && <p className="type-footnote break-words text-danger">{errorMessage(files.error)}</p>}
      {files.isSuccess && files.data.length === 0 && <p className="type-callout text-text-secondary">{t("downloads.noFiles")}</p>}
      {files.data?.map((f) => (
        <ImportFile key={f.path} server={server} target={target} file={f} mode={mode} />
      ))}
    </div>
  );
}

function ImportFile({ server, target, file: f, mode }: { server: ServerEntry; target: { id?: string; folder?: string }; file: ManualImportFile; mode: ImportMode }) {
  const { manualImport } = useDownloadActions(server);
  const [done, setDone] = useState(false);
  const detail = [sizeOf(f.size), f.quality, f.languages.join(", "), f.releaseGroup].filter(Boolean);
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-border pt-2 first-of-type:border-t-0">
      <div className="min-w-0 flex-1">
        <p className="type-footnote break-all">{f.relativePath || f.path}</p>
        <p className="type-footnote text-text-secondary">{detail.join(" · ")}</p>
        {f.rejections.map((r, k) => (
          <p key={k} className="type-footnote break-words text-danger">
            {r}
          </p>
        ))}
        {manualImport.isError && <p className="type-footnote break-words text-danger">{errorMessage(manualImport.error)}</p>}
      </div>
      <Button
        variant="secondary"
        disabled={!f.movieId || manualImport.isPending || done}
        onClick={() => manualImport.mutate({ ...target, path: f.path, mode }, { onSuccess: () => setDone(true) })}
      >
        {done ? t("downloads.importedFile") : t("downloads.importFile")}
      </Button>
    </div>
  );
}

function historyName(type: HistoryEventType): string {
  switch (type) {
    case HistoryEventType.GRABBED:
      return t("downloads.grabbed");
    case HistoryEventType.IMPORTED:
      return t("downloads.imported");
    case HistoryEventType.DOWNLOAD_FAILED:
      return t("downloads.failed");
    case HistoryEventType.FILE_DELETED:
      return t("downloads.fileDeleted");
    case HistoryEventType.FILE_RENAMED:
      return t("downloads.fileRenamed");
    case HistoryEventType.IGNORED:
      return t("downloads.ignored");
    default:
      return "";
  }
}

function History({ server }: { server: ServerEntry }) {
  const [type, setType] = useState(HistoryEventType.UNSPECIFIED);
  const q = useHistory(server, "", type ? [type] : []);
  const events = q.data?.pages.flatMap((p) => p.events) ?? [];
  const types = [
    HistoryEventType.GRABBED,
    HistoryEventType.IMPORTED,
    HistoryEventType.DOWNLOAD_FAILED,
    HistoryEventType.FILE_DELETED,
    HistoryEventType.FILE_RENAMED,
    HistoryEventType.IGNORED,
  ];
  return (
    <>
      <Select
        value={String(type)}
        label={t("downloads.eventType")}
        className="self-start"
        options={[{ value: "0", label: t("downloads.allEvents") }, ...types.map((x) => ({ value: String(x), label: historyName(x) }))]}
        onChange={(v) => setType(Number(v) as HistoryEventType)}
      />
      <Group footer={q.isSuccess ? t("downloads.eventCount", { count: q.data.pages[0]?.total ?? 0 }) : undefined}>
        {events.map((e) => (
          <HistoryRow key={e.id} server={server} event={e} />
        ))}
        {q.isSuccess && events.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("downloads.historyEmpty")}</p>}
        {q.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(q.error)}</p>}
      </Group>
      <LoadMore active={q.hasNextPage && !q.isFetchingNextPage} onVisible={() => void q.fetchNextPage()} />
    </>
  );
}

export function HistoryRow({ server, event: e }: { server: ServerEntry; event: HistoryEvent }) {
  const { markFailed } = useDownloadActions(server);
  const [confirming, setConfirming] = useState(false);
  const data = Object.entries(e.data).toSorted(([a], [b]) => a.localeCompare(b));
  return (
    <div className="flex flex-col gap-1 px-4 py-3">
      <div className="flex flex-wrap items-baseline gap-x-2">
        <span className={`rounded-full px-2 py-0.5 type-caption font-semibold ${e.type === HistoryEventType.DOWNLOAD_FAILED ? "bg-danger text-white" : "bg-fill"}`}>
          {historyName(e.type)}
        </span>
        <span className="type-footnote text-text-secondary">{[e.quality, e.languages.join(", "), e.time && fromNow(timestampDate(e.time))].filter(Boolean).join(" · ")}</span>
      </div>
      <p className="type-footnote break-all">{e.sourceTitle}</p>
      {e.itemId && (
        <p className="type-footnote">
          <ItemLink server={server} itemId={e.itemId} name={t("downloads.item")} />
        </p>
      )}
      {data.length > 0 && (
        <p className="font-mono type-caption break-all text-text-secondary">
          {data.map(([k, v]) => (
            <span key={k} className="mr-3">
              {k}={v}
            </span>
          ))}
        </p>
      )}
      {e.type === HistoryEventType.GRABBED && (
        <div className="flex flex-wrap items-center gap-2 pt-1">
          {confirming ? (
            <>
              <span className="type-callout">{t("downloads.markFailedTitle")}</span>
              <Button variant="plain" onClick={() => setConfirming(false)}>
                {t("action.cancel")}
              </Button>
              <Button variant="secondary" className="text-danger" disabled={markFailed.isPending} onClick={() => markFailed.mutate(e.id, { onSuccess: () => setConfirming(false) })}>
                {t("downloads.markFailed")}
              </Button>
            </>
          ) : (
            <Button variant="plain" className="text-danger" onClick={() => setConfirming(true)}>
              {t("downloads.markFailed")}
            </Button>
          )}
          {markFailed.isError && <span className="type-footnote break-words text-danger">{errorMessage(markFailed.error)}</span>}
        </div>
      )}
    </div>
  );
}

function Blocklist({ server }: { server: ServerEntry }) {
  const q = useBlocklist(server);
  const entries = q.data?.pages.flatMap((p) => p.entries) ?? [];
  return (
    <>
      <Group footer={t("downloads.blocklistFooter")}>
        {entries.map((e) => (
          <BlocklistRow key={e.id} server={server} entry={e} />
        ))}
        {q.isSuccess && entries.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("downloads.blocklistEmpty")}</p>}
        {q.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(q.error)}</p>}
      </Group>
      <LoadMore active={q.hasNextPage && !q.isFetchingNextPage} onVisible={() => void q.fetchNextPage()} />
    </>
  );
}

function BlocklistRow({ server, entry: e }: { server: ServerEntry; entry: BlocklistEntry }) {
  const { unblock } = useDownloadActions(server);
  const detail = [e.quality, e.languages.join(", "), e.size > 0n && sizeOf(e.size), e.indexerName, e.time && fromNow(timestampDate(e.time))].filter(Boolean);
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 px-4 py-3">
      <div className="min-w-0 flex-1">
        <p className="type-footnote break-all">{e.sourceTitle}</p>
        <p className="type-footnote text-text-secondary">{detail.join(" · ")}</p>
        {e.message && <p className="type-footnote break-words text-text-secondary">{e.message}</p>}
        {e.itemId && (
          <p className="type-footnote">
            <ItemLink server={server} itemId={e.itemId} name={t("downloads.item")} />
          </p>
        )}
        {unblock.isError && <p className="type-footnote break-words text-danger">{errorMessage(unblock.error)}</p>}
      </div>
      <Button variant="plain" className="text-danger" disabled={unblock.isPending} onClick={() => unblock.mutate([e.id])}>
        {t("downloads.unblock")}
      </Button>
    </div>
  );
}

/** A folder's files, imported for the wanted movies their names match. */
function FolderImport({ server }: { server: ServerEntry }) {
  const [path, setPath] = useState("");
  const [folder, setFolder] = useState("");
  return (
    <Group footer={t("downloads.importFooter")}>
      <form
        className="flex flex-wrap items-center gap-3 px-4 py-2.5"
        onSubmit={(e) => {
          e.preventDefault();
          setFolder(path.trim());
        }}
      >
        <input className={`${input} flex-1 font-mono`} aria-label={t("downloads.folder")} placeholder={t("downloads.folder")} value={path} onChange={(e) => setPath(e.target.value)} />
        <Button type="submit" disabled={!path.trim()}>
          {t("downloads.scan")}
        </Button>
      </form>
      {folder && (
        <div className="px-4 pb-3">
          <ImportFiles key={folder} server={server} target={{ folder }} />
        </div>
      )}
    </Group>
  );
}

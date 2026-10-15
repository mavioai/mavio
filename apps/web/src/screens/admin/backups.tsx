// The settings' Backups and Logs (docs/clients.md §6.3.14): the server's
// backups, made, downloaded and deleted here, and its recent log records.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, useBackupDownload, useBackups, useLogs } from "@mavio/client-ts";
import type { Backup } from "@mavio/proto/mavio/system/v1/backup_pb";
import { LogLevel, type LogRecord } from "@mavio/proto/mavio/system/v1/system_pb";
import { useState } from "react";

import { locale, t } from "../../i18n.ts";
import { Button, Group, Row, Select } from "../../ui.tsx";
import { Confirm, fromNow, Page } from "./common.tsx";

const megabytes = new Intl.NumberFormat([locale], { style: "unit", unit: "megabyte", maximumFractionDigits: 1 });
const gigabytes = new Intl.NumberFormat([locale], { style: "unit", unit: "gigabyte", maximumFractionDigits: 2 });

/** A file's size in megabytes, or gigabytes from a thousand megabytes. */
function sizeOf(bytes: bigint) {
  const n = Number(bytes);
  return n >= 1e9 ? gigabytes.format(n / 1e9) : megabytes.format(n / 1e6);
}

/** The section of the server chosen in the settings. */
export function BackupsSection({ server }: { server: ServerEntry }) {
  const { list, create } = useBackups(server);
  return (
    <Page title={t("settings.backups")} message={t("backups.message")}>
      <Group footer={t("backups.restoreFooter")}>
        <Row label={t("backups.create")} detail={create.isPending ? t("backups.creating") : t("backups.createDetail")}>
          <Button disabled={create.isPending} onClick={() => create.mutate()}>
            {create.isPending ? "…" : t("backups.createAction")}
          </Button>
        </Row>
        {create.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(create.error)}</p>}
      </Group>
      <Group title={t("backups.list")}>
        {(list.data ?? []).map((b) => (
          <BackupRow key={b.name} server={server} backup={b} />
        ))}
        {list.isSuccess && list.data.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("backups.none")}</p>}
        {list.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(list.error)}</p>}
      </Group>
    </Page>
  );
}

function BackupRow({ server, backup }: { server: ServerEntry; backup: Backup }) {
  const { remove } = useBackups(server);
  const download = useBackupDownload(server);
  const [removing, setRemoving] = useState(false);
  const save = () =>
    download.mutate(backup.name, {
      onSuccess: (blob) => {
        const a = document.createElement("a");
        a.href = URL.createObjectURL(blob);
        a.download = backup.name;
        a.click();
        URL.revokeObjectURL(a.href);
      },
    });
  if (removing)
    return (
      <Confirm
        title={t("backups.removeTitle", { name: backup.name })}
        message={remove.isError ? errorMessage(remove.error) : t("backups.removeMessage")}
        action={t("backups.remove")}
        busy={remove.isPending}
        onCancel={() => setRemoving(false)}
        onConfirm={() => remove.mutate(backup.name, { onSuccess: () => setRemoving(false) })}
      />
    );
  const made = backup.createTime && timestampDate(backup.createTime);
  return (
    <>
      <Row
        label={backup.name}
        detail={[made && fromNow(made), sizeOf(backup.sizeBytes), backup.version && t("backups.version", { version: backup.version })].filter(Boolean).join(" · ")}
      >
        <Button variant="secondary" disabled={download.isPending} onClick={save}>
          {download.isPending ? "…" : t("backups.download")}
        </Button>
        <Button variant="plain" className="text-danger" onClick={() => setRemoving(true)}>
          {t("backups.remove")}
        </Button>
      </Row>
      {download.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(download.error)}</p>}
    </>
  );
}

const levelNames = (): Record<number, string> => ({
  [LogLevel.DEBUG]: t("logs.debug"),
  [LogLevel.INFO]: t("logs.info"),
  [LogLevel.WARN]: t("logs.warn"),
  [LogLevel.ERROR]: t("logs.error"),
});

const levelColors: Record<number, string> = {
  [LogLevel.DEBUG]: "text-text-tertiary",
  [LogLevel.INFO]: "text-text-secondary",
  [LogLevel.WARN]: "text-[#FF9F0A]",
  [LogLevel.ERROR]: "text-danger",
};

const clock = new Intl.DateTimeFormat([locale], { dateStyle: "short", timeStyle: "medium" });

/** The section of the server chosen in the settings. */
export function LogsSection({ server }: { server: ServerEntry }) {
  const [level, setLevel] = useState(LogLevel.INFO);
  const [hours, setHours] = useState(24);
  const [filter, setFilter] = useState("");
  const logs = useLogs(server, level, hours);
  const names = levelNames();
  const words = filter.toLowerCase().split(/\s+/).filter(Boolean);
  const shown = (logs.data ?? []).filter((r) => {
    const text = [r.message, ...Object.entries(r.attributes).flat()].join(" ").toLowerCase();
    return words.every((w) => text.includes(w));
  });
  return (
    <Page title={t("settings.logs")} message={t("logs.message")}>
      <div className="flex flex-wrap items-center gap-2">
        <Select
          value={String(level)}
          label={t("logs.level")}
          options={[LogLevel.DEBUG, LogLevel.INFO, LogLevel.WARN, LogLevel.ERROR].map((l) => ({ value: String(l), label: names[l] }))}
          onChange={(v) => setLevel(Number(v) as LogLevel)}
        />
        <Select
          value={String(hours)}
          label={t("logs.period")}
          options={[
            { value: "1", label: t("logs.lastHour") },
            { value: "24", label: t("logs.lastDay") },
            { value: "0", label: t("logs.all") },
          ]}
          onChange={(v) => setHours(Number(v))}
        />
        <input
          type="search"
          className="min-h-9 min-w-0 flex-1 rounded-md border border-border bg-surface px-3 type-body placeholder:text-text-tertiary focus:border-focus focus:outline-none"
          aria-label={t("logs.filter")}
          placeholder={t("logs.filter")}
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        <Button variant="secondary" disabled={logs.isFetching} onClick={() => void logs.refetch()}>
          {logs.isFetching ? "…" : t("logs.refresh")}
        </Button>
      </div>
      <Group footer={logs.isSuccess ? t("logs.count", { count: shown.length }) : undefined}>
        {shown.map((r, i) => (
          <LogRow key={i} record={r} level={names[r.level] ?? ""} />
        ))}
        {logs.isSuccess && shown.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("logs.none")}</p>}
        {logs.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(logs.error)}</p>}
      </Group>
    </Page>
  );
}

function LogRow({ record, level }: { record: LogRecord; level: string }) {
  const attrs = Object.entries(record.attributes).toSorted(([a], [b]) => a.localeCompare(b));
  return (
    <div className="flex flex-col gap-0.5 px-4 py-2">
      <div className="flex flex-wrap items-baseline gap-x-2 type-footnote">
        <span className="text-text-secondary tabular-nums">{record.time ? clock.format(timestampDate(record.time)) : ""}</span>
        <span className={`font-semibold ${levelColors[record.level] ?? ""}`}>{level}</span>
      </div>
      <p className="type-callout break-words">{record.message}</p>
      {attrs.length > 0 && (
        <p className="font-mono type-caption break-all text-text-secondary">
          {attrs.map(([k, v]) => (
            <span key={k} className="mr-3">
              {k}={v}
            </span>
          ))}
        </p>
      )}
    </div>
  );
}

// The settings' Backups, Logs and API Keys (docs/clients.md §6.3.14–§6.3.15):
// the server's backups, made, downloaded and deleted here, its recent log
// records, and the keys integrations call its API with.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, useApiKeys, useBackupDownload, useBackups, useLogs, useUsers } from "@mavio/client-ts";
import type { ApiKey } from "@mavio/proto/mavio/auth/v1/auth_pb";
import type { Backup } from "@mavio/proto/mavio/system/v1/backup_pb";
import { LogLevel, type LogRecord } from "@mavio/proto/mavio/system/v1/system_pb";
import { useState } from "react";

import { locale, t } from "../../i18n.ts";
import { Button, Group, Row, Select } from "../../ui.tsx";
import { Confirm, fromNow, input, Page, sizeOf } from "./common.tsx";

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

/** The section of the server chosen in the settings. */
export function ApiKeysSection({ server }: { server: ServerEntry }) {
  const { list, create } = useApiKeys(server);
  const [name, setName] = useState("");
  const [made, setMade] = useState<{ id: string; name: string; token: string }>();
  const [copied, setCopied] = useState(false);
  const users = useUsers(server);
  const userName = (id: string) => users.data?.find((u) => u.id === id)?.name ?? "";
  const add = () =>
    create.mutate(name.trim(), {
      onSuccess: (r) => (setMade({ id: r.apiKey?.id ?? "", name: r.apiKey?.name ?? name.trim(), token: r.token }), setName(""), setCopied(false)),
    });
  return (
    <Page title={t("settings.apiKeys")} message={t("keys.message")}>
      <Group footer={t("keys.createFooter")}>
        <form
          className="flex flex-wrap items-center gap-3 px-4 py-2.5"
          onSubmit={(e) => {
            e.preventDefault();
            add();
          }}
        >
          <input
            className={`${input} flex-1`}
            aria-label={t("keys.name")}
            placeholder={t("keys.name")}
            maxLength={100}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <Button type="submit" disabled={!name.trim() || create.isPending}>
            {create.isPending ? "…" : t("keys.create")}
          </Button>
        </form>
        {create.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(create.error)}</p>}
      </Group>
      {made && list.data?.some((k) => k.id === made.id) && (
        <Group title={t("keys.madeTitle", { name: made.name })} footer={t("keys.madeFooter")}>
          <div className="flex flex-wrap items-center gap-3 px-4 py-2.5">
            <code className="min-w-0 flex-1 font-mono type-callout break-all select-all">{made.token}</code>
            <Button variant="secondary" onClick={() => void navigator.clipboard?.writeText(made.token).then(() => setCopied(true))}>
              {copied ? t("users.copied") : t("keys.copy")}
            </Button>
            <Button variant="plain" onClick={() => setMade(undefined)}>
              {t("keys.done")}
            </Button>
          </div>
        </Group>
      )}
      <Group title={t("keys.list")}>
        {(list.data ?? []).map((k) => (
          <ApiKeyRow key={k.id} server={server} apiKey={k} userName={userName(k.userId)} />
        ))}
        {list.isSuccess && list.data.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("keys.none")}</p>}
        {list.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(list.error)}</p>}
      </Group>
    </Page>
  );
}

function ApiKeyRow({ server, apiKey, userName }: { server: ServerEntry; apiKey: ApiKey; userName: string }) {
  const { revoke } = useApiKeys(server);
  const [revoking, setRevoking] = useState(false);
  if (revoking)
    return (
      <Confirm
        title={t("keys.revokeTitle", { name: apiKey.name })}
        message={revoke.isError ? errorMessage(revoke.error) : t("keys.revokeMessage")}
        action={t("keys.revoke")}
        busy={revoke.isPending}
        onCancel={() => setRevoking(false)}
        onConfirm={() => revoke.mutate(apiKey.id, { onSuccess: () => setRevoking(false) })}
      />
    );
  const created = apiKey.createTime && timestampDate(apiKey.createTime);
  const used = apiKey.lastUseTime && timestampDate(apiKey.lastUseTime);
  return (
    <Row
      label={apiKey.name}
      detail={[
        userName && t("keys.actsAs", { name: userName }),
        created && t("keys.created", { when: fromNow(created) }),
        used ? t("keys.used", { when: fromNow(used) }) : t("keys.neverUsed"),
      ]
        .filter(Boolean)
        .join(" · ")}
    >
      <Button variant="plain" className="text-danger" onClick={() => setRevoking(true)}>
        {t("keys.revoke")}
      </Button>
    </Row>
  );
}

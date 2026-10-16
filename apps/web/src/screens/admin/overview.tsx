// The settings' Overview (docs/clients.md §6.3.6): the server chosen —
// its version and what it runs on, whether it serves and since when —
// who plays what on it, and what happened on it lately.
import { durationMs, timestampDate } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, useActivities, useAllSessions, useSystemInfo } from "@mavio/client-ts";
import { PlayMethod } from "@mavio/proto/mavio/playback/v1/playback_pb";
import type { Session } from "@mavio/proto/mavio/session/v1/session_pb";
import { type Activity, Severity } from "@mavio/proto/mavio/system/v1/activity_pb";
import { GetHealthResponse_Status } from "@mavio/proto/mavio/system/v1/system_pb";
import { useState } from "react";

import { Tip } from "../../controls/tooltip.tsx";
import { t } from "../../i18n.ts";
import { Group, Row, subtitleOf } from "../../ui.tsx";
import { Select } from "../../controls/select.tsx";
import { ColorTile, fromNow, lengthOf, Page } from "./common.tsx";

const methods: Partial<Record<PlayMethod, () => string>> = {
  [PlayMethod.DIRECT_PLAY]: () => t("overview.directPlay"),
  [PlayMethod.DIRECT_STREAM]: () => t("overview.directStream"),
  [PlayMethod.TRANSCODE]: () => t("overview.transcode"),
};

const severities: Record<Severity, { color: string; title: () => string }> = {
  [Severity.UNSPECIFIED]: { color: "var(--color-text-tertiary)", title: () => t("overview.severityInfo") },
  [Severity.INFO]: { color: "var(--color-text-tertiary)", title: () => t("overview.severityInfo") },
  [Severity.WARNING]: { color: "#FF9500", title: () => t("overview.severityWarning") },
  [Severity.ERROR]: { color: "#FF3B30", title: () => t("overview.severityError") },
};

/** The section of the server chosen in the settings. */
export function OverviewSection({ server }: { server: ServerEntry }) {
  return (
    <Page title={t("settings.overview")} message={server.name}>
      <ServerInfo server={server} />
      <NowPlaying server={server} />
      <Activities server={server} />
    </Page>
  );
}

function ServerInfo({ server }: { server: ServerEntry }) {
  const { info, health } = useSystemInfo(server);
  const i = info.data;
  const serving = health.data?.status === GetHealthResponse_Status.SERVING;
  const fact = (label: string, value?: string) =>
    value && (
      <Row key={label} label={label}>
        <span className="min-w-0 break-all text-right type-callout text-text-secondary">{value}</span>
      </Row>
    );
  return (
    <Group title={t("overview.server")}>
      <Row label={t("overview.status")}>
        <span className="flex items-center gap-2 type-callout text-text-secondary">
          <span className={`size-2 rounded-full ${health.isPending ? "bg-fill" : serving ? "bg-[#34C759]" : "bg-danger"}`} />
          {health.isPending ? "…" : serving ? t("overview.serving") : health.isError ? errorMessage(health.error) : t("overview.notServing")}
        </span>
      </Row>
      {fact(t("overview.version"), i?.version)}
      {fact(t("overview.platform"), i && `${i.os}/${i.arch}`)}
      {fact(t("overview.database"), i && (i.database === "postgres" ? "PostgreSQL" : i.database === "sqlite" ? "SQLite" : i.database))}
      {fact(t("overview.ffmpeg"), i && (i.ffmpegVersion || t("overview.ffmpegMissing")))}
      {fact(t("overview.uptime"), i?.startTime && lengthOf(Date.now() - timestampDate(i.startTime).getTime()))}
      {info.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(info.error)}</p>}
    </Group>
  );
}

function NowPlaying({ server }: { server: ServerEntry }) {
  const sessions = useAllSessions(server);
  const playing = (sessions.data ?? []).filter((s) => s.nowPlaying?.item);
  return (
    <Group title={t("overview.nowPlaying")}>
      {playing.map((s) => (
        <PlayingRow key={s.id} session={s} />
      ))}
      {sessions.isSuccess && playing.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("overview.nothingPlaying")}</p>}
      {sessions.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(sessions.error)}</p>}
    </Group>
  );
}

function PlayingRow({ session: s }: { session: Session }) {
  const np = s.nowPlaying;
  const item = np?.item;
  if (!np || !item) return null;
  const runtime = item.runtime ? durationMs(item.runtime) : 0;
  const position = np.position ? durationMs(np.position) : 0;
  const progress = runtime > 0 ? Math.min(1, position / runtime) : undefined;
  const sub = subtitleOf(item);
  return (
    <div className="flex flex-col gap-2 px-4 py-2.5">
      <div className="flex items-center gap-3">
        <ColorTile icon={np.paused ? "pause" : "play"} color={np.paused ? "#8E8E93" : "#34C759"} />
        <div className="min-w-0 flex-1">
          <div className="truncate type-body">{[item.name, sub].filter(Boolean).join(" · ")}</div>
          <div className="truncate type-footnote text-text-secondary">
            {[s.userName || s.pluginId, s.deviceName, s.client].filter(Boolean).join(" · ")}
          </div>
        </div>
        <span className="shrink-0 type-footnote text-text-secondary">{methods[np.method]?.()}</span>
      </div>
      {progress !== undefined && (
        <div className="flex items-center gap-3 pl-11">
          <div className="h-1 flex-1 overflow-hidden rounded-full bg-fill">
            <div className="h-full rounded-full bg-accent" style={{ width: `${progress * 100}%` }} />
          </div>
          <span className="shrink-0 type-caption text-text-secondary tabular-nums">
            {clock(position)} / {clock(runtime)}
          </span>
        </div>
      )}
    </div>
  );
}

/** A position as "1:02:03" or "2:03". */
function clock(ms: number): string {
  const s = Math.floor(ms / 1000);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const pad = (n: number) => String(n).padStart(2, "0");
  return h ? `${h}:${pad(m)}:${pad(s % 60)}` : `${m}:${pad(s % 60)}`;
}

function Activities({ server }: { server: ServerEntry }) {
  const [min, setMin] = useState(Severity.UNSPECIFIED);
  const [limit, setLimit] = useState(20);
  const activities = useActivities(server, min, limit);
  const list = activities.data?.activities ?? [];
  return (
    <Group
      title={
        <span className="flex items-center justify-between gap-3">
          {t("overview.activity")}
          <Select
            value={String(min)}
            label={t("overview.activityShown")}
            options={[
              { value: String(Severity.UNSPECIFIED), label: t("overview.activityAll") },
              { value: String(Severity.WARNING), label: t("overview.activityWarnings") },
              { value: String(Severity.ERROR), label: t("overview.activityErrors") },
            ]}
            onChange={(v) => setMin(Number(v) as Severity)}
          />
        </span>
      }
    >
      {list.map((a) => (
        <ActivityRow key={a.id} activity={a} />
      ))}
      {activities.isSuccess && list.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("overview.noActivity")}</p>}
      {activities.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(activities.error)}</p>}
      {(activities.data?.total ?? 0) > list.length && (
        <button type="button" className="min-h-11 px-4 text-left type-body text-accent-text hover:bg-surface-raised" onClick={() => setLimit(limit + 50)}>
          {t("overview.showMore")}
        </button>
      )}
    </Group>
  );
}

function ActivityRow({ activity: a }: { activity: Activity }) {
  const s = severities[a.severity];
  return (
    <div className="flex items-start gap-3 px-4 py-2.5">
      <span role="img" aria-label={s.title()} className="mt-2 size-2 shrink-0 rounded-full" style={{ background: s.color }} />
      <div className="min-w-0 flex-1">
        <div className="type-body">{a.title}</div>
        {a.message && <div className="type-footnote break-words text-text-secondary">{a.message}</div>}
      </div>
      {a.time && (
        <Tip label={timestampDate(a.time).toLocaleString()}>
          <time className="shrink-0 type-footnote text-text-secondary" dateTime={timestampDate(a.time).toISOString()}>
            {fromNow(timestampDate(a.time))}
          </time>
        </Tip>
      )}
    </div>
  );
}

// The settings' Activity (docs/clients.md §6.3.6): who plays what on the
// server chosen, and what happened on it lately.
import { durationMs, timestampDate } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, useActivities, useAllSessions } from "@mavio/client-ts";
import { PlayMethod } from "@mavio/proto/mavio/playback/v1/playback_pb";
import type { Session } from "@mavio/proto/mavio/session/v1/session_pb";
import { type Activity, Severity } from "@mavio/proto/mavio/system/v1/activity_pb";
import { useState } from "react";

import { Select } from "../../controls/select.tsx";
import { Tip } from "../../controls/tooltip.tsx";
import { t } from "../../i18n.ts";
import { Group, subtitleOf } from "../../ui.tsx";
import { ColorTile, fromNow, Page } from "./common.tsx";

const methods: Partial<Record<PlayMethod, () => string>> = {
  [PlayMethod.DIRECT_PLAY]: () => t("activity.directPlay"),
  [PlayMethod.DIRECT_STREAM]: () => t("activity.directStream"),
  [PlayMethod.TRANSCODE]: () => t("activity.transcode"),
};

const severities: Record<Severity, { color: string; title: () => string }> = {
  [Severity.UNSPECIFIED]: { color: "var(--color-text-tertiary)", title: () => t("activity.severityInfo") },
  [Severity.INFO]: { color: "var(--color-text-tertiary)", title: () => t("activity.severityInfo") },
  [Severity.WARNING]: { color: "#FF9500", title: () => t("activity.severityWarning") },
  [Severity.ERROR]: { color: "#FF3B30", title: () => t("activity.severityError") },
};

/** The section of who plays on the server chosen and what happened on it. */
export function ActivitySection({ server }: { server: ServerEntry }) {
  return (
    <Page title={t("settings.activity")} message={t("settings.activityMessage")}>
      <NowPlaying server={server} />
      <Activities server={server} />
    </Page>
  );
}

function NowPlaying({ server }: { server: ServerEntry }) {
  const sessions = useAllSessions(server);
  const playing = (sessions.data ?? []).filter((s) => s.nowPlaying?.item);
  return (
    <Group title={t("activity.nowPlaying")}>
      {playing.map((s) => (
        <PlayingRow key={s.id} session={s} />
      ))}
      {sessions.isSuccess && playing.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("activity.nothingPlaying")}</p>}
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
          {t("activity.recent")}
          <Select
            value={String(min)}
            label={t("activity.activityShown")}
            options={[
              { value: String(Severity.UNSPECIFIED), label: t("activity.activityAll") },
              { value: String(Severity.WARNING), label: t("activity.activityWarnings") },
              { value: String(Severity.ERROR), label: t("activity.activityErrors") },
            ]}
            onChange={(v) => setMin(Number(v) as Severity)}
          />
        </span>
      }
    >
      {list.map((a) => (
        <ActivityRow key={a.id} activity={a} />
      ))}
      {activities.isSuccess && list.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("activity.noActivity")}</p>}
      {activities.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(activities.error)}</p>}
      {(activities.data?.total ?? 0) > list.length && (
        <button type="button" className="min-h-11 px-4 text-left type-body text-accent-text hover:tint" onClick={() => setLimit(limit + 50)}>
          {t("activity.showMore")}
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

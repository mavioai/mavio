// The settings' Tasks (docs/clients.md §6.3.7): the server's recurring
// tasks, its own and its plugins', with how often they run, how their
// last run went and when they run next, to run now; and the jobs of its
// queue, filtered by state.
import { durationMs, timestampDate, type Timestamp } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, useJobs, useRunTask, useTasks } from "@mavio/client-ts";
import { type Job, JobState, type Task } from "@mavio/proto/mavio/system/v1/task_pb";
import { useState } from "react";

import { locale, t } from "../../i18n.ts";
import { Button, Group, Row, Select } from "../../ui.tsx";
import { fromNow, lengthOf, Page } from "./common.tsx";

const states: Record<JobState, { color: string; title: () => string }> = {
  [JobState.UNSPECIFIED]: { color: "var(--color-text-tertiary)", title: () => "" },
  [JobState.PENDING]: { color: "var(--color-text-tertiary)", title: () => t("tasks.pending") },
  [JobState.RUNNING]: { color: "#007AFF", title: () => t("tasks.running") },
  [JobState.SUCCEEDED]: { color: "#34C759", title: () => t("tasks.succeeded") },
  [JobState.FAILED]: { color: "#FF3B30", title: () => t("tasks.failed") },
};

const ago = (ts?: Timestamp) => (ts ? fromNow(timestampDate(ts)) : undefined);

/** The section of the server chosen in the settings. */
export function TasksSection({ server }: { server: ServerEntry }) {
  const tasks = useTasks(server);
  const byName = (a: Task, b: Task) => a.name.localeCompare(b.name, locale);
  const own = (tasks.data ?? []).filter((x) => !x.pluginId).sort(byName);
  const plugins = (tasks.data ?? []).filter((x) => x.pluginId).sort(byName);
  return (
    <Page title={t("settings.tasks")} message={t("tasks.message")}>
      {tasks.isError && <p className="type-callout text-danger">{errorMessage(tasks.error)}</p>}
      {own.length > 0 && <TaskGroup server={server} title={t("tasks.server")} tasks={own} />}
      {plugins.length > 0 && <TaskGroup server={server} title={t("settings.plugins")} tasks={plugins} />}
      <Jobs server={server} />
    </Page>
  );
}

function TaskGroup({ server, title, tasks }: { server: ServerEntry; title: string; tasks: Task[] }) {
  const run = useRunTask(server);
  // A notice of running a task, shown until its next run finishes.
  const [notices, setNotices] = useState<Record<string, { text: string; run?: string }>>({});
  const runOf = (task: Task) => task.lastRun?.id;
  return (
    <Group title={title}>
      {tasks.map((task) => {
        const last = task.lastRun;
        const status = task.running
          ? t("tasks.runningNow")
          : last
            ? `${states[last.state].title()} ${ago(last.finishTime ?? last.runTime) ?? ""}`.trim()
            : t("tasks.neverRun");
        const next = task.nextRunTime && !task.running ? t("tasks.next", { when: ago(task.nextRunTime) ?? "" }) : undefined;
        const every = task.interval ? t("tasks.every", { length: lengthOf(durationMs(task.interval)) }) : t("tasks.onDemand");
        const busy = run.isPending && run.variables === task.id;
        const notice = notices[task.id]?.run === runOf(task) ? notices[task.id]?.text : undefined;
        return (
          <div key={task.id} className="flex min-h-13 flex-wrap items-center gap-3 px-4 py-2.5">
            <div className="min-w-0 flex-1 basis-60">
              <div className="type-body">{task.name}</div>
              {task.description && <div className="type-footnote text-text-secondary">{task.description}</div>}
              <div className="mt-0.5 flex flex-wrap items-center gap-x-2 type-footnote text-text-secondary">
                <span className="inline-flex items-center gap-1.5">
                  <span
                    className={`size-2 rounded-full ${task.running ? "animate-pulse" : ""}`}
                    style={{ background: task.running ? states[JobState.RUNNING].color : last ? states[last.state].color : "var(--color-fill)" }}
                  />
                  {task.running ? status : (notice ?? status)}
                </span>
                <span>· {every}</span>
                {next && <span>· {next}</span>}
              </div>
              {last?.state === JobState.FAILED && last.lastError && !task.running && (
                <div className="mt-0.5 type-footnote break-words text-danger">{last.lastError}</div>
              )}
            </div>
            <Button
              variant="secondary"
              disabled={busy || task.running}
              onClick={() =>
                run.mutate(task.id, {
                  onSuccess: (r) =>
                    setNotices((n) => ({ ...n, [task.id]: { text: r.enqueued ? t("tasks.queued") : t("tasks.alreadyQueued"), run: runOf(task) } })),
                  onError: (e) => setNotices((n) => ({ ...n, [task.id]: { text: errorMessage(e), run: runOf(task) } })),
                })
              }
            >
              {busy ? "…" : t("tasks.run")}
            </Button>
          </div>
        );
      })}
    </Group>
  );
}

const filters: { value: string; states: JobState[]; title: () => string }[] = [
  { value: "all", states: [], title: () => t("tasks.jobsAll") },
  { value: "active", states: [JobState.PENDING, JobState.RUNNING], title: () => t("tasks.jobsActive") },
  { value: "failed", states: [JobState.FAILED], title: () => t("tasks.jobsFailed") },
  { value: "succeeded", states: [JobState.SUCCEEDED], title: () => t("tasks.jobsSucceeded") },
];

function Jobs({ server }: { server: ServerEntry }) {
  const [filter, setFilter] = useState("all");
  const [limit, setLimit] = useState(50);
  const jobs = useJobs(server, filters.find((f) => f.value === filter)?.states ?? [], limit);
  const list = jobs.data?.jobs ?? [];
  return (
    <Group
      title={
        <span className="flex items-center justify-between gap-3">
          {t("tasks.jobs")}
          <Select
            value={filter}
            label={t("tasks.jobsShown")}
            options={filters.map((f) => ({ value: f.value, label: f.title() }))}
            onChange={setFilter}
          />
        </span>
      }
      footer={t("tasks.jobsFooter")}
    >
      {list.map((j) => (
        <JobRow key={j.id} job={j} />
      ))}
      {jobs.isSuccess && list.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("tasks.noJobs")}</p>}
      {jobs.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(jobs.error)}</p>}
      {(jobs.data?.total ?? 0) > list.length && (
        <button type="button" className="min-h-11 px-4 text-left type-body text-accent-text hover:bg-surface-raised" onClick={() => setLimit(limit + 100)}>
          {t("tasks.showMore")}
        </button>
      )}
    </Group>
  );
}

function JobRow({ job: j }: { job: Job }) {
  const s = states[j.state];
  const when =
    j.state === JobState.PENDING
      ? j.runTime && t("tasks.runs", { when: ago(j.runTime) ?? "" })
      : j.finishTime
        ? ago(j.finishTime)
        : ago(j.runTime ?? j.createTime);
  return (
    <Row
      label={<span className="break-all">{j.kind}</span>}
      detail={
        <span className="flex flex-wrap gap-x-2 whitespace-normal">
          <span className="inline-flex items-center gap-1.5">
            <span className="size-2 rounded-full" style={{ background: s.color }} />
            {s.title()}
          </span>
          {when && <span>· {when}</span>}
          {j.attempts > 1 && <span>· {t("tasks.attempts", { attempts: j.attempts, max: j.maxAttempts })}</span>}
          {j.lastError && <span className="basis-full break-words text-danger">{j.lastError}</span>}
        </span>
      }
    />
  );
}

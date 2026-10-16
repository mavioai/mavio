// Issues (docs/clients.md §6.3.25): problems reported with a movie, series,
// season or episode from its More menu, an item's open issues on its
// details, the settings' issues of a server — the user's own, or everyone's
// for its administrators — and an issue with its comments, commented on,
// resolved, reopened and deleted.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  type ServerEntry,
  useCurrentUser,
  useIssue,
  useIssueActions,
  useIssues,
  useItem,
  useUsers,
} from "@mavio/client-ts";
import { type Issue, IssueStatus, IssueType } from "@mavio/proto/mavio/acquisition/v1/issue_pb";
import { type Item, ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { Chips, NumberField } from "../controls/inputs.tsx";
import { t } from "../i18n.ts";
import { Button, Group, LoadMore } from "../ui.tsx";
import { Select } from "../controls/select.tsx";
import { fromNow, Page } from "./admin/common.tsx";
import { episodeCode } from "./admin/wanted.tsx";
import { field } from "../controls/styles.ts";
import { Modal } from "../controls/dialog.tsx";

/** Kinds problems are reported with. */
export const reportable = (kind: ItemKind) => [ItemKind.MOVIE, ItemKind.SERIES, ItemKind.SEASON, ItemKind.EPISODE].includes(kind);

/** Whether the user may report problems on a server: administrators, and users allowed to. */
export function useMayReport(server: ServerEntry): boolean {
  const me = useCurrentUser(server).data;
  return !!me && (me.admin || !!me.policy?.reportIssues);
}

const types: { value: IssueType; label: () => string }[] = [
  { value: IssueType.VIDEO, label: () => t("issues.video") },
  { value: IssueType.AUDIO, label: () => t("issues.audio") },
  { value: IssueType.SUBTITLES, label: () => t("issues.subtitles") },
  { value: IssueType.OTHER, label: () => t("issues.other") },
];

const typeName = (type: IssueType) => types.find((x) => x.value === type)?.label() ?? "";

/** The season or episode of a series an issue is about. */
function numbersOf(i: Issue): string {
  if (i.seasonNumber <= 0) return "";
  return i.episodeNumber > 0 ? episodeCode(i.seasonNumber, i.episodeNumber) : t("discover.season", { number: i.seasonNumber });
}

/** The names of the users of a server: everyone's for administrators, else the user's own and "Administrator". */
function useNames(server: ServerEntry) {
  const me = useCurrentUser(server).data;
  const users = useUsers(server, !!me?.admin);
  return (id: string) => {
    if (id === server.userId) return server.userName;
    return users.data?.find((u) => u.id === id)?.name ?? t("issues.administrator");
  };
}

/** Reports a problem with an item: its kind, for a series the season and episode, and what is wrong. */
export function ReportDialog({ server, item, onClose }: { server: ServerEntry; item: Item; onClose: () => void }) {
  const actions = useIssueActions(server);
  const [type, setType] = useState(IssueType.VIDEO);
  const [season, setSeason] = useState(0);
  const [episode, setEpisode] = useState(0);
  const [message, setMessage] = useState("");
  const [opened, setOpened] = useState<string>();
  if (opened) return <IssueDialog server={server} id={opened} onClose={onClose} />;
  const series = item.kind === ItemKind.SERIES;
  return (
    <Modal title={t("issues.reportTitle", { name: item.name })} onClose={onClose}>
      <form
        className="flex min-h-0 flex-col gap-3 overflow-y-auto"
        onSubmit={(e) => {
          e.preventDefault();
          actions.create.mutate(
            { itemId: item.id, type, seasonNumber: series ? season : 0, episodeNumber: series && season > 0 ? episode : 0, message: message.trim() },
            { onSuccess: (issue) => (issue ? setOpened(issue.id) : onClose()) },
          );
        }}
      >
        <Chips
          value={String(type)}
          label={t("issues.type")}
          options={types.map((x) => ({ value: String(x.value), label: x.label() }))}
          onChange={(v) => setType(Number(v) as IssueType)}
        />
        {series && (
          <div className="flex gap-3">
            <div className="flex flex-1 flex-col gap-1 type-footnote text-text-secondary">
              {t("issues.season")}
              <NumberField label={t("issues.season")} min={0} value={season} onChange={(v) => setSeason(v ?? 0)} />
            </div>
            <div className="flex flex-1 flex-col gap-1 type-footnote text-text-secondary">
              {t("issues.episode")}
              <NumberField label={t("issues.episode")} min={0} disabled={season === 0} value={season === 0 ? 0 : episode} onChange={(v) => setEpisode(v ?? 0)} />
            </div>
          </div>
        )}
        {series && <p className="type-footnote text-text-secondary">{t("issues.allNumbers")}</p>}
        <textarea
          className={`${field} min-h-28 py-2`}
          aria-label={t("issues.message")}
          placeholder={t("issues.message")}
          maxLength={10000}
          value={message}
          onChange={(e) => setMessage(e.target.value)}
        />
        {actions.create.error && <p className="type-footnote break-words text-danger">{errorMessage(actions.create.error)}</p>}
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={!message.trim() || actions.create.isPending}>
            {t("issues.send")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

/** An issue: its item, kind and status, its comments, a comment to add, and resolving, reopening or deleting it. */
export function IssueDialog({ server, id, onClose }: { server: ServerEntry; id: string; onClose: () => void }) {
  const issue = useIssue(server, id);
  const actions = useIssueActions(server);
  const nameOf = useNames(server);
  const [comment, setComment] = useState("");
  const [deleting, setDeleting] = useState(false);
  const i = issue.data;
  const item = useItem(i ? server : undefined, i?.itemId ?? "");
  const busy = actions.resolve.isPending || actions.reopen.isPending || actions.remove.isPending;
  const error = issue.error ?? actions.comment.error ?? actions.resolve.error ?? actions.reopen.error ?? actions.remove.error;
  const open = i?.status === IssueStatus.OPEN;
  return (
    <Modal title={i ? [typeName(i.type), item.data?.item?.name].filter(Boolean).join(" · ") : t("issues.title")} wide onClose={onClose}>
      {i && (
        <>
          <div className="flex flex-wrap items-center gap-2 type-footnote text-text-secondary">
            <StatusBadge status={i.status} />
            {item.data?.item && (
              <Link to="/s/$serverId/item/$itemId" params={{ serverId: server.id, itemId: i.itemId }} className="text-accent-text hover:underline" onClick={onClose}>
                {item.data.item.name}
              </Link>
            )}
            {numbersOf(i) && <span>{numbersOf(i)}</span>}
            <span>{t("issues.reportedBy", { name: nameOf(i.createdBy) })}</span>
          </div>
          <ol className="flex min-h-0 flex-col gap-3 overflow-y-auto">
            {i.comments.map((c) => (
              <li key={c.id} className="rounded-[12px] bg-fill px-3.5 py-2.5">
                <p className="type-footnote text-text-secondary">
                  {[nameOf(c.userId), c.createTime ? fromNow(timestampDate(c.createTime)) : ""].filter(Boolean).join(" · ")}
                </p>
                <p className="type-body break-words whitespace-pre-line">{c.message}</p>
              </li>
            ))}
          </ol>
          <form
            className="flex items-end gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              actions.comment.mutate({ id, message: comment.trim() }, { onSuccess: () => setComment("") });
            }}
          >
            <textarea
              className={`${field} min-h-11 py-2`}
              aria-label={t("issues.comment")}
              placeholder={t("issues.comment")}
              maxLength={10000}
              rows={2}
              value={comment}
              onChange={(e) => setComment(e.target.value)}
            />
            <Button type="submit" variant="secondary" disabled={!comment.trim() || actions.comment.isPending}>
              {t("issues.commentSend")}
            </Button>
          </form>
        </>
      )}
      {error && <p className="type-footnote break-words text-danger">{errorMessage(error)}</p>}
      {deleting ? (
        <div className="flex flex-wrap items-center justify-end gap-2">
          <span className="mr-auto type-callout">{t("issues.confirmDelete")}</span>
          <Button variant="plain" onClick={() => setDeleting(false)}>
            {t("action.cancel")}
          </Button>
          <Button variant="secondary" className="text-danger" disabled={busy} onClick={() => actions.remove.mutate(id, { onSuccess: onClose })}>
            {t("issues.delete")}
          </Button>
        </div>
      ) : (
        <div className="flex flex-wrap justify-end gap-2">
          {i && (
            <Button variant="plain" className="mr-auto text-danger" onClick={() => setDeleting(true)}>
              {t("issues.delete")}
            </Button>
          )}
          {i && (
            <Button variant="secondary" disabled={busy} onClick={() => (open ? actions.resolve : actions.reopen).mutate(id)}>
              {open ? t("issues.resolve") : t("issues.reopen")}
            </Button>
          )}
          <Button onClick={onClose}>{t("keys.done")}</Button>
        </div>
      )}
    </Modal>
  );
}

function StatusBadge({ status }: { status: IssueStatus }) {
  const open = status === IssueStatus.OPEN;
  return (
    <span className={`rounded-full px-2 py-0.5 type-caption font-semibold ${open ? "bg-accent text-on-accent" : "bg-[#34C759] text-white"}`}>
      {open ? t("issues.open") : t("issues.resolved")}
    </span>
  );
}

/** An issue in a list: its item, kind, numbers and status, who reported it and when it last changed. */
function IssueRow({ server, issue: i, onOpen, item: showItem = true }: { server: ServerEntry; issue: Issue; onOpen: () => void; item?: boolean }) {
  const item = useItem(showItem ? server : undefined, i.itemId);
  const nameOf = useNames(server);
  const label = [showItem ? item.data?.item?.name : undefined, typeName(i.type), numbersOf(i)].filter(Boolean).join(" · ");
  const first = i.comments[0]?.message ?? "";
  return (
    <button type="button" onClick={onOpen} className="flex min-h-13 w-full items-center gap-3 px-4 py-2.5 text-left hover:bg-surface-raised">
      <span className="min-w-0 flex-1">
        <span className="block truncate type-body">{label}</span>
        <span className="block truncate type-footnote text-text-secondary">
          {[first, nameOf(i.createdBy), i.updateTime ? fromNow(timestampDate(i.updateTime)) : ""].filter(Boolean).join(" · ")}
        </span>
      </span>
      <StatusBadge status={i.status} />
    </button>
  );
}

/** The open issues of an item on its details, for whoever may see them. */
export function ItemIssues({ server, item }: { server: ServerEntry; item: Item }) {
  const list = useIssues(server, [IssueStatus.OPEN], item.id);
  const [open, setOpen] = useState<string>();
  const issues = list.data?.pages.flatMap((p) => p.issues) ?? [];
  if (issues.length === 0) return null;
  return (
    <section className="flex flex-col gap-4 px-margin pt-8">
      <h2 className="type-title3">{t("issues.itemTitle")}</h2>
      <div className="flex max-w-3xl flex-col divide-y divide-border overflow-hidden rounded-[14px] bg-surface">
        {issues.map((i) => (
          <IssueRow key={i.id} server={server} issue={i} item={false} onOpen={() => setOpen(i.id)} />
        ))}
      </div>
      {open && <IssueDialog server={server} id={open} onClose={() => setOpen(undefined)} />}
    </section>
  );
}

type Filter = "open" | "resolved" | "all";

const filters: Record<Filter, IssueStatus[]> = { open: [IssueStatus.OPEN], resolved: [IssueStatus.RESOLVED], all: [] };

/** The settings' issues of the server chosen, open ones at first. */
export function IssuesSection({ server }: { server: ServerEntry }) {
  const [filter, setFilter] = useState<Filter>("open");
  const list = useIssues(server, filters[filter]);
  const [open, setOpen] = useState<string>();
  const pages = list.data?.pages ?? [];
  const issues = pages.flatMap((p) => p.issues);
  return (
    <Page title={t("settings.issues")} message={t("issues.intro")}>
      <Select
        value={filter}
        label={t("issues.filter")}
        className="self-start"
        options={[
          { value: "open", label: t("issues.open") },
          { value: "resolved", label: t("issues.resolved") },
          { value: "all", label: t("issues.all") },
        ]}
        onChange={setFilter}
      />
      <Group footer={list.isSuccess ? t("issues.count", { count: pages[0]?.total ?? 0 }) : undefined}>
        {issues.map((i) => (
          <IssueRow key={i.id} server={server} issue={i} onOpen={() => setOpen(i.id)} />
        ))}
        {list.isSuccess && issues.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("issues.none")}</p>}
        {list.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(list.error)}</p>}
      </Group>
      <LoadMore active={list.hasNextPage && !list.isFetchingNextPage} onVisible={() => void list.fetchNextPage()} />
      {open && <IssueDialog server={server} id={open} onClose={() => setOpen(undefined)} />}
    </Page>
  );
}

// Requests (docs/clients.md §6.3.2): the requests of the server that
// discovers, the user's own or, for its administrators, everyone's, by
// status; administrators approve, decline and retry them, and the seasons
// of a pending series request are changed and pending requests deleted.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  type ServerEntry,
  useAdminServers,
  useQuota,
  useRequestActions,
  useRequests,
  useServer,
  useTitle,
  useUsers,
} from "@mavio/client-ts";
import { type Quota, type Request, RequestStatus } from "@mavio/proto/mavio/acquisition/v1/request_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { getRouteApi, Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { t } from "../i18n.ts";
import { Artwork, Button, LoadMore, Loading, ScreenTitle, StatusView } from "../ui.tsx";
import { fromNow } from "./admin/common.tsx";
import { kindParam, RequestDialog, sized } from "./discover.tsx";

const route = getRouteApi("/s/$serverId/requests");

/** The statuses a screen's filter shows, by its name in addresses. */
export const requestFilters = {
  pending: [RequestStatus.PENDING],
  approved: [RequestStatus.APPROVED],
  available: [RequestStatus.COMPLETED],
  failed: [RequestStatus.FAILED],
  declined: [RequestStatus.DECLINED],
} as const;

export type RequestFilter = keyof typeof requestFilters;

const filterNames: Record<RequestFilter | "all", () => string> = {
  all: () => t("requests.all"),
  pending: () => t("requests.pending"),
  approved: () => t("requests.approved"),
  available: () => t("requests.available"),
  failed: () => t("requests.failed"),
  declined: () => t("requests.declined"),
};

function statusName(status: RequestStatus): string {
  switch (status) {
    case RequestStatus.PENDING:
      return t("requests.pending");
    case RequestStatus.APPROVED:
      return t("requests.approved");
    case RequestStatus.COMPLETED:
      return t("requests.available");
    case RequestStatus.FAILED:
      return t("requests.failed");
    case RequestStatus.DECLINED:
      return t("requests.declined");
    default:
      return "";
  }
}

const statusStyles: Partial<Record<RequestStatus, string>> = {
  [RequestStatus.PENDING]: "bg-fill",
  [RequestStatus.APPROVED]: "bg-accent text-on-accent",
  [RequestStatus.COMPLETED]: "bg-[#34C759] text-white",
  [RequestStatus.FAILED]: "bg-danger text-white",
  [RequestStatus.DECLINED]: "bg-fill text-text-secondary",
};

/** Requests of a server, filtered by status. */
export function RequestsScreen() {
  const { serverId } = route.useParams();
  const { status } = route.useSearch();
  const navigate = useNavigate();
  const server = useServer(serverId);
  const admin = useAdminServers().some((s) => s.id === serverId);
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  return (
    <div className="pt-2 pb-8">
      <ScreenTitle>{t("requests.title")}</ScreenTitle>
      <div className="flex flex-wrap gap-2 px-margin pb-3" role="tablist">
        {(["all", ...Object.keys(requestFilters)] as (RequestFilter | "all")[]).map((f) => {
          const on = (status ?? "all") === f;
          return (
            <button
              key={f}
              type="button"
              role="tab"
              aria-selected={on}
              onClick={() => void navigate({ to: "/s/$serverId/requests", params: { serverId }, search: { status: f === "all" ? undefined : f }, replace: true })}
              className={`min-h-9 rounded-full px-4 type-callout font-semibold ${on ? "bg-button text-on-button" : "bg-fill hover:bg-fill-selected"}`}
            >
              {filterNames[f]()}
            </button>
          );
        })}
      </div>
      {!admin && <Quotas server={server} />}
      <RequestList key={status ?? "all"} server={server} statuses={status ? [...requestFilters[status]] : []} admin={admin} />
    </div>
  );
}

function quotaText(q: Quota | undefined, movies: boolean): string | undefined {
  if (!q || q.limit === 0) return undefined;
  const args = { remaining: q.remaining, limit: q.limit, days: q.days };
  if (movies) return q.days > 0 ? t("requests.movieQuotaDays", args) : t("requests.movieQuota", args);
  return q.days > 0 ? t("requests.seasonQuotaDays", args) : t("requests.seasonQuota", args);
}

/** How many more movies and seasons the user may request, when limited. */
function Quotas({ server }: { server: ServerEntry }) {
  const quota = useQuota(server);
  const lines = [quotaText(quota.data?.movie, true), quotaText(quota.data?.series, false)].filter(Boolean);
  if (lines.length === 0) return null;
  return <p className="px-margin pb-3 type-callout text-text-secondary">{lines.join(" · ")}</p>;
}

function RequestList({ server, statuses, admin }: { server: ServerEntry; statuses: RequestStatus[]; admin: boolean }) {
  const q = useRequests(server, statuses);
  const users = useUsers(server, admin);
  const nameOf = (id: string) => (admin ? (users.data?.find((u) => u.id === id)?.name ?? "") : "");
  if (q.isPending) return <Loading />;
  if (q.isError) return <StatusView icon="error" title={t("state.error")} message={errorMessage(q.error)} />;
  const requests = q.data.pages.flatMap((p) => p.requests);
  if (requests.length === 0) return <StatusView icon="discover" title={t("requests.none")} message={t("requests.noneMessage")} />;
  return (
    <>
      <div className="mx-margin flex flex-col divide-y divide-border overflow-hidden rounded-[14px] bg-surface">
        {requests.map((r) => (
          <RequestRow key={r.id} server={server} request={r} admin={admin} userName={nameOf(r.requestedBy)} />
        ))}
      </div>
      <LoadMore active={q.hasNextPage && !q.isFetchingNextPage} onVisible={() => void q.fetchNextPage()} />
    </>
  );
}

function seasonsText(r: Request): string {
  const numbers = r.seasons.map((s) => s.seasonNumber).sort((a, b) => a - b);
  if (numbers.length === 0) return "";
  return numbers.map((n) => (n === 0 ? t("discover.specials") : t("discover.season", { number: n }))).join(", ");
}

function RequestRow({ server, request: r, admin, userName }: { server: ServerEntry; request: Request; admin: boolean; userName: string }) {
  const series = r.mediaKind === MediaKind.SERIES;
  const title = useTitle(server, r.mediaKind, r.tmdbId);
  const actions = useRequestActions(server);
  const [editing, setEditing] = useState(false);
  const [removing, setRemoving] = useState(false);
  const pending = r.status === RequestStatus.PENDING;
  const made = r.createTime && timestampDate(r.createTime);
  const error = [actions.approve, actions.decline, actions.retry, actions.update, actions.remove].find((m) => m.isError)?.error;
  const busy = [actions.approve, actions.decline, actions.retry, actions.update, actions.remove].some((m) => m.isPending);
  const detail = [
    series ? t("discover.series") : t("discover.movie"),
    seasonsText(r),
    userName && t("requests.by", { name: userName }),
    made && fromNow(made),
  ].filter(Boolean);
  return (
    <div className="flex gap-3 px-4 py-3">
      <Link
        to="/s/$serverId/title/$kind/$tmdbId"
        params={{ serverId: server.id, kind: kindParam(r.mediaKind), tmdbId: String(r.tmdbId) }}
        className="w-14 shrink-0 sm:w-16"
      >
        <Artwork src={sized(title.data?.posterUrl ?? "", 154)} alt="" className="aspect-[2/3] rounded-md" />
      </Link>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex flex-wrap items-baseline gap-x-2">
          <Link
            to="/s/$serverId/title/$kind/$tmdbId"
            params={{ serverId: server.id, kind: kindParam(r.mediaKind), tmdbId: String(r.tmdbId) }}
            className="type-headline hover:underline"
          >
            {r.title || title.data?.name || r.tmdbId}
          </Link>
          {r.year > 0 && <span className="type-callout text-text-secondary">{r.year}</span>}
          <span className={`rounded-full px-2 py-0.5 type-caption font-semibold ${statusStyles[r.status] ?? "bg-fill"}`}>{statusName(r.status)}</span>
        </div>
        <p className="type-footnote break-words text-text-secondary">{detail.join(" · ")}</p>
        {r.status === RequestStatus.FAILED && r.message && <p className="type-footnote break-words text-danger">{r.message}</p>}
        {removing ? (
          <div className="flex flex-wrap items-center gap-2 pt-1">
            <span className="type-callout">{t("requests.removeTitle")}</span>
            <Button variant="plain" onClick={() => setRemoving(false)}>
              {t("action.cancel")}
            </Button>
            <Button variant="secondary" className="text-danger" disabled={busy} onClick={() => actions.remove.mutate(r.id)}>
              {t("requests.remove")}
            </Button>
          </div>
        ) : (
          <div className="flex flex-wrap gap-2 pt-1">
            {admin && pending && (
              <>
                <Button disabled={busy} onClick={() => actions.approve.mutate(r.id)}>
                  {t("requests.approve")}
                </Button>
                <Button variant="secondary" disabled={busy} onClick={() => actions.decline.mutate(r.id)}>
                  {t("requests.decline")}
                </Button>
              </>
            )}
            {admin && r.status === RequestStatus.FAILED && (
              <Button disabled={busy} onClick={() => actions.retry.mutate(r.id)}>
                {t("requests.retry")}
              </Button>
            )}
            {pending && series && title.data && (
              <Button variant="secondary" disabled={busy} onClick={() => setEditing(true)}>
                {t("requests.editSeasons")}
              </Button>
            )}
            {(admin || pending) && (
              <Button variant="plain" className="text-danger" disabled={busy} onClick={() => setRemoving(true)}>
                {t("requests.remove")}
              </Button>
            )}
          </div>
        )}
        {error != null && <p className="type-footnote break-words text-danger">{errorMessage(error)}</p>}
      </div>
      {editing && title.data && (
        <RequestDialog
          title={title.data}
          requested={r.seasons.map((s) => s.seasonNumber)}
          busy={actions.update.isPending}
          onCancel={() => setEditing(false)}
          onRequest={(seasons) => actions.update.mutate({ id: r.id, seasons: seasons ?? [] }, { onSettled: () => setEditing(false) })}
        />
      )}
    </div>
  );
}

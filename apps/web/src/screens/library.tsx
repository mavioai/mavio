// A library: its Recommended tab of hubs and the grid of all its items,
// sorted and filtered (docs/clients.md §6.3.1).
import {
  type Filter,
  type ServerEntry,
  type Sort,
  useLibrary,
  useLibraryItems,
  useRecommended,
  useServer,
} from "@mavio/client-ts";
import type { MessageKey } from "@mavio/design/strings";
import { getRouteApi, Link, useNavigate } from "@tanstack/react-router";

import { t } from "../i18n.ts";
import { Grid, ItemCard, LoadMore, Loading, ScreenTitle, StatusView } from "../ui.tsx";
import { SectionView } from "./home.tsx";

const route = getRouteApi("/s/$serverId/library/$libraryId");

const sortLabels: Record<Sort, MessageKey> = {
  name: "library.sortName",
  dateAdded: "library.sortDateAdded",
  released: "library.sortReleased",
  rating: "library.sortRating",
  lastPlayed: "library.sortLastPlayed",
  random: "library.sortRandom",
};

const filterLabels: Record<Filter, MessageKey> = {
  all: "library.filterAll",
  unplayed: "library.filterUnplayed",
  played: "library.filterPlayed",
  favorites: "library.filterFavorites",
};

export function LibraryScreen() {
  const { serverId, libraryId } = route.useParams();
  const server = useServer(serverId);
  if (!server?.token) return <StatusView icon="error" title={t("auth.signedOut")} />;
  return <Library server={server} libraryId={libraryId} />;
}

function Library({ server, libraryId }: { server: ServerEntry; libraryId: string }) {
  const { tab } = route.useSearch();
  const library = useLibrary(server, libraryId);
  const tabClass =
    "rounded-full px-4 py-1.5 type-headline text-text-secondary hover:text-on-background data-[active=true]:bg-on-background data-[active=true]:text-background";
  return (
    <div>
      <ScreenTitle>{library.data?.name ?? " "}</ScreenTitle>
      <div className="flex gap-2 px-margin pb-2">
        <Link from="/s/$serverId/library/$libraryId" search={{}} data-active={tab !== "all"} className={tabClass}>
          {t("library.recommended")}
        </Link>
        <Link
          from="/s/$serverId/library/$libraryId"
          search={(s) => ({ ...s, tab: "all" as const })}
          data-active={tab === "all"}
          className={tabClass}
        >
          {t("library.all")}
        </Link>
      </div>
      {tab === "all" ? <AllItems server={server} libraryId={libraryId} /> : <Recommended server={server} libraryId={libraryId} />}
    </div>
  );
}

function Recommended({ server, libraryId }: { server: ServerEntry; libraryId: string }) {
  const q = useRecommended(server, libraryId);
  if (q.isPending) return <Loading />;
  if (q.isError) {
    return <StatusView icon="error" title={t("state.error")} action={{ label: t("action.retry"), onClick: () => void q.refetch() }} />;
  }
  if (q.data.length === 0) return <StatusView title={t("state.empty")} />;
  return (
    <div>
      {q.data.map((s) => (
        <SectionView key={s.id} section={s} several={false} />
      ))}
    </div>
  );
}

function AllItems({ server, libraryId }: { server: ServerEntry; libraryId: string }) {
  const { sort = "name", filter = "all" } = route.useSearch();
  const navigate = useNavigate({ from: "/s/$serverId/library/$libraryId" });
  const q = useLibraryItems(server, libraryId, sort, filter);
  const select =
    "min-h-9 rounded-md border border-border bg-surface px-2 type-callout text-on-background focus:border-focus focus:outline-none";
  const total = q.data?.pages[0]?.total;
  return (
    <div>
      <div className="flex flex-wrap items-center gap-3 px-margin pt-2 pb-4">
        <label className="flex items-center gap-2 type-callout text-text-secondary">
          {t("library.sort")}
          <select
            className={select}
            value={sort}
            onChange={(e) => void navigate({ search: (s) => ({ ...s, sort: e.target.value as Sort }) })}
          >
            {(Object.keys(sortLabels) as Sort[]).map((k) => (
              <option key={k} value={k}>
                {t(sortLabels[k])}
              </option>
            ))}
          </select>
        </label>
        <label className="flex items-center gap-2 type-callout text-text-secondary">
          {t("library.filter")}
          <select
            className={select}
            value={filter}
            onChange={(e) =>
              void navigate({
                search: (s) => ({ ...s, filter: e.target.value === "all" ? undefined : (e.target.value as Filter) }),
              })
            }
          >
            {(Object.keys(filterLabels) as Filter[]).map((k) => (
              <option key={k} value={k}>
                {t(filterLabels[k])}
              </option>
            ))}
          </select>
        </label>
        {total !== undefined && (
          <span className="ml-auto type-callout text-text-tertiary">{t("library.itemCount", { count: total })}</span>
        )}
      </div>
      {q.isPending ? (
        <Loading />
      ) : q.isError ? (
        <StatusView icon="error" title={t("state.error")} action={{ label: t("action.retry"), onClick: () => void q.refetch() }} />
      ) : total === 0 ? (
        <StatusView title={t("state.empty")} />
      ) : (
        <>
          <Grid>
            {q.data.pages.flatMap((p) =>
              p.items.map((item) => (
                <ItemCard key={item.id} server={server} serverId={server.id} item={item} fluid />
              )),
            )}
          </Grid>
          <LoadMore active={q.hasNextPage && !q.isFetchingNextPage} onVisible={() => void q.fetchNextPage()} />
        </>
      )}
    </div>
  );
}

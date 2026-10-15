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

import { useCallback, useRef, useState } from "react";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { Grid, ItemCard, LoadMore, Loading, StatusView, useDismiss } from "../ui.tsx";
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
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  return <Library server={server} libraryId={libraryId} />;
}

function Library({ server, libraryId }: { server: ServerEntry; libraryId: string }) {
  const { tab } = route.useSearch();
  const library = useLibrary(server, libraryId);
  const segment =
    "rounded-[7px] px-4 py-1 type-callout font-semibold text-text-secondary hover:text-on-background data-[active=true]:bg-surface-overlay data-[active=true]:text-on-background data-[active=true]:shadow-level1";
  return (
    <div>
      <div className="flex flex-wrap items-end gap-x-6 gap-y-3 px-margin pt-6 pb-3">
        <h1 className="type-title1">{library.data?.name ?? " "}</h1>
        <div role="tablist" className="flex rounded-[9px] bg-fill p-0.5">
          <Link role="tab" from="/s/$serverId/library/$libraryId" search={{}} data-active={tab !== "all"} className={segment}>
            {t("library.recommended")}
          </Link>
          <Link
            role="tab"
            from="/s/$serverId/library/$libraryId"
            search={(s) => ({ ...s, tab: "all" as const })}
            data-active={tab === "all"}
            className={segment}
          >
            {t("library.all")}
          </Link>
        </div>
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
  const total = q.data?.pages[0]?.total;
  return (
    <div>
      <div className="flex flex-wrap items-center gap-2 px-margin pt-1 pb-5">
        {(Object.keys(filterLabels) as Filter[]).map((k) => (
          <Link
            key={k}
            from="/s/$serverId/library/$libraryId"
            search={(s) => ({ ...s, filter: k === "all" ? undefined : k })}
            data-active={filter === k}
            className="inline-flex h-8 items-center rounded-full bg-fill px-3.5 type-callout font-medium hover:bg-fill-selected data-[active=true]:bg-button data-[active=true]:text-on-button"
          >
            {t(filterLabels[k])}
          </Link>
        ))}
        <SortMenu sort={sort} onSort={(k) => void navigate({ search: (s) => ({ ...s, sort: k }) })} />
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

/** The button naming the order of the items, which opens the orders to choose from. */
function SortMenu({ sort, onSort }: { sort: Sort; onSort: (s: Sort) => void }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const close = useCallback(() => setOpen(false), []);
  useDismiss(ref, open, close);
  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="inline-flex h-8 items-center gap-1.5 rounded-full px-3 type-callout text-text-secondary hover:bg-fill hover:text-on-background"
      >
        <Icon name="sort" size={16} />
        {t("library.sortBy", { sort: t(sortLabels[sort]) })}
      </button>
      {open && (
        <div
          role="menu"
          className="absolute top-full left-0 z-30 mt-1.5 w-56 rounded-[12px] border border-border bg-surface-overlay/90 p-1.5 shadow-level3 backdrop-blur-xl"
        >
          {(Object.keys(sortLabels) as Sort[]).map((k) => (
            <button
              key={k}
              type="button"
              role="menuitemradio"
              aria-checked={k === sort}
              onClick={() => (onSort(k), close())}
              className="flex min-h-9 w-full items-center gap-2 rounded-lg px-2.5 text-left type-body hover:bg-fill"
            >
              <span className="w-4">{k === sort && <Icon name="check" size={16} className="text-accent-text" />}</span>
              {t(sortLabels[k])}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

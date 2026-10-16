// A library: its Recommended tab of hubs and the grid of all its items,
// sorted and filtered (docs/clients.md §6.3.1).
import {
  type Filter,
  type ItemValue,
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
import { Icon } from "../icon.tsx";
import { usePrefs } from "../prefs.ts";
import { Grid, ItemCard, LoadMore, Loading, StatusView } from "../ui.tsx";
import { Menu, MenuChoice, MenuOption } from "../controls/menu.tsx";
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

const valueLabels: Record<ItemValue["kind"], MessageKey> = {
  genre: "library.genreValue",
  tag: "library.tagValue",
  studio: "library.studioValue",
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
  const navigate = useNavigate({ from: "/s/$serverId/library/$libraryId" });
  // On the television the choice looks like the tabs it stands in for:
  // the current one filled, the focused one white. Focus chooses, as on
  // the tabs, and a choice replaces the page rather than adding to the
  // history, so that back leaves the library (docs/tv.md §6.3).
  const { tv } = usePrefs();
  const all = tab === "all";
  const choose = (to: "recommended" | "all") => {
    if (tv && (to === "all") !== all) void navigate({ search: (s) => ({ ...s, tab: to === "all" ? ("all" as const) : undefined }), replace: true });
  };
  const segment = tv
    ? "tv-pill rounded-full px-6 py-2 type-headline text-text-secondary data-[active=true]:bg-fill-selected data-[active=true]:text-on-background"
    : "rounded-[7px] px-4 py-1 type-callout font-semibold text-text-secondary hover:text-on-background data-[active=true]:bg-surface-overlay data-[active=true]:text-on-background data-[active=true]:shadow-level1";
  return (
    <div>
      <div className={`flex flex-wrap gap-x-6 gap-y-3 px-margin pt-6 pb-3 ${tv ? "items-center" : "items-end"}`}>
        <h1 className="type-title1">{library.data?.name ?? " "}</h1>
        <div
          role="tablist"
          data-focus-choice
          data-tv-initial
          className={tv ? "flex gap-1 rounded-full bg-surface-raised/60 p-1.5" : "flex rounded-[9px] bg-fill p-0.5"}
        >
          <Link
            role="tab"
            from="/s/$serverId/library/$libraryId"
            search={{}}
            replace={tv}
            aria-selected={!all}
            data-active={!all}
            onFocus={() => choose("recommended")}
            className={segment}
          >
            {t("library.recommended")}
          </Link>
          <Link
            role="tab"
            from="/s/$serverId/library/$libraryId"
            search={(s) => ({ ...s, tab: "all" as const })}
            replace={tv}
            aria-selected={all}
            data-active={all}
            onFocus={() => choose("all")}
            className={segment}
          >
            {t("library.all")}
          </Link>
        </div>
      </div>
      {all ? <AllItems server={server} libraryId={libraryId} /> : <Recommended server={server} libraryId={libraryId} />}
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
  const { sort = "name", filter = "all", genre, tag, studio } = route.useSearch();
  const navigate = useNavigate({ from: "/s/$serverId/library/$libraryId" });
  // An item's details link to the items of one of its genres, keywords or studios.
  const value: ItemValue | undefined = genre
    ? { kind: "genre", value: genre }
    : tag
      ? { kind: "tag", value: tag }
      : studio
        ? { kind: "studio", value: studio }
        : undefined;
  const q = useLibraryItems(server, libraryId, sort, filter, value);
  const total = q.data?.pages[0]?.total;
  // On the television the filters are chosen by focus, as the tabs are,
  // and neither they nor the order add to the history.
  const { tv } = usePrefs();
  const filterTo = (k: Filter) => (k === "all" ? undefined : k);
  return (
    <div>
      <div className={`flex flex-wrap items-center px-margin pt-1 pb-5 ${tv ? "gap-4" : "gap-2"}`}>
        {value && (
          <Link
            from="/s/$serverId/library/$libraryId"
            search={(s) => ({ ...s, genre: undefined, tag: undefined, studio: undefined })}
            replace
            aria-label={`${t(valueLabels[value.kind], { value: value.value })}, ${t("library.clearValue")}`}
            className={`inline-flex items-center gap-1.5 rounded-full bg-accent font-semibold text-on-accent hover:opacity-90 ${
              tv ? "h-12 px-5 type-headline" : "h-8 pr-2.5 pl-3.5 type-callout"
            }`}
          >
            {t(valueLabels[value.kind], { value: value.value })}
            <Icon name="close" size={tv ? 20 : 14} />
          </Link>
        )}
        <div role="radiogroup" aria-label={t("library.filter")} data-focus-choice className={`flex flex-wrap ${tv ? "gap-1 rounded-full bg-surface-raised/60 p-1.5" : "gap-2"}`}>
          {(Object.keys(filterLabels) as Filter[]).map((k) => (
            <Link
              key={k}
              role="radio"
              aria-checked={filter === k}
              from="/s/$serverId/library/$libraryId"
              search={(s) => ({ ...s, filter: filterTo(k) })}
              replace={tv}
              data-active={filter === k}
              onFocus={() => tv && filter !== k && void navigate({ search: (s) => ({ ...s, filter: filterTo(k) }), replace: true })}
              className={
                tv
                  ? "tv-pill rounded-full px-5 py-2 type-headline text-text-secondary data-[active=true]:bg-fill-selected data-[active=true]:text-on-background"
                  : "inline-flex h-8 items-center rounded-full bg-fill px-3.5 type-callout font-medium hover:bg-fill-selected data-[active=true]:bg-button data-[active=true]:text-on-button"
              }
            >
              {t(filterLabels[k])}
            </Link>
          ))}
        </div>
        <SortMenu sort={sort} onSort={(k) => void navigate({ search: (s) => ({ ...s, sort: k }), replace: tv })} />
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
  const { tv } = usePrefs();
  return (
    <Menu
      width={tv ? "w-80" : "w-56"}
      trigger={
        <button
          type="button"
          className={`inline-flex items-center rounded-full text-text-secondary hover:bg-fill hover:text-on-background data-popup-open:bg-fill ${
            tv ? "h-12 gap-2 px-5 type-headline" : "h-8 gap-1.5 px-3 type-callout"
          }`}
        >
          <Icon name="sort" size={tv ? 22 : 16} />
          {t("library.sortBy", { sort: t(sortLabels[sort]) })}
        </button>
      }
    >
      <MenuChoice value={sort} onChange={onSort}>
        {(Object.keys(sortLabels) as Sort[]).map((k) => (
          <MenuOption key={k} value={k}>
            {t(sortLabels[k])}
          </MenuOption>
        ))}
      </MenuChoice>
    </Menu>
  );
}

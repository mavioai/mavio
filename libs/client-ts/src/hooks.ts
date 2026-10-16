// The TanStack Query hooks of the screens (docs/clients.md §4.1), over every
// server the user is signed in to. Query keys start with ["mavio", serverId],
// so that a server's events invalidate its queries alone.
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { HubService } from "@mavio/proto/mavio/hub/v1/hub_pb";
import {
  type Item,
  ItemKind,
  ItemService,
  type ListItemsRequest,
  ListItemsRequestSchema,
  SortField,
} from "@mavio/proto/mavio/library/v1/item_pb";
import { type Library, LibraryService } from "@mavio/proto/mavio/library/v1/library_pb";
import { type UserData, UserDataSchema, UserDataService } from "@mavio/proto/mavio/user/v1/user_pb";
import {
  keepPreviousData,
  type QueryClient,
  type QueryKey,
  type UseQueryResult,
  useInfiniteQuery,
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { createContext, useCallback, useContext, useMemo, useRef, useSyncExternalStore } from "react";

import { type Hubs, merge, type Section } from "./home.ts";
import type { ServerEntry, Servers, SignIn } from "./servers.ts";

const ServersContext = createContext<Servers | undefined>(undefined);

/** Provides the servers to the hooks. */
export const ServersProvider = ServersContext.Provider;

/** The servers the hooks use. */
export function useServersStore(): Servers {
  const s = useContext(ServersContext);
  if (!s) throw new Error("useServersStore outside ServersProvider");
  return s;
}

/** Every server the user added, in their order, signed in or not. */
export function useServers(): ServerEntry[] {
  const s = useServersStore();
  return useSyncExternalStore(s.subscribe, s.snapshot);
}

/** The sign-in: a Mavio account's or a standalone server user's; undefined when signed out. */
export function useSignIn(): SignIn | undefined {
  const s = useServersStore();
  return useSyncExternalStore(s.subscribe, s.signInSnapshot);
}

/** The servers signed in to, in the user's order. */
export function useSignedIn(): ServerEntry[] {
  const signIn = useSignIn();
  return useMemo(() => signIn?.servers ?? [], [signIn]);
}

/** One server, once it is known. */
export function useServer(id: string): ServerEntry | undefined {
  return useServers().find((s) => s.id === id);
}

/** The key of a query about a server. */
export function serverKey(serverId: string, ...rest: unknown[]): QueryKey {
  return ["mavio", serverId, ...rest];
}

/**
 * How long a library's items, lists and people stay fresh. The server's
 * events refresh them as they change (events.ts), so they are fetched
 * again only after a while, in case an event was missed.
 */
export const libraryStaleTime = 5 * 60_000;

/** A message for an error of a call. */
export function errorMessage(err: unknown): string {
  return err instanceof ConnectError ? err.rawMessage : err instanceof Error ? err.message : String(err);
}

/** Whether a password sign-in failed for want of the user's TOTP code. */
export function isTotpRequired(err: unknown): boolean {
  return err instanceof ConnectError && err.code === Code.FailedPrecondition;
}

/** Whether a call failed for what it asked for not being there, as an item deleted. */
export function isNotFound(err: unknown): boolean {
  return err instanceof ConnectError && err.code === Code.NotFound;
}

/** Whether a call failed for lack of a valid sign-in, such as a wrong password. */
export function isUnauthenticated(err: unknown): boolean {
  return err instanceof ConnectError && err.code === Code.Unauthenticated;
}

function hubsOf(q: UseQueryResult<Awaited<ReturnType<typeof listHubs>>>): Hubs {
  if (q.data) return q.isError ? { state: "cached", hubs: q.data } : { state: "fresh", hubs: q.data };
  if (q.isError) return { state: "unavailable", reason: errorMessage(q.error) };
  return { state: "loading" };
}

async function listHubs(servers: Servers, server: ServerEntry, libraryId?: string) {
  const r = await servers.client(HubService, server).listHubs({ libraryId });
  return r.hubs;
}

/**
 * Home across servers: each server's hubs as they arrive, in their places,
 * merged as Plex does (docs/clients.md §5.4). A server that stops answering
 * keeps its last hubs, marked offline.
 */
export function useHome(): Section[] {
  const servers = useServersStore();
  const list = useSignedIn();
  return useQueries({
    queries: list.map((s) => ({
      queryKey: serverKey(s.id, "home"),
      queryFn: () => listHubs(servers, s),
      staleTime: 60_000,
    })),
    combine: (results) =>
      merge(list.map((s, i) => ({ serverId: s.id, serverName: s.name, hubs: hubsOf(results[i]) }))),
  });
}

/** A library's Recommended tab: its hubs, as sections of its server. */
export function useRecommended(server: ServerEntry, libraryId: string) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "recommended", libraryId),
    queryFn: () => listHubs(servers, server, libraryId),
    select: (hubs) => merge([{ serverId: server.id, serverName: server.name, hubs: { state: "fresh", hubs } }]),
    staleTime: 60_000,
  });
}

const pageSize = 60;

/** All the items of a hub, page by page. */
export function useHubItems(server: ServerEntry, hubId: string) {
  const servers = useServersStore();
  return useInfiniteQuery({
    queryKey: serverKey(server.id, "hub", hubId),
    queryFn: async ({ pageParam }) => {
      const r = await servers.client(HubService, server).getHub({ id: hubId, offset: pageParam, limit: pageSize });
      return { hub: r.hub, next: r.hub?.more ? pageParam + (r.hub.items.length || pageSize) : undefined };
    },
    initialPageParam: 0,
    getNextPageParam: (last) => last.next,
    staleTime: libraryStaleTime,
  });
}

/** A server's libraries with the server. */
export interface ServerLibraries {
  server: ServerEntry;
  libraries: Library[];
  pending: boolean;
  error?: string;
}

/** The libraries of every server, in the user's order of servers. */
export function useLibraries(): ServerLibraries[] {
  const servers = useServersStore();
  const list = useSignedIn();
  return useQueries({
    queries: list.map((s) => ({
      queryKey: serverKey(s.id, "libraries"),
      queryFn: async () => (await servers.client(LibraryService, s).listLibraries({})).libraries,
      staleTime: 5 * 60_000,
    })),
    combine: (results) =>
      list.map((server, i) => ({
        server,
        libraries: results[i].data ?? [],
        pending: results[i].isPending,
        error: results[i].isError ? errorMessage(results[i].error) : undefined,
      })),
  });
}

/** One library. */
export function useLibrary(server: ServerEntry, libraryId: string) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "library", libraryId),
    queryFn: async () => (await servers.client(LibraryService, server).getLibrary({ id: libraryId })).library,
    staleTime: 5 * 60_000,
  });
}

/** The orders of a library's grid. */
export type Sort = "name" | "dateAdded" | "released" | "rating" | "lastPlayed" | "random";

/** The filters of a library's grid. */
export type Filter = "all" | "unplayed" | "played" | "favorites";

const sorts: Record<Sort, [SortField, boolean]> = {
  name: [SortField.NAME, false],
  dateAdded: [SortField.DATE_ADDED, true],
  released: [SortField.PREMIERE_DATE, true],
  rating: [SortField.COMMUNITY_RATING, true],
  lastPlayed: [SortField.LAST_PLAYED, true],
  random: [SortField.RANDOM, false],
};

/** A genre, tag or studio a library's grid shows only the items of. */
export interface ItemValue {
  kind: "genre" | "tag" | "studio";
  value: string;
}

/** The request of a page of a library's grid. */
export function libraryItemsRequest(libraryId: string, sort: Sort, filter: Filter, offset: number, value?: ItemValue): ListItemsRequest {
  const [field, descending] = sorts[sort];
  const req = create(ListItemsRequestSchema, {
    libraryIds: [libraryId],
    topLevel: true,
    sort: [{ field, descending }],
    offset,
    limit: pageSize,
    genres: value?.kind === "genre" ? [value.value] : [],
    tags: value?.kind === "tag" ? [value.value] : [],
    studios: value?.kind === "studio" ? [value.value] : [],
  });
  if (filter === "played") req.played = true;
  if (filter === "unplayed") req.played = false;
  if (filter === "favorites") req.favorite = true;
  return req;
}

/** A library's grid: the items at its top, sorted and filtered, page by page. */
export function useLibraryItems(server: ServerEntry, libraryId: string, sort: Sort, filter: Filter, value?: ItemValue) {
  const servers = useServersStore();
  return useInfiniteQuery({
    queryKey: serverKey(server.id, "items", libraryId, sort, filter, value),
    queryFn: async ({ pageParam }) => {
      const r = await servers
        .client(ItemService, server)
        .listItems(libraryItemsRequest(libraryId, sort, filter, pageParam, value));
      const end = pageParam + r.items.length;
      return { items: r.items, total: r.total, next: r.items.length > 0 && end < r.total ? end : undefined };
    },
    initialPageParam: 0,
    getNextPageParam: (last) => last.next,
    staleTime: libraryStaleTime,
    // Another order or filter keeps showing the grid until its items arrive.
    placeholderData: keepPreviousData,
  });
}

function itemQuery(servers: Servers, server: ServerEntry, itemId: string) {
  return {
    queryKey: serverKey(server.id, "item", itemId),
    queryFn: () => servers.client(ItemService, server).getItem({ id: itemId }),
    staleTime: libraryStaleTime,
  };
}

/** An item with its media sources and credits. */
export function useItem(server: ServerEntry | undefined, itemId: string) {
  const servers = useServersStore();
  return useQuery({
    ...itemQuery(servers, server ?? ({ id: "" } as ServerEntry), itemId),
    enabled: !!server,
  });
}

const prefetchDelay = 150;

/**
 * Handlers that fetch an item ahead of its page while the pointer rests on
 * a link to it, so that the page opens at once.
 */
export function usePrefetchItem(serverId: string, itemId: string) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const onPointerEnter = useCallback(
    (e: { pointerType: string }) => {
      const server = servers.get(serverId);
      if (e.pointerType !== "mouse" || !server?.token) return;
      timer.current = setTimeout(() => void qc.prefetchQuery(itemQuery(servers, server, itemId)), prefetchDelay);
    },
    [servers, qc, serverId, itemId],
  );
  const onPointerLeave = useCallback(() => clearTimeout(timer.current), []);
  return { onPointerEnter, onPointerLeave };
}

/** An item's seasons, episodes, tracks or members. */
export function useChildren(server: ServerEntry, item: Item | undefined) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "children", item?.id),
    queryFn: async () =>
      (
        await servers
          .client(ItemService, server)
          .listItems({
            parentId: item!.id,
            // A collection's members in the order they were added.
            sort: [{ field: item!.kind === ItemKind.COLLECTION ? SortField.LIST_ORDER : SortField.INDEX }],
          })
      ).items,
    enabled: !!item,
    staleTime: libraryStaleTime,
  });
}

/** A series' episodes, by season and number. */
export function useEpisodes(server: ServerEntry, seriesId: string | undefined) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "episodes", seriesId),
    queryFn: async () => {
      const { items } = await servers
        .client(ItemService, server)
        .listItems({ parentId: seriesId, recursive: true, kinds: [ItemKind.EPISODE] });
      return items.toSorted(byEpisode);
    },
    enabled: !!seriesId,
    staleTime: libraryStaleTime,
  });
}

function byEpisode(a: Item, b: Item): number {
  return a.parentIndexNumber - b.parentIndexNumber || a.indexNumber - b.indexNumber;
}

/** The episode after an episode in its series, by season and number. */
export function useNextEpisode(server: ServerEntry, item: Item | undefined) {
  const episodes = useEpisodes(server, item?.kind === ItemKind.EPISODE ? item.seriesId : undefined);
  return { ...episodes, data: episodes.data && item ? (nextEpisode(episodes.data, item.id) ?? null) : undefined };
}

/** Of a series' episodes in any order, the one after id by season and number. */
export function nextEpisode(episodes: Item[], id: string): Item | undefined {
  const order = episodes.toSorted(byEpisode);
  const at = order.findIndex((i) => i.id === id);
  return at >= 0 ? order[at + 1] : undefined;
}

/**
 * The episode of a series to watch next, of its episodes in any order: the
 * one played last if it is unfinished, else the first unplayed one after
 * it, else the first unplayed one, else the first. Specials count only in
 * a series that has nothing else.
 */
export function seriesNextUp(episodes: Item[]): Item | undefined {
  const regular = episodes.filter((e) => e.parentIndexNumber > 0);
  const order = (regular.length ? regular : episodes).toSorted(byEpisode);
  const time = (e: Item) => {
    const t = e.userData?.lastPlayedTime;
    return t ? Number(t.seconds) * 1000 + t.nanos / 1e6 : 0;
  };
  let last = -1;
  for (let i = 0; i < order.length; i++) {
    if (time(order[i]) > 0 && (last < 0 || time(order[i]) >= time(order[last]))) last = i;
  }
  const unplayed = (e: Item) => !e.userData?.played;
  if (last >= 0) {
    if (unplayed(order[last]) && order[last].userData?.position) return order[last];
    const after = order.slice(last + 1).find(unplayed);
    if (after) return after;
  }
  return order.find(unplayed) ?? order[0];
}

/** Items like an item. */
export function useSimilar(server: ServerEntry, itemId: string) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "similar", itemId),
    queryFn: async () => (await servers.client(ItemService, server).listSimilarItems({ id: itemId, limit: 20 })).items,
    staleTime: 10 * 60_000,
  });
}

/** An item's trailers, featurettes and other extras, by name. */
export function useExtras(server: ServerEntry, itemId: string) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "extras", itemId),
    queryFn: async () =>
      (await servers.client(ItemService, server).listItems({ ownerId: itemId, sort: [{ field: SortField.NAME }] })).items,
    staleTime: 10 * 60_000,
  });
}

/** The providers whose IDs tell that two servers have the same title. */
const matchingProviders = ["tmdb", "tvdb", "imdb"];

/** A copy of a title on another server. */
export interface Copy {
  server: ServerEntry;
  item: Item;
}

/** The same title on the user's other servers, matched by its external IDs, in their order. */
export function useOtherCopies(serverId: string, item: Item | undefined): Copy[] {
  const servers = useServersStore();
  const ids = Object.fromEntries(
    Object.entries(item?.externalIds ?? {}).filter(([p, v]) => matchingProviders.includes(p) && v !== ""),
  );
  const others = useSignedIn().filter((s) => s.id !== serverId);
  const enabled = !!item && Object.keys(ids).length > 0;
  return useQueries({
    queries: others.map((s) => ({
      queryKey: serverKey(s.id, "copies", item?.kind, ids),
      queryFn: async () => {
        const r = await servers
          .client(ItemService, s)
          .listItems({ kinds: [item!.kind], recursive: true, externalIds: ids, limit: 1 });
        return r.items[0] ?? null;
      },
      enabled,
      staleTime: 5 * 60_000,
    })),
    combine: (results) =>
      others.flatMap((server, i) => {
        const found = results[i].data;
        return found ? [{ server, item: found }] : [];
      }),
  });
}

/** One server's search results. */
export interface SearchResults {
  server: ServerEntry;
  items: Item[];
  pending: boolean;
  error?: string;
}

/** The kinds search finds: titles, not the seasons and folders inside them. */
const searchKinds = [
  ItemKind.MOVIE,
  ItemKind.SERIES,
  ItemKind.EPISODE,
  ItemKind.VIDEO,
  ItemKind.MUSIC_ARTIST,
  ItemKind.MUSIC_ALBUM,
  ItemKind.TRACK,
  ItemKind.MUSIC_VIDEO,
  ItemKind.AUDIOBOOK,
  ItemKind.BOOK,
  ItemKind.PHOTO_ALBUM,
  ItemKind.COLLECTION,
  ItemKind.PLAYLIST,
];

/** Searches every server at once, the results in the user's order of servers. */
export function useSearch(query: string): SearchResults[] {
  const servers = useServersStore();
  const list = useSignedIn();
  const q = query.trim();
  return useQueries({
    queries: list.map((s) => ({
      queryKey: serverKey(s.id, "search", q),
      queryFn: async () =>
        (await servers.client(ItemService, s).listItems({ search: q, kinds: searchKinds, limit: 50 })).items,
      enabled: q !== "",
      staleTime: 60_000,
    })),
    combine: (results) =>
      list.map((server, i) => ({
        server,
        items: results[i].data ?? [],
        pending: q !== "" && results[i].isPending,
        error: results[i].isError ? errorMessage(results[i].error) : undefined,
      })),
  });
}

/** The kinds of a server's queries whose lists the user's data chooses or orders, or sums up for a series. */
const userDataKinds = new Set(["item", "children", "items", "episodes", "home", "hub", "recommended"]);

/** A change of an item's user data, given what it was. */
export type UserDataChange = (old: UserData | undefined) => UserData;

function userDataQueries(serverId: string) {
  return { queryKey: serverKey(serverId), predicate: (q: { queryKey: QueryKey }) => userDataKinds.has(q.queryKey[2] as string) };
}

/**
 * Changes the user data of the items in every cached query of a server,
 * leaving the others be, and returns the queries changed as they were.
 */
function patchUserData(qc: QueryClient, serverId: string, change: ReadonlyMap<string, UserDataChange>) {
  const before: [QueryKey, unknown][] = [];
  for (const [key, data] of qc.getQueriesData({ queryKey: serverKey(serverId) })) {
    const next = withUserData(data, change);
    if (next === data) continue;
    before.push([key, data]);
    qc.setQueryData(key, next);
  }
  return before;
}

/**
 * Data with the user data of the items in it changed by change, keyed by
 * item ID; what holds no such item is returned as it was.
 */
export function withUserData<T>(data: T, change: ReadonlyMap<string, UserDataChange>): T {
  if (Array.isArray(data)) {
    let changed = false;
    const out = data.map((v) => {
      const w = withUserData(v, change);
      if (w !== v) changed = true;
      return w;
    });
    return (changed ? out : data) as T;
  }
  if (typeof data !== "object" || data === null) return data;
  const o = data as Record<string, unknown>;
  // Plain objects and messages, whose prototype may hold their defaults.
  if (Object.getPrototypeOf(o) !== Object.prototype && typeof o.$typeName !== "string") return data;
  let out: Record<string, unknown> | undefined;
  for (const [k, v] of Object.entries(o)) {
    const w = withUserData(v, change);
    if (w !== v) (out ??= copy(o))[k] = w;
  }
  const c = o.$typeName === "mavio.library.v1.Item" ? change.get(o.id as string) : undefined;
  if (c) (out ??= copy(o)).userData = c(o.userData as UserData | undefined);
  return (out ?? data) as T;
}

/** A shallow copy of an object with its prototype. */
function copy<T extends object>(o: T): T {
  return Object.assign(Object.create(Object.getPrototypeOf(o) as object | null) as T, o);
}

/**
 * Puts user data into the cached items of a server and refreshes the lists
 * it may change, such as the unplayed and Continue Watching; a refresh
 * already under way is not started again, so that the server's answer and
 * its event refresh them once.
 */
export function applyUserData(qc: QueryClient, serverId: string, list: UserData[]) {
  if (list.length === 0) return;
  patchUserData(qc, serverId, new Map(list.map((u): [string, UserDataChange] => [u.itemId, () => u])));
  void qc.invalidateQueries(userDataQueries(serverId), { cancelRefetch: false });
}

/** An item's user data as the server will make it: marked, played or not, it has no position. */
export function markedUserData(u: { itemId: string; played?: boolean; favorite?: boolean }): UserDataChange {
  return (old) => {
    const next = old ? copy(old) : create(UserDataSchema, { itemId: u.itemId });
    if (u.favorite !== undefined) next.favorite = u.favorite;
    if (u.played !== undefined) {
      next.played = u.played;
      next.position = undefined;
    }
    return next;
  };
}

/**
 * Marks items played or favorite. The cached items show the change at
 * once, and return to what they were should the server refuse it.
 */
export function useUserData(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const filters = userDataQueries(server.id);
  return useMutation({
    mutationFn: (u: { itemId: string; played?: boolean; favorite?: boolean }) =>
      servers.client(UserDataService, server).updateUserData(u),
    onMutate: async (u) => {
      // A refresh under way would bring back what is being changed.
      await qc.cancelQueries(filters);
      return { before: patchUserData(qc, server.id, new Map([[u.itemId, markedUserData(u)]])) };
    },
    onError: (_err, _u, context) => {
      for (const [key, data] of context?.before ?? []) qc.setQueryData(key, data);
    },
    onSuccess: (r) => {
      if (r.userData) applyUserData(qc, server.id, [r.userData]);
    },
  });
}

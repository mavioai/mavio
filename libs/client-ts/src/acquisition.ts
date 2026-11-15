// How a server acquires movies and episodes (docs/clients.md §6.3.16–§6.3.17,
// §6.3.4): the wanted movies and episodes, searched and synced by RSS; how
// an item is acquired; the releases the indexers have of it, to grab; and
// the downloads' queue, history and blocklist, and manual import.
import { Code, ConnectError } from "@connectrpc/connect";
import {
  DownloadService,
  type HistoryEventType,
  type ManualImportRequestSchema,
  type RemoveFromQueueRequestSchema,
} from "@mavio/proto/mavio/acquisition/v1/download_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { type Release, type ReleaseSearchType, ReleaseService } from "@mavio/proto/mavio/acquisition/v1/release_pb";
import type { RequestSettings } from "@mavio/proto/mavio/acquisition/v1/request_pb";
import {
  type AddMovieRequestSchema,
  type AddSeriesRequestSchema,
  type SeriesMonitor,
  type Wanted,
  type WantedFilter,
  WantedSchema,
  type WantedKind,
  WantedService,
} from "@mavio/proto/mavio/acquisition/v1/wanted_pb";
import { type Item, ItemKind, ItemService, SortField } from "@mavio/proto/mavio/library/v1/item_pb";
import { LibraryKind } from "@mavio/proto/mavio/library/v1/library_pb";
import { clone, type MessageInitShape } from "@bufbuild/protobuf";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

const pageSize = 50;

/** The page after one of offset and length, of total; undefined after the last. */
function nextOffset(offset: number, length: number, total: number): number | undefined {
  return length > 0 && offset + length < total ? offset + length : undefined;
}

/** The monitored movies or episodes missing a file or below their cutoff, page by page. */
export function useWantedList(server: ServerEntry, filter: WantedFilter, kind: WantedKind) {
  const servers = useServersStore();
  return useInfiniteQuery({
    queryKey: serverKey(server.id, "acquisition", "wanted", "list", filter, kind),
    queryFn: async ({ pageParam }) => {
      const r = await servers.client(WantedService, server).listWanted({ filter, kind, limit: pageSize, offset: pageParam });
      return { ...r, next: nextOffset(pageParam, r.movies.length + r.episodes.length, r.total) };
    },
    initialPageParam: 0,
    getNextPageParam: (last) => last.next,
  });
}

/** How a movie or series is acquired; undefined when it is not wanted. */
export function useWanted(server: ServerEntry, itemId: string, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "wanted", "item", itemId),
    queryFn: async (): Promise<Wanted | null> => {
      try {
        return (await servers.client(WantedService, server).getWanted({ itemId })).wanted ?? null;
      } catch (e) {
        if (e instanceof ConnectError && e.code === Code.NotFound) return null;
        throw e;
      }
    },
    enabled: enabled && !!itemId,
    retry: false,
  });
}

/** What can be done to wanted items, each refreshing what acquisition and the library show. */
export function useWantedActions(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const c = () => servers.client(WantedService, server);
  const onSuccess = () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: serverKey(server.id, "acquisition") }),
      qc.invalidateQueries({ queryKey: serverKey(server.id, "item") }),
      qc.invalidateQueries({ queryKey: serverKey(server.id, "children") }),
      qc.invalidateQueries({ queryKey: serverKey(server.id, "episodes") }),
    ]);
  return {
    search: useMutation({
      mutationFn: async (v: { itemIds?: string[]; filter?: WantedFilter; kind?: WantedKind }) => (await c().searchWanted(v)).queued,
      onSuccess,
    }),
    syncRss: useMutation({ mutationFn: () => c().syncRSS({}) }),
    setMonitored: useMutation({ mutationFn: (v: { itemIds: string[]; monitored: boolean }) => c().setMonitored({ change: { case: "items", value: v } }), onSuccess }),
    monitorSeries: useMutation({ mutationFn: (v: { itemIds: string[]; monitor: SeriesMonitor }) => c().setMonitored({ change: { case: "series", value: v } }),
      onSuccess,
     }),
    update: useMutation({ mutationFn: async (wanted: Wanted) => (await c().updateWanted({ wanted })).wanted, onSuccess }),
    // A movie's or series' wanted settings, and a series' monitoring choice applied to its episodes, as Sonarr's series editor.
    save: useMutation({
      mutationFn: async (v: { wanted: Wanted; monitor?: SeriesMonitor }) => {
        const saved = (await c().updateWanted({ wanted: v.wanted })).wanted;
        if (v.monitor) await c().setMonitored({ change: { case: "series", value: { itemIds: [v.wanted.itemId], monitor: v.monitor } } });
        return saved;
      },
      onSuccess,
    }),
    // Movies and series are monitored through their wanted; seasons and episodes by setMonitored.
    setWantedMonitored: useMutation({
      mutationFn: async (v: { itemId: string; monitored: boolean }) => {
        const w = (await c().getWanted({ itemId: v.itemId })).wanted;
        if (!w) throw new ConnectError("not wanted", Code.NotFound);
        const next = clone(WantedSchema, w);
        next.monitored = v.monitored;
        return (await c().updateWanted({ wanted: next })).wanted;
      },
      onSuccess,
    }),
  };
}

/** The tags typed for a wanted movie or series, separated by commas: trimmed, without empty or repeated ones. */
export function parseTags(typed: string): string[] {
  return [...new Set(typed.split(",").map((s) => s.trim()).filter(Boolean))];
}

/** The library and quality profile a title is added to the wanted with. */
export interface WantedTarget {
  libraryId: string;
  qualityProfileId: string;
}

/**
 * Where a movie or series is added to the wanted by default: the library
 * and quality profile approved requests of its kind go to — an anime
 * series' own, else a series' — or the first of its kind where those are
 * unset or no longer offered.
 */
export function wantedTarget(
  kind: MediaKind,
  anime: boolean,
  settings: RequestSettings | undefined,
  libraries: { id: string; kind: LibraryKind }[],
  profiles: { id: string; kind: MediaKind }[],
): WantedTarget {
  const movie = kind === MediaKind.MOVIE;
  const libs = libraries.filter((l) => l.kind === (movie ? LibraryKind.MOVIES : LibraryKind.SHOWS)).map((l) => l.id);
  const profs = profiles.filter((p) => p.kind === kind).map((p) => p.id);
  const pick = (offered: string[], ...preferred: (string | undefined)[]) => preferred.find((id) => !!id && offered.includes(id)) ?? offered[0] ?? "";
  const s = settings;
  return movie
    ? { libraryId: pick(libs, s?.movieLibraryId), qualityProfileId: pick(profs, s?.movieQualityProfileId) }
    : {
        libraryId: pick(libs, anime ? s?.animeLibraryId : undefined, s?.seriesLibraryId),
        qualityProfileId: pick(profs, anime ? s?.animeQualityProfileId : undefined, s?.seriesQualityProfileId),
      };
}

/**
 * Adding a movie by its TMDB ID or a series by its TVDB ID to a library's
 * wanted, as Radarr's and Sonarr's Add New; resolves to the new item's ID
 * and refreshes what acquisition, the library and discovery show.
 */
export function useAddWanted(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const c = () => servers.client(WantedService, server);
  const onSuccess = () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: serverKey(server.id, "acquisition") }),
      qc.invalidateQueries({ queryKey: serverKey(server.id, "discover") }),
      qc.invalidateQueries({ queryKey: serverKey(server.id, "items") }),
    ]);
  return {
    movie: useMutation({ mutationFn: async (v: MessageInitShape<typeof AddMovieRequestSchema>) => (await c().addMovie(v)).itemId, onSuccess }),
    series: useMutation({ mutationFn: async (v: MessageInitShape<typeof AddSeriesRequestSchema>) => (await c().addSeries(v)).itemId, onSuccess }),
  };
}

/** The releases the indexers have of an item, searched when asked. */
export function useReleases(server: ServerEntry, itemId: string, enabled: boolean) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "releases", itemId),
    queryFn: () => servers.client(ReleaseService, server).searchReleases({ itemId }),
    enabled,
    staleTime: 5 * 60_000,
    retry: false,
  });
}

/** Grabbing a release for an item, sent to a client or the first of its protocol. */
export function useGrab(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { itemId: string; release: Release; clientPluginId?: string; clientId?: string }) =>
      (await servers.client(DownloadService, server).grabRelease(v)).item,
    onSuccess: () => qc.invalidateQueries({ queryKey: serverKey(server.id, "acquisition", "downloads") }),
  });
}

/** A search of a server's indexers by text, as Prowlarr's, run while a search is given. */
export interface ReleaseSearch {
  query: string;
  type?: ReleaseSearchType;
  /** Newznab categories; empty for those of the type. */
  categories?: number[];
  /** The indexers to search; empty for every enabled indexer. */
  indexers?: { pluginId: string; indexerId: string }[];
}

/** The releases of a search of a server's indexers by text, and the indexers that failed. */
export function useReleaseSearch(server: ServerEntry, search: ReleaseSearch | undefined) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "releases", "search", search),
    queryFn: () => servers.client(ReleaseService, server).searchReleases(search!),
    enabled: !!search,
    staleTime: 5 * 60_000,
    retry: false,
  });
}

/**
 * Sending a release to a download client without an item: the client's
 * alone, neither tracked nor imported, as Prowlarr's searches grab. Resolves
 * to the client's name.
 */
export function useSendRelease(server: ServerEntry) {
  const servers = useServersStore();
  return useMutation({
    mutationFn: async (v: { release: Release; clientPluginId?: string; clientId?: string }) =>
      (await servers.client(DownloadService, server).grabRelease(v)).clientName,
  });
}

/** The tracked downloads, of an item or all, most recently grabbed first, page by page. */
export function useQueue(server: ServerEntry, itemId = "", includeFinished = false) {
  const servers = useServersStore();
  return useInfiniteQuery({
    queryKey: serverKey(server.id, "acquisition", "downloads", "queue", itemId, includeFinished),
    queryFn: async ({ pageParam }) => {
      const r = await servers.client(DownloadService, server).listQueue({ itemId: itemId || undefined, includeFinished, limit: pageSize, offset: pageParam });
      return { ...r, next: nextOffset(pageParam, r.items.length, r.total) };
    },
    initialPageParam: 0,
    getNextPageParam: (last) => last.next,
    refetchInterval: 10_000,
  });
}

/** What happened in acquiring an item or all, newest first, page by page. */
export function useHistory(server: ServerEntry, itemId = "", types: HistoryEventType[] = []) {
  const servers = useServersStore();
  return useInfiniteQuery({
    queryKey: serverKey(server.id, "acquisition", "downloads", "history", itemId, types),
    queryFn: async ({ pageParam }) => {
      const r = await servers.client(DownloadService, server).listHistory({ itemId: itemId || undefined, types, limit: pageSize, offset: pageParam });
      return { ...r, next: nextOffset(pageParam, r.events.length, r.total) };
    },
    initialPageParam: 0,
    getNextPageParam: (last) => last.next,
  });
}

/** The releases not to grab again, newest first, page by page. */
export function useBlocklist(server: ServerEntry, itemId = "") {
  const servers = useServersStore();
  return useInfiniteQuery({
    queryKey: serverKey(server.id, "acquisition", "downloads", "blocklist", itemId),
    queryFn: async ({ pageParam }) => {
      const r = await servers.client(DownloadService, server).listBlocklist({ itemId: itemId || undefined, limit: pageSize, offset: pageParam });
      return { ...r, next: nextOffset(pageParam, r.entries.length, r.total) };
    },
    initialPageParam: 0,
    getNextPageParam: (last) => last.next,
  });
}

/** What can be done to downloads, each refreshing the queue, history and blocklist. */
export function useDownloadActions(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const c = () => servers.client(DownloadService, server);
  const onSuccess = () => qc.invalidateQueries({ queryKey: serverKey(server.id, "acquisition") });
  return {
    remove: useMutation({ mutationFn: (v: MessageInitShape<typeof RemoveFromQueueRequestSchema>) => c().removeFromQueue(v), onSuccess }),
    markFailed: useMutation({ mutationFn: (historyEventId: string) => c().markFailed({ historyEventId }), onSuccess }),
    unblock: useMutation({ mutationFn: (ids: string[]) => c().deleteBlocklist({ ids }), onSuccess }),
    manualImport: useMutation({ mutationFn: (v: MessageInitShape<typeof ManualImportRequestSchema>) => c().manualImport(v), onSuccess }),
  };
}

/**
 * The video files of a tracked download, or of a folder, as a manual import
 * would take them: for the movie each matches, or the one given, or for the
 * episodes of a series each holds.
 */
export function useManualImport(server: ServerEntry, target: { id?: string; folder?: string; movieId?: string; seriesId?: string }, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "downloads", "manual-import", target.id ?? "", target.folder ?? "", target.movieId ?? "", target.seriesId ?? ""),
    queryFn: async () => (await servers.client(DownloadService, server).listManualImport(target)).files,
    enabled: enabled && !!(target.id || target.folder),
    retry: false,
  });
}

/** The movies or series of a server's libraries a file can be imported for by hand, by name, those without files too. */
export function useImportTargets(server: ServerEntry, kind: ItemKind.MOVIE | ItemKind.SERIES, search: string, enabled = true) {
  const servers = useServersStore();
  const q = search.trim();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "import-targets", kind, q),
    queryFn: async () =>
      (
        await servers
          .client(ItemService, server)
          .listItems({ kinds: [kind], search: q || undefined, includeVirtual: true, sort: [{ field: SortField.NAME }], limit: 20 })
      ).items,
    enabled,
    placeholderData: (previous) => previous,
  });
}

/** A series' episodes, those without files too, by season and number. */
export function useSeriesEpisodes(server: ServerEntry, seriesId: string | undefined) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "series-episodes", seriesId),
    queryFn: async () => {
      const { items } = await servers
        .client(ItemService, server)
        .listItems({ parentId: seriesId, recursive: true, kinds: [ItemKind.EPISODE], includeVirtual: true });
      return items.toSorted((a, b) => a.parentIndexNumber - b.parentIndexNumber || a.indexNumber - b.indexNumber);
    },
    enabled: !!seriesId,
  });
}

/** The seasons of a series' episodes, in order; specials, season 0, last. */
export function seasonsOf(episodes: Pick<Item, "parentIndexNumber">[]): number[] {
  return [...new Set(episodes.map((e) => e.parentIndexNumber))].sort((a, b) => (a === 0 ? 1 : b === 0 ? -1 : a - b));
}

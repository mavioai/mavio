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
import { type Release, ReleaseService } from "@mavio/proto/mavio/acquisition/v1/release_pb";
import {
  type SeriesMonitor,
  type Wanted,
  type WantedFilter,
  WantedSchema,
  type WantedKind,
  WantedService,
} from "@mavio/proto/mavio/acquisition/v1/wanted_pb";
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

/** The video files of a tracked download, or of a folder, as a manual import would take them. */
export function useManualImport(server: ServerEntry, target: { id?: string; folder?: string; movieId?: string }, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "downloads", "manual-import", target.id ?? "", target.folder ?? "", target.movieId ?? ""),
    queryFn: async () => (await servers.client(DownloadService, server).listManualImport(target)).files,
    enabled: enabled && !!(target.id || target.folder),
    retry: false,
  });
}

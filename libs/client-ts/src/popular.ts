// Popular searches (docs/tv.md §6.4): the queries each server records and
// ranks, merged across the servers signed in to. A search is recorded when
// a result it found is opened, so that what the keyboard types on the way
// to a query is not.
import { ItemService } from "@mavio/proto/mavio/library/v1/item_pb";
import { useMutation, useQueries } from "@tanstack/react-query";
import { useCallback } from "react";

import { serverKey, useServersStore, useSignedIn } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/** How many popular searches are shown. */
export const popularSearchLimit = 10;

/**
 * Merges the servers' popular searches, each ranked best first, into one
 * list: the first of every server, then the second of every server, and so
 * on, each query once whatever its case or spacing, at most limit.
 */
export function mergePopularSearches(lists: string[][], limit = popularSearchLimit): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  const longest = Math.max(0, ...lists.map((l) => l.length));
  for (let i = 0; i < longest && out.length < limit; i++) {
    for (const list of lists) {
      const q = list[i]?.trim().replace(/\s+/g, " ");
      if (!q) continue;
      const key = q.toLowerCase();
      if (seen.has(key)) continue;
      seen.add(key);
      out.push(q);
      if (out.length === limit) break;
    }
  }
  return out;
}

/** The popular searches of every server signed in to, merged; pending until each has answered or failed. */
export function usePopularSearches(): { queries: string[]; pending: boolean } {
  const servers = useServersStore();
  const list = useSignedIn();
  return useQueries({
    queries: list.map((s) => ({
      queryKey: serverKey(s.id, "popularSearches"),
      queryFn: async () => (await servers.client(ItemService, s).listPopularSearches({ limit: popularSearchLimit })).queries,
      staleTime: 5 * 60_000,
    })),
    combine: (results) => ({
      queries: mergePopularSearches(results.map((r) => r.data ?? [])),
      pending: results.some((r) => r.isPending),
    }),
  });
}

/** Records on a server that a search for a query found what was opened; failures are ignored. */
export function useRecordSearch(): (server: ServerEntry, query: string) => void {
  const servers = useServersStore();
  const { mutate } = useMutation({
    mutationFn: async ({ server, query }: { server: ServerEntry; query: string }) =>
      (await servers.client(ItemService, server).recordSearch({ query })).recorded,
  });
  return useCallback(
    (server: ServerEntry, query: string) => {
      const q = query.trim();
      if (q !== "") mutate({ server, query: q });
    },
    [mutate],
  );
}

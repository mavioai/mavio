// Discovery and requests (docs/clients.md §6.3.2): the catalogs of the
// first server that discovers, their titles page by page, a title with its
// seasons, and requests of it.
import { type Catalog, DiscoverService, type DiscoveredTitle, type Title } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { type Request, RequestService } from "@mavio/proto/mavio/acquisition/v1/request_pb";
import { CatalogKind } from "@mavio/proto/mavio/plugin/v1/discovery_pb";
import { useInfiniteQuery, useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore, useSignedIn } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/** A row of Discover: a catalog, of one kind of title unless it mixes them. */
export interface CatalogRow {
  catalog: Catalog;
  /** Unspecified for a catalog listing movies and series together. */
  mediaKind: MediaKind;
}

/** The catalogs Discover shows, in its order; those that need a title to start from are left out. */
const rowKinds = [CatalogKind.TRENDING, CatalogKind.POPULAR, CatalogKind.UPCOMING, CatalogKind.TOP_RATED, CatalogKind.LIST];

/** A catalog's rows: one, or one per kind of title it lists apart. */
export function rowsOf(catalogs: Catalog[]): CatalogRow[] {
  const rows: CatalogRow[] = [];
  for (const kind of rowKinds) {
    for (const catalog of catalogs.filter((c) => c.kind === kind)) {
      if (catalog.mixed || catalog.mediaKinds.length < 2) {
        rows.push({ catalog, mediaKind: catalog.mixed ? MediaKind.UNSPECIFIED : (catalog.mediaKinds[0] ?? MediaKind.UNSPECIFIED) });
      } else {
        for (const mediaKind of [MediaKind.MOVIE, MediaKind.SERIES]) {
          if (catalog.mediaKinds.includes(mediaKind)) rows.push({ catalog, mediaKind });
        }
      }
    }
  }
  return rows;
}

/**
 * The server Discover asks: the first signed-in server whose discovery
 * providers offer catalogs, with its catalogs; pending while servers have
 * not answered.
 */
export function useDiscovery(language: string): { server?: ServerEntry; catalogs: Catalog[]; pending: boolean } {
  const servers = useServersStore();
  const list = useSignedIn();
  return useQueries({
    queries: list.map((s) => ({
      queryKey: serverKey(s.id, "discover", "catalogs", language),
      queryFn: async () => (await servers.client(DiscoverService, s).listCatalogs({ language })).catalogs,
      staleTime: 10 * 60_000,
      retry: false,
    })),
    combine: (results) => {
      for (const [i, r] of results.entries()) {
        if (r.isPending) return { catalogs: [], pending: true };
        if (r.data && r.data.length > 0) return { server: list[i], catalogs: r.data, pending: false };
      }
      return { catalogs: [], pending: false };
    },
  });
}

/** A catalog's titles page by page; tmdbId names the title of recommendations and similar titles. */
export function useCatalog(server: ServerEntry | undefined, catalog: string, mediaKind: MediaKind, language: string, tmdbId = 0) {
  const servers = useServersStore();
  return useInfiniteQuery({
    queryKey: serverKey(server?.id ?? "", "discover", "catalog", catalog, mediaKind, tmdbId, language),
    queryFn: async ({ pageParam }) => {
      const r = await servers.client(DiscoverService, server!).discover({ catalog, mediaKind, tmdbId, language, page: pageParam });
      return { titles: r.titles, next: r.page < r.totalPages ? r.page + 1 : undefined };
    },
    initialPageParam: 1,
    getNextPageParam: (last) => last.next,
    enabled: !!server,
    staleTime: 5 * 60_000,
  });
}

/** The titles of all a catalog's pages loaded, without the same title twice. */
export function titlesOf(pages: { titles: DiscoveredTitle[] }[] | undefined): DiscoveredTitle[] {
  const seen = new Set<string>();
  return (pages ?? []).flatMap((p) =>
    p.titles.filter((t) => {
      const key = `${t.mediaKind}/${t.externalIds.tmdb ?? t.name}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    }),
  );
}

/** A movie or series by its TMDB ID, with what the server has of it and of its seasons. */
export function useTitle(server: ServerEntry | undefined, mediaKind: MediaKind, tmdbId: number) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server?.id ?? "", "discover", "title", mediaKind, tmdbId),
    queryFn: async (): Promise<Title | undefined> =>
      (await servers.client(DiscoverService, server!).getTitle({ mediaKind, tmdbId })).title,
    enabled: !!server && tmdbId > 0,
  });
}

/**
 * Requests a movie, or seasons of a series (all those not requested yet
 * when seasons is unset), refreshing what discovery shows of it.
 */
export function useRequestTitle(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ mediaKind, tmdbId, seasons }: { mediaKind: MediaKind; tmdbId: number; seasons?: number[] }): Promise<Request | undefined> =>
      (
        await servers.client(RequestService, server).createRequest({
          mediaKind,
          tmdbId,
          seasons: seasons ?? [],
          allSeasons: mediaKind === MediaKind.SERIES && seasons === undefined,
        })
      ).request,
    onSettled: () => qc.invalidateQueries({ queryKey: serverKey(server.id, "discover") }),
  });
}

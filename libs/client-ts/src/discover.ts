// Discovery and requests (docs/clients.md §6.3.2): the catalogs of the
// first server that discovers, their titles page by page, filtered and
// sorted (§6.3.28), the values of their filters, titles found by name, a
// title with its seasons, and requests of it.
import type { MessageInitShape } from "@bufbuild/protobuf";
import { type Catalog, type DiscoverRequestSchema, DiscoverService, type DiscoveredTitle, type SeasonEpisode, type Title } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { type Request, RequestService } from "@mavio/proto/mavio/acquisition/v1/request_pb";
import { CatalogKind, type DiscoverFilter, DiscoverSort } from "@mavio/proto/mavio/plugin/v1/discovery_pb";
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

/** A filter value chosen by its provider's ID, with its name to show. */
export interface NamedValue {
  id: string;
  name: string;
}

/** How a catalog's titles are filtered and sorted; unset fields do not filter. */
export interface CatalogFilters {
  sort?: DiscoverSort;
  ascending?: boolean;
  genres?: string[];
  studios?: NamedValue[];
  keywords?: NamedValue[];
  watchProviders?: string[];
  /** ISO 3166-1 country of the watch providers. */
  watchRegion?: string;
  originalLanguage?: string;
  /** Years of release, inclusive. */
  yearFrom?: number;
  yearTo?: number;
  /** Minutes. */
  runtimeMin?: number;
  runtimeMax?: number;
  /** 0–10. */
  ratingMin?: number;
  votesMin?: number;
  /** The TMDB collection of a collection catalog. */
  collectionId?: number;
}

/** How many filters are set, the sort aside. */
export function filterCount(f: CatalogFilters): number {
  return [
    f.genres?.length,
    f.studios?.length,
    f.keywords?.length,
    f.watchProviders?.length,
    f.originalLanguage,
    f.yearFrom || f.yearTo,
    f.runtimeMin || f.runtimeMax,
    f.ratingMin,
    f.votesMin,
  ].filter(Boolean).length;
}

/** The fields of a DiscoverRequest that filter and sort as filters do. */
export function discoverRequestOf(f: CatalogFilters): MessageInitShape<typeof DiscoverRequestSchema> {
  const providers = f.watchProviders ?? [];
  return {
    sort: f.sort ?? DiscoverSort.UNSPECIFIED,
    ascending: f.ascending ?? false,
    genres: f.genres ?? [],
    studios: (f.studios ?? []).map((v) => v.id),
    keywords: (f.keywords ?? []).map((v) => v.id),
    watchProviders: providers,
    watchRegion: providers.length > 0 ? (f.watchRegion ?? "") : "",
    originalLanguage: f.originalLanguage ?? "",
    releasedFrom: f.yearFrom ? `${f.yearFrom}-01-01` : "",
    releasedTo: f.yearTo ? `${f.yearTo}-12-31` : "",
    runtimeMin: f.runtimeMin ?? 0,
    runtimeMax: f.runtimeMax ?? 0,
    ratingMin: f.ratingMin ?? 0,
    votesMin: f.votesMin ?? 0,
    tmdbCollectionId: f.collectionId ?? 0,
  };
}

/**
 * A catalog's titles page by page, filtered and sorted; tmdbId names the
 * title of recommendations and similar titles.
 */
export function useCatalog(server: ServerEntry | undefined, catalog: string, mediaKind: MediaKind, language: string, tmdbId = 0, filters: CatalogFilters = {}) {
  const servers = useServersStore();
  return useInfiniteQuery({
    queryKey: serverKey(server?.id ?? "", "discover", "catalog", catalog, mediaKind, tmdbId, language, filters),
    queryFn: async ({ pageParam }) => {
      const r = await servers
        .client(DiscoverService, server!)
        .discover({ ...discoverRequestOf(filters), catalog, mediaKind, tmdbId, language, page: pageParam });
      return { titles: r.titles, next: r.page < r.totalPages ? r.page + 1 : undefined };
    },
    initialPageParam: 1,
    getNextPageParam: (last) => last.next,
    enabled: !!server,
    staleTime: 5 * 60_000,
  });
}

/**
 * The values a catalog's filter takes: all of them for genres and watch
 * providers, those matching a query for studios and keywords, none
 * without one.
 */
export function useFilterValues(
  server: ServerEntry | undefined,
  v: { catalog: string; filter: DiscoverFilter; mediaKind: MediaKind; query?: string; language: string; region?: string },
  enabled = true,
) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server?.id ?? "", "discover", "filterValues", v),
    queryFn: async () => (await servers.client(DiscoverService, server!).listFilterValues({ ...v, query: v.query ?? "", region: v.region ?? "" })).values,
    enabled: !!server && enabled,
    staleTime: 60 * 60_000,
  });
}

/** Movies and series the first discovery provider that searches finds by name, page by page; none for an empty query. */
export function useSearchTitles(server: ServerEntry | undefined, query: string, language: string) {
  const servers = useServersStore();
  return useInfiniteQuery({
    queryKey: serverKey(server?.id ?? "", "discover", "search", query, language),
    queryFn: async ({ pageParam }) => {
      const r = await servers.client(DiscoverService, server!).searchTitles({ query, language, page: pageParam });
      return { titles: r.titles, next: r.page < r.totalPages ? r.page + 1 : undefined };
    },
    initialPageParam: 1,
    getNextPageParam: (last) => last.next,
    enabled: !!server && query.trim() !== "",
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

/**
 * A movie or series by its TMDB ID, with what the server has of it and of
 * its seasons, in a language (ISO 639-1), the server's metadata language
 * when empty.
 */
export function useTitle(server: ServerEntry | undefined, mediaKind: MediaKind, tmdbId: number, language = "") {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server?.id ?? "", "discover", "title", mediaKind, tmdbId, language),
    queryFn: async (): Promise<Title | undefined> =>
      (await servers.client(DiscoverService, server!).getTitle({ mediaKind, tmdbId, language })).title,
    enabled: !!server && tmdbId > 0,
  });
}

/**
 * The episodes of a season of a series by its TMDB ID, in a language (ISO
 * 639-1), the server's metadata language when empty; fetched while enabled,
 * as when the season is expanded.
 */
export function useSeason(server: ServerEntry | undefined, tmdbId: number, seasonNumber: number, language = "", enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server?.id ?? "", "discover", "season", tmdbId, seasonNumber, language),
    queryFn: async (): Promise<SeasonEpisode[]> =>
      (await servers.client(DiscoverService, server!).getSeason({ tmdbId, seasonNumber, language })).episodes,
    enabled: enabled && !!server && tmdbId > 0,
    staleTime: 60 * 60_000,
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
    onSettled: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: serverKey(server.id, "discover") }),
        qc.invalidateQueries({ queryKey: serverKey(server.id, "requests") }),
      ]),
  });
}

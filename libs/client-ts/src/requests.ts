// Requests of movies and seasons of series (docs/clients.md §6.3.2): the
// user's own, or everyone's for administrators, page by page; approving,
// declining, retrying, changing the seasons of and deleting them; and the
// user's quotas.
import { RequestService, type RequestStatus } from "@mavio/proto/mavio/acquisition/v1/request_pb";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

const pageSize = 30;

/** The requests in any of the statuses, all when none, newest first, page by page. */
export function useRequests(server: ServerEntry | undefined, statuses: RequestStatus[]) {
  const servers = useServersStore();
  return useInfiniteQuery({
    queryKey: serverKey(server?.id ?? "", "requests", "list", statuses),
    queryFn: async ({ pageParam }) => {
      const r = await servers.client(RequestService, server!).listRequests({ statuses, limit: pageSize, offset: pageParam });
      return { requests: r.requests, total: r.total, next: pageParam + r.requests.length < r.total ? pageParam + r.requests.length : undefined };
    },
    initialPageParam: 0,
    getNextPageParam: (last) => last.next,
    enabled: !!server,
  });
}

/** How much of their quotas the user has used. */
export function useQuota(server: ServerEntry | undefined) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server?.id ?? "", "requests", "quota"),
    queryFn: () => servers.client(RequestService, server!).getQuota({}),
    enabled: !!server,
  });
}

/** What can be done to a request or the blocklist, each refreshing the requests and what discovery shows. */
export function useRequestActions(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const c = () => servers.client(RequestService, server);
  const onSuccess = () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: serverKey(server.id, "requests") }),
      qc.invalidateQueries({ queryKey: serverKey(server.id, "discover") }),
    ]);
  return {
    approve: useMutation({ mutationFn: (id: string) => c().approveRequest({ id }), onSuccess }),
    decline: useMutation({ mutationFn: (id: string) => c().declineRequest({ id }), onSuccess }),
    retry: useMutation({ mutationFn: (id: string) => c().retryRequest({ id }), onSuccess }),
    update: useMutation({ mutationFn: (v: { id: string; seasons: number[] }) => c().updateRequest(v), onSuccess }),
    remove: useMutation({ mutationFn: (id: string) => c().deleteRequest({ id }), onSuccess }),
    blockCollection: useMutation({ mutationFn: (tmdbCollectionId: number) => c().blockCollection({ tmdbCollectionId }), onSuccess }),
    unblockCollection: useMutation({ mutationFn: (tmdbCollectionId: number) => c().unblockCollection({ tmdbCollectionId }), onSuccess }),
  };
}

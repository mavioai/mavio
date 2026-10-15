// Issues (docs/clients.md §6.3.25): the problems users report with items,
// the user's own or everyone's for administrators, page by page; reported,
// commented on, resolved, reopened and deleted; and the current user, who
// may report them.
import type { MessageInitShape } from "@bufbuild/protobuf";
import { type CreateIssueRequestSchema, IssueService, IssueSort, type IssueStatus } from "@mavio/proto/mavio/acquisition/v1/issue_pb";
import { UserService } from "@mavio/proto/mavio/user/v1/user_pb";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

const pageSize = 30;

/** The signed-in user of a server, with their policy. */
export function useCurrentUser(server: ServerEntry | undefined) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server?.id ?? "", "currentUser", server?.userId),
    queryFn: async () => (await servers.client(UserService, server!).getCurrentUser({})).user ?? null,
    staleTime: 5 * 60_000,
    enabled: !!server,
  });
}

/** The issues in any of the statuses, all when none, of an item when given, last changed first, page by page. */
export function useIssues(server: ServerEntry, statuses: IssueStatus[], itemId?: string) {
  const servers = useServersStore();
  return useInfiniteQuery({
    queryKey: serverKey(server.id, "issues", "list", statuses, itemId),
    queryFn: async ({ pageParam }) => {
      const r = await servers.client(IssueService, server).listIssues({ statuses, itemId, sort: IssueSort.MODIFIED, limit: pageSize, offset: pageParam });
      return { issues: r.issues, total: r.total, next: pageParam + r.issues.length < r.total ? pageParam + r.issues.length : undefined };
    },
    initialPageParam: 0,
    getNextPageParam: (last) => last.next,
  });
}

/** An issue with its comments. */
export function useIssue(server: ServerEntry, id: string) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "issues", "one", id),
    queryFn: async () => (await servers.client(IssueService, server).getIssue({ id })).issue,
  });
}

/** What can be done to issues, each refreshing them. */
export function useIssueActions(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const c = () => servers.client(IssueService, server);
  const onSuccess = () => qc.invalidateQueries({ queryKey: serverKey(server.id, "issues") });
  return {
    create: useMutation({
      mutationFn: async (v: MessageInitShape<typeof CreateIssueRequestSchema>) => (await c().createIssue(v)).issue,
      onSuccess,
    }),
    comment: useMutation({ mutationFn: (v: { id: string; message: string }) => c().addComment(v), onSuccess }),
    resolve: useMutation({ mutationFn: (id: string) => c().resolveIssue({ id }), onSuccess }),
    reopen: useMutation({ mutationFn: (id: string) => c().reopenIssue({ id }), onSuccess }),
    // The deleted issue goes from the cache rather than being fetched again.
    remove: useMutation({
      mutationFn: (id: string) => c().deleteIssue({ id }),
      onSuccess: (_, id) => {
        qc.removeQueries({ queryKey: serverKey(server.id, "issues", "one", id) });
        return qc.invalidateQueries({ queryKey: serverKey(server.id, "issues", "list") });
      },
    }),
  };
}

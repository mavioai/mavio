// A person's page (docs/clients.md §6.3.19): who they are, and the titles
// of the server crediting them.
import { ItemKind, ItemService, SortField } from "@mavio/proto/mavio/library/v1/item_pb";
import { useQuery } from "@tanstack/react-query";

import { libraryStaleTime, serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/** A person with their biography, dates and links. */
export function usePerson(server: ServerEntry, personId: string) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "person", personId),
    queryFn: async () => (await servers.client(ItemService, server).getPerson({ id: personId })).person ?? null,
    staleTime: libraryStaleTime,
  });
}

/** The movies, series and episodes crediting a person, newest first. */
export function usePersonItems(server: ServerEntry, personId: string) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "person", personId, "items"),
    queryFn: async () =>
      (
        await servers.client(ItemService, server).listItems({
          personId,
          kinds: [ItemKind.MOVIE, ItemKind.SERIES, ItemKind.EPISODE],
          sort: [{ field: SortField.PREMIERE_DATE, descending: true }],
        })
      ).items,
    staleTime: libraryStaleTime,
  });
}

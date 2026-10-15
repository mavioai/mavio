// An item's metadata, for administrators (docs/clients.md §6.3.22): its
// fields edited by an update mask, refreshed from its files and the
// providers, identified anew from the providers' search results, and its
// images chosen from the providers' or uploaded.
import type { MessageInitShape } from "@bufbuild/protobuf";
import type { ImageKind, ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { type ItemMetadataSchema, MetadataService } from "@mavio/proto/mavio/library/v1/metadata_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/** The kinds of external IDs items of a kind carry. */
export function useExternalIdKinds(server: ServerEntry, itemKind: ItemKind, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "external-id-kinds", itemKind),
    queryFn: async () => (await servers.client(MetadataService, server).listExternalIdKinds({ itemKind })).kinds,
    staleTime: Infinity,
    enabled,
  });
}

/** What the providers find an item may be, searched when a query is given. */
export function useRemoteSearch(server: ServerEntry, itemId: string, query?: { name: string; year?: number }) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "remote-search", itemId, query?.name, query?.year),
    queryFn: async () => (await servers.client(MetadataService, server).searchRemote({ itemId, ...query })).results,
    enabled: query !== undefined,
    staleTime: Infinity,
  });
}

/** The images the providers have of an item, of a kind. */
export function useRemoteImages(server: ServerEntry, itemId: string, kind: ImageKind) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "remote-images", itemId, kind),
    queryFn: async () => (await servers.client(MetadataService, server).listRemoteImages({ itemId, kind })).images,
    staleTime: Infinity,
  });
}

/** What changes an item's metadata and images; each refreshes what shows the item. */
export function useMetadataActions(server: ServerEntry, itemId: string) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const c = () => servers.client(MetadataService, server);
  const onSuccess = () =>
    Promise.all(["item", "children", "items", "episodes", "home"].map((k) => qc.invalidateQueries({ queryKey: serverKey(server.id, k) })));
  return {
    update: useMutation({
      mutationFn: async (v: { metadata: MessageInitShape<typeof ItemMetadataSchema>; paths: string[] }) =>
        (await c().updateItem({ id: itemId, metadata: v.metadata, updateMask: { paths: v.paths } })).item,
      onSuccess,
    }),
    refresh: useMutation({
      mutationFn: async (replaceMetadata: boolean) => (await c().refreshItem({ id: itemId, replaceMetadata })).item,
      onSuccess,
    }),
    identify: useMutation({
      mutationFn: async (externalIds: Record<string, string>) => (await c().identifyItem({ itemId, externalIds })).item,
      onSuccess,
    }),
    setImage: useMutation({
      mutationFn: async (v: { kind: ImageKind; source: { case: "url"; value: string } | { case: "data"; value: Uint8Array } }) =>
        (await c().setItemImage({ itemId, kind: v.kind, source: v.source })).image,
      onSuccess,
    }),
    deleteImage: useMutation({ mutationFn: (imageId: string) => c().deleteItemImage({ itemId, imageId }), onSuccess }),
  };
}

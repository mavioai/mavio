// Playlists and collections (docs/clients.md §6.3.20): the caller's
// playlists and their entries in order, the server's collections, and what
// changes them.
import { CollectionService } from "@mavio/proto/mavio/library/v1/collection_pb";
import { ItemKind, ItemService, SortField } from "@mavio/proto/mavio/library/v1/item_pb";
import { PlaylistService } from "@mavio/proto/mavio/library/v1/playlist_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/** The caller's playlists. */
export function usePlaylists(server: ServerEntry, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "playlists"),
    queryFn: async () => (await servers.client(PlaylistService, server).listPlaylists({})).playlists,
    enabled,
  });
}

/** A playlist's entries in order. */
export function usePlaylistEntries(server: ServerEntry, playlistId: string) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "playlists", playlistId, "entries"),
    queryFn: async () => (await servers.client(PlaylistService, server).listPlaylistEntries({ id: playlistId })).entries,
  });
}

/** The server's collections, by name. */
export function useCollections(server: ServerEntry, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "collections"),
    queryFn: async () =>
      (await servers.client(ItemService, server).listItems({ kinds: [ItemKind.COLLECTION], sort: [{ field: SortField.NAME }] })).items,
    enabled,
  });
}

/**
 * Refreshes what playlists and collections change: the lists, items and
 * libraries' contents; after a deletion, not the item deleted.
 */
function useRefresh(server: ServerEntry, deleted = false) {
  const qc = useQueryClient();
  const keys = ["playlists", "collections", "children", "items", ...(deleted ? [] : ["item"])];
  return () => Promise.all(keys.map((k) => qc.invalidateQueries({ queryKey: serverKey(server.id, k) })));
}

/** What changes the caller's playlists. */
export function usePlaylistActions(server: ServerEntry) {
  const servers = useServersStore();
  const c = () => servers.client(PlaylistService, server);
  const onSuccess = useRefresh(server);
  const onDeleted = useRefresh(server, true);
  return {
    create: useMutation({ mutationFn: async (v: { name: string; itemIds: string[] }) => (await c().createPlaylist(v)).playlist, onSuccess }),
    rename: useMutation({ mutationFn: (v: { id: string; name: string }) => c().updatePlaylist(v), onSuccess }),
    remove: useMutation({ mutationFn: (id: string) => c().deletePlaylist({ id }), onSuccess: onDeleted }),
    add: useMutation({ mutationFn: (v: { id: string; itemIds: string[] }) => c().addToPlaylist(v), onSuccess }),
    removeEntries: useMutation({ mutationFn: (v: { id: string; entryIds: string[] }) => c().removeFromPlaylist(v), onSuccess }),
    move: useMutation({ mutationFn: (v: { id: string; entryId: string; position: number }) => c().movePlaylistEntry(v), onSuccess }),
  };
}

/** What changes the server's collections, for administrators. */
export function useCollectionActions(server: ServerEntry) {
  const servers = useServersStore();
  const c = () => servers.client(CollectionService, server);
  const onSuccess = useRefresh(server);
  const onDeleted = useRefresh(server, true);
  return {
    create: useMutation({ mutationFn: async (v: { name: string; itemIds: string[] }) => (await c().createCollection(v)).collection, onSuccess }),
    rename: useMutation({ mutationFn: (v: { id: string; name: string }) => c().updateCollection(v), onSuccess }),
    remove: useMutation({ mutationFn: (id: string) => c().deleteCollection({ id }), onSuccess: onDeleted }),
    add: useMutation({ mutationFn: (v: { id: string; itemIds: string[] }) => c().addToCollection(v), onSuccess }),
    removeItems: useMutation({ mutationFn: (v: { id: string; itemIds: string[] }) => c().removeFromCollection(v), onSuccess }),
  };
}

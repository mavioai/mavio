// A server's storage (docs/clients.md §6.3.10): the disks of its computer,
// the rclone rcd it runs or reaches, the kinds of remotes and their settings, the remotes configured,
// with the questions their configuration asks, their folders, and the
// download clients of cloud drives' offline downloads.
import { type OfflineClient, StorageService } from "@mavio/proto/mavio/storage/v1/storage_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/** Whether the server reaches an rcd, and its version. */
export function useStorageStatus(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "storage", "status"),
    queryFn: () => servers.client(StorageService, server).getStatus({}),
  });
}

/** The disks of the server's computer, with their sizes. */
export function useDisks(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "storage", "disks"),
    queryFn: async () => (await servers.client(StorageService, server).listDisks({})).disks,
  });
}

/** How much of a remote is used; an error for a remote whose backend does not tell. */
export function useRemoteUsage(server: ServerEntry, remote: string) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "storage", "usage", remote),
    queryFn: () => servers.client(StorageService, server).getRemoteUsage({ remote }),
    retry: false,
    staleTime: 60_000,
  });
}

/** The kinds of remotes and the settings each takes. */
export function useStorageProviders(server: ServerEntry, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "storage", "providers"),
    queryFn: async () => (await servers.client(StorageService, server).listProviders({})).providers,
    staleTime: Infinity,
    enabled,
  });
}

/** The remotes configured. */
export function useRemotes(server: ServerEntry, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "storage", "remotes"),
    queryFn: async () => (await servers.client(StorageService, server).listRemotes({})).remotes,
    enabled,
  });
}

/** The folders of a remote's folder, "rclone:<remote>:<path>". */
export function useRemoteFolders(server: ServerEntry, path: string, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "storage", "folders", path),
    queryFn: async () => (await servers.client(StorageService, server).listFolders({ path })).folders,
    enabled,
    retry: false,
  });
}

/**
 * Configuring remotes: creating, updating and continuing each answer a
 * remote and, while its configuration needs more, a question to continue
 * with.
 */
export function useRemoteActions(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const onSettled = () => qc.invalidateQueries({ queryKey: serverKey(server.id, "storage", "remotes") });
  const client = () => servers.client(StorageService, server);
  return {
    create: useMutation({
      mutationFn: (r: { name: string; type: string; parameters: Record<string, string> }) => client().createRemote(r),
      onSettled,
    }),
    update: useMutation({ mutationFn: (r: { name: string; parameters: Record<string, string> }) => client().updateRemote(r), onSettled }),
    answer: useMutation({ mutationFn: (r: { name: string; state: string; answer: string }) => client().continueRemote(r), onSettled }),
    remove: useMutation({ mutationFn: (name: string) => client().deleteRemote({ name }), onSettled }),
  };
}

/** The download clients of offline downloads. */
export function useOfflineClients(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "storage", "offline"),
    queryFn: async () => (await servers.client(StorageService, server).listOfflineClients({})).clients,
  });
}

/** Replaces the download clients of offline downloads. */
export function useSetOfflineClients(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (clients: OfflineClient[]) => servers.client(StorageService, server).setOfflineClients({ clients }),
    onSettled: () => qc.invalidateQueries({ queryKey: serverKey(server.id, "storage", "offline") }),
  });
}

// What decides which releases a server grabs (docs/clients.md §6.3.11):
// quality definitions and profiles, custom formats, delay and release
// profiles and remote path mappings, and the indexers and download
// clients its plugins have configured.
import { DownloadService } from "@mavio/proto/mavio/acquisition/v1/download_pb";
import {
  type CustomFormat,
  type DelayProfile,
  type MediaKind,
  ProfileService,
  type QualityDefinition,
  type QualityProfile,
  type ReleaseProfile,
  type RemotePathMapping,
} from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { ReleaseService } from "@mavio/proto/mavio/acquisition/v1/release_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/** The indexers the indexer plugins have configured, and the plugins that failed to list theirs. */
export function useIndexers(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "indexers"),
    queryFn: () => servers.client(ReleaseService, server).listIndexers({}),
  });
}

/** The download clients the download client plugins have configured. */
export function useDownloadClients(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "clients"),
    queryFn: () => servers.client(DownloadService, server).listDownloadClients({}),
  });
}

/** The sizes of a kind's qualities. */
export function useQualityDefinitions(server: ServerEntry, kind: MediaKind) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const key = serverKey(server.id, "acquisition", "definitions", kind);
  return {
    list: useQuery({ queryKey: key, queryFn: async () => (await servers.client(ProfileService, server).listQualityDefinitions({ kind })).definitions }),
    update: useMutation({
      mutationFn: (definitions: QualityDefinition[]) => servers.client(ProfileService, server).updateQualityDefinitions({ kind, definitions }),
      onSuccess: (r) => qc.setQueryData(key, r.definitions),
    }),
  };
}

/**
 * A list kept by ProfileService and the mutations of its entries, each
 * refreshing the list.
 */
function useEntries<T, C, R, U, D>(
  server: ServerEntry,
  name: string,
  list: () => Promise<T[]>,
  create: (v: C) => Promise<R>,
  update: (v: U) => Promise<unknown>,
  remove: (v: D) => Promise<unknown>,
) {
  const qc = useQueryClient();
  const queryKey = serverKey(server.id, "acquisition", name);
  const onSettled = () => qc.invalidateQueries({ queryKey });
  return {
    list: useQuery({ queryKey, queryFn: list }),
    create: useMutation({ mutationFn: create, onSettled }),
    update: useMutation({ mutationFn: update, onSettled }),
    remove: useMutation({ mutationFn: remove, onSettled }),
  };
}

/** The quality profiles, of every kind. */
export function useQualityProfiles(server: ServerEntry) {
  const servers = useServersStore();
  const c = () => servers.client(ProfileService, server);
  return useEntries(
    server,
    "profiles",
    async () => (await c().listQualityProfiles({})).profiles,
    (profile: QualityProfile) => c().createQualityProfile({ profile }),
    (profile: QualityProfile) => c().updateQualityProfile({ profile }),
    (id: string) => c().deleteQualityProfile({ id }),
  );
}

/** The custom formats. */
export function useCustomFormats(server: ServerEntry) {
  const servers = useServersStore();
  const c = () => servers.client(ProfileService, server);
  return useEntries(
    server,
    "formats",
    async () => (await c().listCustomFormats({})).formats,
    (format: CustomFormat) => c().createCustomFormat({ format }),
    (format: CustomFormat) => c().updateCustomFormat({ format }),
    (id: string) => c().deleteCustomFormat({ id }),
  );
}

/** The delay profiles in order, the default one last, and their reordering. */
export function useDelayProfiles(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const c = () => servers.client(ProfileService, server);
  const entries = useEntries(
    server,
    "delays",
    async () => (await c().listDelayProfiles({})).profiles,
    (profile: DelayProfile) => c().createDelayProfile({ profile }),
    (profile: DelayProfile) => c().updateDelayProfile({ profile }),
    (id: string) => c().deleteDelayProfile({ id }),
  );
  return {
    ...entries,
    reorder: useMutation({
      mutationFn: (ids: string[]) => c().reorderDelayProfiles({ ids }),
      onSuccess: (r) => qc.setQueryData(serverKey(server.id, "acquisition", "delays"), r.profiles),
    }),
  };
}

/** The release profiles. */
export function useReleaseProfiles(server: ServerEntry) {
  const servers = useServersStore();
  const c = () => servers.client(ProfileService, server);
  return useEntries(
    server,
    "releases",
    async () => (await c().listReleaseProfiles({})).profiles,
    (profile: ReleaseProfile) => c().createReleaseProfile({ profile }),
    (profile: ReleaseProfile) => c().updateReleaseProfile({ profile }),
    (id: string) => c().deleteReleaseProfile({ id }),
  );
}

/** The remote path mappings, which are added and deleted but not changed. */
export function useRemotePathMappings(server: ServerEntry) {
  const servers = useServersStore();
  const c = () => servers.client(ProfileService, server);
  const { list, create, remove } = useEntries(
    server,
    "mappings",
    async () => (await c().listRemotePathMappings({})).mappings,
    (mapping: RemotePathMapping) => c().createRemotePathMapping({ mapping }),
    async () => undefined,
    (id: string) => c().deleteRemotePathMapping({ id }),
  );
  return { list, create, remove };
}

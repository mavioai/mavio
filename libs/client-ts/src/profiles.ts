// What decides which releases a server grabs (docs/clients.md §6.3.11):
// quality definitions and profiles, custom formats, delay and release
// profiles, and the indexers and download clients its plugins have
// configured, with where the server reads the clients' folders.
import { type DownloadClientPath, DownloadService } from "@mavio/proto/mavio/acquisition/v1/download_pb";
import {
  type CustomFormat,
  type DelayProfile,
  type MediaKind,
  ProfileService,
  type QualityDefinition,
  type QualityProfile,
  type QualityProfileItem,
  QualityProfileItemSchema,
  QualityProfileSchema,
  type ReleaseProfile,
} from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { clone, create } from "@bufbuild/protobuf";
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

/** The folders of a download client, with where the server reads each. */
export function useDownloadClientPaths(server: ServerEntry, pluginId: string, clientId: string) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "client-paths", pluginId, clientId),
    queryFn: () => servers.client(DownloadService, server).getDownloadClientPaths({ pluginId, clientId }),
    enabled: !!pluginId && !!clientId,
  });
}

/** Replaces where the server reads the folders of a download client. */
export function useUpdateDownloadClientPaths(server: ServerEntry, pluginId: string, clientId: string) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const key = serverKey(server.id, "acquisition", "client-paths", pluginId, clientId);
  return useMutation({
    mutationFn: (paths: DownloadClientPath[]) => servers.client(DownloadService, server).updateDownloadClientPaths({ pluginId, clientId, paths }),
    onSuccess: (res) => qc.setQueryData(key, res),
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
 * A list of acquisition's and the mutations of its entries, each
 * refreshing the list.
 */
export function useEntries<T, C, R, U, D>(
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


/** The ID a profile's item is ranked and cut off by: a group's, else its quality's. */
export const profileItemId = (it: QualityProfileItem) => it.id || it.quality;

/** The first ID of the groups of a profile, as Radarr numbers them. */
const firstGroupId = 1000;

/** A profile with its items and cutoff changed, the rest as it was. */
function withItems(p: QualityProfile, items: QualityProfileItem[], cutoff = p.cutoff): QualityProfile {
  const out = clone(QualityProfileSchema, p);
  out.items = items;
  out.cutoff = cutoff;
  return out;
}

/** A quality of a profile, on its own or in a group, allowed or not. */
const quality = (q: number, allowed: boolean) => create(QualityProfileItemSchema, { quality: q, allowed });

/**
 * A profile with the items of keys (profileItemId) merged into a group of
 * their qualities, lowest first as they were, named name, at the place of
 * the best of them. The group is allowed when any of them was, and is the
 * cutoff when one of them was. Fewer than two qualities make no group.
 */
export function groupQualities(p: QualityProfile, keys: readonly number[], name: string): QualityProfile {
  const picked = p.items.filter((it) => keys.includes(profileItemId(it)));
  const qualities = picked.flatMap((it) => (it.id ? it.items : [it]));
  if (qualities.length < 2) return p;
  const id = Math.max(firstGroupId - 1, ...p.items.map((it) => it.id)) + 1;
  const allowed = picked.some((it) => it.allowed);
  const group = create(QualityProfileItemSchema, { id, name: name.trim(), allowed, items: qualities.map((q) => quality(q.quality, allowed)) });
  const best = p.items.indexOf(picked[picked.length - 1]);
  const items = p.items.filter((it) => !picked.includes(it));
  items.splice(best - (picked.length - 1), 0, group);
  return withItems(p, items, keys.includes(p.cutoff) ? id : p.cutoff);
}

/**
 * A profile with the group of id parted into its qualities, in its place,
 * each allowed as the group was; the best of them takes its place as the
 * cutoff.
 */
export function ungroupQualities(p: QualityProfile, id: number): QualityProfile {
  const group = p.items.find((it) => it.id === id);
  if (!group) return p;
  const parts = group.items.map((q) => quality(q.quality, group.allowed));
  const items = p.items.flatMap((it) => (it === group ? parts : [it]));
  return withItems(p, items, p.cutoff === id ? parts[parts.length - 1].quality : p.cutoff);
}

/**
 * A profile with a quality taken out of the group of id, placed on its own
 * just below the group; a group left with one quality is parted.
 */
export function removeFromGroup(p: QualityProfile, id: number, q: number): QualityProfile {
  const group = p.items.find((it) => it.id === id);
  if (!group || !group.items.some((g) => g.quality === q)) return p;
  if (group.items.length <= 2) return ungroupQualities(p, id);
  const kept = patchedGroup(group, { items: group.items.filter((g) => g.quality !== q) });
  const items = p.items.flatMap((it) => (it === group ? [quality(q, group.allowed), kept] : [it]));
  return withItems(p, items);
}

/** A group of a profile changed. */
function patchedGroup(it: QualityProfileItem, patch: Partial<Pick<QualityProfileItem, "name" | "items">>): QualityProfileItem {
  const out = clone(QualityProfileItemSchema, it);
  Object.assign(out, patch);
  return out;
}

/** A profile with the group of id named name. */
export function renameGroup(p: QualityProfile, id: number, name: string): QualityProfile {
  return withItems(
    p,
    p.items.map((it) => (it.id === id ? patchedGroup(it, { name }) : it)),
  );
}

/** A profile with the quality at index i of the group of id moved by places within it, lowest first. */
export function moveInGroup(p: QualityProfile, id: number, i: number, by: number): QualityProfile {
  return withItems(
    p,
    p.items.map((it) => {
      if (it.id !== id) return it;
      const to = Math.min(it.items.length - 1, Math.max(0, i + by));
      const items = [...it.items];
      const [q] = items.splice(i, 1);
      items.splice(to, 0, q);
      return patchedGroup(it, { items });
    }),
  );
}

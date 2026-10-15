// How a server acquires subtitles (docs/clients.md §6.3.12): subtitle
// profiles and the libraries they are assigned to, the settings, and the
// providers and translators its plugins have configured; and a movie's or
// episode's own subtitles.
import type { MessageInitShape } from "@bufbuild/protobuf";
import {
  type SubtitleLanguageSpecSchema,
  type SubtitleProfile,
  type SubtitleSettings,
  SubtitleService,
} from "@mavio/proto/mavio/acquisition/v1/subtitle_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import { useEntries } from "./profiles.ts";
import type { ServerEntry } from "./servers.ts";

/** The subtitle profiles, by name. */
export function useSubtitleProfiles(server: ServerEntry) {
  const servers = useServersStore();
  const c = () => servers.client(SubtitleService, server);
  return useEntries(
    server,
    "subtitle-profiles",
    async () => (await c().listSubtitleProfiles({})).profiles,
    (profile: SubtitleProfile) => c().createSubtitleProfile({ profile }),
    (profile: SubtitleProfile) => c().updateSubtitleProfile({ profile }),
    (id: string) => c().deleteSubtitleProfile({ id }),
  );
}

/**
 * The profiles assigned to libraries, movies and series, as a map from
 * target to profile, and assigning one; an empty profile removes the
 * assignment.
 */
export function useSubtitleAssignments(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const queryKey = serverKey(server.id, "acquisition", "subtitle-assignments");
  const c = () => servers.client(SubtitleService, server);
  return {
    list: useQuery({
      queryKey,
      queryFn: async () => new Map((await c().listSubtitleProfileAssignments({})).assignments.map((a) => [a.targetId, a.profileId])),
    }),
    assign: useMutation({
      mutationFn: (v: { targetId: string; profileId: string }) => c().assignSubtitleProfile(v),
      onSettled: () => qc.invalidateQueries({ queryKey }),
    }),
  };
}

/** The subtitle settings and their replacement. */
export function useSubtitleSettings(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const queryKey = serverKey(server.id, "acquisition", "subtitle-settings");
  const c = () => servers.client(SubtitleService, server);
  return {
    get: useQuery({ queryKey, queryFn: async () => (await c().getSubtitleSettings({})).settings }),
    update: useMutation({
      mutationFn: (settings: SubtitleSettings) => c().updateSubtitleSettings({ settings }),
      onSuccess: (r) => qc.setQueryData(queryKey, r.settings),
    }),
  };
}

/** The providers the subtitle provider plugins have configured. */
export function useSubtitleProviders(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "subtitle-providers"),
    queryFn: () => servers.client(SubtitleService, server).listSubtitleProviders({}),
  });
}

/** The translators the subtitle translator plugins have configured. */
export function useSubtitleTranslators(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "subtitle-translators"),
    queryFn: () => servers.client(SubtitleService, server).listSubtitleTranslators({}),
  });
}

/** A movie's or episode's subtitles: its profile, those it has and those it lacks (§6.3.21). */
export function useItemSubtitles(server: ServerEntry, itemId: string, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "item-subtitles", itemId),
    queryFn: () => servers.client(SubtitleService, server).getItemSubtitles({ itemId }),
    enabled,
  });
}

/** The providers' subtitles of a movie or episode, best first, searched when asked. */
export function useSubtitleSearch(server: ServerEntry, itemId: string, languages: MessageInitShape<typeof SubtitleLanguageSpecSchema>[], enabled: boolean) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "acquisition", "subtitle-search", itemId, languages.map((l) => `${l.language}:${l.forced}:${l.hearingImpaired}`)),
    queryFn: () => servers.client(SubtitleService, server).searchItemSubtitles({ itemId, languages }),
    enabled,
    staleTime: 5 * 60_000,
    retry: false,
  });
}

/** What changes a movie's or episode's subtitles, each refreshing them and the item's streams. */
export function useItemSubtitleActions(server: ServerEntry, itemId: string) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const c = () => servers.client(SubtitleService, server);
  const onSuccess = () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: serverKey(server.id, "acquisition", "item-subtitles", itemId) }),
      qc.invalidateQueries({ queryKey: serverKey(server.id, "item", itemId) }),
    ]);
  type Spec = MessageInitShape<typeof SubtitleLanguageSpecSchema>;
  return {
    download: useMutation({
      mutationFn: async (v: { provider: string; subtitleId: string; language: Spec }) => (await c().downloadItemSubtitle({ itemId, ...v })).path,
      onSuccess,
    }),
    upload: useMutation({
      mutationFn: async (v: { data: Uint8Array; format: string; language: Spec }) => (await c().uploadSubtitle({ itemId, ...v })).path,
      onSuccess,
    }),
    extract: useMutation({
      mutationFn: async (v: { streamIndex: number; language?: Spec }) => (await c().extractSubtitle({ itemId, ...v })).path,
      onSuccess,
    }),
    remove: useMutation({ mutationFn: (path: string) => c().deleteSubtitle({ itemId, path }), onSuccess }),
    modify: useMutation({ mutationFn: (v: { path: string; mods: string[] }) => c().modifySubtitle({ itemId, ...v }), onSuccess }),
    sync: useMutation({ mutationFn: (v: { path: string; referencePath?: string }) => c().syncSubtitle({ itemId, ...v }), onSuccess }),
    translate: useMutation({
      mutationFn: async (v: { source: { case: "path"; value: string } | { case: "streamIndex"; value: number }; language: Spec; translator?: string }) =>
        (await c().translateSubtitle({ itemId, ...v })).jobId,
    }),
  };
}

// How a server acquires subtitles (docs/clients.md §6.3.12): subtitle
// profiles and the libraries they are assigned to, the settings, and the
// providers and translators its plugins have configured.
import {
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

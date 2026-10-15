// Server administration (docs/clients.md §6.3.4–§6.3.7): a server's
// libraries, with the folders of its file system to choose from, the
// languages, countries and ratings it knows, its users and invitations,
// what it runs and plays, what happened on it, its tasks and jobs, and
// its settings.
import { LibraryService, type LibrarySpec } from "@mavio/proto/mavio/library/v1/library_pb";
import { SessionService } from "@mavio/proto/mavio/session/v1/session_pb";
import { ActivityService, type Severity } from "@mavio/proto/mavio/system/v1/activity_pb";
import { LocalizationService } from "@mavio/proto/mavio/system/v1/localization_pb";
import { type ServerSettings, SystemService } from "@mavio/proto/mavio/system/v1/system_pb";
import { type JobState, TaskService } from "@mavio/proto/mavio/system/v1/task_pb";
import {
  type CreateInvitationRequestSchema,
  type CreateUserRequestSchema,
  type UpdateUserRequestSchema,
  UserService,
} from "@mavio/proto/mavio/user/v1/user_pb";
import type { MessageInitShape } from "@bufbuild/protobuf";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/** A server's libraries, those users curate included. */
export function useServerLibraries(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "libraries"),
    queryFn: async () => (await servers.client(LibraryService, server).listLibraries({})).libraries,
  });
}

/** What can be done to a server's libraries, each refreshing them when done. */
export function useLibraryActions(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const onSettled = async () => {
    await qc.invalidateQueries({ queryKey: serverKey(server.id, "libraries") });
    await qc.invalidateQueries({ queryKey: serverKey(server.id, "library") });
  };
  const client = () => servers.client(LibraryService, server);
  return {
    create: useMutation({ mutationFn: async (spec: LibrarySpec) => (await client().createLibrary({ spec })).library, onSettled }),
    update: useMutation({
      mutationFn: async ({ id, spec }: { id: string; spec: LibrarySpec }) => (await client().updateLibrary({ id, spec })).library,
      onSettled,
    }),
    remove: useMutation({ mutationFn: (id: string) => client().deleteLibrary({ id }), onSettled }),
    /** Enqueues a scan; enqueued is false when one was already pending. */
    scan: useMutation({ mutationFn: (id: string) => client().scanLibrary({ id }) }),
  };
}

/** A folder of the server's file system and the folders in it; the roots when path is empty. */
export function useDirectory(server: ServerEntry, path: string, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "directory", path),
    queryFn: () => servers.client(SystemService, server).listDirectory({ path }),
    enabled,
    retry: false,
  });
}

/** The languages and countries a server knows, for metadata. */
export function useLocalization(server: ServerEntry) {
  const servers = useServersStore();
  const client = () => servers.client(LocalizationService, server);
  const languages = useQuery({
    queryKey: serverKey(server.id, "localization", "languages"),
    queryFn: async () => (await client().listLanguages({})).languages,
    staleTime: Infinity,
  });
  const countries = useQuery({
    queryKey: serverKey(server.id, "localization", "countries"),
    queryFn: async () => (await client().listCountries({})).countries,
    staleTime: Infinity,
  });
  return { languages: languages.data ?? [], countries: countries.data ?? [] };
}

/** The content ratings of a country, for limiting what users see; the server's country when empty. */
export function useRatings(server: ServerEntry, country = "") {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "localization", "ratings", country),
    queryFn: async () => (await servers.client(LocalizationService, server).listRatings({ country })).ratings,
    staleTime: Infinity,
  });
}

/** A server's users. */
export function useUsers(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "users"),
    queryFn: async () => (await servers.client(UserService, server).listUsers({})).users,
  });
}

/** What an administrator can do to a server's users, each refreshing them when done. */
export function useUserActions(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const onSettled = () => qc.invalidateQueries({ queryKey: serverKey(server.id, "users") });
  const client = () => servers.client(UserService, server);
  return {
    create: useMutation({ mutationFn: async (req: MessageInitShape<typeof CreateUserRequestSchema>) => (await client().createUser(req)).user, onSettled }),
    update: useMutation({ mutationFn: async (req: MessageInitShape<typeof UpdateUserRequestSchema>) => (await client().updateUser(req)).user, onSettled }),
    remove: useMutation({ mutationFn: (id: string) => client().deleteUser({ id }), onSettled }),
    /** Sets a user's password, which makes a managed user a member. */
    setPassword: useMutation({
      mutationFn: ({ id, password }: { id: string; password: string }) => client().setPassword({ userId: id, newPassword: password }),
      onSettled,
    }),
    /** Sets a managed user's PIN, or clears it when empty. */
    setPin: useMutation({ mutationFn: ({ id, pin }: { id: string; pin: string }) => client().setPin({ userId: id, pin }), onSettled }),
    deleteTotp: useMutation({ mutationFn: (id: string) => client().deleteTotp({ userId: id }), onSettled }),
    deletePasskey: useMutation({
      mutationFn: ({ id, passkeyId }: { id: string; passkeyId: string }) => client().deletePasskey({ userId: id, id: passkeyId }),
      onSettled,
    }),
  };
}

/** A server's invitations that have not expired, newest first. */
export function useInvitations(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "invitations"),
    queryFn: async () => (await servers.client(UserService, server).listInvitations({})).invitations,
  });
}

/** Making and revoking a server's invitations. */
export function useInvitationActions(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const onSettled = () => qc.invalidateQueries({ queryKey: serverKey(server.id, "invitations") });
  const client = () => servers.client(UserService, server);
  return {
    /** Makes an invitation; its token comes back only here. */
    create: useMutation({ mutationFn: (req: MessageInitShape<typeof CreateInvitationRequestSchema>) => client().createInvitation(req), onSettled }),
    remove: useMutation({ mutationFn: (id: string) => client().deleteInvitation({ id }), onSettled }),
  };
}

/** The link of an invitation: the web UI of the server, with the token in the fragment so that it is never sent. */
export function invitationLink(server: ServerEntry, token: string): string {
  return `${server.url.replace(/\/$/, "")}/invite#${encodeURIComponent(token)}`;
}

/** What the server runs on and since when, and whether it serves. */
export function useSystemInfo(server: ServerEntry) {
  const servers = useServersStore();
  const client = () => servers.client(SystemService, server);
  const info = useQuery({ queryKey: serverKey(server.id, "system", "info"), queryFn: () => client().getSystemInfo({}) });
  const health = useQuery({
    queryKey: serverKey(server.id, "system", "health"),
    queryFn: () => client().getHealth({}),
    refetchInterval: 30_000,
  });
  return { info, health };
}

/** Every user's sessions, those playing first, refreshed every few seconds. */
export function useAllSessions(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "sessions", "all"),
    queryFn: async () => (await servers.client(SessionService, server).listSessions({ allUsers: true })).sessions,
    refetchInterval: 5_000,
  });
}

/** The server's latest activities, newest first, those less severe than minSeverity left out. */
export function useActivities(server: ServerEntry, minSeverity: Severity, limit: number) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "activities", minSeverity, limit),
    queryFn: () => servers.client(ActivityService, server).listActivities({ minSeverity, limit }),
    refetchInterval: 30_000,
  });
}

/** The server's recurring tasks, refreshed often while one runs. */
export function useTasks(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "tasks"),
    queryFn: async () => (await servers.client(TaskService, server).listTasks({})).tasks,
    refetchInterval: (q) => (q.state.data?.some((t) => t.running) ? 2_000 : 15_000),
  });
}

/** Runs a task now; enqueued is false when a run was already queued. */
export function useRunTask(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => servers.client(TaskService, server).runTask({ id }),
    onSettled: async () => {
      await qc.invalidateQueries({ queryKey: serverKey(server.id, "tasks") });
      await qc.invalidateQueries({ queryKey: serverKey(server.id, "jobs") });
    },
  });
}

/** The jobs of the server's queue in the states given, or all, most recently created first. */
export function useJobs(server: ServerEntry, states: JobState[], limit: number) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "jobs", states, limit),
    queryFn: () => servers.client(TaskService, server).listJobs({ states, limit }),
    refetchInterval: 5_000,
  });
}

/** A server's settings, with the hardware accelerations its ffmpeg supports. */
export function useServerSettings(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "settings"),
    queryFn: () => servers.client(SystemService, server).getServerSettings({}),
  });
}

/**
 * Changes a server's settings: change is applied to the settings the server
 * has now, so that what other pages change in the meantime is kept.
 */
export function useUpdateServerSettings(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (change: (s: ServerSettings) => void) => {
      const client = servers.client(SystemService, server);
      const { settings } = await client.getServerSettings({});
      if (!settings) throw new Error("the server sent no settings");
      change(settings);
      return (await client.updateServerSettings({ settings })).settings;
    },
    onSettled: async () => {
      await qc.invalidateQueries({ queryKey: serverKey(server.id, "settings") });
      await qc.invalidateQueries({ queryKey: serverKey(server.id, "plugins", "catalogs") });
    },
  });
}

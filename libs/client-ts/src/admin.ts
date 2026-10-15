// Server administration (docs/clients.md §6.3.4, §6.3.5): a server's
// libraries, with the folders of its file system to choose from, the
// languages, countries and ratings it knows, and its users and
// invitations.
import { LibraryService, type LibrarySpec } from "@mavio/proto/mavio/library/v1/library_pb";
import { LocalizationService } from "@mavio/proto/mavio/system/v1/localization_pb";
import { SystemService } from "@mavio/proto/mavio/system/v1/system_pb";
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

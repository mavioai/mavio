// Server administration (docs/clients.md §6.3.4): a server's libraries,
// with the folders of its file system to choose from, and the languages
// and countries it knows.
import { LibraryService, type LibrarySpec } from "@mavio/proto/mavio/library/v1/library_pb";
import { LocalizationService } from "@mavio/proto/mavio/system/v1/localization_pb";
import { SystemService } from "@mavio/proto/mavio/system/v1/system_pb";
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

// Plugin management (docs/clients.md §6.3.3): the servers the user
// administers, their plugins and those of their catalogs, installing,
// updating, uninstalling, enabling and disabling them, and the catalogs.
import { UserService } from "@mavio/proto/mavio/user/v1/user_pb";
import { SystemService } from "@mavio/proto/mavio/system/v1/system_pb";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore, useSignedIn } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/** The signed-in servers on which the user is an administrator, in the user's order. */
export function useAdminServers(): ServerEntry[] {
  const servers = useServersStore();
  const list = useSignedIn();
  return useQueries({
    queries: list.map((s) => ({
      queryKey: serverKey(s.id, "currentUser", s.userId),
      queryFn: async () => (await servers.client(UserService, s).getCurrentUser({})).user ?? null,
      staleTime: 5 * 60_000,
      retry: false,
    })),
    combine: (results) => list.filter((_, i) => results[i]?.data?.admin === true),
  });
}

/** The plugins of a server's plugin folder. */
export function usePlugins(server: ServerEntry | undefined) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server?.id ?? "", "plugins", "installed"),
    queryFn: async () => (await servers.client(SystemService, server!).listPlugins({})).plugins,
    enabled: !!server,
  });
}

/** The plugins of a server's catalogs, with the versions it can run. */
export function useCatalogPlugins(server: ServerEntry | undefined) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server?.id ?? "", "plugins", "catalog"),
    queryFn: async () => (await servers.client(SystemService, server!).listCatalogPlugins({})).plugins,
    enabled: !!server,
    retry: false,
  });
}

/** The URLs of a server's plugin catalogs. */
export function usePluginCatalogs(server: ServerEntry | undefined) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server?.id ?? "", "plugins", "catalogs"),
    queryFn: async () => (await servers.client(SystemService, server!).getServerSettings({})).settings?.pluginCatalogs ?? [],
    enabled: !!server,
  });
}

/** What can be done to a server's plugins, each refreshing them when done. */
export function usePluginActions(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const onSettled = () => qc.invalidateQueries({ queryKey: serverKey(server.id, "plugins") });
  const client = () => servers.client(SystemService, server);
  return {
    /** Installs a plugin, or changes its version; the newest when version is unset. */
    install: useMutation({ mutationFn: ({ id, version }: { id: string; version?: string }) => client().installPlugin({ pluginId: id, version }), onSettled }),
    uninstall: useMutation({ mutationFn: (id: string) => client().uninstallPlugin({ pluginId: id }), onSettled }),
    setEnabled: useMutation({
      mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => client().setPluginEnabled({ pluginId: id, enabled }),
      onSettled,
    }),
    /** Replaces the catalogs' URLs, keeping the other settings. */
    setCatalogs: useMutation({
      mutationFn: async (urls: string[]) => {
        const { settings } = await client().getServerSettings({});
        if (!settings) throw new Error("the server sent no settings");
        settings.pluginCatalogs = urls;
        return client().updateServerSettings({ settings });
      },
      onSettled,
    }),
  };
}

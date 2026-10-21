// Plugin management (docs/clients.md §6.3.3): the servers the user
// administers, their plugins and those of their catalogs, installing,
// updating, uninstalling, enabling and disabling them, and the catalogs.
import type { ContainerMode } from "@mavio/proto/mavio/plugin/v1/container_pb";
import { UserService } from "@mavio/proto/mavio/user/v1/user_pb";
import { type PluginConfigOption, PluginManagementService } from "@mavio/proto/mavio/system/v1/plugin_pb";
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

/**
 * The plugins of a server's plugin folder, and those under development,
 * which are fetched again every few seconds while there are any, since the
 * server restarts them when they are built.
 */
export function usePlugins(server: ServerEntry | undefined) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server?.id ?? "", "plugins", "installed"),
    queryFn: async () => (await servers.client(PluginManagementService, server!).listPlugins({})).plugins,
    enabled: !!server,
    refetchInterval: (q) => (q.state.data?.some((p) => p.devFolder) ? 2000 : false),
  });
}

/** The server's recent log records of a plugin, newest first, fetched again every few seconds. */
export function usePluginLogs(server: ServerEntry, id: string) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "logs", "plugin", id),
    queryFn: async () => (await servers.client(SystemService, server).listLogs({ pluginId: id, limit: 200 })).records,
    refetchInterval: 3000,
  });
}

/** The plugins of a server's catalogs, with the versions it can run. */
export function useCatalogPlugins(server: ServerEntry | undefined) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server?.id ?? "", "plugins", "catalog"),
    queryFn: async () => (await servers.client(PluginManagementService, server!).listCatalogPlugins({})).plugins,
    enabled: !!server,
    retry: false,
  });
}

/** The URLs of a server's plugin catalogs. */
export function usePluginCatalogs(server: ServerEntry | undefined) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server?.id ?? "", "plugins", "catalogs"),
    queryFn: async () => (await servers.client(PluginManagementService, server!).getPluginCatalogs({})).urls,
    enabled: !!server,
  });
}

/** A plugin's configuration as JSON, empty when never configured. */
export function usePluginConfig(server: ServerEntry, id: string, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "plugins", "config", id),
    queryFn: async () => (await servers.client(PluginManagementService, server).getPluginConfig({ pluginId: id })).configJson,
    enabled,
  });
}

/** Every plugin's containers the server runs or was given an address of, by plugin. */
export function useContainers(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "plugins", "containers"),
    queryFn: async () => (await servers.client(PluginManagementService, server).listContainers({})).containers,
    refetchInterval: 5000,
  });
}

/** The container engine the server found, or why it found none. */
export function useContainerRuntime(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "plugins", "runtime"),
    queryFn: async () => (await servers.client(PluginManagementService, server).getContainerRuntime({})).runtime,
  });
}

/** A plugin's container dependencies, with their states and the commands running them. */
export function usePluginContainers(server: ServerEntry, id: string, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "plugins", "containers", id),
    queryFn: async () => (await servers.client(PluginManagementService, server).listPluginContainers({ pluginId: id })).containers,
    enabled,
    // Managed dependencies pull and start in the background.
    refetchInterval: 5000,
  });
}

/**
 * The options of the sources a plugin's configuration schema names
 * (x-mavio-options), by source: undefined while they load or when they
 * cannot be had.
 */
export function usePluginConfigOptions(server: ServerEntry, id: string, sources: string[]): (source: string) => PluginConfigOption[] | undefined {
  const servers = useServersStore();
  const options = useQueries({
    queries: sources.map((source) => ({
      queryKey: serverKey(server.id, "plugins", "options", id, source),
      queryFn: async () => (await servers.client(PluginManagementService, server).listPluginConfigOptions({ pluginId: id, source })).options,
      retry: false,
    })),
    combine: (results) => new Map(sources.map((s, i) => [s, results[i]?.data])),
  });
  return (source) => options.get(source);
}

/** What can be done to a server's plugins, each refreshing them when done. */
export function usePluginActions(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const onSettled = () => qc.invalidateQueries({ queryKey: serverKey(server.id, "plugins") });
  const client = () => servers.client(PluginManagementService, server);
  return {
    /** Installs a plugin, or changes its version; the newest when version is unset. */
    install: useMutation({ mutationFn: ({ id, version }: { id: string; version?: string }) => client().installPlugin({ pluginId: id, version }), onSettled }),
    uninstall: useMutation({ mutationFn: (id: string) => client().uninstallPlugin({ pluginId: id }), onSettled }),
    setEnabled: useMutation({
      mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => client().setPluginEnabled({ pluginId: id, enabled }),
      onSettled,
    }),
    /** Stops a plugin and starts it again, as after an upgrade; one under development with what was built. */
    restart: useMutation({ mutationFn: (id: string) => client().restartPlugin({ pluginId: id }), onSettled }),
    /** Sets a plugin's configuration, JSON its schema accepts; the response holds the configuration stored. */
    configure: useMutation({
      mutationFn: ({ id, config }: { id: string; config: string }) => client().setPluginConfig({ pluginId: id, configJson: config }),
      onSettled,
    }),
    /** Sets how a container dependency is reached. */
    setContainer: useMutation({
      mutationFn: (req: { pluginId: string; name: string; mode: ContainerMode; address?: string; useGpu?: boolean; proxy?: string }) =>
        client().setPluginContainer(req),
      onSettled,
    }),
    /** Sets the proxy a plugin reaches sites through: a proxy's ID, "system", or "" for none. */
    setProxy: useMutation({
      mutationFn: ({ id, proxy }: { id: string; proxy: string }) => client().setPluginProxy({ pluginId: id, proxy }),
      onSettled,
    }),
    /** Replaces the catalogs' URLs. */
    setCatalogs: useMutation({
      mutationFn: (urls: string[]) => client().updatePluginCatalogs({ urls }),
      onSettled,
    }),
  };
}

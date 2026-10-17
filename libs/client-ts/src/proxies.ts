// The server's proxies (docs/clients.md §6.3.30): listing them, the
// system proxy first, adding, changing, deleting and testing them.
import { type Proxy, ProxyService } from "@mavio/proto/mavio/system/v1/proxy_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/** The system proxy's ID. */
export const systemProxyId = "system";

/**
 * The server's proxies, the system proxy first; adding, changing and
 * deleting one, which plugins offering them see at once.
 */
export function useProxies(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const queryKey = serverKey(server.id, "proxies");
  const c = () => servers.client(ProxyService, server);
  const onSuccess = () =>
    Promise.all([qc.invalidateQueries({ queryKey }), qc.invalidateQueries({ queryKey: serverKey(server.id, "plugins", "options") })]);
  return {
    list: useQuery({ queryKey, queryFn: async () => (await c().listProxies({})).proxies }),
    create: useMutation({ mutationFn: async (proxy: Proxy) => (await c().createProxy({ proxy })).proxy, onSuccess }),
    update: useMutation({
      mutationFn: async ({ proxy, clearPassword = false }: { proxy: Proxy; clearPassword?: boolean }) => (await c().updateProxy({ proxy, clearPassword })).proxy,
      onSuccess,
    }),
    remove: useMutation({ mutationFn: (id: string) => c().deleteProxy({ id }), onSuccess }),
  };
}

/**
 * Testing a proxy, a stored one by its ID or one being edited, which
 * keeps the stored password when it gives none.
 */
export function useTestProxy(server: ServerEntry) {
  const servers = useServersStore();
  return useMutation({
    mutationFn: (target: { id: string } | { proxy: Proxy }) =>
      servers.client(ProxyService, server).testProxy({ target: "id" in target ? { case: "id", value: target.id } : { case: "proxy", value: target.proxy } }),
  });
}

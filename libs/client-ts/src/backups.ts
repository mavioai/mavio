// A server's backups, log and API keys (docs/clients.md §6.3.14–§6.3.15):
// making, listing, downloading and deleting backups, the recent log
// records, and the keys integrations call the API with.
import { create } from "@bufbuild/protobuf";
import { DurationSchema } from "@bufbuild/protobuf/wkt";
import { AuthService } from "@mavio/proto/mavio/auth/v1/auth_pb";
import { BackupService } from "@mavio/proto/mavio/system/v1/backup_pb";
import { type LogLevel, SystemService } from "@mavio/proto/mavio/system/v1/system_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import { urlOf } from "./server.ts";
import type { ServerEntry } from "./servers.ts";

/** The backups, newest first; making one and deleting one. */
export function useBackups(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const queryKey = serverKey(server.id, "backups");
  const c = () => servers.client(BackupService, server);
  const onSuccess = () => qc.invalidateQueries({ queryKey });
  return {
    list: useQuery({ queryKey, queryFn: async () => (await c().listBackups({})).backups }),
    create: useMutation({ mutationFn: async () => (await c().createBackup({})).backup, onSuccess }),
    remove: useMutation({ mutationFn: (name: string) => c().deleteBackup({ name }), onSuccess }),
  };
}

/**
 * Downloading a backup's file. It is fetched with the session's access
 * token, which a link cannot send, so the whole file is read before it is
 * saved.
 */
export function useBackupDownload(server: ServerEntry) {
  const servers = useServersStore();
  return useMutation({
    mutationFn: async (name: string) => {
      const token = await servers.token(server.id);
      const res = await fetch(urlOf(server, `system/backups/${encodeURIComponent(name)}`), {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
      return await res.blob();
    },
  });
}

/** The server's recent log records at a level or above, newest first, from the last hours when given. */
export function useLogs(server: ServerEntry, minLevel: LogLevel, hours = 0, limit = 1000) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "logs", minLevel, hours, limit),
    queryFn: async () =>
      (
        await servers.client(SystemService, server).listLogs({
          minLevel,
          limit,
          maxAge: hours > 0 ? create(DurationSchema, { seconds: BigInt(hours * 3600) }) : undefined,
        })
      ).records,
  });
}

/** The server's API keys; making one, whose token is returned once, and revoking one. */
export function useApiKeys(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const queryKey = serverKey(server.id, "api-keys");
  const c = () => servers.client(AuthService, server);
  const onSuccess = () => qc.invalidateQueries({ queryKey });
  return {
    list: useQuery({ queryKey, queryFn: async () => (await c().listApiKeys({})).apiKeys }),
    create: useMutation({ mutationFn: (name: string) => c().createApiKey({ name }), onSuccess }),
    revoke: useMutation({ mutationFn: (id: string) => c().revokeApiKey({ id }), onSuccess }),
  };
}

// Remote control (docs/clients.md §6.3.23): the user's devices with what
// they play, and the commands sent to them through their event streams.
import type { MessageInitShape } from "@bufbuild/protobuf";
import { type CommandSchema, SessionService } from "@mavio/proto/mavio/session/v1/session_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/** The user's devices on a server with what they play, online ones first; refreshed as they change. */
export function useSessions(server: ServerEntry, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "sessions", "remote"),
    queryFn: async () => (await servers.client(SessionService, server).listSessions({})).sessions,
    enabled,
  });
}

/** Signs one of the user's devices out, refreshing the devices when done. */
export function useRevokeSession(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => servers.client(SessionService, server).revokeSession({ id }),
    onSettled: () => qc.invalidateQueries({ queryKey: serverKey(server.id, "sessions") }),
  });
}

/** Sends a command to a device. */
export function useSendCommand(server: ServerEntry) {
  const servers = useServersStore();
  return useMutation({
    mutationFn: (v: { sessionId: string; command: MessageInitShape<typeof CommandSchema> }) => servers.client(SessionService, server).sendCommand(v),
  });
}

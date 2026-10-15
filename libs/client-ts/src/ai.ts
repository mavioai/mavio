// The AI gateway of a server (docs/clients.md §6.3.13): its providers and
// their models.
import { Code, ConnectError } from "@connectrpc/connect";
import { AiService } from "@mavio/proto/mavio/system/v1/ai_pb";
import { useQuery } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/**
 * The models of the AI providers; with available, only those of providers
 * signed in to. None when no AI gateway plugin runs.
 */
export function useAiModels(server: ServerEntry, available: boolean) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "ai", "models", available),
    queryFn: async () => {
      try {
        return (await servers.client(AiService, server).listAiModels({ available })).models;
      } catch (e) {
        if (ConnectError.from(e).code === Code.FailedPrecondition) return [];
        throw e;
      }
    },
  });
}

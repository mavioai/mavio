// The AI gateway of a server (docs/clients.md §6.3.13): its providers,
// their credentials — API keys, and OAuth logins followed as they run —
// and their models.
import { Code, ConnectError } from "@connectrpc/connect";
import { AiService, type WatchAiLoginResponse } from "@mavio/proto/mavio/system/v1/ai_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/** Whether an error says that no AI gateway plugin runs. */
export function isNoAiGateway(err: unknown): boolean {
  return err instanceof ConnectError && err.code === Code.FailedPrecondition;
}

/** The AI providers, with how they authenticate and whether they can be used. */
export function useAiProviders(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "ai", "providers"),
    queryFn: async () => (await servers.client(AiService, server).listAiProviders({})).providers,
    retry: (count, err) => !isNoAiGateway(err) && count < 3,
  });
}

/**
 * The models of the AI providers, or of one; with available, only those
 * of providers that can be used. None when no AI gateway plugin runs.
 */
export function useAiModels(server: ServerEntry, available: boolean, provider = "") {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "ai", "models", available, provider),
    queryFn: async () => {
      try {
        return (await servers.client(AiService, server).listAiModels({ available, provider })).models;
      } catch (e) {
        if (isNoAiGateway(e)) return [];
        throw e;
      }
    },
  });
}

/** What can be done to the providers' credentials, each refreshing the providers and models when done. */
export function useAiActions(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const c = () => servers.client(AiService, server);
  const onSuccess = () => qc.invalidateQueries({ queryKey: serverKey(server.id, "ai") });
  return {
    setApiKey: useMutation({ mutationFn: (v: { provider: string; key: string }) => c().setAiApiKey(v), onSuccess }),
    logout: useMutation({ mutationFn: (provider: string) => c().logoutAi({ provider }), onSuccess }),
    startLogin: useMutation({ mutationFn: async (provider: string) => (await c().startAiLogin({ provider })).loginId }),
    answer: useMutation({ mutationFn: (v: { loginId: string; promptId: string; value: string }) => c().answerAiLogin(v) }),
    cancel: useMutation({ mutationFn: (loginId: string) => c().cancelAiLogin({ loginId }) }),
  };
}

/**
 * A login's state as it runs, followed until it ends or the component
 * unmounts; the providers and models are refreshed when it ends.
 */
export function useAiLogin(server: ServerEntry, loginId: string | undefined) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const [state, setState] = useState<{ login?: WatchAiLoginResponse; error?: unknown }>({});
  useEffect(() => {
    setState({});
    if (!loginId) return;
    const abort = new AbortController();
    void (async () => {
      try {
        for await (const login of servers.client(AiService, server).watchAiLogin({ loginId }, { signal: abort.signal })) setState({ login });
      } catch (error) {
        if (!abort.signal.aborted) setState((s) => ({ ...s, error }));
      }
      if (!abort.signal.aborted) void qc.invalidateQueries({ queryKey: serverKey(server.id, "ai") });
    })();
    return () => abort.abort();
    // The stream restarts only for another login or server.
  }, [servers, server.id, loginId, qc]);
  return state;
}

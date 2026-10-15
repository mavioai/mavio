// The AI gateway of a server (docs/clients.md §6.3.13): its providers,
// their credentials — API keys, and OAuth logins followed as they run —
// and their models; and the administrators' conversations with the
// models (§6.3.26), their answers streamed as the model writes them.
import { Code, ConnectError } from "@connectrpc/connect";
import { AiService, type WatchAiLoginResponse } from "@mavio/proto/mavio/system/v1/ai_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";

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

/** The conversations with the models, newest first. */
export function useAiConversations(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "ai", "conversations", "list"),
    queryFn: async () => (await servers.client(AiService, server).listAiConversations({})).conversations,
    retry: (count, err) => !isNoAiGateway(err) && count < 3,
  });
}

/** A conversation with its messages. */
export function useAiConversation(server: ServerEntry, id: string) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "ai", "conversations", "one", id),
    queryFn: async () => servers.client(AiService, server).getAiConversation({ id }),
  });
}

/** Starting and deleting conversations, each refreshing them. */
export function useAiConversationActions(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const c = () => servers.client(AiService, server);
  const list = () => qc.invalidateQueries({ queryKey: serverKey(server.id, "ai", "conversations", "list") });
  return {
    create: useMutation({
      mutationFn: async (v: { provider: string; model: string; instructions: string; title: string }) => (await c().createAiConversation(v)).conversation!,
      onSuccess: list,
    }),
    // The deleted conversation goes from the cache rather than being fetched again.
    remove: useMutation({
      mutationFn: (id: string) => c().deleteAiConversation({ id }),
      onSuccess: (_, id) => {
        qc.removeQueries({ queryKey: serverKey(server.id, "ai", "conversations", "one", id) });
        return list();
      },
    }),
  };
}

/** A message being answered: its text and the answer so far. */
export interface AiExchange {
  text: string;
  answer: string;
  error?: unknown;
}

/**
 * Sends messages to a conversation, with the one being answered and its
 * answer as the model writes it; the conversation is refreshed with both
 * once answered, or with what was kept of them once failed. A failed
 * message keeps its error until it is sent again, as a new message, or
 * another is sent.
 */
export function useAiSend(server: ServerEntry, id: string) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const [exchange, setExchange] = useState<AiExchange>();
  const current = useRef("");
  const send = useCallback(
    async (text: string) => {
      const requestId = crypto.randomUUID();
      current.current = requestId;
      const mine = () => current.current === requestId;
      const refresh = () => qc.invalidateQueries({ queryKey: serverKey(server.id, "ai", "conversations") });
      setExchange({ text, answer: "" });
      try {
        for await (const m of servers.client(AiService, server).sendAiMessage({ conversationId: id, text, requestId }))
          if (m.event.case === "text" && mine()) setExchange({ text, answer: m.event.value });
        await refresh();
        if (mine()) setExchange(undefined);
      } catch (error) {
        await refresh();
        if (mine()) setExchange({ text, answer: "", error });
      }
    },
    [servers, server, id, qc],
  );
  const retry = useCallback(() => exchange && send(exchange.text), [send, exchange]);
  return { exchange, send, retry, sending: !!exchange && exchange.error === undefined };
}

// The assistant (docs/assistant.md): what a server's assistant can do, its
// requests with their events as they stream, listening through the
// server's recognizer and speaking with its voices; the user's
// conversations and memories, and the server's assistant settings.
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import {
  type AskRequest,
  AskRequestSchema,
  type AskResponse,
  type AssistantSettings,
  AssistantService,
} from "@mavio/proto/mavio/assistant/v1/assistant_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import { urlOf } from "./server.ts";
import type { ServerEntry, Servers } from "./servers.ts";

/**
 * What a server's assistant can do for the user, in a language: whether it
 * is on, the AI path, memory, the recognizers and voices, and examples.
 */
export function useAssistant(server: ServerEntry | undefined, language: string) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server?.id ?? "", "assistant", language),
    queryFn: () => servers.client(AssistantService, server!).getAssistant({ language }),
    enabled: !!server,
    staleTime: 5 * 60_000,
  });
}

/** Sends a request to the assistant, whose events stream until it ends or signal aborts. */
export function ask(
  servers: Servers,
  server: ServerEntry,
  req: MessageInitShape<typeof AskRequestSchema> | AskRequest,
  signal?: AbortSignal,
): AsyncIterable<AskResponse> {
  return servers.client(AssistantService, server).ask(create(AskRequestSchema, req), { signal });
}

/**
 * Starts listening through the server's recognizer (§8.2): the WebSocket
 * URL to send 16 kHz mono 16-bit PCM to, signed for a minute, and the
 * recognizer chosen.
 */
export async function startListening(
  servers: Servers,
  server: ServerEntry,
  language: string,
  recognizer = "",
): Promise<{ url: string; recognizer: string }> {
  const r = await servers.client(AssistantService, server).startListening({ language, recognizer });
  return { url: urlOf(server, r.url).replace(/^http/, "ws"), recognizer: r.recognizer };
}

/** What the listening route sends back. */
export interface Heard {
  text?: string;
  language?: string;
  /** The utterance ended: nothing more is sent. */
  final?: boolean;
  error?: string;
}

/**
 * Speaks a text with a server voice (§8.3): its audio, Opus in Ogg unless
 * MP3 is asked for. An empty voice is the settings' for the language.
 */
export async function speakWithServer(
  servers: Servers,
  server: ServerEntry,
  speech: { text: string; language: string; voice?: string; format?: "opus" | "mp3" },
  signal?: AbortSignal,
): Promise<Blob> {
  const token = await servers.token(server.id);
  const res = await fetch(urlOf(server, "assistant/speech"), {
    method: "POST",
    headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify({ voice: "", format: "opus", ...speech }),
    signal,
  });
  if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
  return await res.blob();
}

/** The user's conversations with the assistant, latest first. */
export function useAssistantConversations(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "assistant", "conversations"),
    queryFn: async () => (await servers.client(AssistantService, server).listAssistantConversations({})).conversations,
  });
}

/** A conversation's messages. */
export function useAssistantConversation(server: ServerEntry, id: string | undefined) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "assistant", "conversations", id),
    queryFn: () => servers.client(AssistantService, server).getAssistantConversation({ id: id! }),
    enabled: !!id,
  });
}

/** The user's memories. */
export function useAssistantMemories(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "assistant", "memories"),
    queryFn: async () => (await servers.client(AssistantService, server).listAssistantMemories({})).memories,
  });
}

/** What can be done to conversations and memories, each refreshing them when done. */
export function useAssistantActions(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  const c = () => servers.client(AssistantService, server);
  const onSuccess = () => qc.invalidateQueries({ queryKey: serverKey(server.id, "assistant") });
  return {
    deleteConversation: useMutation({ mutationFn: (id: string) => c().deleteAssistantConversation({ id }), onSuccess }),
    /** Adds a memory with an empty ID, else edits it. */
    updateMemory: useMutation({ mutationFn: (v: { id: string; text: string }) => c().updateAssistantMemory(v), onSuccess }),
    /** Deletes memories; none deletes every one. */
    deleteMemories: useMutation({ mutationFn: (ids: string[]) => c().deleteAssistantMemories({ ids }), onSuccess }),
  };
}

/** The server's assistant settings, for administrators. */
export function useAssistantSettings(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "assistant", "settings"),
    queryFn: async () => (await servers.client(AssistantService, server).getAssistantSettings({})).settings,
  });
}

/** Saves the server's assistant settings. */
export function useUpdateAssistantSettings(server: ServerEntry) {
  const servers = useServersStore();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (settings: AssistantSettings) => servers.client(AssistantService, server).updateAssistantSettings({ settings }),
    onSuccess: () => qc.invalidateQueries({ queryKey: serverKey(server.id, "assistant") }),
  });
}

// Where a sentence ends: its punctuation, then a space or the end of the
// text; CJK's full stops need no space.
const sentenceEnd = /[.!?…](?=\s)|[。！？]/g;

/**
 * Cuts an answer into sentences as its text arrives, so that speaking
 * starts before it ends (§8.3): push gives the sentences the text added
 * completes, and end the rest.
 */
export class Sentences {
  private text = "";

  push(more: string): string[] {
    this.text += more;
    const out: string[] = [];
    let start = 0;
    for (const m of this.text.matchAll(sentenceEnd)) {
      const end = m.index + m[0].length;
      const s = this.text.slice(start, end).trim();
      if (s) out.push(s);
      start = end;
    }
    this.text = this.text.slice(start);
    return out;
  }

  end(): string[] {
    const s = this.text.trim();
    this.text = "";
    return s ? [s] : [];
  }
}

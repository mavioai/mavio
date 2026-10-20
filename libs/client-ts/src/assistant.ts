// The assistant (docs/assistant.md): what a server's assistant can do, its
// requests with their events as they stream, voice sessions and the speech
// kept of their answers; the user's conversations and memories, and the
// server's assistant settings.
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import {
  type AskRequest,
  AskRequestSchema,
  type AskResponse,
  type AssistantSettings,
  AssistantService,
  type StartVoiceSessionRequestSchema,
} from "@mavio/proto/mavio/assistant/v1/assistant_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import { urlOf } from "./server.ts";
import type { ServerEntry, Servers } from "./servers.ts";

/**
 * What a server's assistant can do for the user, in a language: whether it
 * is on, the AI path, memory, voice sessions, and examples.
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
 * Starts a voice session (§8.1): the WebSocket URL to connect to within a
 * minute, and the session's ID.
 */
export async function startVoiceSession(
  servers: Servers,
  server: ServerEntry,
  req: MessageInitShape<typeof StartVoiceSessionRequestSchema>,
): Promise<{ url: string; sessionId: string }> {
  const r = await servers.client(AssistantService, server).startVoiceSession(req);
  return { url: urlOf(server, r.url).replace(/^http/, "ws"), sessionId: r.sessionId };
}

const speechTypes: [format: string, type: string][] = [
  ["opus", 'audio/ogg; codecs="opus"'],
  ["aac", "audio/aac"],
  ["mp3", "audio/mpeg"],
  ["flac", "audio/flac"],
  ["wav", "audio/wav"],
];

/**
 * The encodings of speech this browser plays (§8.8), which the server
 * converts speech in another to; none outside a browser, which the server
 * takes for playing anything.
 */
export function speechFormats(): string[] {
  if (typeof document === "undefined") return [];
  const audio = document.createElement("audio");
  return speechTypes.filter(([, type]) => audio.canPlayType(type) !== "").map(([format]) => format);
}

/**
 * An answer's kept speech (§8.8), from AssistantMessage.speech_url, in an
 * encoding of the formats played, this browser's unless given.
 */
export async function keptSpeech(
  servers: Servers,
  server: ServerEntry,
  url: string,
  signal?: AbortSignal,
  formats: string[] = speechFormats(),
): Promise<Blob> {
  const token = await servers.token(server.id);
  const query = formats.length ? `${url.includes("?") ? "&" : "?"}formats=${formats.join(",")}` : "";
  const res = await fetch(urlOf(server, url + query), { headers: token ? { Authorization: `Bearer ${token}` } : {}, signal });
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

/** The server's assistant settings, for administrators, with the recognizers and voices to choose from. */
export function useAssistantSettings(server: ServerEntry) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "assistant", "settings"),
    queryFn: () => servers.client(AssistantService, server).getAssistantSettings({}),
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

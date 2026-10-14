// The event stream of each server (docs/clients.md §3): changes refresh the
// server's queries, so that the screens never poll. Streams reconnect with
// backoff and end when the user signs out of their server.
import { Code, ConnectError } from "@connectrpc/connect";
import { type Event, EventService } from "@mavio/proto/mavio/session/v1/session_pb";
import { type QueryClient, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { serverKey, useServersStore, useSignedIn } from "./hooks.ts";
import type { ServerEntry, Servers } from "./servers.ts";

/** The delay before the attempt-th reconnection: doubling from 1 s to 30 s, with jitter. */
export function backoff(attempt: number, random = Math.random): number {
  const base = Math.min(30_000, 1000 * 2 ** attempt);
  return base / 2 + random() * (base / 2);
}

/**
 * Whether an event refreshes its server's queries: library and user data
 * changes touch items in every list of the server, and a new stream may
 * follow missed events.
 */
export function refreshes(event: Event): boolean {
  const k = event.kind.case;
  return k === "connected" || k === "libraryChanged" || k === "userDataChanged";
}

function sleep(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve) => {
    const t = setTimeout(resolve, ms);
    signal.addEventListener("abort", () => (clearTimeout(t), resolve()), { once: true });
  });
}

/** Follows a server's events until signal aborts or the server ends the session. */
export async function follow(
  servers: Servers,
  server: ServerEntry,
  onEvent: (event: Event) => void,
  signal: AbortSignal,
) {
  let attempt = 0;
  let first = true;
  while (!signal.aborted) {
    try {
      for await (const r of servers.client(EventService, server).subscribe({}, { signal })) {
        if (!r.event) continue;
        attempt = 0;
        // The first stream's Connected repeats what the screens just loaded.
        if (r.event.kind.case === "connected" && first) {
          first = false;
          continue;
        }
        first = false;
        onEvent(r.event);
      }
    } catch (err) {
      if (signal.aborted) return;
      // The transport refreshed the expired token, or signed the user out.
      if (ConnectError.from(err).code === Code.Unauthenticated && !servers.get(server.id)?.token) return;
    }
    await sleep(backoff(attempt++), signal);
  }
}

function refresh(qc: QueryClient, serverId: string, event: Event) {
  if (refreshes(event)) void qc.invalidateQueries({ queryKey: serverKey(serverId) });
}

/** Follows the events of every server the user is signed in to, for as long as the component is mounted. */
export function useEvents() {
  const servers = useServersStore();
  const list = useSignedIn();
  const qc = useQueryClient();
  const key = list.map((s) => `${s.id} ${s.url} ${s.sessionId}`).join("\n");
  useEffect(() => {
    const abort = new AbortController();
    for (const s of list) void follow(servers, s, (e) => refresh(qc, s.id, e), abort.signal);
    return () => abort.abort();
    // The streams restart only when a server, its address or its session changes.
  }, [servers, qc, key]);
}

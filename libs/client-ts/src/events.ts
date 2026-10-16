// The event stream of each server (docs/clients.md §3): changes refresh the
// server's queries they touch, so that the screens never poll, and every event reaches
// the app, which carries out remote commands and SyncPlay. Streams
// reconnect with backoff and end when the user signs out of their server.
import { Code, ConnectError } from "@connectrpc/connect";
import { type Event, EventService } from "@mavio/proto/mavio/session/v1/session_pb";
import { type QueryClient, type QueryKey, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

import { applyUserData, serverKey, useServersStore, useSignedIn } from "./hooks.ts";
import type { ServerEntry, Servers } from "./servers.ts";

/** The delay before the attempt-th reconnection: doubling from 1 s to 30 s, with jitter. */
export function backoff(attempt: number, random = Math.random): number {
  const base = Math.min(30_000, 1000 * 2 ** attempt);
  return base / 2 + random() * (base / 2);
}

/**
 * The kinds of a server's queries that show its libraries' items, or what
 * their items make, such as whether a discovered title is in a library.
 */
const libraryKinds = new Set([
  "acquisition",
  "children",
  "collections",
  "copies",
  "discover",
  "episodes",
  "extras",
  "home",
  "hub",
  "item",
  "items",
  "libraries",
  "library",
  "lyrics",
  "person",
  "playlists",
  "recommended",
  "requests",
  "search",
  "similar",
]);

/**
 * Of a server's query keys, which an event refreshes: a library change
 * those of its items (a title's ratings are not of the library), a new
 * stream all of them, since it may follow missed events, and the sessions'
 * a change of sessions. Changes of user data are put into the cache
 * instead (applyUserData).
 */
export function refreshes(event: Event, key: QueryKey): boolean {
  switch (event.kind.case) {
    case "connected":
      return true;
    case "libraryChanged":
      return libraryKinds.has(key[2] as string) && !(key[2] === "discover" && key[3] === "ratings");
    case "sessionsChanged":
      return key[2] === "sessions";
    default:
      return false;
  }
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
  if (event.kind.case === "userDataChanged") applyUserData(qc, serverId, event.kind.value.userData);
  else void qc.invalidateQueries({ queryKey: serverKey(serverId), predicate: (q) => refreshes(event, q.queryKey) });
}

/**
 * Follows the events of every server the user is signed in to, for as long
 * as the component is mounted, and hands each to onEvent.
 */
export function useEvents(onEvent?: (server: ServerEntry, event: Event) => void) {
  const servers = useServersStore();
  const list = useSignedIn();
  const qc = useQueryClient();
  const handler = useRef(onEvent);
  useEffect(() => {
    handler.current = onEvent;
  });
  const key = list.map((s) => `${s.id} ${s.url} ${s.sessionId}`).join("\n");
  useEffect(() => {
    const abort = new AbortController();
    for (const s of list) {
      void follow(
        servers,
        s,
        (e) => {
          refresh(qc, s.id, e);
          handler.current?.(s, e);
        },
        abort.signal,
      );
    }
    return () => abort.abort();
    // The streams restart only when a server, its address or its session changes.
  }, [servers, qc, key]);
}

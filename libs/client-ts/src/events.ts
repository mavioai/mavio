// The event stream of each server (docs/clients.md §3): changes refresh the
// server's queries they touch, so that the screens never poll, and every event reaches
// the app, which carries out remote commands and SyncPlay. Streams
// reconnect with backoff and end when the user signs out of their server.
// The tabs of a browser share its session, whose stream the server keeps
// one of: the tab in front holds it and passes its events on to the others.
import { fromBinary, toBinary } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { type Event, EventSchema, EventService } from "@mavio/proto/mavio/session/v1/session_pb";
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

/** What the tabs of a browser tell each other of a session's stream. */
type Shared = { event: Uint8Array } | { ask: true } | { keeps: boolean };

/** How long a tab coming to the front waits to hear whether the holder keeps the stream. */
const askTimeout = 300;

/**
 * Follows a server's events in the tab holding its session's stream, which
 * passes them on to the browser's other tabs, handed to relayed there. The
 * tab coming to the front takes the stream, unless the holder keeps it,
 * and a tab whose stream was taken waits to hold it again, as when the
 * other closes. Without locks or channels, as outside a browser, the tab
 * follows the events alone.
 */
function hold(
  servers: Servers,
  server: ServerEntry,
  onEvent: (event: Event) => void,
  relayed: (event: Event) => void,
  keep: () => boolean,
  signal: AbortSignal,
) {
  const locks = globalThis.navigator?.locks;
  if (!locks || typeof BroadcastChannel === "undefined" || typeof document === "undefined") {
    void follow(servers, server, onEvent, signal);
    return;
  }
  const name = `mavio.events ${server.id} ${server.sessionId}`;
  const channel = new BroadcastChannel(name);
  const post = (m: Shared) => channel.postMessage(m);
  let holding: AbortController | undefined;
  let waiting: AbortController | undefined;
  let answer: ((keeps: boolean) => void) | undefined;
  channel.onmessage = ({ data }: MessageEvent<Shared>) => {
    if ("event" in data) {
      if (!holding) relayed(fromBinary(EventSchema, data.event));
    } else if ("ask" in data) {
      if (holding) post({ keeps: keep() });
    } else answer?.(data.keeps);
  };
  const request = (steal: boolean) => {
    waiting?.abort();
    waiting = undefined;
    if (signal.aborted) return;
    // A request waiting in turn, which taking the stream gives up.
    const wait = steal ? undefined : new AbortController();
    waiting = wait;
    const own = new AbortController();
    locks
      .request(name, wait ? { signal: wait.signal } : { steal: true }, async () => {
        if (waiting === wait) waiting = undefined;
        holding = own;
        await follow(
          servers,
          server,
          (e) => {
            onEvent(e);
            post({ event: toBinary(EventSchema, e) });
          },
          AbortSignal.any([signal, own.signal]),
        );
      })
      .catch(() => {
        // Taken by a tab in front: this one waits its turn again.
        own.abort();
        if (holding === own) holding = undefined;
        if (!wait?.signal.aborted) request(false);
      });
  };
  const front = async () => {
    if (holding || signal.aborted || document.visibilityState !== "visible") return;
    const keeps = await new Promise<boolean>((resolve) => {
      answer = resolve;
      post({ ask: true });
      setTimeout(() => resolve(false), askTimeout);
    });
    answer = undefined;
    if (!keeps && !holding && !signal.aborted && document.visibilityState === "visible") request(true);
  };
  const onVisible = () => void front();
  document.addEventListener("visibilitychange", onVisible);
  addEventListener("focus", onVisible);
  signal.addEventListener(
    "abort",
    () => {
      waiting?.abort();
      channel.close();
      document.removeEventListener("visibilitychange", onVisible);
      removeEventListener("focus", onVisible);
    },
    { once: true },
  );
  request(false);
  void front();
}

function refresh(qc: QueryClient, serverId: string, event: Event) {
  if (event.kind.case === "userDataChanged") applyUserData(qc, serverId, event.kind.value.userData);
  else void qc.invalidateQueries({ queryKey: serverKey(serverId), predicate: (q) => refreshes(event, q.queryKey) });
}

/**
 * Follows the events of every server the user is signed in to, for as long
 * as the component is mounted, and hands each to onEvent in the tab holding
 * the session's stream; the other tabs only refresh their queries. keep
 * tells whether this tab keeps the stream from a tab coming to the front,
 * as while it plays.
 */
export function useEvents(onEvent?: (server: ServerEntry, event: Event) => void, keep: () => boolean = () => false) {
  const servers = useServersStore();
  const list = useSignedIn();
  const qc = useQueryClient();
  const handler = useRef(onEvent);
  const keeps = useRef(keep);
  useEffect(() => {
    handler.current = onEvent;
    keeps.current = keep;
  });
  const key = list.map((s) => `${s.id} ${s.url} ${s.sessionId}`).join("\n");
  useEffect(() => {
    const abort = new AbortController();
    for (const s of list) {
      hold(
        servers,
        s,
        (e) => {
          refresh(qc, s.id, e);
          handler.current?.(s, e);
        },
        (e) => refresh(qc, s.id, e),
        () => keeps.current(),
        abort.signal,
      );
    }
    return () => abort.abort();
    // The streams restart only when a server, its address or its session changes.
  }, [servers, qc, key]);
}

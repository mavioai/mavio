// The servers' events inside the web UI: the shell follows every server's
// stream and hands each event to the hub, from which the player and the
// remote commands take theirs (docs/clients.md §6.3.23).
import type { ServerEntry } from "@mavio/client-ts";
import type { Event } from "@mavio/proto/mavio/session/v1/session_pb";
import { createContext, useContext, useEffect, useRef } from "react";

type Listener = (server: ServerEntry, event: Event) => void;

/** Hands the events of the servers to whoever listens. */
export class EventHub {
  private listeners = new Set<Listener>();

  /** Hands an event of a server to every listener. */
  emit = (server: ServerEntry, event: Event) => {
    for (const l of this.listeners) l(server, event);
  };

  /** Listens to the events until the returned function is called. */
  on(listener: Listener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }
}

const HubContext = createContext<EventHub | undefined>(undefined);

/** Provides the app's event hub. */
export const EventHubProvider = HubContext.Provider;

/** The app's event hub. */
export function useEventHub(): EventHub {
  const hub = useContext(HubContext);
  if (!hub) throw new Error("useEventHub outside EventHubProvider");
  return hub;
}

/** Calls handler with every event of the servers for as long as the component is mounted. */
export function useServerEvents(handler: Listener) {
  const hub = useEventHub();
  const ref = useRef(handler);
  useEffect(() => {
    ref.current = handler;
  });
  useEffect(() => hub.on((s, e) => ref.current(s, e)), [hub]);
}

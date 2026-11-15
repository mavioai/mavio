// The player's local queue (docs/clients.md §6.5) is kept with the
// player's entry in the browser's history, which a reload keeps too: the
// player reads it there, and going on to the next or previous entry
// replaces the entry with the queue moved. Whatever plays several items —
// a playlist or collection, a remote Play command, the assistant's play
// action — opens the player through playItems.
import { type PlayQueue, playQueue, queueFor } from "@mavio/client-ts";
import { type AnyRouter, useRouterState } from "@tanstack/react-router";

declare module "@tanstack/react-router" {
  interface HistoryState {
    /** The player's local queue, whose entry playing is the player's item. */
    queue?: PlayQueue;
  }
}

/** Where the player starts its first item, and the streams it plays. */
export interface PlaySearch {
  start?: number;
  source?: string;
  audio?: number;
  subtitle?: number;
}

/**
 * Opens the player on a queue's entry, or on the only item; the queue is
 * kept with the player's entry in the history.
 */
export function playQueueAt(router: AnyRouter, serverId: string, queue: PlayQueue, search: PlaySearch = {}, replace = false) {
  return router.navigate({
    to: "/s/$serverId/play/$itemId",
    params: { serverId, itemId: queue.itemIds[queue.index] },
    search,
    state: { queue },
    replace,
  });
}

/**
 * Plays items in order from startIndex, the player going on to the next
 * at the end of one; a single item plays alone. False when there is none.
 */
export function playItems(router: AnyRouter, serverId: string, itemIds: readonly string[], startIndex = 0, search: PlaySearch = {}, replace = false): boolean {
  const queue = playQueue(itemIds, startIndex);
  if (queue) return (void playQueueAt(router, serverId, queue, search, replace), true);
  const id = itemIds[startIndex] ?? itemIds[0];
  if (!id) return false;
  void router.navigate({ to: "/s/$serverId/play/$itemId", params: { serverId, itemId: id }, search, replace });
  return true;
}

/** The queue of the player's entry in the history, if it plays the item. */
export function usePlayQueue(itemId: string): PlayQueue | undefined {
  const value = useRouterState({ select: (s) => s.location.state.queue });
  return queueFor(value, itemId);
}

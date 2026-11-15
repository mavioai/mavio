// The player's local play queue (docs/clients.md §6.5): items played in
// order, as a playlist's entries, a collection's members, or the items of
// a remote Play command or of the assistant's play action, the player
// going on to the next at the end of one. The same item may come several
// times, so the queue names the entry playing by its index.

/** Items played in order, and the index of the one playing. */
export interface PlayQueue {
  itemIds: string[];
  index: number;
}

/** The most items a queue holds, as many as a Play command carries. */
export const maxQueueItems = 1000;

/**
 * A queue of items starting at an index, which falls back to the first
 * when out of range; undefined for fewer than two items, which play alone.
 */
export function playQueue(itemIds: readonly string[], startIndex = 0): PlayQueue | undefined {
  const ids = itemIds.filter(Boolean).slice(0, maxQueueItems);
  if (ids.length < 2) return undefined;
  const index = Number.isInteger(startIndex) && startIndex >= 0 && startIndex < ids.length ? startIndex : 0;
  return { itemIds: ids, index };
}

/** The items in a random order (Fisher–Yates), starting at the first. */
export function shuffledQueue(itemIds: readonly string[], random: () => number = Math.random): PlayQueue | undefined {
  const ids = itemIds.filter(Boolean).slice(0, maxQueueItems);
  for (let i = ids.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1));
    [ids[i], ids[j]] = [ids[j]!, ids[i]!];
  }
  return playQueue(ids);
}

/** The queue moved to the next entry, or back to the previous one; undefined past either end. */
export function queueStep(q: PlayQueue | undefined, by: 1 | -1): PlayQueue | undefined {
  if (!q) return undefined;
  const index = q.index + by;
  return index >= 0 && index < q.itemIds.length ? { itemIds: q.itemIds, index } : undefined;
}

/**
 * The queue kept with a player's entry in the browser's history, checked:
 * undefined unless it is a queue whose entry playing is the item.
 */
export function queueFor(value: unknown, itemId: string): PlayQueue | undefined {
  if (typeof value !== "object" || value === null) return undefined;
  const { itemIds, index } = value as Partial<PlayQueue>;
  if (!Array.isArray(itemIds) || !itemIds.every((id) => typeof id === "string") || typeof index !== "number") return undefined;
  if (itemIds.length < 2 || itemIds.length > maxQueueItems || itemIds[index] !== itemId) return undefined;
  return value as PlayQueue;
}

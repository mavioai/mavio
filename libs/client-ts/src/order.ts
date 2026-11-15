// Ordered choices (docs/clients.md §6.3.4): what a list of the settings
// turns on and orders, as a library's providers and its hubs, each on or
// off and moved up or down.

/** One of the values an order may hold, and whether it holds it. */
export interface Choice<T> {
  value: T;
  on: boolean;
}

/**
 * The choices of an order among the values available: those it holds
 * first, on and in its order, those it does not after them, off, in their
 * order; an unset order holds every value available. A value it holds that
 * is no longer available keeps its place.
 */
export function choicesOf<T>(available: readonly T[], order: readonly T[] | undefined): Choice<T>[] {
  if (order === undefined) return available.map((value) => ({ value, on: true }));
  return [
    ...order.map((value) => ({ value, on: true })),
    ...available.filter((v) => !order.includes(v)).map((value) => ({ value, on: false })),
  ];
}

/** The order the choices make: the values turned on, in their order. */
export function orderOf<T>(choices: readonly Choice<T>[]): T[] {
  return choices.filter((c) => c.on).map((c) => c.value);
}

/** The choices with the one at index i turned on or off. */
export function toggled<T>(choices: readonly Choice<T>[], i: number): Choice<T>[] {
  return choices.map((c, j) => (j === i ? { ...c, on: !c.on } : c));
}

/** The list with the entry at index i moved by places, kept within it. */
export function moved<T>(choices: readonly T[], i: number, by: number): T[] {
  const to = Math.min(choices.length - 1, Math.max(0, i + by));
  const out = [...choices];
  const [c] = out.splice(i, 1);
  out.splice(to, 0, c);
  return out;
}

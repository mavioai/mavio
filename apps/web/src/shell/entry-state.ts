// What a page shows open, as an overview expanded, is kept with its entry
// in the browser's history: Back returns to the page as it was left, which
// its scroll position also needs, while the page opened anew starts afresh.
import { useRouterState } from "@tanstack/react-router";
import { useEffect, useState } from "react";

/** The values of the entries' states, by entry and name. */
const states = new Map<string, unknown>();

/** A state of the page kept with its entry in the history, under name. */
export function useEntryState<T>(name: string, initial: T): [T, (v: T) => void] {
  const entry = useRouterState({ select: (s) => s.location.state.__TSR_key ?? s.location.href });
  const id = `${entry} ${name}`;
  const [value, setValue] = useState<T>(() => (states.has(id) ? (states.get(id) as T) : initial));
  // An entry replaced while the page shows, as by a choice kept in the
  // address, keeps the state too.
  useEffect(() => {
    states.set(id, value);
  }, [id, value]);
  return [value, setValue];
}

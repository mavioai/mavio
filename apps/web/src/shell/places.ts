// The places of the sidebar and of the phone's tabs (docs/clients.md §4.1),
// each keeping the pages opened from it, as the Apple TV app's do: moving
// to another place and back returns to the page last shown there,
// scrolled where it was left; choosing the place shown returns to its top.
// The wider layouts' Back steps back through the place's pages alone.
import { type AnyRouter, useRouter } from "@tanstack/react-router";
import { type MouseEvent, useEffect, useSyncExternalStore } from "react";

import { parentOf } from "./back.ts";
import { returnToScroll } from "./up.ts";

/** The wider layouts' places: the pages the sidebar opens, and the user menu's. */
const sidebarPlaces = /^\/(?:|libraries|discover|search|profile|tools\/[^/]+|s\/[^/]+\/library\/[^/]+)$/;
/** The phone's: its tabs, search and the user's page; a library is a page of Libraries. */
const tabPlaces = /^\/(?:|libraries|discover|search|profile|tools\/[^/]+)$/;
/** The places of the layout shown. */
let places = sidebarPlaces;

/** Whether a path is a place's. */
export function isPlace(pathname: string): boolean {
  return places.test(pathname);
}

/** A page of a place: its address, and its entry's index in the history. */
type Page = { href: string; index: number };

/** Each place's pages, from the place to the page last shown, by the place's path. */
const stacks = new Map<string, Page[]>();
/** The place of each entry of the history, by its key. */
const entries = new Map<string, string>();
/** The place shown. */
let current: string | undefined;
const listeners = new Set<() => void>();

function pathOf(href: string): string {
  return href.split(/[?#]/)[0];
}

/** The shell's pages are kept: not the settings', the player's or signing in's. */
function kept(pathname: string): boolean {
  return !pathname.startsWith("/settings") && !pathname.includes("/play/") && !["/sign-in", "/setup", "/auth/callback", "/invite"].includes(pathname);
}

/** The pages above a page opened directly, up to its place, as if it had been opened from them. */
function chain(pathname: string, page: Page): Page[] {
  const pages = [page];
  for (let p = pathname, i = 0; !isPlace(p) && i < 8; i++) {
    p = parentOf(p);
    pages.unshift({ href: p, index: -1 });
  }
  return pages;
}

type Location = { href: string; state: { __TSR_key?: string; __TSR_index?: number } };

function visit({ href, state }: Location, replace: boolean) {
  const pathname = pathOf(href);
  if (!kept(pathname)) return;
  const page = { href, index: state.__TSR_index ?? -1 };
  const key = state.__TSR_key;
  let place: string;
  if (isPlace(pathname)) {
    place = pathname;
    stacks.set(place, [page]);
  } else {
    // An entry of the history returned to is its place's still.
    const known = (key && entries.get(key)) || current;
    const stack = known ? stacks.get(known) : undefined;
    if (!known || !stack) {
      const pages = chain(pathname, page);
      place = pathOf(pages[0].href);
      stacks.set(place, pages);
    } else {
      place = known;
      // A page already among the place's ends them, as Back or a link up returns to it.
      const i = stack.findIndex((p) => p.href === href);
      if (i >= 0) stack.splice(i, stack.length, page);
      else if (replace) stack.splice(-1, 1, page);
      else stack.push(page);
    }
  }
  current = place;
  if (key) entries.set(key, place);
  for (const l of listeners) l();
}

/** Keeps each place's pages as the history moves, the phone's places where compact. */
export function usePlaces(compact: boolean) {
  const router = useRouter();
  useEffect(() => {
    // The places change with the layout: each starts again from the page shown.
    places = compact ? tabPlaces : sidebarPlaces;
    stacks.clear();
    entries.clear();
    current = undefined;
    visit(router.history.location, false);
    return router.history.subscribe(({ location, action }) => visit(location, action.type === "REPLACE"));
  }, [router, compact]);
}

/** The place shown. */
export function usePlace(): string | undefined {
  return useSyncExternalStore(
    (l) => {
      listeners.add(l);
      return () => listeners.delete(l);
    },
    () => current,
  );
}

/** The props of a place's link: marked while its place is shown, and opening the place as openPlace does. */
export function usePlaceLink(): (to: string) => { "data-place"?: true; onClick: (e: MouseEvent) => void } {
  const router = useRouter();
  const place = usePlace();
  return (to) => ({ "data-place": place === to || undefined, onClick: (e: MouseEvent) => openPlace(e, router, to) });
}

/**
 * A place's link's click: another place opens at the page last shown there,
 * and the place shown at its top, each scrolled where it was left.
 */
export function openPlace(e: MouseEvent, router: AnyRouter, place: string) {
  if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
  const stack = stacks.get(place);
  if (!stack) return;
  const page = current === place ? stack[0] : stack.at(-1)!;
  if (page.href === router.history.location.href) return;
  e.preventDefault();
  // The page opened is the place's.
  current = place;
  returnToScroll();
  void router.navigate({ href: page.href, resetScroll: false });
}

/** Back from a page of a place: to the page it was opened from, else a level up. */
export function goBack(router: AnyRouter) {
  const { href, state } = router.history.location;
  const stack = current ? stacks.get(current) : undefined;
  const i = stack?.findIndex((p) => p.href === href) ?? -1;
  const previous = stack && i > 0 ? stack[i - 1] : undefined;
  if (!previous) return void router.navigate({ to: parentOf(pathOf(href)), replace: true });
  // The entry before this one in the history is the page's own: the browser goes back to it.
  if (previous.index >= 0 && previous.index === (state.__TSR_index ?? -1) - 1) return router.history.back();
  returnToScroll();
  void router.navigate({ href: previous.href, resetScroll: false });
}

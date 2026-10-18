// The remote's Back on the television (docs/tv.md §4): it always goes up
// a level rather than through the browser's history. A menu or dialog
// closes itself first, as Escape closes it; on a tab's page Back
// moves to the tab, the page at its top; any deeper page returns to where
// it was opened from, focus and position as they were.
import { type ParsedLocation, useRouter } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { isMenu, menuOf } from "../screens/settings-menus.ts";
import { focusState, type Point, pointOf } from "./focus.ts";
import { focusable, openPopup } from "./spatial.ts";
import { typedInto } from "./tv-keyboard.tsx";

/** The pages of the tabs, which Back leaves for the tabs. */
const tabPages = ["/", "/libraries", "/discover"];

/** Where Back goes from a page opened directly, with nowhere to return to. */
export function parentOf(pathname: string): string {
  // A title's cast and crew, to the title.
  const credits = /^(.*\/(?:item|title\/[^/]+)\/[^/]+)\/(?:credits|crew)$/.exec(pathname);
  if (credits) return credits[1];
  // A section of the settings, to its menu or their list.
  if (pathname.startsWith("/settings/")) {
    const menu = menuOf(pathname.slice("/settings/".length));
    return menu ? `/settings/${menu}` : "/settings";
  }
  if (pathname.endsWith("/requests")) return "/discover";
  if (pathname.includes("/library/")) return "/libraries";
  if (pathname.includes("/title/") || pathname.includes("/discover/")) return "/discover";
  return "/";
}

/** The settings' parameters naming a page within a section, as a plugin or a profile. */
const subpages = ["plugin", "catalogs", "library", "user", "remote", "acquisition", "subtitles", "id"];

/** The pages within a page within a section, by the page they lie in. */
const nested: Record<string, Record<string, string>> = { acquisition: { format: "formats" } };

/**
 * How deep a page lies in the settings: their list 0, a menu 1, a section
 * 2, a page within one 3 and a page within that 4; any other page -1.
 */
function depthOf(pathname: string, search: Record<string, unknown>): number {
  if (pathname === "/settings") return 0;
  if (!pathname.startsWith("/settings/")) return -1;
  if (isMenu(pathname.slice("/settings/".length))) return 1;
  if (Object.entries(nested).some(([k, pages]) => typeof search[k] === "string" && Object.hasOwn(pages, search[k]))) return 4;
  return subpages.some((k) => search[k] !== undefined) ? 3 : 2;
}

/**
 * The parameters of the page a page within a section lies in: the page
 * within it a nested page lies in, else the section as it was shown, as
 * the plugins with their tab.
 */
function upOf(search: Record<string, unknown>): Record<string, unknown> {
  for (const [k, pages] of Object.entries(nested)) {
    const page = typeof search[k] === "string" && Object.hasOwn(pages, search[k]) ? pages[search[k]] : undefined;
    if (page) return { server: search.server, [k]: page };
  }
  return Object.fromEntries(Object.entries(search).filter(([k]) => !subpages.includes(k) && k !== "view" && k !== "filter"));
}

/** The pages of the history by their place in it, as the router saw them. */
type Entry = { href: string; depth: number };

function indexOf(l: ParsedLocation): number {
  return (l.state as { __TSR_index?: number }).__TSR_index ?? 0;
}

/**
 * The element focused on a page, found again by its link or its place,
 * else focus goes to the element nearest its point.
 */
type Spot = { href?: string; nth: number; index: number; point: Point; rowScroll?: number };

function keyOf(l: ParsedLocation): string {
  return l.state.__TSR_key ?? l.href;
}

function spotOf(el: Element | null): Spot | undefined {
  if (!(el instanceof HTMLElement) || el === document.body) return undefined;
  const all = [...document.querySelectorAll<HTMLElement>(focusable)];
  const href = el.getAttribute("href") ?? undefined;
  return {
    href,
    nth: href ? all.filter((e) => e.getAttribute("href") === href).indexOf(el) : 0,
    index: all.indexOf(el),
    point: pointOf(el),
    rowScroll: el.closest(".scroll-row")?.scrollLeft,
  };
}

function find(spot: Spot): HTMLElement | undefined {
  const all = [...document.querySelectorAll<HTMLElement>(focusable)];
  if (spot.href) return all.filter((e) => e.getAttribute("href") === spot.href)[spot.nth];
  return all[spot.index];
}

/**
 * Focuses el where the page was left: its row scrolled as it was, and the
 * page and the row scrolled further only to show it.
 */
function refocus(el: HTMLElement, spot: Spot) {
  el.focus({ preventScroll: true });
  const v = el.getBoundingClientRect();
  if (v.bottom < 0 || v.top > innerHeight) el.scrollIntoView({ block: "nearest" });
  const row = el.closest<HTMLElement>(".scroll-row");
  if (!row) return;
  if (spot.rowScroll !== undefined) row.scrollLeft = spot.rowScroll;
  const r = el.getBoundingClientRect();
  const box = row.getBoundingClientRect();
  const pad = parseFloat(getComputedStyle(row).paddingLeft);
  if (r.left < box.left + pad) row.scrollLeft -= box.left + pad - r.left;
  else if (r.right > box.right - pad) row.scrollLeft += r.right - (box.right - pad);
}

/**
 * Where focus was on each page when it was left, by its history entry:
 * kept apart from the shell, which the settings and the player leave.
 */
const spots = new Map<string, Spot>();

/**
 * Goes up a level on Escape, and on Backspace but in a field a keyboard
 * types into, while on;
 * the player goes back by itself (docs/tv.md §7).
 */
export function useTvBack(on = true) {
  const router = useRouter();
  const [entries] = useState(() => new Map<number, Entry>());

  // Each page keeps where focus was when it was left, and gets it back on
  // return; the router restores its scroll.
  useEffect(() => {
    if (!on) return;
    const first = router.state.location;
    entries.set(indexOf(first), { href: first.href, depth: depthOf(first.pathname, first.search) });
    const offBefore = router.subscribe("onBeforeNavigate", ({ fromLocation, toLocation }) => {
      const spot = fromLocation && spotOf(document.activeElement);
      if (fromLocation && spot) spots.set(keyOf(fromLocation), spot);
      // The keeper of focus (focus.ts) waits while focus is put back.
      focusState.restoring = spots.has(keyOf(toLocation));
    });
    const offRendered = router.subscribe("onRendered", ({ toLocation }) => {
      entries.set(indexOf(toLocation), { href: toLocation.href, depth: depthOf(toLocation.pathname, toLocation.search) });
      const spot = spots.get(keyOf(toLocation));
      if (!spot) {
        focusState.restoring = false;
        return;
      }
      // The page's items may still be on their way from the cache; failing
      // them, focus goes to the element nearest where it was.
      let tries = 60;
      const attempt = () => {
        const el = find(spot);
        if (el) refocus(el, spot);
        else if (--tries > 0) return void requestAnimationFrame(attempt);
        else focusState.last = spot.point;
        focusState.restoring = false;
      };
      requestAnimationFrame(attempt);
    });
    return () => {
      offBefore();
      offRendered();
    };
  }, [router, on]);

  useEffect(() => {
    if (!on) return;
    const key = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey) return;
      if (router.state.location.pathname.includes("/play/")) return;
      // Backspace is a field's own only once a keyboard types into it; the
      // remote's Back goes back from it (tv-keyboard.tsx types for it).
      const typing = typedInto(document.activeElement);
      if (e.key !== "Escape" && (e.key !== "Backspace" || typing)) return;
      // An open popup closes on Escape by itself; Back is its Escape too.
      if (openPopup()) {
        if (e.key === "Backspace") {
          e.preventDefault();
          (document.activeElement ?? document.body).dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
        }
        return;
      }
      e.preventDefault();
      const { pathname } = router.state.location;
      if (tabPages.includes(pathname)) {
        const tabs = document.querySelector<HTMLElement>("[data-tv-tabs]");
        const current = tabs?.querySelector<HTMLElement>("a[data-status=active]");
        // A tab's page without its tab, as Discover with no server
        // discovering, leaves for Home.
        if (!current) return void router.navigate({ to: "/", replace: true });
        if (!tabs) return;
        if (!tabs.parentElement?.contains(document.activeElement)) {
          // From the page to its tab, the page back at its top.
          window.scrollTo({ top: 0 });
          for (const row of document.querySelectorAll<HTMLElement>("main .scroll-row")) row.scrollLeft = 0;
          current.focus({ preventScroll: true });
        } else if (pathname !== "/") {
          // From a tab to Home's, which focus opens.
          tabs.querySelector<HTMLElement>('a[href="/"]')?.focus();
        }
        return;
      }
      // Within the settings back goes up a level: to the last page before
      // in the history lying above this one, past those beside it, as a
      // plugin's tabs or a link of the trail opened; failing one, to its
      // parent.
      const location = router.state.location;
      const depth = depthOf(pathname, location.search);
      if (depth >= 0) {
        const here = indexOf(location);
        for (let i = here - 1; i >= 0; i--) {
          const e = entries.get(i);
          if (e && e.depth < depth) return void router.history.go(i - here);
        }
        if (depth >= 3) return void router.navigate({ to: pathname, search: upOf(location.search), replace: true } as never);
        return void router.navigate({ to: parentOf(pathname), replace: true });
      }
      if (router.history.canGoBack()) router.history.back();
      else void router.navigate({ to: parentOf(pathname), replace: true });
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, [router, on]);
}

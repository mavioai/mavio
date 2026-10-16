// Focus on the television is always somewhere, and shows (docs/tv.md §4):
// a page opens with focus on the element it names with data-tv-initial,
// else on its first; focus that is lost, as with the element that had it,
// goes to the element nearest where it was. The player alone may have none.
import { useRouter } from "@tanstack/react-router";
import { useEffect } from "react";

import { chosenIn, focusable, popups, visible } from "./spatial.ts";

/** A point in the page's coordinates, which scrolling leaves in place. */
export type Point = { x: number; y: number };

/**
 * What the keeper goes by: where the element focused last was, whether
 * Back is still putting focus back where a page was left (back.ts), and
 * when the page last changed.
 */
export const focusState: { last?: Point; lastElement?: HTMLElement; restoring: boolean; navigated: number } = {
  restoring: false,
  navigated: 0,
};

/** How long a button disabled while busy, which loses focus, is waited for to take it back. */
const busyWait = 10000;

/** Where an element's center is on the page. */
export function pointOf(el: Element): Point {
  const r = el.getBoundingClientRect();
  return { x: r.left + r.width / 2 + scrollX, y: r.top + r.height / 2 + scrollY };
}

/** How long a page that names its first focus is waited for, as its items come. */
const initialWait = 3000;
/** How long any other page is given to show its elements before focus goes to its first. */
const settle = 300;

function nearestTo(p: Point, els: HTMLElement[]): HTMLElement | undefined {
  let best: HTMLElement | undefined;
  let bestDistance = Number.POSITIVE_INFINITY;
  for (const el of els) {
    const q = pointOf(el);
    const d = Math.hypot(q.x - p.x, q.y - p.y);
    if (d < bestDistance) {
      bestDistance = d;
      best = el;
    }
  }
  return best;
}

/** Where focus goes when nothing has it, if anywhere yet. */
function target(): HTMLElement | undefined {
  const scope = document.querySelector<HTMLElement>(popups);
  const all = [...(scope ?? document).querySelectorAll<HTMLElement>(focusable)].filter(visible);
  const waited = performance.now() - focusState.navigated;
  if (!scope && !focusState.last) {
    const initial = document.querySelector<HTMLElement>("[data-tv-initial]");
    if (initial) {
      const el = initial.matches(focusable) ? initial : all.find((e) => initial.contains(e));
      if (el || waited < initialWait) return el;
    }
  }
  if (all.length === 0) return undefined;
  if (focusState.last) return nearestTo(focusState.last, all);
  if (scope) return all[0];
  if (waited < settle) return undefined;
  // The page's content, once it came, rather than the tabs above it.
  const content = all.find((el) => el.closest("main"));
  if (!content && document.querySelector("main") && waited < initialWait) return undefined;
  return content ?? all[0];
}

/** Keeps an element focused on every page but the player's, while on. */
export function useTvFocus(on: boolean) {
  const router = useRouter();
  useEffect(() => {
    if (!on) return;
    let disabledAt = 0;
    const onFocus = (e: FocusEvent) => {
      if (!(e.target instanceof HTMLElement)) return;
      focusState.last = pointOf(e.target);
      focusState.lastElement = e.target;
    };
    const offBefore = router.subscribe("onBeforeNavigate", () => {
      focusState.last = undefined;
      focusState.lastElement = undefined;
      focusState.navigated = performance.now();
    });
    const tick = () => {
      const active = document.activeElement;
      if (active && active !== document.body && active.isConnected) return;
      if (focusState.restoring || router.state.location.pathname.includes("/play/")) return;
      // A button disabled while it is busy gets focus back once done.
      const was = focusState.lastElement;
      if (was?.isConnected && was.matches(":disabled") && visible(was)) {
        disabledAt ||= performance.now();
        if (performance.now() - disabledAt < busyWait) return;
      }
      disabledAt = 0;
      const found = target();
      if (!found) return;
      const el = chosenIn(found) ?? found;
      el.focus({ preventScroll: true });
      const r = el.getBoundingClientRect();
      if (r.bottom < 0 || r.top > innerHeight) el.scrollIntoView({ block: "nearest" });
    };
    document.addEventListener("focusin", onFocus);
    const timer = setInterval(tick, 150);
    return () => {
      document.removeEventListener("focusin", onFocus);
      offBefore();
      clearInterval(timer);
    };
  }, [router, on]);
}

// Focus that moves with the arrow keys by rows, as a remote moves it on a
// television (docs/tv.md §4); where it cannot move, the focused element
// shakes.
import { useEffect } from "react";

type Direction = "ArrowUp" | "ArrowDown" | "ArrowLeft" | "ArrowRight";

/** The elements focus can move to. */
export const focusable = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea",
  "[tabindex]",
]
  .map((s) => `${s}:not([tabindex="-1"])`)
  .join(", ");

function visible(el: HTMLElement): boolean {
  const r = el.getBoundingClientRect();
  return r.width > 0 && r.height > 0 && getComputedStyle(el).visibility !== "hidden" && !el.closest("[inert],[aria-hidden=true]");
}

/**
 * The element nearest to from in a direction: of those wholly beyond its
 * edge, the one with the least distance along the direction plus twice
 * its offset across it, so that rows and columns are kept.
 */
export function nearest(from: DOMRect, dir: Direction, candidates: HTMLElement[]): HTMLElement | undefined {
  const cx = from.left + from.width / 2;
  const cy = from.top + from.height / 2;
  let best: HTMLElement | undefined;
  let bestScore = Number.POSITIVE_INFINITY;
  for (const el of candidates) {
    const r = el.getBoundingClientRect();
    const x = r.left + r.width / 2;
    const y = r.top + r.height / 2;
    let along: number;
    let across: number;
    switch (dir) {
      case "ArrowRight":
        along = r.left - from.right;
        across = Math.abs(y - cy);
        break;
      case "ArrowLeft":
        along = from.left - r.right;
        across = Math.abs(y - cy);
        break;
      case "ArrowDown":
        along = r.top - from.bottom;
        across = Math.abs(x - cx);
        break;
      case "ArrowUp":
        along = from.top - r.bottom;
        across = Math.abs(x - cx);
        break;
    }
    if (along < -1) continue;
    const score = along + across * 2;
    if (score < bestScore) {
      bestScore = score;
      best = el;
    }
  }
  return best;
}

type Box = { el: HTMLElement; r: DOMRect };

/** Whether two boxes lie side by side, overlapping by half the smaller's height. */
function sideBySide(a: DOMRect, b: DOMRect): boolean {
  return Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top) >= Math.min(a.height, b.height) / 2;
}

/**
 * The rows the elements form, from the top: the elements side by side,
 * each row from left to right, those a row scrolls out of view included.
 */
export function rowsOf(els: HTMLElement[]): Box[][] {
  const boxes = els.map((el) => ({ el, r: el.getBoundingClientRect() })).sort((a, b) => a.r.top - b.r.top || a.r.left - b.r.left);
  const rows: Box[][] = [];
  for (const b of boxes) {
    const row = rows.find((row) => sideBySide(row[0].r, b.r));
    if (row) row.push(b);
    else rows.push([b]);
  }
  return rows.map((row) => row.sort((a, b) => a.r.left - b.r.left));
}

/** Whether an element is in view across its row, which may scroll it out. */
function shown(b: Box): boolean {
  const box = b.el.closest(".scroll-row")?.getBoundingClientRect() ?? { left: 0, right: innerWidth };
  const x = (b.r.left + b.r.right) / 2;
  return x >= box.left && x <= box.right;
}

/** Of a row's elements in view, the one nearest x across. */
function closestTo(row: Box[], x: number): HTMLElement {
  const pool = row.some(shown) ? row.filter(shown) : row;
  const at = (b: Box) => Math.abs((b.r.left + b.r.right) / 2 - x);
  return pool.reduce((a, b) => (at(b) < at(a) ? b : a)).el;
}

/** A row's first or last element in view. */
function endOf(row: Box[], last: boolean): HTMLElement {
  const pool = row.some(shown) ? row.filter(shown) : row;
  return (last ? pool[pool.length - 1] : pool[0]).el;
}

/** The part of the screen focus moves within: a marked region, else the page. */
function regionOf(el: HTMLElement): Element {
  return el.closest("[data-focus-region]") ?? el.closest("main") ?? document.body;
}

/**
 * Where focus goes from active (docs/tv.md §4). Up and down go to the
 * row above or below, to its element nearest across; left and right move
 * along the row, and from its ends to the last element in view of the row
 * above or the first of the row below, unless another region lies beside,
 * as the search's keyboard beside its results. Above the first row are the
 * tabs, at the current one. Undefined when there is nowhere to go.
 */
export function moveFrom(active: HTMLElement, dir: Direction, all: HTMLElement[], tabs?: Element): HTMLElement | undefined {
  const region = regionOf(active);
  const rows = rowsOf(all.filter((el) => regionOf(el) === region));
  const ri = rows.findIndex((row) => row.some((b) => b.el === active));
  const row = rows[ri];
  const i = row.findIndex((b) => b.el === active);
  const here = row[i].r;
  const x = (here.left + here.right) / 2;
  const inTabs = tabs !== undefined && region === tabs;
  // Below the tabs is the page.
  const below = inTabs ? rowsOf(all.filter((el) => !tabs.contains(el)))[0] : rows[ri + 1];
  const above = () => {
    if (!tabs || inTabs) return undefined;
    const items = all.filter((el) => tabs.contains(el));
    return items.find((el) => el.matches("[data-status=active]")) ?? (items.length ? closestTo(rowsOf(items)[0], x) : undefined);
  };
  switch (dir) {
    case "ArrowUp":
      return rows[ri - 1] ? closestTo(rows[ri - 1], x) : above();
    case "ArrowDown":
      return below && closestTo(below, x);
    case "ArrowLeft":
    case "ArrowRight": {
      const right = dir === "ArrowRight";
      const along = row[i + (right ? 1 : -1)];
      if (along) return along.el;
      const beside = !inTabs && nearest(here, dir, all.filter((el) => regionOf(el) !== region && !tabs?.contains(el)));
      if (beside) return beside;
      if (right) return below && endOf(below, false);
      return rows[ri - 1] ? endOf(rows[ri - 1], true) : above();
    }
  }
}

/** Shakes an element briefly, where focus cannot move. */
function shake(el: HTMLElement) {
  el.classList.remove("tv-shake");
  void el.offsetWidth;
  el.classList.add("tv-shake");
  el.addEventListener("animationend", () => el.classList.remove("tv-shake"), { once: true });
}

/** Moves focus with the arrow keys; Back is the shell's (back.ts). */
export function useSpatialNavigation() {
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey) return;
      const active = document.activeElement instanceof HTMLElement ? document.activeElement : undefined;
      const typing = active?.matches("input, textarea, select") ?? false;
      if (!["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"].includes(e.key)) return;
      // The caret of a field moves sideways.
      if (typing && (e.key === "ArrowLeft" || e.key === "ArrowRight")) return;
      // An open menu keeps focus within it and the button that opened it
      // (data-focus-scope), a dialog within itself.
      const open = document.querySelector<HTMLElement>("[aria-modal]") ?? document.querySelector<HTMLElement>("[role=menu]");
      const scope = open && (open.closest<HTMLElement>("[data-focus-scope]") ?? open);
      const all = [...(scope ?? document).querySelectorAll<HTMLElement>(focusable)].filter(visible);
      e.preventDefault();
      if (!active || active === document.body || !all.includes(active)) {
        (scope ? all[0] : (document.querySelector<HTMLElement>("main " + focusable.split(",")[0]) ?? all[0]))?.focus();
        return;
      }
      const tabs = scope ? undefined : (document.querySelector("[data-focus-region=tabs]") ?? undefined);
      const next = moveFrom(active, e.key as Direction, all, tabs);
      if (!next) {
        shake(active);
        return;
      }
      next.focus({ preventScroll: true });
      next.scrollIntoView({ block: "nearest", inline: "nearest", behavior: "smooth" });
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, []);
}

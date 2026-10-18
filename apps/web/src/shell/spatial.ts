// Focus that moves with the arrow keys by rows, as a remote moves it on a
// television (docs/tv.md §4); where it cannot move, the focused element
// shakes.
import { useEffect } from "react";

type Direction = "ArrowUp" | "ArrowDown" | "ArrowLeft" | "ArrowRight";

/** The elements focus can move to. */
export const focusable = [
  // A television opens no web pages: links to them only show where they go.
  'a[href]:not([target="_blank"])',
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea",
  "[tabindex]",
]
  .map((s) => `${s}:not([tabindex="-1"])`)
  .join(", ");

/**
 * The popups open over the page: dialogs, menus, lists of options, and
 * what else asks for focus as it shows (data-tv-popup), as a message.
 */
export const popups = "[aria-modal=true], [role=alertdialog], [role=menu], [role=listbox], [data-tv-popup]";

/** Whether an element shows, so that focus may go to it. */
export function visible(el: HTMLElement): boolean {
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
 * A region in the page's flow (data-focus-flow), one of several stacked
 * as a title's top is: up and down leave it only from its first and last
 * rows, to the region or row before or after it in the page, and left and
 * right keep within it.
 */
const flow = "[data-focus-flow]";

/**
 * Columns side by side within a region of the flow (data-focus-column),
 * as an overview beside its facts: each has rows of its own, and left and
 * right cross between them only from and to what is marked to
 * (data-focus-beside), as the leading crew and the box of facts.
 */
const crosses = "[data-focus-beside]";

/** The column of a region el is in, while beside another, else the region. */
function columnOf(el: HTMLElement): Element {
  const region = regionOf(el);
  const column = el.closest("[data-focus-column]");
  if (!column || !region.contains(column)) return region;
  const r = column.getBoundingClientRect();
  const besides = [...region.querySelectorAll("[data-focus-column]")].some((c) => {
    const o = c.getBoundingClientRect();
    return c !== column && (o.left >= r.right || o.right <= r.left);
  });
  return besides ? column : region;
}

/**
 * An element at the end of what is above it (data-focus-end), as the link
 * under the leading crew to the whole crew: up and down reach it only from
 * the last element of a row, and pass it from the others.
 */
const end = "[data-focus-end]";

/** Whether b comes after a in the document. */
function after(a: Element, b: Element): boolean {
  return !!(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING) && !a.contains(b);
}

/**
 * Across a region in the page's flow: from row, the first element in view
 * of what follows it in the page, or the nearest across of the last row of
 * what precedes it; undefined where neither side is such a region.
 */
function acrossFlow(row: Box[], down: boolean, x: number, all: HTMLElement[], tabs?: Element): HTMLElement | undefined {
  const region = regionOf(row[0].el);
  // A region of the flow is left whole, past its other columns.
  const leaving = !!row[0].el.closest(flow);
  const page = all.filter(
    (el) => !tabs?.contains(el) && !(leaving && regionOf(el) === region) && row.every((b) => (down ? after(b.el, el) : after(el, b.el))),
  );
  const next = down ? page[0] : page[page.length - 1];
  if (!next) return undefined;
  const to = regionOf(next);
  if (to === region || !(next.closest(flow) || row[0].el.closest(flow))) return undefined;
  if (down) return endOf(rowsOf(page.filter((el) => columnOf(el) === columnOf(next)))[0], false);
  const rows = rowsOf(page.filter((el) => regionOf(el) === to));
  return closestTo(rows[rows.length - 1], x);
}

/** The chosen element of a choice that focus chooses (data-focus-choice). */
const chosen = "[aria-selected=true], [aria-checked=true]";

/** The chosen element of the choice el is in, if any. */
export function chosenIn(el: HTMLElement): HTMLElement | undefined {
  return el.closest("[data-focus-choice]")?.querySelector<HTMLElement>(chosen) ?? undefined;
}

/** Focus entering a choice from outside lands on its chosen element. */
function enter(active: HTMLElement, next: HTMLElement | undefined): HTMLElement | undefined {
  const choice = next?.closest("[data-focus-choice]");
  if (!next || !choice || choice.contains(active)) return next;
  return chosenIn(next) ?? next;
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
  return enter(active, step(active, dir, all, tabs));
}

function step(active: HTMLElement, dir: Direction, all: HTMLElement[], tabs?: Element): HTMLElement | undefined {
  const region = regionOf(active);
  const column = columnOf(active);
  const rows = rowsOf(all.filter((el) => columnOf(el) === column));
  const ri = rows.findIndex((row) => row.some((b) => b.el === active));
  const row = rows[ri];
  const i = row.findIndex((b) => b.el === active);
  const here = row[i].r;
  const x = (here.left + here.right) / 2;
  const inTabs = tabs !== undefined && region === tabs;
  // The row above or below, past what only a row's last element reaches.
  const rowAt = (d: 1 | -1) => {
    for (let j = ri + d; rows[j]; j += d) {
      const r = rows[j].filter((b) => i === row.length - 1 || !b.el.matches(end));
      if (r.length) return r;
    }
  };
  const previous = rowAt(-1);
  // Below the tabs is the page.
  const below = inTabs ? rowsOf(all.filter((el) => !tabs.contains(el)))[0] : rowAt(1);
  const above = () => {
    if (!tabs || inTabs) return undefined;
    const items = all.filter((el) => tabs.contains(el));
    return items.find((el) => el.matches(`[data-status=active], ${chosen}`)) ?? (items.length ? closestTo(rowsOf(items)[0], x) : undefined);
  };
  const inFlow = !inTabs && !!active.closest(flow);
  switch (dir) {
    case "ArrowUp": {
      // A row of the page goes first to a region of the flow between it and the row above.
      const across = inTabs || (inFlow && previous) ? undefined : acrossFlow(row, false, x, all, tabs);
      return across ?? (previous ? closestTo(previous, x) : above());
    }
    case "ArrowDown": {
      const across = inTabs || (inFlow && below) ? undefined : acrossFlow(row, true, x, all, tabs);
      return across ?? (below && closestTo(below, x));
    }
    case "ArrowLeft":
    case "ArrowRight": {
      const right = dir === "ArrowRight";
      const along = row[i + (right ? 1 : -1)];
      if (along) return along.el;
      if (inFlow) {
        const other = active.closest(crosses) && all.filter((el) => regionOf(el) === region && columnOf(el) !== column && el.closest(crosses));
        const across = other && nearest(here, dir, other);
        if (across) return across;
        return right ? below && endOf(below, false) : previous && endOf(previous, true);
      }
      const beside = !inTabs && nearest(here, dir, all.filter((el) => regionOf(el) !== region && !tabs?.contains(el)));
      if (beside) return beside;
      if (right) return below && endOf(below, false);
      return previous ? endOf(previous, true) : above();
    }
  }
}

/**
 * Whether a field keeps an arrow for itself: a slider's sideways, and a
 * caret's while it has room to move, as up and down between a text box's
 * lines; at the field's edges focus leaves it.
 */
function keeps(el: HTMLElement, key: Direction): boolean {
  const sideways = key === "ArrowLeft" || key === "ArrowRight";
  if (el instanceof HTMLInputElement && el.type === "range") return sideways;
  if (!(el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement)) return false;
  // Fields without a caret to ask, as a number's, keep none.
  const start = el.selectionStart;
  if (start === null || el.selectionEnd === null) return false;
  if (start !== el.selectionEnd) return sideways;
  const v = el.value;
  const lines = el instanceof HTMLTextAreaElement;
  switch (key) {
    case "ArrowLeft":
      return start > 0;
    case "ArrowRight":
      return start < v.length;
    case "ArrowUp":
      return lines && start > 0 && v.lastIndexOf("\n", start - 1) >= 0;
    case "ArrowDown":
      return lines && v.indexOf("\n", start) >= 0;
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
export function useSpatialNavigation(on = true) {
  useEffect(() => {
    if (!on) return;
    const key = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey) return;
      const active = document.activeElement instanceof HTMLElement ? document.activeElement : undefined;
      if (!["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"].includes(e.key)) return;
      if (active && keeps(active, e.key as Direction)) return;
      // The player seeks with the arrows while none of its controls has focus.
      if ((!active || active === document.body) && document.querySelector("[data-tv-player]")) return;
      // An open dialog, menu or list of options keeps focus within itself;
      // the keys a menu or list moves through its items with never reach here.
      const scope =
        document.querySelector<HTMLElement>("[aria-modal], [data-tv-popup]") ?? document.querySelector<HTMLElement>("[role=menu], [role=listbox]");
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
      // Within the page's first screen, as a hero's buttons are, the page
      // returns to its top, showing all of what is above them.
      if (!scope && next.getBoundingClientRect().bottom + scrollY <= innerHeight) scrollTo({ top: 0, behavior: "smooth" });
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, [on]);
}

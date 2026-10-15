// Focus that moves with the arrow keys to the nearest element in their
// direction, as a remote moves it on a television (docs/tv.md).
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
      const next = nearest(
        active.getBoundingClientRect(),
        e.key as Direction,
        all.filter((el) => el !== active && !el.contains(active) && !active.contains(el)),
      );
      if (next) {
        next.focus({ preventScroll: true });
        next.scrollIntoView({ block: "nearest", inline: "nearest", behavior: "smooth" });
      }
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, []);
}

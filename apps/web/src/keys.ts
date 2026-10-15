// Keyboard shortcuts (docs/clients.md §6.3): the keys of each platform,
// full screen as the browser's own, and their labels, shown only where a
// keyboard is at hand.
import { useSyncExternalStore } from "react";

/** Whether the shortcuts are the Mac's, with Command and Control. */
export const mac = /Mac|iPhone|iPad/.test(navigator.platform);

/** A shortcut of the web UI. */
export type Shortcut = "search" | "settings" | "tvMode" | "fullScreen";

const labels: Record<Shortcut, [mac: string, other: string]> = {
  search: ["⌘K", "Ctrl+K"],
  settings: ["⌘,", "Ctrl+,"],
  tvMode: ["⌃⌘T", "Shift+F11"],
  fullScreen: ["⌃⌘F", "F11"],
};

/** The label of a shortcut on this platform. */
export function shortcutLabel(s: Shortcut): string {
  return labels[s][mac ? 0 : 1];
}

/** The shortcut a key press makes, if any. */
export function shortcutOf(e: KeyboardEvent): Shortcut | undefined {
  if (e.altKey) return undefined;
  if (mac) {
    if (e.ctrlKey && e.metaKey && !e.shiftKey) return e.code === "KeyF" ? "fullScreen" : e.code === "KeyT" ? "tvMode" : undefined;
    if (!e.metaKey || e.ctrlKey || e.shiftKey) return undefined;
  } else {
    if (e.key === "F11" && !e.ctrlKey && !e.metaKey) return e.shiftKey ? "tvMode" : "fullScreen";
    if (!e.ctrlKey || e.metaKey || e.shiftKey) return undefined;
  }
  return e.key === "k" ? "search" : e.key === "," ? "settings" : undefined;
}

// A precise pointer that hovers comes with a keyboard: a computer, not a
// phone, a tablet or a television's browser.
const keyboard = matchMedia("(hover: hover) and (pointer: fine)");

function subscribe(l: () => void) {
  keyboard.addEventListener("change", l);
  return () => keyboard.removeEventListener("change", l);
}

/** Whether to show the labels of the shortcuts, which only a keyboard can use. */
export function useShortcutsShown(): boolean {
  return useSyncExternalStore(subscribe, () => keyboard.matches);
}

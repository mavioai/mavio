// Full screen for the whole page, which TV mode's menu and its F key
// toggle; the player keeps its own.
import { useSyncExternalStore } from "react";

function subscribe(l: () => void) {
  document.addEventListener("fullscreenchange", l);
  return () => document.removeEventListener("fullscreenchange", l);
}

/** Whether the page is in full screen. */
export function useFullScreen(): boolean {
  return useSyncExternalStore(subscribe, () => !!document.fullscreenElement);
}

/** Enters full screen, or leaves it. */
export function toggleFullScreen() {
  if (document.fullscreenElement) void document.exitFullscreen();
  else void document.documentElement.requestFullscreen().catch(() => {});
}

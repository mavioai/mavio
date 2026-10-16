// A page's link up to the page above it, as a settings page's to its list,
// goes there anew rather than back, which the router would show from its
// top: the page above is shown where it was scrolled when it was left.
import { useRouter } from "@tanstack/react-router";
import { type MouseEvent, useEffect } from "react";

/** Where each page was scrolled when it was last left, by its address and by its path. */
const scrolls = new Map<string, number>();
const byPath = new Map<string, number>();
/** Whether the navigation under way follows a link up. */
let goingUp = false;

/** Keeps where pages were left, and returns to it after a link up. */
export function useUpScroll() {
  const router = useRouter();
  useEffect(() => {
    const offBefore = router.subscribe("onBeforeNavigate", ({ fromLocation }) => {
      if (!fromLocation) return;
      scrolls.set(fromLocation.href, window.scrollY);
      byPath.set(fromLocation.pathname, window.scrollY);
    });
    const offRendered = router.subscribe("onRendered", ({ toLocation }) => {
      if (!goingUp) return;
      goingUp = false;
      // A link up names the page's path alone where what it showed, as a season, is in its address.
      const top = scrolls.get(toLocation.href) ?? byPath.get(toLocation.pathname);
      if (top === undefined) return;
      // The page may still grow as what it shows comes from the cache.
      let tries = 30;
      const restore = () => {
        window.scrollTo({ top });
        if (Math.abs(window.scrollY - top) > 1 && --tries > 0) requestAnimationFrame(restore);
      };
      restore();
    });
    return () => {
      offBefore();
      offRendered();
    };
  }, [router]);
}

/** The props of a link up, which returns to where the page above was scrolled. */
export const up = {
  resetScroll: false,
  onClick: (e: MouseEvent) => {
    // Not when the link opens elsewhere, as in another tab.
    if (e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey) goingUp = true;
  },
} as const;

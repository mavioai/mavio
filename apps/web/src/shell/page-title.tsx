// Every page names itself: the browser's tab and history show its title,
// and, but on the television, a page opened moves focus to its heading, or
// assistive technology is told its title where it has none.
import { useRouter } from "@tanstack/react-router";
import { useEffect, useState } from "react";

/** How long a page is given to show its heading, as what it shows comes. */
const headingWait = 3000;

/**
 * The page's heading: a section's named with data-title, else the first
 * h1, outside dialogs; an h1 marked data-title-later only once waited for
 * the section's.
 */
function headingOf(waited: boolean): HTMLElement | undefined {
  const outside = (el: Element) => !el.closest("[role=dialog],[role=alertdialog]");
  const named = [...document.querySelectorAll<HTMLElement>("[data-title]")].find(outside);
  const h1 = [...document.querySelectorAll<HTMLElement>("h1")].find(outside);
  return named ?? (h1?.hasAttribute("data-title-later") && !waited ? undefined : h1);
}

/** The page's title: its heading's text, else the destination of the navigation it is. */
function titleOf(heading?: HTMLElement): string | undefined {
  const text = heading?.textContent?.trim();
  if (text) return text;
  return document.querySelector("nav [aria-current=page]")?.textContent?.trim() || undefined;
}

/** Keeps the document's title, and, while focus is moved, the page's focus and its announcement. */
export function PageTitle({ moveFocus }: { moveFocus: boolean }) {
  const router = useRouter();
  const [announced, setAnnounced] = useState("");
  useEffect(() => {
    let timer: ReturnType<typeof setInterval> | undefined;
    const settle = (opened: boolean) => {
      clearInterval(timer);
      const start = performance.now();
      const step = () => {
        const waited = performance.now() - start >= headingWait;
        const heading = headingOf(waited);
        const title = titleOf(heading);
        document.title = title ? `${title} · Mavio` : "Mavio";
        if (!heading?.textContent?.trim() && !waited) return;
        clearInterval(timer);
        if (!opened || !moveFocus) return;
        // Focus left inside the page, as a field typed in, stays.
        const active = document.activeElement;
        if (active && active !== document.body && active.isConnected && !active.closest("nav")) return;
        if (heading) {
          if (!heading.hasAttribute("tabindex")) heading.tabIndex = -1;
          heading.focus({ preventScroll: true });
        } else if (title) setAnnounced(title);
      };
      timer = setInterval(step, 100);
      step();
    };
    // A page opened anew, rather than the same page with other choices.
    const off = router.subscribe("onRendered", ({ fromLocation, toLocation }) => {
      const opened = !!fromLocation && fromLocation.pathname !== toLocation.pathname && !toLocation.pathname.includes("/play/");
      setAnnounced("");
      settle(opened);
    });
    settle(false);
    return () => {
      off();
      clearInterval(timer);
    };
  }, [router, moveFocus]);
  return (
    <div aria-live="polite" aria-atomic className="sr-only">
      {announced}
    </div>
  );
}

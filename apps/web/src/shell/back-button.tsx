// The wide layout's Back (docs/clients.md §4.1): a round button of glass at
// the top left of a page opened from another, as the Apple TV app's. The
// sidebar's places have none: Back steps back through a place's pages
// (places.ts) and does not move between places. It floats over a page's
// hero (data-hero), and has a row of its own above a page without one,
// 12 px from the content's top and left edges at every width. The phone's
// header shows it in the wordmark's place (PhoneBack).
import { useLocation, useRouter } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { Tip } from "../controls/tooltip.tsx";
import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { goBack, isPlace } from "./places.ts";

/** Back over the page while it is one opened from another. */
export function BackButton() {
  const router = useRouter();
  const { pathname } = useLocation();
  if (isPlace(pathname)) return null;
  return (
    <div className="sticky top-0 z-30 h-15 [main:has([data-hero],>[data-loading])_&]:h-0">
      <Tip label={t("action.back")} side="right">
        <button
          type="button"
          aria-label={t("action.back")}
          onClick={() => goBack(router)}
          className="absolute top-3 left-3 flex size-9 items-center justify-center rounded-full bg-[var(--tabbar-glass)] text-on-background shadow-[var(--tabbar-shadow)] backdrop-blur-md backdrop-saturate-[1.8] hover:bg-[image:linear-gradient(rgb(128_128_128/0.1),rgb(128_128_128/0.1))]"
        >
          <Icon name="back" size={18} />
        </button>
      </Tip>
    </div>
  );
}

/** The phone's Back, in the header in place of the wordmark, while the page is one opened from another. */
export function PhoneBack({ fallback }: { fallback: ReactNode }) {
  const router = useRouter();
  const { pathname } = useLocation();
  if (isPlace(pathname)) return fallback;
  return (
    <button
      type="button"
      aria-label={t("action.back")}
      onClick={() => goBack(router)}
      className="-ml-2 flex size-9 items-center justify-center rounded-full text-on-background hover:bg-fill"
    >
      <Icon name="back" size={22} />
    </button>
  );
}

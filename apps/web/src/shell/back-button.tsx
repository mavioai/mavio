// The wide layout's Back (docs/clients.md §6.3): a round button of glass at
// the top left of a page opened from another, as the Apple TV app's. The
// sidebar's pages are its places, and have none: Back does not move
// between them. It floats over a page's hero (data-hero), and has a row of
// its own above a page without one.
import { useLocation, useRouter } from "@tanstack/react-router";

import { Tip } from "../controls/tooltip.tsx";
import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { parentOf } from "./back.ts";

/** The pages the sidebar opens, and the user menu's. */
const places = /^\/(?:|libraries|discover|search|profile|tools\/[^/]+|s\/[^/]+\/library\/[^/]+)$/;

/** Back over the page while it is one opened from another; it returns there, else a level up. */
export function BackButton() {
  const router = useRouter();
  const { pathname } = useLocation();
  if (places.test(pathname)) return null;
  return (
    <div className="sticky top-0 z-30 h-12 [main:has([data-hero])_&]:h-0">
      <Tip label={t("action.back")} side="right">
        <button
          type="button"
          aria-label={t("action.back")}
          onClick={() => {
            if (router.history.canGoBack()) router.history.back();
            else void router.navigate({ to: parentOf(pathname), replace: true });
          }}
          className="absolute top-3 left-margin [main:has([data-hero])_&]:top-6 flex size-9 items-center justify-center rounded-full bg-[var(--tabbar-glass)] text-on-background shadow-[var(--tabbar-shadow)] backdrop-blur-md backdrop-saturate-[1.8] hover:bg-[image:linear-gradient(rgb(128_128_128/0.1),rgb(128_128_128/0.1))]"
        >
          <Icon name="back" size={18} />
        </button>
      </Tip>
    </div>
  );
}

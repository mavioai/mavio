// The television layout (docs/tv.md), the same as on tvOS and
// Android TV: tabs in a pill at the top, the user menu's dots at the right,
// and focus moved with the arrow keys.
import { Link, Outlet, useLocation, useNavigate } from "@tanstack/react-router";
import { useSyncExternalStore } from "react";

import { t } from "../i18n.ts";
import { useTvBack } from "./back.ts";
import { useSpatialNavigation } from "./spatial.ts";
import { UserButton } from "./user-menu.tsx";

const tabs: { to: "/" | "/libraries" | "/search"; label: () => string }[] = [
  { to: "/", label: () => t("nav.home") },
  { to: "/libraries", label: () => t("nav.libraries") },
  { to: "/search", label: () => t("nav.search") },
];

export function TvShell() {
  useSpatialNavigation();
  useTvBack();
  const { pathname } = useLocation();
  const navigate = useNavigate();
  // On a tab's own page, as on tvOS, focusing another tab opens it in its
  // place, so that Back never steps through the tabs; deeper pages leave the
  // tabs to select, so that moving up does not leave them.
  // Search opens only when selected everywhere, since its page hides the tabs.
  const onTab = tabs.some((d) => d.to === pathname);
  const search = pathname === "/search";
  // A title's details and a library fill the screen without the tabs, the
  // library under its name; back leaves them.
  const details = pathname.includes("/item/");
  const library = pathname.includes("/library/");
  // Home's hero reaches the top of the screen, behind the tabs.
  const behind = pathname === "/";
  const scrolled = useScrolled();
  return (
    <div className="min-h-screen">
      {!search && !details && !library && (
        <header className="pointer-events-none fixed inset-x-0 top-0 z-30 flex items-center justify-center px-margin pt-8 pb-6">
          {/* Scrolled content passes under the tabs, never through them. */}
          <div
            aria-hidden
            className={`absolute inset-0 bg-background transition-opacity duration-200 ${scrolled ? "opacity-100" : "opacity-0"}`}
          />
          <div data-focus-region="tabs" className="relative flex w-full items-center justify-center">
            <nav data-tv-tabs aria-label={t("nav.navigation")} className="pointer-events-auto flex gap-1 rounded-full bg-surface-raised/60 p-1.5 backdrop-blur-xl">
              {tabs.map((d) => (
                <Link
                  key={d.to}
                  to={d.to}
                  activeOptions={{ exact: d.to === "/" }}
                  onFocus={() => onTab && d.to !== pathname && d.to !== "/search" && void navigate({ to: d.to, replace: true })}
                  className="tv-pill rounded-full px-7 py-2.5 type-headline text-text-secondary data-[status=active]:bg-fill-selected data-[status=active]:text-on-background"
                >
                  {d.label()}
                </Link>
              ))}
            </nav>
            {/* The user menu's dots, at the screen's right, centered on the tabs. */}
            <div className="pointer-events-auto absolute right-0">
              <UserButton dots placement="below" />
            </div>
          </div>
        </header>
      )}
      <main className={behind || details ? "pb-16" : search || library ? "pt-12 pb-16" : "pt-32 pb-16"}>
        <Outlet />
      </main>
    </div>
  );
}

function subscribeScroll(l: () => void) {
  window.addEventListener("scroll", l, { passive: true });
  return () => window.removeEventListener("scroll", l);
}

/** Whether the page has scrolled from its top. */
function useScrolled(): boolean {
  return useSyncExternalStore(subscribeScroll, () => window.scrollY > 0);
}

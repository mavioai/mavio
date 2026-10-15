// The television layout (docs/clients.md §6.3), the same as on tvOS and
// Android TV: tabs in a pill at the top, the user's avatar beside them,
// and focus moved with the arrow keys.
import { Link, Outlet, useLocation, useNavigate } from "@tanstack/react-router";
import { useSyncExternalStore } from "react";

import { t } from "../i18n.ts";
import { useSpatialNavigation } from "./spatial.ts";
import { UserButton } from "./user-menu.tsx";

const tabs: { to: "/" | "/libraries" | "/search"; label: () => string }[] = [
  { to: "/", label: () => t("nav.home") },
  { to: "/libraries", label: () => t("nav.libraries") },
  { to: "/search", label: () => t("nav.search") },
];

export function TvShell() {
  useSpatialNavigation();
  const { pathname } = useLocation();
  const navigate = useNavigate();
  // On a tab's own page, as on tvOS, focusing another tab opens it; deeper
  // pages leave the tabs to select, so that moving up does not leave them.
  const onTab = tabs.some((d) => d.to === pathname);
  // Home's hero reaches the top of the screen, behind the tabs.
  const behind = pathname === "/" || pathname.includes("/item/");
  const scrolled = useScrolled();
  return (
    <div className="min-h-screen">
      <header className="pointer-events-none fixed inset-x-0 top-0 z-30 flex items-center justify-center px-margin pt-8 pb-6">
        {/* Scrolled content passes under the tabs, never through them. */}
        <div
          aria-hidden
          className={`absolute inset-0 bg-background transition-opacity duration-200 ${scrolled ? "opacity-100" : "opacity-0"}`}
        />
        <div className="relative flex w-full items-center justify-center">
          <nav aria-label={t("nav.navigation")} className="pointer-events-auto flex gap-1 rounded-full bg-surface-raised/60 p-1.5 backdrop-blur-xl">
            {tabs.map((d) => (
              <Link
                key={d.to}
                to={d.to}
                activeOptions={{ exact: d.to === "/" }}
              onFocus={() => onTab && d.to !== pathname && void navigate({ to: d.to })}
                className="tv-pill rounded-full px-7 py-2.5 type-headline text-text-secondary data-[status=active]:bg-fill-selected data-[status=active]:text-on-background"
              >
                {d.label()}
              </Link>
            ))}
          </nav>
          {/* The avatar, centered on the tabs. */}
          <div className="pointer-events-auto absolute -right-10">
            <UserButton compact placement="below" />
          </div>
        </div>
      </header>
      <main className={behind ? "pb-16" : "pt-32 pb-16"}>
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

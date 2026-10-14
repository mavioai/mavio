// The television layout (docs/clients.md §6.3), the same as on tvOS and
// Android TV: tabs in a pill at the top, the user's avatar beside them,
// and focus moved with the arrow keys.
import { Link, Outlet, useLocation } from "@tanstack/react-router";

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
  // Home's hero reaches the top of the screen, behind the tabs.
  const behind = pathname === "/" || pathname.includes("/item/");
  return (
    <div className="min-h-screen">
      <header className="pointer-events-none fixed inset-x-0 top-0 z-30 flex items-center justify-center px-margin pt-8">
        <nav aria-label={t("nav.navigation")} className="pointer-events-auto flex gap-1 rounded-full bg-surface/60 p-1.5 backdrop-blur-xl">
          {tabs.map((d) => (
            <Link
              key={d.to}
              to={d.to}
              activeOptions={{ exact: d.to === "/" }}
              className="tv-pill rounded-full px-7 py-2.5 type-headline text-text-secondary data-[status=active]:bg-fill-selected data-[status=active]:text-on-background"
            >
              {d.label()}
            </Link>
          ))}
        </nav>
        <div className="pointer-events-auto absolute top-8 right-margin">
          <UserButton compact placement="below" />
        </div>
      </header>
      <main className={behind ? "pb-16" : "pt-32 pb-16"}>
        <Outlet />
      </main>
    </div>
  );
}

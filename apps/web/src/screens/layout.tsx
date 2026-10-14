// The shell of the web UI: a sidebar on wide windows and a tab bar on narrow
// ones (docs/clients.md §6.3), around every screen but sign-in and the
// player.
import { useEvents, useLibraries, useServersStore, useSignedIn } from "@mavio/client-ts";
import { Link, Navigate, Outlet, useLocation } from "@tanstack/react-router";
import { useEffect } from "react";

import { t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";

const destinations: { to: "/" | "/libraries" | "/search" | "/profile"; icon: IconName; label: () => string }[] = [
  { to: "/", icon: "home", label: () => t("nav.home") },
  { to: "/libraries", icon: "movies", label: () => t("nav.libraries") },
  { to: "/search", icon: "search", label: () => t("nav.search") },
  { to: "/profile", icon: "profile", label: () => t("nav.profile") },
];

export function Layout() {
  const servers = useServersStore();
  const signedIn = useSignedIn();
  const { pathname } = useLocation();
  useEvents();

  // The servers the user added in other browsers, kept by the serving server.
  const homeToken = signedIn.find((s) => s.url === servers.home)?.token;
  useEffect(() => {
    if (homeToken) servers.load().catch(() => {});
  }, [servers, homeToken]);

  const bare = pathname.startsWith("/sign-in") || pathname.includes("/play/");
  if (signedIn.length === 0 && !pathname.startsWith("/sign-in")) {
    return <Navigate to="/sign-in" />;
  }
  if (bare) return <Outlet />;
  return (
    <div className="min-h-screen md:flex">
      <Sidebar />
      <main className="min-w-0 flex-1 pb-20 md:pb-8">
        <Outlet />
      </main>
      <TabBar />
    </div>
  );
}

function Sidebar() {
  const libraries = useLibraries();
  const several = libraries.length > 1;
  return (
    <nav className="sticky top-0 hidden h-screen w-60 shrink-0 flex-col gap-1 overflow-y-auto border-r border-border bg-surface p-3 md:flex">
      <div className="px-3 pt-2 pb-4 type-title2 text-on-background">Mavio</div>
      {destinations.map((d) => (
        <Link
          key={d.to}
          to={d.to}
          activeOptions={{ exact: d.to === "/" }}
          className="flex min-h-11 items-center gap-3 rounded-md px-3 type-headline text-text-secondary hover:bg-surface-raised data-[status=active]:bg-surface-raised data-[status=active]:text-on-background"
        >
          {({ isActive }) => (
            <>
              <Icon name={d.icon} fill={isActive} />
              {d.label()}
            </>
          )}
        </Link>
      ))}
      <div className="mt-4 flex flex-col gap-0.5">
        {libraries.map(({ server, libraries: list }) => (
          <div key={server.id} className="flex flex-col gap-0.5">
            {several && list.length > 0 && (
              <div className="px-3 pt-3 pb-1 type-caption text-text-tertiary uppercase">{server.name}</div>
            )}
            {list.map((l) => (
              <Link
                key={l.id}
                to="/s/$serverId/library/$libraryId"
                params={{ serverId: server.id, libraryId: l.id }}
                className="truncate rounded-md px-3 py-2 type-callout text-text-secondary hover:bg-surface-raised data-[status=active]:text-on-background"
              >
                {l.name}
              </Link>
            ))}
          </div>
        ))}
      </div>
    </nav>
  );
}

function TabBar() {
  return (
    <nav className="fixed inset-x-0 bottom-0 z-20 flex border-t border-border bg-surface/95 pb-[env(safe-area-inset-bottom)] backdrop-blur md:hidden">
      {destinations.map((d) => (
        <Link
          key={d.to}
          to={d.to}
          activeOptions={{ exact: d.to === "/" }}
          className="flex min-h-14 flex-1 flex-col items-center justify-center gap-0.5 type-caption text-text-secondary data-[status=active]:text-accent-text"
        >
          {({ isActive }) => (
            <>
              <Icon name={d.icon} fill={isActive} />
              {d.label()}
            </>
          )}
        </Link>
      ))}
    </nav>
  );
}

// The shell of the web UI (docs/clients.md §6.3), chosen by the window's
// width: a tab bar below 600 px, a rail with the sidebar over the content
// up to 840 px, and the sidebar beside the content from there, collapsible
// to the rail. TV mode replaces them with the television layout. Sign-in,
// setup, the player and the settings bring their own.
import { useDiscovery, useEvents, useServersStore, useSignIn } from "@mavio/client-ts";
import { Link, Navigate, Outlet, useLocation, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { useEventHub } from "../events.tsx";
import { discoverLanguage, t } from "../i18n.ts";
import { toggleFullScreen } from "../fullscreen.ts";
import { Icon, type IconName } from "../icon.tsx";
import { shortcutOf } from "../keys.ts";
import { Wordmark } from "../logo.tsx";
import { setPrefs, usePrefs, useWidthClass } from "../prefs.ts";
import { Avatar } from "../ui.tsx";
import { RemoteCommands } from "../screens/remote.tsx";
import { GroupIndicator } from "../screens/syncplay.tsx";
import { SyncPlayProvider } from "../syncplay.tsx";
import { SidebarPanel, SidebarResizer, sidebarWidth } from "./sidebar.tsx";
import { TvShell } from "./tv.tsx";
import { useAccount } from "./user-menu.tsx";

export function Shell() {
  return (
    <SyncPlayProvider>
      <Content />
      <RemoteCommands />
      <GroupIndicator />
    </SyncPlayProvider>
  );
}

function Content() {
  const servers = useServersStore();
  const signIn = useSignIn();
  const { pathname } = useLocation();
  const prefs = usePrefs();
  const width = useWidthClass();
  useEvents(useEventHub().emit);
  useShortcuts(prefs.tv);

  // A Mavio account follows the servers it reaches, as it opens.
  const account = signIn?.account?.id;
  useEffect(() => {
    if (account) servers.sync().catch(() => {});
  }, [servers, account]);

  const bare = ["/sign-in", "/setup", "/auth/callback", "/invite"].includes(pathname);
  if (!signIn && !bare) return <Navigate to="/sign-in" />;
  if (bare || pathname.includes("/play/")) return <Outlet />;
  if (pathname.startsWith("/settings")) return <Outlet />;
  if (prefs.tv) return <TvShell />;
  if (width === "compact") return <CompactShell />;
  return <DesktopShell overlay={width === "medium"} />;
}

/**
 * The shortcuts of keys.ts: search, the settings, TV mode, and full screen
 * with the browser's own keys, which the page takes so that its menus
 * know it is in full screen.
 */
function useShortcuts(tv: boolean) {
  const navigate = useNavigate();
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      const s = shortcutOf(e);
      if (s === "search") {
        // The sidebar's field where it shows, else the Search page.
        const box = document.querySelector<HTMLInputElement>("[data-search-box]");
        if (box) box.focus();
        else void navigate({ to: "/search" });
      }
      else if (s === "settings") void navigate({ to: "/settings" });
      else if (s === "tvMode") setPrefs({ tv: !tv });
      else if (s === "fullScreen") toggleFullScreen();
      else return;
      e.preventDefault();
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, [navigate, tv]);
}

/**
 * The sidebar and the content. Up to 840 px the sidebar is a rail whose
 * toggle lays the full sidebar over the content; wider, the toggle
 * collapses it to the rail and is remembered.
 */
function DesktopShell({ overlay }: { overlay: boolean }) {
  const prefs = usePrefs();
  const [over, setOver] = useState(false);
  const { pathname } = useLocation();
  useEffect(() => setOver(false), [pathname, overlay]);
  const rail = overlay || !prefs.sidebar;
  return (
    <div className="flex min-h-screen">
      <aside
        className="sticky top-0 h-screen shrink-0"
        style={{ width: rail ? "var(--mavio-layout-rail)" : sidebarWidth }}
      >
        <SidebarPanel rail={rail} onToggle={() => (overlay ? setOver(true) : setPrefs({ sidebar: !prefs.sidebar }))} />
        {!rail && <SidebarResizer />}
      </aside>
      {over && (
        <div className="fixed inset-0 z-40 bg-scrim" onPointerDown={(e) => e.target === e.currentTarget && setOver(false)}>
          <aside className="h-full shadow-level3" style={{ width: sidebarWidth }}>
            <SidebarPanel rail={false} onToggle={() => setOver(false)} />
          </aside>
        </div>
      )}
      <main className="min-w-0 flex-1 pb-8">
        <Outlet />
      </main>
    </div>
  );
}

const allTabs: { to: "/" | "/libraries" | "/discover" | "/search"; icon: IconName; label: () => string }[] = [
  { to: "/", icon: "home", label: () => t("nav.home") },
  { to: "/libraries", icon: "movies", label: () => t("nav.libraries") },
  { to: "/discover", icon: "discover", label: () => t("nav.discover") },
  { to: "/search", icon: "search", label: () => t("nav.search") },
];

/** The phone layout: the wordmark and the user's avatar on top, tabs below. */
function CompactShell() {
  const account = useAccount();
  // Discover is a tab where a server discovers.
  const discovers = !!useDiscovery(discoverLanguage).server;
  const tabs = allTabs.filter((d) => d.to !== "/discover" || discovers);
  return (
    <div className="min-h-screen pb-[calc(56px+env(safe-area-inset-bottom))]">
      <header className="sticky top-0 z-20 flex h-13 items-center justify-between bg-background/80 px-margin pt-[env(safe-area-inset-top)] backdrop-blur-xl">
        <Wordmark height={20} />
        <Link to="/settings" aria-label={t("nav.settings")} className="rounded-full">
          <Avatar name={account.name} size={32} />
        </Link>
      </header>
      <main>
        <Outlet />
      </main>
      <nav
        aria-label={t("nav.navigation")}
        className="fixed inset-x-0 bottom-0 z-20 flex border-t border-border bg-surface/85 pb-[env(safe-area-inset-bottom)] backdrop-blur-xl"
      >
        {tabs.map((d) => (
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
    </div>
  );
}

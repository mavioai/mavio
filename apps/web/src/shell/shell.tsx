// The shell of the web UI (docs/clients.md §6.3), chosen by the window's
// width: a tab bar below 600 px, a rail with the sidebar over the content
// up to 840 px, and the sidebar beside the content from there, collapsible
// to the rail. TV mode replaces them with the television layout. Sign-in,
// the player and the settings bring their own.
import { useEvents, useServersStore, useSignedIn } from "@mavio/client-ts";
import { Link, Navigate, Outlet, useLocation, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { Wordmark } from "../logo.tsx";
import { setPrefs, usePrefs, useWidthClass } from "../prefs.ts";
import { Avatar } from "../ui.tsx";
import { SidebarPanel } from "./sidebar.tsx";
import { TvShell } from "./tv.tsx";
import { useAccount } from "./user-menu.tsx";

export function Shell() {
  const servers = useServersStore();
  const signedIn = useSignedIn();
  const { pathname } = useLocation();
  const prefs = usePrefs();
  const width = useWidthClass();
  useEvents();
  useShortcuts();

  // The servers the user added in other browsers, kept by the serving server.
  const homeToken = signedIn.find((s) => s.url === servers.home)?.token;
  useEffect(() => {
    if (homeToken) servers.load().catch(() => {});
  }, [servers, homeToken]);

  if (signedIn.length === 0 && !pathname.startsWith("/sign-in")) {
    return <Navigate to="/sign-in" />;
  }
  if (pathname.startsWith("/sign-in") || pathname.includes("/play/")) return <Outlet />;
  if (pathname.startsWith("/settings")) return <Outlet />;
  if (prefs.tv) return <TvShell />;
  if (width === "compact") return <CompactShell />;
  return <DesktopShell overlay={width === "medium"} />;
}

/** ⌘K searches and ⌘, opens the settings, as on the Mac. */
function useShortcuts() {
  const navigate = useNavigate();
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey) || e.altKey || e.shiftKey) return;
      if (e.key === "k") void navigate({ to: "/search" });
      else if (e.key === ",") void navigate({ to: "/settings" });
      else return;
      e.preventDefault();
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, [navigate]);
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
        className="sticky top-0 h-screen shrink-0 py-2.5 pl-2.5"
        style={{ width: rail ? "var(--mavio-layout-rail)" : "var(--mavio-layout-sidebar)" }}
      >
        <SidebarPanel rail={rail} onToggle={() => (overlay ? setOver(true) : setPrefs({ sidebar: !prefs.sidebar }))} />
      </aside>
      {over && (
        <div className="fixed inset-0 z-40 bg-scrim" onPointerDown={(e) => e.target === e.currentTarget && setOver(false)}>
          <aside className="h-full w-[280px] p-2.5 shadow-level3">
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

const tabs: { to: "/" | "/libraries" | "/search"; icon: IconName; label: () => string }[] = [
  { to: "/", icon: "home", label: () => t("nav.home") },
  { to: "/libraries", icon: "movies", label: () => t("nav.libraries") },
  { to: "/search", icon: "search", label: () => t("nav.search") },
];

/** The phone layout: the wordmark and the user's avatar on top, tabs below. */
function CompactShell() {
  const account = useAccount();
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

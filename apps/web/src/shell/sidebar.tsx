// The sidebar of the desktop layout (docs/clients.md §6.3): the wordmark,
// search, Home, the libraries of the shown sign-in and the user at its
// foot. Collapsed, it is a rail of icons.
import { useDiscovery, useLibraries } from "@mavio/client-ts";
import { Link, useLocation, useNavigate } from "@tanstack/react-router";
import { type ReactNode, useEffect, useState } from "react";

import { discoverLanguage, t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { shortcutLabel, useShortcutsShown } from "../keys.ts";
import { Wordmark } from "../logo.tsx";
import { libraryIcon } from "../ui.tsx";
import { UserButton } from "./user-menu.tsx";

/** The sidebar's panel, expanded or as a rail; onToggle expands or collapses it. */
export function SidebarPanel({ rail, onToggle }: { rail: boolean; onToggle: () => void }) {
  const libraries = useLibraries();
  const several = libraries.length > 1;
  const discovers = !!useDiscovery(discoverLanguage).server;
  const shortcuts = useShortcutsShown();
  return (
    <nav
      aria-label={t("nav.navigation")}
      className="flex h-full flex-col rounded-[18px] border border-border bg-sidebar"
    >
      <div className={`flex h-14 shrink-0 items-center ${rail ? "justify-center" : "justify-between pr-2.5 pl-4"}`}>
        {!rail && <Wordmark height={22} />}
        <button
          type="button"
          onClick={onToggle}
          aria-label={rail ? t("nav.expandSidebar") : t("nav.collapseSidebar")}
          title={rail ? t("nav.expandSidebar") : t("nav.collapseSidebar")}
          className="flex h-9 w-9 items-center justify-center rounded-lg text-text-secondary hover:bg-fill hover:text-on-background"
        >
          <Icon name="sidebar" size={20} />
        </button>
      </div>
      <div className={`flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto pb-2 ${rail ? "items-center px-2" : "px-2.5"}`}>
        {rail ? (
          <NavItem rail to="/search" icon="search" label={t("nav.search")} />
        ) : (
          <SearchBox shortcut={shortcuts ? shortcutLabel("search") : undefined} />
        )}
        <NavItem rail={rail} to="/" icon="home" label={t("nav.home")} exact />
        {discovers && <NavItem rail={rail} to="/discover" icon="discover" label={t("nav.discover")} />}
        {rail ? (
          <div className="my-2 h-px w-8 bg-border" />
        ) : (
          <div className="px-2.5 pt-4 pb-1 type-caption font-semibold text-text-tertiary">{t("nav.libraries")}</div>
        )}
        {libraries.map(({ server, libraries: list }) => (
          <div key={server.id} className={`flex flex-col gap-0.5 ${rail ? "items-center" : ""}`}>
            {several && list.length > 0 && !rail && (
              <div className="truncate px-2.5 pt-2 pb-0.5 type-caption text-text-tertiary">{server.name}</div>
            )}
            {list.map((l) => (
              <Link
                key={l.id}
                to="/s/$serverId/library/$libraryId"
                params={{ serverId: server.id, libraryId: l.id }}
                title={rail ? (several ? `${l.name} · ${server.name}` : l.name) : undefined}
                aria-label={rail ? l.name : undefined}
                className={itemClass(rail)}
              >
                <Icon name={libraryIcon(l.kind)} size={20} className="shrink-0 text-accent-text" />
                {!rail && <span className="truncate">{l.name}</span>}
              </Link>
            ))}
          </div>
        ))}
      </div>
      <div className={`shrink-0 border-t border-border ${rail ? "p-2" : "p-1.5"}`}>
        <UserButton compact={rail} />
      </div>
    </nav>
  );
}

/** The sidebar's search field, whose search runs on the Search page when it is confirmed. */
function SearchBox({ shortcut }: { shortcut?: string }) {
  const navigate = useNavigate();
  const location = useLocation();
  const q = location.pathname === "/search" ? ((location.search as { q?: string }).q ?? "") : "";
  const [text, setText] = useState(q);
  useEffect(() => setText(q), [q]);
  return (
    <form
      role="search"
      onSubmit={(e) => {
        e.preventDefault();
        if (text.trim()) void navigate({ to: "/search", search: { q: text.trim() } });
      }}
      className="mb-2 flex h-9 items-center gap-2 rounded-[10px] bg-fill px-2.5 type-callout text-text-tertiary"
    >
      <Icon name="search" size={16} />
      <input
        data-search-box
        type="search"
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={t("nav.search")}
        aria-label={t("nav.search")}
        enterKeyHint="search"
        className="min-w-0 flex-1 bg-transparent text-on-background outline-none placeholder:text-text-tertiary"
      />
      {shortcut && !text && <kbd className="font-sans type-caption">{shortcut}</kbd>}
    </form>
  );
}

function itemClass(rail: boolean) {
  return `flex items-center gap-2.5 rounded-[10px] type-callout text-on-background hover:bg-fill data-[status=active]:bg-fill-selected data-[status=active]:font-semibold ${
    rail ? "h-11 w-11 justify-center" : "min-h-9 px-2.5"
  }`;
}

function NavItem({
  rail,
  to,
  icon,
  label,
  exact = false,
}: {
  rail: boolean;
  to: "/" | "/discover" | "/search";
  icon: IconName;
  label: string;
  exact?: boolean;
}): ReactNode {
  return (
    <Link
      to={to}
      activeOptions={{ exact }}
      title={rail ? label : undefined}
      aria-label={rail ? label : undefined}
      className={itemClass(rail)}
    >
      {({ isActive }) => (
        <>
          <Icon name={icon} fill={isActive} size={20} className="shrink-0" />
          {!rail && label}
        </>
      )}
    </Link>
  );
}

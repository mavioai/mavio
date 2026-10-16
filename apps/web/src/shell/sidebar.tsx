// The sidebar of the desktop layout (docs/clients.md §6.3): the wordmark,
// search, Home, the libraries of the shown sign-in and the user at its
// foot. Collapsed, it is a rail of icons.
import { useDiscovery, useLibraries } from "@mavio/client-ts";
import { Link, useLocation, useNavigate } from "@tanstack/react-router";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { Tip } from "../controls/tooltip.tsx";
import { discoverLanguage, t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { shortcutLabel, useShortcutsShown } from "../keys.ts";
import { prefs, setPrefs, sidebarWidths } from "../prefs.ts";
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
      className="flex h-full flex-col border-r border-border bg-sidebar"
    >
      <div className={`flex h-14 shrink-0 items-center ${rail ? "justify-center" : "justify-between pr-2.5 pl-4"}`}>
        {!rail && <Wordmark height={22} />}
        <Tip label={rail ? t("nav.expandSidebar") : t("nav.collapseSidebar")} side={rail ? "right" : "bottom"}>
          <button
            type="button"
            onClick={onToggle}
            aria-label={rail ? t("nav.expandSidebar") : t("nav.collapseSidebar")}
            className="flex h-9 w-9 items-center justify-center rounded-lg text-text-secondary hover:bg-fill hover:text-on-background"
          >
            <Icon name="sidebar" size={20} />
          </button>
        </Tip>
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
              <Tip key={l.id} label={several ? `${l.name} · ${server.name}` : l.name} side="right" disabled={!rail}>
                <Link
                  to="/s/$serverId/library/$libraryId"
                  params={{ serverId: server.id, libraryId: l.id }}
                  aria-label={rail ? l.name : undefined}
                  className={itemClass(rail)}
                >
                  <Icon name={libraryIcon(l.kind)} size={20} className="shrink-0 text-accent-text" />
                  {!rail && <span className="truncate">{l.name}</span>}
                </Link>
              </Tip>
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
    <Tip label={label} side="right" disabled={!rail}>
      <Link to={to} activeOptions={{ exact }} aria-label={rail ? label : undefined} className={itemClass(rail)}>
        {({ isActive }) => (
          <>
            <Icon name={icon} fill={isActive} size={20} className="shrink-0" />
            {!rail && label}
          </>
        )}
      </Link>
    </Tip>
  );
}

/** The width of an expanded sidebar: the one chosen by dragging its edge, else the layout's. */
export const sidebarWidth = "var(--mavio-sidebar-width, var(--mavio-layout-sidebar))";

/**
 * The right edge of an expanded sidebar, dragged or moved with the arrow
 * keys to widen or narrow it within sidebarWidths, and double-clicked to
 * return it to the layout's width. The width is remembered, and shared by
 * the app's sidebar and the settings'.
 */
export function SidebarResizer() {
  const ref = useRef<HTMLDivElement>(null);
  const [dragging, setDragging] = useState(false);
  const current = () => ref.current?.parentElement?.getBoundingClientRect().width ?? 0;
  const resize = (width: number) => setPrefs({ sidebarWidth: Math.round(Math.min(sidebarWidths.max, Math.max(sidebarWidths.min, width))) });
  return (
    <div
      ref={ref}
      role="separator"
      aria-orientation="vertical"
      aria-label={t("nav.resizeSidebar")}
      aria-valuemin={sidebarWidths.min}
      aria-valuemax={sidebarWidths.max}
      aria-valuenow={Math.round(prefs().sidebarWidth ?? current())}
      tabIndex={0}
      className={`absolute inset-y-0 -right-1 z-10 w-2 cursor-col-resize outline-none after:absolute after:inset-y-0 after:left-[3px] after:w-0.5 after:transition-colors hover:after:bg-accent focus-visible:after:bg-accent ${
        dragging ? "after:bg-accent" : ""
      }`}
      onPointerDown={(e) => {
        if (e.button !== 0) return;
        e.preventDefault();
        e.currentTarget.setPointerCapture(e.pointerId);
        const from = e.clientX - current();
        setDragging(true);
        const el = e.currentTarget;
        const move = (m: PointerEvent) => resize(m.clientX - from);
        const up = () => {
          setDragging(false);
          el.removeEventListener("pointermove", move);
          el.removeEventListener("pointerup", up);
          el.removeEventListener("pointercancel", up);
        };
        el.addEventListener("pointermove", move);
        el.addEventListener("pointerup", up);
        el.addEventListener("pointercancel", up);
      }}
      onDoubleClick={() => setPrefs({ sidebarWidth: undefined })}
      onKeyDown={(e) => {
        const step = e.key === "ArrowLeft" ? -10 : e.key === "ArrowRight" ? 10 : 0;
        if (step === 0) return;
        e.preventDefault();
        resize(current() + step);
      }}
    />
  );
}

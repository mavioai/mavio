import { type QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createRootRouteWithContext, HeadContent, Link, Outlet, Scripts, useRouter, useRouterState } from "@tanstack/react-router";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { authClient } from "../lib/auth-client.ts";
import type { Locale } from "../lib/i18n.ts";
import { currentLocale, useLocale, useT } from "../lib/locale.tsx";
import { Wordmark } from "../logo.tsx";
import styles from "../styles.css?url";
import { Avatar, ConfirmProvider, Icon, type IconName } from "../ui.tsx";

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  beforeLoad: (): { locale: Locale } => ({ locale: currentLocale() }),
  head: () => ({
    meta: [
      { charSet: "utf-8" },
      { name: "viewport", content: "width=device-width, initial-scale=1, viewport-fit=cover" },
      { name: "color-scheme", content: "dark light" },
      { title: "Mavio" },
    ],
    links: [{ rel: "stylesheet", href: styles }],
  }),
  component: Root,
});

/** The pages of signing in and of answering a device, which are cards of their own without the site's header. */
const cards = ["/sign-in", "/sign-up", "/forgot-password", "/reset-password", "/verified", "/authorize", "/link"];

function Root() {
  const { queryClient } = Route.useRouteContext();
  const card = useRouterState({ select: (s) => cards.includes(s.location.pathname) });
  return (
    <Document>
      <QueryClientProvider client={queryClient}>
        <ConfirmProvider>
          {card ? (
            <Outlet />
          ) : (
            <div className="flex min-h-dvh flex-col">
              <Header />
              <Outlet />
            </div>
          )}
        </ConfirmProvider>
      </QueryClientProvider>
    </Document>
  );
}

/** A link of the header's navigation, its icon filled when it is the page's. */
function NavLink({ to, icon, children }: { to: "/" | "/home" | "/friends"; icon: IconName; children: ReactNode }) {
  const active = useRouterState({ select: (s) => (to === "/" ? s.location.pathname === "/" || s.location.pathname.startsWith("/servers/") : s.location.pathname === to) });
  return (
    <Link
      to={to}
      aria-current={active ? "page" : undefined}
      className={`inline-flex h-9 items-center gap-1.5 rounded-full px-3.5 type-callout font-semibold transition-colors ${active ? "bg-fill text-on-background" : "text-text-secondary hover:text-on-background"}`}
    >
      <Icon name={icon} fill={active} size={18} />
      {children}
    </Link>
  );
}

/** The header of the account's pages: the wordmark, the navigation and the account's menu. */
function Header() {
  const t = useT();
  const { data } = authClient.useSession();
  const managed = (data?.user as { kind?: string } | undefined)?.kind === "managed";
  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/80 backdrop-blur-xl backdrop-saturate-150">
      <div className="mx-auto flex max-w-[1080px] flex-wrap items-center gap-x-6 gap-y-2 px-margin py-2.5">
        <Link to="/" aria-label="Mavio" className="flex h-9 items-center">
          <Wordmark height={22} />
        </Link>
        {data && (
          <>
            <nav className="order-last -mx-1 flex w-full items-center gap-1 overflow-x-auto sm:order-none sm:mx-0 sm:w-auto sm:flex-1">
              <NavLink to="/" icon="server">
                {t("site.nav.servers")}
              </NavLink>
              <NavLink to="/home" icon="home">
                {t("site.nav.home")}
              </NavLink>
              {!managed && (
                <NavLink to="/friends" icon="users">
                  {t("site.nav.friends")}
                </NavLink>
              )}
            </nav>
            <AccountMenu user={data.user} managed={managed} />
          </>
        )}
      </div>
    </header>
  );
}

/** The account's avatar, opening its menu: its page, claiming a server and signing out. */
function AccountMenu({ user, managed }: { user: { name: string; email: string; username?: string | null }; managed: boolean }) {
  const t = useT();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  useEffect(() => setOpen(false), [pathname]);
  useEffect(() => {
    if (!open) return;
    const away = (e: Event) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    const key = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    document.addEventListener("pointerdown", away);
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("pointerdown", away);
      document.removeEventListener("keydown", key);
    };
  }, [open]);
  const item = "flex min-h-10 w-full items-center gap-2.5 rounded-lg px-2.5 text-start type-body hover:bg-fill";
  const email = user.email.startsWith("managed:") ? undefined : user.email;
  return (
    <div ref={ref} className="relative ml-auto">
      <button
        type="button"
        aria-label={t("site.nav.menu")}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="flex rounded-full transition hover:opacity-80"
      >
        <Avatar name={user.name || user.username || "?"} size={32} />
      </button>
      {open && (
        <div
          role="menu"
          className="absolute top-full right-0 z-50 mt-2 w-64 rounded-[14px] border border-border bg-surface-overlay/90 p-1.5 shadow-level3 backdrop-blur-xl"
        >
          <div className="flex items-center gap-2.5 px-2.5 py-2">
            <Avatar name={user.name || user.username || "?"} size={36} />
            <div className="min-w-0">
              <div className="truncate type-headline">{user.name}</div>
              <div className="truncate type-footnote text-text-secondary">{email ?? `@${user.username}`}</div>
            </div>
          </div>
          <div className="my-1 h-px bg-border" />
          <Link to="/account" role="menuitem" className={item}>
            <Icon name="profile" size={20} />
            {t("site.nav.account")}
          </Link>
          {!managed && (
            <Link to="/claim" role="menuitem" className={item}>
              <Icon name="add" size={20} />
              {t("site.nav.claim")}
            </Link>
          )}
          <div className="my-1 h-px bg-border" />
          <button
            type="button"
            role="menuitem"
            className={item}
            onClick={async () => {
              await authClient.signOut();
              await router.navigate({ to: "/sign-in" });
            }}
          >
            <Icon name="signOut" size={20} />
            {t("site.nav.signOut")}
          </button>
        </div>
      )}
    </div>
  );
}

function Document({ children }: { children: ReactNode }) {
  const locale = useLocale();
  return (
    <html lang={locale}>
      <head>
        <HeadContent />
      </head>
      <body>
        {children}
        <Scripts />
      </body>
    </html>
  );
}

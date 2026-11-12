import { type QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createRootRouteWithContext, HeadContent, Link, Outlet, Scripts, useRouter, useRouterState } from "@tanstack/react-router";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { authClient } from "../lib/auth-client.ts";
import type { Locale } from "../lib/i18n.ts";
import { currentLocale, useLocale, useT } from "../lib/locale.tsx";
import { LanguageSelect } from "../ui.tsx";
import { hasSession, useSignedIn } from "../lib/session.ts";
import { Wordmark } from "../logo.tsx";
import styles from "../styles.css?url";
import { Avatar, ConfirmProvider, Icon, type IconName } from "../ui.tsx";

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  beforeLoad: (): { locale: Locale; chosen?: Locale; session: boolean } => ({ ...currentLocale(), session: hasSession() }),
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

/** Whether a page is one of the documentation's, which draw their own navigation. */
const isDocs = (pathname: string) => pathname === "/docs" || pathname.startsWith("/docs/");

function Root() {
  const { queryClient } = Route.useRouteContext();
  const card = useRouterState({ select: (s) => cards.includes(s.location.pathname) || isDocs(s.location.pathname) });
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
              <Footer />
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

/**
 * The header of the site's pages: the wordmark, and the navigation and the
 * account's menu when signed in, or the ways in when not.
 */
function Header() {
  const t = useT();
  const { data } = authClient.useSession();
  const signedIn = useSignedIn();
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
        {!signedIn && <Visitor />}
      </div>
    </header>
  );
}

/** The header's links for visitors: the documentation, Mavio Pass, signing in and up. */
function Visitor() {
  const t = useT();
  const link = "inline-flex h-9 items-center rounded-full px-3.5 type-callout font-semibold text-text-secondary transition-colors hover:text-on-background";
  return (
    <>
      <nav className="flex items-center gap-1 max-sm:hidden">
        <a href="/docs" className={link}>
          {t("site.nav.docs")}
        </a>
        <Link to="/pass" className={link} activeProps={{ className: "bg-fill text-on-background" }}>
          {t("site.nav.pass")}
        </Link>
      </nav>
      <div className="ml-auto flex items-center gap-1">
        <Link to="/sign-in" className={link}>
          {t("site.action.signIn")}
        </Link>
        <Link to="/sign-up" className="inline-flex h-9 items-center rounded-full bg-button px-4 type-callout font-semibold text-on-button transition hover:opacity-85">
          {t("site.nav.getStarted")}
        </Link>
      </div>
    </>
  );
}

/** The foot of the site's pages: the documentation, Mavio Pass, the policies, the way to write to support and the language. */
function Footer() {
  const t = useT();
  const link = "hover:text-on-background";
  return (
    <footer className="mt-auto border-t border-border">
      <div className="mx-auto flex max-w-[1080px] flex-wrap items-center gap-x-6 gap-y-3 px-margin py-6 type-footnote text-text-secondary">
        <Wordmark height={16} />
        <nav className="flex flex-wrap gap-x-5 gap-y-2">
          <a href="/docs" className={link}>
            {t("site.nav.docs")}
          </a>
          <Link to="/pass" className={link}>
            {t("site.nav.pass")}
          </Link>
          <Link to="/privacy" className={link}>
            {t("site.footer.privacy")}
          </Link>
          <Link to="/terms" className={link}>
            {t("site.footer.terms")}
          </Link>
          <a href="mailto:support@mavio.dev" className={link}>
            {t("site.footer.contact")}
          </a>
        </nav>
        <LanguageSelect className="ml-auto" />
      </div>
    </footer>
  );
}

/** The account's avatar, opening its menu: its page, Mavio Pass, claiming a server, the staff's tools and signing out. */
function AccountMenu({ user, managed }: { user: { name: string; email: string; username?: string | null; role?: string | null }; managed: boolean }) {
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
          <Link to="/pass" role="menuitem" className={item}>
            <Icon name="key" size={20} />
            {t("site.nav.pass")}
          </Link>
          {!managed && (
            <Link to="/claim" role="menuitem" className={item}>
              <Icon name="add" size={20} />
              {t("site.nav.claim")}
            </Link>
          )}
          {user.role === "staff" && (
            <Link to="/staff" role="menuitem" className={item}>
              <Icon name="admin" size={20} />
              {t("site.nav.staff")}
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
  const session = Route.useRouteContext({ select: (c) => c.session });
  return (
    <html lang={locale} data-session={session ? "1" : undefined}>
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

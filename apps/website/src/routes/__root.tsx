import { type QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createRootRouteWithContext, HeadContent, Link, Outlet, Scripts, useRouter } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { authClient } from "../lib/auth-client.ts";
import styles from "../styles.css?url";

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  head: () => ({
    meta: [
      { charSet: "utf-8" },
      { name: "viewport", content: "width=device-width, initial-scale=1" },
      { title: "Mavio" },
    ],
    links: [{ rel: "stylesheet", href: styles }],
  }),
  component: Root,
});

function Root() {
  const { queryClient } = Route.useRouteContext();
  return (
    <Document>
      <QueryClientProvider client={queryClient}>
        <Header />
        <Outlet />
      </QueryClientProvider>
    </Document>
  );
}

function Header() {
  const { data } = authClient.useSession();
  const router = useRouter();
  const link = "type-callout text-text-secondary hover:text-on-background";
  const managed = (data?.user as { kind?: string } | undefined)?.kind === "managed";
  return (
    <header className="flex items-center gap-6 border-b border-border px-6 py-3">
      <Link to="/" className="type-title3 font-bold">
        Mavio
      </Link>
      {data && (
        <nav className="flex flex-1 items-center gap-5">
          <Link to="/" className={link}>
            Servers
          </Link>
          <Link to="/home" className={link}>
            Home
          </Link>
          {!managed && (
            <>
              <Link to="/friends" className={link}>
                Friends
              </Link>
              <Link to="/claim" className={link}>
                Claim a server
              </Link>
            </>
          )}
          <Link to="/account" className={link}>
            Account
          </Link>
          <button
            type="button"
            className={`${link} ml-auto`}
            onClick={async () => {
              await authClient.signOut();
              await router.navigate({ to: "/sign-in" });
            }}
          >
            Sign out
          </button>
        </nav>
      )}
    </header>
  );
}

function Document({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
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

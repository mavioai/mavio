import { isNotFound, keepQueries, Servers, ServersProvider } from "@mavio/client-ts";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { TooltipProvider } from "./controls/tooltip.tsx";
import { EventHub, EventHubProvider } from "./events.tsx";
import { locale } from "./i18n.ts";
import { applyTheme } from "./prefs.ts";
import { router } from "./router.tsx";
import "./styles.css";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // An unreachable server fails fast: Home shows the others meanwhile;
      // what is not there is not asked for again.
      retry: (count, err) => count < 1 && !isNotFound(err),
      refetchOnWindowFocus: false,
    },
  },
});

// The web UI is served by a Mavio server at its root, or as app.mavio.dev
// by no server, built with mavio.dev's API (docs/clients.md §4.1).
const servers = new Servers(localStorage, location.origin, {
  mavioDevUrl: import.meta.env.VITE_MAVIO_DEV_URL || undefined,
  language: locale,
  version: import.meta.env.MAVIO_VERSION as string,
});

const events = new EventHub();

// What Home, the libraries and the details showed is kept in IndexedDB
// for this sign-in, and shown at once next time, offline or while the
// servers answer; another version of the web UI drops it.
const kept = keepQueries(queryClient, servers, { buster: (import.meta.env.MAVIO_VERSION as string | undefined) ?? "" });

document.documentElement.lang = locale;
applyTheme();

// The page waits a little for the kept cache, which IndexedDB reads in
// milliseconds, so that the first screen draws from it.
void Promise.race([kept.restored, new Promise((r) => setTimeout(r, 500))]).then(() =>
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <ServersProvider value={servers}>
        <QueryClientProvider client={queryClient}>
          <EventHubProvider value={events}>
            <TooltipProvider>
              <RouterProvider router={router} />
            </TooltipProvider>
          </EventHubProvider>
        </QueryClientProvider>
      </ServersProvider>
    </StrictMode>,
  ),
);

import { Servers, ServersProvider } from "@mavio/client-ts";
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
      // An unreachable server fails fast: Home shows the others meanwhile.
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

// The web UI is served by a Mavio server at its root, or as app.mavio.dev
// by no server, built with mavio.dev's API (docs/clients.md §4.1).
const servers = new Servers(localStorage, location.origin, { mavioDevUrl: import.meta.env.VITE_MAVIO_DEV_URL || undefined });

const events = new EventHub();

document.documentElement.lang = locale;
applyTheme();

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
);

// What the components drawing items need around them in a story: a router
// for their links, a query client and the servers, none of which is called;
// and made-up items with the generated artwork in ./art.
import { create } from "@bufbuild/protobuf";
import { durationFromMs } from "@bufbuild/protobuf/wkt";
import { ImageKind, type Item, ItemKind, ItemSchema } from "@mavio/proto/mavio/library/v1/item_pb";
import { type Server, Servers, ServersProvider } from "@mavio/client-ts";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryHistory, createRootRoute, createRouter, RouterContextProvider } from "@tanstack/react-router";
import type { Decorator } from "@storybook/react-vite";
import { type ReactNode, useState } from "react";

/** The server the stories' items are on: their artwork is served beside the stories. */
export const server: Server = { url: "" };
export const serverId = "story";

/** A router, a query client and the servers, as the web UI's screens have them. */
function App({ children }: { children: ReactNode }) {
  const [app] = useState(() => ({
    queries: new QueryClient({ defaultOptions: { queries: { retry: false, enabled: false } } }),
    servers: new Servers(sessionStorage, location.origin),
    router: createRouter({ routeTree: createRootRoute(), history: createMemoryHistory() }),
  }));
  return (
    <ServersProvider value={app.servers}>
      <QueryClientProvider client={app.queries}>
        <RouterContextProvider router={app.router}>{children}</RouterContextProvider>
      </QueryClientProvider>
    </ServersProvider>
  );
}

/** Draws the story inside App. */
export const withApp: Decorator = (Story) => (
  <App>
    <Story />
  </App>
);

/** A made-up item: a movie unless kind says otherwise, with artwork when art names one of ./art. */
export function item({
  id,
  name,
  kind = ItemKind.MOVIE,
  year,
  art,
  runtimeMinutes,
  watchedMinutes,
  played = false,
  series,
  season,
  episode,
}: {
  id: string;
  name: string;
  kind?: ItemKind;
  year?: number;
  art?: string;
  runtimeMinutes?: number;
  watchedMinutes?: number;
  played?: boolean;
  series?: string;
  season?: number;
  episode?: number;
}): Item {
  const thumb = art?.startsWith("thumb");
  return create(ItemSchema, {
    id,
    name,
    kind,
    productionYear: year,
    seriesName: series,
    parentIndexNumber: season,
    indexNumber: episode,
    runtime: runtimeMinutes ? durationFromMs(runtimeMinutes * 60_000) : undefined,
    images: art
      ? [{ kind: thumb ? ImageKind.THUMB : ImageKind.PRIMARY, url: `art/${art}.svg`, width: thumb ? 1600 : 1000, height: thumb ? 900 : 1500 }]
      : [],
    userData: { played, position: watchedMinutes ? durationFromMs(watchedMinutes * 60_000) : undefined },
  });
}

import { createRootRoute, createRoute, createRouter, redirect } from "@tanstack/react-router";
import type { Filter, Sort } from "@mavio/client-ts";

import { Details } from "./screens/details.tsx";
import { CatalogScreen, DiscoverScreen, TitleScreen } from "./screens/discover.tsx";
import { HomeScreen } from "./screens/home.tsx";
import { HubScreen } from "./screens/hub.tsx";
import { LibrariesScreen } from "./screens/libraries.tsx";
import { LibraryScreen } from "./screens/library.tsx";
import { PlayerScreen } from "./screens/player.tsx";
import { type RequestFilter, requestFilters, RequestsScreen } from "./screens/requests.tsx";
import { SearchScreen } from "./screens/search.tsx";
import { SettingsScreen, settingsSearch } from "./screens/settings.tsx";
import {
  CallbackScreen,
  InviteScreen,
  type SetupSearch,
  SetupScreen,
  setupSteps,
  type SignInSearch,
  SignInScreen,
  signInSteps,
} from "./screens/sign-in.tsx";
import { Shell } from "./shell/shell.tsx";

const root = createRootRoute({ component: Shell });

const home = createRoute({ getParentRoute: () => root, path: "/", component: HomeScreen });

const signIn = createRoute({
  getParentRoute: () => root,
  path: "/sign-in",
  validateSearch: (s: Record<string, unknown>): SignInSearch => ({
    step: signInSteps.find((x) => x === s.step),
  }),
  component: SignInScreen,
});

const setup = createRoute({
  getParentRoute: () => root,
  path: "/setup",
  validateSearch: (s: Record<string, unknown>): SetupSearch => ({
    step: setupSteps.find((x) => x === s.step),
  }),
  component: SetupScreen,
});

// Where mavio.dev comes back to (docs/accounts.md §6).
const callback = createRoute({ getParentRoute: () => root, path: "/auth/callback", component: CallbackScreen });

// An invitation link, its token in the fragment (docs/accounts.md §4.3).
const invite = createRoute({ getParentRoute: () => root, path: "/invite", component: InviteScreen });

const libraries = createRoute({ getParentRoute: () => root, path: "/libraries", component: LibrariesScreen });

const search = createRoute({
  getParentRoute: () => root,
  path: "/search",
  validateSearch: (s: Record<string, unknown>): { q?: string } => ({
    q: typeof s.q === "string" && s.q !== "" ? s.q : undefined,
  }),
  component: SearchScreen,
});

const settings = createRoute({ getParentRoute: () => root, path: "/settings", validateSearch: settingsSearch, component: SettingsScreen });

const settingsSection = createRoute({
  getParentRoute: () => root,
  path: "/settings/$section",
  validateSearch: settingsSearch,
  component: SettingsScreen,
});

// The profile became the settings.
const profile = createRoute({
  getParentRoute: () => root,
  path: "/profile",
  beforeLoad: () => {
    throw redirect({ to: "/settings" });
  },
});

const sorts: Sort[] = ["name", "dateAdded", "released", "rating", "lastPlayed", "random"];
const filters: Filter[] = ["all", "unplayed", "played", "favorites"];

const library = createRoute({
  getParentRoute: () => root,
  path: "/s/$serverId/library/$libraryId",
  validateSearch: (s: Record<string, unknown>): { tab?: "all"; sort?: Sort; filter?: Filter } => ({
    tab: s.tab === "all" ? "all" : undefined,
    sort: sorts.includes(s.sort as Sort) ? (s.sort as Sort) : undefined,
    filter: filters.includes(s.filter as Filter) && s.filter !== "all" ? (s.filter as Filter) : undefined,
  }),
  component: LibraryScreen,
});

const discover = createRoute({ getParentRoute: () => root, path: "/discover", component: DiscoverScreen });

const catalog = createRoute({
  getParentRoute: () => root,
  path: "/s/$serverId/discover/$catalog",
  validateSearch: (s: Record<string, unknown>): { kind?: "movie" | "series"; tmdb?: number } => ({
    kind: s.kind === "movie" || s.kind === "series" ? s.kind : undefined,
    tmdb: Number(s.tmdb) > 0 ? Number(s.tmdb) : undefined,
  }),
  component: CatalogScreen,
});

const requests = createRoute({
  getParentRoute: () => root,
  path: "/s/$serverId/requests",
  validateSearch: (s: Record<string, unknown>): { status?: RequestFilter } => ({
    status: typeof s.status === "string" && s.status in requestFilters ? (s.status as RequestFilter) : undefined,
  }),
  component: RequestsScreen,
});

const title = createRoute({ getParentRoute: () => root, path: "/s/$serverId/title/$kind/$tmdbId", component: TitleScreen });

const hub = createRoute({ getParentRoute: () => root, path: "/s/$serverId/hub/$hubId", component: HubScreen });

const details = createRoute({ getParentRoute: () => root, path: "/s/$serverId/item/$itemId", component: Details });

const player = createRoute({
  getParentRoute: () => root,
  path: "/s/$serverId/play/$itemId",
  validateSearch: (s: Record<string, unknown>): { start?: number; source?: string } => ({
    start: Number(s.start) > 0 ? Number(s.start) : undefined,
    source: typeof s.source === "string" && s.source !== "" ? s.source : undefined,
  }),
  component: PlayerScreen,
});

export const router = createRouter({
  routeTree: root.addChildren([
    home,
    signIn,
    setup,
    callback,
    invite,
    libraries,
    search,
    discover,
    catalog,
    title,
    requests,
    settings,
    settingsSection,
    profile,
    library,
    hub,
    details,
    player,
  ]),
  scrollRestoration: true,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

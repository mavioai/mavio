import { createRootRoute, createRoute, createRouter, Outlet } from "@tanstack/react-router";

const root = createRootRoute({
  component: () => (
    <div className="min-h-screen bg-background text-on-background">
      <Outlet />
    </div>
  ),
});

const home = createRoute({
  getParentRoute: () => root,
  path: "/",
  component: () => <h1 className="p-6 text-2xl font-semibold">Mavio</h1>,
});

export const router = createRouter({ routeTree: root.addChildren([home]) });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

// app.mavio.dev: the web UI served by no server (docs/clients.md §4.1), built
// with mavio.dev's API and deployed as static assets, its routes falling
// back to index.html.
import { defineConfig } from "cf/config";

export default defineConfig({
  worker: {
    name: "mavio-app",
    compatibilityDate: "2026-09-01",
    domains: ["app.mavio.dev"],
    assets: { notFoundHandling: "single-page-application" },
  },
});

// mavio.dev's Worker (docs/mavio-dev.md §6). The secrets are set with
// `cf workers secrets update`; .env.local holds development's.
import { bindings, defineConfig, exports } from "cf/config";

import * as entrypoint from "./src/server.ts" with { type: "cf-worker" };

const name = "mavio-website";

// The relays' base URLs by region, each the --url of its relay, such as
// { eu: "https://relay-eu.mavio.dev" }; without them there are no relays.
const relays: Record<string, string> = {};

export default defineConfig(({ mode }) => ({
  worker: {
    name,
    entrypoint,
    compatibilityDate: "2026-09-01",
    compatibilityFlags: ["nodejs_compat"],
    domains: ["mavio.dev"],
    env: {
      BASE_URL: bindings.text(mode === "development" ? "http://localhost:5173" : "https://mavio.dev"),
      APP_URL: bindings.text("https://app.mavio.dev"),
      ACME_DOMAIN: bindings.text("acme.mavio.dev"),
      HYPERDRIVE: bindings.hyperdrive({
        id: "mavio-website",
        dev: { connectionString: "postgres://mavio:mavio@localhost:5432/mavio" },
      }),
      NOTICES: bindings.durableObject({ worker: name, exportName: "NoticeHub" }),
      BETTER_AUTH_SECRET: bindings.secret(),
      CLOUDFLARE_ZONE_ID: bindings.secret(),
      CLOUDFLARE_API_TOKEN: bindings.secret(),
      GOOGLE_CA_PROJECT: bindings.secret(),
      GOOGLE_CA_CREDENTIALS: bindings.secret(),
      RELAYS: Object.keys(relays).length > 0 && bindings.json(relays),
      RELAY_TICKET_SECRET: bindings.secret(),
    },
    exports: { NoticeHub: exports.durableObject({ storage: "sqlite" }) },
    observability: { enabled: true },
  },
}));

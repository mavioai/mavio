// mavio.dev's Worker (docs/mavio-dev.md §6). The secrets are set with
// `cf workers secrets update`; .dev.vars holds development's.
import { bindings, defineConfig, exports } from "cf/config";

const name = "mavio-website";

export default defineConfig({
  worker: {
    name,
    entrypoint: "./src/server.ts",
    compatibilityDate: "2026-09-01",
    compatibilityFlags: ["nodejs_compat"],
    domains: ["mavio.dev"],
    env: {
      BASE_URL: bindings.text("https://mavio.dev"),
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
      RELAY_TICKET_SECRET: bindings.secret(),
    },
    exports: { NoticeHub: exports.durableObject({ storage: "sqlite" }) },
    observability: { enabled: true },
  },
});

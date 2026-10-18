import { cloudflare } from "@cloudflare/vite-plugin";
import tailwindcss from "@tailwindcss/vite";
import { tanstackStart } from "@tanstack/react-start/plugin/vite";
import react from "@vitejs/plugin-react";
import { readFileSync } from "node:fs";
import { defineConfig } from "vite";

// In development, DEV_TLS_CERT and DEV_TLS_KEY, PEM files, serve it over
// HTTPS on every interface, so that other devices on the network, whose
// browsers sign in only in secure contexts, reach it at DEV_ORIGIN.
const cert = process.env.DEV_TLS_CERT;
const key = process.env.DEV_TLS_KEY;

// mavio.dev on Cloudflare Workers (docs/mavio-dev.md §2). The Worker
// answers CORS itself, as in production: Vite's own answers only localhost.
export default defineConfig({
  plugins: [cloudflare({ viteEnvironment: { name: "ssr" } }), tanstackStart(), react(), tailwindcss()],
  server: { cors: false, ...(cert && key ? { host: true, https: { cert: readFileSync(cert), key: readFileSync(key) } } : {}) },
});

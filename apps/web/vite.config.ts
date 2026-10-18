import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { readFileSync } from "node:fs";
import { defineConfig } from "vite";

// During development the UI calls the server at MAVIO_SERVER, else the one
// running on its default port.
const server = process.env.MAVIO_SERVER ?? "http://localhost:8686";

// DEV_TLS_CERT and DEV_TLS_KEY, PEM files, serve it over HTTPS on every
// interface, so that other devices on the network, whose browsers sign in
// only in secure contexts, reach it.
const cert = process.env.DEV_TLS_CERT;
const key = process.env.DEV_TLS_KEY;

// The server embeds the build and serves it at / (docs/clients.md §4.1).
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: "../server/internal/webui/dist",
    emptyOutDir: true,
    // The API's generated descriptors and hls.js are large by themselves.
    chunkSizeWarningLimit: 1024,
  },
  server: {
    ...(cert && key ? { host: true, https: { cert: readFileSync(cert), key: readFileSync(key) } } : {}),
    proxy: {
      "/mavio.": server,
      "/signed/": server,
      "/images/": server,
      "/media/": server,
      "/system/": server,
      // The assistant's speech, and the WebSocket it listens on.
      "/assistant/": { target: server, ws: true },
    },
  },
});

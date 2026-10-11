import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// The server embeds the build and serves it at / (docs/clients.md §4.1).
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: "../server/internal/webui/dist",
    emptyOutDir: true,
  },
  server: {
    // During development the UI calls the server running on its default port.
    proxy: { "/mavio.": "http://localhost:8686" },
  },
});

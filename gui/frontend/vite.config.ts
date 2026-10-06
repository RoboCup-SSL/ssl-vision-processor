import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";

// https://vite.dev/config/
export default defineConfig({
  // tailwindcss() must come before svelte() so Tailwind's own PostCSS-less
  // Vite transform sees .svelte files before vite-plugin-svelte compiles them.
  plugins: [tailwindcss(), svelte()],
  build: {
    // One bundle on purpose: the host serves it over the local network, so
    // splitting saves nothing. The default 500 kB warning only flags that.
    chunkSizeWarningLimit: 1000,
  },
  server: {
    // The Go host serves /api and /ws on :8085; everything else (this dev
    // server) is same-origin so no CORS handling is needed on either side.
    proxy: {
      "/api": "http://localhost:8085",
      "/ws": { target: "ws://localhost:8085", ws: true },
    },
  },
});

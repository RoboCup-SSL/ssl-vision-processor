/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";

// https://vite.dev/config/
export default defineConfig({
  // tailwindcss() must come before svelte() so Tailwind's own PostCSS-less
  // Vite transform sees .svelte files before vite-plugin-svelte compiles them.
  plugins: [tailwindcss(), svelte()],
  // Unit tests (npm test): the pure logic in src/lib and scripts/. jsdom
  // stands in for the browser; Svelte's browser build keeps $state and
  // $effect.root working in modules under test.
  resolve: process.env["VITEST"] ? { conditions: ["browser"] } : undefined,
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.ts", "scripts/**/*.test.mjs"],
    setupFiles: ["src/test-setup.ts"],
  },
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

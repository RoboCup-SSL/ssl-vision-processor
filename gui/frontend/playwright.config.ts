// End-to-end smoke tests (e2e/): the real Go host, serving the built frontend,
// driven in Chrome. Run with `make e2e` from gui/, which builds both first.
// Uses the installed Google Chrome (channel "chrome") rather than a
// downloaded browser; GitHub's Ubuntu runners ship it too.
import { defineConfig } from "@playwright/test";

const PORT = 8096;

export default defineConfig({
  testDir: "e2e",
  // One host for the whole run, so tests share its state: each starts from
  // the file on disk (see the beforeEach in smoke.spec.ts).
  workers: 1,
  fullyParallel: false,
  forbidOnly: Boolean(process.env["CI"]),
  reporter: process.env["CI"] ? "github" : "list",
  use: {
    baseURL: `http://127.0.0.1:${String(PORT)}`,
    channel: "chrome",
    trace: "retain-on-failure",
  },
  webServer: {
    command: `node e2e/serve.mjs ${String(PORT)}`,
    url: `http://127.0.0.1:${String(PORT)}/api/health`,
    reuseExistingServer: false,
    timeout: 30_000,
    stdout: "pipe",
  },
});

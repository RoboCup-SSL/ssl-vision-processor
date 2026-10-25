// Starts the Go host for the e2e tests on a scratch copy of the server test
// fixture, so a test can never write to a tracked file, with sample debug
// snapshots for camera 0. The binary is
// GUI_BIN, which `make e2e` builds. Runs from the repo root so the rulebook
// presets (config/legacy/) resolve.
import { spawn } from "node:child_process";
import { copyFileSync, mkdirSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const port = process.argv[2] ?? "8096";
const bin = process.env["GUI_BIN"];

if (!bin) {
  console.error("GUI_BIN is not set; run `make e2e` from gui/");
  process.exit(1);
}

const here = dirname(fileURLToPath(import.meta.url));
const repo = resolve(here, "../../..");
const dir = mkdtempSync(join(tmpdir(), "vision-gui-e2e-"));
const config = join(dir, "vision.yml");

copyFileSync(
  join(repo, "gui/internal/config/testdata/vision.yml"),
  config,
);

// Debug snapshots for camera 0, as vision_processor writes them: the corner
// picker's calibration input and a raw frame.
const img = join(dir, "img");
mkdirSync(img);
for (const view of ["geomcalib_input", "raw"]) {
  copyFileSync(join(repo, "line_corners.png"), join(img, `0.${view}.png`));
}

const host = spawn(
  bin,
  [
    "-address",
    `127.0.0.1:${port}`,
    "-config",
    config,
    "-imgDir",
    img,
    "-logFile",
    "",
    "-logLevel",
    "Warn",
  ],
  { cwd: repo, stdio: "inherit" },
);

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => {
    host.kill(signal);
  });
}

host.on("exit", (code) => {
  process.exit(code ?? 0);
});

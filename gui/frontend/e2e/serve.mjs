// Starts the Go host for the e2e tests on a scratch copy of the server test
// fixture, so a test can never write to a tracked file, with sample debug
// snapshots for camera 0. The binary is
// GUI_BIN, which `make e2e` builds. Runs from the repo root so the rulebook
// presets (config/legacy/) resolve.
import { spawn } from "node:child_process";
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  writeFileSync,
} from "node:fs";
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

// The fixture uses the league's default vision and game controller groups,
// where this host would publish the fixture's calibrations to every real
// GUI host and vision_processor on the network. A private group and ports
// keep it to itself.
const octet = () => 1 + Math.floor(Math.random() * 254);
const group = `239.255.${String(octet())}.${String(octet())}`;
const base = 20000 + Math.floor(Math.random() * 20000);
const fixture = readFileSync(
  join(repo, "gui/internal/config/testdata/vision.yml"),
  "utf8",
);
const isolated = fixture.replace(
  /^defaults:\n/m,
  `defaults:\n  network:\n    vision_ip: ${group}\n    vision_port: ${String(base)}\n    gc_ip: ${group}\n    gc_port: ${String(base + 1)}\n`,
);

if (isolated === fixture) {
  console.error("serve.mjs: no defaults: block in the fixture to isolate");
  process.exit(1);
}

writeFileSync(config, isolated);
console.error(
  `e2e host: vision ${group}:${String(base)}, game controller ${group}:${String(base + 1)}`,
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

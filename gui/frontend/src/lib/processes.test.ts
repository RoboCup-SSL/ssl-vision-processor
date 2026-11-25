import { describe, expect, it } from "vitest";
import {
  lastLine,
  processHealth,
  type ProcessStatus,
} from "./processes.svelte";

const proc = (p: Partial<ProcessStatus>): ProcessStatus => ({
  cameraId: 0,
  configPath: "cam0.yml",
  state: "running",
  since: "2026-11-18T00:00:00Z",
  restarts: 0,
  pendingKeys: [],
  crashes: 0,
  output: [],
  ...p,
});

describe("processHealth", () => {
  it("names each state for a status dot", () => {
    expect(processHealth(proc({}))).toBe("running");
    expect(processHealth(proc({ state: "waiting" }))).toBe("starting");
    expect(processHealth(proc({ state: "waiting", crashes: 1 }))).toBe(
      "starting",
    );
    expect(processHealth(proc({ state: "waiting", crashes: 2 }))).toBe(
      "failing",
    );
    expect(processHealth(proc({ state: "running", crashes: 3 }))).toBe(
      "failing",
    );
    expect(processHealth(proc({ state: "conflict", crashes: 3 }))).toBe("held");
    expect(processHealth(proc({ state: "no_camera", crashes: 3 }))).toBe(
      "noCamera",
    );
  });
});

describe("lastLine", () => {
  it("is the last non-blank line it printed, trimmed", () => {
    expect(
      lastLine(proc({ output: ["a", "  no such device ", "", " "] })),
    ).toBe("no such device");
    expect(lastLine(proc({ output: [] }))).toBe("");
  });
});

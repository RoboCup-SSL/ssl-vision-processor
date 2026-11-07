import { describe, expect, it } from "vitest";
// Every code the host can raise, written by gui/internal/alerts' tests
// (go test ./internal/alerts -run Codes -update).
import codes from "./alert-codes.json";
import { alerts } from "./alerts";
import { hostAlerts } from "./hostAlerts.svelte";

// Params for any code: present() reads only the ones its code names.
const PARAMS = {
  addresses: ["10.0.0.1", "10.0.0.2"],
  address: "224.5.23.2:10006",
  changes: 2,
  color: "yellow",
  error: "",
  everHeard: false,
  file: "robot-heights.yml",
  interface: "lo",
  issue: "noHeight",
  path: "vision.yml",
  problem: "no usable network interface",
  slot: 1,
  socket: "vision",
  team: "",
};

describe("alerts", () => {
  it.each(codes)("words %s itself instead of falling back", (code) => {
    hostAlerts.list = [
      {
        id: code,
        code,
        severity: "warning",
        cameraId: 0,
        params: PARAMS,
        message: "HOST MESSAGE",
      },
    ];

    const [alert] = alerts();
    const fallback = code.replaceAll("_", " ");

    expect(alert?.title).toBeTruthy();
    expect(alert?.title).not.toContain(code);
    expect(alert?.title).not.toContain(fallback);
  });

  it("falls back to the host's message for a code it doesn't know", () => {
    hostAlerts.list = [
      {
        id: "x",
        code: "brand_new",
        severity: "caution",
        message: "Something new happened.",
      },
    ];

    expect(alerts()[0]?.detail).toBe("Something new happened.");
  });
});

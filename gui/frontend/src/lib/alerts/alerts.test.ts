import { describe, expect, it } from "vitest";
// Every code the host can raise, written by gui/internal/alerts' tests
// (go test ./internal/alerts -run Codes -update).
import codes from "./alert-codes.json";
import { alarmLevel, alerts } from "./alerts";
import { hostAlerts, type HostAlert } from "./hostAlerts.svelte";

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
  kind: "calibration",
  senders: ["10.0.0.1:10006 (vp-1)", "10.0.0.2:41234 (gui-host)"],
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

describe("alarmLevel", () => {
  const alert = (severity: HostAlert["severity"]): HostAlert =>
    ({ id: severity, code: severity, severity }) as HostAlert;

  it("is the most severe of errors and warnings, ignoring cautions", () => {
    hostAlerts.list = [];
    expect(alarmLevel()).toBeNull();

    hostAlerts.list = [alert("caution")];
    expect(alarmLevel()).toBeNull();

    hostAlerts.list = [alert("caution"), alert("warning")];
    expect(alarmLevel()).toBe("warning");

    hostAlerts.list = [alert("warning"), alert("error"), alert("caution")];
    expect(alarmLevel()).toBe("error");
  });
});

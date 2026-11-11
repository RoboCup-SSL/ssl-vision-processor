import { flushSync } from "svelte";
import { beforeEach, describe, expect, it } from "vitest";
import { blankField } from "../geometry.svelte";
import {
  config,
  type CameraStatus,
  type Change,
  type ConfigDocument,
} from "../config.svelte";
import { network, type NetworkState } from "../network.svelte";
import { hostAlerts, type HostAlert } from "../alerts/hostAlerts.svelte";
import {
  allItems,
  checklist,
  isTicked,
  itemState,
  pendingBefore,
  restartChecklist,
  setTicked,
} from "./checklist.svelte";

function doc(cameraIds: number[]): ConfigDocument {
  return {
    version: 2,
    field: { ...blankField(), fieldLength: 9000, fieldWidth: 6000 },
    optionalFieldLines: {
      goal2Goal: true,
      halfway: true,
      centerCircle: true,
      penalty: true,
    },
    cameras: cameraIds.map((cameraId) => ({
      cameraId,
      config: { camera: { exposure: 10 }, color: { orange: [1, 2, 3] } },
    })),
  };
}

function state(
  over: {
    path?: string;
    firstRun?: boolean;
    changes?: number;
    cameras?: CameraStatus[];
  } = {},
): void {
  config.state = {
    path: over.path ?? "vision.yml",
    file: over.path ?? "vision.yml",
    firstRun: over.firstRun ?? false,
    revision: 1,
    changes: Array.from(
      { length: over.changes ?? 0 },
      (_, i): Change => ({
        path: `p${String(i)}`,
        jsonPath: `p${String(i)}`,
        before: null,
        after: null,
        section: "camera",
      }),
    ),
    cameras: over.cameras ?? [],
    localCameras: false,
  };
  flushSync();
}

function calibration(
  cameraId: number,
  calibration: CameraStatus["calibration"],
  warning?: string,
): CameraStatus {
  return {
    cameraId,
    calibration,
    liveResolution: [0, 0],
    warnings: warning ? [{ code: warning, message: `${warning} msg` }] : [],
  };
}

function alert(
  code: string,
  severity: HostAlert["severity"] = "warning",
): HostAlert {
  return { id: code, code, severity } as HostAlert;
}

beforeEach(() => {
  localStorage.clear();
  config.doc = doc([0, 1]);
  state();
  checklist.ticks = {};
  checklist.connection = "open";
  network.state = { cameras: [] } as unknown as NetworkState;
  hostAlerts.list = [];
});

describe("automatic items", () => {
  it("save waits for the first save, then for unsaved changes", () => {
    state({ firstRun: true });
    expect(itemState("save")).toMatchObject({
      status: "todo",
      note: { key: "firstRun" },
    });

    state({ changes: 3 });
    expect(itemState("save")).toMatchObject({
      status: "todo",
      note: { key: "unsaved", count: 3 },
    });

    state();
    expect(itemState("save").status).toBe("done");
  });

  it("calibration is done per camera only once locked", () => {
    state({ cameras: [calibration(0, "locked"), calibration(1, "live")] });

    const item = itemState("calibration");
    expect(item.status).toBe("todo");
    expect(item.cameras?.map((c) => c.status)).toEqual(["done", "todo"]);
    expect(item.cameras?.[1]?.note).toEqual({ key: "calibratedNotLocked" });

    state({ cameras: [calibration(0, "locked"), calibration(1, "locked")] });
    expect(itemState("calibration").status).toBe("done");
  });

  it("a calibration that no longer fits the field needs attention", () => {
    state({
      cameras: [
        calibration(0, "locked", "field_changed"),
        calibration(1, "locked"),
      ],
    });

    const item = itemState("calibration");
    expect(item.status).toBe("attention");
    expect(item.cameras?.[0]?.note).toEqual({
      key: "warning",
      message: "field_changed msg",
    });
  });

  it("a host alert puts its item in attention", () => {
    expect(itemState("network").status).toBe("done");

    hostAlerts.list = [alert("socket_problem")];
    expect(itemState("network").status).toBe("attention");
    expect(itemState("layout").status).toBe("done");
  });

  it("a caution leaves its item done", () => {
    hostAlerts.list = [
      alert("loopback_multicast_off", "caution"),
      alert("team_default_height", "caution"),
    ];
    expect(itemState("network").status).toBe("done");
    expect(itemState("teams").status).toBe("done");
  });

  it("a per-camera item with no cameras isn't done", () => {
    config.doc = doc([]);
    expect(itemState("processors")).toMatchObject({
      status: "todo",
      cameras: [],
    });
  });
});

describe("manual items", () => {
  it("ask for a recheck once what they judged changes", () => {
    setTicked("image:0", true);
    expect(itemState("image").cameras?.[0]?.status).toBe("done");

    const camera = config.doc?.cameras[0];
    if (camera?.config) camera.config["camera"] = { exposure: 20 };
    expect(itemState("image").cameras?.[0]?.status).toBe("recheck");
    expect(itemState("image").status).toBe("recheck");

    setTicked("image:0", true);
    expect(itemState("image").cameras?.[0]?.status).toBe("done");
  });

  it("aren't disturbed by settings listed in another order", () => {
    setTicked("field", true);

    if (config.doc) {
      const reversed = Object.fromEntries(
        Object.entries(config.doc.field).reverse(),
      );
      config.doc.field = reversed as typeof config.doc.field;
    }

    expect(itemState("field").status).toBe("done");
  });

  it("untick back to to-do", () => {
    setTicked("liveField", true);
    setTicked("liveField", false);

    expect(isTicked("liveField")).toBe(false);
    expect(itemState("liveField").status).toBe("todo");
  });
});

describe("restart", () => {
  it("clears ticks but not what the host reports", () => {
    state({ cameras: [calibration(0, "locked"), calibration(1, "locked")] });
    setTicked("field", true);

    restartChecklist();

    expect(itemState("field").status).toBe("todo");
    expect(itemState("calibration").status).toBe("done");
    // The first step still to do opens.
    expect(checklist.expanded).toBe("field");
  });
});

describe("ticks", () => {
  it("are kept per config file", () => {
    state({ path: "a.yml" });
    setTicked("field", true);

    state({ path: "b.yml" });
    expect(isTicked("field")).toBe(false);

    state({ path: "a.yml" });
    expect(isTicked("field")).toBe(true);
  });
});

describe("order advice", () => {
  it("names only the earlier steps that aren't done", () => {
    state({ cameras: [calibration(0, "none"), calibration(1, "none")] });

    expect(pendingBefore("calibration", allItems())).toEqual([
      "field",
      "image",
    ]);

    setTicked("field", true);
    expect(pendingBefore("calibration", allItems())).toEqual(["image"]);
  });
});

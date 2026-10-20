import { beforeEach, describe, expect, it } from "vitest";
import { config, type CameraDoc, type ConfigDocument } from "./config.svelte";
import {
  applyPlan,
  cameraCount,
  minCameraCount,
  planCount,
  planMove,
  regionLabel,
  slots,
} from "./cameraLayout";

function camera(id: number, extra: Partial<CameraDoc> = {}): CameraDoc {
  return { cameraId: id, instance: `host-${String(id)}`, ...extra };
}

function doc(cameras: CameraDoc[], layout?: number): ConfigDocument {
  return {
    version: 1,
    field: { fieldLength: 9000, fieldWidth: 6000 },
    optionalFieldLines: {
      goal2Goal: true,
      halfway: true,
      centerCircle: true,
      penalty: true,
    },
    cameras,
    ...(layout ? { layout: { cameraCount: layout } } : {}),
  };
}

beforeEach(() => {
  config.doc = null;
  config.state = null;
});

describe("cameraCount", () => {
  it("rounds the number of cameras up to a power of 2", () => {
    expect(cameraCount(doc([]))).toBe(1);
    expect(cameraCount(doc([camera(0)]))).toBe(1);
    expect(cameraCount(doc([camera(0), camera(1), camera(2)]))).toBe(4);
  });

  it("prefers layout.cameraCount", () => {
    expect(cameraCount(doc([camera(0)], 4))).toBe(4);
  });
});

describe("regions", () => {
  it("names quadrants like camera_ids.png: y first, then x", () => {
    const d = doc([], 4);

    expect([0, 1, 2, 3].map((id) => regionLabel(d, id))).toEqual([
      "−x −y",
      "−x +y",
      "+x −y",
      "+x +y",
    ]);
  });

  it("splits two cameras along the field's length", () => {
    const d = doc([], 2);

    expect([0, 1].map((id) => regionLabel(d, id))).toEqual(["−x", "+x"]);
  });

  it("marks empty regions", () => {
    const s = slots(doc([camera(0), camera(3)], 4));

    expect(s.map((x) => x.severity)).toEqual(["ok", "empty", "empty", "ok"]);
  });
});

describe("planMove and applyPlan", () => {
  it("swaps two cameras, keeping settings and dropping what was region-specific", () => {
    config.doc = doc(
      [
        camera(0, {
          config: { camera: { path: "/dev/video0" } },
          seed: {
            resolution: [1920, 1080],
            lineCorners: [
              [1, 1],
              [2, 2],
              [3, 3],
              [4, 4],
            ],
          },
          calibration: { lockedAt: "", fieldHash: "", camera: {} },
        }),
        camera(3),
      ],
      4,
    );

    const plan = planMove(config.doc, 0, 3);
    expect(plan.map((s) => [s.from, s.to])).toEqual([
      [0, 3],
      [3, 0],
    ]);
    expect(plan[0]).toMatchObject({
      seedStale: true,
      calibrationRemoved: true,
    });

    applyPlan(plan);

    const moved = config.doc.cameras.find((c) => c.instance === "host-0");
    expect(moved?.cameraId).toBe(3);
    expect(moved?.config).toEqual({ camera: { path: "/dev/video0" } });
    expect(moved?.calibration).toBeUndefined();
    expect(moved?.seed?.slot).toEqual({ cameraId: 0, cameraCount: 4 });
    expect(config.doc.cameras.map((c) => c.cameraId)).toEqual([0, 3]);
  });

  it("moves a camera into an empty region without touching others", () => {
    config.doc = doc([camera(0), camera(1)], 4);

    applyPlan(planMove(config.doc, 1, 2));

    expect(config.doc.cameras.map((c) => [c.instance, c.cameraId])).toEqual([
      ["host-0", 0],
      ["host-1", 2],
    ]);
  });

  it("doesn't pin the layout when the count is unchanged", () => {
    config.doc = doc([camera(0), camera(1)]);

    applyPlan(planCount(config.doc, 2), 2);

    expect(config.doc.layout).toBeUndefined();
  });

  it("sets the layout and re-slots every seed when the count changes", () => {
    config.doc = doc([
      camera(0, { seed: { resolution: [0, 0], lineCorners: [] } }),
    ]);

    applyPlan(planCount(config.doc, 4), 4);

    expect(config.doc.layout).toEqual({ cameraCount: 4 });
    expect(config.doc.cameras[0]?.seed?.slot).toEqual({
      cameraId: 0,
      cameraCount: 1,
    });
  });

  it("won't shrink below the highest camera_id", () => {
    expect(minCameraCount(doc([camera(0), camera(3)]))).toBe(4);
    expect(minCameraCount(doc([camera(1)]))).toBe(2);
  });
});

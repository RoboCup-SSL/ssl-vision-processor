import { describe, expect, it } from "vitest";
import { CAMERA_RANGES, toDisplay, toFile, type Range } from "./cameraRanges";

const range = (scale: number): Range => ({
  min: 0,
  max: 100,
  step: 1,
  unit: "",
  scale,
  fallback: 1,
});

// Whether value is a whole number of steps above min, within float error.
function onStep(r: Range, value: number): boolean {
  const steps = (value - r.min) / r.step;
  return Math.abs(steps - Math.round(steps)) < 1e-9;
}

describe("unit conversion", () => {
  it("multiplies by the scale into the file and divides back out", () => {
    expect(toFile(range(1000), 5)).toBe(5000);
    expect(toDisplay(range(1000), 5000)).toBe(5);
  });

  it("rounds away float noise", () => {
    expect(toFile(range(0.001), 166)).toBe(0.166);
    expect(toDisplay(range(3), 1)).toBe(0.333333);
  });

  it("round-trips a display value", () => {
    for (const scale of [1, 0.001, 1000, 0.01])
      for (const v of [0, 0.5, 7, 166, 20000])
        expect(toDisplay(range(scale), toFile(range(scale), v))).toBe(v);
  });
});

describe("CAMERA_RANGES", () => {
  const cases = Object.entries(CAMERA_RANGES).flatMap(([driver, ranges]) =>
    Object.entries(ranges).map(
      ([setting, r]) => [`${driver}.${setting}`, r] as [string, Range],
    ),
  );

  it.each(cases)("%s is a usable slider", (_, r) => {
    expect(r.min).toBeLessThan(r.max);
    expect(r.step).toBeGreaterThan(0);
    expect(r.scale).toBeGreaterThan(0);
    expect(r.step).toBeLessThanOrEqual(r.max - r.min);
  });

  it.each(cases)(
    "%s starts manual mode inside the range, on a step",
    (_, r) => {
      expect(r.fallback).toBeGreaterThanOrEqual(r.min);
      expect(r.fallback).toBeLessThanOrEqual(r.max);
      expect(onStep(r, r.fallback)).toBe(true);
    },
  );

  it("keeps exposure in µs where the driver takes µs, capped at 20 ms", () => {
    for (const driver of ["SPINNAKER", "MVIMPACT", "OPENCV"] as const) {
      expect(CAMERA_RANGES[driver].exposure.unit).toBe("µs");
      expect(CAMERA_RANGES[driver].exposure.max).toBe(20000);
    }
  });
});

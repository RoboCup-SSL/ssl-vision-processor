import { describe, expect, it } from "vitest";
import {
  boxesOverlap,
  CAMERA_COUNT_OPTIONS,
  computeFieldSlice,
  type FieldSlice,
} from "./fieldSplit";

// Division B: 9000 × 6000 mm, centered on the origin.
const L = 9000;
const W = 6000;

const box = (minX: number, minY: number, maxX: number, maxY: number) => ({
  minX,
  minY,
  maxX,
  maxY,
});

const slices = (amount: number, length = L, width = W): FieldSlice[] =>
  Array.from({ length: amount }, (_, id) =>
    computeFieldSlice(id, amount, length, width),
  );

const area = (s: FieldSlice) => (s.maxX - s.minX) * (s.maxY - s.minY);

describe("computeFieldSlice", () => {
  it("gives one camera the whole field", () => {
    expect(computeFieldSlice(0, 1, L, W)).toEqual(
      box(-L / 2, -W / 2, L / 2, W / 2),
    );
  });

  it("treats a camera count below 1 as one camera", () => {
    expect(computeFieldSlice(0, 0, L, W)).toEqual(
      computeFieldSlice(0, 1, L, W),
    );
  });

  it("splits two cameras along the length: -x, then +x", () => {
    expect(slices(2)).toEqual([
      box(-4500, -3000, 0, 3000),
      box(0, -3000, 4500, 3000),
    ]);
  });

  it("splits four cameras into quadrants, -y before +y within each half", () => {
    expect(slices(4)).toEqual([
      box(-4500, -3000, 0, 0),
      box(-4500, 0, 0, 3000),
      box(0, -3000, 4500, 0),
      box(0, 0, 4500, 3000),
    ]);
  });

  it("halves whichever side is longer, so a wide field splits along y", () => {
    expect(slices(2, 4500, 6000)).toEqual([
      box(-2250, -3000, 2250, 0),
      box(-2250, 0, 2250, 3000),
    ]);
  });

  it("wraps a camera id past the count", () => {
    expect(computeFieldSlice(5, 4, L, W)).toEqual(
      computeFieldSlice(1, 4, L, W),
    );
  });

  it.each([1, 2, 4, 8])("tiles the field exactly with %i cameras", (amount) => {
    const all = slices(amount);
    const total = all.reduce((sum, s) => sum + area(s), 0);
    expect(total).toBe(L * W);

    for (const s of all) {
      expect(s.minX).toBeGreaterThanOrEqual(-L / 2);
      expect(s.maxX).toBeLessThanOrEqual(L / 2);
      expect(s.minY).toBeGreaterThanOrEqual(-W / 2);
      expect(s.maxY).toBeLessThanOrEqual(W / 2);
    }
    for (let a = 0; a < all.length; a++)
      for (let b = a + 1; b < all.length; b++) {
        const [p, q] = [all[a], all[b]];
        if (!p || !q) throw new Error("missing slice");
        const overlapX = Math.min(p.maxX, q.maxX) - Math.max(p.minX, q.minX);
        const overlapY = Math.min(p.maxY, q.maxY) - Math.max(p.minY, q.minY);
        expect(overlapX <= 0 || overlapY <= 0).toBe(true);
      }
  });

  it("is only offered power-of-two camera counts, the ones that tile", () => {
    const counts = [...CAMERA_COUNT_OPTIONS.full, ...CAMERA_COUNT_OPTIONS.half];
    for (const n of counts) expect(Math.log2(n) % 1).toBe(0);
  });
});

describe("boxesOverlap", () => {
  const a = box(0, 0, 10, 10);

  it.each([
    ["overlapping", box(5, 5, 15, 15), true],
    ["contained", box(2, 2, 3, 3), true],
    ["containing", box(-5, -5, 20, 20), true],
    ["touching at an edge", box(10, 0, 20, 10), true],
    ["touching at a corner", box(10, 10, 20, 20), true],
    ["apart in x", box(11, 0, 20, 10), false],
    ["apart in y", box(0, -20, 10, -1), false],
  ])("%s: %s", (_, b, want) => {
    expect(boxesOverlap(a, b)).toBe(want);
    expect(boxesOverlap(b, a)).toBe(want);
  });
});

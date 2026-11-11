import { describe, expect, it } from "vitest";
import {
  aroundFirst,
  clockwiseFromFirst,
  goalMarkings,
  homography,
  rotateTo,
  type Segment,
} from "./corners";
import { computeFieldSlice } from "./fieldSplit";
import { blankField } from "./geometry.svelte";

// Clockwise on screen (y down): top left, top right, bottom right, bottom left.
const tl = { x: 10, y: 10 };
const tr = { x: 90, y: 12 };
const br = { x: 88, y: 80 };
const bl = { x: 12, y: 78 };

describe("rotateTo", () => {
  it("puts index first, keeping the order around the polygon", () => {
    expect(rotateTo([tl, tr, br, bl], 2)).toEqual([br, bl, tl, tr]);
    expect(rotateTo([tl, tr, br, bl], 0)).toEqual([tl, tr, br, bl]);
  });
});

describe("aroundFirst", () => {
  it("uncrosses a seed saved with the origin moved to the front", () => {
    // What the picker used to save for origin br: [br, tl, tr, bl].
    expect(aroundFirst([br, tl, tr, bl])).toEqual([br, bl, tl, tr]);
  });

  it("leaves an ordered seed alone", () => {
    expect(aroundFirst([tr, br, bl, tl])).toEqual([tr, br, bl, tl]);
  });
});

describe("clockwiseFromFirst", () => {
  it("keeps the first and runs the rest clockwise on screen", () => {
    expect(clockwiseFromFirst([bl, tl, tr, br])).toEqual([bl, tl, tr, br]);
    expect(clockwiseFromFirst([bl, br, tr, tl])).toEqual([bl, tl, tr, br]);
  });
});

describe("homography", () => {
  it("maps the four corners onto the four points, and lines to lines", () => {
    const field = [
      { x: 0, y: 0 },
      { x: 0, y: 100 },
      { x: 200, y: 100 },
      { x: 200, y: 0 },
    ];
    const image = [bl, tl, tr, br];
    const map = homography(field, image);
    if (!map) throw new Error("no homography");

    field.forEach((p, i) => {
      const q = map(p);
      expect(q?.x).toBeCloseTo(image[i]?.x ?? NaN, 6);
      expect(q?.y).toBeCloseTo(image[i]?.y ?? NaN, 6);
    });
  });

  it("is undefined for points in a line", () => {
    const line = [0, 1, 2, 3].map((i) => ({ x: i, y: i }));
    expect(homography(line, [tl, tr, br, bl])).toBeUndefined();
  });
});

describe("goalMarkings", () => {
  const field = {
    ...blankField(),
    fieldLength: 9000,
    fieldWidth: 6000,
    goalWidth: 1000,
    goalDepth: 180,
    penaltyAreaDepth: 1000,
    penaltyAreaWidth: 2000,
  };

  // The x extents of what's drawn, and whether it reaches past y = 0.
  function extent(segments: Segment[]): {
    minX: number;
    maxX: number;
    minY: number;
    maxY: number;
  } {
    const points = segments.flat();

    return {
      minX: Math.min(...points.map((p) => p.x)),
      maxX: Math.max(...points.map((p) => p.x)),
      minY: Math.min(...points.map((p) => p.y)),
      maxY: Math.max(...points.map((p) => p.y)),
    };
  }

  it("draws both goals for one camera", () => {
    const all = goalMarkings(field, computeFieldSlice(0, 1, 9000, 6000));
    expect(all).toHaveLength(12);
    expect(extent(all)).toEqual({
      minX: -4680,
      maxX: 4680,
      minY: -1000,
      maxY: 1000,
    });
  });

  it("draws one whole goal for each half of two", () => {
    const left = goalMarkings(field, computeFieldSlice(0, 2, 9000, 6000));
    expect(extent(left)).toEqual({
      minX: -4680,
      maxX: -3500,
      minY: -1000,
      maxY: 1000,
    });
  });

  it("draws half a goal at the corner for each quarter of four", () => {
    const quarter = goalMarkings(field, computeFieldSlice(0, 4, 9000, 6000));
    expect(extent(quarter)).toEqual({
      minX: -4680,
      maxX: -3500,
      minY: -1000,
      maxY: 0,
    });
  });
});

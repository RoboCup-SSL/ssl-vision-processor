import { beforeEach, describe, expect, it } from "vitest";
import {
  CANONICAL_COLORS,
  COLOR_CLASSES,
  colorFromDoc,
  MAX_REACHABLE_Y,
  MIN_REACHABLE_Y,
  rgbToCss,
  rgbToYuv,
  writeColorToDoc,
  yuvGamutOverflow,
  yuvInGamut,
  yuvToRgb,
  type ColorConfigData,
  type RGB,
} from "./color.svelte";
import { config, type ConfigDocument } from "./config.svelte";
import { schema, type Schema } from "./schema.svelte";

describe("RGB and YUV", () => {
  it("map black and white to the studio-swing Y limits", () => {
    expect(rgbToYuv({ r: 0, g: 0, b: 0 })).toEqual({
      y: MIN_REACHABLE_Y,
      u: 128,
      v: 128,
    });
    expect(rgbToYuv({ r: 255, g: 255, b: 255 })).toEqual({
      y: MAX_REACHABLE_Y,
      u: 128,
      v: 128,
    });
  });

  it("round-trip every canonical color to within rounding", () => {
    for (const cls of COLOR_CLASSES) {
      const rgb = CANONICAL_COLORS[cls];
      const back = yuvToRgb(rgbToYuv(rgb));

      for (const ch of ["r", "g", "b"] as const) {
        expect(
          Math.abs(back[ch] - rgb[ch]),
          `${cls}.${ch}`,
        ).toBeLessThanOrEqual(3);
      }
    }
  });

  it("round-trip a grid of colors to within rounding", () => {
    for (let r = 0; r <= 255; r += 51)
      for (let g = 0; g <= 255; g += 51)
        for (let b = 0; b <= 255; b += 51) {
          const back = yuvToRgb(rgbToYuv({ r, g, b }));
          expect(
            Math.max(
              Math.abs(back.r - r),
              Math.abs(back.g - g),
              Math.abs(back.b - b),
            ),
          ).toBeLessThanOrEqual(3);
        }
  });

  it("clamp a decode that falls outside 0-255", () => {
    expect(yuvToRgb({ y: 0, u: 128, v: 128 })).toEqual({ r: 0, g: 0, b: 0 });
    expect(yuvToRgb({ y: 255, u: 128, v: 128 })).toEqual({
      r: 255,
      g: 255,
      b: 255,
    });
  });
});

describe("gamut", () => {
  it("puts every RGB camera color within rounding of the gamut", () => {
    for (let r = 0; r <= 255; r += 15)
      for (let g = 0; g <= 255; g += 15)
        for (let b = 0; b <= 255; b += 15) {
          expect(yuvGamutOverflow(rgbToYuv({ r, g, b }))).toBeLessThan(3);
        }
  });

  // Known bug (REVIEW.md T7-8): yuvInGamut has no rounding tolerance, so
  // rgbToYuv's integer rounding leaves ~4% of real colors, every saturated
  // primary and white among them, outside by up to 2.9. Remove .fails once fixed.
  it.fails("holds every color an RGB camera can produce", () => {
    for (const cls of COLOR_CLASSES) {
      expect(yuvInGamut(rgbToYuv(CANONICAL_COLORS[cls])), cls).toBe(true);
    }
    expect(yuvInGamut(rgbToYuv({ r: 255, g: 255, b: 255 }))).toBe(true);
  });

  it("excludes saturated chroma at extreme brightness", () => {
    const c = { y: MAX_REACHABLE_Y, u: 255, v: 255 };

    expect(yuvInGamut(c)).toBe(false);
    expect(yuvGamutOverflow(c)).toBeGreaterThan(0);
  });

  it("measures how far out a point is, so the nearer of two can be picked", () => {
    const near = yuvGamutOverflow({ y: 128, u: 220, v: 128 });
    const far = yuvGamutOverflow({ y: 128, u: 255, v: 128 });

    expect(near).toBeGreaterThan(0);
    expect(far).toBeGreaterThan(near);
  });
});

it("rgbToCss writes a CSS rgb()", () => {
  expect(rgbToCss({ r: 1, g: 22, b: 255 })).toBe("rgb(1, 22, 255)");
});

// The document side: vision_processor's fallbacks (from the schema) under
// defaults.color, under the camera's own color: block.
const FALLBACK: Record<string, unknown> = {
  "color.reference_force": 0.5,
  "color.history_force": 0.3,
  "color.min_reference_force": 0.1,
  "color.yellow": [255, 255, 0],
  "color.blue": [0, 0, 255],
  "color.green": [0, 255, 0],
  "color.pink": [255, 0, 255],
  "color.orange": [255, 128, 0],
  "color.field": [0, 128, 0],
};

function docOf(): ConfigDocument {
  if (!config.doc) throw new Error("no document loaded");
  return config.doc;
}

const noop = (): void => undefined;

const rgb = (r: number, g: number, b: number): RGB => ({ r, g, b });

function camera(): { cameraId: number; config?: Record<string, unknown> } {
  return docOf().cameras[0] as unknown as {
    cameraId: number;
    config?: Record<string, unknown>;
  };
}

beforeEach(() => {
  schema.value = {
    settings: Object.fromEntries(
      Object.entries(FALLBACK).map(([k, v]) => [k, { default: v }]),
    ),
    drivers: [],
  } as unknown as Schema;
  config.doc = { cameras: [{ cameraId: 0 }] } as unknown as ConfigDocument;
});

describe("colorFromDoc", () => {
  it("is vision_processor's fallbacks for a camera with nothing set", () => {
    const c = colorFromDoc(0);

    expect(c.referenceForce).toBe(0.5);
    expect(c.orange).toEqual(rgb(255, 128, 0));
    expect(c.field).toEqual(rgb(0, 128, 0));
  });

  it("layers the shared defaults, then the camera's own overrides", () => {
    docOf().defaults = {
      color: { orange: [200, 100, 0], history_force: 0.4 },
    };
    camera().config = { color: { orange: [210, 90, 10] } };

    const c = colorFromDoc(0);
    expect(c.orange).toEqual(rgb(210, 90, 10));
    expect(c.historyForce).toBe(0.4);
    expect(c.referenceForce).toBe(0.5);
  });

  it("ignores values of the wrong shape", () => {
    camera().config = {
      color: {
        orange: [1, 2],
        field: "green",
        reference_force: "high",
        pink: [1, 2, "3"],
      },
    };

    const c = colorFromDoc(0);
    expect(c.orange).toEqual(rgb(255, 128, 0));
    expect(c.field).toEqual(rgb(0, 128, 0));
    expect(c.referenceForce).toBe(0.5);
    expect(c.pink).toEqual(rgb(255, 0, 255));
  });

  it("returns copies, so editing the result doesn't touch the fallbacks", () => {
    const a = colorFromDoc(0);
    a.orange.r = 1;

    expect(colorFromDoc(0).orange.r).toBe(255);
  });
});

describe("writeColorToDoc", () => {
  const edit = (change: (c: ColorConfigData) => void): void => {
    const c = colorFromDoc(0);
    change(c);
    writeColorToDoc(0, c);
  };

  it("writes nothing when nothing changed", () => {
    edit(noop);

    expect(camera().config).toBeUndefined();
  });

  it("writes only the values that differ from what the camera inherits", () => {
    edit((c) => {
      c.orange = rgb(250, 120, 5);
      c.minReferenceForce = 0.2;
    });

    expect(camera().config).toEqual({
      color: { orange: [250, 120, 5], min_reference_force: 0.2 },
    });
  });

  it("keeps an existing override even when set back to the inherited value", () => {
    camera().config = { color: { orange: [1, 2, 3] } };

    edit((c) => {
      c.orange = rgb(255, 128, 0);
    });

    expect(camera().config).toEqual({ color: { orange: [255, 128, 0] } });
  });

  it("keeps keys the Color page doesn't edit", () => {
    camera().config = {
      color: { something_else: 7 },
      camera: { driver: "OPENCV" },
    };

    edit((c) => {
      c.field = rgb(0, 100, 0);
    });

    expect(camera().config).toEqual({
      color: { field: [0, 100, 0], something_else: 7 },
      camera: { driver: "OPENCV" },
    });
  });

  it("compares against the shared defaults, not just the fallbacks", () => {
    docOf().defaults = { color: { orange: [200, 100, 0] } };

    edit(noop);
    expect(camera().config).toBeUndefined();

    edit((c) => {
      c.orange = rgb(255, 128, 0);
    });
    expect(camera().config).toEqual({ color: { orange: [255, 128, 0] } });
  });

  it("does nothing for a camera that isn't in the document", () => {
    const before = JSON.stringify(config.doc);

    writeColorToDoc(9, colorFromDoc(0));

    expect(JSON.stringify(config.doc)).toBe(before);
  });
});

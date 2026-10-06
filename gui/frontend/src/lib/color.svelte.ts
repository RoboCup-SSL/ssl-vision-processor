// Shared state for the Color config tab. colorConfig.config is an editing
// buffer the color widgets bind to; ColorPanel keeps it in sync with the
// selected camera's color: block in the host's working document (see
// colorFromDoc/writeColorToDoc below), where edits go live.
import { config, cameraDoc } from "./config.svelte";

export interface RGB {
  r: number;
  g: number;
  b: number;
}

export interface YUV {
  y: number;
  u: number;
  v: number;
}

function clamp255(v: number): number {
  return Math.round(Math.min(255, Math.max(0, v)));
}

// Same BT.601 studio-swing coefficients as kernel/rgba2nv12.cl's rgb2nv12
// kernel -- this codebase's own existing RGB->YUV conversion (used to
// encode the RTP H.264 stream), reused here so "YUV" means the same thing
// in the color picker as it already does everywhere else in this repo,
// rather than picking a different, arbitrary YUV variant just for display.
export function rgbToYuv(c: RGB): YUV {
  const { r, g, b } = c;

  return {
    y: clamp255((66 * r + 129 * g + 25 * b) / 256 + 16),
    u: clamp255((-38 * r - 74 * g + 112 * b) / 256 + 128),
    v: clamp255((112 * r - 94 * g - 18 * b) / 256 + 128),
  };
}

// The standard BT.601 studio-swing decode -- the inverse of rgbToYuv above.
export function yuvToRgb(c: YUV): RGB {
  const y = c.y - 16;
  const u = c.u - 128;
  const v = c.v - 128;

  return {
    r: clamp255((298 * y + 409 * v + 128) / 256),
    g: clamp255((298 * y - 100 * u - 208 * v + 128) / 256),
    b: clamp255((298 * y + 516 * u + 128) / 256),
  };
}

// Most (Y,U,V) points don't correspond to any real RGB color -- at a given Y,
// only a hexagon-ish region of the U/V square is reachable, the rest requires
// clipping. yuvToRgb hides that by clamping; this checks the *unclamped*
// decode against [0,255] so a caller can tell reachable points from clipped
// ones (YuvPositionPane.svelte shades the unreachable ones on its canvas,
// same convention as a CIE chromaticity diagram graying out everything
// outside the sRGB triangle).
export function yuvInGamut(c: YUV): boolean {
  return yuvGamutOverflow(c) === 0;
}

// How far the unclamped decode lands outside [0,255], summed over R/G/B --
// 0 means in gamut. Lets a caller pick the "least unreachable" Y for a U/V
// that isn't reachable at any brightness, not just test yes/no.
export function yuvGamutOverflow(c: YUV): number {
  const y = c.y - 16;
  const u = c.u - 128;
  const v = c.v - 128;

  const r = (298 * y + 409 * v + 128) / 256;
  const g = (298 * y - 100 * u - 208 * v + 128) / 256;
  const b = (298 * y + 516 * u + 128) / 256;

  const over = (ch: number): number => (ch < 0 ? -ch : ch > 255 ? ch - 255 : 0);
  return over(r) + over(g) + over(b);
}

// The only Y values any real RGB (0-255 each) can ever produce -- see
// rgbToYuv: Y = (66R+129G+25B)/256+16 hits its extremes at black (16) and
// white (~235). Values outside this are the "superblack"/"superwhite" range
// broadcast video reserves for sync signals, not a reachable camera color, so
// a Y control has nothing valid to show past these bounds.
export const MIN_REACHABLE_Y = 16;
export const MAX_REACHABLE_Y = 235;

export const COLOR_CLASSES = [
  "yellow",
  "blue",
  "green",
  "pink",
  "orange",
  "field",
] as const;

export type ColorClass = (typeof COLOR_CLASSES)[number];

// "Shell" is the colored marker disc on top of a robot -- yellow/blue mark
// which shell is which team (both centered), green/pink mark orientation
// (both to the side of center). Shared between ColorPanel and
// YuvPositionPane, which both render a class selector.
export const CLASS_LABELS: Record<ColorClass, string> = {
  yellow: "Yellow (shell, center)",
  blue: "Blue (shell, center)",
  green: "Green (shell, side)",
  pink: "Pink (shell, side)",
  orange: "Orange (ball)",
  field: "Field (carpet)",
};

export function rgbToCss(c: RGB): string {
  return `rgb(${String(c.r)}, ${String(c.g)}, ${String(c.b)})`;
}

// Each class's "identity" color, not its tunable reference value -- used
// only when showing multiple classes' markers on the same YUV pane at once
// (YuvPositionPane's "All" view), where six markers all drawn in their own
// current reference color would frequently collide or be hard to tell apart
// (two classes can easily sit close together in U/V, and a config gone
// slightly astray could put one right on top of another). A fixed, primary
// color per class is stable regardless of what's actually configured, so the
// legend never changes shape. field is the one non-marker class (carpet, not
// a robot/ball color) and gets neutral gray rather than a forced primary.
export const CANONICAL_COLORS: Record<ColorClass, RGB> = {
  yellow: { r: 255, g: 255, b: 0 },
  blue: { r: 0, g: 0, b: 255 },
  green: { r: 0, g: 255, b: 0 },
  pink: { r: 255, g: 0, b: 255 },
  orange: { r: 255, g: 165, b: 0 },
  field: { r: 128, g: 128, b: 128 },
};

// One-letter marker badges for the "All" view -- a color-independent
// fallback identifier (colorblindness, or two canonical colors landing close
// together) alongside the fill color itself.
export const CLASS_LETTERS: Record<ColorClass, string> = {
  yellow: "Y",
  blue: "B",
  green: "G",
  pink: "P",
  orange: "O",
  field: "F",
};

export interface ColorConfigData {
  referenceForce: number;
  historyForce: number;
  minReferenceForce: number;
  orange: RGB;
  field: RGB;
  yellow: RGB;
  blue: RGB;
  green: RGB;
  pink: RGB;
}

// Matches vision_processor's own Resources.cpp fallbacks (see
// colorReferenceDefaults in internal/config/color.go) -- what a camera runs
// for anything neither it nor the shared defaults set.
export function defaultColorConfig(): ColorConfigData {
  return {
    referenceForce: 0.1,
    historyForce: 0.7,
    minReferenceForce: 0.05,
    orange: { r: 192, g: 128, b: 64 },
    field: { r: 128, g: 128, b: 128 },
    yellow: { r: 255, g: 128, b: 0 },
    blue: { r: 0, g: 128, b: 255 },
    green: { r: 0, g: 255, b: 128 },
    pink: { r: 255, g: 0, b: 128 },
  };
}

export const colorConfig = $state<{ config: ColorConfigData }>({
  config: defaultColorConfig(),
});

// Whether the minimum-reference-weight floor has been unlocked for editing
// this session (see WeightTriangle.svelte's unlock-confirmation popup).
// Module-level, not component-local state: MainContent.svelte destroys and
// recreates ColorPanel/WeightTriangle every time the user switches config
// tabs away and back ({#if}/{:else if} on nav.selectedCategoryId), so a
// component-local flag would silently re-lock on every tab switch --
// defeating "stays unlocked for the remainder of the session." Resets only
// on a full page reload, since it's never persisted anywhere.
export const minReferenceForceUnlock = $state({ unlocked: false });

// config.yml's own key for each force field.
const FORCE_KEYS = {
  referenceForce: "reference_force",
  historyForce: "history_force",
  minReferenceForce: "min_reference_force",
} as const;

type Block = Record<string, unknown>;

function asBlock(value: unknown): Block {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? (value as Block)
    : {};
}

function fromBlock(block: Block, base: ColorConfigData): ColorConfigData {
  const out: ColorConfigData = {
    ...base,
    ...Object.fromEntries(COLOR_CLASSES.map((cls) => [cls, { ...base[cls] }])),
  };

  for (const [field, key] of Object.entries(FORCE_KEYS) as [
    keyof typeof FORCE_KEYS,
    string,
  ][]) {
    const value = block[key];
    if (typeof value === "number") out[field] = value;
  }

  for (const cls of COLOR_CLASSES) {
    const value = block[cls];
    if (
      Array.isArray(value) &&
      value.length === 3 &&
      value.every((n) => typeof n === "number")
    ) {
      const [r, g, b] = value as [number, number, number];
      out[cls] = { r, g, b };
    }
  }

  return out;
}

// What a camera runs before its own overrides: vision_processor's built-in
// fallbacks under the document's shared defaults.color.
function inheritedColor(): ColorConfigData {
  return fromBlock(
    asBlock(asBlock(config.doc?.defaults)["color"]),
    defaultColorConfig(),
  );
}

// The camera's effective color settings.
export function colorFromDoc(cameraId: number): ColorConfigData {
  return fromBlock(
    asBlock(asBlock(cameraDoc(cameraId)?.config)["color"]),
    inheritedColor(),
  );
}

// Writes data as the camera's own color: overrides, keeping the block
// minimal: a value equal to what the camera inherits is only written if the
// camera already overrode it, so an untouched color doesn't show up as a
// change to save.
export function writeColorToDoc(cameraId: number, data: ColorConfigData): void {
  const camera = cameraDoc(cameraId);
  if (!camera) return;

  const existing = asBlock(asBlock(camera.config)["color"]);
  const inherited = inheritedColor();
  const block: Block = {};

  const put = (key: string, value: unknown, inheritedValue: unknown): void => {
    if (
      key in existing ||
      JSON.stringify(value) !== JSON.stringify(inheritedValue)
    ) {
      block[key] = value;
    }
  };

  for (const [field, key] of Object.entries(FORCE_KEYS) as [
    keyof typeof FORCE_KEYS,
    string,
  ][]) {
    put(key, data[field], inherited[field]);
  }

  for (const cls of COLOR_CLASSES) {
    const { r, g, b } = data[cls];
    const base = inherited[cls];
    put(cls, [r, g, b], [base.r, base.g, base.b]);
  }

  // Keys this panel doesn't edit ride along unchanged.
  for (const [key, value] of Object.entries(existing)) {
    if (!(key in block)) block[key] = value;
  }

  if (JSON.stringify(block) === JSON.stringify(existing)) return;

  camera.config ??= {};

  if (Object.keys(block).length === 0) {
    delete camera.config["color"];
  } else {
    camera.config["color"] = block;
  }
}

// Shared state for the Color config tab, same pattern as lineCorners.svelte.ts:
// a module-level $state object plus load/save built on api.ts's shared
// request/loading-state helpers. Mirrors gui's ColorConfig (see
// internal/geometry/colorconfig.go) field-for-field -- this is the JSON
// shape /api/config/color actually sends and accepts.
import { requestJSON, withLoadingState } from "./api";

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
  const y = c.y - 16;
  const u = c.u - 128;
  const v = c.v - 128;

  const r = (298 * y + 409 * v + 128) / 256;
  const g = (298 * y - 100 * u - 208 * v + 128) / 256;
  const b = (298 * y + 516 * u + 128) / 256;

  return r >= 0 && r <= 255 && g >= 0 && g <= 255 && b >= 0 && b <= 255;
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
// colorReferenceDefaults in colorconfig.go) so the panel shows something
// sane for the instant before the first load response arrives, rather than
// black/zeroed swatches.
function defaultColorConfig(): ColorConfigData {
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

export const colorConfig = $state<{
  config: ColorConfigData;
  loading: boolean;
  saving: boolean;
  error: string | null;
  savedAt: number | null;
}>({
  config: defaultColorConfig(),
  loading: false,
  saving: false,
  error: null,
  savedAt: null,
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

// Loads the instance's config.yml color: block (see
// geometry.ReadColorConfig) -- always a complete, concrete config, since
// every color/force resolves to either what's set or vision_processor's own
// default, never "unset."
export async function loadColorConfig(): Promise<void> {
  await withLoadingState(
    (v) => (colorConfig.loading = v),
    (v) => (colorConfig.error = v),
    async () => {
      const response = await requestJSON("/api/config/color");
      colorConfig.config = (await response.json()) as ColorConfigData;
    },
  );
}

// Saves the given config to the instance's config.yml (see
// geometry.WriteColorConfig). Server-side validation (force ranges, the
// minReferenceForce floor, 0-255 channels) can reject this -- the error
// message from requestJSON is the backend's own, surfaced as-is.
export async function saveColorConfig(cfg: ColorConfigData): Promise<void> {
  colorConfig.savedAt = null;

  await withLoadingState(
    (v) => (colorConfig.saving = v),
    (v) => (colorConfig.error = v),
    async () => {
      await requestJSON("/api/config/color", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(cfg),
      });

      colorConfig.config = cfg;
      colorConfig.savedAt = Date.now();
    },
  );
}

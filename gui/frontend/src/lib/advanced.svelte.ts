// vision_processor's thresholds: and tracking: settings, edited in vision.yml's
// defaults so every camera shares them. Defaults and restart-only keys are
// from src/Resources.cpp (applyTunables reloads the rest live).
import { without } from "./util";
import { config } from "./config.svelte";
import type { advanced } from "./text/advanced";

// Every setting has a label and notes in text/advanced.ts.
export type AdvancedKey = keyof typeof advanced.fields;

export type Block = "thresholds" | "tracking";

export interface AdvancedSetting {
  block: Block;
  key: AdvancedKey;
  // vision_processor's fallback when the key is unset.
  fallback: number;
  // The slider's range; the number box takes anything valid.
  min: number;
  max: number;
  step: number;
  unit?: string;
  // Read only at startup (Resources' constructor), not on reload.
  restart?: boolean;
}

export const ADVANCED: Record<
  "blobs" | "tolerances" | "tracking",
  AdvancedSetting[]
> = {
  blobs: [
    {
      block: "thresholds",
      key: "circularity",
      fallback: 15,
      min: 0,
      max: 60,
      step: 0.5,
    },
    {
      block: "thresholds",
      key: "score",
      fallback: 5,
      min: 0,
      max: 20,
      step: 0.5,
    },
    {
      block: "thresholds",
      key: "min_confidence",
      fallback: 0.2,
      min: 0,
      max: 1,
      step: 0.01,
    },
    {
      block: "thresholds",
      key: "resampling_factor",
      fallback: 1,
      min: 0.25,
      max: 2,
      step: 0.05,
      unit: "×",
    },
    {
      block: "thresholds",
      key: "blobs",
      fallback: 2000,
      min: 100,
      max: 10000,
      step: 100,
      restart: true,
    },
  ],
  tolerances: [
    {
      block: "thresholds",
      key: "min_cam_edge_distance",
      fallback: 170,
      min: 0,
      max: 500,
      step: 5,
      unit: "mm",
    },
    {
      block: "thresholds",
      key: "clipping_tolerance",
      fallback: 10,
      min: 0,
      max: 50,
      step: 1,
      unit: "mm",
    },
    {
      block: "thresholds",
      key: "geometry_tolerance",
      fallback: 10,
      min: 0,
      max: 100,
      step: 1,
      unit: "mm",
      restart: true,
    },
  ],
  tracking: [
    {
      block: "tracking",
      key: "min_tracking_radius",
      fallback: 20,
      min: 0,
      max: 200,
      step: 5,
      unit: "mm",
    },
    {
      block: "tracking",
      key: "max_bot_acceleration",
      fallback: 6.5,
      min: 0,
      max: 20,
      step: 0.5,
      unit: "m/s²",
    },
  ],
};

function block(name: Block): Record<string, unknown> | undefined {
  const b = config.doc?.defaults?.[name];

  return b && typeof b === "object"
    ? (b as Record<string, unknown>)
    : undefined;
}

// The shared value, or undefined when vision_processor's fallback applies.
export function advancedValue(s: AdvancedSetting): number | undefined {
  const v = block(s.block)?.[s.key];

  return typeof v === "number" ? v : undefined;
}

// Sets the shared value; undefined removes it, back to the fallback. An
// emptied block is removed too, so vision.yml stays minimal.
export function setAdvanced(
  s: AdvancedSetting,
  value: number | undefined,
): void {
  const doc = config.doc;
  if (!doc) return;

  const current = block(s.block) ?? {};
  const next =
    value === undefined
      ? without(current, s.key)
      : {
          ...current,
          [s.key]: s.key === "blobs" ? Math.round(value) : value,
        };

  const defaults = doc.defaults ?? {};
  doc.defaults =
    Object.keys(next).length === 0
      ? without(defaults, s.block)
      : { ...defaults, [s.block]: next };
}

// Cameras whose own config sets any of these, so they ignore the shared value.
export function advancedOverrides(): number[] {
  return (config.doc?.cameras ?? [])
    .filter(
      (c) =>
        c.config?.["thresholds"] !== undefined ||
        c.config?.["tracking"] !== undefined,
    )
    .map((c) => c.cameraId);
}

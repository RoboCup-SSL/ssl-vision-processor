// vision_processor's thresholds:, tracking: and debug: settings, edited in
// vision.yml's defaults so every camera shares them. Fallbacks, ranges, and which keys need
// a restart come from the host's schema.
import { without } from "./util";
import { config } from "./config.svelte";
import { setting } from "./schema.svelte";
import type { advanced } from "./text/advanced";

// Every setting has a label and notes in text/advanced.ts.
export type AdvancedKey = keyof typeof advanced.fields;

export type Block = "thresholds" | "tracking" | "debug";

const BLOCKS: Block[] = ["thresholds", "tracking", "debug"];

export interface AdvancedSetting {
  block: Block;
  key: AdvancedKey;
}

// How each card groups the settings.
export const ADVANCED: Record<
  "blobs" | "tolerances" | "tracking" | "diagnostics",
  AdvancedSetting[]
> = {
  blobs: [
    { block: "thresholds", key: "circularity" },
    { block: "thresholds", key: "score" },
    { block: "thresholds", key: "min_confidence" },
    { block: "thresholds", key: "resampling_factor" },
    { block: "thresholds", key: "blobs" },
  ],
  tolerances: [
    { block: "thresholds", key: "min_cam_edge_distance" },
    { block: "thresholds", key: "clipping_tolerance" },
    { block: "thresholds", key: "geometry_tolerance" },
  ],
  tracking: [
    { block: "tracking", key: "min_tracking_radius" },
    { block: "tracking", key: "max_bot_acceleration" },
  ],
  // debug:'s other keys stay hand-edited in vision.yml: ground_truth is for
  // the benchmarks, debug_images' files lack a camera prefix, and wait_for_geometry
  // hangs a first run (nothing is published before the first save).
  diagnostics: [{ block: "debug", key: "debug_stream_interval_ms" }],
};

export interface AdvancedSpec {
  // vision_processor's fallback when the key is unset.
  fallback: number;
  // The slider's range; the number box takes anything valid.
  min: number;
  max: number;
  step: number;
  unit?: string;
  // Read only at startup, not on reload.
  restart: boolean;
}

// What the schema says about s.
export function advancedSpec(s: AdvancedSetting): AdvancedSpec {
  const spec = setting(`${s.block}.${s.key}`);
  const range = spec.range ?? { min: 0, max: 100, step: 1 };

  return {
    fallback: spec.default as number,
    ...range,
    unit: spec.unit,
    restart: spec.apply === "restart",
  };
}

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

  const integer = setting(`${s.block}.${s.key}`).type === "integer";
  const current = block(s.block) ?? {};
  const next =
    value === undefined
      ? without(current, s.key)
      : { ...current, [s.key]: integer ? Math.round(value) : value };

  const defaults = doc.defaults ?? {};
  doc.defaults =
    Object.keys(next).length === 0
      ? without(defaults, s.block)
      : { ...defaults, [s.block]: next };
}

// Cameras whose own config sets any of these, so they ignore the shared value.
export function advancedOverrides(): number[] {
  return (config.doc?.cameras ?? [])
    .filter((c) => BLOCKS.some((b) => c.config?.[b] !== undefined))
    .map((c) => c.cameraId);
}

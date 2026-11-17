// A camera's field line detection settings: config.yml's geometry: block,
// minus what the host derives (camera_amount, line_corners). Per camera, like
// the camera: block in cameraSettings.svelte.ts: the camera's own value, else
// vision.yml's defaults.geometry, else vision_processor's fallback. Edits go
// into the camera's own block, and setting a key back to what it would
// inherit removes it.
//
// vision_processor uses these only while it calibrates: at startup, and only
// when the host isn't publishing a locked calibration for the camera.
import { config, cameraDoc } from "./config.svelte";
import { fallback, setting } from "./schema.svelte";

export type LineKey =
  | "field_line_threshold"
  | "min_line_segment_length"
  | "max_line_segment_offset"
  | "max_line_segment_angle";

export const LINE_KEYS: LineKey[] = [
  "field_line_threshold",
  "min_line_segment_length",
  "max_line_segment_offset",
  "max_line_segment_angle",
];

export type GeometryKey = LineKey | "refinement";

export interface LineSpec {
  min: number;
  max: number;
  step: number;
  unit?: string;
  fallback: number;
}

type Block = Record<string, unknown>;

function asBlock(value: unknown): Block {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Block)
    : {};
}

function inheritedBlock(): Block {
  return asBlock(config.doc?.defaults?.["geometry"]);
}

function ownBlock(cameraId: number): Block {
  return asBlock(asBlock(cameraDoc(cameraId)?.config)["geometry"]);
}

function inheritedValue(key: GeometryKey): unknown {
  const block = inheritedBlock();

  return key in block ? block[key] : fallback(`geometry.${key}`);
}

export function geometryValue(cameraId: number, key: GeometryKey): unknown {
  const own = ownBlock(cameraId);

  return key in own ? own[key] : inheritedValue(key);
}

// Whether the camera sets key itself rather than inheriting it.
export function overridden(cameraId: number, key: GeometryKey): boolean {
  return key in ownBlock(cameraId);
}

export function lineSpec(key: LineKey): LineSpec {
  const spec = setting(`geometry.${key}`);
  const range = spec.range ?? { min: 0, max: 100, step: 1 };

  return {
    min: range.min,
    max: range.max,
    step: range.step,
    unit: spec.unit,
    fallback: spec.default as number,
  };
}

// Sets key for the camera; undefined removes its override, so it inherits.
export function setGeometryKey(
  cameraId: number,
  key: GeometryKey,
  value: number | boolean | undefined,
): void {
  const camera = cameraDoc(cameraId);
  if (!camera) return;

  const own = ownBlock(cameraId);

  if (value === undefined || value === inheritedValue(key)) {
    if (!(key in own)) return;

    writeBlock(
      cameraId,
      Object.fromEntries(Object.entries(own).filter(([k]) => k !== key)),
    );

    return;
  }

  if (own[key] === value) return;

  writeBlock(cameraId, { ...own, [key]: value });
}

function writeBlock(cameraId: number, block: Block): void {
  const camera = cameraDoc(cameraId);
  if (!camera) return;

  camera.config ??= {};

  if (Object.keys(block).length === 0) {
    delete camera.config["geometry"];
  } else {
    camera.config["geometry"] = block;
  }
}

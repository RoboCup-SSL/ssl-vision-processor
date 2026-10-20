// Camera Layout: which physical camera (host + device) covers which region
// of the field. A region is a slot: camera_id out of the camera count, split
// as in src/CameraModel.cpp's visibleFieldExtentEstimation (camera_ids.png).
// Any mix of hosts and cameras is valid; one host may run several.
import {
  config,
  cameraStatus,
  type CameraDoc,
  type ConfigDocument,
} from "./config.svelte";
import { computeFieldSlice, type FieldSlice } from "./fieldSplit";
import { network, type DetectionSource } from "./network.svelte";

// Detection sources heard for cameraId, receiving ones first.
export function liveSources(cameraId: number): DetectionSource[] {
  return (network.state?.cameras ?? [])
    .filter((s) => s.cameraId === cameraId)
    .sort((a, b) => Number(b.receiving) - Number(a.receiving));
}

// "10.0.0.5, 30 fps", "silent (10.0.0.5)", or "not heard".
export function liveLabel(sources: DetectionSource[]): string {
  const receiving = sources.filter((s) => s.receiving);
  if (receiving.length > 0) {
    return receiving
      .map((s) => `${s.address}, ${String(s.fps)} fps`)
      .join("; ");
  }

  const silent = sources[0];

  return silent ? `silent (${silent.address})` : "not heard";
}

// camera_ids heard with no camera in the document, e.g. a processor whose
// cam_id was never added here or was set by hand to a free slot.
export function unplacedSources(doc: ConfigDocument): DetectionSource[] {
  const known = new Set(doc.cameras.map((c) => c.cameraId));

  return (network.state?.cameras ?? []).filter((s) => !known.has(s.cameraId));
}

// Matches gui/internal/config's CameraCounts.
export const CAMERA_COUNTS = [1, 2, 4];

// Matches gui/internal/config's Document.CameraCount.
export function cameraCount(doc: ConfigDocument): number {
  if (doc.layout && doc.layout.cameraCount > 0) return doc.layout.cameraCount;

  let n = 1;
  while (n < doc.cameras.length) n *= 2;

  return n;
}

export function slotSlice(doc: ConfigDocument, cameraId: number): FieldSlice {
  return computeFieldSlice(
    cameraId,
    cameraCount(doc),
    doc.field.fieldLength ?? 0,
    doc.field.fieldWidth ?? 0,
  );
}

// "−x +y" style name for a region: the side of each axis it's on, or
// "whole field" when it isn't split at all.
export function regionLabel(doc: ConfigDocument, cameraId: number): string {
  const s = slotSlice(doc, cameraId);
  const length = doc.field.fieldLength ?? 0;
  const width = doc.field.fieldWidth ?? 0;
  const parts: string[] = [];

  if (s.maxX - s.minX < length) parts.push(s.minX + s.maxX < 0 ? "−x" : "+x");
  if (s.maxY - s.minY < width) parts.push(s.minY + s.maxY < 0 ? "−y" : "+y");

  return parts.length > 0 ? parts.join(" ") : "whole field";
}

// The device a camera opens: the camera: block from defaults with the
// camera's own override over it.
export interface Device {
  driver?: string;
  path?: string;
  id?: number;
}

export function cameraDevice(doc: ConfigDocument, camera: CameraDoc): Device {
  const base = doc.defaults?.["camera"] as Device | undefined;
  const own = camera.config?.["camera"] as Device | undefined;

  return { ...base, ...own };
}

// Short device name for tight spaces: the last path segment (a by-id link's
// name), or the driver's camera index.
export function deviceLabel(device: Device): string {
  if (device.path) return device.path.split("/").pop() ?? device.path;
  if (device.id !== undefined) return `id ${String(device.id)}`;

  return "default device";
}

export function hostName(camera: CameraDoc): string {
  return camera.instance === undefined || camera.instance === ""
    ? "(no host)"
    : camera.instance;
}

// One color per host, so a split like "host-a covers both left regions"
// reads at a glance.
// Tailwind classes from the theme (gui/CLAUDE.md: no hex in components);
// written out whole so Tailwind generates each. The first is the GUI's
// primary.
const HOST_COLORS = [
  "bg-primary-700",
  "bg-amber-600",
  "bg-green-600",
  "bg-fuchsia-600",
  "bg-rose-600",
  "bg-indigo-600",
];

// A background class for host's dot.
export function hostColor(doc: ConfigDocument, host: string): string {
  const hosts = [...new Set(doc.cameras.map((c) => c.instance ?? ""))].sort();

  return HOST_COLORS[hosts.indexOf(host) % HOST_COLORS.length] ?? "bg-gray-400";
}

export type Severity = "ok" | "warning" | "error" | "empty";

export interface SlotInfo {
  cameraId: number;
  slice: FieldSlice;
  region: string;
  camera?: CameraDoc;
  sources: DetectionSource[];
  // More than one address is sending this camera_id right now.
  conflict: boolean;
  severity: Severity;
}

export function slots(doc: ConfigDocument): SlotInfo[] {
  return Array.from({ length: cameraCount(doc) }, (_, cameraId) => {
    const camera = doc.cameras.find((c) => c.cameraId === cameraId);
    const warnings = cameraStatus(cameraId)?.warnings ?? [];
    const sources = liveSources(cameraId);
    const conflict = sources.filter((s) => s.receiving).length > 1;

    let severity: Severity = "ok";
    if (conflict) severity = "error";
    else if (!camera) severity = "empty";
    else if (warnings.length > 0) severity = "warning";

    return {
      cameraId,
      slice: slotSlice(doc, cameraId),
      region: regionLabel(doc, cameraId),
      camera,
      sources,
      conflict,
      severity,
    };
  });
}

// What moving cameras between slots does, for the confirm dialog. Settings
// stay with the physical camera; the seed and calibration belong to the
// region they were made for.
export interface Consequence {
  camera: CameraDoc;
  from: number;
  to: number;
  seedStale: boolean;
  calibrationRemoved: boolean;
}

function consequence(camera: CameraDoc, to: number): Consequence {
  return {
    camera,
    from: camera.cameraId,
    to,
    seedStale: (camera.seed?.lineCorners.length ?? 0) === 4,
    calibrationRemoved: camera.calibration !== undefined,
  };
}

// A move, swap, or camera count change waiting on confirmation.
export interface PendingMove {
  title: string;
  plan: Consequence[];
  // Set for a camera count change.
  count?: number;
}

// Moving the camera in slot from to slot to, swapping with whatever is there.
export function planMove(
  doc: ConfigDocument,
  from: number,
  to: number,
): Consequence[] {
  const a = doc.cameras.find((c) => c.cameraId === from);
  const b = doc.cameras.find((c) => c.cameraId === to);
  if (!a || from === to) return [];

  return b ? [consequence(a, to), consequence(b, from)] : [consequence(a, to)];
}

// Changing the count re-splits the field, so every camera's region changes
// even though its id doesn't.
export function planCount(doc: ConfigDocument, count: number): Consequence[] {
  if (count === cameraCount(doc)) return [];

  return doc.cameras.map((c) => consequence(c, c.cameraId));
}

// The smallest count every current camera_id still fits in.
export function minCameraCount(doc: ConfigDocument): number {
  const top = Math.max(-1, ...doc.cameras.map((c) => c.cameraId));

  return CAMERA_COUNTS.find((n) => n > top) ?? Infinity;
}

// Changes how the GUI orients a camera's video, dropping the display block
// when it's back to the default.
export function setDisplay(
  cameraId: number,
  change: { rotate?: number; mirror?: boolean },
): void {
  const camera = config.doc?.cameras.find((c) => c.cameraId === cameraId);
  if (!camera) return;

  const rotate = ((((change.rotate ?? camera.display?.rotate ?? 0) % 360) +
    360) %
    360) as 0 | 90 | 180 | 270;
  const mirror = change.mirror ?? camera.display?.mirror ?? false;

  if (rotate === 0 && !mirror) delete camera.display;
  else
    camera.display = {
      ...(rotate ? { rotate } : {}),
      ...(mirror ? { mirror } : {}),
    };
}

// Adds a camera for a heard but unplaced camera_id, named after the address
// it was heard from until someone renames it.
export function addHeardCamera(source: DetectionSource): void {
  const doc = config.doc;
  if (!doc || doc.cameras.some((c) => c.cameraId === source.cameraId)) return;

  doc.cameras.push({ cameraId: source.cameraId, instance: source.address });
  doc.cameras.sort((a, b) => a.cameraId - b.cameraId);
}

// Applies a plan from planMove or planCount to the live document. The seed
// is kept but tagged with the slot it was picked for, which the host reports
// as stale; the calibration is dropped, which the host withholds.
export function applyPlan(plan: Consequence[], count?: number): void {
  const doc = config.doc;
  if (!doc) return;

  const oldCount = cameraCount(doc);

  // Look every camera up before changing any id: a swap reuses both ids.
  const steps = plan.map((step) => ({
    step,
    camera: doc.cameras.find((c) => c.cameraId === step.from),
  }));

  for (const { step, camera } of steps) {
    if (!camera) continue;

    if (camera.seed && !camera.seed.slot) {
      camera.seed.slot = { cameraId: step.from, cameraCount: oldCount };
    }

    delete camera.calibration;
    camera.cameraId = step.to;
  }

  // Only an actual change: setting the count it already has would pin it,
  // so it no longer follows the number of cameras.
  if (count !== undefined && count !== oldCount) {
    doc.layout = { ...doc.layout, cameraCount: count };
  }

  doc.cameras.sort((a, b) => a.cameraId - b.cameraId);
}

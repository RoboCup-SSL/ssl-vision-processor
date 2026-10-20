// The host's working vision.yml document, shared by every tab. Edits apply to
// the running system as soon as they reach the host; only Save writes the
// file. Components bind straight into config.doc -- a root effect below
// notices any change and debounces a PUT, so no component needs to call a
// setter or remember to save.
import { requestJSON } from "./api";
import { topic } from "./wrapper-bus";
import {
  refreshFieldMarkings,
  type FieldConfig,
  type OptionalFieldLines,
} from "./geometry.svelte";

import { app } from "./text/app";

export type Section =
  | "field"
  | "camera"
  | "geometry"
  | "color"
  | "network"
  | "layout"
  | "advanced"
  | "overview"
  | "other";

// Mirrors gui/internal/config's Change: one leaf difference between two
// documents, keyed by its vision.yml path.
export interface Change {
  path: string;
  before: unknown;
  after: unknown;
  section: Section;
  cameraId?: number;
}

export interface Warning {
  code: string;
  message: string;
}

export interface CameraStatus {
  cameraId: number;
  calibration: "none" | "live" | "locked";
  live?: Record<string, unknown>;
  liveResolution: [number, number];
  warnings: Warning[];
}

export interface ConfigState {
  path: string;
  revision: number;
  // What saving would write: disk -> working.
  changes: Change[];
  // Set while vision.yml on disk differs from what was loaded or saved.
  external?: { error?: string; changes: Change[] };
  cameras: CameraStatus[];
  renderErrors?: Record<string, string>;
}

export interface Seed {
  // [width, height] the corners were picked at; [0, 0] if unknown.
  resolution: [number, number];
  lineCorners: [number, number][];
  goalSideMarker?: number;
  // The slot the corners were picked for; unset for older seeds.
  slot?: Slot;
}

// A team's robot height: another height table entry's (name), or a custom
// height in mm. Neither set: a custom height not entered yet.
export interface TeamOverride {
  name?: string;
  height?: number;
}

// A camera's place in the layout: its id out of the camera count.
export interface Slot {
  cameraId: number;
  cameraCount: number;
}

// How the GUI orients a camera's video. Never sent to vision_processor.
export interface Display {
  rotate?: 0 | 90 | 180 | 270;
  mirror?: boolean;
}

export interface Calibration {
  lockedAt: string;
  fieldHash: string;
  camera: Record<string, unknown>;
}

export interface CameraDoc {
  cameraId: number;
  instance?: string;
  configPath?: string;
  config?: Record<string, unknown>;
  seed?: Seed;
  calibration?: Calibration;
  display?: Display;
}

export interface ConfigDocument {
  version: number;
  field: FieldConfig;
  optionalFieldLines: OptionalFieldLines;
  models?: Record<string, unknown>;
  defaults?: Record<string, unknown>;
  // Unset: the camera count rounded up to a power of 2 (cameraCount()).
  layout?: { cameraCount: number };
  // Robot height overrides by game controller team name; see
  // gui/internal/config's Teams.
  teams?: {
    overrides?: Record<string, TeamOverride>;
    // Per color, off a competition field.
    byColor?: { yellow?: TeamOverride; blue?: TeamOverride };
  };
  // A match setup with a game controller, rather than lab use; see
  // gui/internal/config's Document.CompetitionField.
  competitionField?: boolean;
  cameras: CameraDoc[];
  // Settings for the GUI host alone; see gui/internal/config's Host.
  host?: {
    interfaces?: { auto?: boolean; skip?: string[] };
  };
}

interface ConfigResponse {
  document: ConfigDocument;
  state: ConfigState;
}

interface ConflictResponse {
  error: "stale" | "disk_changed" | "no_live_calibration";
  message: string;
}

export const config = $state<{
  doc: ConfigDocument | null;
  // Revision doc was last confirmed at by the host.
  revision: number;
  state: ConfigState | null;
  // Last failed request, shown as a banner (e.g. a rejected edit).
  error: string | null;
  busy: boolean;
}>({
  doc: null,
  revision: 0,
  state: null,
  error: null,
  busy: false,
});

const PUT_DEBOUNCE_MS = 250;

// JSON of the doc as last sent to (or received from) the host -- the root
// effect compares against this so a doc that just arrived from the host isn't
// echoed straight back.
let lastSynced = "";
let putTimer: ReturnType<typeof setTimeout> | null = null;
let inFlight: Promise<void> | null = null;

export function cameraDoc(cameraId: number): CameraDoc | undefined {
  return config.doc?.cameras.find((c) => c.cameraId === cameraId);
}

export function cameraStatus(cameraId: number): CameraStatus | undefined {
  return config.state?.cameras.find((c) => c.cameraId === cameraId);
}

export function changesFor(section: Section, cameraId?: number): Change[] {
  return (config.state?.changes ?? []).filter(
    (c) =>
      c.section === section &&
      (cameraId === undefined ||
        c.cameraId === undefined ||
        c.cameraId === cameraId),
  );
}

export async function loadConfig(): Promise<void> {
  try {
    const response = await requestJSON("/api/config");
    const data = (await response.json()) as ConfigResponse;
    lastSynced = JSON.stringify(data.document);
    config.doc = data.document;
    config.revision = data.state.revision;
    config.state = data.state;
    await refreshFieldMarkings();
  } catch (err) {
    config.error = message(err);
  }
}

// Resolves once no edit is waiting to be sent or in flight, so a save sees
// everything the user has typed.
export async function flushEdits(): Promise<void> {
  if (putTimer !== null) {
    clearTimeout(putTimer);
    putTimer = null;
    void sendPending();
  }

  while (inFlight) await inFlight;
}

function schedulePut(): void {
  if (putTimer !== null) clearTimeout(putTimer);

  putTimer = setTimeout(() => {
    putTimer = null;
    void sendPending();
  }, PUT_DEBOUNCE_MS);
}

async function sendPending(): Promise<void> {
  if (inFlight) {
    await inFlight;
  }

  const json = JSON.stringify(config.doc);
  if (!config.doc || json === lastSynced) return;

  lastSynced = json;
  inFlight = putDoc(json);

  try {
    await inFlight;
  } finally {
    inFlight = null;
  }

  // Anything typed while that was in flight goes out next.
  if (JSON.stringify(config.doc) !== lastSynced) void sendPending();
}

async function putDoc(json: string): Promise<void> {
  const response = await fetch("/api/config", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: `{"revision":${String(config.revision)},"document":${json}}`,
  });

  if (response.ok) {
    config.revision = (
      (await response.json()) as { revision: number }
    ).revision;
    config.error = null;
    await refreshFieldMarkings();

    return;
  }

  if (response.status === 409) {
    // Another tab edited first. Take the host's version rather than
    // clobbering it.
    config.error = app.editedElsewhere;
    await loadConfig();

    return;
  }

  config.error = await response.text();
}

$effect.root(() => {
  $effect(() => {
    const json = JSON.stringify(config.doc);
    if (config.doc && json !== lastSynced) schedulePut();
  });

  // State arrives on every host change and once a second (live calibration
  // state comes from the network). A revision we don't have means someone
  // else changed the document -- another tab, or a lock/reload -- so refetch,
  // unless our own edit is mid-flight and about to report that revision.
  $effect(() => {
    return topic<ConfigState>("config.state").subscribe((state) => {
      if (!state) return;

      config.state = state;

      if (
        config.doc &&
        state.revision !== config.revision &&
        !inFlight &&
        putTimer === null
      ) {
        void loadConfig();
      }
    });
  });
});

async function action(
  url: string,
  init: RequestInit = {},
): Promise<ConflictResponse | null> {
  config.busy = true;

  try {
    const response = await fetch(url, init);

    if (response.status === 409) {
      return (await response.json()) as ConflictResponse;
    }

    if (!response.ok) {
      throw new Error(await response.text());
    }

    config.error = null;

    return null;
  } finally {
    config.busy = false;
    await loadConfig();
  }
}

function post(url: string, body?: unknown): Promise<ConflictResponse | null> {
  return action(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}

// Writes the working document to vision.yml. A conflict comes back rather
// than throwing: "disk_changed" means the file was edited elsewhere and the
// caller should let the user choose (state.external now describes it).
export async function saveConfig(
  force = false,
): Promise<ConflictResponse | null> {
  await flushEdits();

  return guarded(() =>
    post("/api/config/save", { revision: config.revision, force }),
  );
}

export async function saveConfigAs(path: string): Promise<void> {
  await flushEdits();
  await guarded(() =>
    post("/api/config/save-as", { revision: config.revision, path }),
  );
}

export async function loadConfigFrom(path: string): Promise<void> {
  await guarded(() => post("/api/config/load", { path }));
}

// Accepts vision.yml as it is on disk, discarding unsaved edits.
export async function reloadFromDisk(): Promise<void> {
  await guarded(() => post("/api/config/reload"));
}

export async function lockCalibration(cameraId: number): Promise<void> {
  await flushEdits();

  const conflict = await guarded(() =>
    post(`/api/config/cameras/${String(cameraId)}/calibration`),
  );
  if (conflict) config.error = conflict.message;
}

export async function unlockCalibration(cameraId: number): Promise<void> {
  await flushEdits();
  await guarded(() =>
    action(`/api/config/cameras/${String(cameraId)}/calibration`, {
      method: "DELETE",
    }),
  );
}

async function guarded<T>(fn: () => Promise<T>): Promise<T | null> {
  try {
    return await fn();
  } catch (err) {
    config.error = message(err);

    return null;
  }
}

function message(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

// A short, human-readable rendering of a Change value for the confirmation
// dialogs: whole blocks (a locked calibration, a new camera) are summarized
// rather than dumped.
export function formatValue(value: unknown): string {
  if (value === undefined || value === null) return "(none)";

  if (typeof value === "object" && !Array.isArray(value)) {
    const keys = Object.keys(value);

    return `{${String(keys.length)} fields}`;
  }

  return JSON.stringify(value);
}

export const SECTION_LABELS: Record<Section, string> = {
  field: "Field Dimensions",
  camera: "Camera Settings",
  geometry: "Geometry",
  color: "Color",
  network: "Network",
  layout: "Camera Layout",
  advanced: "Advanced",
  overview: "Overview",
  other: "Other settings",
};

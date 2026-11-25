// The host's working vision.yml document, shared by every tab. Edits apply to
// the running system as soon as they reach the host; only Save writes the
// file. Components bind straight into config.doc -- a root effect below
// notices any change and debounces a PUT, so no component needs to call a
// setter or remember to save.
import { errorMessage, requestJSON } from "./api";
import type {
  ApiError,
  CameraDoc,
  CameraStatus,
  Change,
  ConfigDocument,
  ConfigResponse,
  ConfigState,
  Section,
} from "./apiTypes";
import { onTopic } from "./bus.svelte";
import { refreshFieldMarkings } from "./geometry.svelte";

import { app } from "./text/app";
import { loadSchema } from "./schema.svelte";

// The API's types (apiTypes.ts, generated from the host's spec), re-exported
// so components import the document's shapes from where they edit it.
export type {
  CameraDoc,
  CameraStatus,
  Calibration,
  Change,
  ConfigDocument,
  ConfigState,
  Display,
  Section,
  Seed,
  Slot,
  TeamOverride,
  Warning,
} from "./apiTypes";

// A 409's body: "stale", "disk_changed" or "no_live_calibration" in error.
type ConflictResponse = ApiError;

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
// How soon an edit the host didn't receive is sent again.
const RETRY_MS = 2_000;

// JSON of the doc as last accepted by (or received from) the host -- the root
// effect compares against this so a doc that just arrived from the host isn't
// echoed straight back, and an edit that never arrived is still unsent.
let lastSynced = "";
let putTimer: ReturnType<typeof setTimeout> | null = null;
let retryTimer: ReturnType<typeof setTimeout> | null = null;
let inFlight: Promise<boolean> | null = null;

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
    // Before the document, so everything rendered from it has the schema.
    await loadSchema();
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
  if (putTimer !== null || retryTimer !== null) {
    if (putTimer !== null) clearTimeout(putTimer);
    if (retryTimer !== null) clearTimeout(retryTimer);
    putTimer = null;
    retryTimer = null;
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
  // A loop: another waiter may have started the next send first.
  while (inFlight) await inFlight;

  const json = JSON.stringify(config.doc);
  if (!config.doc || json === lastSynced) return;

  inFlight = putDoc(json);
  let delivered: boolean;

  try {
    delivered = await inFlight;
  } finally {
    inFlight = null;
  }

  if (!delivered) {
    retryLater();

    return;
  }

  // Anything typed while that was in flight goes out next.
  if (JSON.stringify(config.doc) !== lastSynced) void sendPending();
}

function retryLater(): void {
  if (retryTimer !== null) return;

  retryTimer = setTimeout(() => {
    retryTimer = null;
    void sendPending();
  }, RETRY_MS);
}

// False when the host never answered (restarting, network down): the edit
// stays unsent. Any answer, even a refusal, counts as delivered.
async function putDoc(json: string): Promise<boolean> {
  let response: Response;

  try {
    response = await fetch("/api/config", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: `{"revision":${String(config.revision)},"document":${json}}`,
    });
  } catch {
    config.error = app.notSent;

    return false;
  }

  if (response.ok) {
    lastSynced = json;
    config.revision = (
      (await response.json()) as { revision: number }
    ).revision;
    config.error = null;
    await refreshFieldMarkings();

    return true;
  }

  if (response.status === 409) {
    // Another tab edited first. Take the host's version rather than
    // clobbering it.
    config.error = app.editedElsewhere;
    await loadConfig();

    return true;
  }

  // Refused (T4-3: the form keeps the refused value until the next edit).
  lastSynced = json;
  config.error = await errorMessage(response);

  return true;
}

$effect.root(() => {
  $effect(() => {
    const json = JSON.stringify(config.doc);
    if (config.doc && json !== lastSynced) schedulePut();
  });

  // State arrives on every host change and once a second (live calibration
  // state comes from the network). A revision we don't have means someone
  // else changed the document -- another tab, or a lock/reload -- so refetch,
  // unless our own edit is mid-flight and about to report that revision, or
  // waiting to be retried (a reload would drop it).
  $effect(() => {
    return onTopic("config.state", (state) => {
      config.state = state;

      if (
        config.doc &&
        state.revision !== config.revision &&
        !inFlight &&
        putTimer === null &&
        retryTimer === null
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
      throw new Error(await errorMessage(response));
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

export async function saveConfigAs(name: string): Promise<void> {
  await flushEdits();
  await guarded(() =>
    post("/api/config/save-as", { revision: config.revision, name }),
  );
}

export async function loadConfigFrom(name: string): Promise<void> {
  await guarded(() => post("/api/config/load", { name }));
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
  field_dimensions: "Dimensions",
  camera: "Camera",
  geometry: "Geometry",
  color: "Color",
  network: "Network",
  layout: "Layout",
  advanced: "Advanced",
  team_assignment: "Teams",
  other: "Other settings",
};

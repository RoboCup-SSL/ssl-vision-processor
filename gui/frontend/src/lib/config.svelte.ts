// The host's working vision.yml document, shared by every tab. Edits apply to
// the running system as soon as they reach the host; only Save writes the
// file. Components bind straight into config.doc -- a root effect below
// notices any change and debounces a PUT, so no component needs to call a
// setter or remember to save.
import { api, errorMessage, unwrap } from "./api";
import type {
  ApiError,
  CameraDoc,
  CameraStatus,
  Change,
  ConfigDocument,
  ConfigResponse,
  ConfigState,
  PutConfigRequest,
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

// The GUI edits defaults and each camera's config generically, guided by
// GET /api/schema, where the spec types them key by key; these two are where
// one becomes the other.
export async function getConfig(): Promise<ConfigResponse> {
  return (await unwrap(api.GET("/api/config"))) as ConfigResponse;
}

type SpecDocument = PutConfigRequest["document"];

export async function loadConfig(): Promise<void> {
  try {
    // Before the document, so everything rendered from it has the schema.
    await loadSchema();
    const data = await getConfig();
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
  let answer;

  try {
    answer = await api.PUT("/api/config", {
      body: {
        revision: config.revision,
        document: JSON.parse(json) as SpecDocument,
      },
    });
  } catch {
    config.error = app.notSent;

    return false;
  }

  if (answer.data) {
    lastSynced = json;
    config.revision = answer.data.revision;
    config.error = null;
    await refreshFieldMarkings();

    return true;
  }

  if (answer.response.status === 409) {
    // Another tab edited first. Take the host's version rather than
    // clobbering it.
    config.error = app.editedElsewhere;
    await loadConfig();

    return true;
  }

  // Refused: the host still has its own version.
  lastSynced = json;
  config.error = errorMessage(answer.error, answer.response);
  dropRefused(json);

  return true;
}

// Goes back to the host's document after it refused json, so the form shows
// what's in use. Not while a field is being typed in (a number often passes
// through a refused value): then once it's left. A later edit is sent as
// usual, and its own answer decides.
function dropRefused(json: string): void {
  const restore = (): void => {
    if (JSON.stringify(config.doc) === json) void loadConfig();
  };

  if (document.activeElement?.matches("input, textarea")) {
    document.addEventListener("focusout", restore, { once: true });
  } else {
    restore();
  }
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

// Runs a call that changes the document on the host, then reloads it. A 409
// comes back for the caller to handle; any other error throws.
async function action(
  call: () => Promise<{ error?: ApiError; response: Response }>,
): Promise<ConflictResponse | null> {
  config.busy = true;

  try {
    const { error, response } = await call();

    if (response.status === 409 && error) return error;
    if (!response.ok) throw new Error(errorMessage(error, response));

    config.error = null;

    return null;
  } finally {
    config.busy = false;
    await loadConfig();
  }
}

// Writes the working document to vision.yml. A conflict comes back rather
// than throwing: "disk_changed" means the file was edited elsewhere and the
// caller should let the user choose (state.external now describes it).
export async function saveConfig(
  force = false,
): Promise<ConflictResponse | null> {
  await flushEdits();

  return guarded(() =>
    action(() =>
      api.POST("/api/config/save", {
        body: { revision: config.revision, force },
      }),
    ),
  );
}

export async function saveConfigAs(name: string): Promise<void> {
  await flushEdits();
  await guarded(() =>
    action(() =>
      api.POST("/api/config/save-as", {
        body: { revision: config.revision, name },
      }),
    ),
  );
}

export async function loadConfigFrom(name: string): Promise<void> {
  await guarded(() =>
    action(() => api.POST("/api/config/load", { body: { name } })),
  );
}

// Accepts vision.yml as it is on disk, discarding unsaved edits.
export async function reloadFromDisk(): Promise<void> {
  await guarded(() => action(() => api.POST("/api/config/reload")));
}

export async function lockCalibration(cameraId: number): Promise<void> {
  await flushEdits();

  const conflict = await guarded(() =>
    action(() =>
      api.POST("/api/config/cameras/{id}/calibration", {
        params: { path: { id: cameraId } },
      }),
    ),
  );
  if (conflict) config.error = conflict.message;
}

export async function unlockCalibration(cameraId: number): Promise<void> {
  await flushEdits();
  await guarded(() =>
    action(() =>
      api.DELETE("/api/config/cameras/{id}/calibration", {
        params: { path: { id: cameraId } },
      }),
    ),
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

// The setup checklist (ChecklistPanel.svelte): the steps a new setup goes
// through, in the order they're usually done. A guide, not a gate: the order
// is only advice, and nothing is ever disabled.
//
// Items are either automatic, derived from what the host reports (config,
// network.state, alerts) so the user can't tick them, or manual, for checks
// only a person can make ("the image looks right"). A manual tick stores a
// fingerprint of the settings it judged; when those change, the item asks for
// a recheck instead of keeping a stale tick. Field dimensions is manual too,
// since a new config already has the rulebook field: applying the wizard
// ticks it (wizard.svelte.ts).
//
// Ticks are per browser and per config file, in localStorage like
// preferences.svelte.ts. Restarting clears the ticks and never touches config.
import { config, type CameraDoc } from "../config.svelte";
import { network } from "../network.svelte";
import { hostAlerts } from "../alerts/hostAlerts.svelte";
import { connectionState, type ConnectionState } from "../wrapper-bus";

export type Status = "done" | "todo" | "attention" | "recheck";

export const ITEM_IDS = [
  "connection",
  "network",
  "field",
  "layout",
  "processors",
  "image",
  "calibration",
  "color",
  "teams",
  "save",
  "liveField",
] as const;

export type ItemId = (typeof ITEM_IDS)[number];

interface ItemDef {
  manual: boolean;
  perCamera: boolean;
  // Advice only: shown as "usually done after" while these aren't done.
  after: ItemId[];
}

export const ITEMS: Record<ItemId, ItemDef> = {
  connection: { manual: false, perCamera: false, after: [] },
  network: { manual: false, perCamera: false, after: ["connection"] },
  field: { manual: true, perCamera: false, after: [] },
  layout: { manual: false, perCamera: false, after: ["field"] },
  processors: { manual: false, perCamera: true, after: ["network", "layout"] },
  image: { manual: true, perCamera: true, after: ["processors"] },
  calibration: { manual: false, perCamera: true, after: ["field", "image"] },
  color: { manual: true, perCamera: true, after: ["image"] },
  teams: { manual: false, perCamera: false, after: [] },
  save: { manual: false, perCamera: false, after: [] },
  liveField: {
    manual: true,
    perCamera: false,
    after: ["calibration", "color", "save"],
  },
};

// Which host alert codes put an automatic item in "attention". Cautions
// don't: they're often fine, so they leave the item done.
const ALERT_CODES: Partial<Record<ItemId, string[]>> = {
  network: ["socket_problem", "loopback_multicast_off"],
  layout: ["camera_unplaced", "region_uncovered", "camera_id_conflict"],
  teams: ["team_default_height"],
};

// Camera warnings (config.state.cameras) that mean a calibration no longer
// fits: the field changed, or the camera's resolution did.
const CALIBRATION_WARNINGS = ["field_changed", "calibration_aspect_mismatch"];

// A note is a key into text/checklist.ts's notes, plus any argument it takes,
// so this module stays free of wording.
export type Note =
  | { key: "heard"; fps: number | undefined }
  | { key: "unsaved"; count: number }
  | { key: "warning"; message: string }
  | {
      key:
        | "connecting"
        | "disconnected"
        | "noNetworkState"
        | "networkProblem"
        | "layoutProblem"
        | "notHeard"
        | "notCalibrated"
        | "calibratedNotLocked"
        | "locked"
        | "teamsProblem"
        | "firstRun"
        | "saved";
    };

export interface CameraRow {
  cameraId: number;
  // The tick's key, for manual rows.
  key: string;
  status: Status;
  note?: Note;
}

export interface ItemState {
  id: ItemId;
  status: Status;
  note?: Note;
  cameras?: CameraRow[];
}

const PANEL_KEY = "vision-processor-gui:checklist-panel";
const TICKS_PREFIX = "vision-processor-gui:checklist:";

export const checklist = $state<{
  open: boolean;
  // The one expanded item in the panel, if any.
  expanded: ItemId | null;
  // Tick key ("field", "image:0") -> fingerprint of what was checked.
  ticks: Record<string, string>;
  connection: ConnectionState;
}>({
  open: loadPanelOpen(),
  expanded: null,
  ticks: {},
  connection: "connecting",
});

// The config file the ticks belong to; ticks change only once it's known.
let ticksPath: string | null = null;

function loadPanelOpen(): boolean {
  try {
    return localStorage.getItem(PANEL_KEY) === "open";
  } catch {
    return false;
  }
}

function savePanelOpen(): void {
  try {
    localStorage.setItem(PANEL_KEY, checklist.open ? "open" : "closed");
  } catch {
    // Private browsing / storage disabled: the panel just won't reopen itself.
  }
}

function loadTicks(path: string): Record<string, string> {
  try {
    const raw = localStorage.getItem(TICKS_PREFIX + path);
    const parsed: unknown = raw ? JSON.parse(raw) : {};

    return parsed && typeof parsed === "object"
      ? (parsed as Record<string, string>)
      : {};
  } catch {
    return {};
  }
}

function saveTicks(): void {
  if (ticksPath === null) return;

  try {
    localStorage.setItem(
      TICKS_PREFIX + ticksPath,
      JSON.stringify(checklist.ticks),
    );
  } catch {
    // As above: ticks just won't survive a reload.
  }
}

// JSON with object keys sorted, so the same settings fingerprint the same
// whatever order the host or a preset happened to list them in.
function stableJSON(value: unknown): string {
  return JSON.stringify(value, (_key, v: unknown) =>
    v && typeof v === "object" && !Array.isArray(v)
      ? Object.fromEntries(
          Object.entries(v as Record<string, unknown>).sort(([a], [b]) =>
            a.localeCompare(b),
          ),
        )
      : v,
  );
}

function cameraBlock(camera: CameraDoc, block: string): unknown {
  return {
    defaults: config.doc?.defaults?.[block],
    camera: camera.config?.[block],
  };
}

// What a manual tick judged, so a later change to it asks for a recheck.
function fingerprint(key: string): string {
  const [id, camera] = key.split(":");
  const cam = config.doc?.cameras.find((c) => String(c.cameraId) === camera);

  switch (id) {
    case "field":
    case "liveField":
      return stableJSON({
        field: config.doc?.field,
        lines: config.doc?.optionalFieldLines,
      });
    case "image":
      return cam ? stableJSON(cameraBlock(cam, "camera")) : "";
    case "color":
      return cam ? stableJSON(cameraBlock(cam, "color")) : "";
    default:
      return "";
  }
}

function manualStatus(key: string): Status {
  const tick = checklist.ticks[key];
  if (tick === undefined) return "todo";

  return tick === fingerprint(key) ? "done" : "recheck";
}

export function tickKey(id: ItemId, cameraId?: number): string {
  return cameraId === undefined ? id : `${id}:${String(cameraId)}`;
}

export function isTicked(key: string): boolean {
  return checklist.ticks[key] !== undefined;
}

export function setTicked(key: string, ticked: boolean): void {
  checklist.ticks = ticked
    ? { ...checklist.ticks, [key]: fingerprint(key) }
    : Object.fromEntries(
        Object.entries(checklist.ticks).filter(([k]) => k !== key),
      );

  saveTicks();
}

function alertOpen(id: ItemId): boolean {
  const codes = ALERT_CODES[id] ?? [];

  return hostAlerts.list.some(
    (a) => a.severity !== "caution" && codes.includes(a.code),
  );
}

function cameraRows(
  id: ItemId,
  row: (camera: CameraDoc) => { status: Status; note?: Note },
): CameraRow[] {
  return (config.doc?.cameras ?? []).map((camera) => ({
    cameraId: camera.cameraId,
    key: tickKey(id, camera.cameraId),
    ...row(camera),
  }));
}

function processorRow(camera: CameraDoc): { status: Status; note: Note } {
  const source = network.state?.cameras.find(
    (s) => s.cameraId === camera.cameraId,
  );

  return source?.receiving
    ? { status: "done", note: { key: "heard", fps: source.fps } }
    : { status: "todo", note: { key: "notHeard" } };
}

function calibrationRow(camera: CameraDoc): { status: Status; note: Note } {
  const status = config.state?.cameras.find(
    (c) => c.cameraId === camera.cameraId,
  );
  const warning = status?.warnings.find((w) =>
    CALIBRATION_WARNINGS.includes(w.code),
  );

  if (warning) {
    return {
      status: "attention",
      note: { key: "warning", message: warning.message },
    };
  }

  switch (status?.calibration) {
    case "locked":
      return { status: "done", note: { key: "locked" } };
    case "live":
      return { status: "todo", note: { key: "calibratedNotLocked" } };
    default:
      return { status: "todo", note: { key: "notCalibrated" } };
  }
}

// A per-camera item is as far along as its furthest-behind camera.
function rollUp(rows: CameraRow[]): Status {
  if (rows.length === 0) return "todo";
  if (rows.some((r) => r.status === "attention")) return "attention";
  if (rows.some((r) => r.status === "recheck")) return "recheck";
  if (rows.every((r) => r.status === "done")) return "done";

  return "todo";
}

function withRows(id: ItemId, rows: CameraRow[]): ItemState {
  return { id, status: rollUp(rows), cameras: rows };
}

function alertItem(id: ItemId, problem: Note): ItemState {
  return alertOpen(id)
    ? { id, status: "attention", note: problem }
    : { id, status: "done" };
}

export function itemState(id: ItemId): ItemState {
  switch (id) {
    case "connection":
      if (checklist.connection === "open") return { id, status: "done" };

      return checklist.connection === "closed"
        ? { id, status: "attention", note: { key: "disconnected" } }
        : { id, status: "todo", note: { key: "connecting" } };
    case "network":
      if (!network.state) {
        return { id, status: "todo", note: { key: "noNetworkState" } };
      }

      return alertItem(id, { key: "networkProblem" });
    case "layout":
      if (!config.doc?.cameras.length) return { id, status: "todo" };

      return alertItem(id, { key: "layoutProblem" });
    case "processors":
      return withRows(id, cameraRows(id, processorRow));
    case "calibration":
      return withRows(id, cameraRows(id, calibrationRow));
    case "image":
    case "color":
      return withRows(
        id,
        cameraRows(id, (camera) => ({
          status: manualStatus(tickKey(id, camera.cameraId)),
        })),
      );
    case "teams":
      return alertItem(id, { key: "teamsProblem" });
    case "save": {
      const unsaved = config.state?.changes.length ?? 0;

      if (config.state?.firstRun) {
        return { id, status: "todo", note: { key: "firstRun" } };
      }

      return unsaved > 0
        ? { id, status: "todo", note: { key: "unsaved", count: unsaved } }
        : { id, status: "done", note: { key: "saved" } };
    }
    case "field":
    case "liveField":
      return { id, status: manualStatus(id) };
  }
}

export function allItems(): ItemState[] {
  return ITEM_IDS.map(itemState);
}

// The steps an item usually comes after that aren't done yet.
export function pendingBefore(id: ItemId, items: ItemState[]): ItemId[] {
  return ITEMS[id].after.filter(
    (before) => items.find((i) => i.id === before)?.status !== "done",
  );
}

export function openChecklist(): void {
  checklist.open = true;
  checklist.expanded ??=
    allItems().find((i) => i.status !== "done")?.id ?? null;
  savePanelOpen();
}

export function closeChecklist(): void {
  checklist.open = false;
  savePanelOpen();
}

export function toggleChecklist(): void {
  if (checklist.open) closeChecklist();
  else openChecklist();
}

// Clears every tick for this config file. Settings are left alone, so
// automatic items show as done again at once.
export function restartChecklist(): void {
  checklist.ticks = {};
  saveTicks();
  checklist.expanded = allItems().find((i) => i.status !== "done")?.id ?? null;
}

$effect.root(() => {
  $effect(() => {
    return connectionState.subscribe((state) => {
      checklist.connection = state;
    });
  });

  // Each config file keeps its own ticks: load them when the file changes
  // (startup, Load…, Save as…).
  $effect(() => {
    const path = config.state?.path;
    if (path === undefined || path === ticksPath) return;

    ticksPath = path;
    checklist.ticks = loadTicks(path);
  });
});

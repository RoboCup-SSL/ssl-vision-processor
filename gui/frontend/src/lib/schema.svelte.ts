// What the host says about vision_processor's settings (GET /api/schema; see
// gui/internal/config's schema.go): each key's type, fallback, bounds, and
// whether a change needs a restart. The one copy of what src/Resources.cpp
// does with config.yml, so nothing here repeats it. loadConfig loads it before
// the document, so anything rendered from the document can rely on it.
import { requestJSON } from "./api";

// When a change takes effect: live (reloaded within a second), at the next
// restart, written by the host itself (derived), or read only by the host.
export type Apply = "live" | "restart" | "derived" | "host";

export interface Setting {
  type:
    | "integer"
    | "number"
    | "boolean"
    | "string"
    | "rgb"
    | "whiteBalance"
    | "points";
  // vision_processor's fallback; null when there is none or it depends on
  // other keys.
  default: unknown;
  apply: Apply;
  unit?: string;
  minimum?: number;
  maximum?: number;
  exclusiveMinimum?: boolean;
  enum?: string[];
  // A sensible span for a control, narrower than the bounds.
  range?: { min: number; max: number; step: number };
}

export interface DriverCapabilities {
  id: string;
  selectsBy: "id" | "path";
  gamma: boolean;
  whiteBalanceProfiles: boolean;
}

export interface Schema {
  settings: Record<string, Setting>;
  drivers: DriverCapabilities[];
  cameraCounts: number[];
}

export const schema = $state<{ value: Schema | null }>({ value: null });

let loading: Promise<void> | null = null;

// Loads the schema once; later calls wait on the same request. A failure is
// thrown to the caller and retried on the next call.
export function loadSchema(): Promise<void> {
  loading ??= requestJSON("/api/schema")
    .then((response) => response.json())
    .then((value) => {
      schema.value = value as Schema;
    })
    .catch((err: unknown) => {
      loading = null;
      throw err;
    });

  return loading;
}

// The setting at path, e.g. "thresholds.blobs". Throws if the schema isn't
// loaded or has no such key: both are bugs, not runtime conditions.
export function setting(path: string): Setting {
  const s = schema.value?.settings[path];
  if (!s) throw new Error(`schema: no setting ${path}`);

  return s;
}

// vision_processor's fallback for path.
export function fallback(path: string): unknown {
  return setting(path).default;
}

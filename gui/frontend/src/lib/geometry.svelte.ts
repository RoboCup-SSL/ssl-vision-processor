// Shared state for the virtual field editor. A module-level $state object,
// per the project convention (this replaces a store; see CLAUDE.md).
import type {
  SSL_GeometryFieldSizeJson,
  SSL_FieldLineSegmentJson,
  SSL_FieldCircularArcJson,
} from "../proto/vision/ssl_vision_geometry_pb";
import { requestJSON, withLoadingState } from "./api";

// The editable dimensions: everything the generated field size carries except
// fieldLines/fieldArcs, which are derived server-side and never hand-edited.
export type FieldConfig = Omit<
  SSL_GeometryFieldSizeJson,
  "fieldLines" | "fieldArcs"
>;

export interface OptionalFieldLines {
  goal2Goal: boolean;
  halfway: boolean;
  centerCircle: boolean;
  penalty: boolean;
}

interface FieldConfigResponse {
  path?: string;
  field: FieldConfig;
  optionalFieldLines: OptionalFieldLines;
}

// A rulebook preset, as served by GET /api/geometry/presets -- read live
// from geometry-divA.yml/geometry-divB.yml on the Go host, not duplicated
// here. See the setup wizard's WizardStart.svelte for what's done with one
// once loaded.
export interface FieldPreset extends FieldConfigResponse {
  name: string;
}

// Just enough of the full /api/geometry response to render the sketch -- not
// calib, not source, nothing else this editor doesn't need.
interface GeometryResponse {
  geometry?: {
    field?: {
      fieldLines?: SSL_FieldLineSegmentJson[];
      fieldArcs?: SSL_FieldCircularArcJson[];
    };
  };
}

export const virtualField = $state<{
  path: string;
  field: FieldConfig;
  optionalFieldLines: OptionalFieldLines;
  fieldLines: SSL_FieldLineSegmentJson[];
  fieldArcs: SSL_FieldCircularArcJson[];
  presets: FieldPreset[];
  loading: boolean;
  saving: boolean;
  error: string | null;
  dirty: boolean;
}>({
  path: "",
  field: {},
  optionalFieldLines: {
    goal2Goal: false,
    halfway: false,
    centerCircle: false,
    penalty: false,
  },
  fieldLines: [],
  fieldArcs: [],
  presets: [],
  loading: false,
  saving: false,
  error: null,
  dirty: false,
});

// Presets are loaded separately from loadVirtualField: a preset endpoint that
// can't be read (see the Go handler's graceful degradation) shouldn't stop
// the actual field config from loading, and vice versa -- a convenience, not
// required, so a failure here is silently ignored rather than surfaced.
export async function loadFieldPresets(): Promise<void> {
  try {
    const response = await requestJSON("/api/geometry/presets");
    virtualField.presets = (await response.json()) as FieldPreset[];
  } catch {
    // An empty list just means the preset section has nothing to offer.
  }
}

// Refreshes fieldLines/fieldArcs from /api/geometry -- the field-config
// endpoints don't carry the generated markings themselves, and every one of
// them (load, save, save-as) leaves the server having just regenerated these
// from whatever field config is now active.
async function refreshFieldMarkings(): Promise<void> {
  const response = await requestJSON("/api/geometry");
  const geometry = (await response.json()) as GeometryResponse;
  virtualField.fieldLines = geometry.geometry?.field?.fieldLines ?? [];
  virtualField.fieldArcs = geometry.geometry?.field?.fieldArcs ?? [];
}

function applyFieldConfigResponse(config: FieldConfigResponse): void {
  virtualField.path = config.path ?? "";
  virtualField.field = config.field;
  virtualField.optionalFieldLines = config.optionalFieldLines;
  virtualField.dirty = false;
}

export async function loadVirtualField(): Promise<void> {
  await withLoadingState(
    (v) => (virtualField.loading = v),
    (v) => (virtualField.error = v),
    async () => {
      const response = await requestJSON("/api/geometry/field");
      applyFieldConfigResponse((await response.json()) as FieldConfigResponse);
      await refreshFieldMarkings();
    },
  );
}

export async function saveVirtualField(): Promise<void> {
  await withLoadingState(
    (v) => (virtualField.saving = v),
    (v) => (virtualField.error = v),
    async () => {
      await requestJSON("/api/geometry/field", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          field: virtualField.field,
          optionalFieldLines: virtualField.optionalFieldLines,
        } satisfies FieldConfigResponse),
      });

      virtualField.dirty = false;
      await refreshFieldMarkings();
    },
  );
}

// Writes the current field config to a new path and switches to editing that
// file (see geometry.Geometry.SaveAs) -- subsequent saveVirtualField calls go
// there, not wherever this session started out.
export async function saveVirtualFieldAs(path: string): Promise<void> {
  await withLoadingState(
    (v) => (virtualField.saving = v),
    (v) => (virtualField.error = v),
    async () => {
      await requestJSON("/api/geometry/field/save-as", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          path,
          field: virtualField.field,
          optionalFieldLines: virtualField.optionalFieldLines,
        } satisfies FieldConfigResponse),
      });

      virtualField.path = path;
      virtualField.dirty = false;
      await refreshFieldMarkings();
    },
  );
}

// Discards the current field config and replaces it with whatever's at path
// (see geometry.Geometry.LoadFrom). Existing calibrations are dropped
// server-side -- they belonged to the field this used to be.
export async function loadVirtualFieldFrom(path: string): Promise<void> {
  await withLoadingState(
    (v) => (virtualField.loading = v),
    (v) => (virtualField.error = v),
    async () => {
      const response = await requestJSON("/api/geometry/field/load", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ path }),
      });

      applyFieldConfigResponse((await response.json()) as FieldConfigResponse);
      await refreshFieldMarkings();
    },
  );
}

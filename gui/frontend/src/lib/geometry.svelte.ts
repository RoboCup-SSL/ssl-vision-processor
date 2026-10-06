// Field types and the live field markings. The editable field itself lives in
// the host's working document (config.svelte.ts's config.doc.field); this
// module holds what's derived from it on the host -- the generated lines and
// arcs -- plus the rulebook presets.
import type {
  SSL_GeometryFieldSizeJson,
  SSL_FieldLineSegmentJson,
  SSL_FieldCircularArcJson,
} from "../proto/vision/ssl_vision_geometry_pb";
import { requestJSON } from "./api";

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

// A rulebook preset, as served by GET /api/geometry/presets -- read live
// from geometry-divA.yml/geometry-divB.yml on the Go host, not duplicated
// here. See the setup wizard's WizardStart.svelte for what's done with one
// once loaded.
export interface FieldPreset {
  name: string;
  field: FieldConfig;
  optionalFieldLines: OptionalFieldLines;
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
  fieldLines: SSL_FieldLineSegmentJson[];
  fieldArcs: SSL_FieldCircularArcJson[];
  presets: FieldPreset[];
}>({
  fieldLines: [],
  fieldArcs: [],
  presets: [],
});

// A convenience, not required: a preset endpoint that can't be read (see the
// Go handler's graceful degradation) just leaves the list empty.
export async function loadFieldPresets(): Promise<void> {
  try {
    const response = await requestJSON("/api/geometry/presets");
    virtualField.presets = (await response.json()) as FieldPreset[];
  } catch {
    // An empty list just means there's nothing to offer.
  }
}

// Refreshes fieldLines/fieldArcs from /api/geometry, which the host
// regenerates from the field whenever the working document changes.
export async function refreshFieldMarkings(): Promise<void> {
  try {
    const response = await requestJSON("/api/geometry");
    const geometry = (await response.json()) as GeometryResponse;
    virtualField.fieldLines = geometry.geometry?.field?.fieldLines ?? [];
    virtualField.fieldArcs = geometry.geometry?.field?.fieldArcs ?? [];
  } catch {
    // The sketch keeps its last markings; the next edit retries.
  }
}

// Field types and the live field markings. The editable field itself lives in
// the host's working document (config.svelte.ts's config.doc.field); this
// module holds what's derived from it on the host -- the generated lines and
// arcs -- plus the rulebook presets.
import type {
  SSL_FieldLineSegmentJson,
  SSL_FieldCircularArcJson,
} from "../proto/vision/ssl_vision_geometry_pb";
import { requestJSON } from "./api";
import type { FieldConfig, FieldPreset } from "./apiTypes";

// The editable field (dimensions, optional lines) and a rulebook preset
// (GET /api/geometry/presets, read live from geometry-divA.yml and
// geometry-divB.yml on the host), from the host's spec. See the setup
// wizard's WizardStart.svelte for what's done with a preset once loaded.
export type { FieldConfig, FieldPreset, OptionalFieldLines } from "./apiTypes";

// A field with every dimension 0: a placeholder until a real one (the
// document's, or a preset's) replaces it.
export function blankField(): FieldConfig {
  return {
    ballRadius: 0,
    boundaryWidth: 0,
    boundaryWidthGoalLine: 0,
    centerCircleRadius: 0,
    fieldLength: 0,
    fieldWidth: 0,
    goalCenterToPenaltyMark: 0,
    goalDepth: 0,
    goalHeight: 0,
    goalSubstitutionAreaWidth: 0,
    goalWidth: 0,
    lineThickness: 0,
    maxRobotRadius: 0,
    penaltyAreaDepth: 0,
    penaltyAreaWidth: 0,
  };
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

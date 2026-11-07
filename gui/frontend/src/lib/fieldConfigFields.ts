// Shared display metadata for FieldConfig/OptionalFieldLines, so the field
// list and optional-line list aren't duplicated between FieldEditor.svelte
// and the setup wizard's equivalent steps.
import type { FieldConfig, OptionalFieldLines } from "./geometry.svelte";

export interface FieldConfigField {
  key: keyof FieldConfig;
  label: string;
  // Which optional line this dimension only makes sense with, if any (e.g. no
  // penalty area means no penalty area depth/width to enter). Used by the
  // wizard's Dimensions step, which asks about markings first specifically so
  // it can skip fields that don't apply; FieldEditor.svelte ignores this and
  // always shows every field, since a direct-edit form has no such ordering.
  gatedBy?: keyof OptionalFieldLines;
}

// Order matches how someone would actually measure a field: outer boundary
// first, then the goal, then the penalty area, then the smaller markings.
// fieldLength is deliberately excluded -- FieldEditor.svelte handles it
// separately (its half/full toggle); a caller that doesn't do that (the
// wizard) prepends its own { key: "fieldLength", ... } entry.
export const DIMENSION_FIELDS: FieldConfigField[] = [
  { key: "fieldWidth", label: "Field width" },
  { key: "boundaryWidth", label: "Boundary width" },
  { key: "boundaryWidthGoalLine", label: "Boundary width (goal line)" },
  { key: "goalWidth", label: "Goal width" },
  { key: "goalDepth", label: "Goal depth" },
  { key: "goalHeight", label: "Goal height" },
  { key: "penaltyAreaDepth", label: "Penalty area depth", gatedBy: "penalty" },
  { key: "penaltyAreaWidth", label: "Penalty area width", gatedBy: "penalty" },
  {
    key: "centerCircleRadius",
    label: "Center circle radius",
    gatedBy: "centerCircle",
  },
  { key: "lineThickness", label: "Line thickness" },
  { key: "ballRadius", label: "Ball radius" },
  { key: "maxRobotRadius", label: "Max robot radius" },
];

export interface OptionalLineField {
  key: keyof OptionalFieldLines;
  label: string;
}

export const OPTIONAL_LINE_FIELDS: OptionalLineField[] = [
  { key: "halfway", label: "Halfway line" },
  { key: "goal2Goal", label: "Center line (goal-to-goal)" },
  { key: "centerCircle", label: "Center circle" },
  { key: "penalty", label: "Penalty area" },
];

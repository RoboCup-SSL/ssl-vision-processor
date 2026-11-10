// Setup wizard (wizard/).

import { plural } from "./common";
export const wizard = {
  start: "Start from a regulation field, or configure a custom one.",
  markings: "Which markings does this field have?",
  review: "Review, then apply to put these values live on the field.",
  // `backticks` show as code.
  applyNote: (path: string): string =>
    `Applying takes effect immediately but doesn't save -- use Save in the settings menu (or Ctrl+S) to write \`${path}\`, then calibrate each camera on its own Geometry tab.`,
  // The dimensions step. Saying which way each runs keeps length and width
  // from being swapped.
  halfLength: (full: number): string =>
    `Half length: goal line to halfway line (full: ${String(full)}mm)`,
  fieldWidth: "Field width: along the goal line",
  layoutCount: (regions: number, half: boolean): string =>
    `Sets the layout to ${String(regions)} ${plural(regions, "region")}${half ? ", since a half field is half of a full one" : ""}. Assign cameras to regions on the Layout page.`,
  finishCameras: (regions: number, changes: boolean): string =>
    `Cameras: ${String(regions)} ${plural(regions, "region")}${changes ? ". This changes every camera's region: their line corners need re-picking and locked calibrations are removed." : ""}`,
};

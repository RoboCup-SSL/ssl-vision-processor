// Live Field page (live/LiveFieldPanel.svelte): detections and autoref
// tracker output drawn on the field. `backticks` show as code.

import { steady } from "./common";

export const liveField = {
  heading: "Live Field",
  intro:
    "Robots and balls as the vision_processors detect them, unfiltered, and as each autoref's tracker reports them. Autorefs appear as the host hears them on the tracker group.",

  waitingForField: "Waiting for the field geometry from the host…",

  view: {
    title: "View",
    primary: "Show",
    robots: "Robots",
    numbered: "Numbered",
    pattern: "Shell pattern",
    velocities: "Velocities",
    flipped: "Swap sides",
    axes: "Origin and axes",
    cameraTint: "Outline by camera",
  },

  sources: {
    title: "Sources",
    vision: "Vision (unfiltered)",
    notHeard: (name: string): string => `${name} (not heard)`,
    overlayHint:
      "Other sources draw under the one shown, at their own opacity, outlined in their color. Comparing them shows filtering and lag.",
    opacity: "Opacity",
    noTrackers: (address: string): string =>
      `No autoref heard on ${address}. Its address is on the Network page.`,
    trackerProblem: (problem: string): string =>
      `The tracker socket isn't open: ${problem}`,
  },

  status: {
    receiving: (fps: number, ageMs: number | null): string =>
      `${steady(String(fps), 3)} fps${ageMs === null ? "" : `, ${steady(ageMs.toFixed(0), 4)} ms ago`}`,
    silent: "Silent",
  },

  cameras: {
    title: "Cameras",
    camera: "Camera",
    address: "From",
    rate: "Rate",
    processing: "Processing",
    processingHint:
      "Capture to send, by the vision_processor's own clock. Age is since the host received the frame.",
    shown: "Shown",
    none: "No vision_processor heard yet.",
    ms: (ms: number): string => `${steady(ms.toFixed(1), 5)} ms`,
  },

  teams: (yellow: string, blue: string): string =>
    `Yellow: ${yellow || "?"} · Blue: ${blue || "?"}`,
};

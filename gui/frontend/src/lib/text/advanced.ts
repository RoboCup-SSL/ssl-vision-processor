// Advanced page (config/AdvancedPanel.svelte): vision_processor's
// thresholds: and tracking: settings, shared by every camera. Notes are
// from config.yml's comments and src/Resources.cpp. `backticks` show as code.

import { plural } from "./common";
export const advanced = {
  heading: "Advanced",
  intro:
    "Detection thresholds and tracking limits, shared by every camera (vision.yml's `defaults.thresholds` and `defaults.tracking`). The defaults suit most fields; change these only to fix a specific detection problem. Most apply live; the ones marked restart need a vision_processor restart.",
  overriding: (cameras: number[]): string =>
    `${plural(cameras.length, "Camera")} ${cameras.join(", ")} override${cameras.length === 1 ? "s" : ""} some of these in their own config, so they don't follow this page for those values.`,
  restart: "Applies when vision_processor restarts.",
  defaultIs: (value: string): string => `Default: ${value}.`,
  reset: "Reset to default",

  cards: {
    blobs: "Blob detection",
    tolerances: "Geometry tolerances",
    tracking: "Tracking",
  },

  fields: {
    circularity: {
      label: "Circularity",
      notes: [
        "Minimum mean cosine similarity of a blob's border gradient (not normalized, 0 - 195075).",
        "Higher misses real blobs; lower adds false positives.",
      ],
    },
    score: {
      label: "Ball score",
      notes: ["Minimum circularity / (3 × stddev) for a ball blob."],
    },
    blobs: {
      label: "Max blobs",
      notes: [
        "Most blobs processed per frame. Shouldn't need changing; vision_processor logs a warning when a frame hits it.",
      ],
    },
    min_confidence: {
      label: "Min confidence",
      notes: [
        "Robots and balls below this confidence (0 - 1) aren't reported.",
      ],
    },
    resampling_factor: {
      label: "Resampling",
      notes: [
        "Scales the field image blobs are searched in. Below 1 is faster and coarser; above 1 is finer and slower.",
      ],
    },
    min_cam_edge_distance: {
      label: "Ball edge distance",
      notes: [
        "A ball closer than this to the edge of the camera's view, inside the field, is ignored: it's likely half out of frame.",
      ],
    },
    clipping_tolerance: {
      label: "Clipping tolerance",
      notes: ["How far two detected objects may overlap."],
    },
    geometry_tolerance: {
      label: "Geometry tolerance",
      notes: [
        "Added to geometry checks: the field border cutoff and which blobs count as field lines.",
      ],
    },
    min_tracking_radius: {
      label: "Min tracking radius",
      notes: [
        "Smallest radius searched for a robot's blobs from one frame to the next.",
      ],
    },
    max_bot_acceleration: {
      label: "Max robot acceleration",
      notes: [
        "Fastest a robot is expected to accelerate. Sets how far the search for its blobs grows between frames.",
      ],
    },
  },
};

// The left sidebar's lower nav list, one entry per config.yml top-level
// section (see config.yml / config-camtest.yml at the repo root) plus one
// for the shared field geometry. Field names/comments below are copied
// straight from config.yml -- ground truth for whichever contributor builds
// out a real form for a category, not invented.
//
// scope: "shared" categories are one thing for the whole deployment
// (geometry.yml, edited by FieldEditor already). "per-instance" categories
// are a single vision processor's own config.yml, which this host does not
// yet read, write, or push to any instance -- see gui/CLAUDE.md's "Not yet
// built". Their panels are placeholders until that exists.
export type ConfigScope = "shared" | "per-instance";

export interface ConfigField {
  name: string;
  comment: string;
}

export interface ConfigCategory {
  id: string;
  label: string;
  scope: ConfigScope;
  /** The config.yml top-level key this maps to, or null for the shared field geometry. */
  yamlKey: string | null;
  fields: ConfigField[];
}

export const CONFIG_CATEGORIES: ConfigCategory[] = [
  {
    id: "field",
    label: "Virtual Field",
    scope: "shared",
    yamlKey: null, // geometry.yml's field: block, not config.yml
    fields: [],
  },
  {
    id: "camera",
    label: "Camera",
    scope: "per-instance",
    yamlKey: "camera",
    fields: [
      { name: "driver", comment: "SPINNAKER, MVIMPACT, or OPENCV" },
      { name: "id / path", comment: "Which camera device" },
      { name: "width / height", comment: "0 = highest supported resolution" },
      { name: "exposure", comment: "ms; 0.0 = automatic" },
      { name: "gain", comment: "0.0 = automatic" },
      { name: "gamma", comment: "1.0 = no gamma" },
      { name: "white_balance", comment: "OUTDOOR/INDOOR, or manual red/blue" },
    ],
  },
  {
    id: "geometry",
    label: "Geometry",
    scope: "per-instance",
    yamlKey: "geometry", // config.yml's geometry: block -- NOT geometry.yml's field:
    fields: [
      { name: "camera_amount", comment: "Total cameras over the field" },
      {
        name: "camera_height",
        comment: "mm; 0.0 = automatic (fails if camera looks perpendicular)",
      },
      {
        name: "line_corners",
        comment:
          "Pixel corners for calibration seeding -- this is what the Corner Picker below produces",
      },
      { name: "refinement", comment: "Field line pixel refinement toggle" },
      { name: "field_line_threshold", comment: "0-255 brightness delta" },
      { name: "min_line_segment_length", comment: "px" },
      { name: "max_line_segment_offset", comment: "px" },
      { name: "max_line_segment_angle", comment: "degrees" },
    ],
  },
  {
    id: "thresholds",
    label: "Thresholds",
    scope: "per-instance",
    yamlKey: "thresholds",
    fields: [
      { name: "circularity", comment: "0 - 195075.0" },
      { name: "score", comment: "min circularity/(3*stddev)" },
      { name: "blobs", comment: "max blobs processed per frame" },
      { name: "min_confidence", comment: "0.0 - 1.0" },
      { name: "min_cam_edge_distance", comment: "mm" },
      { name: "clipping_tolerance", comment: "mm" },
      { name: "geometry_tolerance", comment: "mm" },
    ],
  },
  {
    id: "color",
    label: "Color",
    scope: "per-instance",
    yamlKey: "color",
    fields: [
      { name: "reference_force", comment: "0.0 - 0.5-history_force/2" },
      { name: "history_force", comment: "0.0 - 1.0-reference_force" },
      { name: "orange / field", comment: "ball / carpet reference colors" },
      { name: "yellow / blue", comment: "center blob reference colors" },
      { name: "green / pink", comment: "side blob reference colors" },
    ],
  },
  {
    id: "tracking",
    label: "Tracking",
    scope: "per-instance",
    yamlKey: "tracking",
    fields: [
      { name: "min_tracking_radius", comment: "mm" },
      { name: "max_bot_acceleration", comment: "m/s^2" },
    ],
  },
  {
    id: "network",
    label: "Network",
    scope: "per-instance",
    yamlKey: "network",
    fields: [
      { name: "gc_ip / gc_port", comment: "game controller multicast" },
      { name: "vision_ip / vision_port", comment: "vision multicast" },
    ],
  },
  {
    id: "stream",
    label: "Stream",
    scope: "per-instance",
    yamlKey: "stream",
    fields: [
      { name: "active", comment: "encode and send a live stream" },
      { name: "raw_feed", comment: "raw camera footage only" },
      { name: "ip_base_prefix / ip_base_end", comment: "stream destination" },
      { name: "port", comment: "stream port" },
    ],
  },
  {
    id: "debug",
    label: "Debug",
    scope: "per-instance",
    yamlKey: "debug",
    // Not to be confused with the mockup's future "Debug Console" (a live log
    // viewer over internal/hub) -- this is config.yml's debug: block, which
    // controls the vision processor's own diagnostic image output.
    fields: [
      { name: "ground_truth", comment: "used by blob/geometry benchmarks" },
      { name: "wait_for_geometry", comment: "hold frames until calibrated" },
      { name: "debug_images", comment: "save additional debug images in img/" },
      {
        name: "debug_stream_interval_ms",
        comment: "periodic img/.sample.<camId>.png while uncalibrated",
      },
    ],
  },
];

// The main column's own horizontal tab bar: quick access to the categories
// used constantly while working on one camera, separate from the sidebar's
// full list of all nine. Start small and add to this as more categories earn
// a spot -- it's deliberately a subset, not a duplicate of CONFIG_CATEGORIES.
export const TAB_CATEGORY_IDS = ["field", "geometry", "color"];

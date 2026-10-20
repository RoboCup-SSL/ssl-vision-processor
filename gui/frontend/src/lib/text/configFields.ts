// The config.yml settings each placeholder page lists
// (config/ConfigFieldList.svelte), with the comment config.yml gives each.
// Copied from config.yml; keep them in step with it.
import type { ConfigField } from "../layout/configCategories";

export const CONFIG_FIELDS = {
  camera: [
    { name: "driver", comment: "SPINNAKER, MVIMPACT, or OPENCV" },
    { name: "id / path", comment: "Which camera device" },
    { name: "width / height", comment: "0 = highest supported resolution" },
    { name: "exposure", comment: "ms; 0.0 = automatic" },
    { name: "gain", comment: "0.0 = automatic" },
    { name: "gamma", comment: "1.0 = no gamma" },
    { name: "white_balance", comment: "OUTDOOR/INDOOR, or manual red/blue" },
  ],
  geometry: [
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
  color: [
    { name: "reference_force", comment: "0.0 - 0.5-history_force/2" },
    { name: "history_force", comment: "0.0 - 1.0-reference_force" },
    { name: "orange / field", comment: "ball / carpet reference colors" },
    { name: "yellow / blue", comment: "center blob reference colors" },
    { name: "green / pink", comment: "side blob reference colors" },
  ],
  network: [
    { name: "gc_ip / gc_port", comment: "game controller multicast" },
    { name: "vision_ip / vision_port", comment: "vision multicast" },
  ],
  stream: [
    { name: "active", comment: "encode and send a live stream" },
    { name: "raw_feed", comment: "raw camera footage only" },
    { name: "ip_base_prefix / ip_base_end", comment: "stream destination" },
    { name: "port", comment: "stream port" },
  ],
  debug: [
    { name: "ground_truth", comment: "used by blob/geometry benchmarks" },
    { name: "wait_for_geometry", comment: "hold frames until calibrated" },
    { name: "debug_images", comment: "save additional debug images in img/" },
    {
      name: "debug_stream_interval_ms",
      comment: "periodic img/.sample.<camId>.png while uncalibrated",
    },
  ],
} satisfies Record<string, ConfigField[]>;

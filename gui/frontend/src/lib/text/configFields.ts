// Each category's config.yml settings, with the comment config.yml gives
// each. No page shows them since the last placeholder page went.
// Copied from config.yml; keep them in step with it.
import type { ConfigField } from "../layout/configCategories";

export const CONFIG_FIELDS = {
  camera: [
    {
      name: "driver",
      comment: "DUMMY (none selected), SPINNAKER, MVIMPACT, DC1394, or OPENCV",
    },
    { name: "id / device", comment: "Which camera device" },
    { name: "width / height", comment: "0 = highest supported resolution" },
    { name: "exposure", comment: "µs; 0 = automatic" },
    { name: "gain", comment: "0.0 = automatic" },
    { name: "gamma", comment: "1.0 = no gamma" },
    { name: "white_balance", comment: "OUTDOOR/INDOOR, or manual red/blue" },
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
} satisfies Record<string, ConfigField[]>;

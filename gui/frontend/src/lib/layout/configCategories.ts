// Every page, one entry per config.yml top-level
// section (see config.yml / config-camtest.yml at the repo root) plus one
// for the shared field geometry. Each category's settings and their
// config.yml comments are in text/configFields.ts.
//
// scope: "shared" categories are one thing for the whole deployment
// (vision.yml's field, and its defaults.network addresses). "per-instance" categories
// are one camera's own settings, edited in vision.yml's cameras[].config.
import { CONFIG_FIELDS } from "../text/configFields";

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
  /** Shown only with "Expert user" on in the settings menu. */
  expert?: boolean;
  fields: ConfigField[];
}

export const CONFIG_CATEGORIES: ConfigCategory[] = [
  {
    id: "alerts",
    label: "Alerts",
    scope: "shared",
    yamlKey: null,
    fields: [],
  },
  {
    id: "live_field",
    label: "Live Field",
    // Live detections and autoref tracker output drawn on the field.
    scope: "shared",
    yamlKey: null,
    fields: [],
  },
  {
    id: "team_assignment",
    label: "Teams",
    scope: "shared",
    yamlKey: null, // vision.yml's teams
    fields: [],
  },
  {
    id: "network",
    label: "Network",
    // vision.yml's defaults.network: one pair of groups for the host and
    // every vision_processor. Edited by config/NetworkPanel.svelte.
    scope: "shared",
    yamlKey: "network",
    fields: CONFIG_FIELDS.network,
  },
  {
    id: "field_dimensions",
    label: "Dimensions",
    scope: "shared",
    yamlKey: null, // geometry.yml's field: block, not config.yml
    fields: [],
  },
  {
    id: "layout",
    label: "Layout",
    scope: "shared",
    yamlKey: null, // vision.yml's layout: and each camera's camera_id
    fields: [],
  },
  {
    id: "advanced",
    label: "Advanced",
    scope: "shared",
    expert: true,
    yamlKey: null, // vision.yml's defaults.thresholds and defaults.tracking
    fields: [],
  },
  {
    id: "camera",
    label: "Camera",
    scope: "per-instance",
    yamlKey: "camera",
    fields: CONFIG_FIELDS.camera,
  },
  {
    id: "geometry",
    label: "Geometry",
    scope: "per-instance",
    yamlKey: "geometry", // config.yml's geometry: block -- NOT geometry.yml's field:
    fields: [], // edited by config/GeometryPanel.svelte
  },
  {
    id: "color",
    label: "Color",
    scope: "per-instance",
    yamlKey: "color",
    fields: CONFIG_FIELDS.color,
  },
  {
    id: "stream",
    label: "Stream",
    scope: "per-instance",
    yamlKey: "stream",
    fields: CONFIG_FIELDS.stream,
  },
];

// The tab bar's pages, in order: all but Alerts, which has its own sidebar
// block.
export const TAB_CATEGORY_IDS = CONFIG_CATEGORIES.map((c) => c.id).filter(
  (id) => id !== "alerts",
);

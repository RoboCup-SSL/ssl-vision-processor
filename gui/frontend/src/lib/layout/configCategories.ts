// The left sidebar's lower nav list, one entry per config.yml top-level
// section (see config.yml / config-camtest.yml at the repo root) plus one
// for the shared field geometry. Each category's settings and their
// config.yml comments are in text/configFields.ts.
//
// scope: "shared" categories are one thing for the whole deployment
// (vision.yml's field, and its defaults.network addresses). "per-instance" categories
// are a single vision processor's own config.yml, which this host does not
// yet read, write, or push to any instance -- see gui/CLAUDE.md's "Not yet
// built". Their panels are placeholders until that exists.
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
  /** False for pages reached some other way (Alerts, from its own sidebar block). */
  inNav?: boolean;
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
    inNav: false,
  },
  {
    id: "overview",
    label: "Overview",
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
    id: "field",
    label: "Field Dimensions",
    scope: "shared",
    yamlKey: null, // geometry.yml's field: block, not config.yml
    fields: [],
  },
  {
    id: "layout",
    label: "Camera Layout",
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
    label: "Camera Settings",
    scope: "per-instance",
    yamlKey: "camera",
    fields: CONFIG_FIELDS.camera,
  },
  {
    id: "geometry",
    label: "Geometry",
    scope: "per-instance",
    yamlKey: "geometry", // config.yml's geometry: block -- NOT geometry.yml's field:
    fields: CONFIG_FIELDS.geometry,
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
  {
    id: "debug",
    label: "Debug",
    scope: "per-instance",
    yamlKey: "debug",
    // Not to be confused with the mockup's future "Debug Console" (a live log
    // viewer over internal/hub) -- this is config.yml's debug: block, which
    // controls the vision processor's own diagnostic image output.
    fields: CONFIG_FIELDS.debug,
  },
];

// The main column's own horizontal tab bar: quick access to the categories
// used constantly while working on one camera, separate from the sidebar's
// full list of all nine. Start small and add to this as more categories earn
// a spot -- it's deliberately a subset, not a duplicate of CONFIG_CATEGORIES.
export const TAB_CATEGORY_IDS = [
  "overview",
  "network",
  "field",
  "layout",
  "advanced",
  "camera",
  "geometry",
  "color",
];

// The host's API types, generated from gui/openapi.json (npm run gen:api, run
// by `make generate`). Import API shapes from here, by these names, rather
// than writing a copy of a Go type: the generated file follows the Go code,
// and CI fails when it's out of date.
import type { components } from "./generated/api";

type Schemas = components["schemas"];

// vision_processor's config.yml layout (defaults, a camera's config). The
// spec types it key by key, but the GUI edits it generically, guided by
// GET /api/schema at run time.
export type Settings = Record<string, unknown>;

export type ConfigDocument = Omit<
  Schemas["ConfigDocument"],
  "defaults" | "cameras"
> & {
  defaults?: Settings;
  cameras: CameraDoc[];
};
export type CameraDoc = Omit<Schemas["ConfigCamera"], "config"> & {
  config?: Settings;
};
export type Seed = Schemas["ConfigSeed"];
export type Slot = Schemas["ConfigSlot"];
export type Display = Schemas["ConfigDisplay"];
export type Calibration = Schemas["ConfigCalibration"];
export type TeamOverride = Schemas["ConfigTeamOverride"];
export type FieldConfig = Schemas["GeometryFieldConfig"];
export type OptionalFieldLines = Schemas["GeometryOptionalLinesConfig"];

export type ConfigResponse = Omit<Schemas["ConfigResponse"], "document"> & {
  document: ConfigDocument;
};
export type ConfigState = Schemas["ConfigState"];
export type Change = Schemas["ConfigChange"];
export type Section = Change["section"];
export type CameraStatus = Schemas["ConfigCameraStatus"];
export type Warning = Schemas["ConfigWarning"];
export type RevisionResponse = Schemas["RevisionResponse"];

export type NetworkState = Schemas["NetworkState"];
export type SocketStatus = Schemas["MulticastStatus"];
export type HostInterface = Schemas["MulticastInterface"];
export type DetectionSource = Schemas["DetectionsSource"];
export type RefereeState = Schemas["RefereeState"];
export type RefereeTeam = Schemas["RefereeTeam"];

export type HostAlert = Schemas["AlertsAlert"];
export type Severity = HostAlert["severity"];

export type SettingsSchema = Schemas["ConfigSchema"];
export type Setting = Schemas["ConfigSetting"];
export type Apply = Setting["apply"];
export type DriverCapabilities = Schemas["ConfigDriver"];

export type FieldPreset = Schemas["FieldPresetResponse"];
export type ConfigFile = Schemas["ConfigFile"];
export type VideoDevice = Schemas["V4LDevice"];
export type SnapshotEntry = Schemas["SnapshotEntry"];
export type ProcessStatus = Schemas["SupervisorStatus"];

export type ApiError = Schemas["HttpjsonErrorBody"];
export type ErrorCode = ApiError["error"];

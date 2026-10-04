// A camera's camera: block (vision_processor's src/driver/cameradriver.cpp),
// as the Camera Settings tab edits it. Values shown are effective ones: the
// C++ fallbacks, then vision.yml's defaults.camera, then the camera's own
// config.camera. Edits go into the camera's own block, like the color panel:
// a key is written when it differs from what the camera would inherit, and
// removed when it's set back.
//
// vision_processor reads camera: only at startup, so nothing here takes
// effect until it restarts.
import { config, cameraDoc } from "./config.svelte";

export type Driver = "SPINNAKER" | "MVIMPACT" | "OPENCV";

export type WhiteBalance = "OUTDOOR" | "INDOOR" | { red: number; blue: number };

export interface CameraSettings {
  driver: Driver;
  id: number;
  // Unset means /dev/video{id}.
  path?: string;
  width: number;
  height: number;
  exposure: number;
  gain: number;
  gamma: number;
  white_balance: WhiteBalance;
}

export type CameraKey = keyof CameraSettings;

// What each driver does with the block (src/driver/*.cpp).
export interface DriverInfo {
  label: string;
  // Selected by index in the SDK's camera list, or (OpenCV) by path.
  selectsBy: "id" | "path";
  gamma: boolean;
  // Separate OUTDOOR and INDOOR auto white balance profiles.
  wbProfiles: boolean;
}

export const DRIVERS: Record<Driver, DriverInfo> = {
  SPINNAKER: {
    label: "Spinnaker (FLIR)",
    selectsBy: "id",
    gamma: true,
    wbProfiles: true,
  },
  MVIMPACT: {
    label: "mvIMPACT (Bluefox3)",
    selectsBy: "id",
    gamma: false,
    wbProfiles: false,
  },
  OPENCV: {
    label: "OpenCV (V4L2 cameras, files)",
    selectsBy: "path",
    // Documented as supported, though see the gamma warning in the panel.
    gamma: true,
    wbProfiles: false,
  },
};

// cameradriver.cpp's fallbacks for keys the block leaves out.
const CPP_DEFAULTS: CameraSettings = {
  driver: "SPINNAKER",
  id: 0,
  width: 0,
  height: 0,
  exposure: 0,
  gain: 0,
  gamma: 1,
  white_balance: "OUTDOOR",
};

type Block = Record<string, unknown>;

function asBlock(value: unknown): Block {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Block)
    : {};
}

function inheritedBlock(): Block {
  return asBlock(config.doc?.defaults?.["camera"]);
}

function ownBlock(cameraId: number): Block {
  return asBlock(asBlock(cameraDoc(cameraId)?.config)["camera"]);
}

function inheritedValue(key: CameraKey): unknown {
  const block = inheritedBlock();

  return key in block ? block[key] : CPP_DEFAULTS[key];
}

export function cameraSettings(cameraId: number): CameraSettings {
  return {
    ...CPP_DEFAULTS,
    ...inheritedBlock(),
    ...ownBlock(cameraId),
  };
}

// Sets one key for the camera. undefined removes the camera's override, so
// it inherits again.
export function setCameraKey<K extends CameraKey>(
  cameraId: number,
  key: K,
  value: CameraSettings[K] | undefined,
): void {
  const camera = cameraDoc(cameraId);
  if (!camera) return;

  const own = ownBlock(cameraId);
  const same = JSON.stringify(value) === JSON.stringify(inheritedValue(key));

  // Back to what it would inherit anyway: drop the override, so undoing an
  // edit leaves the file as it was.
  if (value === undefined || same) {
    if (!(key in own)) return;

    writeBlock(
      cameraId,
      Object.fromEntries(Object.entries(own).filter(([k]) => k !== key)),
    );

    return;
  }

  if (JSON.stringify(own[key]) === JSON.stringify(value)) return;

  writeBlock(cameraId, { ...own, [key]: value });
}

function writeBlock(cameraId: number, block: Block): void {
  const camera = cameraDoc(cameraId);
  if (!camera) return;

  camera.config ??= {};

  if (Object.keys(block).length === 0) {
    delete camera.config["camera"];
  } else {
    camera.config["camera"] = block;
  }
}

// A capture device on the host, from GET /api/camera/devices.
export interface VideoDevice {
  path: string;
  node: string;
  name: string;
  kind: "by-id" | "by-path" | "node";
}

export async function loadVideoDevices(): Promise<VideoDevice[]> {
  try {
    const response = await fetch("/api/camera/devices");

    return response.ok ? ((await response.json()) as VideoDevice[]) : [];
  } catch {
    return [];
  }
}

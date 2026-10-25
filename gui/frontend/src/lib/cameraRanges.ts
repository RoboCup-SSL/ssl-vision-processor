// Slider ranges for the Camera Settings tab.
//
// PLACEHOLDERS. A camera's real limits are only known to whoever can talk to
// it: the vision_processor, through its SDK or V4L2. Once it reports them
// over the network (SSL_VPConfig), these become fallbacks. The number box next
// to each slider takes any value, so a range here never limits a setting.
//
// Exposure is µs for every driver; the host converts it to each driver's
// config.yml unit. The slider stops at 20 ms: past that, motion blur ruins
// detection and the frame rate drops. OPENCV steps by 100 µs, V4L2's
// exposure_time_absolute unit.
//
// OPENCV's other ranges use the team's UC70 dev webcam (v4l2-ctl
// --list-ctrls), in the units vision_processor's OpenCV driver actually
// sends (src/driver/opencvdriver.cpp):
//   - gain: sent as is. The UC70 takes 0-8.
//   - gamma: sent as is, where V4L2 means gamma × 100. The UC70 takes
//     100-300.
// SPINNAKER and MVIMPACT use typical machine-vision ranges.
import type { Driver } from "./cameraSettings.svelte";

export interface Range {
  // In display units: what the slider and number box show.
  min: number;
  max: number;
  step: number;
  unit: string;
  // File value = display value × scale.
  scale: number;
  // Display value to start from when switching off Auto.
  fallback: number;
}

export interface CameraRanges {
  exposure: Range;
  gain: Range;
  gamma: Range;
  whiteBalance: Range;
}

export const CAMERA_RANGES: Record<Driver, CameraRanges> = {
  SPINNAKER: {
    exposure: {
      min: 10,
      max: 20000,
      step: 10,
      unit: "µs",
      scale: 1,
      fallback: 5000,
    },
    gain: { min: 0.1, max: 48, step: 0.1, unit: "dB", scale: 1, fallback: 5 },
    gamma: { min: 0.25, max: 4, step: 0.05, unit: "", scale: 1, fallback: 1.5 },
    whiteBalance: {
      min: 0.25,
      max: 4,
      step: 0.01,
      unit: "",
      scale: 1,
      fallback: 1,
    },
  },
  MVIMPACT: {
    exposure: {
      min: 10,
      max: 20000,
      step: 10,
      unit: "µs",
      scale: 1,
      fallback: 5000,
    },
    gain: { min: 0.1, max: 24, step: 0.1, unit: "dB", scale: 1, fallback: 5 },
    // mvIMPACT ignores gamma; the panel hides it.
    gamma: { min: 0.25, max: 4, step: 0.05, unit: "", scale: 1, fallback: 1.5 },
    whiteBalance: {
      min: 0.1,
      max: 4,
      step: 0.01,
      unit: "",
      scale: 1,
      fallback: 1,
    },
  },
  OPENCV: {
    exposure: {
      min: 100,
      max: 20000,
      step: 100,
      unit: "µs",
      scale: 1,
      fallback: 5000,
    },
    // 0 means automatic in the file, so manual starts at 1.
    gain: {
      min: 1,
      max: 8,
      step: 1,
      unit: "camera units",
      scale: 1,
      fallback: 4,
    },
    gamma: {
      min: 100,
      max: 300,
      step: 1,
      unit: "× 0.01",
      scale: 1,
      fallback: 110,
    },
    // The UC70 has no red/blue balance controls; a generic 8-bit range.
    whiteBalance: {
      min: 0,
      max: 255,
      step: 1,
      unit: "camera units",
      scale: 1,
      fallback: 128,
    },
  },
};

// File value → display value.
export function toDisplay(range: Range, value: number): number {
  return round(value / range.scale);
}

// Display value → file value, rounded so 166 × 0.001 is 0.166, not
// 0.16600000000000001.
export function toFile(range: Range, value: number): number {
  return round(value * range.scale);
}

function round(value: number): number {
  return Math.round(value * 1e6) / 1e6;
}

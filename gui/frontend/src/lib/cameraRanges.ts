// Slider ranges for the Camera Settings tab.
//
// PLACEHOLDERS. A camera's real limits are only known to whoever can talk to
// it: the vision_processor, through its SDK or V4L2. Once it reports them
// over the network (SSL_VPConfig), these become fallbacks. The number box next
// to each slider takes any value, so a range here never limits a setting.
//
// OPENCV uses the team's UC70 dev webcam (v4l2-ctl --list-ctrls), expressed in
// the units vision_processor's OpenCV driver actually sends, which aren't the
// ones config.yml documents (src/driver/opencvdriver.cpp):
//   - exposure: the file's value × 1000 becomes V4L2 exposure_time_absolute,
//     in 100 µs steps. The UC70 takes 3-2047 (0.3-204.7 ms).
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
      min: 0.01,
      max: 30,
      step: 0.01,
      unit: "ms",
      scale: 1,
      fallback: 5,
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
      min: 0.01,
      max: 30,
      step: 0.01,
      unit: "ms",
      scale: 1,
      fallback: 5,
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
      min: 3,
      max: 2047,
      step: 1,
      unit: "× 100 µs",
      scale: 0.001,
      fallback: 166,
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

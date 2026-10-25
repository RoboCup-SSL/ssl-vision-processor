// Camera Settings page (config/CameraPanel.svelte). Notes are the "Show
// extra tooltips" text beside a setting; warnings always show under it.
import { PLACEHOLDER_RANGE, SELECT_INSTANCE } from "./common";

export const camera = {
  heading: "Camera settings",
  // The banner above the video. `backticks` show as code.
  restartBanner: (camera: number): string =>
    `These apply when cam ${String(camera)}'s vision_processor restarts: it reads \`camera:\` only at startup.`,
  writesTo: (path: string): string => `The host writes them to \`${path}\`.`,
  noConfigPath:
    "This camera has no `config_path`, so the host doesn't write its config file; copy these to it by hand.",
  // The host's own vision_processor for this camera (gui started with -vp).
  process: {
    banner: (camera: number): string =>
      `The host runs cam ${String(camera)}'s vision_processor and restarts it when a setting it reads only at startup changes.`,
    state: {
      running: "Running",
      restarting: "Restarting",
      waiting: "Waiting to start",
      stopped: "Stopped",
    },
    pid: (pid: number): string => `pid ${String(pid)}`,
    restarts: (n: number): string =>
      n === 1 ? "1 restart" : `${String(n)} restarts`,
    pending: (keys: string[]): string =>
      `Restarting shortly for ${keys.join(", ")}.`,
    lastExit: (how: string): string => `Last run: ${how}.`,
    crashes: (n: number): string =>
      `Failed ${String(n)} times in a row; retrying, waiting longer each time. See its output below.`,
    restartNow: "Restart now",
    output: "Output",
  },
  slotChip:
    "Which region of the field this camera covers. Change it on the Camera Layout page.",
  noInstance: SELECT_INSTANCE,

  driver: [
    "SPINNAKER and MVIMPACT need vision_processor built with their SDKs.",
    "Unset means SPINNAKER.",
  ],
  cameraIndex: ["Position in the SDK's camera list, in detection order."],

  path: {
    notes: [
      "Device, image, or video file. Unset means /dev/video{id}.",
      "The list shows cameras on the host running this GUI.",
    ],
    byId: "Follows this camera to any USB port.",
    byPath: "Follows the USB port, whichever camera is in it.",
    notOnHost: "Not on this host; fine if the camera is on another machine.",
    unstable:
      "/dev/videoN can change when devices re-enumerate. A /dev/v4l/by-id path follows the camera.",
    kind: {
      "by-id": "this camera, any port",
      "by-path": "this USB port",
      node: "may change",
    },
  },

  resolution: {
    opencv: [
      "0 asks for the largest the camera offers.",
      "Check the size under the video: OpenCV doesn't always get what it asks for.",
    ],
    sdk: [
      "0 is the sensor's maximum.",
      "Bayer cameras are processed at half this internally.",
    ],
    notWhole: "Width and height must be whole numbers.",
    setBoth: "Set both, or 0 for both (the camera's maximum).",
  },

  exposure: {
    notes: [
      "Longer is brighter but blurs motion.",
      "At or above the frame time (33 ms at 30 fps) the frame rate drops.",
    ],
    opencv: [
      "V4L2 sets exposure in 100 µs steps. The host writes config.yml in the unit vision_processor's OpenCV driver expects (5000 µs is 0.05).",
    ],
    sdk: [
      "The host writes config.yml in milliseconds, the unit this driver reads.",
    ],
    // Beside the unit, while not automatic.
    ms: (micros: number): string => `${(micros / 1000).toFixed(2)} ms`,
    opencvAutoBroken:
      "vision_processor's OpenCV driver sends auto_exposure = 1 for Auto, which V4L2 cameras read as Manual: the camera keeps its last exposure.",
  },

  gain: {
    notes: [
      "Brighter but noisier. Noise means more false blobs and more processing time.",
      "Robot ids or team colors flickering means the image is too bright.",
      "0 means automatic, so a manual gain of exactly 0 isn't possible.",
    ],
    opencv: [
      "Auto leaves the camera's own setting: OpenCV doesn't turn auto gain on.",
      "Placeholder range: the UC70 dev camera's 0-8.",
    ],
    sdk: [PLACEHOLDER_RANGE],
  },

  gamma: {
    notes: [
      "Below 1 evens out bright and dark areas; above 1 adds color contrast.",
    ],
    opencv: [
      "V4L2 counts gamma × 100 and vision_processor sends the file's value unscaled, so 100 here is a gamma of 1.0.",
      "Placeholder range: the UC70 dev camera's 100-300.",
    ],
    sdk: [PLACEHOLDER_RANGE],
    opencvIgnored:
      "vision_processor's OpenCV driver currently ignores gamma other than 1.0 (opencvdriver.cpp:43 checks the wrong way round).",
  },

  whiteBalance: {
    profiles: [
      "Outdoor and indoor are Spinnaker's two auto profiles, for green and gray carpet.",
    ],
    mvimpact: ["Automatic calibrates once, from the first frame after start."],
    auto: ["Automatic turns on the camera's own auto white balance."],
    channelOpencv: [
      "Sent as V4L2 red/blue balance in camera units. Many UVC webcams only have a color temperature control, so this may do nothing; the UC70 dev camera has no red/blue controls at all.",
      "Placeholder range: generic 8-bit.",
    ],
    channelSpinnaker: "Balance ratio relative to green; 1.0 is neutral.",
    channelMvimpact: "Gain relative to green; 1.0 is neutral.",
  },
};

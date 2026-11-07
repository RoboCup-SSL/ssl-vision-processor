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
      "Shown in the camera's 100 µs steps: vision_processor sends the file's value × 1000, so 166 here is 16.6 ms and the file stores 0.166.",
      "Placeholder range: the UC70 dev camera's 3-2047.",
    ],
    sdk: [PLACEHOLDER_RANGE],
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

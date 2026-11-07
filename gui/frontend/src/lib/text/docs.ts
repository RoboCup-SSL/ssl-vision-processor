// Resolution docs for known alerts, behind the Alerts page's "How to fix":
// short, self-contained steps an operator can follow at the field. An alert
// links to one by id (AlertAction "doc"); the Alerts page shows it in a
// dialog. Add a doc here, then reference it
// from the alert in alerts/alerts.ts.
//
// Each step is plain text; a step starting with "$ " is shown as a command.

export interface ResolutionDoc {
  title: string;
  // Why it happens, in a sentence or two.
  cause: string;
  steps: string[];
}

export const RESOLUTION_DOCS = {
  "config-render-failed": {
    title: "A camera's config.yml can't be written",
    cause:
      "The host regenerates each local camera's config.yml from vision.yml on every change. Writing it failed, so that vision_processor keeps its old settings.",
    steps: [
      "Read the error on the alert: usually a missing directory or a permission problem.",
      "Check the camera's config_path in vision.yml points somewhere this host can write.",
      "Fix the path or permissions; the host retries on the next change.",
    ],
  },
  "camera-id-conflict": {
    title: "Two vision_processors send the same camera ID",
    cause:
      "Each vision_processor must have its own cam_id. Two with the same one overwrite each other's detections and fight over one calibration.",
    steps: [
      "Open Camera Layout and note the addresses listed for this camera ID.",
      "On one of those machines, change cam_id in its config.yml to the free region it actually covers.",
      "Restart that vision_processor. The conflict clears within a few seconds.",
    ],
  },
  "socket-problem": {
    title: "A network socket isn't fully open",
    cause:
      "The host couldn't join the multicast group or open a send socket, often because no network interface is usable (cable unplugged, suspended, or every interface skipped).",
    steps: [
      "Check the machine's network cable or Wi-Fi.",
      "On the Network page, check the host interfaces: with Auto off, at least one must be selected.",
      "The host retries every 5 s; nothing needs restarting once the network is back.",
    ],
  },
  "vision-silent": {
    title: "No detections heard",
    cause:
      "No vision_processor detection packets arrived on the vision address. Either nothing is running, or the address, port, or network differs.",
    steps: [
      "Check at least one vision_processor is running.",
      "On the Network page, check the vision address and port match its config.yml (defaults.network in vision.yml).",
      "Check both machines are on the same network and the switch passes multicast.",
    ],
  },
  "gc-silent": {
    title: "No game controller heard",
    cause:
      "No referee messages arrived on the game controller address. Fine when no game controller is running.",
    steps: [
      "If a game controller should be running, check its publish address and port match the Network page.",
      "Check both machines are on the same network.",
    ],
  },
  "team-height-missing": {
    title: "A team would get a default robot height",
    cause:
      "vision_processor looks each team up in its height table by its exact game controller name. A team that isn't listed keeps that color's previous height: the table's mean before any known team has played, otherwise the last known team's, which after a color switch is the other team's. Robots are then placed at the wrong height, which shifts them on the field.",
    steps: [
      "Open the Overview page and find the team under Teams.",
      "Turn off Derive from game controller, then pick the table entry that matches the team, or Custom and enter its robot height in mm.",
      "Save. The override is kept by team name, so it follows the team if the colors are switched.",
      "Until vision_processor reads these overrides, also add the team to the height table (bot_heights_file, robot-heights.yml by default) under its exact game controller name, and restart the vision_processors.",
    ],
  },
  "heights-unreadable": {
    title: "The robot height table can't be read",
    cause:
      "vision_processor needs its bot_heights_file to place robots at the right height, and fails to start without it.",
    steps: [
      "Check the path in vision.yml's defaults.bot_heights_file (robot-heights.yml when unset). A relative path is relative to where vision_processor runs.",
      "Check the file is valid YAML: one `team name: height in mm` per line.",
    ],
  },
  "loopback-multicast-off": {
    title: "Multicast is off on the loopback interface",
    cause:
      "Programs on this machine that multicast over loopback (e.g. a vision_processor and the GUI on one laptop with no network) won't hear each other.",
    steps: [
      "Enable it until the next reboot:",
      "$ sudo ip link set lo multicast on",
      "Not needed when every program uses a real network interface.",
    ],
  },
  "region-uncovered": {
    title: "A region of the field has no camera",
    cause:
      "The camera count splits the field into regions, and no camera is assigned to this one. Nothing there is detected.",
    steps: [
      "Open Camera Layout.",
      "Assign a camera to the region, add the vision_processor heard for it, or lower the camera count.",
    ],
  },
  "camera-unplaced": {
    title: "A vision_processor isn't in the layout",
    cause:
      "Detections arrive with a camera ID that no camera in vision.yml has, so the GUI can't configure or calibrate it.",
    steps: [
      "Open Camera Layout and find it under Heard but not placed.",
      "Add it as a camera, or fix its cam_id if it's wrong.",
    ],
  },
  "file-changed": {
    title: "vision.yml changed on disk",
    cause:
      "Something other than this GUI edited vision.yml since it was loaded or saved.",
    steps: [
      "Open the settings menu (gear, top right).",
      "Load from disk to take the file's version, or Save to overwrite it with the GUI's.",
    ],
  },
  "seed-stale": {
    title: "Line corners were picked for another region",
    cause:
      "The camera moved to a different region (Camera Layout), so its picked corners mark the wrong field corners.",
    steps: [
      "Open the camera's Geometry tab.",
      "Move the corner markers onto the corners of the new region; the first goes on the region's (minX, minY) corner, as the picker says.",
      "Restart the vision_processor so it recalibrates.",
    ],
  },
  "seed-resolution": {
    title: "Line corners don't match the camera resolution",
    cause:
      "The corners were picked on an image of a different size than the camera now sends.",
    steps: [
      "Open the camera's Geometry tab.",
      "Same aspect ratio: use Rescale corners. Different aspect ratio: re-pick them.",
    ],
  },
  "seed-resolution-unknown": {
    title: "Line corners have no recorded resolution",
    cause:
      "They were saved before the resolution was recorded (e.g. imported from an old config.yml), so a resolution change can't be detected.",
    steps: [
      "Open the camera's Geometry tab and move any marker; that records the resolution.",
    ],
  },
  "calibration-field-changed": {
    title: "Locked calibration predates a field change",
    cause:
      "The field dimensions changed after this calibration was locked, so it was solved against the old field lines.",
    steps: [
      "Open the camera's Geometry tab.",
      "Unlock the calibration and restart the vision_processor to recalibrate, then lock the new one.",
    ],
  },
  "calibration-resolution": {
    title: "Locked calibration doesn't match the camera resolution",
    cause:
      "The calibration was solved at a different aspect ratio than the camera now sends; vision_processor can't rescale it.",
    steps: [
      "Open the camera's Geometry tab.",
      "Unlock, restart the vision_processor to recalibrate, then lock again.",
    ],
  },
} satisfies Record<string, ResolutionDoc>;

export type DocId = keyof typeof RESOLUTION_DOCS;

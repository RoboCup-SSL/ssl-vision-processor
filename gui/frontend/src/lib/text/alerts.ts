// The Alerts page and its sidebar summary (alerts/). Each alert's "How to
// fix" text is in docs.ts.

export const alerts = {
  heading: "Alerts",
  intro:
    "Known problems, live. Errors mean something isn't working; warnings are likely wrong and worth fixing before a match; cautions are worth knowing and often fine.",
  severities: { error: "Errors", warning: "Warnings", caution: "Cautions" },
  empty: {
    error: "No errors.",
    warning: "No warnings.",
    caution: "No cautions.",
  },

  // Action buttons.
  howToFix: "How to fix",
  pages: {
    overview: "Overview",
    layout: "Camera Layout",
    geometry: "Geometry",
    network: "Network",
  },

  renderFailed: (camera: string): string =>
    `Can't write camera ${camera}'s config.yml`,
  conflict: (camera: number): string =>
    `Camera ID ${String(camera)} is sent by several vision_processors`,
  heardFrom: (addresses: string[]): string =>
    `Heard from ${addresses.join(", ")}.`,
  uncovered: (camera: number, region: string): string =>
    `No camera covers region ${String(camera)} (${region})`,
  uncoveredDetail: "Nothing in that part of the field is detected.",
  unplaced: (camera: number, address: string): string =>
    `Camera ${String(camera)} at ${address} isn't in the layout`,
  unplacedDetail:
    "It sends detections, but vision.yml has no camera with that ID.",
  fileChanged: (path: string): string => `${path} changed on disk`,
  fileChangedDetail: (differences: number): string =>
    `${String(differences)} difference(s) from what the GUI has.`,
  socket: (label: string, problem: string): string =>
    `${label} socket: ${problem}`,
  socketDetail: (address: string): string =>
    `On ${address}. The host retries every 5 s.`,
  socketLabels: { vision: "Vision", gc: "Game controller" },
  visionSilent: "No detections heard",
  gcSilent: "No game controller heard",
  silentDetail: (address: string, everHeard: boolean): string =>
    `Nothing received on ${address}${everHeard ? " recently" : " yet"}.`,
  teamDefaultHeight: (color: string, team: string): string =>
    `${color} team "${team}" would get a default robot height`,
  colorDefaultHeight: (color: string): string =>
    `${color} robots would get a default robot height`,
  heightsUnreadable: (file: string): string =>
    `Can't read the robot height table ${file}`,
  loopback: (iface: string): string => `Multicast is off on ${iface}`,
  loopbackDetail:
    "Only matters when programs on this machine talk over loopback.",

  // Titles for the host's per-camera warning codes (gui/internal/config's
  // status.go). The host's own message is the detail. Unlisted codes show
  // the code itself.
  cameraWarning: (camera: number, code: string): string =>
    `Camera ${String(camera)}: ${CAMERA_WARNING_TITLES[code] ?? code.replaceAll("_", " ")}`,
};

const CAMERA_WARNING_TITLES: Record<string, string> = {
  field_changed: "calibration predates a field change",
  calibration_aspect_mismatch: "calibration doesn't match the resolution",
  seed_stale: "line corners picked for another region",
  seed_aspect_mismatch: "line corners don't match the resolution",
  seed_rescalable: "line corners need rescaling",
  seed_resolution_unknown: "line corners have no recorded resolution",
};

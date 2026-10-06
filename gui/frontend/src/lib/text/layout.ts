// Camera Layout page (config/LayoutPanel.svelte, config/LayoutMap.svelte).
// Functions fill in values; edit the words around them freely.

const plural = (n: number, word: string): string =>
  `${word}${n === 1 ? "" : "s"}`;

export const layout = {
  heading: "Camera Layout",
  intro:
    "Each vision_processor covers one region of the field. Its camera ID and the camera count decide which region, and so the corners its calibration fits to. One host can run any number of cameras.",

  cameraCount: [
    "vision_processor's geometry.camera_amount. The field is halved along its longer side for each doubling.",
  ],
  uncovered: (ids: number[]): string =>
    `No camera covers ${plural(ids.length, "region")} ${ids.join(", ")}.`,
  countTooSmall: "Move cameras with higher IDs to lower ones first.",
  eachRegion: "plus the boundary on outer edges",

  axes: (flipX: boolean, flipY: boolean): string =>
    `+x points ${flipX ? "left" : "right"}, +y ${flipY ? "down" : "up"}`,
  mapHelp:
    "Click a region to select it; it plays live. The view is saved in this browser only.",

  conflict:
    "Several vision_processors send this camera ID. Each must have its own; check their cam_id.",
  cameraPicker: ["Picking a camera that covers another region swaps the two."],
  videoOrientation: [
    "Turns this camera's video to match the field, for a camera mounted at an angle. Only the GUI uses it.",
  ],
  remote:
    "Remote: this host can't write its config.yml yet. Set cam_id there by hand.",

  unplaced: {
    title: "Heard but not placed",
    intro:
      "These vision_processors send detections with a camera ID that no camera here has.",
    outside: (count: number): string =>
      `Outside the ${String(count)}-camera layout; its cam_id is wrong or the count is too low.`,
  },

  // The confirm dialog for a swap, move, or count change.
  confirm: {
    splitTitle: (count: number): string =>
      `Split the field ${String(count)} ${plural(count, "way")}`,
    swapTitle: (a: number, b: number): string =>
      `Swap cameras ${String(Math.min(a, b))} and ${String(Math.max(a, b))}`,
    moveTitle: (from: number, to: number): string =>
      `Move camera ${String(from)} to ${String(to)}`,
    moved: (from: number, to: number): string =>
      `camera ${String(from)} → ${String(to)}`,
    newRegion: (id: number): string => `camera ${String(id)}, new region`,
    settingsStay: "Camera and color settings stay with it.",
    seedStale:
      "Its line corners are kept but marked for re-picking: they were picked for the old region.",
    calibrationRemoved:
      "Its locked calibration is removed. It recalibrates when its vision_processor restarts.",
    remote: "Remote: set its cam_id by hand.",
    restart:
      "vision_processor reads its camera ID and count only at startup. Restart the affected ones after saving.",
  },
};

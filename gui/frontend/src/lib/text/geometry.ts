// Geometry page (config/GeometryPanel.svelte, config/CalibrationCard.svelte).

export const geometry = {
  heading: "Geometry",
  // `backticks` show as code.
  intro: (host: string, camera: number): string =>
    `${host} / cam ${String(camera)}. The numeric settings below (config.yml's \`geometry:\` block) aren't editable here yet; the calibration and corner picker are, and apply live.`,

  calibration: {
    locked: "Locked",
    lockedDetail: (when: string, resolution: string): string =>
      `at ${when} (${resolution}). Published in place of anything the camera sends, and kept across restarts once saved.`,
    live: "Live, not locked",
    liveDetail:
      ". The camera calibrated itself; it recalibrates on its next restart unless you lock this.",
    none: "None.",
    noneDetail: "No calibration received from this camera yet.",
    unlock: "Unlock (recalibrates on processor restart)",
    footer:
      "Locking and unlocking apply immediately; Save writes them to the file. There's no way yet to make a running vision_processor recalibrate on request -- it needs a restart.",
  },

  cornerPicker: {
    title: "Corner picker",
    intro:
      "Drag the four markers onto the real field corners in the image below, then click the number on whichever one sits where the goal line meets the touchline nearest this field's (0,0) corner -- that one turns green and becomes first in the output. The other three can be in any order; the calibration algorithm works that out itself. Changes apply when you let go of a marker; Save keeps them.",
    region: (
      camera: number,
      count: number,
      label: string,
      bounds: { minX: number; maxX: number; minY: number; maxY: number },
    ): string =>
      `This camera covers region ${String(camera)} of ${String(count)} (${label}): x ${String(bounds.minX)} … ${String(bounds.maxX)}, y ${String(bounds.minY)} … ${String(bounds.maxY)} mm. The first marker goes on its (${String(bounds.minX)}, ${String(bounds.minY)}) corner.`,
    rescalable: (picked: string, image: string): string =>
      `Corners were picked at ${picked}, but this image is ${image}. Same aspect ratio, so they can be scaled to fit.`,
    rescale: (image: string): string => `Rescale corners to ${image}`,
    otherAspect: (picked: string, image: string): string =>
      `Corners were picked at ${picked}, but this image is ${image} -- a different aspect ratio, so they can't be scaled. Re-pick them on this image.`,
    unknownResolution:
      "These corners were saved without the resolution they were picked at, so a camera resolution change can't be detected. Move any marker to record it.",
  },

  lockTitle: "Store the calibration this camera last sent",
  nothingToLock: "No calibration received from this camera yet",
  relock: "Re-lock latest live calibration",
  lock: "Lock current calibration",
  unlockTitle: "Unlock calibration?",
  unlockConfirm: "Unlock",
  confirmUnlock: (camera: number): string =>
    `Unlock camera ${String(camera)}'s calibration? It stops being published now, and the camera recalibrates the next time its vision_processor restarts.`,
};

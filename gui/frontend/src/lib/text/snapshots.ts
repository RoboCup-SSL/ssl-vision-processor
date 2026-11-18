// Debug image cards (SnapshotViews.svelte) on the Color and Geometry pages:
// images vision_processor writes to img/, which the host lists and serves.

export interface SnapshotView {
  // The <view> in img/<camera>.<view>.<ext>.
  view: string;
  label: string;
}

export const snapshots = {
  refresh: "Refresh",
  failed: (reason: string): string => `Couldn't list the images: ${reason}`,

  color: {
    title: "Detection views",
    intro:
      "What ball and robot detection works from: the flat image and the blob centers found in it. Click one for full size.",
    empty: (camera: number): string =>
      `Camera ${String(camera)} hasn't written these yet. It writes them once calibrated, at the Snapshot refresh interval on the Advanced page (off by default).`,
    views: [
      { view: "flat", label: "Flat" },
      { view: "blob", label: "Blobs" },
    ] satisfies SnapshotView[],
  },

  geometry: {
    title: "Calibration views",
    intro:
      "What this camera's last calibration saw: the field line pixels it found, the line segments, and the corner and refined fits drawn over the pixels. Click one for full size.",
    empty: (camera: number): string =>
      `Camera ${String(camera)} hasn't written these yet. It writes them each time it calibrates.`,
    views: [
      { view: "pixels", label: "Line pixels" },
      { view: "lines", label: "Line segments" },
      { view: "pixels.corner", label: "Corner fit" },
      { view: "pixels.refined", label: "Refined fit" },
    ] satisfies SnapshotView[],
  },
};

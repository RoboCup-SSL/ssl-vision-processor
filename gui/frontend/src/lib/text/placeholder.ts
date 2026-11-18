// The Stream page (config/StreamPanel.svelte). `backticks` show as code.

export const stream = {
  heading: "Live video",
  intro: (host: string, camera: number): string =>
    `${host} / cam ${String(camera)}'s H.264 stream, relayed by the host without re-encoding. vision_processor cycles through its views (raw, then processed) unless \`stream.raw_feed\` is set. For exact pixels, such as picking corners, use the snapshots on the Geometry tab.`,
};

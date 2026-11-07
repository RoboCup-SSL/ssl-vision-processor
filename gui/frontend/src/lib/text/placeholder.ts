// Pages not built yet (config/ConfigCategoryPlaceholder.svelte), and the
// Stream page (config/StreamPanel.svelte). `backticks` show as code.

export const placeholder = {
  editing: (host: string, camera: number, key: string): string =>
    `Editing ${host} / cam ${String(camera)}'s \`${key}:\` block. Not wired to a backend yet -- there is nowhere to read or write a specific instance's config.yml over the network. See \`internal/config\` in gui/CLAUDE.md's "Not yet built".`,
  contributing:
    "Contributing this panel? Replace this file with a real form for the fields below (see config.yml at the repo root for exact defaults/ranges).",
};

export const stream = {
  heading: "Live video",
  intro: (host: string, camera: number): string =>
    `${host} / cam ${String(camera)}'s H.264 stream, relayed by the host without re-encoding. vision_processor cycles through its views (raw, then processed) unless \`stream.raw_feed\` is set. For exact pixels, such as picking corners, use the snapshots on the Geometry tab.`,
};

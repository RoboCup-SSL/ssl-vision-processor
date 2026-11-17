// Color page (config/ColorPanel.svelte, config/YuvPositionPane.svelte,
// config/WeightTriangle.svelte). `backticks` show as code.

export const color = {
  heading: "Color",
  editing: (host: string, camera: number): string =>
    `Editing ${host} / cam ${String(camera)}'s \`color:\` block.`,

  updateWeights: {
    heading: "Update weights",
    intro:
      "How much each new color leans on its configured reference vs. last frame's color vs. what was actually sampled this frame. Drag the marker, or edit the minimum reference weight directly -- see src/blobs/colorupdate.cpp's updateColor for the exact blend this mirrors.",
    unlockTitle: "Unlock the reference weight floor?",
    unlockBody:
      "This floor keeps the reference color from being tuned toward 0. Reference is the only weight never gated on having samples this frame -- without a floor, a color that goes sample-starved for a while has nothing left pulling it back toward its configured value.",
    unlockConfirm: "I understand, unlock",
  },

  picker: {
    restoreAllTitle: "Restore all colors?",
    restoreAllBody:
      "Resets all six reference colors to their defaults. Undo can step back through them one color at a time.",
    smallGamut: (percent: number, dark: boolean): string =>
      `Only ${String(percent)}% of the square is a real color at this brightness -- the outlined shape, not a full square, is expected here: brightness this close to ${dark ? "black" : "white"} genuinely has few reachable colors. Not a rendering glitch.`,
    help: (label: string): string =>
      `Drag or click inside the square to set ${label}'s color at the brightness shown. Dragging the slider (or scrolling over either control) only previews a different brightness -- the marker stays exactly where the saved color actually is, even inside the shaded region if that color isn't reachable at the previewed brightness; nothing is saved until you click or drag inside the square again. Ctrl+Z (Cmd+Z on Mac) undoes the last drag, one gesture at a time. The currently-autoadapted position and per-frame blob samples aren't shown yet: both need the other contributor's protobuf work to reach this host.`,
    defaultLabel: "the reference color",
  },
};

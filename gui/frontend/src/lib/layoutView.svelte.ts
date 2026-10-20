// How this browser shows the Camera Layout map. Per browser, in
// localStorage, since each operator may stand on a different side of the
// field. Same pattern as preferences.svelte.ts.
const STORAGE_KEY = "vision-processor-gui:layout-view";

export type FeedMode = "off" | "keyframes" | "live";

interface LayoutView {
  // Turns the field around, for an operator on the other side of it.
  rotate180: boolean;
  // Flips the field left to right.
  mirror: boolean;
  // Video in the tiles: none, about one frame a second (the selected tile
  // still plays live), or every tile live.
  feeds: FeedMode;
}

function load(): LayoutView {
  const defaults: LayoutView = {
    rotate180: false,
    mirror: false,
    feeds: "keyframes",
  };

  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return defaults;

    return { ...defaults, ...(JSON.parse(raw) as Partial<LayoutView>) };
  } catch {
    return defaults;
  }
}

export const layoutView = $state<LayoutView>(load());

$effect.root(() => {
  $effect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(layoutView));
    } catch {
      // Storage disabled: the view just resets on reload.
    }
  });
});

// Whether field x and y run backwards on screen.
export function flips(): { x: boolean; y: boolean } {
  return {
    x: layoutView.rotate180 !== layoutView.mirror,
    y: layoutView.rotate180,
  };
}

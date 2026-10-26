// How this browser shows the Live Field. Per viewer, in localStorage, like
// preferences.svelte.ts: nothing here is part of vision.yml.
import { VISION } from "./sources";

const STORAGE_KEY = "vision-processor-gui:live-field";

export type RobotStyle = "numbered" | "pattern";

export interface Overlay {
  on: boolean;
  // 0 to 1.
  opacity: number;
}

export interface LiveView {
  robotStyle: RobotStyle;
  // The source drawn on top, fully opaque.
  primary: string;
  // Other sources by key, drawn under the primary when on.
  overlays: Record<string, Overlay>;
  // Vision cameras left out of the raw detections.
  hiddenCameras: number[];
  // Outlines each raw detection in its camera's color.
  cameraTint: boolean;
  velocities: boolean;
  // Swaps sides: draws the field rotated by 180 degrees.
  flipped: boolean;
}

export const DEFAULT_OVERLAY: Overlay = { on: false, opacity: 0.4 };

function defaults(): LiveView {
  return {
    robotStyle: "numbered",
    primary: VISION,
    overlays: {},
    hiddenCameras: [],
    cameraTint: false,
    velocities: false,
    flipped: false,
  };
}

function load(): LiveView {
  const view = defaults();

  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return view;

    const saved = JSON.parse(raw) as Partial<LiveView>;

    return { ...view, ...saved };
  } catch {
    return view;
  }
}

export const liveView = $state<LiveView>(load());

$effect.root(() => {
  $effect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(liveView));
    } catch {
      // Storage disabled: the view just won't persist across reloads.
    }
  });
});

export function overlayFor(key: string): Overlay {
  return liveView.overlays[key] ?? DEFAULT_OVERLAY;
}

export function setOverlay(key: string, change: Partial<Overlay>): void {
  liveView.overlays[key] = { ...overlayFor(key), ...change };
}

export function toggleCamera(camera: number, shown: boolean): void {
  const hidden = liveView.hiddenCameras.filter((c) => c !== camera);
  liveView.hiddenCameras = shown ? hidden : [...hidden, camera];
}

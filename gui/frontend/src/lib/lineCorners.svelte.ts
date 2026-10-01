// Shared state for the corner picker, mirroring geometry.svelte.ts's pattern
// for the virtual field editor: a module-level $state object plus load/save
// functions built on the shared api.ts helpers, instead of the corner picker
// hand-rolling its own one-off fetch/loading/error shape.
import { requestJSON, withLoadingState } from "./api";

export interface Corner {
  x: number;
  y: number;
}

interface LineCornersResponse {
  corners?: Corner[];
  goalSideMarker?: number;
}

export const lineCorners = $state<{
  corners: Corner[];
  goalSideMarker: number | null;
  loading: boolean;
  saving: boolean;
  error: string | null;
  savedAt: number | null;
}>({
  corners: [],
  goalSideMarker: null,
  loading: false,
  saving: false,
  error: null,
  savedAt: null,
});

// Loads whatever was last saved to the instance's config.yml (see
// geometry.ReadLineCorners). lineCorners.corners stays empty if nothing's
// been saved yet, or if what's there isn't a complete set of 4 -- that's the
// normal, not-yet-calibrated state, not a failure.
export async function loadLineCorners(): Promise<void> {
  await withLoadingState(
    (v) => (lineCorners.loading = v),
    (v) => (lineCorners.error = v),
    async () => {
      const response = await requestJSON("/api/config/line-corners");
      const data = (await response.json()) as LineCornersResponse;
      lineCorners.corners = data.corners?.length === 4 ? data.corners : [];
      lineCorners.goalSideMarker = data.goalSideMarker ?? null;
    },
  );
}

// Saves corners (already reordered so the goal-side corner is first -- see
// orderedCorners in CornerPicker.svelte) and which original on-screen marker
// number (1-4) was chosen, to the instance's config.yml (see
// geometry.WriteLineCorners).
export async function saveLineCorners(
  corners: Corner[],
  goalSideMarker: number,
): Promise<void> {
  lineCorners.savedAt = null;

  await withLoadingState(
    (v) => (lineCorners.saving = v),
    (v) => (lineCorners.error = v),
    async () => {
      await requestJSON("/api/config/line-corners", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ corners, goalSideMarker }),
      });

      lineCorners.savedAt = Date.now();
    },
  );
}

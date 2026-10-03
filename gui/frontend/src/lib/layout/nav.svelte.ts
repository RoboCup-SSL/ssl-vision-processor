// Shared navigation state for the two-column shell: which vision-role
// instance and which config category are selected. A module-level $state
// object, per the project convention (see CLAUDE.md).
import { CONFIG_CATEGORIES, type ConfigCategory } from "./configCategories";
import { config } from "../config.svelte";

// One row in the instance list: a single camera role, keyed by camera_id (its
// position on the field) -- a host running all 4 cameras of a quad setup
// appears as 4 rows. Comes from vision.yml's cameras list.
export interface VisionInstance {
  id: string;
  host: string;
  cameraId: number;
}

export const nav = $state<{
  selectedInstanceId: string | null;
  selectedCategoryId: string;
}>({
  selectedInstanceId: null,
  selectedCategoryId: "field",
});

export function instances(): VisionInstance[] {
  return (config.doc?.cameras ?? []).map((c) => ({
    id: `cam:${String(c.cameraId)}`,
    host: c.instance ?? "",
    cameraId: c.cameraId,
  }));
}

// The selected camera, or the first one until something's been picked.
export function selectedInstance(): VisionInstance | undefined {
  const all = instances();

  return all.find((i) => i.id === nav.selectedInstanceId) ?? all[0];
}

export function selectedCategory(): ConfigCategory {
  const found = CONFIG_CATEGORIES.find((c) => c.id === nav.selectedCategoryId);
  if (found) return found;

  // "field" is always present in CONFIG_CATEGORIES (see configCategories.ts);
  // a thrown error here means that constant was edited to remove it, which
  // is a real bug worth surfacing loudly rather than typing around.
  const fallback = CONFIG_CATEGORIES.find((c) => c.id === "field");
  if (fallback) return fallback;

  throw new Error("CONFIG_CATEGORIES is missing the 'field' category");
}

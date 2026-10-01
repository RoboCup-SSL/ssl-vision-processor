// Shared navigation state for the two-column shell: which vision-role
// instance and which config category are selected. A module-level $state
// object, per the project convention (see CLAUDE.md).
import { CONFIG_CATEGORIES, type ConfigCategory } from "./configCategories";

// One row in the instance list: a single camera role on a single host, not a
// machine. A host running all 4 cameras of a quad setup appears as 4 rows
// sharing the same host, one per camera_id -- see the note in App.svelte
// about internal/discovery not existing yet.
export interface VisionInstance {
  id: string;
  host: string;
  cameraId: number;
}

// TODO(discovery): mock data. Replace with instances built from SSL_VPConfig
// announces once internal/discovery exists (see gui/CLAUDE.md's "Not yet
// built"). Kept non-empty so the panel has something to click while that's
// unbuilt, rather than looking broken.
const MOCK_INSTANCES: VisionInstance[] = [
  { id: "camtest:0", host: "camtest-host", cameraId: 0 },
  { id: "camtest:1", host: "camtest-host", cameraId: 1 },
];

export const nav = $state<{
  instances: VisionInstance[];
  selectedInstanceId: string | null;
  selectedCategoryId: string;
}>({
  instances: MOCK_INSTANCES,
  selectedInstanceId: MOCK_INSTANCES[0]?.id ?? null,
  selectedCategoryId: "field",
});

export function selectedInstance(): VisionInstance | undefined {
  return nav.instances.find((i) => i.id === nav.selectedInstanceId);
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

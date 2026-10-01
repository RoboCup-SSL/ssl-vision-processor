<script lang="ts">
  import { nav } from "./nav.svelte";
  import { CONFIG_CATEGORIES, TAB_CATEGORY_IDS } from "./configCategories";

  // Same source data as the sidebar (CONFIG_CATEGORIES), just a curated
  // subset in a fixed order -- both bind to nav.selectedCategoryId, so
  // picking a tab here and picking the matching entry in the sidebar do the
  // exact same thing and stay in sync automatically.
  let tabs = $derived(
    TAB_CATEGORY_IDS.map((id) =>
      CONFIG_CATEGORIES.find((c) => c.id === id),
    ).filter((c) => c !== undefined),
  );
</script>

<div class="tab-bar" role="tablist">
  {#each tabs as tab (tab.id)}
    <button
      type="button"
      role="tab"
      aria-selected={tab.id === nav.selectedCategoryId}
      class:selected={tab.id === nav.selectedCategoryId}
      class:shared={tab.scope === "shared"}
      onclick={() => {
        nav.selectedCategoryId = tab.id;
      }}
    >
      {tab.label}
    </button>
  {/each}
</div>

<style>
  .tab-bar {
    display: flex;
    gap: 0.25rem;
    border-bottom: 1px solid #ddd;
    margin-bottom: 1rem;
  }

  button {
    padding: 0.5rem 1rem;
    border: none;
    border-bottom: 2px solid transparent;
    background: none;
    font-size: 0.9rem;
    cursor: pointer;
    color: #555;
  }

  button:hover {
    color: #000;
  }

  button.selected {
    color: #1a56db;
    border-bottom-color: #1a56db;
    font-weight: 600;
  }

  /* The one shared (non-per-instance) tab gets a visual break after it, so
     it doesn't read as just another camera-specific setting. */
  button.shared {
    margin-right: 0.5rem;
    padding-right: 1rem;
    border-right: 1px solid #ddd;
  }
</style>

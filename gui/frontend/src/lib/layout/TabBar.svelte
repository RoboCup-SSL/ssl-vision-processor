<script lang="ts">
  import { tabs as tabsTheme, tabItem } from "flowbite-svelte";
  import { nav, isCategoryDirty, categoryHref } from "./nav.svelte";
  import { CONFIG_CATEGORIES, TAB_CATEGORY_IDS } from "./configCategories";

  // Same source data as the sidebar (CONFIG_CATEGORIES), just a curated
  // subset in a fixed order -- both follow nav.selectedCategoryId, so picking
  // a tab here and picking the matching entry in the sidebar do the exact
  // same thing and stay in sync automatically.
  let tabs = $derived(
    TAB_CATEGORY_IDS.map((id) =>
      CONFIG_CATEGORIES.find((c) => c.id === id),
    ).filter((c) => c !== undefined),
  );

  // Flowbite's Tabs component keeps exactly one tab selected and ignores
  // pages that aren't tabs (Network, Stream, ... from the sidebar), so it
  // can't follow nav. These are plain links styled with its own "underline"
  // theme instead: they look like Flowbite Tabs, and none is selected on a
  // page that isn't a tab.
  const theme = tabsTheme({ tabStyle: "underline" });
  const item = tabItem();
</script>

<ul
  class={theme.base({ class: "mb-4 border-b border-gray-200" })}
  role="tablist"
>
  {#each tabs as tab, i (tab.id)}
    {@const selected = tab.id === nav.selectedCategoryId}
    <li
      class={item.base({
        // A break after the last shared tab, so the camera-specific ones
        // read as their own group.
        class:
          tab.scope === "shared" && tabs[i + 1]?.scope !== "shared"
            ? "me-2 border-e border-gray-200 pe-2"
            : "",
      })}
      role="presentation"
    >
      <a
        href={categoryHref(tab.id)}
        role="tab"
        aria-selected={selected}
        class={item.button({
          class: selected
            ? theme.active({ class: "px-4 py-2.5 font-semibold" })
            : theme.inactive({ class: "px-4 py-2.5" }),
        })}
      >
        {tab.label}{isCategoryDirty(tab.id) ? "*" : ""}
      </a>
    </li>
  {/each}
</ul>

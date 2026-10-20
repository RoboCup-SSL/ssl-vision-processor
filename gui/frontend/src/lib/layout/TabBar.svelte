<script lang="ts">
  import { tabs as tabsTheme, tabItem } from "flowbite-svelte";
  import {
    nav,
    isCategoryDirty,
    categoryHref,
    selectedInstance,
    isCategoryVisible,
  } from "./nav.svelte";
  import {
    CONFIG_CATEGORIES,
    TAB_CATEGORY_IDS,
    type ConfigCategory,
  } from "./configCategories";
  import { app } from "../text/app";

  // Same source data as the sidebar (CONFIG_CATEGORIES), just a curated
  // subset in a fixed order -- both follow nav.selectedCategoryId, so picking
  // a tab here and picking the matching entry in the sidebar do the exact
  // same thing and stay in sync automatically.
  let tabs = $derived(
    TAB_CATEGORY_IDS.map((id) =>
      CONFIG_CATEGORIES.find((c) => c.id === id),
    ).filter(
      (c): c is ConfigCategory => c !== undefined && isCategoryVisible(c),
    ),
  );

  // Shared settings, then the selected camera's, each under a header that
  // only labels the group.
  let groups = $derived<{ label: string; tabs: ConfigCategory[] }[]>([
    {
      label: app.tabGroups.global,
      tabs: tabs.filter((t) => t.scope === "shared"),
    },
    {
      label: app.tabGroups.perCamera(selectedInstance()?.cameraId),
      tabs: tabs.filter((t) => t.scope === "per-instance"),
    },
  ]);

  // Flowbite's Tabs component keeps exactly one tab selected and ignores
  // pages that aren't tabs (Stream, Debug, ... from the sidebar), so it
  // can't follow nav. These are plain links styled with its own "underline"
  // theme instead: they look like Flowbite Tabs, and none is selected on a
  // page that isn't a tab.
  const theme = tabsTheme({ tabStyle: "underline" });
  const item = tabItem();
</script>

<div
  class="mb-4 flex flex-wrap items-end gap-x-4 border-b border-gray-200 dark:border-gray-700"
>
  {#each groups as group, i (i)}
    <div class="flex flex-col">
      <div
        class="rounded-b-md border border-t-0 border-gray-200 bg-gray-50 px-3 py-0.5 text-center text-xs font-semibold tracking-wider text-gray-500 uppercase dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
      >
        {group.label}
      </div>
      <ul class={theme.base({ class: "flex-nowrap" })} role="tablist">
        {#each group.tabs as tab (tab.id)}
          {@const selected = tab.id === nav.selectedCategoryId}
          <li class={item.base()} role="presentation">
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
              {tab.label}<!-- Always there, so a tab doesn't change width as
                   its unsaved-changes asterisk comes and goes.
              --><span
                class:invisible={!isCategoryDirty(tab.id)}
                aria-hidden={!isCategoryDirty(tab.id)}>*</span
              >
            </a>
          </li>
        {/each}
      </ul>
    </div>
  {/each}
</div>

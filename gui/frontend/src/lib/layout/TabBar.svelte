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
  import { untrack } from "svelte";
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
  let groups = $derived.by(() => {
    const camera = selectedInstance()?.cameraId;

    return [
      {
        label: app.tabGroups.global,
        short: app.tabGroups.globalShort,
        tabs: tabs.filter((t) => t.scope === "shared"),
      },
      {
        label: app.tabGroups.perCamera(camera),
        short: app.tabGroups.perCameraShort(camera),
        tabs: tabs.filter((t) => t.scope === "per-instance"),
      },
    ];
  });

  // Flowbite's Tabs component keeps exactly one tab selected and ignores
  // pages that aren't tabs (Stream, Debug, ... from the sidebar), so it
  // can't follow nav. These are plain links styled with its own "underline"
  // theme instead: they look like Flowbite Tabs, and none is selected on a
  // page that isn't a tab.
  const theme = tabsTheme({ tabStyle: "underline" });
  const item = tabItem();

  // Always one row, never wrapping (labels breaking in two, groups
  // stacking). While the groups' headers fit, they sit over the tabs; when
  // they don't, the row goes compact, with short labels inline, and scrolls
  // sideways if even that doesn't fit. Measured like the header's socket
  // badges (Shell.svelte): the width the full row needed is remembered, and
  // it comes back once there's that much room again. The edge with more tabs
  // past it fades, so it's clear there's more.
  let scroller = $state<HTMLElement>();
  let compact = $state(false);
  let fullWidth = 0;
  let fadeLeft = $state(false);
  let fadeRight = $state(false);

  function fit(): void {
    if (!scroller) return;

    if (!compact && scroller.scrollWidth > scroller.clientWidth) {
      fullWidth = scroller.scrollWidth;
      compact = true;
    } else if (compact && scroller.clientWidth >= fullWidth) {
      compact = false;
    }

    updateFades();
  }

  function updateFades(): void {
    if (!scroller) return;

    fadeLeft = scroller.scrollLeft > 0;
    fadeRight =
      scroller.scrollLeft + scroller.clientWidth < scroller.scrollWidth - 1;
  }

  $effect(() => {
    if (!scroller) return;

    const observer = new ResizeObserver(fit);
    observer.observe(scroller);

    return () => {
      observer.disconnect();
    };
  });

  // The scroller's own size doesn't change when its contents do, so measure
  // again after switching style, and when the tabs or labels change (expert
  // mode, another camera).
  $effect(() => {
    void compact;
    void groups;
    untrack(fit);
  });

  // Keeps the selected tab in view, picked here or from the sidebar. By hand
  // rather than scrollIntoView, which would also scroll the page to the bar.
  const REVEAL_MARGIN = 32;

  $effect(() => {
    void nav.selectedCategoryId;
    if (!scroller) return;

    const tab = scroller.querySelector('[aria-selected="true"]');
    if (!tab) return;

    const box = scroller.getBoundingClientRect();
    const tabBox = tab.getBoundingClientRect();

    if (tabBox.left < box.left) {
      scroller.scrollLeft -= box.left - tabBox.left + REVEAL_MARGIN;
    } else if (tabBox.right > box.right) {
      scroller.scrollLeft += tabBox.right - box.right + REVEAL_MARGIN;
    }

    updateFades();
  });
</script>

<!-- The bottom line is an inset shadow on the scroller rather than a border
     the tabs overlap (Flowbite's -mb-px), which the scroller would clip. -->
<div
  bind:this={scroller}
  onscroll={updateFades}
  class={[
    "tab-scroller mb-4 flex overflow-x-auto",
    compact ? "items-center" : "items-end gap-x-4",
  ]}
  style:--fade-left={fadeLeft ? "2rem" : "0px"}
  style:--fade-right={fadeRight ? "2rem" : "0px"}
>
  {#each groups as group, i (i)}
    {#if compact && i > 0}
      <span
        class="mx-3 h-5 w-px shrink-0 bg-gray-200 dark:bg-gray-700"
        aria-hidden="true"
      ></span>
    {/if}
    <div class={["flex shrink-0", compact ? "items-center" : "flex-col"]}>
      {#if compact}
        <span
          class="pr-1 text-xs font-semibold tracking-wider whitespace-nowrap text-gray-400 uppercase dark:text-gray-500"
          aria-hidden="true">{group.short}</span
        >
      {:else}
        <div
          class="rounded-b-md border border-t-0 border-gray-200 bg-gray-50 px-3 py-0.5 text-center text-xs font-semibold tracking-wider whitespace-nowrap text-gray-500 uppercase dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
          aria-hidden="true"
        >
          {group.label}
        </div>
      {/if}
      <ul
        class={theme.base({ class: "mb-0 flex-nowrap" })}
        role="tablist"
        aria-label={group.label}
      >
        {#each group.tabs as tab (tab.id)}
          {@const selected = tab.id === nav.selectedCategoryId}
          {@const padding = compact ? "px-3 py-2.5" : "px-4 py-2.5"}
          <li class={item.base()} role="presentation">
            <a
              href={categoryHref(tab.id)}
              role="tab"
              aria-selected={selected}
              class={item.button({
                class: selected
                  ? theme.active({
                      class: `${padding} font-semibold whitespace-nowrap`,
                    })
                  : theme.inactive({ class: `${padding} whitespace-nowrap` }),
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

<style>
  .tab-scroller {
    box-shadow: inset 0 -1px 0 var(--color-gray-200);
    scrollbar-width: thin;
    mask-image: linear-gradient(
      to right,
      transparent 0,
      #000 var(--fade-left),
      #000 calc(100% - var(--fade-right)),
      transparent 100%
    );
  }

  :global(.dark) .tab-scroller {
    box-shadow: inset 0 -1px 0 var(--color-gray-700);
  }
</style>

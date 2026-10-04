<script lang="ts">
  import { Heading, SidebarGroup, SidebarItem } from "flowbite-svelte";
  import {
    nav,
    selectedInstance,
    isCategoryDirty,
    categoryHref,
  } from "./nav.svelte";
  import { CONFIG_CATEGORIES } from "./configCategories";

  let sharedCategories = $derived(
    CONFIG_CATEGORIES.filter((c) => c.scope === "shared"),
  );
  let instanceCategories = $derived(
    CONFIG_CATEGORIES.filter((c) => c.scope === "per-instance"),
  );

  // Computed once, matching MainContent.svelte's own pattern for the same
  // function, rather than calling selectedInstance() again at each of the
  // template sites below.
  let instance = $derived(selectedInstance());

  const HEADING =
    "mt-5 mb-1 px-2 text-xs font-semibold tracking-wider text-gray-500 uppercase";
</script>

<nav aria-label="Settings">
  <Heading tag="h2" class={HEADING}>Shared</Heading>
  <SidebarGroup class="space-y-0.5">
    {#each sharedCategories as category (category.id)}
      <SidebarItem
        href={categoryHref(category.id)}
        label={`${category.label}${isCategoryDirty(category.id) ? "*" : ""}`}
        spanClass=""
        active={category.id === nav.selectedCategoryId}
      />
    {/each}
  </SidebarGroup>

  <Heading tag="h2" class={HEADING}>
    {#if instance}
      {instance.host} / cam {instance.cameraId}
    {:else}
      Per-instance (no cameras yet)
    {/if}
  </Heading>
  <SidebarGroup class="space-y-0.5">
    {#each instanceCategories as category (category.id)}
      <!-- Without a camera there's nothing to link to; an <a> without href
           isn't focusable, which is what disabled should be. -->
      <SidebarItem
        href={instance ? categoryHref(category.id) : undefined}
        label={`${category.label}${isCategoryDirty(category.id) ? "*" : ""}`}
        spanClass=""
        aria-disabled={!instance}
        class={instance ? "" : "pointer-events-none opacity-40"}
        active={category.id === nav.selectedCategoryId}
      />
    {/each}
  </SidebarGroup>
</nav>

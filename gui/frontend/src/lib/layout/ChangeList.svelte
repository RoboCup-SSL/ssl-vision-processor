<script lang="ts">
  import { Listgroup, ListgroupItem, Heading } from "flowbite-svelte";
  import {
    formatValue,
    SECTION_LABELS,
    type Change,
    type Section,
  } from "../config.svelte";

  interface Props {
    changes: Change[];
  }

  let { changes }: Props = $props();

  const SECTION_ORDER: Section[] = [
    "field",
    "layout",
    "camera",
    "geometry",
    "color",
    "network",
    "other",
  ];

  // Grouped by tab, then camera, so the list reads the way the UI is laid out.
  let groups = $derived(
    SECTION_ORDER.flatMap((section) => {
      const inSection = changes.filter((c) => c.section === section);
      const cameras = [...new Set(inSection.map((c) => c.cameraId))].sort(
        (a, b) => (a ?? -1) - (b ?? -1),
      );

      return cameras.map((cameraId) => ({
        key: `${section}:${String(cameraId)}`,
        title:
          cameraId === undefined
            ? SECTION_LABELS[section]
            : `${SECTION_LABELS[section]} — cam ${String(cameraId)}`,
        changes: inSection.filter((c) => c.cameraId === cameraId),
      }));
    }),
  );
</script>

<div class="change-list">
  {#each groups as group (group.key)}
    <Heading tag="h4" class="mt-3 mb-1 text-sm font-semibold first:mt-0"
      >{group.title}</Heading
    >
    <Listgroup class="w-full">
      {#each group.changes as change (change.path)}
        <ListgroupItem
          class="flex flex-wrap justify-between gap-2 px-3 py-1.5 text-sm font-normal"
        >
          <code class="path">{change.path}</code>
          <span class="values">
            <span class="before">{formatValue(change.before)}</span>
            →
            <span class="after">{formatValue(change.after)}</span>
          </span>
        </ListgroupItem>
      {/each}
    </Listgroup>
  {/each}
</div>

<style>
  .change-list {
    max-height: 50vh;
    overflow-y: auto;
    font-size: 0.85rem;
  }

  .path {
    color: var(--color-gray-700);
  }

  .values {
    font-family: monospace;
    overflow-wrap: anywhere;
  }

  .before {
    color: var(--color-red-700);
  }

  .after {
    color: var(--color-green-800);
  }
</style>

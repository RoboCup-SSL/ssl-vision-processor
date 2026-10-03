<script lang="ts">
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

  const SECTION_ORDER: Section[] = ["field", "geometry", "color", "other"];

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
    <h4>{group.title}</h4>
    <ul>
      {#each group.changes as change (change.path)}
        <li>
          <code class="path">{change.path}</code>
          <span class="values">
            <span class="before">{formatValue(change.before)}</span>
            →
            <span class="after">{formatValue(change.after)}</span>
          </span>
        </li>
      {/each}
    </ul>
  {/each}
</div>

<style>
  .change-list {
    max-height: 50vh;
    overflow-y: auto;
    font-size: 0.85rem;
  }

  h4 {
    margin: 0.75rem 0 0.25rem;
    font-weight: 600;
  }

  h4:first-child {
    margin-top: 0;
  }

  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  li {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.2rem 0;
    border-bottom: 1px solid #eee;
  }

  .path {
    color: #444;
  }

  .values {
    font-family: monospace;
    overflow-wrap: anywhere;
  }

  .before {
    color: #b00020;
  }

  .after {
    color: #1b5e20;
  }
</style>

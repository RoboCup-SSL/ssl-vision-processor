<script lang="ts">
  import { Tooltip } from "flowbite-svelte";
  import { preferences } from "./preferences.svelte";

  // An info icon with a tooltip of succinct notes, behind the "Show extra
  // tooltips" preference. The slot is kept when there's nothing to show, so
  // whatever sits beside it doesn't shift.
  interface Props {
    id: string;
    notes: string[];
  }

  let { id, notes }: Props = $props();
</script>

{#if preferences.tooltipsEnabled && notes.length > 0}
  <button type="button" {id} class="info" aria-label={notes.join(" ")}>ⓘ</button
  >
  <Tooltip triggeredBy={`#${id}`} placement="right" class="max-w-xs">
    {#each notes as note (note)}
      <p>{note}</p>
    {/each}
  </Tooltip>
{:else}
  <span class="slot"></span>
{/if}

<style>
  .info,
  .slot {
    width: 1.25rem;
    height: 1.25rem;
  }

  .info {
    padding: 0;
    border: none;
    background: none;
    color: var(--color-primary-700);
    font-size: 0.95rem;
    line-height: 1;
    cursor: help;
  }

  .info:hover:not(:disabled) {
    background: none;
  }
</style>

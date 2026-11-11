<script lang="ts">
  import { Accordion, Button, Progressbar } from "flowbite-svelte";
  import {
    CheckCircleSolid,
    CloseOutline,
    RefreshOutline,
  } from "flowbite-svelte-icons";
  import ConfirmModal from "../ConfirmModal.svelte";
  import { checklist as text } from "../text/checklist";
  import ChecklistItem from "./ChecklistItem.svelte";
  import {
    allItems,
    closeChecklist,
    restartChecklist,
  } from "./checklist.svelte";

  // The docked right-hand column (layout/Shell.svelte): beside the page, not
  // over it, so a step can be followed while it's being done.
  let items = $derived(allItems());
  let done = $derived(items.filter((i) => i.status === "done").length);
  let complete = $derived(done === items.length);

  let confirmingRestart = $state(false);
</script>

<ConfirmModal
  bind:open={confirmingRestart}
  title={text.restartTitle}
  message={text.restartMessage}
  confirmLabel={text.restartConfirm}
  onconfirm={restartChecklist}
/>

<aside
  class="checklist-panel flex flex-col gap-3 bg-white px-4 py-4 dark:bg-gray-800"
  aria-label={text.title}
>
  <div class="flex items-center justify-between gap-2">
    <h2 class="text-base font-semibold text-gray-900 dark:text-white">
      {text.title}
    </h2>
    <Button
      color="alternative"
      size="xs"
      pill
      class="h-7 w-7 p-0"
      aria-label={text.close}
      title={text.close}
      onclick={closeChecklist}
    >
      <CloseOutline size="sm" />
    </Button>
  </div>

  <div class="flex flex-col gap-1">
    <Progressbar
      progress={Math.round((done / items.length) * 100)}
      size="h-2"
      color={complete ? "green" : "primary"}
    />
    <span class="text-xs text-gray-500 dark:text-gray-400"
      >{text.progress(done, items.length)}</span
    >
  </div>

  {#if complete}
    <div
      class="flex flex-col gap-2 rounded-lg border border-green-200 bg-green-50 p-3 text-sm text-green-800 dark:border-green-800 dark:bg-gray-800 dark:text-green-400"
    >
      <span class="flex items-center gap-2 font-semibold"
        ><CheckCircleSolid size="sm" />{text.complete}</span
      >
      <p>{text.completeDetail}</p>
      <Button
        size="xs"
        color="green"
        class="self-start"
        onclick={closeChecklist}>{text.close}</Button
      >
    </div>
  {:else}
    <p class="text-sm text-gray-600 dark:text-gray-400">{text.intro}</p>
  {/if}

  <Accordion flush multiple>
    {#each items as item (item.id)}
      <ChecklistItem {item} {items} />
    {/each}
  </Accordion>

  <Button
    color="alternative"
    size="xs"
    class="self-start"
    onclick={() => {
      confirmingRestart = true;
    }}><RefreshOutline class="me-1.5" size="xs" />{text.restart}</Button
  >
</aside>

<style>
  /* Stays in view while the page beside it scrolls, above the log panel
     (--log-height, from Shell.svelte). */
  .checklist-panel {
    position: sticky;
    top: 0;
    max-height: calc(100vh - var(--log-height, 0px));
    overflow-y: auto;
    border-left: 1px solid var(--color-gray-200);
  }

  :global(.dark) .checklist-panel {
    border-color: var(--color-gray-700);
  }
</style>

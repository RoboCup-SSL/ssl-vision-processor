<script lang="ts">
  import { Badge, Button } from "flowbite-svelte";
  import {
    CheckCircleSolid,
    ClipboardCheckOutline,
  } from "flowbite-svelte-icons";
  import { checklist as text } from "../text/checklist";
  import { allItems, checklist, toggleChecklist } from "./checklist.svelte";

  // The top bar's way into the setup checklist: progress at a glance, yellow
  // when something that was done needs attention again.
  let items = $derived(allItems());
  let done = $derived(items.filter((i) => i.status === "done").length);
  let complete = $derived(done === items.length);
  let needsAttention = $derived(
    items.some((i) => i.status === "attention" || i.status === "recheck"),
  );
</script>

<Button
  color="alternative"
  size="xs"
  pill
  class="h-8 gap-1.5 px-3 whitespace-nowrap"
  title={text.pillTitle}
  aria-expanded={checklist.open}
  onclick={toggleChecklist}
>
  <ClipboardCheckOutline size="sm" />
  <!-- Icon only on narrow screens, where the top bar's badges need the room. -->
  <span class="hidden 2xl:inline">{text.pill}</span>
  {#if complete}
    <CheckCircleSolid
      class="text-green-600 dark:text-green-500"
      size="sm"
      ariaLabel={text.complete}
    />
  {:else}
    <Badge rounded color={needsAttention ? "yellow" : "gray"}
      >{done}/{items.length}</Badge
    >
  {/if}
</Button>

<script lang="ts">
  import {
    CheckCircleOutline,
    CheckCircleSolid,
    ExclamationCircleSolid,
    RefreshOutline,
  } from "flowbite-svelte-icons";
  import { checklist as text } from "../text/checklist";
  import type { Status } from "./checklist.svelte";

  let { status }: { status: Status } = $props();
</script>

{#if status === "done"}
  <CheckCircleSolid
    class="shrink-0 text-green-600 dark:text-green-500"
    size="sm"
    ariaLabel={text.status.done}
  />
{:else if status === "attention"}
  <ExclamationCircleSolid
    class="shrink-0 text-yellow-500 dark:text-yellow-300"
    size="sm"
    ariaLabel={text.status.attention}
  />
{:else if status === "recheck"}
  <RefreshOutline
    class="shrink-0 text-yellow-500 dark:text-yellow-300"
    size="sm"
    ariaLabel={text.status.recheck}
  />
{:else}
  <CheckCircleOutline
    class="shrink-0 text-gray-400 dark:text-gray-500"
    size="sm"
    ariaLabel={text.status.todo}
  />
{/if}

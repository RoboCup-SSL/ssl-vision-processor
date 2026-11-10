<script lang="ts">
  import { Button, Checkbox } from "flowbite-svelte";
  import { checklist as text } from "../text/checklist";
  import { isTicked, setTicked, type Status } from "./checklist.svelte";

  // A manual item's tick. Once what it judged has changed, re-ticking is one
  // click rather than an untick and tick.
  let { key, status }: { key: string; status: Status } = $props();
</script>

{#if status === "recheck"}
  <Button
    size="xs"
    color="alternative"
    onclick={() => {
      setTicked(key, true);
    }}>{text.reconfirm}</Button
  >
{:else}
  <Checkbox
    class="text-sm"
    bind:checked={
      () => isTicked(key),
      (v: boolean) => {
        setTicked(key, v);
      }
    }>{text.confirm}</Checkbox
  >
{/if}

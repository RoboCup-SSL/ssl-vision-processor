<script lang="ts">
  import { onMount } from "svelte";
  import { Button, Spinner } from "flowbite-svelte";
  import { virtualField, loadFieldPresets } from "../../geometry.svelte";
  import { skipToFinish, nextStep, applyPresetToDraft } from "../wizard.svelte";

  onMount(() => {
    if (virtualField.presets.length === 0) void loadFieldPresets();
  });

  function chooseDivision(name: string): void {
    const preset = virtualField.presets.find((p) => p.name === name);
    if (!preset) return;

    applyPresetToDraft(preset);
    skipToFinish();
  }
</script>

<div class="flex flex-col gap-4">
  <p class="text-sm text-gray-600">
    Start from a regulation field, or configure a custom one.
  </p>

  {#if virtualField.presets.length === 0}
    <div class="flex items-center gap-2 text-sm text-gray-500">
      <Spinner size="4" /> Loading presets...
    </div>
  {:else}
    {#each virtualField.presets as preset (preset.name)}
      <Button
        color="alternative"
        class="justify-start"
        onclick={() => {
          chooseDivision(preset.name);
        }}
      >
        Regulation {preset.name}
      </Button>
    {/each}
  {/if}

  <Button color="alternative" class="justify-start" onclick={nextStep}>
    Custom field
  </Button>
</div>

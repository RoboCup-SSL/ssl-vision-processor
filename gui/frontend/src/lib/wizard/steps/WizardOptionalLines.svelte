<script lang="ts">
  import { wizard } from "../wizard.svelte";
  import { OPTIONAL_LINE_FIELDS } from "../../fieldConfigFields";
  import WizardFieldPreview from "./WizardFieldPreview.svelte";
</script>

<div class="flex flex-col gap-4 sm:flex-row">
  <div class="flex flex-1 flex-col gap-2">
    <p class="text-sm text-gray-600">Which markings does this field have?</p>

    {#each OPTIONAL_LINE_FIELDS as { key, label } (key)}
      <label class="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          bind:checked={wizard.draft.optionalFieldLines[key]}
        />
        {label}
      </label>
    {/each}
  </div>

  <div class="flex flex-1 items-start justify-center">
    <WizardFieldPreview />
  </div>
</div>

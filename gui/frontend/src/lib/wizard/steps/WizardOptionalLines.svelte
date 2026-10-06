<script lang="ts">
  import { wizard as text } from "../../text/wizard";
  import { Checkbox } from "flowbite-svelte";
  import { wizard } from "../wizard.svelte";
  import { OPTIONAL_LINE_FIELDS } from "../../fieldConfigFields";
  import WizardFieldPreview from "./WizardFieldPreview.svelte";
</script>

<div class="flex flex-col gap-4 sm:flex-row">
  <div class="flex flex-1 flex-col gap-2">
    <p class="text-sm text-gray-600 dark:text-gray-400">
      {text.markings}
    </p>

    {#each OPTIONAL_LINE_FIELDS as { key, label } (key)}
      <Checkbox bind:checked={wizard.draft.optionalFieldLines[key]}>
        {label}
      </Checkbox>
    {/each}
  </div>

  <div class="flex flex-1 items-start justify-center">
    <WizardFieldPreview />
  </div>
</div>

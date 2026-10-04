<script lang="ts">
  import { Helper, Label, Radio, Select } from "flowbite-svelte";
  import { CAMERA_COUNT_OPTIONS } from "../../fieldSplit";
  import { wizard } from "../wizard.svelte";

  function setFieldLayout(layout: "full" | "half"): void {
    wizard.fieldLayout = layout;

    const options = CAMERA_COUNT_OPTIONS[layout];
    if (!options.includes(wizard.cameraCount)) {
      wizard.cameraCount = options[0] ?? 1;
    }
  }
</script>

<div class="flex flex-col gap-4">
  <fieldset class="flex flex-col gap-2">
    <legend class="mb-1 text-sm font-medium">Field layout</legend>
    <Radio
      name="wizard-layout"
      value="full"
      group={wizard.fieldLayout}
      onchange={() => {
        setFieldLayout("full");
      }}>Full field</Radio
    >
    <Radio
      name="wizard-layout"
      value="half"
      group={wizard.fieldLayout}
      onchange={() => {
        setFieldLayout("half");
      }}>Half field</Radio
    >
  </fieldset>

  <div class="flex flex-col gap-1">
    <Label for="wizard-cameras">Cameras</Label>
    <Select
      id="wizard-cameras"
      size="sm"
      class="w-32"
      placeholder=""
      value={wizard.cameraCount}
      onchange={(e: Event) => {
        wizard.cameraCount = Number(
          (e.currentTarget as HTMLSelectElement).value,
        );
      }}
    >
      {#each CAMERA_COUNT_OPTIONS[wizard.fieldLayout] as count (count)}
        <option value={count}>{count}</option>
      {/each}
    </Select>
  </div>

  <Helper>
    This isn't written anywhere yet: set <code>camera_amount</code> to
    {wizard.cameraCount} in each camera's own config.yml by hand.
  </Helper>
</div>

<script lang="ts">
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
    <label class="flex items-center gap-2 text-sm">
      <input
        type="radio"
        name="wizard-layout"
        checked={wizard.fieldLayout === "full"}
        onchange={() => {
          setFieldLayout("full");
        }}
      />
      Full field
    </label>
    <label class="flex items-center gap-2 text-sm">
      <input
        type="radio"
        name="wizard-layout"
        checked={wizard.fieldLayout === "half"}
        onchange={() => {
          setFieldLayout("half");
        }}
      />
      Half field
    </label>
  </fieldset>

  <label class="flex flex-col gap-1 text-sm">
    Cameras
    <select
      class="w-32 rounded border border-gray-300 p-1.5"
      value={wizard.cameraCount}
      onchange={(e) => {
        wizard.cameraCount = Number(e.currentTarget.value);
      }}
    >
      {#each CAMERA_COUNT_OPTIONS[wizard.fieldLayout] as count (count)}
        <option value={count}>{count}</option>
      {/each}
    </select>
  </label>

  <p class="text-sm text-gray-500">
    This isn't written anywhere yet: set <code>camera_amount</code> to
    {wizard.cameraCount} in each camera's own config.yml by hand.
  </p>
</div>

<script lang="ts">
  import { Input, Label, P } from "flowbite-svelte";
  import {
    DIMENSION_FIELDS,
    type FieldConfigField,
  } from "../../fieldConfigFields";
  import { wizard } from "../wizard.svelte";
  import WizardDimensionsPreview from "./WizardDimensionsPreview.svelte";

  // A short, curated list, not FieldEditor's full one: goal depth/height, line
  // thickness, ball radius, and max robot radius are close enough to standard
  // for most setups to skip asking here. They're still editable in the main
  // Field Dimensions page for anyone who needs to adjust them.
  const WIZARD_DIMENSION_KEYS: FieldConfigField["key"][] = [
    "fieldWidth",
    "penaltyAreaDepth",
    "penaltyAreaWidth",
    "goalWidth",
    "centerCircleRadius",
  ];

  const otherFields = WIZARD_DIMENSION_KEYS.map((key) =>
    DIMENSION_FIELDS.find((f) => f.key === key),
  ).filter((f): f is FieldConfigField => f !== undefined);

  // optionalLines runs before this step specifically so a field with no
  // penalty area, say, never gets asked for a penalty area depth/width --
  // gatedBy-less fields (fieldWidth, goalWidth) always show.
  let fields = $derived(
    otherFields.filter(
      (f) => !f.gatedBy || wizard.draft.optionalFieldLines[f.gatedBy],
    ),
  );

  // People measure a half field with a tape measure, not a full one -- there
  // is no full-field number to enter directly, so ask for the half and double
  // it, the same way FieldEditor's half/full toggle does.
  let halfLength = $derived(
    Math.round((wizard.draft.field.fieldLength ?? 0) / 2),
  );

  function setHalfLength(value: number): void {
    wizard.draft.field.fieldLength = value * 2;
  }
</script>

<div class="flex flex-col gap-2">
  <P class="text-sm text-gray-600 dark:text-gray-400">
    All dimensions are in millimeters.
  </P>

  {#if wizard.fieldLayout === "half"}
    <div class="flex items-center justify-between gap-2">
      <Label for="wizard-half-length" class="font-normal"
        >Half length (full: {wizard.draft.field.fieldLength ?? 0}mm)</Label
      >
      <div class="w-28 shrink-0">
        <Input
          id="wizard-half-length"
          type="number"
          size="sm"
          value={halfLength}
          oninput={(e: Event) => {
            setHalfLength((e.currentTarget as HTMLInputElement).valueAsNumber);
          }}
        />
      </div>
    </div>
  {:else}
    <div class="flex items-center justify-between gap-2">
      <Label for="wizard-field-length" class="font-normal">Field length</Label>
      <div class="w-28 shrink-0">
        <Input
          id="wizard-field-length"
          type="number"
          size="sm"
          bind:value={wizard.draft.field.fieldLength}
        />
      </div>
    </div>
  {/if}

  {#each fields as { key, label } (key)}
    <div class="flex items-center justify-between gap-2">
      <Label for={`wizard-${key}`} class="font-normal">{label}</Label>
      <div class="w-28 shrink-0">
        <Input
          id={`wizard-${key}`}
          type="number"
          size="sm"
          bind:value={wizard.draft.field[key]}
        />
      </div>
    </div>
  {/each}

  <div class="mt-2 px-2">
    <WizardDimensionsPreview />
  </div>
</div>

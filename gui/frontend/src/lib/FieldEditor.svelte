<script lang="ts">
  import SettingsCard from "./SettingsCard.svelte";
  import FormRow from "./FormRow.svelte";
  import {
    Button,
    Checkbox,
    Input,
    Radio,
    Spinner,
    Heading,
    P,
  } from "flowbite-svelte";
  import { virtualField } from "./geometry.svelte";
  import { config } from "./config.svelte";
  import FieldSketch from "./FieldSketch.svelte";
  import { cameraCount, regionLabel, slotSlice } from "./cameraLayout";
  import { selectedInstance, categoryHref } from "./layout/nav.svelte";
  import { DIMENSION_FIELDS, OPTIONAL_LINE_FIELDS } from "./fieldConfigFields";
  import { openWizard } from "./wizard/wizard.svelte";

  // "Half field" is a data-entry convenience, not a wire concept: the backend
  // (SSL_GeometryFieldSize.field_length) only ever means the FULL field.
  // config.doc.field.fieldLength keeps that meaning always; this toggle
  // just changes what the length input shows and how it's written back, so
  // switching modes never silently mutates already-loaded data. See
  // gui/CLAUDE.md and src/CameraModel.cpp's visibleFieldExtentEstimation for
  // why length (not width) is what gets halved.
  let fieldLayout = $state<"full" | "half">("full");

  // The selected camera's region, from the Camera Layout page, so the
  // markings outside it render dimmed.
  let instance = $derived(selectedInstance());
  let count = $derived(config.doc ? cameraCount(config.doc) : 1);
  let fieldSlice = $derived(
    config.doc && instance && count > 1
      ? slotSlice(config.doc, instance.cameraId)
      : undefined,
  );

  let halfLength = $derived(
    Math.round((config.doc?.field.fieldLength ?? 0) / 2),
  );

  function setHalfLength(value: number): void {
    if (config.doc) config.doc.field.fieldLength = value * 2;
  }

  function setFieldLayout(layout: "full" | "half"): void {
    fieldLayout = layout;
  }

  // The form's labels ("Boundary width (goal line)") need more room than
  // FormRow's default.
  const LABEL_WIDTH = "12rem";
</script>

<section class="field-editor">
  <div class="title-row">
    <Heading tag="h2" class="mb-2 text-xl font-semibold">Virtual field</Heading>
    <span class="path">{config.state?.path ?? ""}</span>
    <Button size="xs" outline class="ms-auto" onclick={openWizard}>
      Run setup wizard
    </Button>
  </div>

  {#if config.doc}
    {@const doc = config.doc}
    <div class="layout">
      <form>
        <SettingsCard title="Field layout">
          <div class="radios">
            <Radio
              name="fieldLayout"
              value="full"
              group={fieldLayout}
              onchange={() => {
                setFieldLayout("full");
              }}>Full field</Radio
            >
            <Radio
              name="fieldLayout"
              value="half"
              group={fieldLayout}
              onchange={() => {
                setFieldLayout("half");
              }}>Half field</Radio
            >
          </div>

          {#if fieldLayout === "half"}
            <FormRow
              label={`Half length (full: ${String(doc.field.fieldLength ?? 0)}mm)`}
              for="field-half-length"
              labelWidth={LABEL_WIDTH}
            >
              <Input
                id="field-half-length"
                type="number"
                size="sm"
                value={halfLength}
                oninput={(e: Event) => {
                  setHalfLength(
                    (e.currentTarget as HTMLInputElement).valueAsNumber,
                  );
                }}
              />
            </FormRow>
          {/if}

          <FormRow label="Cameras" labelWidth={LABEL_WIDTH}>
            <span class="flex flex-wrap items-center gap-2 text-sm">
              {count} region{count === 1 ? "" : "s"}{instance && count > 1
                ? `; camera ${String(instance.cameraId)} covers ${regionLabel(doc, instance.cameraId)}`
                : ""}
              <Button
                size="xs"
                color="alternative"
                href={categoryHref("layout")}>Camera Layout →</Button
              >
            </span>
          </FormRow>
        </SettingsCard>

        <SettingsCard title="Dimensions">
          {#each DIMENSION_FIELDS as { key, label } (key)}
            <FormRow {label} for={`field-${key}`} labelWidth={LABEL_WIDTH}>
              <Input
                id={`field-${key}`}
                type="number"
                size="sm"
                bind:value={doc.field[key]}
              />
            </FormRow>
          {/each}
        </SettingsCard>

        <SettingsCard title="Markings present on this field">
          <div class="checks">
            {#each OPTIONAL_LINE_FIELDS as { key, label } (key)}
              <Checkbox bind:checked={doc.optionalFieldLines[key]}>
                {label}
              </Checkbox>
            {/each}
          </div>
        </SettingsCard>
      </form>

      <div class="sketch">
        <FieldSketch
          fieldLength={doc.field.fieldLength ?? 0}
          fieldWidth={doc.field.fieldWidth ?? 0}
          lines={virtualField.fieldLines}
          arcs={virtualField.fieldArcs}
          slice={fieldSlice}
        />
      </div>
    </div>
  {:else}
    <P
      size="sm"
      class="mb-2 text-gray-600 dark:text-gray-400 flex items-center gap-2"
      ><Spinner size="4" /> Loading…</P
    >
  {/if}
</section>

<style>
  .field-editor {
    max-width: 900px;
  }

  .title-row {
    display: flex;
    align-items: baseline;
    gap: 0.75rem;
    margin-bottom: 0.5rem;
  }

  .path {
    font-family: monospace;
    font-size: 0.8rem;
    color: var(--color-gray-600);
  }

  .layout {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1.5rem;
    align-items: start;
  }

  .radios,
  .checks {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    margin: 0.4rem 0;
  }
</style>

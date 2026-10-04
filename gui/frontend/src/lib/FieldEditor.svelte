<script lang="ts">
  import SettingsCard from "./SettingsCard.svelte";
  import FormRow from "./FormRow.svelte";
  import {
    Button,
    Checkbox,
    Input,
    Radio,
    Select,
    Spinner,
    Heading,
    P,
  } from "flowbite-svelte";
  import { virtualField } from "./geometry.svelte";
  import { config } from "./config.svelte";
  import FieldSketch from "./FieldSketch.svelte";
  import { computeFieldSlice, CAMERA_COUNT_OPTIONS } from "./fieldSplit";
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

  // camAmount here is informational only -- it belongs in each vision
  // processor's own config.yml (SSL_VPConfigGeometry.camera_amount), which
  // this host does not yet read, write, or push to any instance.
  let cameraCount = $state(1);
  let cameraId = $state(0);

  let cameraAmount = $derived(fieldLayout === "half" ? 2 : cameraCount);

  // Which one this instance is, out of cameraAmount -- clamped so a stale
  // selection (e.g. picked "camera 3 of 4" then switched to 2 cameras) can't
  // point past the end.
  let clampedCameraId = $derived(Math.min(cameraId, cameraAmount - 1));

  // The FieldSketch highlight: which portion of the field cameraId is
  // responsible for out of cameraAmount, so the rest of the markings (e.g.
  // the other goal's penalty box) render dimmed.
  let fieldSlice = $derived(
    computeFieldSlice(
      clampedCameraId,
      cameraAmount,
      config.doc?.field.fieldLength ?? 0,
      config.doc?.field.fieldWidth ?? 0,
    ),
  );

  let halfLength = $derived(
    Math.round((config.doc?.field.fieldLength ?? 0) / 2),
  );

  function setHalfLength(value: number): void {
    if (config.doc) config.doc.field.fieldLength = value * 2;
  }

  function setFieldLayout(layout: "full" | "half"): void {
    fieldLayout = layout;

    const options = CAMERA_COUNT_OPTIONS[layout];
    if (!options.includes(cameraCount)) {
      cameraCount = options[0] ?? 1;
    }
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

          <FormRow
            label={`Cameras (camera_amount: ${String(cameraAmount)})`}
            for="field-camera-count"
            labelWidth={LABEL_WIDTH}
          >
            <Select
              id="field-camera-count"
              size="sm"
              placeholder=""
              value={cameraCount}
              onchange={(e: Event) => {
                cameraCount = Number(
                  (e.currentTarget as HTMLSelectElement).value,
                );
              }}
            >
              {#each CAMERA_COUNT_OPTIONS[fieldLayout] as count (count)}
                <option value={count}>{count}</option>
              {/each}
            </Select>
          </FormRow>

          <FormRow
            label="This camera (camera_id)"
            for="field-camera-id"
            labelWidth={LABEL_WIDTH}
          >
            <Select
              id="field-camera-id"
              size="sm"
              placeholder=""
              value={clampedCameraId}
              onchange={(e: Event) => {
                cameraId = Number((e.currentTarget as HTMLSelectElement).value);
              }}
            >
              {#each Array.from(Array(cameraAmount).keys()) as id (id)}
                <option value={id}>{id}</option>
              {/each}
            </Select>
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

<script lang="ts">
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
</script>

<section class="field-editor">
  <div class="title-row">
    <h2>Virtual field</h2>
    <span class="path">{config.state?.path ?? ""}</span>
    <button type="button" class="wizard-button" onclick={openWizard}>
      Run setup wizard
    </button>
  </div>

  {#if config.doc}
    {@const doc = config.doc}
    <div class="layout">
      <form>
        <fieldset>
          <legend>Field layout</legend>

          <label class="radio-row">
            <span>
              <input
                type="radio"
                name="fieldLayout"
                checked={fieldLayout === "full"}
                onchange={() => {
                  setFieldLayout("full");
                }}
              />
              Full field
            </span>
            <span>
              <input
                type="radio"
                name="fieldLayout"
                checked={fieldLayout === "half"}
                onchange={() => {
                  setFieldLayout("half");
                }}
              />
              Half field
            </span>
          </label>

          {#if fieldLayout === "half"}
            <label>
              Half length (full: {doc.field.fieldLength ?? 0}mm)
              <input
                type="number"
                value={halfLength}
                oninput={(e) => {
                  setHalfLength(e.currentTarget.valueAsNumber);
                }}
              />
            </label>
          {/if}

          <label>
            Cameras (camera_amount: {cameraAmount})
            <select
              value={cameraCount}
              onchange={(e) => {
                cameraCount = Number(e.currentTarget.value);
              }}
            >
              {#each CAMERA_COUNT_OPTIONS[fieldLayout] as count (count)}
                <option value={count}>{count}</option>
              {/each}
            </select>
          </label>

          <label>
            This camera (camera_id)
            <select
              value={clampedCameraId}
              onchange={(e) => {
                cameraId = Number(e.currentTarget.value);
              }}
            >
              {#each Array.from(Array(cameraAmount).keys()) as id (id)}
                <option value={id}>{id}</option>
              {/each}
            </select>
          </label>
        </fieldset>

        <fieldset>
          <legend>Dimensions</legend>
          {#each DIMENSION_FIELDS as { key, label } (key)}
            <label>
              {label}
              <input type="number" bind:value={doc.field[key]} />
            </label>
          {/each}
        </fieldset>

        <fieldset>
          <legend>Markings present on this field</legend>
          {#each OPTIONAL_LINE_FIELDS as { key, label } (key)}
            <label class="checkbox">
              <input
                type="checkbox"
                bind:checked={doc.optionalFieldLines[key]}
              />
              {label}
            </label>
          {/each}
        </fieldset>
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
    <p class="hint">Loading...</p>
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

  .wizard-button {
    margin-left: auto;
    padding: 0.3rem 0.7rem;
    border: 1px solid #1a56db;
    border-radius: 4px;
    background: none;
    color: #1a56db;
    font-size: 0.8rem;
    cursor: pointer;
  }

  .wizard-button:hover {
    background: #eff6ff;
  }

  .title-row h2 {
    margin: 0;
  }

  .path {
    font-family: monospace;
    font-size: 0.8rem;
    color: #666;
  }

  .layout {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1.5rem;
    align-items: start;
  }

  fieldset {
    border: 1px solid #ddd;
    border-radius: 4px;
    margin-bottom: 1rem;
  }

  label {
    display: flex;
    justify-content: space-between;
    gap: 0.5rem;
    margin: 0.4rem 0;
    font-size: 0.85rem;
  }

  label.checkbox {
    justify-content: flex-start;
  }

  label.radio-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.2rem;
  }

  label.radio-row span {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-weight: normal;
  }

  input[type="number"] {
    width: 6rem;
  }

  .hint {
    color: #888;
    font-size: 0.8rem;
    font-style: italic;
  }
</style>

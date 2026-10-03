<script lang="ts">
  import { onMount } from "svelte";
  import type { VisionInstance } from "../layout/nav.svelte";
  import {
    colorConfig,
    loadColorConfig,
    saveColorConfig,
    rgbToCss,
    rgbToYuv,
    COLOR_CLASSES,
    CLASS_LABELS,
    type ColorClass,
  } from "../color.svelte";
  import WeightTriangle from "./WeightTriangle.svelte";
  import ColorUpdateEquation from "./ColorUpdateEquation.svelte";
  import YuvPositionPane from "./YuvPositionPane.svelte";
  import {
    Table,
    TableHead,
    TableHeadCell,
    TableBody,
    TableBodyRow,
    TableBodyCell,
    Button,
  } from "flowbite-svelte";

  // No `category` prop, unlike GeometryPanel/ConfigCategoryPlaceholder:
  // those still show category.fields via the doc-only ConfigFieldList
  // because they have numeric placeholders alongside a partial real editor.
  // Every field this category lists is already a live editor below, so
  // there's nothing left for that list to document.
  interface Props {
    instance: VisionInstance | undefined;
  }

  let { instance }: Props = $props();

  // Which single reference color is shown/edited right now -- deliberately
  // one at a time with a selector, not six editors at once (see the plan this
  // was scoped from: a v1 for the reference-color side of issue 18, ahead of
  // the live/autoadapted display and blob-sample debug data the other
  // contributor's protobuf work will eventually add).
  let selectedClass = $state<ColorClass>("orange");

  onMount(() => {
    void loadColorConfig();
  });

  let updateForce = $derived(
    1 - colorConfig.config.referenceForce - colorConfig.config.historyForce,
  );

  function handleSave(): void {
    void saveColorConfig(colorConfig.config);
  }
</script>

<section class="color-panel">
  <h2>Color</h2>

  {#if instance}
    <p class="hint">
      Editing {instance.host} / cam {instance.cameraId}'s
      <code>color:</code> block.
    </p>
  {:else}
    <p class="hint">Select a vision processor on the left first.</p>
  {/if}

  {#if colorConfig.loading}
    <p class="hint">Loading...</p>
  {/if}

  <YuvPositionPane
    bind:color={colorConfig.config[selectedClass]}
    colors={colorConfig.config}
    bind:selectedClass
  />

  <div class="rgb-editor">
    <label>
      R
      <input
        type="number"
        min="0"
        max="255"
        bind:value={colorConfig.config[selectedClass].r}
      />
    </label>
    <label>
      G
      <input
        type="number"
        min="0"
        max="255"
        bind:value={colorConfig.config[selectedClass].g}
      />
    </label>
    <label>
      B
      <input
        type="number"
        min="0"
        max="255"
        bind:value={colorConfig.config[selectedClass].b}
      />
    </label>
    <span
      class="swatch large"
      style:background={rgbToCss(colorConfig.config[selectedClass])}
    ></span>
  </div>

  <Table striped>
    <TableHead>
      <TableHeadCell>Color</TableHeadCell>
      <TableHeadCell class="px-1.5 py-1">R</TableHeadCell>
      <TableHeadCell class="px-1.5 py-1">G</TableHeadCell>
      <TableHeadCell class="px-1.5 py-1">B</TableHeadCell>
      <TableHeadCell class="w-10 p-0"></TableHeadCell>
      <TableHeadCell class="px-1.5 py-1">Y</TableHeadCell>
      <TableHeadCell class="px-1.5 py-1">U</TableHeadCell>
      <TableHeadCell class="px-1.5 py-1">V</TableHeadCell>
    </TableHead>
    <TableBody>
      {#each COLOR_CLASSES as cls (cls)}
        {@const rgb = colorConfig.config[cls]}
        {@const yuv = rgbToYuv(rgb)}
        <TableBodyRow>
          <TableBodyCell>
            <span class="cell-color">
              <span class="swatch" style:background={rgbToCss(rgb)}></span>
              {CLASS_LABELS[cls]}
            </span>
          </TableBodyCell>
          <TableBodyCell class="px-1.5 py-1">{rgb.r}</TableBodyCell>
          <TableBodyCell class="px-1.5 py-1">{rgb.g}</TableBodyCell>
          <TableBodyCell class="px-1.5 py-1">{rgb.b}</TableBodyCell>
          <TableBodyCell class="w-10 p-0"></TableBodyCell>
          <TableBodyCell class="px-1.5 py-1">{yuv.y}</TableBodyCell>
          <TableBodyCell class="px-1.5 py-1">{yuv.u}</TableBodyCell>
          <TableBodyCell class="px-1.5 py-1">{yuv.v}</TableBodyCell>
        </TableBodyRow>
      {/each}
    </TableBody>
  </Table>

  <h3>Update weights</h3>
  <p class="hint">
    How much each new color leans on its configured reference vs. last frame's
    color vs. what was actually sampled this frame. Drag the marker, or edit the
    minimum reference weight directly -- see src/blobs/colorupdate.cpp's
    updateColor for the exact blend this mirrors.
  </p>
  <WeightTriangle
    bind:referenceForce={colorConfig.config.referenceForce}
    bind:historyForce={colorConfig.config.historyForce}
    bind:minReferenceForce={colorConfig.config.minReferenceForce}
  />

  <ColorUpdateEquation
    referenceForce={colorConfig.config.referenceForce}
    historyForce={colorConfig.config.historyForce}
    {updateForce}
    referenceLabel={selectedClass}
    referenceColor={colorConfig.config[selectedClass]}
  />

  <Table striped>
    <TableHead>
      <TableHeadCell>Component</TableHeadCell>
      <TableHeadCell class="px-1.5 py-1">Weight</TableHeadCell>
    </TableHead>
    <TableBody>
      <TableBodyRow>
        <TableBodyCell>Reference</TableBodyCell>
        <TableBodyCell class="px-1.5 py-1"
          >{(colorConfig.config.referenceForce * 100).toFixed(
            1,
          )}%</TableBodyCell
        >
      </TableBodyRow>
      <TableBodyRow>
        <TableBodyCell>History</TableBodyCell>
        <TableBodyCell class="px-1.5 py-1"
          >{(colorConfig.config.historyForce * 100).toFixed(1)}%</TableBodyCell
        >
      </TableBodyRow>
      <TableBodyRow>
        <TableBodyCell>Current frame</TableBodyCell>
        <TableBodyCell class="px-1.5 py-1"
          >{(updateForce * 100).toFixed(1)}%</TableBodyCell
        >
      </TableBodyRow>
      <TableBodyRow>
        <TableBodyCell>Minimum reference floor</TableBodyCell>
        <TableBodyCell class="px-1.5 py-1"
          >{(colorConfig.config.minReferenceForce * 100).toFixed(
            1,
          )}%</TableBodyCell
        >
      </TableBodyRow>
    </TableBody>
  </Table>

  <div class="save-row">
    <Button
      size="sm"
      color="primary"
      onclick={handleSave}
      disabled={colorConfig.saving}
    >
      {colorConfig.saving ? "Saving..." : "Save to config.yml"}
    </Button>
    {#if colorConfig.savedAt}
      <span class="saved">Saved.</span>
    {/if}
    {#if colorConfig.error}
      <span class="error">Error: {colorConfig.error}</span>
    {/if}
  </div>
</section>

<style>
  .color-panel {
    /* Wide enough for YuvPositionPane's plane+slider+readout row (1100px
       max-width there); everything else in this panel is narrower anyway. */
    max-width: 1100px;
  }

  h2 {
    margin: 0 0 0.5rem;
  }

  h3 {
    margin: 1.5rem 0 0.25rem;
  }

  .hint {
    color: #666;
    font-size: 0.85rem;
  }

  code {
    background: #eee;
    padding: 0.1rem 0.3rem;
    border-radius: 3px;
  }

  .swatch {
    display: inline-block;
    width: 0.9rem;
    height: 0.9rem;
    border-radius: 2px;
    border: 1px solid rgba(0, 0, 0, 0.2);
  }

  .swatch.large {
    width: 2rem;
    height: 2rem;
    border-radius: 4px;
  }

  .cell-color {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
  }

  .rgb-editor {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-top: 0.5rem;
  }

  .rgb-editor label {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    font-size: 0.8rem;
    color: #444;
  }

  .rgb-editor input {
    width: 4.5rem;
  }

  .save-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 1.5rem;
  }

  .saved {
    color: #1b5e20;
    font-size: 0.85rem;
  }

  .error {
    color: #b00020;
    font-size: 0.85rem;
  }
</style>

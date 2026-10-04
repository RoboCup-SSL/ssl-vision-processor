<script lang="ts">
  import { untrack } from "svelte";
  import type { VisionInstance } from "../layout/nav.svelte";
  import { config } from "../config.svelte";
  import {
    colorConfig,
    colorFromDoc,
    writeColorToDoc,
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
    Input,
    Label,
    Spinner,
    Heading,
    P,
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

  let cameraId = $derived(instance?.cameraId);

  // Which camera colorConfig.config currently mirrors. Edits are only
  // written back once it has been filled from that camera's document entry --
  // otherwise the placeholder defaults would overwrite its real overrides.
  let syncedCamera = $state<number | undefined>(undefined);

  // Document -> buffer: on camera switch, and whenever the document's color
  // changes underneath (a load, a reload, another tab).
  $effect(() => {
    if (cameraId === undefined || !config.doc) return;

    const fresh = colorFromDoc(cameraId);
    const id = cameraId;

    untrack(() => {
      if (
        JSON.stringify(fresh) !==
        JSON.stringify($state.snapshot(colorConfig.config))
      ) {
        colorConfig.config = fresh;
      }

      syncedCamera = id;
    });
  });

  // Buffer -> document: every edit goes live through the working document.
  $effect(() => {
    const data = $state.snapshot(colorConfig.config);
    const id = cameraId;

    untrack(() => {
      if (id !== undefined && id === syncedCamera) writeColorToDoc(id, data);
    });
  });

  let updateForce = $derived(
    1 - colorConfig.config.referenceForce - colorConfig.config.historyForce,
  );
</script>

<section class="color-panel">
  <Heading tag="h2" class="mb-2 text-xl font-semibold">Color</Heading>

  {#if instance}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400">
      Editing {instance.host} / cam {instance.cameraId}'s
      <code>color:</code> block.
    </P>
  {:else}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400"
      >Select a vision processor on the left first.</P
    >
  {/if}

  {#if !config.doc}
    <P
      size="sm"
      class="mb-2 text-gray-600 dark:text-gray-400 flex items-center gap-2"
      ><Spinner size="4" /> Loading…</P
    >
  {/if}

  <YuvPositionPane
    bind:color={colorConfig.config[selectedClass]}
    colors={colorConfig.config}
    bind:selectedClass
  />

  <div class="rgb-editor">
    <div class="channel">
      <Label for="rgb-r" class="font-normal">R</Label>
      <div class="w-20 shrink-0">
        <Input
          id="rgb-r"
          type="number"
          size="sm"
          min="0"
          max="255"
          bind:value={colorConfig.config[selectedClass].r}
        />
      </div>
    </div>
    <div class="channel">
      <Label for="rgb-g" class="font-normal">G</Label>
      <div class="w-20 shrink-0">
        <Input
          id="rgb-g"
          type="number"
          size="sm"
          min="0"
          max="255"
          bind:value={colorConfig.config[selectedClass].g}
        />
      </div>
    </div>
    <div class="channel">
      <Label for="rgb-b" class="font-normal">B</Label>
      <div class="w-20 shrink-0">
        <Input
          id="rgb-b"
          type="number"
          size="sm"
          min="0"
          max="255"
          bind:value={colorConfig.config[selectedClass].b}
        />
      </div>
    </div>
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

  <Heading tag="h3" class="mb-1 text-base font-semibold">Update weights</Heading
  >
  <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400">
    How much each new color leans on its configured reference vs. last frame's
    color vs. what was actually sampled this frame. Drag the marker, or edit the
    minimum reference weight directly -- see src/blobs/colorupdate.cpp's
    updateColor for the exact blend this mirrors.
  </P>
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
</section>

<style>
  .color-panel {
    /* Wide enough for YuvPositionPane's plane+slider+readout row (1100px
       max-width there); everything else in this panel is narrower anyway. */
    max-width: 1100px;
  }

  code {
    background: var(--color-gray-100);
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

  .channel {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
</style>

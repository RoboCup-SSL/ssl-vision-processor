<script lang="ts">
  import FieldSketch from "../FieldSketch.svelte";
  import { SKETCH_MARGIN } from "../fieldSplit";
  import { virtualField } from "../geometry.svelte";
  import type { ConfigDocument, CameraDoc } from "../config.svelte";
  import VideoPlayer from "../video/VideoPlayer.svelte";
  import { layoutView, flips } from "../layoutView.svelte";
  import { video as videoText } from "../text/video";
  import type { StreamStatus } from "../video/videoStream";
  import {
    slots,
    cameraDevice,
    deviceLabel,
    hostColor,
    hostName,
    liveLabel,
    type SlotInfo,
  } from "../cameraLayout";

  interface Props {
    doc: ConfigDocument;
    selected: number;
    onselect: (cameraId: number) => void;
  }

  let { doc, selected, onselect }: Props = $props();

  let length = $derived(doc.field.fieldLength ?? 0);
  let width = $derived(doc.field.fieldWidth ?? 0);
  let halfLength = $derived(length / 2 + SKETCH_MARGIN);
  let halfWidth = $derived(width / 2 + SKETCH_MARGIN);

  // Inset per side, in percent, so neighbouring tiles' borders don't touch.
  const GAP = 0.4;

  let flip = $derived(flips());

  // The latest received frame rate per camera, from each tile's video status
  // (about once a second), with when it arrived.
  let fps = $state<Record<number, { value: number; at: number } | undefined>>(
    {},
  );
  let now = $state(Date.now());

  function onstatus(status: StreamStatus): void {
    fps[status.camera] = status.receiving
      ? { value: status.fps, at: Date.now() }
      : undefined;
  }

  // A rate goes stale when its player stops reporting: video turned off, the
  // camera moved, the stream gone. Shown only while recent.
  const FPS_FRESH_MS = 2500;

  $effect(() => {
    const timer = setInterval(() => {
      now = Date.now();
    }, 1000);

    return () => {
      clearInterval(timer);
    };
  });

  function freshFps(camera: number): number | undefined {
    const reading = fps[camera];

    return reading && now - reading.at < FPS_FRESH_MS
      ? reading.value
      : undefined;
  }

  // Field mm to percent of the sketch: +x right and +y up, unless the view
  // is turned around or mirrored.
  function placement(slot: SlotInfo): string {
    const { minX, maxX, minY, maxY } = slot.slice;
    const fromLeft = flip.x ? halfLength - maxX : minX + halfLength;
    const fromTop = flip.y ? minY + halfWidth : halfWidth - maxY;
    const left = (fromLeft / (2 * halfLength)) * 100 + GAP;
    const top = (fromTop / (2 * halfWidth)) * 100 + GAP;
    const w = ((maxX - minX) / (2 * halfLength)) * 100 - 2 * GAP;
    const h = ((maxY - minY) / (2 * halfWidth)) * 100 - 2 * GAP;

    return `left:${String(left)}%;top:${String(top)}%;width:${String(w)}%;height:${String(h)}%`;
  }

  // The tile corner the details go in: the outer one on screen, preferring
  // the top and the left. A tile spanning the whole width or height counts as
  // left or top, so one camera is top-left, two are top-left and top-right,
  // and four take all four corners.
  function corner(slot: SlotInfo): { top: boolean; left: boolean } {
    const { minX, maxX, minY, maxY } = slot.slice;
    const centerX = (minX + maxX) / 2;
    const centerY = (minY + maxY) / 2;
    const fullWidth = maxX - minX >= length;
    const fullHeight = maxY - minY >= width;

    return {
      left: fullWidth || (flip.x ? centerX > 0 : centerX < 0),
      top: fullHeight || (flip.y ? centerY < 0 : centerY > 0),
    };
  }

  // The camera's own orientation (how it's mounted), then the view's. A
  // mirror before a rotation equals the opposite rotation after it, so the
  // result is still one rotation and an optional mirror.
  function videoOrientation(camera: CameraDoc): {
    rotate: number;
    mirror: boolean;
  } {
    const turn =
      (camera.display?.rotate ?? 0) + (layoutView.rotate180 ? 180 : 0);
    const mirror = camera.display?.mirror ?? false;

    return layoutView.mirror
      ? { rotate: -turn, mirror: !mirror }
      : { rotate: turn, mirror };
  }
</script>

<div class="relative">
  <div
    style:transform={`scale(${flip.x ? "-1" : "1"}, ${flip.y ? "-1" : "1"})`}
  >
    <FieldSketch
      fieldLength={length}
      fieldWidth={width}
      lines={virtualField.fieldLines}
      arcs={virtualField.fieldArcs}
      square={false}
    />
  </div>

  {#each slots(doc) as slot (slot.cameraId)}
    {@const camera = slot.camera}
    {@const video = camera !== undefined && layoutView.feeds !== "off"}
    {@const at = corner(slot)}
    <button
      type="button"
      class="tile absolute overflow-hidden rounded border-2 text-left text-xs"
      class:selected={slot.cameraId === selected}
      class:warning={slot.severity === "warning"}
      class:empty={slot.severity === "empty"}
      class:error={slot.severity === "error"}
      class:video
      style={placement(slot)}
      aria-pressed={slot.cameraId === selected}
      onclick={() => {
        onselect(slot.cameraId);
      }}
    >
      {#if video && camera}
        {@const orientation = videoOrientation(camera)}
        <VideoPlayer
          cameraId={slot.cameraId}
          compact
          mode={layoutView.feeds === "live" || slot.cameraId === selected
            ? "full"
            : "keyframes"}
          rotate={orientation.rotate}
          mirror={orientation.mirror}
          {onstatus}
        />
      {/if}

      <span
        class="info absolute flex max-w-full flex-col gap-0.5"
        class:top-0={at.top}
        class:bottom-0={!at.top}
        class:left-0={at.left}
        class:right-0={!at.left}
        class:items-start={at.left}
        class:items-end={!at.left}
        class:text-right={!at.left}
        data-corner={`${at.top ? "top" : "bottom"}-${at.left ? "left" : "right"}`}
        class:p-1.5={!video}
        class:caption={video}
      >
        <span class="flex w-full items-center gap-1.5">
          <span class="text-base leading-none font-bold">{slot.cameraId}</span>
          <span class="text-gray-500 dark:text-gray-400">{slot.region}</span>
          {#if video && camera}
            <span class="font-medium">{hostName(camera)}</span>
            <span
              class="inline-block h-1.5 w-1.5 rounded-full"
              class:bg-green-500={slot.sources.some((x) => x.receiving)}
              class:bg-gray-400={!slot.sources.some((x) => x.receiving)}
              title={liveLabel(slot.sources)}
            ></span>
            {#if freshFps(slot.cameraId) !== undefined}
              <span
                class="text-gray-600 tabular-nums dark:text-gray-300"
                title={videoText.fpsTitle}
                >{videoText.fps(freshFps(slot.cameraId) ?? 0)}</span
              >
            {/if}
          {/if}
          {#if slot.conflict}
            <span class="ms-auto text-red-700" title="Camera ID conflict"
              >⚠</span
            >
          {:else if slot.severity === "warning"}
            <span class="ms-auto text-yellow-600" title="Has warnings">⚠</span>
          {/if}
        </span>

        {#if video}
          <!-- Over video only the line above shows; the rest is in the
               region card and the table. -->
        {:else if camera}
          <span class="flex items-center gap-1 font-medium">
            <span
              class={`inline-block h-2 w-2 rounded-full ${hostColor(doc, camera.instance ?? "")}`}
            ></span>
            {hostName(camera)}
          </span>
          <span class="max-w-full truncate text-gray-500 dark:text-gray-400">
            {deviceLabel(cameraDevice(doc, camera))}
          </span>
        {:else}
          <span class="text-gray-500 dark:text-gray-400 italic">No camera</span>
        {/if}

        {#if !video}
          <span
            class="flex items-center gap-1"
            class:text-red-700={slot.conflict}
            class:text-gray-500={!slot.conflict}
          >
            <span
              class="inline-block h-1.5 w-1.5 rounded-full"
              class:bg-green-500={slot.sources.some((x) => x.receiving)}
              class:bg-gray-400={!slot.sources.some((x) => x.receiving)}
            ></span>
            {slot.conflict ? "Conflict: " : ""}{liveLabel(slot.sources)}
          </span>
        {/if}
      </span>
    </button>
  {/each}
</div>

<style>
  .tile {
    background: rgb(255 255 255 / 0.7);
    border-color: rgb(255 255 255 / 0.9);
  }

  .tile:hover {
    background: rgb(255 255 255 / 0.95);
  }

  .tile.warning {
    border-color: var(--color-yellow-400);
  }

  .tile.empty {
    background: rgb(255 255 255 / 0.5);
    border-style: dashed;
    border-color: var(--color-gray-400);
  }

  .tile.error {
    border-color: var(--color-red-500);
  }

  .tile.video {
    background: #111;
  }

  /* Over the video, the details sit in a small box in the corner. */
  /* Over video, the details are a label tucked into the tile's top-left
     corner. A <button> centres its content, so .info is pinned there
     rather than left in the flow. */
  .caption {
    display: inline-flex;
    padding: 0.15rem 0.4rem;
    background: rgb(255 255 255 / 0.8);
  }

  /* Round only the corner facing into the tile. */
  .caption[data-corner="top-left"] {
    border-bottom-right-radius: 0.25rem;
  }

  .caption[data-corner="top-right"] {
    border-bottom-left-radius: 0.25rem;
  }

  .caption[data-corner="bottom-left"] {
    border-top-right-radius: 0.25rem;
  }

  .caption[data-corner="bottom-right"] {
    border-top-left-radius: 0.25rem;
  }

  /* Dark tiles over the field in dark mode, so their dark-mode text reads. */
  :global(.dark) .tile {
    background: rgb(17 24 39 / 0.7);
    border-color: rgb(255 255 255 / 0.35);
  }

  :global(.dark) .tile:hover {
    background: rgb(17 24 39 / 0.9);
  }

  :global(.dark) .tile.empty {
    background: rgb(17 24 39 / 0.45);
  }

  :global(.dark) .caption {
    background: rgb(17 24 39 / 0.8);
  }

  .tile.selected {
    border-color: var(--color-primary-700);
    box-shadow: 0 0 0 2px var(--color-primary-700);
  }
</style>

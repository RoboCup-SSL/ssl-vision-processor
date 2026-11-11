<script lang="ts">
  import type { SSL_GeometryFieldSizeJson } from "../../proto/vision/ssl_vision_geometry_pb";
  import {
    clear,
    drawField,
    drawLayer,
    extentOf,
    viewFor,
    type DrawnLayer,
    type View,
  } from "./draw";
  import { blueOnPositiveHalf } from "../teams.svelte";

  interface Props {
    field: SSL_GeometryFieldSizeJson;
    // Drawn in order, so the last is on top.
    layers: DrawnLayer[];
    flipped: boolean;
    // Marks the origin and the +x and +y directions.
    axes: boolean;
  }

  let { field, layers, flipped, axes }: Props = $props();

  // Two canvases: the field, redrawn only when it or the size changes, and
  // what's on it, redrawn on every new frame.
  let container: HTMLDivElement | undefined = $state();
  let fieldCanvas: HTMLCanvasElement | undefined = $state();
  let liveCanvas: HTMLCanvasElement | undefined = $state();

  let width = $state(0);
  let maxHeight = $state(0);
  let dpr = $state(1);

  let extent = $derived(extentOf(field));
  let height = $derived(
    Math.min(width * (extent.halfY / extent.halfX), maxHeight),
  );
  let view = $derived<View>(viewFor(extent, width, height, dpr, flipped));
  // Goals in the color of the team defending them, once the game controller
  // says which.
  let blueOnPositive = $derived(blueOnPositiveHalf());

  $effect(() => {
    if (!container) return;

    const observer = new ResizeObserver(([entry]) => {
      width = entry?.contentRect.width ?? 0;
      maxHeight = window.innerHeight * 0.75;
      dpr = window.devicePixelRatio || 1;
    });
    observer.observe(container);

    return () => {
      observer.disconnect();
    };
  });

  // Sets a canvas's backing size to the view's, which also clears it; left
  // alone when unchanged, since resizing reallocates.
  function fit(
    canvas: HTMLCanvasElement,
    v: View,
  ): CanvasRenderingContext2D | null {
    const w = Math.round(v.width * v.dpr);
    const h = Math.round(v.height * v.dpr);

    if (canvas.width !== w) canvas.width = w;
    if (canvas.height !== h) canvas.height = h;

    return canvas.getContext("2d");
  }

  $effect(() => {
    if (!fieldCanvas || view.width === 0) return;

    const ctx = fit(fieldCanvas, view);
    if (ctx) drawField(ctx, view, field, { blueOnPositive, axes });
  });

  // At most one draw per display frame, of whatever arrived last: a frame
  // already scheduled just picks up the newer layers.
  let pending = 0;
  let latest: {
    canvas: HTMLCanvasElement;
    view: View;
    layers: DrawnLayer[];
  } | null = null;

  function draw(): void {
    pending = 0;
    if (!latest) return;

    const ctx = fit(latest.canvas, latest.view);
    if (!ctx) return;

    clear(ctx, latest.view);
    for (const d of latest.layers)
      drawLayer(ctx, latest.view, d.layer, d.style);
  }

  $effect(() => {
    if (!liveCanvas || view.width === 0) return;

    latest = { canvas: liveCanvas, view, layers };
    if (pending === 0) pending = requestAnimationFrame(draw);
  });

  $effect(() => {
    return () => {
      cancelAnimationFrame(pending);
    };
  });
</script>

<div bind:this={container} class="w-full">
  <div
    class="relative mx-auto"
    style:width="{view.width}px"
    style:height="{view.height}px"
  >
    <canvas
      bind:this={fieldCanvas}
      class="absolute inset-0 h-full w-full rounded"
    ></canvas>
    <canvas bind:this={liveCanvas} class="absolute inset-0 h-full w-full"
    ></canvas>
  </div>
</div>

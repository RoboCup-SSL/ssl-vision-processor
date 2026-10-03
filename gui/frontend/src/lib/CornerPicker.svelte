<script lang="ts">
  import { untrack } from "svelte";
  import { config, cameraDoc } from "./config.svelte";

  // The corner-drag picker from the calibration UI plan. Corners live in the
  // host's working document as this camera's seed, together with the image
  // resolution they were picked at; a drag commits on release and goes live
  // (regenerating the camera's config.yml), Save persists it.
  //
  // Ordering, traced from src/calib/GeomModel.cpp's cornerCalibration: only
  // the first point in line_corners matters. It must be the corner where the
  // goal line meets the touchline at the field's (minX, minY) -- the other
  // three can be in any order, since the algorithm brute-force searches every
  // valid clockwise rotation of them and keeps whichever fits best. So this
  // picker just needs the user to mark ONE corner, not sort all four.

  interface Props {
    cameraId: number;
  }

  let { cameraId }: Props = $props();

  const view = "geomcalib_input"; // GeomModel.cpp's calibration input snapshot

  interface Point {
    x: number;
    y: number;
  }

  let cacheBuster = $state(Date.now());
  let imageWidth = $state(0);
  let imageHeight = $state(0);

  let corners = $state<Point[]>([]);

  let svgEl: SVGSVGElement | undefined = $state();
  let draggingIndex = $state<number | null>(null);

  // Which marker is the (minX, minY) corner -- see the note above. Defaults
  // to the first handle; click a different one to change it.
  let originIndex = $state(0);

  let seed = $derived(cameraDoc(cameraId)?.seed);

  // JSON of the seed as this picker last wrote it, so the effect below can
  // tell its own commit (keep markers as they are) from a change that came
  // from elsewhere (a load, a hand edit, another tab: re-seat them).
  let lastCommitted = "";

  function handleImageLoad(img: HTMLImageElement): void {
    imageWidth = img.naturalWidth;
    imageHeight = img.naturalHeight;
  }

  // (Re)seats the markers whenever the image size is known and the seed
  // changed from outside this picker. Saved corners come first (already
  // ordered goal-side first, so origin is index 0); otherwise an inset
  // rectangle so all four handles are visible and draggable immediately.
  //
  // Corners that fall outside the current image (saved against a different
  // resolution) would render entirely off the canvas -- the SVG's viewBox
  // always matches the live image -- so they fall back to the rectangle too,
  // with the resolution banner below explaining why.
  $effect(() => {
    const json = JSON.stringify(seed);
    if (imageWidth === 0 || !config.doc) return;

    untrack(() => {
      if (corners.length !== 0 && json === lastCommitted) return;

      lastCommitted = json;
      originIndex = 0;

      const saved = (seed?.lineCorners ?? []).map(([x, y]) => ({ x, y }));
      if (saved.length === 4 && fitsImage(saved)) {
        corners = saved;

        return;
      }

      const marginX = imageWidth * 0.15;
      const marginY = imageHeight * 0.15;
      corners = [
        { x: marginX, y: marginY },
        { x: imageWidth - marginX, y: marginY },
        { x: imageWidth - marginX, y: imageHeight - marginY },
        { x: marginX, y: imageHeight - marginY },
      ];
    });
  });

  function fitsImage(points: Point[]): boolean {
    return points.every(
      (p) => p.x >= 0 && p.x <= imageWidth && p.y >= 0 && p.y <= imageHeight,
    );
  }

  // Writes the markers to the document, origin first, with the resolution
  // they were placed against.
  function commit(): void {
    const camera = cameraDoc(cameraId);
    if (!camera || orderedCorners.length !== 4) return;

    const next = {
      resolution: [imageWidth, imageHeight] as [number, number],
      lineCorners: orderedCorners.map((c) => [c.x, c.y] as [number, number]),
      goalSideMarker: originIndex + 1,
    };

    lastCommitted = JSON.stringify(next);
    camera.seed = next;
  }

  // Screen pixels -> SVG user-space (== image pixel space, since viewBox is
  // set to the image's natural dimensions). Using the SVG's own CTM handles
  // however the browser has scaled it, rather than reimplementing that math.
  function toImagePoint(clientX: number, clientY: number): Point {
    if (!svgEl) return { x: 0, y: 0 };

    const pt = svgEl.createSVGPoint();
    pt.x = clientX;
    pt.y = clientY;

    const ctm = svgEl.getScreenCTM();
    if (!ctm) return { x: 0, y: 0 };

    const p = pt.matrixTransform(ctm.inverse());
    return { x: p.x, y: p.y };
  }

  function startDrag(index: number, event: PointerEvent): void {
    draggingIndex = index;
    (event.target as Element).setPointerCapture(event.pointerId);
  }

  function onDrag(event: PointerEvent): void {
    if (draggingIndex === null) return;

    const p = toImagePoint(event.clientX, event.clientY);
    corners[draggingIndex] = {
      x: Math.round(Math.max(0, Math.min(imageWidth, p.x))),
      y: Math.round(Math.max(0, Math.min(imageHeight, p.y))),
    };
  }

  function endDrag(): void {
    if (draggingIndex === null) return;

    draggingIndex = null;
    commit();
  }

  function chooseOrigin(index: number): void {
    originIndex = index;
    commit();
  }

  function refreshFrame(): void {
    cacheBuster = Date.now();
  }

  // The chosen origin corner first, the other three following in whatever
  // order they're already in -- correct per the note above, no sorting.
  let orderedCorners = $derived.by(() => {
    const origin = corners[originIndex];
    if (!origin) return [];

    return [origin, ...corners.filter((_, i) => i !== originIndex)];
  });

  let yamlSnippet = $derived(
    "line_corners:\n" +
      orderedCorners
        .map((c) => `- [${String(c.x)}, ${String(c.y)}]`)
        .join("\n"),
  );

  // How the saved seed relates to the image now being served.
  let resolutionStatus = $derived.by(() => {
    if (seed?.lineCorners.length !== 4 || imageWidth === 0) {
      return "ok";
    }

    const [w, h] = seed.resolution;
    if (w === 0 || h === 0) return "unknown";
    if (w === imageWidth && h === imageHeight) return "ok";

    return w * imageHeight === h * imageWidth ? "rescalable" : "aspect";
  });

  // Same aspect ratio, so the saved corners map onto the new image by a
  // single scale factor.
  function rescaleSeed(): void {
    if (!seed) return;

    const factor = imageWidth / seed.resolution[0];
    corners = seed.lineCorners.map(([x, y]) => ({
      x: Math.round(x * factor),
      y: Math.round(y * factor),
    }));
    originIndex = 0;
    commit();
  }

  // The label/stroke sizes below are SVG user-space units, i.e. image
  // pixels (viewBox == the image's natural size) -- not screen pixels. They
  // must scale with image resolution the same way the handle radius already
  // does, or they shrink toward invisible on a higher-res camera feed than
  // whatever this was last tuned against.
  let handleRadius = $derived(Math.max(imageWidth, imageHeight) * 0.015);
  let handleStrokeWidth = $derived(handleRadius * 0.2);
  let labelFontSize = $derived(handleRadius * 2.5);
  let labelDy = $derived(-(handleRadius * 1.8));
  let labelStrokeWidth = $derived(handleRadius * 0.3);
</script>

<section class="corner-picker">
  <h2>Corner picker</h2>
  <p class="hint">
    Drag the four markers onto the real field corners in the image below, then
    click the number on whichever one sits where the goal line meets the
    touchline nearest this field's (0,0) corner -- that one turns green and
    becomes first in the output. The other three can be in any order; the
    calibration algorithm works that out itself. Changes apply when you let go
    of a marker; Save keeps them.
  </p>

  {#if resolutionStatus === "rescalable" && seed}
    <div class="banner">
      Corners were picked at {seed.resolution[0]}x{seed.resolution[1]}, but this
      image is {imageWidth}x{imageHeight}. Same aspect ratio, so they can be
      scaled to fit.
      <button type="button" onclick={rescaleSeed}>
        Rescale corners to {imageWidth}x{imageHeight}
      </button>
    </div>
  {:else if resolutionStatus === "aspect" && seed}
    <div class="banner">
      Corners were picked at {seed.resolution[0]}x{seed.resolution[1]}, but this
      image is {imageWidth}x{imageHeight} -- a different aspect ratio, so they can't
      be scaled. Re-pick them on this image.
    </div>
  {:else if resolutionStatus === "unknown"}
    <div class="banner">
      These corners were saved without the resolution they were picked at, so a
      camera resolution change can't be detected. Move any marker to record it.
    </div>
  {/if}

  <button type="button" onclick={refreshFrame}>Refresh frame</button>

  <div
    class="overlay-container"
    role="application"
    aria-label="Calibration corner picker"
    onpointermove={onDrag}
    onpointerup={endDrag}
    onpointercancel={endDrag}
  >
    <img
      src={`/api/snapshot/${String(cameraId)}/${view}?t=${String(cacheBuster)}`}
      alt={`cam ${String(cameraId)} calibration input`}
      onload={(e: Event) => {
        handleImageLoad(e.currentTarget as HTMLImageElement);
      }}
    />

    {#if imageWidth > 0}
      <svg
        bind:this={svgEl}
        viewBox={`0 0 ${String(imageWidth)} ${String(imageHeight)}`}
        preserveAspectRatio="none"
      >
        {#each corners as corner, index (index)}
          <circle
            cx={corner.x}
            cy={corner.y}
            r={handleRadius}
            stroke-width={handleStrokeWidth}
            class="handle"
            class:origin={index === originIndex}
            role="presentation"
            onpointerdown={(e) => {
              startDrag(index, e);
            }}
          />
          <text
            x={corner.x}
            y={corner.y}
            dy={labelDy}
            font-size={labelFontSize}
            stroke-width={labelStrokeWidth}
            role="button"
            tabindex="0"
            onclick={() => {
              chooseOrigin(index);
            }}
            onkeydown={(e) => {
              if (e.key === "Enter" || e.key === " ") chooseOrigin(index);
            }}
          >
            {index + 1}
          </text>
        {/each}
        {#if corners.length === 4}
          <polygon
            points={corners
              .map((c) => `${String(c.x)},${String(c.y)}`)
              .join(" ")}
          />
        {/if}
      </svg>
    {/if}
  </div>

  <pre>{yamlSnippet}</pre>
</section>

<style>
  .corner-picker {
    max-width: 900px;
    margin-top: 2rem;
  }

  .overlay-container {
    position: relative;
    max-width: 640px;
    touch-action: none; /* dragging must not scroll the page on touch */
  }

  .overlay-container img {
    display: block;
    width: 100%;
    height: auto;
    background: #222;
  }

  .overlay-container svg {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }

  .handle {
    fill: orange;
    stroke: black;
    cursor: grab;
  }

  .handle.origin {
    fill: #2ecc40;
  }

  .handle:active {
    cursor: grabbing;
  }

  text {
    fill: white;
    text-anchor: middle;
    paint-order: stroke;
    stroke: black;
    cursor: pointer;
  }

  polygon {
    fill: rgba(255, 165, 0, 0.15);
    stroke: orange;
    stroke-width: 2;
    pointer-events: none;
  }

  pre {
    background: #f5f5f5;
    padding: 0.75rem;
    border-radius: 4px;
    font-size: 0.85rem;
  }

  .banner {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem;
    margin: 0.5rem 0;
    padding: 0.5rem 0.75rem;
    border: 1px solid #ffe1a8;
    border-radius: 4px;
    background: #fff6e5;
    color: #7a4a00;
    font-size: 0.85rem;
  }

  .hint {
    color: #888;
    font-size: 0.8rem;
    font-style: italic;
  }
</style>

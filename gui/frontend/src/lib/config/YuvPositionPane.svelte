<script lang="ts">
  // Issue 18's proposed YUV pane: "reference color positions (editable),
  // currently-autoadapted color positions (display), current color blob
  // sample positions (debug, not yet in the protocol)." This is the first
  // slice -- the editable reference position. Live/autoadapted position and
  // blob samples both wait on the other contributor's protobuf work
  // (VPColor.live isn't reachable from this host yet).
  //
  // U/V (hue+saturation-ish) come from dragging the plane; Y (brightness)
  // comes from the slider beside it -- a 2D slice + 1D slider, the same
  // decomposition every general-purpose color picker uses (HSV square + a
  // separate value bar), rather than one flat control trying to carry all
  // three axes at once.
  import {
    rgbToYuv,
    yuvToRgb,
    yuvInGamut,
    rgbToCss,
    MIN_REACHABLE_Y,
    MAX_REACHABLE_Y,
    COLOR_CLASSES,
    CLASS_LABELS,
    type ColorClass,
    type RGB,
  } from "../color.svelte";
  import { preferences } from "../preferences.svelte";

  interface Props {
    color?: RGB;
    colors: Record<ColorClass, RGB>;
    selectedClass?: ColorClass;
  }

  let {
    color = $bindable({ r: 128, g: 128, b: 128 }),
    colors,
    selectedClass = $bindable("orange"),
  }: Props = $props();

  let label = $derived(CLASS_LABELS[selectedClass]);

  // U and V both range 0-255 in this codebase's own YUV (see
  // color.svelte.ts's rgbToYuv) -- the field is drawn one canvas pixel per
  // U/V value, so PLANE_SIZE doubles as both the value range and the
  // canvas's pixel dimensions.
  const PLANE_SIZE = 255;

  let canvasEl: HTMLCanvasElement | undefined = $state();
  let containerEl: HTMLDivElement | undefined = $state();
  let dragging = $state(false);

  // sliceY is which Y the plane/slider currently show -- distinct from
  // rgbToYuv(color).y (see actualYuv below). It's local, pinned state rather
  // than derived from color every render, specifically to fix a real bug: a
  // pure U/V drag can land on an out-of-gamut point, whose *clamped* RGB
  // implies a slightly different Y than intended. If the redraw were keyed
  // off that re-derived Y, every drag step near the gamut edge would
  // re-trigger a full-canvas repaint -- what looked like "the frame changes
  // while I drag." Pinning sliceY for the duration of interaction (see the
  // sync effect below, gated on !dragging) removes that feedback loop.
  let sliceY = $state(rgbToYuv(color).y);

  // The color actually stored right now, exactly as-is (post-clamp) -- used
  // for the marker position and the U/V readout, so both stay accurate to
  // what's really saved even in the rare case a drag landed out of gamut.
  let actualYuv = $derived(rgbToYuv(color));

  // True whenever the slider/wheel have moved the viewed slice away from the
  // saved color's own brightness -- see onYInput/onWheel below, neither of
  // which touch color. Surfaced in the Y readout so it's visible that
  // nothing has been committed yet.
  let previewing = $derived(sliceY !== actualYuv.y);

  // What color this pane itself last wrote, alongside sliceY -- lets the
  // sync effect below tell "color changed because something outside this
  // pane changed it" apart from "color changed because a plane drag just
  // committed it" (applyPlaneDrag is the only internal writer of color now
  // that the slider/wheel are preview-only). Needed because yuvToRgb ->
  // rgbToYuv isn't perfectly lossless (independent rounding on each side of
  // the round trip can shift Y by 1); without this, a drag commit's own
  // write would immediately get partly undone and re-derived by the sync
  // effect, drifting by a fluctuating +-1 instead of landing exactly on the
  // requested step.
  let lastAppliedColor = color;

  function setSlice(y: number, u: number, v: number): void {
    color = yuvToRgb({ y, u, v });
    // sliceY is set from the *actual* committed color's Y, not the requested
    // y -- yuvToRgb -> rgbToYuv can shift Y by 1 (independent integer
    // rounding on each side), so re-deriving it here is what makes
    // `previewing` correctly read false immediately after a commit, instead
    // of staying stuck on for the rest of the session over 1 unit of
    // rounding noise the user never asked about.
    sliceY = rgbToYuv(color).y;
    lastAppliedColor = color;
  }

  // Keeps sliceY following color when it changes from outside this pane
  // (the R/G/B fields, switching selected class, a fresh /api/config/color
  // load) -- but not while the user is actively dragging the plane or the Y
  // slider (would fight the pinning above), and not for a change this pane
  // just made itself (would reintroduce the rounding-drift this is here to
  // prevent).
  $effect(() => {
    if (dragging) return;
    if (
      color.r === lastAppliedColor.r &&
      color.g === lastAppliedColor.g &&
      color.b === lastAppliedColor.b
    )
      return;

    sliceY = rgbToYuv(color).y;
    lastAppliedColor = color;
  });

  // Redraws only when sliceY changes (a slider drag, or the sync above) --
  // never on a plane-only U/V drag, which is the whole point of pinning Y.
  $effect(() => {
    drawField(sliceY);
  });

  // How much of the square is a real color at the currently-viewed
  // brightness -- shrinks toward the extremes (near black or near white,
  // almost nothing is reachable). Surfaced in the UI specifically so a
  // near-empty square near those extremes reads as "this brightness barely
  // has any real colors," not "the canvas failed to render."
  let gamutFraction = $state(1);

  // Out-of-gamut pixels are shaded as a muted checkerboard rather than drawn
  // with their (clamped, misleading) color -- same convention as a
  // transparency checker, and the same idea as a CIE chromaticity diagram
  // graying out everything outside the reachable triangle. The boundary
  // between the two is additionally outlined in a solid color (see the
  // second pass below): near the brightness extremes the reachable area
  // shrinks to a small, sharply-angled shape rather than a square, and
  // without a deliberate-looking edge that reads as a rendering bug, not "a
  // real, if unfamiliar, region of a color space."
  function drawField(y: number): void {
    if (!canvasEl) return;

    const ctx = canvasEl.getContext("2d");
    if (!ctx) return;

    const size = PLANE_SIZE + 1;
    const inGamut = new Uint8Array(size * size);
    let inGamutCount = 0;

    for (let v = 0; v < size; v++) {
      for (let u = 0; u < size; u++) {
        if (yuvInGamut({ y, u, v })) {
          inGamut[v * size + u] = 1;
          inGamutCount++;
        }
      }
    }

    gamutFraction = inGamutCount / (size * size);

    const image = ctx.createImageData(size, size);
    const isInGamut = (u: number, v: number): boolean =>
      u >= 0 && u < size && v >= 0 && v < size && inGamut[v * size + u] === 1;

    for (let v = 0; v < size; v++) {
      for (let u = 0; u < size; u++) {
        const i = (v * size + u) * 4;

        if (isInGamut(u, v)) {
          // An edge pixel -- in gamut, but with an out-of-gamut neighbor --
          // is drawn as a solid outline instead of its real color, tracing
          // the boundary so it reads as a deliberate shape.
          const isEdge =
            !isInGamut(u - 1, v) ||
            !isInGamut(u + 1, v) ||
            !isInGamut(u, v - 1) ||
            !isInGamut(u, v + 1);

          if (isEdge) {
            image.data[i] = 34;
            image.data[i + 1] = 34;
            image.data[i + 2] = 34;
          } else {
            const rgb = yuvToRgb({ y, u, v });
            image.data[i] = rgb.r;
            image.data[i + 1] = rgb.g;
            image.data[i + 2] = rgb.b;
          }
        } else {
          const shade = (u + v) % 16 < 8 ? 212 : 226;
          image.data[i] = shade;
          image.data[i + 1] = shade;
          image.data[i + 2] = shade;
        }

        image.data[i + 3] = 255;
      }
    }

    ctx.putImageData(image, 0, 0);
  }

  // Uses the container's rendered (CSS) box, not the canvas's own pixel
  // buffer -- the canvas is drawn at PLANE_SIZE+1 px internally but may be
  // displayed at any CSS size, and this keeps drag math correct regardless
  // of that scale.
  function uvFromEvent(event: PointerEvent): { u: number; v: number } {
    if (!containerEl) return { u: actualYuv.u, v: actualYuv.v };

    const rect = containerEl.getBoundingClientRect();
    const u = ((event.clientX - rect.left) / rect.width) * PLANE_SIZE;
    const v = ((event.clientY - rect.top) / rect.height) * PLANE_SIZE;

    return {
      u: Math.round(Math.min(PLANE_SIZE, Math.max(0, u))),
      v: Math.round(Math.min(PLANE_SIZE, Math.max(0, v))),
    };
  }

  function applyPlaneDrag(event: PointerEvent): void {
    const { u, v } = uvFromEvent(event);
    setSlice(sliceY, u, v);
  }

  function startPlaneDrag(event: PointerEvent): void {
    dragging = true;
    (event.target as Element).setPointerCapture(event.pointerId);
    applyPlaneDrag(event);
  }

  function onPlaneDrag(event: PointerEvent): void {
    if (!dragging) return;
    applyPlaneDrag(event);
  }

  function endDrag(): void {
    dragging = false;
  }

  function startYDrag(): void {
    dragging = true;
  }

  // Moving the Y slider only previews a different brightness slice -- it
  // does NOT touch the saved color. A user scrubbing brightness has no way
  // to know whether their current U/V sits in-gamut at wherever they land,
  // so committing a color here would either silently change hue/saturation
  // (gamut clamping distorts more than just brightness) or, worse, do it
  // invisibly while they're "just looking." The marker keeps showing the
  // real saved U/V throughout -- including inside the shaded, unreachable
  // region if this slice can't represent it -- and only a click/drag inside
  // the square itself (applyPlaneDrag) commits a new color.
  function onYInput(event: Event): void {
    sliceY = Number((event.currentTarget as HTMLInputElement).value);
  }

  // Scroll to preview a different brightness slice while hovering either the
  // plane or the slider -- one notch per wheel tick regardless of deltaY's
  // magnitude (trackpads and mice report wildly different deltaY scales, so
  // using the raw value would feel inconsistent across devices).
  // preventDefault stops the page itself from scrolling. Same non-committing
  // behavior as onYInput above -- see its comment.
  function onWheel(event: WheelEvent): void {
    event.preventDefault();

    const step = event.deltaY > 0 ? -1 : 1;
    sliceY = Math.min(
      MAX_REACHABLE_Y,
      Math.max(MIN_REACHABLE_Y, sliceY + step),
    );
  }
</script>

<div class="yuv-pane">
  <div class="plane-row">
    <div
      class="plane"
      bind:this={containerEl}
      onpointerdown={startPlaneDrag}
      onpointermove={onPlaneDrag}
      onpointerup={endDrag}
      onpointercancel={endDrag}
      onwheel={onWheel}
    >
      <canvas
        bind:this={canvasEl}
        width={PLANE_SIZE + 1}
        height={PLANE_SIZE + 1}
      ></canvas>
      <div
        class="marker"
        style:left={`${String((actualYuv.u / PLANE_SIZE) * 100)}%`}
        style:top={`${String((actualYuv.v / PLANE_SIZE) * 100)}%`}
        title={label ? `${label} reference` : "reference"}
      ></div>
    </div>

    <div class="y-slider-wrap" onwheel={onWheel}>
      <input
        type="range"
        class="y-slider"
        min={MIN_REACHABLE_Y}
        max={MAX_REACHABLE_Y}
        value={sliceY}
        onpointerdown={startYDrag}
        onpointerup={endDrag}
        oninput={onYInput}
      />
    </div>

    <div class="side-column">
      <div class="class-list">
        {#each COLOR_CLASSES as cls (cls)}
          <button
            type="button"
            class="class-button"
            class:selected={selectedClass === cls}
            onclick={() => (selectedClass = cls)}
          >
            <span class="swatch" style:background={rgbToCss(colors[cls])}
            ></span>
            {CLASS_LABELS[cls]}
          </button>
        {/each}
      </div>

      <dl class="readout">
        <dt>Y (brightness)</dt>
        <dd>{sliceY}{previewing ? " (previewing)" : ""}</dd>
        <dt>U</dt>
        <dd>{actualYuv.u}</dd>
        <dt>V</dt>
        <dd>{actualYuv.v}</dd>
        <dt>Reachable</dt>
        <dd>{Math.round(gamutFraction * 100)}%</dd>
      </dl>
    </div>
  </div>

  {#if preferences.tooltipsEnabled && gamutFraction < 0.3}
    <p class="gamut-note">
      Only {Math.round(gamutFraction * 100)}% of the square is a real color at
      this brightness -- the outlined shape, not a full square, is expected
      here: brightness this close to {sliceY < 128 ? "black" : "white"} genuinely
      has few reachable colors. Not a rendering glitch.
    </p>
  {/if}

  <p class="hint">
    Drag or click inside the square to set {label || "the reference color"}'s
    color at the brightness shown. Dragging the slider (or scrolling over either
    control) only previews a different brightness -- the marker stays exactly
    where the saved color actually is, even inside the shaded region if that
    color isn't reachable at the previewed brightness; nothing is saved until
    you click or drag inside the square again. The currently-autoadapted
    position and per-frame blob samples aren't shown yet: both need the other
    contributor's protobuf work to reach this host.
  </p>
</div>

<style>
  /* Plane/slider footprint: 220px original -> 440px (2x) -> 660px (another
     1.5x). PLANE_SIZE (the U/V value range, in the script above) is
     unrelated and unchanged; uvFromEvent already reads the container's live
     rendered box via getBoundingClientRect rather than a hardcoded pixel
     size, so resizing here needs no script changes. */
  .yuv-pane {
    max-width: 1100px;
  }

  .plane-row {
    display: flex;
    align-items: flex-start;
    gap: 1rem;
  }

  .plane {
    position: relative;
    width: 660px;
    height: 660px;
    flex-shrink: 0;
    touch-action: none;
    border: 1px solid #ccc;
    border-radius: 4px;
    overflow: hidden;
    cursor: crosshair;
  }

  canvas {
    display: block;
    width: 100%;
    height: 100%;
  }

  .marker {
    position: absolute;
    width: 1.65rem;
    height: 1.65rem;
    margin: -0.825rem 0 0 -0.825rem;
    border-radius: 50%;
    background: white;
    border: 2px solid #222;
    box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.8);
    pointer-events: none;
  }

  /* A rotated horizontal range input, not a "real" vertical one --
     writing-mode/orient-based vertical sliders are unreliable across
     browsers (can end up with no visible track/thumb at all). The wrapper
     is sized to the rotated slider's visual footprint (thin, tall); the
     input itself stays a normal wide/short slider and is rotated -90deg
     (max at top, since -90deg turns "right" into "up") around its own
     center, which native pointer dragging still hit-tests correctly. */
  .y-slider-wrap {
    position: relative;
    width: 2.1rem;
    height: 660px;
    flex-shrink: 0;
  }

  .y-slider {
    position: absolute;
    top: 50%;
    left: 50%;
    width: 660px;
    height: 2.1rem;
    margin: 0;
    transform: translate(-50%, -50%) rotate(-90deg);
    /* appearance: none, not just relying on the native widget -- Firefox's
       native range rendering (-moz-appearance: slider-horizontal) doesn't
       reliably paint at all once the element is absolutely positioned and
       rotated (confirmed: the box takes up its reserved layout space, but
       renders nothing, in Firefox specifically). Drawing the track/thumb
       ourselves below removes the dependence on either engine's native
       paint path under a transform. */
    appearance: none;
    -webkit-appearance: none;
    -moz-appearance: none;
    background: transparent;
    cursor: pointer;
  }

  .y-slider::-webkit-slider-runnable-track {
    width: 100%;
    height: 9px;
    border-radius: 4.5px;
    background: #ccc;
  }

  .y-slider::-moz-range-track {
    width: 100%;
    height: 9px;
    border-radius: 4.5px;
    background: #ccc;
  }

  .y-slider::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 36px;
    height: 36px;
    margin-top: -13.5px; /* centers a 36px thumb on the 9px track above */
    border-radius: 50%;
    background: #1565c0;
    border: 2px solid white;
    box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.2);
  }

  .y-slider::-moz-range-thumb {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    background: #1565c0;
    border: 2px solid white;
    box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.2);
  }

  /* Firefox's default focus ring on a range input is drawn on this
     pseudo-element, outside the thumb -- looks wrong rotated. */
  .y-slider::-moz-focus-outer {
    border: 0;
  }

  .side-column {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .class-list {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .class-button {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.3rem 0.6rem;
    border: 1px solid #ccc;
    border-radius: 999px;
    background: white;
    font-size: 0.85rem;
    cursor: pointer;
  }

  .class-button.selected {
    border-color: #1565c0;
    background: #e8f0fe;
  }

  .swatch {
    display: inline-block;
    width: 0.9rem;
    height: 0.9rem;
    flex-shrink: 0;
    border-radius: 2px;
    border: 1px solid rgba(0, 0, 0, 0.2);
  }

  .readout {
    display: grid;
    grid-template-columns: auto auto;
    gap: 0.15rem 0.6rem;
    font-size: 0.85rem;
  }

  .readout dt {
    color: #666;
  }

  .readout dd {
    margin: 0;
    font-variant-numeric: tabular-nums;
  }

  .gamut-note {
    color: #7a5c00;
    background: #fff8e1;
    border: 1px solid #ffe1a8;
    border-radius: 4px;
    padding: 0.4rem 0.6rem;
    margin-top: 0.5rem;
    font-size: 0.8rem;
  }

  .hint {
    color: #666;
    font-size: 0.8rem;
    margin-top: 0.5rem;
  }
</style>

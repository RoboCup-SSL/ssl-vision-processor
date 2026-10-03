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
    yuvGamutOverflow,
    rgbToCss,
    MIN_REACHABLE_Y,
    MAX_REACHABLE_Y,
    COLOR_CLASSES,
    CLASS_LABELS,
    CANONICAL_COLORS,
    CLASS_LETTERS,
    defaultColorConfig,
    type ColorClass,
    type RGB,
  } from "../color.svelte";
  import { preferences } from "../preferences.svelte";
  import { Button, ButtonGroup, Modal } from "flowbite-svelte";

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

  // "All" shows every class's marker at once, each at its own real U/V,
  // rendered in a fixed identity color (see CANONICAL_COLORS) rather than its
  // actual configured color -- see the conversation this was scoped from:
  // deliberately the "honest" representation (the background stays whatever
  // single Y slice is currently shown, so most of the six will often sit in
  // the shaded/unreachable region relative to it -- that's real information,
  // not a rendering bug, same as the single-marker case).
  let showAll = $state(true);

  let allMarkerPositions = $derived(
    COLOR_CLASSES.map((cls) => ({ cls, yuv: rgbToYuv(colors[cls]) })),
  );

  // U and V both range 0-255 in this codebase's own YUV (see
  // color.svelte.ts's rgbToYuv) -- the field is drawn one canvas pixel per
  // U/V value, so PLANE_SIZE doubles as both the value range and the
  // canvas's pixel dimensions.
  const PLANE_SIZE = 255;

  // Screen space (CSS "top", canvas rows) increases downward; canonical YUV
  // diagrams -- Wikipedia's UV plane included -- draw V increasing upward.
  // Every place that converts between "V, the data value" and "on-screen
  // vertical position" goes through one of these two, so the flip is applied
  // exactly once and can't drift out of sync between the canvas draw, the
  // marker position, and the pointer-drag math.
  function vToScreenFraction(v: number): number {
    return 1 - v / PLANE_SIZE;
  }

  function screenFractionToV(fraction: number): number {
    return (1 - fraction) * PLANE_SIZE;
  }

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

  // Where sliceY sits along the slider's own min-max range, 0 (min) to 1
  // (max) -- drives the floating "Y=..." label's position so it tracks the
  // thumb. Max is at the top of the rendered slider (see .y-slider's own
  // comment on the -90deg rotation), so the label's CSS `top` uses
  // 1 - this fraction.
  let sliderThumbFraction = $derived(
    (sliceY - MIN_REACHABLE_Y) / (MAX_REACHABLE_Y - MIN_REACHABLE_Y),
  );

  // The color actually stored right now, exactly as-is (post-clamp) -- used
  // for the marker position and the U/V readout, so both stay accurate to
  // what's really saved even in the rare case a drag landed out of gamut.
  let actualYuv = $derived(rgbToYuv(color));

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
    // sliceY is deliberately NOT re-derived here on every call -- it used to
    // be, to keep the Y thumb label/readout agreeing with the actual
    // committed color's Y (yuvToRgb -> rgbToYuv can shift Y by 1, independent
    // rounding on each side). But setSlice fires on every pointermove during
    // a drag, and different U/V positions clamp differently, so re-deriving
    // sliceY on every move fed a slightly different Y into the canvas redraw
    // effect below each time -- visible as the gamut boundary (and whichever
    // corner it passes nearest) vibrating while shaking the marker side to
    // side. endPlaneDrag below does this same correction exactly once, after
    // the gesture ends, instead.
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

  // Redraws only when sliceY changes (a slider drag, the sync above, or a
  // drag into the unreachable region refitting Y) -- never on a U/V drag
  // that stays reachable, which is the whole point of pinning Y.
  //
  // Coalesced to one draw per animation frame: drawField is a full 255x255
  // scan, and a drag along the gamut edge can change sliceY on nearly every
  // pointermove, which fire faster than the display refreshes.
  let pendingSliceY: number | null = null;
  let redrawScheduled = false;

  function scheduleRedraw(y: number): void {
    pendingSliceY = y;
    if (redrawScheduled) return;

    redrawScheduled = true;
    requestAnimationFrame(() => {
      redrawScheduled = false;
      if (pendingSliceY !== null) drawField(pendingSliceY);
    });
  }

  $effect(() => {
    scheduleRedraw(sliceY);
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
      // Data stays keyed by v (isInGamut, yuvInGamut, yuvToRgb all reason in
      // data space); only the pixel this row's data ends up written to is
      // flipped, via vToScreenFraction, so V increases upward on screen.
      const row = Math.round(vToScreenFraction(v) * PLANE_SIZE);

      for (let u = 0; u < size; u++) {
        const i = (row * size + u) * 4;

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
    const v = screenFractionToV((event.clientY - rect.top) / rect.height);

    return {
      u: Math.round(Math.min(PLANE_SIZE, Math.max(0, u))),
      v: Math.round(Math.min(PLANE_SIZE, Math.max(0, v))),
    };
  }

  // The Y closest to `anchor` at which (u, v) is a real color. Dragging into
  // the hatched region at the current slice would otherwise commit a clamped
  // RGB whose actual U/V differs from the cursor's -- the marker (drawn at
  // the stored color's real U/V) visibly detaches from the cursor. Moving to
  // a brightness where the cursor's U/V is reachable keeps them together.
  // For each channel the reachable Ys form an interval, so the nearest one
  // is an interval endpoint; a plain scan over the ~220 valid Ys is cheap
  // and also covers the U/V corners that no Y reaches, where it settles for
  // the least-clipped Y instead.
  function fitYToGamut(u: number, v: number, anchor: number): number {
    let bestY = anchor;
    let bestErr = yuvGamutOverflow({ y: anchor, u, v });
    if (bestErr === 0) return anchor;

    for (let y = MIN_REACHABLE_Y; y <= MAX_REACHABLE_Y; y++) {
      const err = yuvGamutOverflow({ y, u, v });
      if (
        err < bestErr ||
        (err === bestErr && Math.abs(y - anchor) < Math.abs(bestY - anchor))
      ) {
        bestErr = err;
        bestY = y;
      }
    }

    return bestY;
  }

  function applyPlaneDrag(event: PointerEvent): void {
    const { u, v } = uvFromEvent(event);
    // Fitted against the live sliceY, so it ratchets: Y only moves when the
    // cursor leaves the region reachable at the *current* brightness, and
    // stays put when the cursor backs off into the newly opened space --
    // letting the user nudge past the edge, then ease back to pick a tone.
    // No drift from this: fitYToGamut returns sliceY unchanged whenever the
    // cursor is reachable, and sliceY is only ever set to its exact integer
    // result, never re-derived from the (rounded/clamped) committed color.
    const y = fitYToGamut(u, v, sliceY);

    // Set directly rather than re-derived from the committed color (see
    // setSlice). Same value on every move while inside the rectangle, which
    // Svelte treats as a no-op -- no redraw; a live redraw only when the
    // cursor's U/V actually needs a different brightness.
    sliceY = y;
    setSlice(y, u, v);
  }

  // One entry per drag *gesture*, not per pointermove -- applyPlaneDrag (and
  // so setSlice) fires on every move while dragging, so recording here
  // (pointerdown, before the drag's first commit) rather than inside
  // setSlice is what makes one Ctrl+Z undo the whole drag back to wherever
  // the marker was before it started, instead of stepping back one
  // sub-pixel move at a time. Keyed by class, since a later drag on a
  // different class shouldn't be affected by (or clear) this one's entry.
  //
  // $state, not a plain array: the Undo/Redo buttons' disabled attribute
  // reads .length, which needs to be reactive.
  interface HistoryEntry {
    cls: ColorClass;
    previousColor: RGB;
  }

  let undoStack = $state<HistoryEntry[]>([]);
  let redoStack = $state<HistoryEntry[]>([]);

  // Undo and redo are the same operation in opposite directions: pop an
  // entry, stash the class's *current* color onto the other stack (so the
  // move back can itself be undone/redone), then apply. Reading colors[cls]
  // rather than the color prop for that stash is what makes this correct
  // even when entry.cls isn't the currently-selected class.
  function applyHistoryEntry(
    sourceStack: HistoryEntry[],
    targetStack: HistoryEntry[],
  ): void {
    const entry = sourceStack.pop();
    if (!entry) return;

    targetStack.push({
      cls: entry.cls,
      previousColor: { ...colors[entry.cls] },
    });

    // Switching selectedClass first, then writing color, matters: color's
    // binding resolves against whatever selectedClass is *at the time of
    // that write* (ColorPanel's bind:color={colorConfig.config[selectedClass]}),
    // so this correctly lands on entry.cls's slot even if a different class
    // is currently selected.
    selectedClass = entry.cls;
    color = entry.previousColor;
  }

  function undoLastMove(): void {
    applyHistoryEntry(undoStack, redoStack);
  }

  function redoLastMove(): void {
    applyHistoryEntry(redoStack, undoStack);
  }

  // Resets to vision_processor's own default reference color
  // (defaultColorConfig -- same values shown before the first /api/config/color
  // response arrives), not the "All" view's fixed identity color
  // (CANONICAL_COLORS is a display-only convenience for telling markers
  // apart, never a value meant to be written back). Goes through the same
  // undo stack as a drag, so it's a normal Ctrl+Z step, not a separate reset
  // path the user has no way to walk back.
  //
  // Writes to `color` (not colors[selectedClass] in place) specifically for
  // the selected class: the dragging-vs-external-change sync effect above
  // compares color's fields against lastAppliedColor by value, and
  // color/colors[selectedClass]/lastAppliedColor can already all alias the
  // same object after a prior drag -- mutating that object's fields in place
  // would make the comparison trivially "unchanged" (same object, so of
  // course its fields equal themselves) and silently skip resyncing sliceY.
  // A fresh object via the spread sidesteps that.
  function restoreSelectedColor(): void {
    undoStack.push({ cls: selectedClass, previousColor: { ...color } });
    redoStack = [];
    color = { ...defaultColorConfig()[selectedClass] };
  }

  function restoreAllColors(): void {
    const defaults = defaultColorConfig();
    const currentlySelected = selectedClass;

    for (const cls of COLOR_CLASSES) {
      undoStack.push({ cls, previousColor: { ...colors[cls] } });

      if (cls === currentlySelected) {
        color = { ...defaults[cls] };
      } else {
        Object.assign(colors[cls], defaults[cls]);
      }
    }

    redoStack = [];
  }

  // Expert mode skips the confirmation, same convention as the rest of the
  // app's guard rails.
  let showRestoreAllConfirm = $state(false);

  function requestRestoreAllColors(): void {
    if (preferences.expertUser) {
      restoreAllColors();
    } else {
      showRestoreAllConfirm = true;
    }
  }

  function confirmRestoreAllColors(): void {
    showRestoreAllConfirm = false;
    restoreAllColors();
  }

  function handleKeydown(event: KeyboardEvent): void {
    const key = event.key.toLowerCase();
    if (key !== "z" && key !== "y") return;
    if (!(event.ctrlKey || event.metaKey)) return;

    // Don't hijack Ctrl+Z/Y while focus is in a text field (R/G/B inputs,
    // the minimum-reference-weight box) -- that's the browser's own native
    // undo-while-typing, unrelated to marker moves.
    const active = document.activeElement;
    if (
      active instanceof HTMLInputElement ||
      active instanceof HTMLTextAreaElement
    )
      return;

    // Ctrl+Z = undo, Ctrl+Shift+Z or Ctrl+Y = redo (covering both common
    // conventions -- macOS/most apps use Shift+Z, Windows commonly uses Y).
    event.preventDefault();
    if (key === "y" || event.shiftKey) {
      redoLastMove();
    } else {
      undoLastMove();
    }
  }

  function startPlaneDrag(event: PointerEvent): void {
    // Otherwise a real mouse drag that strays past the box also starts a
    // native text selection over whatever's under the cursor.
    event.preventDefault();
    dragging = true;
    undoStack.push({ cls: selectedClass, previousColor: { ...color } });
    // A fresh move invalidates whatever could have been redone -- same
    // convention as any other undo/redo stack (a text editor, image editor,
    // ...): redoing back to a state that a new edit has since diverged from
    // isn't meaningful.
    redoStack = [];
    // Captured on the container itself, not event.target -- in "All" mode
    // event.target can be one of the other classes' marker divs (see
    // selectMarkerClass below, which lets a drag starting on the *selected*
    // marker fall through to here), and capture needs to stay anchored to an
    // element that's still there and still bubbles pointermove up to this
    // handler for the rest of the gesture.
    containerEl?.setPointerCapture(event.pointerId);
    applyPlaneDrag(event);
  }

  // In "All" mode, clicking a marker for a class that ISN'T selected means
  // "select that color," not "move the currently-selected color here" --
  // the container's own onpointerdown (startPlaneDrag) would otherwise
  // interpret the click as a plane drag and jump the selected class's color
  // to wherever the other marker happens to sit. stopPropagation prevents
  // that. Clicking the already-selected marker is left alone (event bubbles
  // up to the container normally) so dragging it still works exactly like
  // dragging anywhere else on the plane, including into another marker's
  // space.
  function selectMarkerClass(cls: ColorClass, event: PointerEvent): void {
    if (cls === selectedClass) return;
    event.stopPropagation();
    selectedClass = cls;
  }

  function onPlaneDrag(event: PointerEvent): void {
    if (!dragging) return;
    applyPlaneDrag(event);
  }

  function endDrag(): void {
    dragging = false;
  }

  // Plane-drag-specific: on top of ending the drag, this re-syncs sliceY to
  // the actual committed color's Y exactly once, now that the gesture is
  // over (see setSlice's comment -- it deliberately stops doing this on
  // every intermediate move). Must stay separate from endDrag, which the Y
  // slider's own pointerup also uses: re-deriving sliceY there would snap
  // the slider back to the stored color's Y, destroying the "preview a
  // brightness without committing" behavior -- the slider only works
  // because letting go of it does NOT trigger this re-sync.
  function endPlaneDrag(): void {
    dragging = false;
    sliceY = rgbToYuv(color).y;
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

<svelte:window onkeydown={handleKeydown} />

<div class="yuv-pane">
  <div class="pane-header">
    <h3>Reference colors</h3>
    <div class="history-buttons">
      <Button
        size="xs"
        color="light"
        outline
        disabled={undoStack.length === 0}
        onclick={undoLastMove}
      >
        ↶ Undo
      </Button>
      <Button
        size="xs"
        color="light"
        outline
        disabled={redoStack.length === 0}
        onclick={redoLastMove}
      >
        ↷ Redo
      </Button>
    </div>
  </div>

  <div class="plane-row">
    <div
      class="plane"
      role="application"
      aria-label="YUV reference plane"
      bind:this={containerEl}
      onpointerdown={startPlaneDrag}
      onpointermove={onPlaneDrag}
      onpointerup={endPlaneDrag}
      onpointercancel={endPlaneDrag}
      onwheel={onWheel}
    >
      <canvas
        bind:this={canvasEl}
        width={PLANE_SIZE + 1}
        height={PLANE_SIZE + 1}
      ></canvas>

      {#if showAll}
        {#each allMarkerPositions as m (m.cls)}
          <div
            class="marker marker-all"
            class:active={m.cls === selectedClass}
            style:left={`${String((m.yuv.u / PLANE_SIZE) * 100)}%`}
            style:top={`${String(vToScreenFraction(m.yuv.v) * 100)}%`}
            style:background={rgbToCss(CANONICAL_COLORS[m.cls])}
            title={`${CLASS_LABELS[m.cls]} reference`}
            role="presentation"
            onpointerdown={(e) => {
              selectMarkerClass(m.cls, e);
            }}
          >
            {CLASS_LETTERS[m.cls]}
          </div>
        {/each}
      {:else}
        <div
          class="marker"
          style:left={`${String((actualYuv.u / PLANE_SIZE) * 100)}%`}
          style:top={`${String(vToScreenFraction(actualYuv.v) * 100)}%`}
          title={label ? `${label} reference` : "reference"}
        ></div>
      {/if}
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
      <div
        class="y-thumb-label"
        style:top={`${String((1 - sliderThumbFraction) * 100)}%`}
      >
        Y={sliceY}
      </div>
    </div>

    <div class="side-column">
      <div class="class-list">
        <!-- ButtonGroup + Button rather than ButtonToggleGroup: the toggle
             group is uncontrolled and deselects on a repeat click, which
             would leave neither mode lit. Colors derived from showAll keep
             this always showing exactly one selected. -->
        <ButtonGroup class="mode-group">
          <Button
            color={showAll ? "alternative" : "primary"}
            onclick={() => (showAll = false)}
          >
            Single Color
          </Button>
          <Button
            color={showAll ? "primary" : "alternative"}
            onclick={() => (showAll = true)}
          >
            All Colors
          </Button>
        </ButtonGroup>

        {#each COLOR_CLASSES as cls (cls)}
          <button
            type="button"
            class="class-button"
            class:selected={selectedClass === cls}
            onclick={() => (selectedClass = cls)}
          >
            <span
              class="swatch"
              style:background={rgbToCss(
                showAll ? CANONICAL_COLORS[cls] : colors[cls],
              )}
            ></span>
            {CLASS_LABELS[cls]}
          </button>
        {/each}

        <div
          class="current-swatch"
          style:background={rgbToCss(color)}
          title={`${label} reference`}
        ></div>

        <div class="restore-buttons">
          <Button
            size="xs"
            color="light"
            outline
            onclick={restoreSelectedColor}
          >
            Restore selected color
          </Button>
          <Button
            size="xs"
            color="light"
            outline
            onclick={requestRestoreAllColors}
          >
            Restore all colors
          </Button>
        </div>
      </div>

      <!-- The current color's own values only -- the live-previewed Y (which
           can differ while dragging the slider) has its own "Y=..." label
           floating over the slider instead. -->
      <dl class="readout">
        <dt>Y (brightness)</dt>
        <dd>{actualYuv.y}</dd>
        <dt>U</dt>
        <dd>{actualYuv.u}</dd>
        <dt>V</dt>
        <dd>{actualYuv.v}</dd>
        <dt>Reachable</dt>
        <dd>{Math.round(gamutFraction * 100)}%</dd>
      </dl>
    </div>
  </div>

  <Modal
    title="Restore all colors?"
    bind:open={showRestoreAllConfirm}
    size="xs"
  >
    <p>
      Resets all six reference colors to their defaults. Undo can step back
      through them one color at a time.
    </p>
    {#snippet footer()}
      <Button
        color="alternative"
        onclick={() => (showRestoreAllConfirm = false)}
      >
        Cancel
      </Button>
      <Button color="red" onclick={confirmRestoreAllColors}>Restore all</Button>
    {/snippet}
  </Modal>

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
    you click or drag inside the square again. Ctrl+Z (Cmd+Z on Mac) undoes the
    last drag, one gesture at a time. The currently-autoadapted position and
    per-frame blob samples aren't shown yet: both need the other contributor's
    protobuf work to reach this host.
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

  /* Same width as .plane, so Undo/Redo's right edge lines up with the color
     window's rather than the far side of the side column. */
  .pane-header {
    width: 660px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 0.5rem;
  }

  .pane-header h3 {
    margin: 0;
  }

  .history-buttons {
    display: flex;
    gap: 0.5rem;
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
    /* Alongside startPlaneDrag's preventDefault(): stops a drag that strays
       past this box from text-selecting whatever's under the cursor. */
    user-select: none;
    -webkit-user-select: none;
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

  /* "All" view: one of these per class, filled with its fixed identity
     color (CANONICAL_COLORS) rather than the single marker's actual
     configured color -- see showAll's doc comment. Text color is a fixed
     black-with-white-halo rather than picked per class, since that reads
     against all six identity colors (and field's gray) without needing
     per-color contrast logic. */
  .marker-all {
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.85rem;
    font-weight: 700;
    color: #000;
    text-shadow:
      0 0 2px #fff,
      0 0 2px #fff,
      0 0 2px #fff;
    /* Overrides .marker's pointer-events: none -- in "All" mode each marker
       is a real click target (see selectMarkerClass), not just a display
       overlay. */
    pointer-events: auto;
    cursor: pointer;
  }

  /* The class currently selected for editing stands out among the other
     five -- bigger, thicker ring, a visible halo -- so "All" still reads as
     "here's context, and here's what you're editing," not six equal dots. */
  .marker-all.active {
    z-index: 1;
    width: 2.1rem;
    height: 2.1rem;
    margin: -1.05rem 0 0 -1.05rem;
    border-width: 3px;
    box-shadow:
      0 0 0 1px rgba(255, 255, 255, 0.9),
      0 0 6px 2px rgba(21, 101, 192, 0.6);
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

  /* Tracks the thumb (see sliderThumbFraction) rather than sitting fixed --
     a live "Y=..." readout right where the eye already is while dragging,
     instead of over in the side-column readout. Wider than the slider
     itself (2.1rem) is fine: nothing here clips it, and the gap on either
     side of the slider gives it room. */
  .y-thumb-label {
    position: absolute;
    left: 50%;
    transform: translate(-50%, -50%);
    z-index: 2;
    padding: 0.15rem 0.45rem;
    border-radius: 999px;
    background: #1565c0;
    color: white;
    font-size: 0.8rem;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
    pointer-events: none;
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

  /* Invisible, not gone -- the Y=... label (.y-thumb-label) already marks
     the position and value, so a second, visually-competing circle right
     under it is redundant. Size is kept (not 0) so the drag hit target
     stays the same as before; only the paint is removed. */
  .y-slider::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 36px;
    height: 36px;
    margin-top: -13.5px; /* centers a 36px thumb on the 9px track above */
    background: transparent;
    border: none;
    box-shadow: none;
  }

  .y-slider::-moz-range-thumb {
    width: 36px;
    height: 36px;
    background: transparent;
    border: none;
    box-shadow: none;
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

  .current-swatch {
    width: 100%;
    aspect-ratio: 1;
    margin-top: 0.4rem;
    border: 1px solid rgba(0, 0, 0, 0.2);
    border-radius: 0.5rem;
  }

  .restore-buttons {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    margin-top: 0.4rem;
    padding-top: 0.4rem;
    border-top: 1px solid #ddd;
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

  /* Full column width, split evenly, matching the class buttons below. */
  :global(.mode-group) {
    display: flex;
    width: 100%;
    margin-bottom: 0.4rem;
  }

  :global(.mode-group button) {
    flex: 1;
    white-space: nowrap;
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

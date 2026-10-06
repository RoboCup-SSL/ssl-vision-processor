<script lang="ts">
  import { color } from "../text/color";
  const text = color.updateWeights;
  // Drag-to-set control for the three color-update weights (see
  // src/blobs/colorupdate.cpp:58-61's updateColor -- this triangle IS that
  // blend, visualized): reference_force, history_force, and the implied
  // "current frame" weight (1 - both). The three always sum to 1, so a point
  // inside an equilateral triangle -- its barycentric coordinates against the
  // three vertices -- is exactly the right representation: every point is a
  // valid weighting, every valid weighting is some point.
  //
  // minReferenceForce carves out a strip near the History-Update edge (where
  // referenceForce would be < the floor) as an excluded, shaded region -- see
  // colorconfig.go's ColorConfig.MinReferenceForce for why that floor exists
  // (reference is the only term never gated on having samples this frame; if
  // it can go to ~0, a sample-starved class can drift with nothing pulling it
  // back).

  import { minReferenceForceUnlock } from "../color.svelte";
  import { preferences } from "../preferences.svelte";
  import { Button, Modal, Input, Label } from "flowbite-svelte";

  interface Props {
    referenceForce?: number;
    historyForce?: number;
    minReferenceForce?: number;
  }

  let {
    referenceForce = $bindable(0.1),
    historyForce = $bindable(0.7),
    minReferenceForce = $bindable(0.05),
  }: Props = $props();

  let updateForce = $derived(1 - referenceForce - historyForce);

  // vision_processor's own fallback (Resources.cpp:202-203, colorconfig.go's
  // ReadColorConfig) -- what a color update uses when config.yml has neither
  // force set at all. "Restore defaults" puts the marker back here, not at
  // some arbitrary triangle centroid.
  const DEFAULT_REFERENCE_FORCE = 0.1;
  const DEFAULT_HISTORY_FORCE = 0.7;

  // colorconfig.go's defaultMinReferenceForce -- this one's a Go-host-only
  // safety rail, not a C++ fallback, but "restore defaults" should still put
  // it back rather than leaving behind whatever floor was last typed in.
  const DEFAULT_MIN_REFERENCE_FORCE = 0.05;

  // Triangle vertices in a 0-100 viewBox. Order matters: it fixes which
  // barycentric weight is which force everywhere below.
  const V_REF = { x: 50, y: 6 };
  const V_HIST = { x: 6, y: 94 };
  const V_UPDATE = { x: 94, y: 94 };

  function barycentricToPoint(r: number, h: number, u: number) {
    return {
      x: r * V_REF.x + h * V_HIST.x + u * V_UPDATE.x,
      y: r * V_REF.y + h * V_HIST.y + u * V_UPDATE.y,
    };
  }

  // Standard point-in-triangle barycentric inversion (A=V_REF, B=V_HIST,
  // C=V_UPDATE). Returns unclamped weights -- the caller is responsible for
  // pulling a point dragged outside the triangle back to something valid.
  function pointToBarycentric(p: { x: number; y: number }) {
    const v0 = { x: V_HIST.x - V_REF.x, y: V_HIST.y - V_REF.y };
    const v1 = { x: V_UPDATE.x - V_REF.x, y: V_UPDATE.y - V_REF.y };
    const v2 = { x: p.x - V_REF.x, y: p.y - V_REF.y };

    const d00 = v0.x * v0.x + v0.y * v0.y;
    const d01 = v0.x * v1.x + v0.y * v1.y;
    const d11 = v1.x * v1.x + v1.y * v1.y;
    const d20 = v2.x * v0.x + v2.y * v0.y;
    const d21 = v2.x * v1.x + v2.y * v1.y;

    const denom = d00 * d11 - d01 * d01;
    const h = (d11 * d20 - d01 * d21) / denom;
    const u = (d00 * d21 - d01 * d20) / denom;

    return { r: 1 - h - u, h, u };
  }

  // Rounds to keep the saved/displayed values from carrying float noise
  // picked up by the drag math (e.g. 0.30000000000000004) -- config.yml's own
  // rounding (colorconfig.go's formatForce) is coarser and independent of
  // this; this is purely for a clean live readout while dragging.
  function round(v: number): number {
    return Math.round(v * 1000) / 1000;
  }

  // Clamps a raw drag point to a valid, in-triangle, floor-respecting
  // weighting: negative-component clip + renormalize handles the point
  // having been dragged outside the triangle at all; the floor step then
  // pushes referenceForce up to the minimum while preserving the
  // history:update *ratio* the drag direction implied, rather than just
  // clamping referenceForce and letting the sum drift off 1.
  function clampToValidWeights(raw: { r: number; h: number; u: number }) {
    let r = Math.max(0, raw.r);
    let h = Math.max(0, raw.h);
    let u = Math.max(0, raw.u);

    const sum = r + h + u;
    if (sum <= 0) {
      r = 1;
      h = 0;
      u = 0;
    } else {
      r /= sum;
      h /= sum;
      u /= sum;
    }

    if (r < minReferenceForce) {
      r = minReferenceForce;
      const remaining = 1 - r;
      const hu = h + u;

      if (hu <= 0) {
        h = remaining / 2;
        u = remaining / 2;
      } else {
        h = (h / hu) * remaining;
        u = (u / hu) * remaining;
      }
    }

    return { r: round(r), h: round(h), u: round(u) };
  }

  let svgEl: SVGSVGElement | undefined = $state();
  let dragging = $state(false);

  function toSvgPoint(clientX: number, clientY: number) {
    if (!svgEl) return { x: 0, y: 0 };

    const pt = svgEl.createSVGPoint();
    pt.x = clientX;
    pt.y = clientY;

    const ctm = svgEl.getScreenCTM();
    if (!ctm) return { x: 0, y: 0 };

    const p = pt.matrixTransform(ctm.inverse());
    return { x: p.x, y: p.y };
  }

  function applyDrag(event: PointerEvent): void {
    const svgPoint = toSvgPoint(event.clientX, event.clientY);
    const weights = clampToValidWeights(pointToBarycentric(svgPoint));

    referenceForce = weights.r;
    historyForce = weights.h;
  }

  function startDrag(event: PointerEvent): void {
    dragging = true;
    (event.target as Element).setPointerCapture(event.pointerId);
    applyDrag(event);
  }

  function onDrag(event: PointerEvent): void {
    if (!dragging) return;
    applyDrag(event);
  }

  function endDrag(): void {
    dragging = false;
  }

  // Resets the floor first, then clamps reference/history against *that* --
  // not the old floor -- so this always lands exactly on
  // (0.1, 0.7, 0.2) rather than whatever the previous, possibly-raised floor
  // would have allowed.
  function resetToDefaults(): void {
    minReferenceForce = DEFAULT_MIN_REFERENCE_FORCE;

    const weights = clampToValidWeights({
      r: DEFAULT_REFERENCE_FORCE,
      h: DEFAULT_HISTORY_FORCE,
      u: 1 - DEFAULT_REFERENCE_FORCE - DEFAULT_HISTORY_FORCE,
    });

    referenceForce = weights.r;
    historyForce = weights.h;
  }

  // Raising the floor above the marker's current referenceForce (typed
  // directly into the min-force box, not dragged) previously left the marker
  // sitting inside the now-excluded zone -- the floor line/shading moved
  // (both are $derived off minReferenceForce) but nothing pulled the marker
  // itself back out. This re-clamps whenever the floor moves past wherever
  // the marker already is, same clampToValidWeights a drag or "restore
  // defaults" goes through, so the marker always stays inside the valid
  // region.
  $effect(() => {
    if (referenceForce < minReferenceForce) {
      const weights = clampToValidWeights({
        r: referenceForce,
        h: historyForce,
        u: updateForce,
      });

      referenceForce = weights.r;
      historyForce = weights.h;
    }
  });

  let marker = $derived(
    barycentricToPoint(referenceForce, historyForce, updateForce),
  );

  // The r = minReferenceForce line, parallel to the History-Update edge (that
  // edge is exactly r=0, since neither of its endpoint vertices carries any
  // reference weight). floorA sits on edge Ref-Hist (u=0), floorB on edge
  // Ref-Update (h=0) -- together with V_HIST/V_UPDATE they bound the excluded
  // trapezoid drawn below.
  let floorA = $derived(
    barycentricToPoint(minReferenceForce, 1 - minReferenceForce, 0),
  );
  let floorB = $derived(
    barycentricToPoint(minReferenceForce, 0, 1 - minReferenceForce),
  );
  let excludedZone = $derived(
    [V_HIST, floorA, floorB, V_UPDATE]
      .map((p) => `${String(p.x)},${String(p.y)}`)
      .join(" "),
  );

  function pct(v: number): string {
    return `${(v * 100).toFixed(1)}%`;
  }

  // The minimum-reference-weight box: locked by default (see
  // minReferenceForceUnlock's own doc comment for why this is module-level
  // rather than local state), plus a typing convenience -- entering anything
  // from 1-100 is read as a percentage, not a fraction, since that's the more
  // natural way to think of a weight and 0-1 fractions invite an off-by-100
  // mistake typed straight into config.yml's own units.
  let minForceText = $state(String(minReferenceForce));
  let minForceFocused = $state(false);
  let showUnlockConfirm = $state(false);

  // Expert users skip the unlock ceremony entirely -- the box just starts
  // enabled, no click/popup/acknowledgment needed. This is independent of
  // minReferenceForceUnlock.unlocked (never sets it), so toggling expert
  // mode off mid-session correctly re-locks the box rather than leaving it
  // permanently unlocked from having been "true" once.
  let minForceUnlocked = $derived(
    minReferenceForceUnlock.unlocked || preferences.expertUser,
  );

  // Keeps the text box showing minReferenceForce when it changes from
  // outside this input (e.g. a fresh /api/config/color load resolving) --
  // but only while the user isn't actively editing it, or a load landing
  // mid-keystroke would stomp whatever they were typing.
  $effect(() => {
    if (!minForceFocused) minForceText = String(minReferenceForce);
  });

  function normalizeForceInput(raw: number): number {
    const asFraction = raw >= 1 && raw <= 100 ? raw / 100 : raw;

    return round(Math.min(1, Math.max(0, asFraction)));
  }

  function commitMinForce(): void {
    const parsed = Number(minForceText);

    if (Number.isNaN(parsed)) {
      minForceText = String(minReferenceForce);

      return;
    }

    minReferenceForce = normalizeForceInput(parsed);
    minForceText = String(minReferenceForce);
  }

  function handleMinForceInput(event: Event): void {
    minForceText = (event.currentTarget as HTMLInputElement).value;
  }

  function handleMinForceKeydown(event: KeyboardEvent): void {
    if (event.key !== "Enter") return;

    event.preventDefault();
    commitMinForce();
    (event.currentTarget as HTMLInputElement).blur();
  }

  function handleMinForceFocus(): void {
    minForceFocused = true;
  }

  function handleMinForceBlur(): void {
    minForceFocused = false;
    commitMinForce();
  }

  function requestUnlock(): void {
    showUnlockConfirm = true;
  }

  function cancelUnlock(): void {
    showUnlockConfirm = false;
  }

  function confirmUnlock(): void {
    minReferenceForceUnlock.unlocked = true;
    showUnlockConfirm = false;
  }
</script>

<div class="weight-triangle">
  <div class="triangle-row">
    <svg
      bind:this={svgEl}
      viewBox="0 0 100 110"
      role="application"
      aria-label="Color weight triangle"
      onpointerdown={startDrag}
      onpointermove={onDrag}
      onpointerup={endDrag}
      onpointercancel={endDrag}
    >
      <polygon
        class="triangle"
        points={`${String(V_REF.x)},${String(V_REF.y)} ${String(V_HIST.x)},${String(V_HIST.y)} ${String(V_UPDATE.x)},${String(V_UPDATE.y)}`}
      />
      <polygon class="excluded" points={excludedZone} />
      <line
        class="floor-line"
        x1={floorA.x}
        y1={floorA.y}
        x2={floorB.x}
        y2={floorB.y}
      />

      <text class="vertex-label" x={V_REF.x} y={V_REF.y - 2}>Reference</text>
      <text
        class="vertex-label"
        x={V_HIST.x}
        y={V_HIST.y + 6}
        text-anchor="start">History</text
      >
      <text
        class="vertex-label"
        x={V_UPDATE.x}
        y={V_UPDATE.y + 6}
        text-anchor="end">Current frame</text
      >
      <text
        class="floor-label"
        x={(floorA.x + floorB.x) / 2}
        y={(floorA.y + floorB.y) / 2 + 4}
      >
        min {pct(minReferenceForce)}
      </text>

      <circle class="marker" cx={marker.x} cy={marker.y} r="2.2" />
    </svg>

    <div class="side-column">
      <dl class="readout">
        <dt>Reference</dt>
        <dd>{pct(referenceForce)}</dd>
        <dt>History</dt>
        <dd>{pct(historyForce)}</dd>
        <dt>Current frame</dt>
        <dd>{pct(updateForce)}</dd>
      </dl>

      <Button size="xs" color="light" outline onclick={resetToDefaults}>
        Restore default weights
      </Button>

      <div class="min-force-block">
        <div class="min-force">
          <Label for="min-force" class="font-normal"
            >Minimum reference weight</Label
          >
          <div class="w-20 shrink-0">
            <Input
              id="min-force"
              type="number"
              size="sm"
              step="0.01"
              value={minForceText}
              disabled={!minForceUnlocked}
              oninput={handleMinForceInput}
              onkeydown={handleMinForceKeydown}
              onfocus={handleMinForceFocus}
              onblur={handleMinForceBlur}
            />
          </div>
        </div>

        {#if !minForceUnlocked}
          <Button size="xs" color="light" outline onclick={requestUnlock}>
            Unlock
          </Button>

          <Modal
            title={text.unlockTitle}
            bind:open={showUnlockConfirm}
            size="xs"
          >
            <p>{text.unlockBody}</p>
            {#snippet footer()}
              <Button color="alternative" onclick={cancelUnlock}>Cancel</Button>
              <Button color="red" onclick={confirmUnlock}>
                {text.unlockConfirm}
              </Button>
            {/snippet}
          </Modal>
        {/if}
      </div>
    </div>
  </div>
</div>

<style>
  .weight-triangle {
    /* Matches YuvPositionPane/.color-panel's cap (1100px) so this section
       spans the same width as the color picker above it, for a lined-up
       right edge -- not because the triangle itself needs to be that wide. */
    width: 100%;
    max-width: 1100px;
  }

  /* Same shape as YuvPositionPane's .plane-row: the main visual on the left,
     a vertical stack of controls/readout to its right. */
  .triangle-row {
    display: flex;
    align-items: flex-start;
    gap: 1.5rem;
  }

  svg {
    width: 560px;
    /* viewBox is 100x110, not 100x100 -- the History/Current frame vertex
       labels sit a few units below the triangle's own bottom edge (y=94+6),
       which the original 260px render made negligible but is a real visible
       overlap with whatever follows this component once the triangle is
       this much bigger. Height follows the viewBox's aspect ratio so the
       triangle itself still renders at the full 560px width. */
    height: 616px;
    touch-action: none;
    flex-shrink: 0;
  }

  .triangle {
    fill: var(--surface-muted);
    stroke: var(--line-strong);
    stroke-width: 0.5;
  }

  .excluded {
    fill: rgba(176, 0, 32, 0.08);
    pointer-events: none;
  }

  .floor-line {
    stroke: #b00020;
    stroke-width: 0.4;
    stroke-dasharray: 2 1.5;
    pointer-events: none;
  }

  .floor-label {
    fill: #b00020;
    font-size: 3.2px;
    text-anchor: middle;
    pointer-events: none;
  }

  .vertex-label {
    fill: var(--text-subtle);
    font-size: 4px;
    text-anchor: middle;
    pointer-events: none;
  }

  .marker {
    fill: var(--color-primary-700);
    stroke: white;
    stroke-width: 0.6;
    cursor: grab;
  }

  .side-column {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 1rem;
  }

  .readout {
    display: grid;
    grid-template-columns: auto auto;
    gap: 0.15rem 0.6rem;
    margin: 0;
    font-size: 0.85rem;
  }

  .readout dt {
    color: var(--text-muted);
  }

  .readout dd {
    margin: 0;
    font-variant-numeric: tabular-nums;
  }

  .min-force-block {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0.5rem;
  }

  .min-force {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.85rem;
    color: var(--text-subtle);
  }
</style>

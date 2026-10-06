<script lang="ts">
  // Live view of src/blobs/colorupdate.cpp:58-61's updateColor blend for
  // whichever class is currently selected:
  //   new = referenceForce * reference + historyForce * history + updateForce * current
  // referenceColor is real (it's what's being edited on this panel). history/
  // current are symbolic for now -- the live/autoadapted color and this-frame
  // blob samples both come from the other contributor's protobuf work
  // (VPColor.live, not yet wired to this host), not from config.yml. Swap
  // those two swatches for real ones once that lands; the weight numbers here
  // are already live off the same state the triangle drags.
  import type { RGB } from "../color.svelte";

  interface Props {
    referenceForce: number;
    historyForce: number;
    updateForce: number;
    referenceLabel: string;
    referenceColor: RGB;
  }

  let {
    referenceForce,
    historyForce,
    updateForce,
    referenceLabel,
    referenceColor,
  }: Props = $props();

  function pct(v: number): string {
    return `${(v * 100).toFixed(1)}%`;
  }

  function css(c: RGB): string {
    return `rgb(${String(c.r)}, ${String(c.g)}, ${String(c.b)})`;
  }
</script>

<p class="equation">
  <span class="hint">new {referenceLabel} =</span>
  <span class="term">
    <span class="swatch" style:background={css(referenceColor)}></span>
    {pct(referenceForce)} · reference
  </span>
  <span class="op">+</span>
  <span class="term">
    <span class="swatch placeholder"></span>
    {pct(historyForce)} · history
  </span>
  <span class="op">+</span>
  <span class="term">
    <span class="swatch placeholder"></span>
    {pct(updateForce)} · current frame
  </span>
</p>

<style>
  .equation {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.85rem;
    font-variant-numeric: tabular-nums;
  }

  .hint {
    color: var(--text-muted);
  }

  .term {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
  }

  .op {
    color: var(--text-muted);
  }

  .swatch {
    display: inline-block;
    width: 0.9rem;
    height: 0.9rem;
    border-radius: 2px;
    border: 1px solid rgba(0, 0, 0, 0.2);
  }

  .swatch.placeholder {
    background: repeating-linear-gradient(
      45deg,
      var(--line),
      var(--line) 3px,
      var(--surface-muted) 3px,
      var(--surface-muted) 6px
    );
  }
</style>

<script lang="ts">
  // A schematic, not the real computed geometry: the real fieldLines/fieldArcs
  // only exist server-side once a field is saved (see geometry.svelte.ts's
  // refreshFieldMarkings), but the wizard's draft never touches the server
  // until Finish. Dimensions haven't even been asked yet at this step (Markings
  // runs before Dimensions, see wizard.svelte.ts), so this uses placeholder
  // proportions when the draft has none yet -- it's here to show what each
  // marking *is*, not to preview exact sizing.
  import { wizard } from "../wizard.svelte";

  const DEFAULT_LENGTH = 9000;
  const DEFAULT_WIDTH = 6000;
  const DEFAULT_PENALTY_DEPTH = 1200;
  const DEFAULT_PENALTY_WIDTH = 2400;
  const DEFAULT_CENTER_CIRCLE_RADIUS = 500;

  // Not `??`: 0 is as meaningless a dimension as undefined for this
  // schematic, and should fall back to the default too.
  function orDefault(value: number | undefined, fallback: number): number {
    // eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing -- see above
    return value ? value : fallback;
  }

  let length = $derived(
    orDefault(wizard.draft.field.fieldLength, DEFAULT_LENGTH),
  );
  let width = $derived(orDefault(wizard.draft.field.fieldWidth, DEFAULT_WIDTH));
  let penaltyDepth = $derived(
    orDefault(wizard.draft.field.penaltyAreaDepth, DEFAULT_PENALTY_DEPTH),
  );
  let penaltyWidth = $derived(
    orDefault(wizard.draft.field.penaltyAreaWidth, DEFAULT_PENALTY_WIDTH),
  );
  let centerCircleRadius = $derived(
    orDefault(
      wizard.draft.field.centerCircleRadius,
      DEFAULT_CENTER_CIRCLE_RADIUS,
    ),
  );

  const margin = 400; // mm, so the boundary itself isn't clipped
  let halfLength = $derived(length / 2 + margin);
  let halfWidth = $derived(width / 2 + margin);
  let viewBox = $derived(
    `${String(-halfLength)} ${String(-halfWidth)} ${String(halfLength * 2)} ${String(halfWidth * 2)}`,
  );

  function opacity(shown: boolean): number {
    return shown ? 1 : 0.15;
  }
</script>

<svg {viewBox} transform="scale(1,-1)" role="img" aria-label="Field preview">
  <rect
    x={-length / 2}
    y={-width / 2}
    width={length}
    height={width}
    class="boundary"
  />

  <line
    x1={-length / 2}
    y1="0"
    x2={length / 2}
    y2="0"
    class="marking"
    opacity={opacity(wizard.draft.optionalFieldLines.goal2Goal)}
  />

  <line
    x1="0"
    y1={-width / 2}
    x2="0"
    y2={width / 2}
    class="marking"
    opacity={opacity(wizard.draft.optionalFieldLines.halfway)}
  />

  <circle
    cx="0"
    cy="0"
    r={centerCircleRadius}
    fill="none"
    class="marking"
    opacity={opacity(wizard.draft.optionalFieldLines.centerCircle)}
  />

  {#each [-1, 1] as side (side)}
    <rect
      x={side > 0 ? length / 2 - penaltyDepth : -length / 2}
      y={-penaltyWidth / 2}
      width={penaltyDepth}
      height={penaltyWidth}
      fill="none"
      class="marking"
      opacity={opacity(wizard.draft.optionalFieldLines.penalty)}
    />
  {/each}

  {#if wizard.fieldLayout === "half"}
    <rect
      x="0"
      y={-width / 2}
      width={length / 2}
      height={width}
      class="half-field-mask"
    />
  {/if}
</svg>

<style>
  svg {
    width: 100%;
    max-width: 20rem;
    height: auto;
    aspect-ratio: 1;
    background: #0a5c2e;
    border-radius: 4px;
  }

  .boundary {
    fill: none;
    stroke: white;
    stroke-width: 20;
  }

  .marking {
    stroke: white;
    stroke-width: 15;
    transition: opacity 0.15s;
  }

  .half-field-mask {
    fill: rgba(0, 0, 0, 0.6);
  }
</style>

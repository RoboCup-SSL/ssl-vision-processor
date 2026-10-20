<script lang="ts">
  // Same schematic approach as WizardFieldPreview (see its comment), but here
  // the numbers themselves are the point: every dimension this step's inputs
  // edit gets a label that updates live as the box changes, so it's obvious
  // which line on a real field each input actually measures. Markings are
  // already decided by this step (it runs after WizardOptionalLines), so
  // they're drawn plain -- no ghosting here.
  import { wizard } from "../wizard.svelte";
  import WizardDimensionLine from "./WizardDimensionLine.svelte";
  import WizardRadiusDimension from "./WizardRadiusDimension.svelte";

  const DEFAULT_LENGTH = 9000;
  const DEFAULT_WIDTH = 6000;
  const DEFAULT_GOAL_WIDTH = 1000;
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
  let goalWidth = $derived(
    orDefault(wizard.draft.field.goalWidth, DEFAULT_GOAL_WIDTH),
  );
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

  // The dimension lines' base unit (arrows, weight, font size): proportional
  // to the field itself, not a fixed mm constant -- a fixed size looked right
  // against this test preset's small field and rendered as unreadable specks
  // against a real Division B field. Based on (length + width) directly
  // (not on the viewBox, which includes margin) so margin below can safely be
  // sized as a multiple of this without the two being circular.
  let dimScale = $derived((length + width) * 0.0199);

  // Separate horizontal/vertical margins, not one shared value: the
  // horizontal one has to fit the longest label ("pen. width: 2400mm") even
  // left/right anchored (its *full* width extending from the anchor point,
  // not just half of it, unlike a centered label), which needs real
  // headroom -- but sizing the vertical margin to match made the whole
  // diagram far taller than it needs to be, since the labels stacked above
  // and below the field (length's two lines, the penalty/circle tiers) only
  // ever need a few line-heights of clearance, not a label's full width.
  let marginX = $derived(dimScale * 20);
  let marginY = $derived(dimScale * 7.5);
  let halfLength = $derived(length / 2 + marginX);
  let halfWidth = $derived(width / 2 + marginY);
  let viewBox = $derived(
    `${String(-halfLength)} ${String(-halfWidth)} ${String(halfLength * 2)} ${String(halfWidth * 2)}`,
  );

  // Where penalty depth's and the center circle's labels land, once their
  // leaders carry them out past the top boundary. Penalty width shares the
  // same escape direction (straight up, from its own x) but needs a further
  // tier so its label doesn't collide with depth's.
  let nearTierY = $derived(width / 2 + dimScale * 1.6);
  let farTierY = $derived(width / 2 + dimScale * 4.2);

  // The circle's leader direction: mostly up, tilted left so it clears the
  // halfway line (which runs straight up through x=0) and lands well clear
  // of the penalty labels over on the right.
  const CIRCLE_LEADER_ANGLE_DEG = 110;
  let circleLeaderRad = $derived((CIRCLE_LEADER_ANGLE_DEG * Math.PI) / 180);
  let circleLabelDistance = $derived(nearTierY / Math.sin(circleLeaderRad));

  function round(value: number): string {
    return `${String(Math.round(value))}mm`;
  }
</script>

<svg
  {viewBox}
  transform="scale(1,-1)"
  role="img"
  aria-label="Field dimensions preview"
>
  <rect
    x={-length / 2}
    y={-width / 2}
    width={length}
    height={width}
    class="boundary"
  />

  {#if wizard.draft.optionalFieldLines.goal2Goal}
    <line x1={-length / 2} y1="0" x2={length / 2} y2="0" class="marking" />
  {/if}
  {#if wizard.draft.optionalFieldLines.halfway}
    <line x1="0" y1={-width / 2} x2="0" y2={width / 2} class="marking" />
  {/if}
  {#if wizard.draft.optionalFieldLines.centerCircle}
    <circle cx="0" cy="0" r={centerCircleRadius} fill="none" class="marking" />
  {/if}
  {#if wizard.draft.optionalFieldLines.penalty}
    {#each [-1, 1] as side (side)}
      <rect
        x={side > 0 ? length / 2 - penaltyDepth : -length / 2}
        y={-penaltyWidth / 2}
        width={penaltyDepth}
        height={penaltyWidth}
        fill="none"
        class="marking"
      />
    {/each}
  {/if}

  <!-- field length, measured close off the bottom boundary edge. Always
       spans (and is labeled as) the full field, even in half-field layout --
       dimensioning only the half while measuring the full span read as
       inconsistent, so the half reading is a second line instead of a
       replacement. labelLine nudges the text a touch further out so it
       doesn't sit right on top of the dimension line itself. -->
  <WizardDimensionLine
    orientation="horizontal"
    from={-length / 2}
    to={length / 2}
    line={-width / 2 - dimScale * 1.4}
    labelLine={-width / 2 - dimScale * 2.2}
    featureLine={-width / 2}
    text={`length: ${round(length)}\n(half: ${round(length / 2)})`}
    scale={dimScale}
  />

  <!-- field width, measured close off the left boundary edge -->
  <WizardDimensionLine
    orientation="vertical"
    from={-width / 2}
    to={width / 2}
    line={-length / 2 - dimScale * 1.4}
    featureLine={-length / 2}
    text={`width: ${round(width)}`}
    labelAnchor="right"
    scale={dimScale}
  />

  <!-- goal width, measured right off the goal line itself: already outside
       the boundary the moment it clears the goal line, so it needs no
       further leader out. -->
  <WizardDimensionLine
    orientation="vertical"
    from={-goalWidth / 2}
    to={goalWidth / 2}
    line={length / 2 + dimScale * 1.4}
    labelLine={length / 2 + dimScale * 2.2}
    featureLine={length / 2}
    text={`goal: ${round(goalWidth)}`}
    labelAnchor="left"
    scale={dimScale}
  />

  {#if wizard.draft.optionalFieldLines.penalty}
    <!-- Both the box's own dimension lines sit close to the box itself,
         inside the field, same as every other dimension here -- only their
         labels leader out further, clear of the markings drawn on the field.
         Both exit straight up past the top boundary; width uses labelAt
         (not labelLine) to do that, since labelLine only ever moves along
         *its own* dimension's perpendicular axis (x, for a vertical
         dimension), which can't reach a point above the field -- width's
         far tier keeps it clear of depth's label, which sits at a different
         x but the same near tier. -->
    <WizardDimensionLine
      orientation="horizontal"
      from={length / 2 - penaltyDepth}
      to={length / 2}
      line={penaltyWidth / 2 + dimScale * 1.4}
      featureLine={penaltyWidth / 2}
      labelLine={nearTierY}
      text={`pen. depth: ${round(penaltyDepth)}`}
      labelAnchor="left"
      scale={dimScale}
    />
    <WizardDimensionLine
      orientation="vertical"
      from={-penaltyWidth / 2}
      to={penaltyWidth / 2}
      line={length / 2 - penaltyDepth - dimScale * 1.4}
      featureLine={length / 2 - penaltyDepth}
      labelAt={[length / 2 - penaltyDepth - dimScale * 1.4, farTierY]}
      text={`pen. width: ${round(penaltyWidth)}`}
      labelAnchor="left"
      scale={dimScale}
    />
  {/if}

  {#if wizard.draft.optionalFieldLines.centerCircle}
    <WizardRadiusDimension
      cx={0}
      cy={0}
      radius={centerCircleRadius}
      angleDeg={CIRCLE_LEADER_ANGLE_DEG}
      labelDistance={circleLabelDistance}
      text={`radius: ${round(centerCircleRadius)}`}
      scale={dimScale}
    />
  {/if}
</svg>

<style>
  svg {
    width: 100%;
    height: auto;
    background: #0a5c2e;
    border-radius: 4px;
  }

  .boundary {
    fill: none;
    stroke: white;
    stroke-width: 20;
  }

  .marking {
    fill: none;
    stroke: white;
    stroke-width: 15;
  }
</style>

<script lang="ts">
  // A mechanical-drawing style dimension: extension lines running out from the
  // measured feature to a dimension line with inward-pointing arrowheads. The
  // dimension line itself stays close to the feature it measures (clearest
  // about exactly what's being measured, especially for something like the
  // penalty box that sits well inside the field). Its label doesn't have to
  // stay that close, though -- text left sitting over the field competes with
  // the markings drawn on it, so when `labelLine` differs from `line`, a
  // further plain leader carries the label out to wherever is legible (in
  // practice, always outside the field boundary).
  interface Props {
    orientation: "horizontal" | "vertical";
    // The measured span, along the measurement axis (x for horizontal, y for
    // vertical).
    from: number;
    to: number;
    // Where the dimension line itself sits, on the perpendicular axis --
    // close to the feature.
    line: number;
    // Where the real feature edge is, on the perpendicular axis, if it
    // differs from `line` -- draws extension lines out to the dimension line.
    // Omit when the dimension line sits right on the feature already (e.g.
    // goal width marked directly on the goal line).
    featureLine?: number;
    // Where the label sits, on the perpendicular axis, if it needs to be
    // further out than `line` itself (e.g. clear of the field entirely). A
    // plain leader (no arrowheads) connects the two. Omit to put the label
    // right on the dimension line, same as `line`.
    labelLine?: number;
    // Overrides where the label sits entirely, as a real [x, y] point, for
    // when the label needs to leave via the *other* axis than `labelLine`
    // can reach (e.g. a vertical dimension -- one whose own `line`/`labelLine`
    // only ever move in x -- whose label still needs to exit through the top
    // or bottom). Takes priority over `labelLine` when both are given.
    labelAt?: [number, number];
    // A "\n" in here starts a new line below the first, e.g. a full-length
    // reading with a half-length reading underneath it in parentheses.
    text: string;
    // The base unit everything else here (arrows, line weight, font size) is
    // a multiple of. Fixed mm constants looked right at the small scale
    // they were tuned against, then rendered as unreadable specks once
    // tested against a real Division B field -- the viewBox grows with the
    // field, so what's drawn on it has to scale with it too, not stay a
    // fixed absolute size. The caller derives this from its own viewBox size.
    scale: number;
    // Shifts the label (and, if present, its leader) along the dimension
    // line, away from the midpoint between `from` and `to`. Lets two
    // dimensions whose lines and arrows don't collide still avoid a
    // label-to-label collision, without either one having to move.
    labelOffset?: number;
    // How the label sits relative to its own anchor point: "center" straddles
    // it evenly (the default, right for a label with room on both sides),
    // "left"/"right" grow only rightward/leftward from that point -- useful
    // for packing a label against a boundary it can't cross, or two labels
    // past each other.
    labelAnchor?: "left" | "center" | "right";
  }

  const TEXT_ANCHOR: Record<
    NonNullable<Props["labelAnchor"]>,
    "start" | "middle" | "end"
  > = {
    left: "start",
    center: "middle",
    right: "end",
  };

  let {
    orientation,
    from,
    to,
    line,
    featureLine,
    labelLine,
    labelAt,
    text,
    scale,
    labelOffset = 0,
    labelAnchor = "center",
  }: Props = $props();

  let lines = $derived(text.split("\n"));

  let arrowLength = $derived(scale * 0.65);
  let arrowWidth = $derived(scale * 0.45);
  let overshoot = $derived(scale * 0.55); // how far extension lines run past the dimension line
  let strokeWidth = $derived(scale * 0.08);
  let fontSize = $derived(scale * 1.5);

  // Maps (position along the measurement axis, position on the perpendicular
  // axis) to real (x, y), so the rest of this component can be written once
  // instead of twice.
  function point(along: number, across: number): [number, number] {
    return orientation === "horizontal" ? [along, across] : [across, along];
  }

  function fmt(p: [number, number]): string {
    return `${String(p[0])},${String(p[1])}`;
  }

  let dir = $derived(Math.sign(to - from) || 1);

  function arrowPoints(at: number, inward: number): string {
    const base = at + inward * arrowLength;
    return [
      point(at, line),
      point(base, line - arrowWidth / 2),
      point(base, line + arrowWidth / 2),
    ]
      .map(fmt)
      .join(" ");
  }

  let extensionSign = $derived(
    featureLine === undefined ? 0 : Math.sign(line - featureLine) || 1,
  );

  let along = $derived((from + to) / 2 + labelOffset);
  let effectiveLabelLine = $derived(labelLine ?? line);
  let labelPoint = $derived(labelAt ?? point(along, effectiveLabelLine));
  let hasLeader = $derived(
    labelAt !== undefined || (labelLine !== undefined && labelLine !== line),
  );
</script>

{#if featureLine !== undefined && extensionSign !== 0}
  <line
    x1={point(from, featureLine)[0]}
    y1={point(from, featureLine)[1]}
    x2={point(from, line + extensionSign * overshoot)[0]}
    y2={point(from, line + extensionSign * overshoot)[1]}
    class="extension"
    style:stroke-width={strokeWidth}
  />
  <line
    x1={point(to, featureLine)[0]}
    y1={point(to, featureLine)[1]}
    x2={point(to, line + extensionSign * overshoot)[0]}
    y2={point(to, line + extensionSign * overshoot)[1]}
    class="extension"
    style:stroke-width={strokeWidth}
  />
{/if}

<line
  x1={point(from, line)[0]}
  y1={point(from, line)[1]}
  x2={point(to, line)[0]}
  y2={point(to, line)[1]}
  class="dim-line"
  style:stroke-width={strokeWidth}
/>
<polygon points={arrowPoints(from, dir)} class="arrow" />
<polygon points={arrowPoints(to, -dir)} class="arrow" />

{#if hasLeader}
  <line
    x1={point(along, line)[0]}
    y1={point(along, line)[1]}
    x2={labelPoint[0]}
    y2={labelPoint[1]}
    class="extension"
    style:stroke-width={strokeWidth}
  />
{/if}

<g
  transform={`translate(${String(labelPoint[0])} ${String(labelPoint[1])}) scale(1,-1)`}
>
  <text
    text-anchor={TEXT_ANCHOR[labelAnchor]}
    class="dim-text"
    style:font-size={`${String(fontSize)}px`}
    style:stroke-width={`${String(scale * 0.3)}px`}
  >
    {#each lines as line_, i (i)}
      <tspan x="0" dy={i === 0 ? 0 : fontSize * 1.15}>{line_}</tspan>
    {/each}
  </text>
</g>

<style>
  .extension,
  .dim-line {
    stroke: #111;
  }

  .arrow {
    fill: #111;
  }

  .dim-text {
    fill: #111;
    paint-order: stroke;
    stroke: white;
    stroke-linejoin: round;
  }
</style>

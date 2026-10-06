<script lang="ts">
  // A radius leader, mechanical-drawing style: a line from a point on the
  // circle's edge (arrow touching the circumference, pointing back at the
  // center) continuing straight out to wherever the label can sit clearly --
  // in practice, past the field boundary, same as WizardDimensionLine's
  // labelLine. A circle has no natural "close to the boundary" edge the way a
  // rectangle's sides do, so this is always a single diagonal leader rather
  // than an axis-aligned dimension line plus a separate leader.
  interface Props {
    cx: number;
    cy: number;
    radius: number;
    // Direction the leader points, in degrees, standard math convention
    // (0 = +x, 90 = +y).
    angleDeg: number;
    // Distance from center, along that same direction, where the label sits.
    labelDistance: number;
    text: string;
    scale: number;
    // Overrides the automatic left/center/right pick below, for a leader
    // whose default reads oddly for some other reason (crowding a
    // neighboring label, say).
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
    cx,
    cy,
    radius,
    angleDeg,
    labelDistance,
    text,
    scale,
    labelAnchor,
  }: Props = $props();

  let arrowLength = $derived(scale * 0.65);
  let arrowWidth = $derived(scale * 0.45);
  let strokeWidth = $derived(scale * 0.08);
  let fontSize = $derived(scale * 1.5);

  let rad = $derived((angleDeg * Math.PI) / 180);
  let dirX = $derived(Math.cos(rad));
  let dirY = $derived(Math.sin(rad));

  let edgePoint = $derived<[number, number]>([
    cx + dirX * radius,
    cy + dirY * radius,
  ]);
  let labelPoint = $derived<[number, number]>([
    cx + dirX * labelDistance,
    cy + dirY * labelDistance,
  ]);

  // The arrow tip sits on the circle, pointing back in toward the center.
  let arrowBase = $derived<[number, number]>([
    cx + dirX * (radius + arrowLength),
    cy + dirY * (radius + arrowLength),
  ]);
  // Perpendicular to the leader's own direction, for the arrow's two base
  // corners.
  let perpX = $derived(-dirY);
  let perpY = $derived(dirX);
  let arrowPoints = $derived(
    [
      edgePoint,
      [
        arrowBase[0] + perpX * (arrowWidth / 2),
        arrowBase[1] + perpY * (arrowWidth / 2),
      ] as [number, number],
      [
        arrowBase[0] - perpX * (arrowWidth / 2),
        arrowBase[1] - perpY * (arrowWidth / 2),
      ] as [number, number],
    ]
      .map((p) => `${String(p[0])},${String(p[1])}`)
      .join(" "),
  );

  // A label whose leader points leftward reads better growing further left
  // (ending at the point, not straddling it); one pointing rightward reads
  // better growing rightward. Only matters near the horizontal extremes --
  // dirX close to 0 (a mostly-vertical leader) keeps the default centering.
  // `labelAnchor` above overrides this entirely when given.
  let effectiveAnchor = $derived(
    TEXT_ANCHOR[
      labelAnchor ?? (dirX > 0.3 ? "left" : dirX < -0.3 ? "right" : "center")
    ],
  );
</script>

<line
  x1={edgePoint[0]}
  y1={edgePoint[1]}
  x2={labelPoint[0]}
  y2={labelPoint[1]}
  class="leader"
  style:stroke-width={strokeWidth}
/>
<polygon points={arrowPoints} class="arrow" />

<g
  transform={`translate(${String(labelPoint[0])} ${String(labelPoint[1])}) scale(1,-1)`}
>
  <text
    text-anchor={effectiveAnchor}
    class="dim-text"
    style:font-size={`${String(fontSize)}px`}
    style:stroke-width={`${String(scale * 0.3)}px`}
  >
    {text}
  </text>
</g>

<style>
  .leader {
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

<script lang="ts">
  import type {
    SSL_FieldLineSegmentJson,
    SSL_FieldCircularArcJson,
  } from "../proto/vision/ssl_vision_geometry_pb";
  import { boxesOverlap, type FieldSlice } from "./fieldSplit";

  interface Props {
    fieldLength: number;
    fieldWidth: number;
    lines: SSL_FieldLineSegmentJson[];
    arcs: SSL_FieldCircularArcJson[];
    // When set, markings entirely outside this camera's slice of the field
    // are dimmed and the slice itself is outlined -- so "I picked 2 cameras"
    // visibly shows you get one penalty box, not both. Omit to show every
    // marking at full brightness (the whole-field view).
    slice?: FieldSlice;
  }

  let { fieldLength, fieldWidth, lines, arcs, slice }: Props = $props();

  // Field coordinates: origin at center, +x toward one goal, +y toward one
  // touchline (see gui/CLAUDE.md / internal/geometry/field.go). SVG is
  // y-down, so the viewBox flips y to draw with +y pointing up, matching the
  // physical convention instead of mirroring it.
  const margin = 300; // mm, just so lines at the boundary aren't clipped

  let halfLength = $derived((fieldLength || 1) / 2 + margin);
  let halfWidth = $derived((fieldWidth || 1) / 2 + margin);
  let viewBox = $derived(
    `${String(-halfLength)} ${String(-halfWidth)} ${String(halfLength * 2)} ${String(halfWidth * 2)}`,
  );

  // protobuf-es's JSON type for float/double fields allows "NaN"/"Infinity"/
  // "-Infinity" alongside plain numbers (the protobuf JSON spec's encoding
  // for values a JSON number can't represent). Number() maps all three
  // string forms to their real values, same as any ordinary numeric string.
  function num(
    value: number | "NaN" | "Infinity" | "-Infinity" | undefined,
  ): number {
    return Number(value ?? 0);
  }

  function lineInSlice(line: SSL_FieldLineSegmentJson): boolean {
    if (!slice) return true;

    const x1 = num(line.p1?.x);
    const y1 = num(line.p1?.y);
    const x2 = num(line.p2?.x);
    const y2 = num(line.p2?.y);

    return boxesOverlap(slice, {
      minX: Math.min(x1, x2),
      minY: Math.min(y1, y2),
      maxX: Math.max(x1, x2),
      maxY: Math.max(y1, y2),
    });
  }

  function arcInSlice(arc: SSL_FieldCircularArcJson): boolean {
    if (!slice) return true;

    const cx = num(arc.center?.x);
    const cy = num(arc.center?.y);
    const r = num(arc.radius);

    return boxesOverlap(slice, {
      minX: cx - r,
      minY: cy - r,
      maxX: cx + r,
      maxY: cy + r,
    });
  }
</script>

<svg {viewBox} transform="scale(1,-1)" role="img" aria-label="Field sketch">
  {#if slice}
    <rect
      x={slice.minX}
      y={slice.minY}
      width={slice.maxX - slice.minX}
      height={slice.maxY - slice.minY}
      class="slice-outline"
    />
  {/if}

  {#each lines as line (line.name)}
    <line
      x1={line.p1?.x ?? 0}
      y1={line.p1?.y ?? 0}
      x2={line.p2?.x ?? 0}
      y2={line.p2?.y ?? 0}
      stroke="white"
      stroke-width={line.thickness ?? 10}
      class:dimmed={!lineInSlice(line)}
    />
  {/each}

  {#each arcs as arc (arc.name)}
    <circle
      cx={arc.center?.x ?? 0}
      cy={arc.center?.y ?? 0}
      r={arc.radius ?? 0}
      fill="none"
      stroke="white"
      stroke-width={arc.thickness ?? 10}
      class:dimmed={!arcInSlice(arc)}
    />
  {/each}
</svg>

<style>
  svg {
    width: 100%;
    height: auto;
    aspect-ratio: 1;
    background: #0a5c2e;
    border-radius: 4px;
  }

  .dimmed {
    opacity: 0.25;
  }

  .slice-outline {
    fill: rgba(255, 255, 255, 0.08);
    stroke: orange;
    stroke-width: 20;
    stroke-dasharray: 60 30;
  }
</style>

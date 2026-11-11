// Geometry for the corner picker's four line corners (CornerPicker.svelte).
// Only the first one matters to vision_processor (see the note there), but
// the picker draws them as a polygon in list order, so the other three must
// go around the quadrilateral, not cross it. It also draws the goals and
// penalty areas through them, as vision_processor would read them.

import type { FieldSlice } from "./fieldSplit";
import type { FieldConfig } from "./apiTypes";

export interface Point {
  x: number;
  y: number;
}

// The points rotated so that index comes first, keeping their order around
// the polygon.
export function rotateTo<T>(points: T[], index: number): T[] {
  return [...points.slice(index), ...points.slice(0, index)];
}

// The points in order around their centroid, the first one kept first. A
// seed saved crossed (the picker once moved the origin to the front without
// rotating) draws as a quadrilateral again.
export function aroundFirst(points: Point[]): Point[] {
  const first = points[0];
  if (!first) return [];

  const cx = points.reduce((sum, p) => sum + p.x, 0) / points.length;
  const cy = points.reduce((sum, p) => sum + p.y, 0) / points.length;
  const angle = (p: Point): number => Math.atan2(p.y - cy, p.x - cx);

  const sorted = [...points].sort((a, b) => angle(a) - angle(b));

  return rotateTo(sorted, sorted.indexOf(first));
}

// The corners in the order vision_processor matches them to its region's
// field corners (minX,minY), (minX,maxY), (maxX,maxY), (maxX,minY): the first
// one first, the rest clockwise on screen (GeomModel.cpp's cornerCalibration,
// isClockwiseConvexQuadrilateral and PointGeometryFit).
export function clockwiseFromFirst(points: Point[]): Point[] {
  const [a, b, c, d] = points;
  if (!a || !b || !c || !d) return points;

  // Shoelace in image pixels: y points down, so positive is clockwise on
  // screen.
  const edges: Segment[] = [
    [a, b],
    [b, c],
    [c, d],
    [d, a],
  ];
  const area = edges.reduce((sum, [p, q]) => sum + p.x * q.y - q.x * p.y, 0);

  return area >= 0 ? points : [a, d, c, b];
}

// The projective map taking the four from points to the four to points, or
// undefined when they're degenerate (three in a line, say).
export function homography(
  from: Point[],
  to: Point[],
): ((p: Point) => Point | undefined) | undefined {
  if (from.length !== 4 || to.length !== 4) return undefined;

  // u = (h0 x + h1 y + h2) / (h6 x + h7 y + 1), v likewise with h3..h5:
  // two linear equations per pair, eight unknowns.
  const rows = from.flatMap(({ x, y }, i) => {
    const { x: u, y: v } = to[i] ?? { x: 0, y: 0 };

    return [
      [x, y, 1, 0, 0, 0, -u * x, -u * y, u],
      [0, 0, 0, x, y, 1, -v * x, -v * y, v],
    ];
  });

  const h = solve(rows);
  if (!h) return undefined;

  const [h0 = 0, h1 = 0, h2 = 0, h3 = 0, h4 = 0, h5 = 0, h6 = 0, h7 = 0] = h;

  return (p) => {
    const w = h6 * p.x + h7 * p.y + 1;
    // Behind the camera's horizon: not on screen.
    if (w <= 1e-9) return undefined;

    return {
      x: (h0 * p.x + h1 * p.y + h2) / w,
      y: (h3 * p.x + h4 * p.y + h5) / w,
    };
  };
}

// Gauss-Jordan elimination with partial pivoting on an augmented matrix;
// undefined when it's singular.
function solve(rows: number[][]): number[] | undefined {
  const n = rows.length;
  const m = rows.map((r) => [...r]);
  const at = (r: number, c: number): number => m[r]?.[c] ?? 0;

  for (let col = 0; col < n; col++) {
    let pivot = col;
    for (let r = col + 1; r < n; r++) {
      if (Math.abs(at(r, col)) > Math.abs(at(pivot, col))) pivot = r;
    }

    if (Math.abs(at(pivot, col)) < 1e-12) return undefined;
    const top = m[col];
    const best = m[pivot];
    if (top && best) {
      m[col] = best;
      m[pivot] = top;
    }

    const row = m[col] ?? [];
    const lead = at(col, col);
    for (const [r, other] of m.entries()) {
      if (r === col) continue;

      const factor = (other[col] ?? 0) / lead;
      for (let c = col; c <= n; c++) {
        other[c] = (other[c] ?? 0) - factor * (row[c] ?? 0);
      }
    }
  }

  return m.map((r, i) => (r[n] ?? 0) / (r[i] ?? 1));
}

export type Segment = [Point, Point];

// The goals and penalty areas in field coordinates (mm), as the parts of
// them in or beside slice: a whole goal for a region with one at the middle
// of a side, half of one for a region at a corner. Goals lie outside the
// field lines, so a region on the field's edge keeps what's beyond that edge.
export function goalMarkings(field: FieldConfig, slice: FieldSlice): Segment[] {
  const halfLength = field.fieldLength / 2;
  const halfWidth = field.fieldWidth / 2;
  const reach = Math.max(field.goalDepth, 0) * 2 + 1;
  const edge = (a: number, b: number): boolean => Math.abs(a - b) < 1;

  const clip: FieldSlice = {
    minX: slice.minX - (edge(slice.minX, -halfLength) ? reach : 0),
    maxX: slice.maxX + (edge(slice.maxX, halfLength) ? reach : 0),
    minY: slice.minY - (edge(slice.minY, -halfWidth) ? reach : 0),
    maxY: slice.maxY + (edge(slice.maxY, halfWidth) ? reach : 0),
  };

  const out: Segment[] = [];

  for (const side of [-1, 1]) {
    const goalLine = side * halfLength;
    const boxes = [
      // Penalty area: into the field from the goal line.
      { depth: -side * field.penaltyAreaDepth, width: field.penaltyAreaWidth },
      // Goal: out of the field behind the goal line.
      { depth: side * field.goalDepth, width: field.goalWidth },
    ];

    for (const { depth, width } of boxes) {
      if (depth === 0 || width <= 0) continue;

      // Open on the goal line, which the picker's quadrilateral already is.
      const y = width / 2;
      const nearLeft = { x: goalLine, y: -y };
      const farLeft = { x: goalLine + depth, y: -y };
      const farRight = { x: goalLine + depth, y };
      const nearRight = { x: goalLine, y };
      const sides: Segment[] = [
        [nearLeft, farLeft],
        [farLeft, farRight],
        [farRight, nearRight],
      ];

      for (const [a, b] of sides) {
        const segment = clipSegment(a, b, clip);
        if (segment) out.push(segment);
      }
    }
  }

  return out;
}

// Liang-Barsky: the part of segment a-b inside box, if any.
function clipSegment(a: Point, b: Point, box: FieldSlice): Segment | undefined {
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  let t0 = 0;
  let t1 = 1;

  const edges: [number, number][] = [
    [-dx, a.x - box.minX],
    [dx, box.maxX - a.x],
    [-dy, a.y - box.minY],
    [dy, box.maxY - a.y],
  ];

  for (const [p, q] of edges) {
    if (p === 0) {
      if (q < 0) return undefined;
      continue;
    }

    const t = q / p;
    if (p < 0) t0 = Math.max(t0, t);
    else t1 = Math.min(t1, t);

    if (t0 > t1) return undefined;
  }

  if (t1 - t0 < 1e-9) return undefined;

  return [
    { x: a.x + t0 * dx, y: a.y + t0 * dy },
    { x: a.x + t1 * dx, y: a.y + t1 * dy },
  ];
}

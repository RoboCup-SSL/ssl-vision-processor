// The league's standard robot pattern, as vision_processor detects it
// (src/pattern.h): a team-colored center dot and four ID dots, green or pink.

// Robot frame, mm: +x along the robot's orientation, +y to its left.
export const CENTER_DOT_RADIUS = 25;
export const ID_DOT_RADIUS = 20;

// The four ID dots, in increasing angle from the robot's orientation:
// front left, back left, back right, front right.
export const ID_DOT_POSITIONS: readonly (readonly [number, number])[] = [
  [35, 54.772],
  [-54.772, 35],
  [-54.772, -35],
  [35, -54.772],
];

// One entry per robot ID: bit 3 is the first dot above, bit 0 the last;
// 1 is green, 0 pink. Copied from src/pattern.h's patterns.
const PATTERNS = [
  0b0100, 0b1100, 0b1101, 0b0101, 0b0010, 0b1010, 0b1011, 0b0011, 0b1111,
  0b0000, 0b0110, 0b1001, 0b1110, 0b1000, 0b0111, 0b0001,
];

export type DotColor = "green" | "pink";

// The ID dots' colors for id, in ID_DOT_POSITIONS order, or null for an id
// the standard pattern has no entry for.
export function idDots(id: number | null): DotColor[] | null {
  const bits = id === null ? undefined : PATTERNS[id];
  if (bits === undefined) return null;

  return [3, 2, 1, 0].map((bit) => ((bits >> bit) & 1 ? "green" : "pink"));
}

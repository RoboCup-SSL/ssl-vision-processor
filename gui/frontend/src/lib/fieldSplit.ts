// Mirrors src/CameraModel.cpp's visibleFieldExtentEstimation (the boundary-
// free variant, matching PointGeometryFit's modelCorners in GeomModel.cpp) --
// which portion of the field one camera in a camAmount-way split is
// responsible for. Reimplemented here rather than fetched from the Go host
// because it's pure geometry with no state, and the UI needs it live as the
// user changes camera count/id, not round-tripped through an API call.
// Which camera counts are valid for each field layout: a full field supports
// 1/2/4 cameras, a half field only 1/2. Shared between FieldEditor's layout
// section and the setup wizard's equivalent step.
export const CAMERA_COUNT_OPTIONS: Record<"full" | "half", number[]> = {
  full: [1, 2, 4],
  half: [1, 2],
};

export interface FieldSlice {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
}

export function computeFieldSlice(
  camId: number,
  camAmount: number,
  fieldLength: number,
  fieldWidth: number,
): FieldSlice {
  const amount = Math.max(1, camAmount);

  // Always halves whichever dimension is currently longer -- for a
  // conventional SSL field (length > width) this means camAmount=2 splits
  // along length first, camAmount=4 then also splits width, matching
  // "Double camera field: -x/+x side" and "Quadruple: four quadrants" in
  // SSL_VPConfig's own comment.
  let sizeX = 1;
  let sizeY = 1;
  for (let i = amount; i > 1; i = Math.floor(i / 2)) {
    if (fieldLength / sizeX >= fieldWidth / sizeY) {
      sizeX *= 2;
    } else {
      sizeY *= 2;
    }
  }

  let posX = 0;
  let posY = 0;
  for (let i = camId % amount; i > 0; i--) {
    posY++;
    if (posY === sizeY) {
      posY = 0;
      posX++;
    }
  }

  const extentX = fieldLength / sizeX;
  const extentY = fieldWidth / sizeY;
  const minX = extentX * posX - fieldLength / 2;
  const minY = extentY * posY - fieldWidth / 2;

  return { minX, minY, maxX: minX + extentX, maxY: minY + extentY };
}

// Whether two axis-aligned boxes overlap at all -- used to decide if a field
// marking is at least partially within a camera's slice, not just whether
// it's fully contained (a marking straddling the boundary is still relevant).
export function boxesOverlap(a: FieldSlice, b: FieldSlice): boolean {
  return (
    a.minX <= b.maxX && a.maxX >= b.minX && a.minY <= b.maxY && a.maxY >= b.minY
  );
}

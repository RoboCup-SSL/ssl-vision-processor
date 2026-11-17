// Detection and tracked frames reduced to what the Live Field draws, in one
// shape and one unit (mm, mm/s, radians), so drawing doesn't care where a
// robot came from. Detections are already in mm; tracked frames are in m.
import type {
  SSL_DetectionFrameJson,
  SSL_DetectionRobotJson,
} from "../../proto/vision/ssl_vision_detection_pb";
import type { TrackedFrameJson } from "../../proto/vision/ssl_vision_detection_tracked_pb";
import type {
  Vector2Json,
  Vector3Json,
} from "../../proto/gamecontroller/ssl_gc_geometry_pb";

export type Team = "yellow" | "blue";

export interface Robot {
  team: Team;
  // null when the source doesn't know which robot it is.
  id: number | null;
  x: number;
  y: number;
  // null when the source has no orientation for it.
  orientation: number | null;
  // Detection confidence or tracker visibility, 0 to 1, when given.
  confidence: number | null;
  vx: number | null;
  vy: number | null;
}

export interface Ball {
  x: number;
  y: number;
  z: number | null;
  confidence: number | null;
  vx: number | null;
  vy: number | null;
}

// A kicked ball's path, from where it was kicked to where the tracker
// expects it to stop.
export interface Kick {
  x: number;
  y: number;
  stopX: number | null;
  stopY: number | null;
}

export interface Layer {
  robots: Robot[];
  balls: Ball[];
  kick: Kick | null;
}

export const EMPTY_LAYER: Layer = { robots: [], balls: [], kick: null };

type JsonNumber = number | "NaN" | "Infinity" | "-Infinity" | undefined;

// protojson writes non-finite floats as strings; anything not a finite
// number is treated as missing.
function num(value: JsonNumber): number | null {
  const n = Number(value ?? NaN);

  return Number.isFinite(n) ? n : null;
}

const M_TO_MM = 1000;

function mm(value: JsonNumber): number | null {
  const n = num(value);

  return n === null ? null : n * M_TO_MM;
}

function detectedRobot(team: Team, r: SSL_DetectionRobotJson): Robot | null {
  const x = num(r.x);
  const y = num(r.y);
  if (x === null || y === null) return null;

  return {
    team,
    id: r.robotId ?? null,
    x,
    y,
    orientation: num(r.orientation),
    confidence: num(r.confidence),
    vx: null,
    vy: null,
  };
}

export function fromDetection(frame: SSL_DetectionFrameJson): Layer {
  const robots = [
    ...(frame.robotsYellow ?? []).map((r) => detectedRobot("yellow", r)),
    ...(frame.robotsBlue ?? []).map((r) => detectedRobot("blue", r)),
  ].filter((r): r is Robot => r !== null);

  const balls = (frame.balls ?? []).flatMap((b): Ball[] => {
    const x = num(b.x);
    const y = num(b.y);
    if (x === null || y === null) return [];

    return [
      { x, y, z: num(b.z), confidence: num(b.confidence), vx: null, vy: null },
    ];
  });

  return { robots, balls, kick: null };
}

function point(
  v: Vector2Json | Vector3Json | undefined,
): [number, number] | null {
  const x = mm(v?.x);
  const y = mm(v?.y);

  return x === null || y === null ? null : [x, y];
}

export function fromTracked(frame: TrackedFrameJson): Layer {
  const robots = (frame.robots ?? []).flatMap((r): Robot[] => {
    const pos = point(r.pos);
    const team = r.robotId?.team;
    if (!pos || (team !== "YELLOW" && team !== "BLUE")) return [];

    const vel = point(r.vel);

    return [
      {
        team: team === "YELLOW" ? "yellow" : "blue",
        id: r.robotId?.id ?? null,
        x: pos[0],
        y: pos[1],
        orientation: num(r.orientation),
        confidence: num(r.visibility),
        vx: vel?.[0] ?? null,
        vy: vel?.[1] ?? null,
      },
    ];
  });

  const balls = (frame.balls ?? []).flatMap((b): Ball[] => {
    const pos = point(b.pos);
    if (!pos) return [];

    const vel = point(b.vel);

    return [
      {
        x: pos[0],
        y: pos[1],
        z: mm(b.pos?.z),
        confidence: num(b.visibility),
        vx: vel?.[0] ?? null,
        vy: vel?.[1] ?? null,
      },
    ];
  });

  const kicked = frame.kickedBall;
  const from = point(kicked?.pos);
  const stop = point(kicked?.stopPos);

  return {
    robots,
    balls,
    kick: from
      ? {
          x: from[0],
          y: from[1],
          stopX: stop?.[0] ?? null,
          stopY: stop?.[1] ?? null,
        }
      : null,
  };
}

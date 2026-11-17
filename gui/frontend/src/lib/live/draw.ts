// Canvas drawing for the Live Field. Field and robots are drawn in field
// coordinates (mm, origin at the center, +y up) through worldTransform;
// text and confidence bars in screen pixels, so they stay upright when the
// field is flipped. Colors follow the legacy ssl-vision graphicalClient.
import type { SSL_GeometryFieldSizeJson } from "../../proto/vision/ssl_vision_geometry_pb";
import type { Ball, Kick, Layer, Robot, Team } from "./model";
import {
  CENTER_DOT_RADIUS,
  ID_DOT_POSITIONS,
  ID_DOT_RADIUS,
  idDots,
} from "./patterns";
import type { RobotStyle } from "./liveView.svelte";

export const PALETTE = {
  field: "#009119",
  lines: "#ffffff",
  goal: "#d0d0d0",
  team: { yellow: "#fff33e", blue: "#417eff" },
  axis: { x: "#ff5252", y: "#2fe3ff" },
  teamDark: { yellow: "#cc9d00", blue: "#123ba0" },
  shell: "#202020",
  dot: { green: "#40e040", pink: "#ff5ad2" },
  text: "#000000",
  ball: "#ff8100",
  ballDark: "#de5900",
  velocity: "#ffffff",
};

// Outline colors that tell overlaid sources (and, optionally, cameras)
// apart. Picked to stand out against the field and both team colors.
export const TINTS = [
  "#ffffff",
  "#ff4fd8",
  "#2fe3ff",
  "#ff9f1a",
  "#b388ff",
  "#00ffa3",
  "#ff5252",
  "#c6ff00",
];

export function tint(index: number): string {
  return TINTS[index % TINTS.length] ?? "#ffffff";
}

const ROBOT_RADIUS = 90;
// Half the angle of the flat front, as in the legacy client.
const FRONT_HALF_ANGLE = (40 * Math.PI) / 180;
const BALL_RADIUS = 21;
// Smallest radius on screen, px, so a ball stays visible on a small canvas.
const MIN_BALL_PX = 3;
// Outlines stay near opaque on a faint overlay, so its color still says
// which source it is.
const OUTLINE_MIN_ALPHA = 0.85;
// A velocity is drawn as where it would carry the object in this long, s.
const VELOCITY_HORIZON = 0.5;

export interface FieldExtent {
  // Half the drawn area, mm, field plus boundary.
  halfX: number;
  halfY: number;
}

export function extentOf(field: SSL_GeometryFieldSizeJson): FieldExtent {
  const length = field.fieldLength ?? 0;
  const width = field.fieldWidth ?? 0;
  const boundary = field.boundaryWidth ?? 0;
  const goalLine =
    (field.boundaryWidthGoalLine ?? 0) > 0
      ? (field.boundaryWidthGoalLine ?? 0)
      : boundary;

  return {
    halfX: Math.max(1, length / 2 + Math.max(goalLine, field.goalDepth ?? 0)),
    halfY: Math.max(1, width / 2 + boundary),
  };
}

export interface View {
  // CSS pixels.
  width: number;
  height: number;
  dpr: number;
  // CSS pixels per mm.
  scale: number;
  flipped: boolean;
}

export function viewFor(
  extent: FieldExtent,
  width: number,
  height: number,
  dpr: number,
  flipped: boolean,
): View {
  const scale = Math.min(
    width / (2 * extent.halfX),
    height / (2 * extent.halfY),
  );

  return { width, height, dpr, scale, flipped };
}

// Field mm to CSS pixels.
export function toScreen(view: View, x: number, y: number): [number, number] {
  const f = view.flipped ? -1 : 1;

  return [
    view.width / 2 + f * x * view.scale,
    view.height / 2 - f * y * view.scale,
  ];
}

function screenTransform(ctx: CanvasRenderingContext2D, view: View): void {
  ctx.setTransform(view.dpr, 0, 0, view.dpr, 0, 0);
}

function worldTransform(ctx: CanvasRenderingContext2D, view: View): void {
  screenTransform(ctx, view);
  ctx.translate(view.width / 2, view.height / 2);
  ctx.scale(view.scale, -view.scale);
  if (view.flipped) ctx.rotate(Math.PI);
}

function num(value: number | string | undefined): number {
  const n = Number(value ?? 0);

  return Number.isFinite(n) ? n : 0;
}

// The goal colors at -x and +x: each the color of the team defending it, or
// plain when the game controller doesn't say which.
export function goalColors(
  blueOnPositive: boolean | undefined,
): [string, string] {
  if (blueOnPositive === undefined) return [PALETTE.goal, PALETTE.goal];

  const { yellow, blue } = PALETTE.team;

  return blueOnPositive ? [yellow, blue] : [blue, yellow];
}

export interface FieldOptions {
  // Which half blue defends, for the goal colors; unknown when undefined.
  blueOnPositive?: boolean;
  // Marks the origin and the +x and +y directions.
  axes?: boolean;
}

export function drawField(
  ctx: CanvasRenderingContext2D,
  view: View,
  field: SSL_GeometryFieldSizeJson,
  { blueOnPositive, axes = false }: FieldOptions = {},
): void {
  screenTransform(ctx, view);
  ctx.clearRect(0, 0, view.width, view.height);
  ctx.fillStyle = PALETTE.field;
  ctx.fillRect(0, 0, view.width, view.height);

  worldTransform(ctx, view);
  ctx.strokeStyle = PALETTE.lines;
  ctx.lineCap = "butt";

  for (const line of field.fieldLines ?? []) {
    ctx.lineWidth = Math.max(num(line.thickness), 1 / view.scale);
    ctx.beginPath();
    ctx.moveTo(num(line.p1?.x), num(line.p1?.y));
    ctx.lineTo(num(line.p2?.x), num(line.p2?.y));
    ctx.stroke();
  }

  for (const arc of field.fieldArcs ?? []) {
    ctx.lineWidth = Math.max(num(arc.thickness), 1 / view.scale);
    ctx.beginPath();
    ctx.arc(
      num(arc.center?.x),
      num(arc.center?.y),
      num(arc.radius),
      num(arc.a1),
      num(arc.a2),
    );
    ctx.stroke();
  }

  // Goals: not field markings, so not in the lines; drawn from the sizes.
  const halfLength = (field.fieldLength ?? 0) / 2;
  const halfGoal = (field.goalWidth ?? 0) / 2;
  const depth = field.goalDepth ?? 0;

  const [negative, positive] = goalColors(blueOnPositive);
  ctx.lineWidth = 20;

  for (const side of [-1, 1]) {
    ctx.strokeStyle = side < 0 ? negative : positive;
    ctx.beginPath();
    ctx.moveTo(side * halfLength, halfGoal);
    ctx.lineTo(side * (halfLength + depth), halfGoal);
    ctx.lineTo(side * (halfLength + depth), -halfGoal);
    ctx.lineTo(side * halfLength, -halfGoal);
    ctx.stroke();
  }

  if (axes) drawAxes(ctx, view);
}

// Length of each axis arrow, mm.
const AXIS_LENGTH = 500;

// The origin and an arrow along +x and +y, drawn on screen from toScreen so
// they follow a swapped view while their labels stay upright.
function drawAxes(ctx: CanvasRenderingContext2D, view: View): void {
  screenTransform(ctx, view);

  const [ox, oy] = toScreen(view, 0, 0);
  const head = 14;
  const font = 20;

  ctx.lineWidth = 5;
  ctx.font = `bold ${String(font)}px sans-serif`;
  ctx.textBaseline = "middle";

  const arrows: [string, string, number, number][] = [
    ["+x", PALETTE.axis.x, AXIS_LENGTH, 0],
    ["+y", PALETTE.axis.y, 0, AXIS_LENGTH],
  ];

  for (const [label, color, x, y] of arrows) {
    const [tx, ty] = toScreen(view, x, y);
    const angle = Math.atan2(ty - oy, tx - ox);

    ctx.strokeStyle = color;
    ctx.fillStyle = color;

    // The shaft stops inside the head, so its square end doesn't show past
    // the head's sides.
    ctx.beginPath();
    ctx.moveTo(ox, oy);
    ctx.lineTo(
      tx - head * 0.7 * Math.cos(angle),
      ty - head * 0.7 * Math.sin(angle),
    );
    ctx.stroke();

    ctx.beginPath();
    ctx.moveTo(tx, ty);
    ctx.lineTo(
      tx - head * Math.cos(angle - Math.PI / 6),
      ty - head * Math.sin(angle - Math.PI / 6),
    );
    ctx.lineTo(
      tx - head * Math.cos(angle + Math.PI / 6),
      ty - head * Math.sin(angle + Math.PI / 6),
    );
    ctx.closePath();
    ctx.fill();

    // Past the arrowhead, along the axis.
    ctx.textAlign = "center";
    ctx.fillText(
      label,
      tx + (head + font) * Math.cos(angle),
      ty + (head + font) * Math.sin(angle),
    );
  }

  ctx.fillStyle = PALETTE.lines;
  ctx.beginPath();
  ctx.arc(ox, oy, 6, 0, 2 * Math.PI);
  ctx.fill();
}

export interface LayerStyle {
  robotStyle: RobotStyle;
  // 0 to 1.
  alpha: number;
  // An outline identifying the source or camera, or null for none.
  outline: string | null;
  velocities: boolean;
  // Confidence bars, as the legacy client draws for raw detections.
  confidence: boolean;
}

// One layer as drawn, in order: the last is on top.
export interface DrawnLayer {
  layer: Layer;
  style: LayerStyle;
}

export function clear(ctx: CanvasRenderingContext2D, view: View): void {
  screenTransform(ctx, view);
  ctx.clearRect(0, 0, view.width, view.height);
}

export function drawLayer(
  ctx: CanvasRenderingContext2D,
  view: View,
  layer: Layer,
  style: LayerStyle,
): void {
  ctx.save();
  ctx.globalAlpha = style.alpha;

  if (layer.kick) drawKick(ctx, view, layer.kick);

  for (const robot of layer.robots) drawRobot(ctx, view, robot, style);

  for (const ball of layer.balls) drawBall(ctx, view, ball, style);

  ctx.restore();
}

// The robot's outline in its own frame: a circle with the front cut flat,
// or a full circle without an orientation.
function bodyPath(
  ctx: CanvasRenderingContext2D,
  flat: boolean,
  r: number,
): void {
  ctx.beginPath();

  if (flat) {
    ctx.arc(0, 0, r, FRONT_HALF_ANGLE, 2 * Math.PI - FRONT_HALF_ANGLE);
    ctx.closePath();
  } else {
    ctx.arc(0, 0, r, 0, 2 * Math.PI);
  }
}

function drawRobot(
  ctx: CanvasRenderingContext2D,
  view: View,
  robot: Robot,
  style: LayerStyle,
): void {
  const oriented = robot.orientation !== null;

  if (style.velocities) {
    drawVelocity(ctx, view, robot.x, robot.y, robot.vx, robot.vy);
  }

  worldTransform(ctx, view);
  ctx.translate(robot.x, robot.y);
  ctx.rotate(robot.orientation ?? 0);

  if (style.outline) {
    bodyPath(ctx, oriented, ROBOT_RADIUS + 12);
    strokeOutline(ctx, style, 16);
  }

  if (style.robotStyle === "pattern") {
    drawPattern(ctx, robot, oriented);
  } else {
    bodyPath(ctx, oriented, ROBOT_RADIUS);
    ctx.fillStyle = PALETTE.team[robot.team];
    ctx.fill();
    bodyPath(ctx, oriented, ROBOT_RADIUS - 5);
    ctx.lineWidth = 10;
    ctx.strokeStyle = PALETTE.teamDark[robot.team];
    ctx.stroke();
  }

  const [sx, sy] = toScreen(view, robot.x, robot.y);
  screenTransform(ctx, view);

  // The pattern style labels only robots its dots can't show.
  const pattern = style.robotStyle === "pattern";
  if (!pattern || !oriented || idDots(robot.id) === null) {
    drawId(ctx, view, sx, sy, robot, pattern);
  }

  if (style.confidence && robot.confidence !== null) {
    drawConfidence(ctx, view, sx, sy, robot.team, robot.confidence);
  }
}

function strokeOutline(
  ctx: CanvasRenderingContext2D,
  style: LayerStyle,
  width: number,
): void {
  if (!style.outline) return;

  const alpha = ctx.globalAlpha;
  ctx.globalAlpha = Math.max(alpha, OUTLINE_MIN_ALPHA);
  ctx.lineWidth = width;
  ctx.strokeStyle = style.outline;
  ctx.stroke();
  ctx.globalAlpha = alpha;
}

function drawPattern(
  ctx: CanvasRenderingContext2D,
  robot: Robot,
  oriented: boolean,
): void {
  bodyPath(ctx, oriented, ROBOT_RADIUS);
  ctx.fillStyle = PALETTE.shell;
  ctx.fill();

  ctx.fillStyle = PALETTE.team[robot.team];
  ctx.beginPath();
  ctx.arc(0, 0, CENTER_DOT_RADIUS, 0, 2 * Math.PI);
  ctx.fill();

  // Without an orientation the dots' places are unknown.
  const dots = oriented ? idDots(robot.id) : null;
  if (!dots) return;

  dots.forEach((color, i) => {
    const [x, y] = ID_DOT_POSITIONS[i] ?? [0, 0];
    ctx.fillStyle = PALETTE.dot[color];
    ctx.beginPath();
    ctx.arc(x, y, ID_DOT_RADIUS, 0, 2 * Math.PI);
    ctx.fill();
  });
}

function drawId(
  ctx: CanvasRenderingContext2D,
  view: View,
  sx: number,
  sy: number,
  robot: Robot,
  onShell: boolean,
): void {
  const label = robot.id === null ? "?" : robot.id.toString(16).toUpperCase();

  ctx.font = `bold ${String(Math.max(8, 110 * view.scale))}px sans-serif`;
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.fillStyle = onShell ? PALETTE.lines : PALETTE.text;
  ctx.fillText(label, sx, sy);
}

function drawConfidence(
  ctx: CanvasRenderingContext2D,
  view: View,
  sx: number,
  sy: number,
  team: Team,
  confidence: number,
): void {
  const width = 180 * view.scale;
  const height = Math.max(2, 30 * view.scale);
  const top = sy - (ROBOT_RADIUS + 40) * view.scale - height;
  const left = sx - width / 2;

  ctx.fillStyle = PALETTE.team[team];
  ctx.fillRect(left, top, width * Math.min(1, Math.max(0, confidence)), height);
  ctx.strokeStyle = PALETTE.teamDark[team];
  ctx.lineWidth = 1;
  ctx.strokeRect(left, top, width, height);
}

function drawBall(
  ctx: CanvasRenderingContext2D,
  view: View,
  ball: Ball,
  style: LayerStyle,
): void {
  if (style.velocities)
    drawVelocity(ctx, view, ball.x, ball.y, ball.vx, ball.vy);

  worldTransform(ctx, view);
  const r = Math.max(BALL_RADIUS, MIN_BALL_PX / view.scale);

  if (style.outline) {
    ctx.beginPath();
    ctx.arc(ball.x, ball.y, r * 1.4, 0, 2 * Math.PI);
    strokeOutline(ctx, style, r * 0.5);
  }

  ctx.fillStyle = PALETTE.ballDark;
  ctx.beginPath();
  ctx.arc(ball.x, ball.y, r, 0, 2 * Math.PI);
  ctx.fill();
  ctx.fillStyle = PALETTE.ball;
  ctx.beginPath();
  ctx.arc(ball.x, ball.y, r * 0.75, 0, 2 * Math.PI);
  ctx.fill();
}

function drawVelocity(
  ctx: CanvasRenderingContext2D,
  view: View,
  x: number,
  y: number,
  vx: number | null,
  vy: number | null,
): void {
  if (vx === null || vy === null || (vx === 0 && vy === 0)) return;

  screenTransform(ctx, view);
  const [x1, y1] = toScreen(view, x, y);
  const [x2, y2] = toScreen(
    view,
    x + vx * VELOCITY_HORIZON,
    y + vy * VELOCITY_HORIZON,
  );

  ctx.strokeStyle = PALETTE.velocity;
  ctx.lineWidth = 1.5;
  ctx.beginPath();
  ctx.moveTo(x1, y1);
  ctx.lineTo(x2, y2);
  ctx.stroke();
}

function drawKick(ctx: CanvasRenderingContext2D, view: View, kick: Kick): void {
  if (kick.stopX === null || kick.stopY === null) return;

  screenTransform(ctx, view);
  const [x1, y1] = toScreen(view, kick.x, kick.y);
  const [x2, y2] = toScreen(view, kick.stopX, kick.stopY);

  ctx.save();
  ctx.setLineDash([4, 4]);
  ctx.strokeStyle = PALETTE.ball;
  ctx.lineWidth = 1.5;
  ctx.beginPath();
  ctx.moveTo(x1, y1);
  ctx.lineTo(x2, y2);
  ctx.stroke();
  ctx.restore();
}

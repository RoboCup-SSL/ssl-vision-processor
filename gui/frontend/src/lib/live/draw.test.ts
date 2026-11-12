import { describe, expect, it } from "vitest";
import {
  PALETTE,
  clear,
  drawField,
  drawLayer,
  extentOf,
  viewFor,
  type LayerStyle,
} from "./draw";
import { fromDetection, fromTracked } from "./model";

// jsdom has no canvas: a context that records every call and assignment, so a
// test can check what was drawn without pixels.
interface Recorder {
  ctx: CanvasRenderingContext2D;
  calls: { name: string; args: unknown[] }[];
  sets: { name: string; value: unknown }[];
}

function recorder(): Recorder {
  const calls: Recorder["calls"] = [];
  const sets: Recorder["sets"] = [];
  const state: Record<string, unknown> = {};

  const ctx = new Proxy(state, {
    get(target, name: string) {
      if (name in target) return target[name];
      if (name === "measureText")
        return (text: string) => ({ width: text.length * 6 });

      return (...args: unknown[]) => {
        calls.push({ name, args });
      };
    },
    set(target, name: string, value) {
      sets.push({ name, value });
      target[name] = value;

      return true;
    },
  }) as unknown as CanvasRenderingContext2D;

  return { ctx, calls, sets };
}

const count = (r: Recorder, name: string): number =>
  r.calls.filter((c) => c.name === name).length;
const setTo = (r: Recorder, name: string): unknown[] =>
  r.sets.filter((s) => s.name === name).map((s) => s.value);

const FIELD = {
  fieldLength: 9000,
  fieldWidth: 6000,
  goalWidth: 1000,
  goalDepth: 180,
  boundaryWidth: 300,
  fieldLines: [
    {
      name: "TopTouchLine",
      p1: { x: -4500, y: 3000 },
      p2: { x: 4500, y: 3000 },
      thickness: 10,
    },
    {
      name: "HalfwayLine",
      p1: { x: 0, y: -3000 },
      p2: { x: 0, y: 3000 },
      thickness: 10,
    },
  ],
  fieldArcs: [
    {
      name: "CenterCircle",
      center: { x: 0, y: 0 },
      radius: 500,
      a1: 0,
      a2: 6.28,
      thickness: 10,
    },
  ],
};

const view = viewFor(extentOf(FIELD), 960, 660, 2, false);

const style = (over: Partial<LayerStyle> = {}): LayerStyle => ({
  robotStyle: "numbered",
  alpha: 0.5,
  outline: null,
  velocities: false,
  confidence: false,
  ...over,
});

describe("drawField", () => {
  it("fills the field color, then draws its lines and arcs", () => {
    const r = recorder();
    drawField(r.ctx, view, FIELD);

    expect(r.calls[0]?.name).toBe("setTransform");
    expect(setTo(r, "fillStyle")[0]).toBe(PALETTE.field);
    expect(count(r, "fillRect")).toBeGreaterThan(0);
    expect(count(r, "lineTo") + count(r, "strokeRect")).toBeGreaterThanOrEqual(
      2,
    );
    expect(count(r, "arc")).toBeGreaterThanOrEqual(1);
  });

  it("colors the goals by team only when the sides are known", () => {
    const unknown = recorder();
    drawField(unknown.ctx, view, FIELD);
    const known = recorder();
    drawField(known.ctx, view, FIELD, { blueOnPositive: true });

    const styles = (r: Recorder) => [
      ...setTo(r, "strokeStyle"),
      ...setTo(r, "fillStyle"),
    ];
    expect(styles(unknown)).not.toContain(PALETTE.team.blue);
    expect(styles(known)).toContain(PALETTE.team.blue);
    expect(styles(known)).toContain(PALETTE.team.yellow);
  });

  it("draws more with the axes on", () => {
    const plain = recorder();
    drawField(plain.ctx, view, FIELD);
    const axes = recorder();
    drawField(axes.ctx, view, FIELD, { axes: true });

    expect(axes.calls.length).toBeGreaterThan(plain.calls.length);
  });

  it("copes with a field the host hasn't filled in yet", () => {
    expect(() => {
      drawField(recorder().ctx, viewFor(extentOf({}), 300, 200, 1, true), {});
    }).not.toThrow();
  });
});

describe("drawLayer", () => {
  const detection = fromDetection({
    robotsYellow: [
      { robotId: 3, x: 100, y: -200, orientation: 1.5, confidence: 0.9 },
    ],
    robotsBlue: [{ robotId: 1, x: -500, y: 400, confidence: 0.4 }],
    balls: [{ x: 10, y: 20, confidence: 0.5 }],
  });

  const tracked = fromTracked({
    robots: [
      {
        robotId: { id: 2, team: "BLUE" },
        pos: { x: 1.5, y: -0.25 },
        orientation: 0,
        vel: { x: 1, y: 0 },
      },
    ],
    balls: [{ pos: { x: 0.1, y: 0.2, z: 0.05 }, vel: { x: 2, y: 0 } }],
    kickedBall: { pos: { x: 0, y: 0 }, stopPos: { x: 2, y: 1 } },
  });

  it("draws at the layer's alpha and restores the context", () => {
    const r = recorder();
    drawLayer(r.ctx, view, detection, style({ alpha: 0.3 }));

    expect(setTo(r, "globalAlpha")[0]).toBe(0.3);
    expect(count(r, "save")).toBe(count(r, "restore"));
  });

  it("draws every robot and ball", () => {
    const empty = recorder();
    drawLayer(
      empty.ctx,
      view,
      { ...detection, robots: [], balls: [] },
      style(),
    );
    const full = recorder();
    drawLayer(full.ctx, view, detection, style());

    expect(count(empty, "arc")).toBe(0);
    expect(count(full, "arc")).toBeGreaterThanOrEqual(3);
  });

  it("adds outlines, confidence bars and velocities only when asked", () => {
    const calls = (layer: typeof tracked, s: LayerStyle): number => {
      const r = recorder();
      drawLayer(r.ctx, view, layer, s);
      return r.calls.length;
    };

    expect(calls(tracked, style({ outline: "#f00" }))).toBeGreaterThan(
      calls(tracked, style()),
    );
    expect(calls(tracked, style({ velocities: true }))).toBeGreaterThan(
      calls(tracked, style()),
    );
    // Confidence comes with raw detections.
    expect(calls(detection, style({ confidence: true }))).toBeGreaterThan(
      calls(detection, style()),
    );
  });

  it("numbers every robot, or in the pattern style only those its dots can't show", () => {
    const numbered = recorder();
    drawLayer(numbered.ctx, view, detection, style());
    const pattern = recorder();
    drawLayer(pattern.ctx, view, detection, style({ robotStyle: "pattern" }));

    // The blue robot has no orientation, so no dot pattern.
    expect(count(numbered, "fillText")).toBe(2);
    expect(count(pattern, "fillText")).toBe(1);
  });

  it("draws a kicked ball's path", () => {
    const r = recorder();
    drawLayer(r.ctx, view, { ...tracked, robots: [], balls: [] }, style());

    expect(r.calls.length).toBeGreaterThan(2);
  });
});

it("clear resets the transform and clears the whole view", () => {
  const r = recorder();
  clear(r.ctx, view);

  expect(r.calls.map((c) => c.name)).toEqual(["setTransform", "clearRect"]);
  expect(r.calls[1]?.args).toEqual([0, 0, 960, 660]);
});

import { describe, expect, it } from "vitest";
import type { LiveFieldSnapshot } from "../apiTypes";
import { extentOf, toScreen, viewFor } from "./draw";
import { fromDetection, fromTracked } from "./model";
import { idDots } from "./patterns";
import { VISION, layersOf, processingMs, sourcesOf } from "./sources";

describe("fromDetection", () => {
  it("keeps mm, both teams, and drops robots without a position", () => {
    const layer = fromDetection({
      robotsYellow: [
        { robotId: 3, x: 100, y: -200, orientation: 1.5, confidence: 0.9 },
      ],
      robotsBlue: [
        { robotId: 7, x: "NaN", y: 0 },
        { x: 5, y: 6 },
      ],
      balls: [{ x: 10, y: 20, confidence: 0.5 }],
    });

    expect(layer.robots).toEqual([
      {
        team: "yellow",
        id: 3,
        x: 100,
        y: -200,
        orientation: 1.5,
        confidence: 0.9,
        vx: null,
        vy: null,
      },
      {
        team: "blue",
        id: null,
        x: 5,
        y: 6,
        orientation: null,
        confidence: null,
        vx: null,
        vy: null,
      },
    ]);
    expect(layer.balls).toEqual([
      { x: 10, y: 20, z: null, confidence: 0.5, vx: null, vy: null },
    ]);
  });
});

describe("fromTracked", () => {
  it("converts m to mm and reads team from the robot id", () => {
    const layer = fromTracked({
      robots: [
        {
          robotId: { id: 2, team: "BLUE" },
          pos: { x: 1.5, y: -0.25 },
          orientation: 0,
          vel: { x: 1, y: 0 },
          visibility: 1,
        },
        { robotId: { id: 4, team: "UNKNOWN" }, pos: { x: 0, y: 0 } },
      ],
      balls: [{ pos: { x: 0.1, y: 0.2, z: 0.05 } }],
      kickedBall: { pos: { x: 0, y: 0 }, stopPos: { x: 2, y: 1 } },
    });

    expect(layer.robots).toHaveLength(1);
    expect(layer.robots[0]).toMatchObject({
      team: "blue",
      id: 2,
      x: 1500,
      y: -250,
      vx: 1000,
      vy: 0,
    });
    expect(layer.balls[0]).toMatchObject({ x: 100, y: 200, z: 50 });
    expect(layer.kick).toEqual({ x: 0, y: 0, stopX: 2000, stopY: 1000 });
  });
});

describe("idDots", () => {
  it("gives every standard ID its own pattern", () => {
    const patterns = Array.from({ length: 16 }, (_, id) =>
      idDots(id)?.join(","),
    );

    expect(new Set(patterns).size).toBe(16);
  });

  it("reads src/pattern.h's bits front left first, 1 as green", () => {
    // ID 0 is 0b0100.
    expect(idDots(0)).toEqual(["pink", "green", "pink", "pink"]);
    expect(idDots(8)).toEqual(["green", "green", "green", "green"]);
  });

  it("has no pattern for unknown or non-standard IDs", () => {
    expect(idDots(null)).toBeNull();
    expect(idDots(16)).toBeNull();
  });
});

const at = "2026-10-25T12:00:01.000Z";

function snapshot(): LiveFieldSnapshot {
  return {
    at,
    cameras: [
      {
        cameraId: 0,
        address: "10.0.0.5",
        fps: 60,
        receivedAt: "2026-10-25T12:00:00.990Z",
        receiving: true,
        frame: {
          cameraId: 0,
          tCapture: 1.0,
          tSent: 1.004,
          robotsYellow: [{ x: 0, y: 0 }],
        },
      },
      {
        cameraId: 1,
        address: "10.0.0.5",
        fps: 0,
        receivedAt: "2026-10-25T12:00:00.000Z",
        receiving: false,
      },
    ],
    trackers: [
      {
        uuid: "u1",
        sourceName: "TIGERs AutoRef",
        address: "10.0.0.20",
        fps: 100,
        receivedAt: "2026-10-25T12:00:00.995Z",
        receiving: true,
        frame: { robots: [], balls: [{ pos: { x: 1, y: 0 } }] },
      },
      {
        uuid: "u2",
        sourceName: "",
        address: "10.0.0.21",
        fps: 0,
        receivedAt: "2026-10-25T12:00:00.000Z",
        receiving: false,
      },
    ],
  };
}

describe("sourcesOf", () => {
  it("lists vision first, then each autoref by name or uuid", () => {
    const sources = sourcesOf(snapshot());

    expect(sources.map((s) => s.key)).toEqual([
      VISION,
      "tracker:TIGERs AutoRef",
      "tracker:u2",
    ]);
    expect(sources[0]).toMatchObject({ receiving: true, fps: 60, ageMs: 10 });
    expect(sources[1]).toMatchObject({ name: "TIGERs AutoRef", ageMs: 5 });
  });

  it("lists vision even before anything is heard", () => {
    expect(sourcesOf(null)).toEqual([
      {
        key: VISION,
        kind: "vision",
        name: "",
        receiving: false,
        fps: 0,
        ageMs: null,
      },
    ]);
  });
});

describe("layersOf", () => {
  it("gives one layer per camera with a frame, minus hidden ones", () => {
    expect(layersOf(snapshot(), VISION, []).map((l) => l.camera)).toEqual([0]);
    expect(layersOf(snapshot(), VISION, [0])).toEqual([]);
  });

  it("gives an autoref's frame, or nothing while it's silent", () => {
    const [tigers] = layersOf(snapshot(), "tracker:TIGERs AutoRef", []);

    expect(tigers?.layer.balls[0]).toMatchObject({ x: 1000, y: 0 });
    expect(layersOf(snapshot(), "tracker:u2", [])).toEqual([]);
  });
});

describe("processingMs", () => {
  it("is capture to send in ms", () => {
    const [camera] = snapshot().cameras;

    expect(camera && processingMs(camera)).toBeCloseTo(4);
  });
});

describe("toScreen", () => {
  const extent = extentOf({
    fieldLength: 9000,
    fieldWidth: 6000,
    boundaryWidth: 300,
    goalDepth: 180,
  });

  it("puts +x right and +y up, and swaps both when flipped", () => {
    const view = viewFor(extent, 960, 660, 1, false);

    expect(toScreen(view, 0, 0)).toEqual([480, 330]);
    expect(toScreen(view, 4800, 3300)).toEqual([960, 0]);

    const flipped = viewFor(extent, 960, 660, 1, true);

    expect(toScreen(flipped, 4800, 3300)).toEqual([0, 660]);
  });
});

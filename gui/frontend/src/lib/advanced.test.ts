import { beforeEach, describe, expect, it } from "vitest";
import {
  advancedOverrides,
  advancedSpec,
  advancedValue,
  setAdvanced,
} from "./advanced.svelte";
import { config, type ConfigDocument } from "./config.svelte";
import { schema, type Schema } from "./schema.svelte";

const blobs = { block: "thresholds", key: "blobs" } as const;
const score = { block: "thresholds", key: "score" } as const;
const refresh = { block: "debug", key: "debug_stream_interval_ms" } as const;

function docOf(): ConfigDocument {
  if (!config.doc) throw new Error("no document loaded");
  return config.doc;
}

beforeEach(() => {
  schema.value = {
    settings: {
      "thresholds.blobs": {
        type: "integer",
        default: 2000,
        apply: "restart",
        range: { min: 100, max: 10000, step: 100 },
      },
      "thresholds.score": { type: "number", default: 5, apply: "live" },
      "debug.debug_stream_interval_ms": {
        type: "integer",
        default: 0,
        apply: "live",
        unit: "ms",
        range: { min: 0, max: 5000, step: 100 },
      },
    },
    drivers: [],
  } as unknown as Schema;
  config.doc = { cameras: [] } as unknown as ConfigDocument;
});

describe("advancedSpec", () => {
  it("takes the fallback, range, unit and restart flag from the schema", () => {
    expect(advancedSpec(refresh)).toEqual({
      fallback: 0,
      min: 0,
      max: 5000,
      step: 100,
      unit: "ms",
      restart: false,
    });
    expect(advancedSpec(blobs).restart).toBe(true);
  });

  it("gives a setting without a range a 0 - 100 slider", () => {
    expect(advancedSpec(score)).toMatchObject({ min: 0, max: 100, step: 1 });
  });
});

describe("setAdvanced", () => {
  it("writes into defaults, keeping the block's other keys", () => {
    docOf().defaults = { thresholds: { circularity: 15 }, other: 1 };

    setAdvanced(score, 7.5);

    expect(docOf().defaults).toEqual({
      thresholds: { circularity: 15, score: 7.5 },
      other: 1,
    });
    expect(advancedValue(score)).toBe(7.5);
  });

  it("rounds whole-number settings only", () => {
    setAdvanced(blobs, 2050.4);
    setAdvanced(refresh, 249.6);
    setAdvanced(score, 2.25);

    expect(docOf().defaults).toEqual({
      thresholds: { blobs: 2050, score: 2.25 },
      debug: { debug_stream_interval_ms: 250 },
    });
  });

  it("removes a reset key, then its emptied block", () => {
    docOf().defaults = { debug: { debug_stream_interval_ms: 500 } };

    setAdvanced(refresh, undefined);

    expect(docOf().defaults).toEqual({});
    expect(advancedValue(refresh)).toBeUndefined();
  });

  it("reads a value of the wrong type as unset", () => {
    docOf().defaults = { debug: { debug_stream_interval_ms: "500" } };

    expect(advancedValue(refresh)).toBeUndefined();
  });

  it("writes nothing before the document has loaded", () => {
    config.doc = null;

    setAdvanced(score, 1);

    expect(config.doc).toBeNull();
  });
});

describe("advancedOverrides", () => {
  it("lists cameras whose own config sets any shared block", () => {
    config.doc = {
      cameras: [
        { cameraId: 0 },
        { cameraId: 1, config: { tracking: {} } },
        { cameraId: 2, config: { debug: { debug_images: true } } },
        { cameraId: 3, config: { color: {} } },
      ],
    } as unknown as ConfigDocument;

    expect(advancedOverrides()).toEqual([1, 2]);
  });
});

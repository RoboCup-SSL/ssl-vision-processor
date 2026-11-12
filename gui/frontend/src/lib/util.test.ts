import { describe, expect, it } from "vitest";
import { without } from "./util";

describe("without", () => {
  it("returns a copy without the key, leaving the original alone", () => {
    const o = { a: 1, b: 2 };

    expect(without(o, "a")).toEqual({ b: 2 });
    expect(o).toEqual({ a: 1, b: 2 });
  });

  it("returns a new object even when the key is absent", () => {
    const o = { a: 1 };
    const out = without(o, "z");

    expect(out).toEqual(o);
    expect(out).not.toBe(o);
  });

  it("treats undefined as empty", () => {
    expect(without(undefined, "a")).toEqual({});
  });
});

import { afterEach, describe, expect, it, vi } from "vitest";
import { api, apiPath, unwrap } from "./api";

function answer(status: number, body: string, type = "application/json"): void {
  vi.stubGlobal(
    "fetch",
    vi.fn(() =>
      Promise.resolve(
        new Response(body, {
          status,
          statusText: "Bad Gateway",
          headers: { "Content-Type": type },
        }),
      ),
    ),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("unwrap", () => {
  it("gives the answer's data", async () => {
    answer(200, '["config.state"]');
    expect(await unwrap(api.GET("/api/topics"))).toEqual(["config.state"]);
  });

  it("throws the host's message, one issue per line if it lists them", async () => {
    answer(
      400,
      '{"error":"invalid","message":"2 problems","issues":["a","b"]}',
    );
    await expect(unwrap(api.GET("/api/config"))).rejects.toThrow("a\nb");

    answer(404, '{"error":"not_found","message":"no such camera"}');
    await expect(unwrap(api.GET("/api/config"))).rejects.toThrow(
      "no such camera",
    );
  });

  it("throws what a proxy answered, or else the status", async () => {
    answer(502, "upstream is down", "text/plain");
    await expect(unwrap(api.GET("/api/config"))).rejects.toThrow(
      "upstream is down",
    );

    answer(502, "", "text/plain");
    await expect(unwrap(api.GET("/api/config"))).rejects.toThrow(
      "502 Bad Gateway",
    );
  });

  it("rejects when the host doesn't answer", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => Promise.reject(new TypeError("Failed to fetch"))),
    );
    await expect(unwrap(api.GET("/api/health"))).rejects.toThrow(
      "Failed to fetch",
    );
  });
});

describe("apiPath", () => {
  it("fills in a path's parameters", () => {
    expect(
      apiPath("/api/snapshots/{id}/{view}", { id: 0, view: "pixels.corner" }),
    ).toBe("/api/snapshots/0/pixels.corner");
    expect(apiPath("/ws/video/{id}", { id: 3 })).toBe("/ws/video/3");
    expect(() => apiPath("/ws/video/{id}")).toThrow("no id");
  });
});

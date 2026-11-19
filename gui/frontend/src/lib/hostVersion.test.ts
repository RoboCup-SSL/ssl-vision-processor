import { beforeEach, describe, expect, it, vi } from "vitest";

// The page's version is the first one the host reports; a later different one
// means the host was upgraded under the open page.
let versions: (string | Error)[];

beforeEach(() => {
  vi.resetModules();
  versions = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(() => {
      const next = versions.shift();
      if (next instanceof Error || next === undefined)
        return Promise.reject(next ?? new Error("no reply"));

      return Promise.resolve(
        new Response(JSON.stringify({ status: "ok", version: next })),
      );
    }),
  );
});

describe("hostUpdated", () => {
  it("is false while the host reports the page's own version", async () => {
    const { checkHostVersion, hostUpdated, hostVersion } =
      await import("./hostVersion.svelte");
    versions = ["v2.0.0-alpha.1", "v2.0.0-alpha.1"];

    await checkHostVersion();
    await checkHostVersion();

    expect(hostVersion.page).toBe("v2.0.0-alpha.1");
    expect(hostUpdated()).toBe(false);
  });

  it("is true once the host reports another version", async () => {
    const { checkHostVersion, hostUpdated, hostVersion } =
      await import("./hostVersion.svelte");
    versions = ["v2.0.0-alpha.1", "v2.0.0-alpha.2"];

    await checkHostVersion();
    await checkHostVersion();

    expect(hostVersion).toEqual({
      page: "v2.0.0-alpha.1",
      host: "v2.0.0-alpha.2",
    });
    expect(hostUpdated()).toBe(true);
  });

  it("ignores an unreachable host", async () => {
    const { checkHostVersion, hostUpdated, hostVersion } =
      await import("./hostVersion.svelte");
    versions = ["v2.0.0-alpha.1", new TypeError("Failed to fetch")];

    await checkHostVersion();
    await checkHostVersion();

    expect(hostVersion.host).toBe("v2.0.0-alpha.1");
    expect(hostUpdated()).toBe(false);
  });
});

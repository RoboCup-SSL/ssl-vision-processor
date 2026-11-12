import { tick } from "svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ConfigDocument, ConfigState } from "../config.svelte";

// The module reads the URL hash when it loads, so each test sets the hash
// first and loads a fresh copy (with fresh config and preferences).
async function load(hash = "") {
  history.replaceState(null, "", `/${hash}`);
  vi.resetModules();
  const nav = await import("./nav.svelte");
  const { config } = await import("../config.svelte");
  const { preferences } = await import("../preferences.svelte");

  return { ...nav, config, preferences };
}

function withCameras(
  config: { doc: ConfigDocument | null },
  ...ids: number[]
): void {
  config.doc = {
    cameras: ids.map((cameraId) => ({
      cameraId,
      instance: `host${String(cameraId)}`,
    })),
  } as unknown as ConfigDocument;
}

beforeEach(() => {
  localStorage.clear();
});

describe("the URL hash", () => {
  it("selects the camera and page on load", async () => {
    const { nav } = await load("#2/geometry");

    expect(nav.selectedInstanceId).toBe("cam:2");
    expect(nav.selectedCategoryId).toBe("geometry");
  });

  it("defaults to the field dimensions with no or a malformed hash", async () => {
    expect((await load("")).nav.selectedCategoryId).toBe("field_dimensions");
    expect((await load("#geometry")).nav.selectedCategoryId).toBe(
      "field_dimensions",
    );
    expect((await load("#0/no_such_page")).nav.selectedCategoryId).toBe(
      "field_dimensions",
    );
  });

  it("keeps the selected camera when the hash names a shared page only", async () => {
    const { nav } = await load("#/network");

    expect(nav.selectedInstanceId).toBeNull();
    expect(nav.selectedCategoryId).toBe("network");
  });

  it("follows hash changes, as back and forward make them", async () => {
    const { nav } = await load("#0/camera");

    location.hash = "#1/color";
    window.dispatchEvent(new HashChangeEvent("hashchange"));

    expect(nav.selectedInstanceId).toBe("cam:1");
    expect(nav.selectedCategoryId).toBe("color");
  });

  it("won't open an expert page unless expert mode is on", async () => {
    expect((await load("#0/advanced")).nav.selectedCategoryId).toBe(
      "field_dimensions",
    );

    localStorage.setItem(
      "vision-processor-gui:preferences",
      JSON.stringify({ expertUser: true }),
    );
    const expert = await load("#0/advanced");
    expect(expert.preferences.expertUser).toBe(true);
    expect(expert.nav.selectedCategoryId).toBe("advanced");
  });

  it("leaves an expert page when expert mode is turned off", async () => {
    localStorage.setItem(
      "vision-processor-gui:preferences",
      JSON.stringify({ expertUser: true }),
    );
    const { nav, preferences } = await load("#0/advanced");
    expect(nav.selectedCategoryId).toBe("advanced");

    preferences.expertUser = false;
    await tick();

    expect(nav.selectedCategoryId).toBe("field_dimensions");
  });

  it("is corrected to match the selection, without a history entry", async () => {
    const { nav, config } = await load("");
    withCameras(config, 3);
    const length = history.length;

    nav.selectedCategoryId = "network";
    await tick();

    expect(location.hash).toBe("#3/network");
    expect(history.length).toBe(length);
  });
});

describe("links", () => {
  it("build #camera/page, keeping the other half of the selection", async () => {
    const { navHref, categoryHref, instanceHref, nav, config } =
      await load("#1/color");
    withCameras(config, 0, 1);

    expect(navHref(undefined, "network")).toBe("#/network");
    expect(navHref(4, "camera")).toBe("#4/camera");
    expect(categoryHref("geometry")).toBe("#1/geometry");
    expect(instanceHref(0)).toBe("#0/color");

    nav.selectedCategoryId = "stream";
    expect(instanceHref(0)).toBe("#0/stream");
  });
});

describe("instances", () => {
  it("are one row per camera in vision.yml", async () => {
    const { instances, config } = await load("");
    withCameras(config, 0, 1);

    expect(instances()).toEqual([
      { id: "cam:0", host: "host0", cameraId: 0 },
      { id: "cam:1", host: "host1", cameraId: 1 },
    ]);
  });

  it("select the first camera until one is picked, or when the picked one is gone", async () => {
    const { selectedInstance, nav, config } = await load("#5/camera");
    expect(selectedInstance()).toBeUndefined();

    withCameras(config, 0, 1);
    expect(selectedInstance()?.cameraId).toBe(0);

    nav.selectedInstanceId = "cam:1";
    expect(selectedInstance()?.cameraId).toBe(1);
  });
});

describe("selectedCategory", () => {
  it("is the selected page, or the field dimensions for an unknown id", async () => {
    const { selectedCategory, nav } = await load("#0/network");
    expect(selectedCategory().id).toBe("network");

    nav.selectedCategoryId = "gone";
    expect(selectedCategory().id).toBe("field_dimensions");
  });
});

describe("isCategoryDirty", () => {
  async function withChanges(changes: object[]) {
    const loaded = await load("#1/camera");
    withCameras(loaded.config, 0, 1);
    loaded.config.state = { changes, cameras: [] } as unknown as ConfigState;

    return loaded;
  }

  it("marks a per-camera page dirty only for the selected camera's changes", async () => {
    const { isCategoryDirty } = await withChanges([
      { section: "camera", cameraId: 0, path: "x" },
    ]);
    expect(isCategoryDirty("camera")).toBe(false);

    const other = await withChanges([
      { section: "camera", cameraId: 1, path: "x" },
    ]);
    expect(other.isCategoryDirty("camera")).toBe(true);
  });

  it("marks a shared page dirty for a change on any camera", async () => {
    const { isCategoryDirty } = await withChanges([
      { section: "network", cameraId: 0, path: "x" },
    ]);

    expect(isCategoryDirty("network")).toBe(true);
  });

  it("never marks pages without a section", async () => {
    const { isCategoryDirty } = await withChanges([
      { section: "other", path: "x" },
    ]);

    expect(isCategoryDirty("live_field")).toBe(false);
    expect(isCategoryDirty("alerts")).toBe(false);
  });
});

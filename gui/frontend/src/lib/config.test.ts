import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { app } from "./text/app";
import type { ConfigDocument } from "./config.svelte";

// The shared working document against a stand-in host. Each test loads a fresh
// copy of the module, since it keeps what it last synced at module scope.
type Config = typeof import("./config.svelte");

interface Request {
  method: string;
  url: string;
  body: unknown;
}

type Reply = { status: number; body: unknown } | Error;

function docOf(c: { doc: ConfigDocument | null }): ConfigDocument {
  if (!c.doc) throw new Error("no document loaded");
  return c.doc;
}

let requests: Request[];
let replies: Map<string, Reply[]>;

const doc = () => ({
  version: 2,
  field: { fieldLength: 9000 },
  cameras: [{ cameraId: 0 }],
});
const state = (revision: number) => ({
  revision,
  path: "/cfg/vision.yml",
  changes: [],
  cameras: [],
});

function reply(method: string, url: string, ...answers: Reply[]): void {
  replies.set(`${method} ${url}`, answers);
}

function sent(method: string, url: string): Request[] {
  return requests.filter((r) => r.method === method && r.url === url);
}

function fakeFetch(url: string, init: RequestInit = {}): Promise<Response> {
  const method = init.method ?? "GET";
  const body =
    typeof init.body === "string"
      ? (JSON.parse(init.body) as unknown)
      : undefined;
  requests.push({ method, url, body });

  const queue = replies.get(`${method} ${url}`) ?? [];
  const answer = queue.length > 1 ? queue.shift() : queue[0];
  if (answer instanceof Error) return Promise.reject(answer);
  const { status, body: out } = answer ?? { status: 200, body: {} };

  return Promise.resolve(
    new Response(JSON.stringify(out), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

async function settle(ms = 0): Promise<void> {
  await vi.advanceTimersByTimeAsync(ms);
}

async function loaded(): Promise<Config> {
  const mod = await import("./config.svelte");
  await mod.loadConfig();
  await settle();
  requests = [];

  return mod;
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.resetModules();
  requests = [];
  replies = new Map();
  vi.stubGlobal("fetch", vi.fn(fakeFetch));
  reply("GET", "/api/schema", {
    status: 200,
    body: { settings: {}, drivers: [] },
  });
  reply("GET", "/api/geometry", { status: 200, body: {} });
  reply("GET", "/api/config", {
    status: 200,
    body: { document: doc(), state: state(3) },
  });
  reply("PUT", "/api/config", { status: 200, body: { revision: 4 } });
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("loadConfig", () => {
  it("loads the schema, then the document and its revision", async () => {
    const { config, loadConfig } = await import("./config.svelte");
    await loadConfig();

    expect(requests.map((r) => r.url)).toEqual([
      "/api/schema",
      "/api/config",
      "/api/geometry",
    ]);
    expect(config.doc).toEqual(doc());
    expect(config.revision).toBe(3);
    expect(config.state?.path).toBe("/cfg/vision.yml");
  });

  it("doesn't echo a freshly loaded document back to the host", async () => {
    await loaded();
    await settle(1000);

    expect(sent("PUT", "/api/config")).toEqual([]);
  });

  it("shows the host's error when the document can't be read", async () => {
    reply("GET", "/api/config", {
      status: 500,
      body: { error: "internal", message: "disk on fire" },
    });
    const { config, loadConfig } = await import("./config.svelte");
    await loadConfig();

    expect(config.error).toBe("disk on fire");
    expect(config.doc).toBeNull();
  });
});

describe("edits", () => {
  it("are debounced into one PUT of the whole document at the confirmed revision", async () => {
    const { config } = await loaded();

    docOf(config).field.fieldLength = 9100;
    await settle(100);
    docOf(config).field.fieldLength = 9200;
    await settle(249);
    expect(sent("PUT", "/api/config")).toEqual([]);

    await settle(1);
    const puts = sent("PUT", "/api/config");
    expect(puts).toHaveLength(1);
    expect(puts[0]?.body).toMatchObject({
      revision: 3,
      document: { field: { fieldLength: 9200 } },
    });
    expect(config.revision).toBe(4);
    expect(config.error).toBeNull();
  });

  it("refreshes the field markings after an edit applies", async () => {
    const { config } = await loaded();

    docOf(config).field.fieldLength = 9100;
    await settle(300);

    expect(sent("GET", "/api/geometry")).toHaveLength(1);
  });

  it("show the host's reasons when an edit is rejected", async () => {
    reply("PUT", "/api/config", {
      status: 400,
      body: {
        error: "invalid",
        message: "invalid",
        issues: ["field_length: too small", "field_width: too small"],
      },
    });
    const { config } = await loaded();

    docOf(config).field.fieldLength = 1;
    await settle(300);

    expect(config.error).toBe(
      "field_length: too small\nfield_width: too small",
    );
    expect(config.revision).toBe(3);
  });

  it("take the host's document when another tab edited first", async () => {
    reply("PUT", "/api/config", {
      status: 409,
      body: { error: "stale", message: "stale" },
    });
    const { config } = await loaded();
    reply("GET", "/api/config", {
      status: 200,
      body: { document: doc(), state: state(7) },
    });

    docOf(config).field.fieldLength = 9100;
    await settle(300);

    expect(config.error).toBe(app.editedElsewhere);
    expect(config.revision).toBe(7);
    expect(config.doc?.field.fieldLength).toBe(9000);
  });

  it("made while a PUT is in flight go out next, at the new revision", async () => {
    reply(
      "PUT",
      "/api/config",
      { status: 200, body: { revision: 4 } },
      { status: 200, body: { revision: 5 } },
    );
    const { config } = await loaded();
    let release!: () => void;
    const gate = new Promise<void>((r) => (release = r));
    vi.mocked(fetch).mockImplementationOnce(async (url, init) => {
      await gate;
      return fakeFetch(url as string, init);
    });

    docOf(config).field.fieldLength = 9100;
    await settle(300);
    docOf(config).field.fieldLength = 9200;
    await settle(300);
    release();
    await settle(300);

    const puts = sent("PUT", "/api/config");
    expect(
      puts.map(
        (p) =>
          (p.body as { document: ReturnType<typeof doc> }).document.field
            .fieldLength,
      ),
    ).toEqual([9100, 9200]);
    expect(puts[1]?.body).toMatchObject({ revision: 4 });
    expect(config.revision).toBe(5);
  });

  it.todo(
    "sent while the host is unreachable are reported and retried (REVIEW.md T4-2)",
  );
});

describe("flushEdits", () => {
  it("sends a pending edit at once and waits for it", async () => {
    const { config, flushEdits } = await loaded();

    docOf(config).field.fieldLength = 9100;
    await settle();
    await flushEdits();

    expect(sent("PUT", "/api/config")).toHaveLength(1);
    expect(config.revision).toBe(4);
  });

  it("does nothing when nothing changed", async () => {
    const { flushEdits } = await loaded();

    await flushEdits();

    expect(requests).toEqual([]);
  });
});

describe("actions", () => {
  it("save flushes pending edits first, then saves at the new revision", async () => {
    reply("POST", "/api/config/save", { status: 200, body: { revision: 4 } });
    const { config, saveConfig } = await loaded();

    docOf(config).field.fieldLength = 9100;
    await settle();
    const conflict = await saveConfig();

    expect(requests.map((r) => `${r.method} ${r.url}`).slice(0, 2)).toEqual([
      "PUT /api/config",
      "GET /api/geometry",
    ]);
    expect(sent("POST", "/api/config/save")[0]?.body).toEqual({
      revision: 4,
      force: false,
    });
    expect(conflict).toBeNull();
    expect(config.busy).toBe(false);
  });

  it("save returns a disk conflict for the caller to resolve", async () => {
    const body = {
      error: "disk_changed",
      message: "vision.yml changed on disk",
    };
    reply("POST", "/api/config/save", { status: 409, body });
    const { saveConfig } = await loaded();

    expect(await saveConfig()).toEqual(body);
  });

  it("save as and load send the file name", async () => {
    const { saveConfigAs, loadConfigFrom } = await loaded();

    await saveConfigAs("lab.yml");
    await loadConfigFrom("event.yml");

    expect(sent("POST", "/api/config/save-as")[0]?.body).toEqual({
      revision: 3,
      name: "lab.yml",
    });
    expect(sent("POST", "/api/config/load")[0]?.body).toEqual({
      name: "event.yml",
    });
  });

  it("a failed action shows its reason and reloads the document", async () => {
    reply("POST", "/api/config/load", {
      status: 400,
      body: { error: "invalid", message: "no such file" },
    });
    const { config, loadConfigFrom } = await loaded();

    await loadConfigFrom("missing.yml");

    expect(config.error).toBe("no such file");
    expect(sent("GET", "/api/config")).toHaveLength(1);
  });

  it("locking without a live calibration shows the host's reason", async () => {
    const url = "/api/config/cameras/0/calibration";
    reply("POST", url, {
      status: 409,
      body: { error: "no_live_calibration", message: "nothing to lock yet" },
    });
    const { config, lockCalibration } = await loaded();

    await lockCalibration(0);

    expect(config.error).toBe("nothing to lock yet");
  });

  it("unlocking deletes the camera's calibration", async () => {
    const { unlockCalibration } = await loaded();

    await unlockCalibration(2);

    expect(sent("DELETE", "/api/config/cameras/2/calibration")).toHaveLength(1);
  });

  it("reload accepts the file on disk", async () => {
    const { reloadFromDisk } = await loaded();

    await reloadFromDisk();

    expect(sent("POST", "/api/config/reload")).toHaveLength(1);
  });
});

describe("helpers", () => {
  it("changesFor filters by section and camera, keeping shared changes", async () => {
    const { config, changesFor } = await loaded();
    config.state = {
      ...state(3),
      changes: [
        { section: "camera", cameraId: 0, path: "a" },
        { section: "camera", cameraId: 1, path: "b" },
        { section: "camera", path: "shared" },
        { section: "network", path: "c" },
      ],
    } as unknown as NonNullable<typeof config.state>;

    expect(changesFor("camera", 0).map((c) => c.path)).toEqual(["a", "shared"]);
    expect(changesFor("camera").map((c) => c.path)).toEqual([
      "a",
      "b",
      "shared",
    ]);
    expect(changesFor("network").map((c) => c.path)).toEqual(["c"]);
  });

  it("cameraDoc and cameraStatus find a camera by id", async () => {
    const { config, cameraDoc, cameraStatus } = await loaded();
    config.state = {
      ...state(3),
      cameras: [{ cameraId: 0, locked: true }],
    } as unknown as NonNullable<typeof config.state>;

    expect(cameraDoc(0)).toEqual({ cameraId: 0 });
    expect(cameraDoc(5)).toBeUndefined();
    expect(cameraStatus(0)).toMatchObject({ locked: true });
  });

  it("formatValue summarizes blocks and shows scalars as JSON", async () => {
    const { formatValue } = await import("./config.svelte");

    expect(formatValue(undefined)).toBe("(none)");
    expect(formatValue(null)).toBe("(none)");
    expect(formatValue({ a: 1, b: 2 })).toBe("{2 fields}");
    expect(formatValue([1, 2])).toBe("[1,2]");
    expect(formatValue("x")).toBe('"x"');
    expect(formatValue(3)).toBe("3");
  });
});

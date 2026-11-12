import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { LogRecord } from "../apiTypes";

type HostLog = typeof import("./hostLog.svelte");

// GET /api/logs as a queue of pages. Once it's empty a request hangs, like a
// long poll with nothing new, until it's aborted.
let pages: (object | Error)[];
let urls: string[];

function record(n: number): LogRecord {
  return { seq: n, message: `record ${String(n)}` } as unknown as LogRecord;
}

function page(from: number, to: number, missed = false): object {
  const records = [];
  for (let n = from; n <= to; n++) records.push(record(n));
  return { records, next: to, missed };
}

function fakeFetch(url: string, init: RequestInit = {}): Promise<Response> {
  urls.push(url);
  const next = pages.shift();

  if (next instanceof Error) return Promise.reject(next);
  if (next)
    return Promise.resolve(new Response(JSON.stringify(next), { status: 200 }));

  return new Promise((_, reject) => {
    init.signal?.addEventListener("abort", () => {
      reject(new DOMException("aborted", "AbortError"));
    });
  });
}

let mod: HostLog;

beforeEach(async () => {
  vi.useFakeTimers();
  vi.resetModules();
  pages = [];
  urls = [];
  vi.stubGlobal("fetch", vi.fn(fakeFetch));
  mod = await import("./hostLog.svelte");
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

const seqs = () =>
  mod.logRecords().map((r) => (r as unknown as { seq: number }).seq);

describe("followLog", () => {
  it("asks for what's logged at once, then long-polls after the last record", async () => {
    pages = [page(1, 3), page(4, 4)];
    const stop = mod.followLog("warn");
    await vi.advanceTimersByTimeAsync(0);

    expect(urls).toEqual([
      "/api/logs?after=0&level=warn&limit=1000",
      "/api/logs?after=3&level=warn&limit=1000&wait=25",
      "/api/logs?after=4&level=warn&limit=1000&wait=25",
    ]);
    expect(seqs()).toEqual([1, 2, 3, 4]);
    stop();
  });

  it("reports missed records only after the first page", async () => {
    pages = [page(5, 6, true)];
    const stop = mod.followLog("info");
    await vi.advanceTimersByTimeAsync(0);
    expect(mod.hostLog.missed).toBe(false);
    stop();

    pages = [page(1, 1), page(9, 9, true)];
    const again = mod.followLog("info");
    await vi.advanceTimersByTimeAsync(0);
    expect(mod.hostLog.missed).toBe(true);
    again();
  });

  it("shows a failure and retries after 2 s from the same place", async () => {
    pages = [page(1, 2), new Error("host unreachable"), page(3, 3)];
    const stop = mod.followLog("info");
    await vi.advanceTimersByTimeAsync(0);

    expect(mod.hostLog.error).toBe("host unreachable");
    expect(urls).toHaveLength(2);

    await vi.advanceTimersByTimeAsync(2000);
    expect(urls[2]).toContain("after=2&");
    expect(mod.hostLog.error).toBeNull();
    expect(seqs()).toEqual([1, 2, 3]);
    stop();
  });

  it("stops asking once stopped, and starts over on a new level", async () => {
    pages = [page(1, 2)];
    const stop = mod.followLog("info");
    await vi.advanceTimersByTimeAsync(0);
    stop();
    await vi.advanceTimersByTimeAsync(10_000);
    expect(urls).toHaveLength(2);

    pages = [page(7, 7)];
    const next = mod.followLog("error");
    await vi.advanceTimersByTimeAsync(0);
    expect(urls[2]).toBe("/api/logs?after=0&level=error&limit=1000");
    expect(seqs()).toEqual([7]);
    next();
  });

  it("doesn't retry or show an error for a request it aborted itself", async () => {
    const stop = mod.followLog("info");
    await vi.advanceTimersByTimeAsync(0);
    stop();
    await vi.advanceTimersByTimeAsync(5000);

    expect(mod.hostLog.error).toBeNull();
    expect(urls).toHaveLength(1);
  });

  it("keeps the newest 1000 records", async () => {
    pages = [page(1, 900), page(901, 1200)];
    const stop = mod.followLog("debug");
    await vi.advanceTimersByTimeAsync(0);

    expect(seqs()).toHaveLength(1000);
    expect(seqs()[0]).toBe(201);
    expect(seqs().at(-1)).toBe(1200);
    stop();
  });
});

describe("pause and clear", () => {
  it("holds records while paused and shows them on resume", async () => {
    mod.setPaused(true);
    pages = [page(1, 2), page(3, 3)];
    const stop = mod.followLog("info");
    await vi.advanceTimersByTimeAsync(0);

    expect(seqs()).toEqual([]);
    expect(mod.hostLog.waiting).toBe(3);

    mod.setPaused(false);
    expect(seqs()).toEqual([1, 2, 3]);
    expect(mod.hostLog.waiting).toBe(0);
    stop();
  });

  it("clear empties the panel and the missed flag", async () => {
    pages = [page(1, 2), page(5, 5, true)];
    const stop = mod.followLog("info");
    await vi.advanceTimersByTimeAsync(0);
    expect(mod.hostLog.missed).toBe(true);

    mod.clearLog();
    expect(seqs()).toEqual([]);
    expect(mod.hostLog.missed).toBe(false);
    stop();
  });
});

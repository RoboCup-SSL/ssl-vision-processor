import { afterEach, describe, expect, it, vi } from "vitest";
import { recentErrors, recordError } from "./errorLog";
import { buildReport, isUnavailable, reportFileName } from "./systemReport";

afterEach(() => {
  vi.unstubAllGlobals();
  recentErrors.length = 0;
});

describe("buildReport", () => {
  it("is still made when the host doesn't answer", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => Promise.reject(new Error("connection refused"))),
    );

    const report = await buildReport();

    expect(report.host).toEqual({ error: "connection refused" });
    expect(isUnavailable(report.network)).toBe(true);
    expect(report.browser.userAgent).toBe(navigator.userAgent);
    expect(report.frontend.connection).toBe("closed");
    expect(report.report.format).toBe(1);
    expect(report.config).toBeUndefined();
  });

  it("asks for the configuration only when it's included", async () => {
    const fetch = vi.fn<(url: string) => Promise<Response>>(() =>
      Promise.resolve(new Response(JSON.stringify([]))),
    );
    vi.stubGlobal("fetch", fetch);

    const without = await buildReport();
    const urls = fetch.mock.calls.map(([url]) => url);
    expect(urls).toContain("/api/alerts");
    expect(urls).not.toContain("/api/config");
    expect(without).not.toHaveProperty("config");

    const report = await buildReport({ includeConfig: true });
    expect(fetch.mock.calls.map(([url]) => url)).toContain("/api/config");
    expect(report.config).toEqual([]);
  });

  it("includes the host's answer and the recorded errors", async () => {
    const system = { host: { hostname: "field-pc" } };
    vi.stubGlobal(
      "fetch",
      vi.fn(() => Promise.resolve(new Response(JSON.stringify(system)))),
    );
    recordError(new Error("boom"));

    const report = await buildReport();

    expect(report.host).toEqual(system);
    expect(report.errors).toHaveLength(1);
    expect(report.errors[0]?.message).toBe("boom");
    expect(reportFileName(report)).toMatch(
      /^vision-gui-report-field-pc-\d{8}-\d{6}Z\.json$/,
    );
  });
});

describe("recordError", () => {
  it("keeps only the latest 20", () => {
    for (let i = 0; i < 25; i++) recordError(`error ${String(i)}`);

    expect(recentErrors).toHaveLength(20);
    expect(recentErrors[0]?.message).toBe("error 5");
  });
});

import { describe, expect, it } from "vitest";
import type { BuildInfo } from "../apiTypes";
import { buildSummary, gitSummary, issueUrl } from "./issue";

const build: BuildInfo = {
  version: "v2.0.0-3-gabc1234-dirty",
  repo: {
    commit: "abc1234567890def",
    branch: "master",
    tag: "",
    dirty: true,
  },
  proto: { commit: "fdd7988ac689d894", branch: "", tag: "v1.2", dirty: false },
  stamped: true,
  goVersion: "go1.26.7",
};

describe("gitSummary", () => {
  it("says the short commit, tag, branch and whether it was modified", () => {
    expect(gitSummary(build.repo)).toBe(
      "abc123456789, branch master, modified",
    );
    expect(gitSummary(build.proto)).toBe("fdd7988ac689, tag v1.2, detached");
  });

  it("says unknown without a commit", () => {
    expect(gitSummary({ commit: "", branch: "", tag: "", dirty: false })).toBe(
      "unknown",
    );
  });
});

describe("issueUrl", () => {
  it("fills the body with the build and browser", () => {
    const url = new URL(issueUrl(build));
    const body = url.searchParams.get("body") ?? "";

    expect(url.pathname).toBe("/RoboCup-SSL/ssl-vision-processor/issues/new");
    for (const line of buildSummary(build)) expect(body).toContain(`- ${line}`);
    expect(body).toContain(navigator.userAgent);
  });

  it("still starts an issue when the host didn't answer", () => {
    const body = new URL(issueUrl(null)).searchParams.get("body") ?? "";

    expect(body).toContain("Version: unknown");
  });
});

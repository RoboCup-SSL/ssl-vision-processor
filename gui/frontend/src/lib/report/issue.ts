// A new GitHub issue, its body filled in with what the developers ask first:
// the build, and the browser. The rest comes from the system report the
// body asks for.

import type { BuildInfo, GitState } from "../apiTypes";
import { about } from "../text/about";
import { links } from "../text/links";

const text = about.issue;

// One repository's state on one line: short commit, branch or tag, and
// whether it was modified.
export function gitSummary(git: GitState): string {
  if (!git.commit) return text.unknown;

  return [
    git.commit.slice(0, 12),
    git.tag && `tag ${git.tag}`,
    git.branch ? `branch ${git.branch}` : "detached",
    git.dirty && "modified",
  ]
    .filter(Boolean)
    .join(", ");
}

// The issue's build lines, or one saying the host couldn't be asked.
export function buildSummary(build: BuildInfo | null): string[] {
  if (!build) return [text.buildUnknown];

  return [
    `${text.version}: ${build.version}`,
    `${text.repo}: ${gitSummary(build.repo)}`,
    `${text.proto}: ${gitSummary(build.proto)}`,
  ];
}

export function issueUrl(build: BuildInfo | null): string {
  const url = new URL(links.newIssue);
  url.searchParams.set(
    "body",
    text.body(buildSummary(build), navigator.userAgent),
  );

  return url.toString();
}

// The settings menu's About dialog, System report dialog, and the issue
// Report An Issue starts (report/AboutDialog.svelte,
// report/SystemReportDialog.svelte, report/issue.ts).
// `backticks` show as code.

export const about = {
  title: "About SSL Vision Processor",
  repo: "ssl-vision-processor",
  proto: "ssl-protocol-defs (protocol submodule)",
  detached: "detached",
  noTag: "none",
  clean: "Clean",
  dirty: "Modified",
  unknown: "Unknown",
  dirtyNote:
    "Built from a tree with uncommitted changes, so the commit alone doesn't say what's running.",
  unstamped:
    "Built without `make`, so the branch, tag and protocol definitions aren't known.",
  hostUnavailable: (error: string): string =>
    `Couldn't ask the host for its build: ${error}`,
  // The GUI from Vite's dev server: its build can differ from the host's.
  devServer:
    "This page is served by the Vite dev server (`npm run dev`), not the host, so it may be newer than the host's build.",

  report: {
    title: "Download system report",
    intro:
      "A JSON file for the developers to attach to an issue. It's made in this browser and saved to this computer; nothing is sent anywhere.",
    contains: "It contains:",
    sections: [
      "The build: version, and the git branch, tag and commit of this program and its protocol definitions.",
      "This host: operating system, kernel, hostname, CPUs, time zone, locale, the command line flags it was started with, and every network interface with its addresses.",
      "The multicast sockets: their addresses, what each one hears, and from which senders.",
      "This browser: name and version, operating system, languages, screen size, and video support.",
      "The host's alerts, and its last 50 warnings and errors.",
      "The GUI's settings in this browser, and its last errors in this tab.",
    ],
    includeConfig: "Include the configuration (vision.yml)",
    includeConfigNote:
      "The working copy, unsaved changes included: cameras, calibrations, colors and network addresses.",
    notIncluded:
      "Not included: camera images, or vision_processor's own logs. Attach them separately if the issue needs them.",
    privacy:
      "Check it before posting it publicly: hostnames, IP addresses and file paths can identify your network.",
    gathering: "Gathering…",
    showContents: "Show contents",
    hideContents: "Hide contents",
    copied: "Copied",
    newIssue: "New issue ↗︎",
  },

  // The body of the GitHub issue Report An Issue opens. Markdown.
  issue: {
    version: "Version",
    repo: "ssl-vision-processor",
    proto: "ssl-protocol-defs",
    unknown: "unknown",
    buildUnknown: "Version: unknown (the host didn't answer)",
    body: (build: string[], browser: string): string =>
      [
        "### What happened",
        "",
        "",
        "### What you expected to happen",
        "",
        "",
        "### Steps to reproduce",
        "",
        "1. ",
        "",
        "### Build",
        "",
        ...build.map((line) => `- ${line}`),
        `- Browser: ${browser}`,
        "",
        "Please attach a system report (settings menu, System report…): drag the downloaded file into this box.",
      ].join("\n"),
  },
};

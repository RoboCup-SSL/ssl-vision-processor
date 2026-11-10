// The log panel docked at the bottom of the page (logs/LogPanel.svelte).

export const logs = {
  title: "Host log",
  menuShow: "Show host log",
  level: "Lowest level shown",
  // What each level shows, in the level picker.
  levels: {
    debug: "Debug and up",
    info: "Info and up",
    warn: "Warnings and errors",
    error: "Errors only",
  },
  resize: "Drag to resize the log. Arrow keys work too.",
  expand: "Show the log",
  collapse: "Hide the log",
  close: "Close the log panel. The settings menu opens it again.",
  pause: "Pause",
  resume: "Resume",
  clear: "Clear",
  empty: "Nothing logged at this level yet.",
  paused: (waiting: number): string =>
    waiting === 0
      ? "Paused."
      : `Paused. ${String(waiting)} more ${waiting === 1 ? "record" : "records"} logged since.`,
  missed:
    "Some records were logged too quickly to show here. The host's log file has all of them.",
  unreachable: (error: string): string =>
    `Can't reach the host for its log: ${error}. Retrying.`,
};

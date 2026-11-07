// Overview page (config/OverviewPanel.svelte): the current match's teams and
// the robot height used for each. `backticks` show as code.

export const overview = {
  teams: {
    title: "Teams",
    notApplied:
      "Shown here only for now: vision_processor still takes the team names from the game controller and looks their heights up in its height table.",
    yellow: "Yellow",
    blue: "Blue",
    fromGameController: "Derive from game controller",
    fromGameControllerNotes: [
      "On: the game controller's team and its height from the height table. Off: pick a table entry, or enter a custom height.",
      "On a competition field the choice is kept for the team, so it follows the team if the colors are switched. Otherwise it's kept for the color.",
    ],
    team: "Team",
    custom: "Custom",
    customHeight: "Height",
    waiting:
      "Waiting for the game controller: on a competition field, heights are set per team once it names them.",
    noGameController: "No game controller",
    defaultHeight: (mm: number): string => `default height (${String(mm)} mm)`,
    noName: "(no name)",
    height: (mm: number): string => `${String(mm)} mm`,
    notInTable: "not in the height table",
    // Why a team would get vision_processor's default height.
    issue: {
      notInTable:
        "Not in the height table, so vision_processor uses a default height. Turn off Derive and enter a custom height, then Save.",
      customMissing:
        "Custom height not entered, so the default height applies. Enter it, then Save.",
      noHeight:
        "No game controller team and no height set, so vision_processor uses a default height. Turn off Derive and enter a custom height, then Save.",
      unsaved: "Not saved yet. Save to write it to disk.",
    },
    table: (file: string, fallback: number, max: number): string =>
      `Heights from \`${file}\`. A team not in it gets the default, ${String(fallback)} mm; field checks use the tallest, ${String(max)} mm.`,
    tableError: (file: string, error: string): string =>
      `Can't read the height table \`${file}\`: ${error}`,
  },
};

// The page frame (layout/Shell.svelte) and config syncing (config.svelte.ts):
// header badge tooltips, the settings menu's prompts, and edit conflicts.

import { plural } from "./common";
export const app = {
  competitionField: {
    label: "Competition Field",
    title:
      "A match setup with a game controller. Heights are kept per team so they follow color switches, and missing setup shows as errors. Off: lab use, heights per color.",
  },
  // Headers over the tab bar's two groups.
  tabGroups: {
    global: "Global Configs",
    perCamera: (camera: number | undefined): string =>
      camera === undefined
        ? "Per-Camera Configs"
        : `Per-Camera Configs - Field Cam #${String(camera)}`,
  },
  socketBadge: {
    notOpen: (problem: string): string =>
      `Not open: ${problem}. Retrying. Click to configure.`,
    receiving: (address: string): string =>
      `Receiving on ${address}. Click to configure.`,
    silent: "Nothing heard on this address. Click to configure.",
  },
  discardTitle: "Discard unsaved changes?",
  discardConfirm: "Discard and reload",
  discardAndReload: (unsaved: number): string =>
    `Discard ${String(unsaved)} unsaved change(s) and reload from disk?`,
  editedElsewhere: "Edited elsewhere at the same time; reloaded the latest.",

  sidebar: {
    instancesHelp:
      "One row per camera role, not per machine -- a host running all 4 cameras of a quad setup appears here 4 times. From vision.yml's cameras list.",
    noCameras: "Per-instance (no cameras yet)",
  },

  // The settings menu's file dialogs (layout/ConfigDialogs.svelte).
  // `backticks` show as code.
  dialogs: {
    writesTo: (path: string): string => `Writes to \`${path}\`.`,
    noChanges: "No unsaved changes.",
    saveAsNote: "The GUI edits and watches the new file from then on.",
    loadDiscards: (changes: number): string =>
      `Loading discards ${String(changes)} unsaved ${plural(changes, "change")}, and applies the loaded file live.`,
    loadApplies: "The loaded file applies live.",
    editedOutside: (path: string): string =>
      `\`${path}\` was edited outside the GUI.`,
    cantLoad: "It can't be loaded as it is:",
    wouldChange: "Loading it would change:",
    formattingOnly:
      "The values match what's running; only formatting or comments differ.",
    unsavedConflict: (changes: number): string =>
      `You have ${String(changes)} unsaved ${plural(changes, "change")}. Loading discards them; overwriting replaces the edited file with them.`,
  },
};

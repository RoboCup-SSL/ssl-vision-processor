// The page frame (layout/Shell.svelte) and config syncing (config.svelte.ts):
// header badge tooltips, the settings menu's prompts, and edit conflicts.

import { plural } from "./common";
export const app = {
  competitionField: {
    label: "Competition Field",
    title:
      "A match setup with a game controller. Heights are kept per team so they follow color switches, and missing setup shows as errors. Off: lab use, heights per color.",
    leaveTitle: "Leave competition mode?",
    leaveMessage:
      "Robot heights go back to being chosen per color, so they no longer follow teams when they switch colors, and missing match setup stops showing as errors. Leave it on during a match.",
    leaveConfirm: "Leave competition mode",
  },
  // Headers over the tab bar's two groups, and their short forms, inline
  // before the tabs, for when the headers don't fit (layout/TabBar.svelte).
  tabGroups: {
    label: "Pages",
    global: "Global Configs",
    perCamera: (camera: number | undefined): string =>
      camera === undefined
        ? "Per-Camera Configs"
        : `Per-Camera Configs - Field Cam #${String(camera)}`,
    globalShort: "Global",
    perCameraShort: (camera: number | undefined): string =>
      camera === undefined ? "Per camera" : `Cam ${String(camera)}`,
  },
  socketBadge: {
    notOpen: (problem: string): string =>
      `Not open: ${problem}. Retrying. Click to configure.`,
    receiving: (address: string): string =>
      `Receiving on ${address}. Click to configure.`,
    silent: "Nothing heard on this address. Click to configure.",
  },
  // The badges above folded into one dropdown, when the header is too narrow
  // for them (layout/NetworkStatus.svelte).
  socketMenu: {
    label: "Network",
    title:
      "The vision, game controller and autoref tracker addresses. Click for details.",
    receiving: "Receiving",
    silent: "Nothing heard",
    notOpen: (problem: string): string => `Not open: ${problem}`,
  },
  discardTitle: "Discard unsaved changes?",
  discardConfirm: "Discard and reload",
  discardAndReload: (unsaved: number): string =>
    `Discard ${String(unsaved)} unsaved change(s) and reload from disk?`,
  editedElsewhere: "Edited elsewhere at the same time; reloaded the latest.",
  hostUpdated: (from: string, to: string): string =>
    `The GUI was updated (${from} → ${to}). Reload this page to use the new version; until then it may misbehave.`,
  reload: "Reload",
  notSent:
    "Not sent: the host can't be reached. Your edit is kept here and sent when the host is back.",

  sidebar: {
    instancesHelp:
      "One row per camera role, not per machine -- a host running all 4 cameras of a quad setup appears here 4 times. From vision.yml's cameras list.",
    // The status dot of a vision_processor the host runs itself.
    process: {
      running: "vision_processor running",
      starting: "vision_processor starting",
      failing: "vision_processor keeps exiting",
      held: "vision_processor not started: device in use",
      noCamera: "vision_processor not started: no camera selected",
    },
  },

  // The settings menu's file dialogs (layout/ConfigDialogs.svelte).
  // `backticks` show as code.
  dialogs: {
    writesTo: (path: string): string => `Writes to \`${path}\`.`,
    noChanges: "No unsaved changes.",
    firstRunNote:
      "This is a new configuration. Saving it starts publishing on the network.",
    saveAndPublish: "Save and start publishing",
    fileName: "File name in the config folder",
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

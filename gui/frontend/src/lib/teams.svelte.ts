// Robot height choices in vision.yml's teams block.
//
// On a competition field they're kept by the game controller's team name
// (teams.overrides): the game controller swaps the whole team record, name
// included, when teams switch colors, so a choice keyed by name follows its
// team. Off one (lab use, often with no game controller) they're kept per
// color (teams.by_color). GUI only for now; see gui/internal/config's Teams.
import { without } from "./util";
import { config, type TeamOverride } from "./config.svelte";
import { network } from "./network.svelte";

export type Color = "yellow" | "blue";

export const COLORS: Color[] = ["yellow", "blue"];

// The GUI's robot height for a team or color with nothing set, in mm: the
// Division A limit. vision_processor's own fallback is its table's mean.
export const DEFAULT_ROBOT_HEIGHT = 150;

// Issues that only mean the default height applies, as opposed to a choice
// that's been started but not finished.
export function isDefaultHeightIssue(issue: HeightIssue | null): boolean {
  return issue === "notInTable" || issue === "noHeight";
}

export function competitionField(): boolean {
  return config.doc?.competitionField === true;
}

export function setCompetitionField(on: boolean): void {
  if (config.doc) config.doc.competitionField = on || undefined;
}

// The team the game controller currently puts in color: "" if none, or if
// the game controller has gone quiet (its last names may be a past match's).
export function teamName(color: Color): string {
  const state = network.state;

  return state?.gc.receiving && state.referee.heard
    ? state.referee[color].name
    : "";
}

// Where a color's choice is kept right now: by team on a competition field
// (none until the game controller names the team), by color otherwise.
export type Slot = { team: string } | { color: Color };

export function slotFor(color: Color): Slot | null {
  if (!competitionField()) return { color };

  const team = teamName(color);

  return team ? { team } : null;
}

export function choice(slot: Slot | null): TeamOverride | undefined {
  const teams = config.doc?.teams;
  if (!slot) return undefined;

  return "team" in slot
    ? teams?.overrides?.[slot.team]
    : teams?.byColor?.[slot.color];
}

// Replaces a slot's choice; undefined removes it, and emptied blocks go too,
// so vision.yml stays minimal.
function setChoice(slot: Slot, value: TeamOverride | undefined): void {
  const doc = config.doc;
  if (!doc) return;

  const teams = { ...doc.teams };

  if ("team" in slot) {
    const overrides = without(teams.overrides, slot.team);
    if (value) overrides[slot.team] = value;
    teams.overrides = Object.keys(overrides).length ? overrides : undefined;
  } else {
    const byColor = without(teams.byColor, slot.color);
    if (value) byColor[slot.color] = value;
    teams.byColor = Object.keys(byColor).length ? byColor : undefined;
  }

  doc.teams = teams.overrides || teams.byColor ? teams : undefined;
}

// On: no choice, so the height comes from the table (by game controller
// name) or vision_processor's default. Off: a choice, starting as a custom
// height not entered yet.
export function setFromTable(slot: Slot, on: boolean): void {
  setChoice(slot, on ? undefined : (choice(slot) ?? {}));
}

// Another table entry's height, e.g. when the game controller spells the
// team's name differently.
export function setTeam(slot: Slot, tableName: string): void {
  setChoice(slot, { name: tableName });
}

export function setCustomHeight(slot: Slot, height: number): void {
  setChoice(slot, { height });
}

// Why a color's robots would get vision_processor's default height, or null
// if they have a known one:
//   notInTable: the game controller's team isn't in the table, no choice
//   noHeight: no game controller team and no choice for the color
//   customMissing: a custom height chosen but not entered yet
//   unsaved: a choice not written to disk yet
export type HeightIssue =
  | "notInTable"
  | "noHeight"
  | "customMissing"
  | "unsaved";

export function heightIssue(color: Color): HeightIssue | null {
  const ref = network.state?.referee;
  if (!ref || ref.heightsError) return null;

  const team = teamName(color);
  const slot = slotFor(color);

  // On a competition field nothing is playing until the game controller
  // names a team; the game controller alert covers that.
  if (!slot) return null;

  const own = choice(slot);

  if (!own) {
    if (!team) return "noHeight";

    return ref.heights?.[team] === undefined ? "notInTable" : null;
  }

  if (own.name === undefined && own.height === undefined) {
    return "customMissing";
  }

  return unsaved(slot) ? "unsaved" : null;
}

// Whether the saved file lacks this slot's choice as it is now: an unsaved
// change to it, its fields, or a block above it. Paths are compared whole,
// since team names can contain dots ("N.R.G"): a prefix match would tie a
// team called "N" to N.R.G's changes.
function unsaved(slot: Slot): boolean {
  const key =
    "team" in slot
      ? `teams.overrides.${slot.team}`
      : `teams.by_color.${slot.color}`;
  const paths = new Set([
    "teams",
    "teams.overrides",
    "teams.by_color",
    key,
    `${key}.name`,
    `${key}.height`,
  ]);

  return (config.state?.changes ?? []).some((c) => paths.has(c.path));
}

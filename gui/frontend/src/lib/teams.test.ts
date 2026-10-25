import { beforeEach, describe, expect, it } from "vitest";
import { blankField } from "./geometry.svelte";
import { config, type ConfigDocument, type Change } from "./config.svelte";
import { network, type NetworkState } from "./network.svelte";
import { hostAlerts } from "./alerts/hostAlerts.svelte";
import {
  choice,
  heightIssue,
  setCompetitionField,
  setCustomHeight,
  setFromTable,
  setTeam,
  slotFor,
} from "./teams.svelte";

function emptyDoc(): ConfigDocument {
  return {
    version: 1,
    field: blankField(),
    optionalFieldLines: {
      goal2Goal: true,
      halfway: true,
      centerCircle: true,
      penalty: true,
    },
    cameras: [],
  };
}

// The game controller naming yellow and blue, or silent when null.
function hear(teams: { yellow: string; blue: string } | null): void {
  network.state = {
    gc: { receiving: teams !== null },
    referee: {
      heard: teams ? "2026-01-01T00:00:00Z" : undefined,
      yellow: { name: teams?.yellow ?? "" },
      blue: { name: teams?.blue ?? "" },
      heightsFile: "robot-heights.yml",
      heights: { "ER-Force": 148, "TIGERs Mannheim": 143 },
    },
  } as unknown as NetworkState;
}

function unsavedChanges(...paths: string[]): void {
  config.state = {
    path: "vision.yml",
    revision: 1,
    changes: paths.map(
      (path): Change => ({
        path,
        jsonPath: path,
        before: null,
        after: null,
        section: "overview",
      }),
    ),
    cameras: [],
  };
}

beforeEach(() => {
  config.doc = emptyDoc();
  unsavedChanges();
  hear(null);
});

describe("lab use (no competition field)", () => {
  it("keeps choices per color, editable without a game controller", () => {
    const slot = slotFor("yellow");
    expect(slot).toEqual({ color: "yellow" });

    if (slot) setCustomHeight(slot, 150);

    expect(config.doc?.teams).toEqual({ byColor: { yellow: { height: 150 } } });
  });

  it("starts a custom height as an empty choice", () => {
    const slot = slotFor("blue");
    if (slot) setFromTable(slot, false);

    expect(choice(slot)).toEqual({});
  });
});

// Which issue applies is the host's call (gui/internal/alerts' HeightIssue,
// tested there); the GUI reads it off the alert.
describe("heightIssue", () => {
  it("reads the host's team_default_height alert for the color", () => {
    hostAlerts.list = [
      {
        id: "team-height-blue",
        code: "team_default_height",
        severity: "warning",
        params: { color: "blue", team: "", issue: "customMissing" },
        message: "",
      },
    ];

    expect(heightIssue("blue")).toBe("customMissing");
    expect(heightIssue("yellow")).toBeNull();
  });
});

describe("competition field", () => {
  beforeEach(() => {
    setCompetitionField(true);
  });

  it("waits for the game controller to name a team", () => {
    expect(slotFor("yellow")).toBeNull();
  });

  it("follows a team through a color switch", () => {
    hear({ yellow: "ER-Force", blue: "Custom FC" });
    const blue = slotFor("blue");
    if (blue) setCustomHeight(blue, 160);

    hear({ yellow: "Custom FC", blue: "ER-Force" });

    expect(choice(slotFor("yellow"))).toEqual({ height: 160 });
    expect(choice(slotFor("blue"))).toBeUndefined();
  });

  it("borrows another table entry's height", () => {
    hear({ yellow: "ER Force", blue: "ER-Force" });
    const yellow = slotFor("yellow");
    if (yellow) setTeam(yellow, "ER-Force");

    expect(config.doc?.teams?.overrides).toEqual({
      "ER Force": { name: "ER-Force" },
    });
  });
});

import { beforeEach, describe, expect, it } from "vitest";
import { config, type ConfigDocument, type Change } from "./config.svelte";
import { network, type NetworkState } from "./network.svelte";
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
    field: {},
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

  it("warns that the default height applies with nothing set", () => {
    expect(heightIssue("yellow")).toBe("noHeight");
  });

  it("is fine with a saved custom height", () => {
    const slot = slotFor("blue");
    if (slot) setCustomHeight(slot, 150);

    expect(heightIssue("blue")).toBeNull();
  });

  it("flags a custom height not entered yet", () => {
    const slot = slotFor("blue");
    if (slot) setFromTable(slot, false);

    expect(choice(slot)).toEqual({});
    expect(heightIssue("blue")).toBe("customMissing");
  });
});

describe("competition field", () => {
  beforeEach(() => {
    setCompetitionField(true);
  });

  it("waits for the game controller to name a team", () => {
    expect(slotFor("yellow")).toBeNull();
    expect(heightIssue("yellow")).toBeNull();
  });

  it("follows a team through a color switch", () => {
    hear({ yellow: "ER-Force", blue: "Custom FC" });
    const blue = slotFor("blue");
    if (blue) setCustomHeight(blue, 160);

    hear({ yellow: "Custom FC", blue: "ER-Force" });

    expect(choice(slotFor("yellow"))).toEqual({ height: 160 });
    expect(choice(slotFor("blue"))).toBeUndefined();
  });

  it("flags a team not in the height table", () => {
    hear({ yellow: "ER-Force", blue: "Custom FC" });

    expect(heightIssue("yellow")).toBeNull();
    expect(heightIssue("blue")).toBe("notInTable");
  });

  it("borrows another table entry's height", () => {
    hear({ yellow: "ER Force", blue: "ER-Force" });
    const yellow = slotFor("yellow");
    if (yellow) setTeam(yellow, "ER-Force");

    expect(config.doc?.teams?.overrides).toEqual({
      "ER Force": { name: "ER-Force" },
    });
  });

  it("matches unsaved changes by whole path, not prefix", () => {
    hear({ yellow: "N", blue: "N.R.G" });
    for (const color of ["yellow", "blue"] as const) {
      const slot = slotFor(color);
      if (slot) setCustomHeight(slot, 140);
    }

    // Only N.R.G's override is unsaved; "N" mustn't be tied to it.
    unsavedChanges("teams.overrides.N.R.G.height");

    expect(heightIssue("blue")).toBe("unsaved");
    expect(heightIssue("yellow")).toBeNull();
  });
});

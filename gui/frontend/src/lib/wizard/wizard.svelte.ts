// Shared state for the setup wizard drawer. A module-level $state object,
// per the project convention (see geometry.svelte.ts, nav.svelte.ts).
// optionalLines comes before dimensions: which markings exist decides which
// dimension fields are even worth asking about (a field with no penalty box
// has no penalty box depth/width to enter). See WizardDimensions.svelte.
// There's no dedicated save step: finish is a summary/confirm gate, and
// confirming there is what copies `draft` into the live virtualField state
// (see finishWizard below) -- persisting that to disk stays the normal
// Save/Save As action in the main Virtual Field panel.
import {
  virtualField,
  type FieldConfig,
  type OptionalFieldLines,
  type FieldPreset,
} from "../geometry.svelte";

export const WIZARD_STEPS = [
  "start",
  "layout",
  "optionalLines",
  "dimensions",
  "finish",
] as const;

export type WizardStep = (typeof WIZARD_STEPS)[number];

export const wizard = $state<{
  open: boolean;
  step: WizardStep;
  // Advisory only, for the Layout step and Finish's summary -- camera_amount
  // isn't wired to any instance's config.yml yet (see gui/CLAUDE.md's "Not
  // yet built"), so this never gets written anywhere, just relayed back to
  // the operator as a reminder of what to set by hand.
  fieldLayout: "full" | "half";
  cameraCount: number;
  // A working copy the wizard's own steps read and write, kept separate from
  // virtualField so an abandoned wizard (closed without reaching Finish)
  // never leaves a half-entered field applied to the live editor/webpage.
  // Only finishWizard() below copies this into virtualField.
  draft: { field: FieldConfig; optionalFieldLines: OptionalFieldLines };
}>({
  open: false,
  step: "start",
  fieldLayout: "full",
  cameraCount: 1,
  draft: {
    field: {},
    optionalFieldLines: {
      goal2Goal: false,
      halfway: false,
      centerCircle: false,
      penalty: false,
    },
  },
});

export function openWizard(): void {
  wizard.step = "start";
  wizard.fieldLayout = "full";
  wizard.cameraCount = 1;
  // Seed from the current live field, not blank -- reopening the wizard to
  // adjust an already-configured field should start from what's there.
  // $state.snapshot, not structuredClone: virtualField.field is itself a
  // $state proxy, and structuredClone throws ("Proxy object could not be
  // cloned") on one directly -- snapshot first to get a plain object.
  wizard.draft.field = $state.snapshot(virtualField.field);
  wizard.draft.optionalFieldLines = $state.snapshot(
    virtualField.optionalFieldLines,
  );
  wizard.open = true;
}

export function goToStep(step: WizardStep): void {
  wizard.step = step;
}

// The Division A/B short-circuit: loads the preset into the draft (not
// virtualField directly) and skips straight past layout/optional lines/
// dimensions, since a preset already supplies all three.
export function applyPresetToDraft(preset: FieldPreset): void {
  wizard.draft.field = $state.snapshot(preset.field);
  wizard.draft.optionalFieldLines = $state.snapshot(preset.optionalFieldLines);
}

export function skipToFinish(): void {
  wizard.step = "finish";
}

// Only place the draft ever reaches virtualField: called from Finish's
// confirm action. Marks dirty so the main panel's own Save/Save As (unchanged)
// is what actually persists it.
export function finishWizard(): void {
  virtualField.field = $state.snapshot(wizard.draft.field);
  virtualField.optionalFieldLines = $state.snapshot(
    wizard.draft.optionalFieldLines,
  );
  virtualField.dirty = true;
  wizard.open = false;
}

export function nextStep(): void {
  const index = WIZARD_STEPS.indexOf(wizard.step);
  const next = WIZARD_STEPS[index + 1];
  if (next) wizard.step = next;
}

export function previousStep(): void {
  const index = WIZARD_STEPS.indexOf(wizard.step);
  const prev = WIZARD_STEPS[index - 1];
  if (prev) wizard.step = prev;
}

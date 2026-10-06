// Shared state for the setup wizard drawer. A module-level $state object,
// per the project convention (see geometry.svelte.ts, nav.svelte.ts).
// optionalLines comes before dimensions: which markings exist decides which
// dimension fields are even worth asking about (a field with no penalty box
// has no penalty box depth/width to enter). See WizardDimensions.svelte.
// There's no dedicated save step: finish is a summary/confirm gate, and
// confirming there is what copies `draft` into the host's working document
// (see finishWizard below), where it applies live -- persisting it to disk is
// the normal Save in the settings menu.
import type {
  FieldConfig,
  OptionalFieldLines,
  FieldPreset,
} from "../geometry.svelte";
import { config } from "../config.svelte";
import { applyPlan, cameraCount, planCount } from "../cameraLayout";

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
  // Physical cameras over the physical field. Finish writes the layout's
  // camera count from these (wizardCameraCount).
  fieldLayout: "full" | "half";
  cameraCount: number;
  // A working copy the wizard's own steps read and write, kept separate from
  // config.doc so an abandoned wizard (closed without reaching Finish)
  // never leaves a half-entered field applied to the live editor/webpage.
  // Only finishWizard() below copies this into config.doc.
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
  // Start from the current layout, so skipping the Layout step (a preset)
  // leaves it as it is.
  wizard.cameraCount = config.doc ? cameraCount(config.doc) : 1;
  // Seed from the current live field, not blank -- reopening the wizard to
  // adjust an already-configured field should start from what's there.
  // $state.snapshot, not structuredClone: config.doc is a $state proxy, and
  // structuredClone throws ("Proxy object could not be cloned") on one
  // directly -- snapshot first to get a plain object.
  if (config.doc) {
    wizard.draft.field = $state.snapshot(config.doc.field);
    wizard.draft.optionalFieldLines = $state.snapshot(
      config.doc.optionalFieldLines,
    );
  }
  wizard.open = true;
}

export function goToStep(step: WizardStep): void {
  wizard.step = step;
}

// The Division A/B short-circuit: loads the preset into the draft (not
// config.doc directly) and skips straight past layout/optional lines/
// dimensions, since a preset already supplies all three.
export function applyPresetToDraft(preset: FieldPreset): void {
  wizard.draft.field = $state.snapshot(preset.field);
  wizard.draft.optionalFieldLines = $state.snapshot(preset.optionalFieldLines);
}

export function skipToFinish(): void {
  wizard.step = "finish";
}

// Only place the draft ever reaches config.doc: called from Finish's confirm
// action. It applies live from there; Save persists it.
// The layout's camera count for what the wizard asked. A half field is
// entered as the full field it's half of, so its cameras cover half the
// regions: one camera on a half field is camera 0 or 1 of 2.
export function wizardCameraCount(): number {
  return wizard.fieldLayout === "half"
    ? wizard.cameraCount * 2
    : wizard.cameraCount;
}

export function finishWizard(): void {
  if (config.doc) {
    config.doc.field = $state.snapshot(wizard.draft.field);
    config.doc.optionalFieldLines = $state.snapshot(
      wizard.draft.optionalFieldLines,
    );

    // Same rules as the Camera Layout page: every region changes, so seeds
    // go stale and locked calibrations are dropped.
    const count = wizardCameraCount();
    applyPlan(planCount(config.doc, count), count);
  }

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

<script lang="ts">
  import { Modal, Stepper, Button } from "flowbite-svelte";
  import {
    wizard,
    finishWizard,
    nextStep,
    previousStep,
    WIZARD_STEPS,
    type WizardStep,
  } from "./wizard.svelte";
  import WizardStart from "./steps/WizardStart.svelte";
  import WizardLayout from "./steps/WizardLayout.svelte";
  import WizardOptionalLines from "./steps/WizardOptionalLines.svelte";
  import WizardDimensions from "./steps/WizardDimensions.svelte";
  import WizardFinish from "./steps/WizardFinish.svelte";

  const STEP_LABELS: Record<WizardStep, string> = {
    start: "Start",
    layout: "Layout",
    dimensions: "Dimensions",
    optionalLines: "Markings",
    finish: "Finish",
  };

  let steps = WIZARD_STEPS.map((id) => ({ label: STEP_LABELS[id] }));
  let current = $derived(WIZARD_STEPS.indexOf(wizard.step) + 1);

  // Start drives its own navigation (a division choice jumps straight to
  // Finish), so the generic Back/Next footer only applies to the steps after
  // it.
  let showFooterNav = $derived(wizard.step !== "start");
  let isLastStep = $derived(
    wizard.step === WIZARD_STEPS[WIZARD_STEPS.length - 1],
  );
</script>

<!--
  flowbite-svelte's shipped dialog theme (node_modules/flowbite-svelte/dist/dialog/theme.js)
  has its `position: fixed` variant commented out, so nothing in the library
  actually positions/centers the <dialog> -- it silently relies on the native
  <dialog>:modal UA stylesheet alone. Pinning position/inset/margin here
  explicitly so centering doesn't depend on that.

  No wrapping div here: Modal's own body slot already spaces its direct
  children (space-y-4) and scrolls if needed (overflow-y-auto).

  title, not a manual header div: Dialog (which Modal wraps) already renders
  its own dismiss button whenever `dismissable` is true (the default), so a
  second hand-added CloseButton here was a real, if harmless, duplicate --
  confirmed live, two overlapping close buttons in the rendered DOM. `title`
  is the one Modal prop that gives both a header and that single button
  together.
-->
<Modal
  bind:open={wizard.open}
  title="Virtual field setup"
  size="lg"
  class="fixed inset-0 m-auto"
>
  <Stepper {steps} {current} clickable={false} />

  {#if wizard.step === "start"}
    <WizardStart />
  {:else if wizard.step === "layout"}
    <WizardLayout />
  {:else if wizard.step === "optionalLines"}
    <WizardOptionalLines />
  {:else if wizard.step === "dimensions"}
    <WizardDimensions />
  {:else if wizard.step === "finish"}
    <WizardFinish />
  {/if}

  {#if showFooterNav}
    <div class="flex justify-between border-t border-gray-200 pt-4">
      <Button color="alternative" onclick={previousStep}>Back</Button>
      {#if isLastStep}
        <Button onclick={finishWizard}>Apply</Button>
      {:else}
        <Button onclick={nextStep}>Next</Button>
      {/if}
    </div>
  {/if}
</Modal>

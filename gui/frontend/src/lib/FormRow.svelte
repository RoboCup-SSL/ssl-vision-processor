<script lang="ts">
  import type { Snippet } from "svelte";
  import { Helper, Label } from "flowbite-svelte";
  import NoteTip from "./NoteTip.svelte";

  // One labelled setting in a panel form: label, control, and the optional
  // notes tooltip (behind "Show extra tooltips") on one line, with any
  // messages under the control. Every settings form uses it, so they line up
  // the same way.
  interface Props {
    label: string;
    // The control's id, for the label.
    for?: string;
    notes?: string[];
    // Hints, warnings, and errors under the control, in that order.
    hints?: string[];
    warnings?: string[];
    errors?: string[];
    // The label column; wider for forms with long labels.
    labelWidth?: string;
    children: Snippet;
  }

  let {
    label,
    for: htmlFor,
    notes = [],
    hints = [],
    warnings = [],
    errors = [],
    labelWidth = "7rem",
    children,
  }: Props = $props();

  let noteId = $derived(`${htmlFor ?? label.replace(/\W+/g, "-")}-notes`);
</script>

<div class="form-row" style:--label-width={labelWidth}>
  <Label for={htmlFor} class="text-sm font-normal">{label}</Label>

  <div class="control">
    {@render children()}
  </div>

  <NoteTip id={noteId} {notes} />

  {#if hints.length + warnings.length + errors.length > 0}
    <div class="messages">
      {#each hints as hint (hint)}
        <Helper color="gray">{hint}</Helper>
      {/each}
      {#each warnings as warning (warning)}
        <Helper color="yellow">{warning}</Helper>
      {/each}
      {#each errors as error (error)}
        <Helper color="red">{error}</Helper>
      {/each}
    </div>
  {/if}
</div>

<style>
  .form-row {
    display: grid;
    grid-template-columns: var(--label-width) minmax(0, 1fr) 1.25rem;
    align-items: center;
    column-gap: 0.5rem;
    row-gap: 0.25rem;
    margin: 0.5rem 0;
  }

  .control {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    min-width: 0;
  }

  .messages {
    grid-column: 2 / 4;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
</style>

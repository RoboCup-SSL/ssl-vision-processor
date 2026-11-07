<script lang="ts">
  import { Button, Input, Range, Toggle } from "flowbite-svelte";
  import { untrack } from "svelte";
  import FormRow from "../FormRow.svelte";

  // One numeric setting: a slider for the usual range and a number box for
  // anything outside it. With `auto`, a toggle switches to the value that
  // means automatic (vision_processor's 0 for exposure and gain), and
  // remembers the manual value for switching back.
  interface Props {
    id: string;
    label: string;
    value: number;
    min: number;
    max: number;
    step: number;
    unit?: string;
    // label defaults to "Auto"; gamma's sentinel (1.0) reads "Off".
    auto?: { value: number; fallback: number; label?: string };
    notes?: string[];
    hints?: string[];
    warnings?: string[];
    // A button that puts the setting back to its default; disabled while it
    // already is.
    reset?: { label: string; disabled: boolean; onclick: () => void };
    onchange: (value: number) => void;
  }

  let {
    id,
    label,
    value,
    min,
    max,
    step,
    unit = "",
    auto,
    notes = [],
    hints = [],
    warnings = [],
    reset,
    onchange,
  }: Props = $props();

  let isAuto = $derived(auto?.value === value);

  let lastManual = $state(
    untrack(() => (auto?.value === value ? auto.fallback : value)),
  );

  $effect(() => {
    if (!isAuto) lastManual = value;
  });

  function set(raw: number): void {
    if (Number.isFinite(raw)) onchange(raw);
  }
</script>

<FormRow {label} for={id} {notes} {hints} {warnings}>
  <div class="controls">
    {#if auto}
      <Toggle
        size="small"
        class="shrink-0"
        checked={isAuto}
        onchange={(e: Event) => {
          set(
            (e.currentTarget as HTMLInputElement).checked
              ? auto.value
              : lastManual,
          );
        }}
      >
        <span class="text-xs">{auto.label ?? "Auto"}</span>
      </Toggle>
    {/if}

    <Range
      size="sm"
      class="min-w-24 flex-1"
      {min}
      {max}
      {step}
      value={isAuto ? lastManual : value}
      disabled={isAuto}
      oninput={(e: Event) => {
        set((e.currentTarget as HTMLInputElement).valueAsNumber);
      }}
    />
    <div class="w-24 shrink-0">
      <Input
        {id}
        type="number"
        size="sm"
        {step}
        value={isAuto ? "" : value}
        placeholder={isAuto ? (auto?.label ?? "Auto").toLowerCase() : ""}
        disabled={isAuto}
        onchange={(e: Event) => {
          set((e.currentTarget as HTMLInputElement).valueAsNumber);
        }}
      />
    </div>
    <span class="unit">{unit}</span>
    {#if reset}
      <Button
        size="xs"
        color="alternative"
        class="shrink-0 px-2"
        title={reset.label}
        aria-label={reset.label}
        disabled={reset.disabled}
        onclick={reset.onclick}>↺</Button
      >
    {/if}
  </div>
</FormRow>

<style>
  .controls {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .unit {
    flex-shrink: 0;
    width: 5.5rem;
    color: var(--text-muted);
    font-size: 0.8rem;
  }
</style>

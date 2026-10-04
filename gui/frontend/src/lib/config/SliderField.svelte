<script lang="ts">
  import { untrack } from "svelte";
  import NoteTip from "../NoteTip.svelte";

  // One numeric setting: a slider for the usual range and a number box for
  // anything outside it. With `auto`, a checkbox switches to the value that
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
    warnings?: string[];
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
    warnings = [],
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

<div class="row">
  <label for={id}>{label}</label>

  <div class="controls">
    {#if auto}
      <label class="auto">
        <input
          type="checkbox"
          checked={isAuto}
          onchange={(e) => {
            set(e.currentTarget.checked ? auto.value : lastManual);
          }}
        />
        {auto.label ?? "Auto"}
      </label>
    {/if}

    <input
      type="range"
      {min}
      {max}
      {step}
      value={isAuto ? lastManual : value}
      disabled={isAuto}
      oninput={(e) => {
        set(e.currentTarget.valueAsNumber);
      }}
    />
    <input
      {id}
      type="number"
      class="number"
      {step}
      value={isAuto ? "" : value}
      placeholder={isAuto ? (auto?.label ?? "Auto").toLowerCase() : ""}
      disabled={isAuto}
      onchange={(e) => {
        set(e.currentTarget.valueAsNumber);
      }}
    />
    <span class="unit">{unit}</span>
  </div>

  <NoteTip id={`${id}-notes`} {notes} />
</div>

{#each warnings as warning (warning)}
  <p class="warning">{warning}</p>
{/each}

<style>
  .row {
    display: grid;
    grid-template-columns: 7rem 1fr 1.25rem;
    align-items: center;
    gap: 0.5rem;
    margin: 0.45rem 0;
    font-size: 0.85rem;
  }

  .controls {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .auto {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    font-size: 0.8rem;
  }

  input[type="range"] {
    flex: 1;
    min-width: 6rem;
  }

  .number {
    width: 5.5rem;
    font-size: 0.85rem;
  }

  .unit {
    width: 5.5rem;
    color: #666;
    font-size: 0.8rem;
  }

  .warning {
    margin: 0 0 0.4rem 7.5rem;
    color: #7a4a00;
    font-size: 0.8rem;
  }
</style>

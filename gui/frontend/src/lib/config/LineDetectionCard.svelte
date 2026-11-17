<script lang="ts">
  import { Toggle } from "flowbite-svelte";
  import FormRow from "../FormRow.svelte";
  import SettingsCard from "../SettingsCard.svelte";
  import SliderField from "./SliderField.svelte";
  import { cameraDoc } from "../config.svelte";
  import {
    LINE_KEYS,
    geometryValue,
    lineSpec,
    overridden,
    setGeometryKey,
  } from "../lineDetection.svelte";
  import { geometry } from "../text/geometry";

  const text = geometry.lineDetection;

  interface Props {
    cameraId: number;
  }

  let { cameraId }: Props = $props();

  let locked = $derived(cameraDoc(cameraId)?.calibration !== undefined);
  let refinement = $derived(geometryValue(cameraId, "refinement") === true);
</script>

<SettingsCard title={text.title}>
  <FormRow
    label={text.refinement.label}
    for="geom-refinement"
    notes={text.refinement.notes}
    hints={[locked ? text.locked : text.restart]}
  >
    <Toggle
      id="geom-refinement"
      size="small"
      checked={refinement}
      onchange={(e: Event) => {
        setGeometryKey(
          cameraId,
          "refinement",
          (e.currentTarget as HTMLInputElement).checked,
        );
      }}
    />
  </FormRow>

  {#each LINE_KEYS as key (key)}
    {@const spec = lineSpec(key)}
    {@const field = text.fields[key]}
    <SliderField
      id={`geom-${key}`}
      label={field.label}
      value={geometryValue(cameraId, key) as number}
      min={spec.min}
      max={spec.max}
      step={spec.step}
      unit={spec.unit}
      notes={[...field.notes, text.defaultIs(String(spec.fallback))]}
      reset={{
        label: text.reset,
        disabled: !overridden(cameraId, key),
        onclick: () => {
          setGeometryKey(cameraId, key, undefined);
        },
      }}
      onchange={(value: number) => {
        setGeometryKey(cameraId, key, value);
      }}
    />
  {/each}
</SettingsCard>

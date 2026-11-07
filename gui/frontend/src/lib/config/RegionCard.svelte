<script lang="ts">
  import { Button, Select } from "flowbite-svelte";
  import SettingsCard from "../SettingsCard.svelte";
  import FormRow from "../FormRow.svelte";
  import { layout as text } from "../text/layout";
  import type { CameraStatus, ConfigDocument } from "../config.svelte";
  import { navHref } from "../layout/nav.svelte";
  import {
    cameraDevice,
    deviceLabel,
    hostName,
    liveLabel,
    setDisplay,
    type SlotInfo,
  } from "../cameraLayout";

  // The selected region's details on the Camera Layout page, with the
  // camera picker that moves or swaps cameras between regions.
  interface Props {
    doc: ConfigDocument;
    slot: SlotInfo;
    status: CameraStatus | undefined;
    // Asks to move the camera in region from to this region.
    onmove: (from: number, to: number) => void;
  }

  let { doc, slot, status, onmove }: Props = $props();

  let selected = $derived(slot.cameraId);

  function cameraName(cameraId: number): string {
    const camera = doc.cameras.find((c) => c.cameraId === cameraId);
    if (!camera) return "";

    return `${hostName(camera)} · ${deviceLabel(cameraDevice(doc, camera))}`;
  }

  function mm(v: number): string {
    return String(Math.round(v));
  }
</script>

<SettingsCard title={`Region ${String(selected)} (${slot.region})`}>
  <FormRow label="Bounds" labelWidth="5rem">
    <span class="text-sm">
      x {mm(slot.slice.minX)} … {mm(slot.slice.maxX)}, y {mm(slot.slice.minY)} … {mm(
        slot.slice.maxY,
      )} mm
    </span>
  </FormRow>

  <FormRow
    label="Live"
    labelWidth="5rem"
    errors={slot.conflict ? [text.conflict] : []}
  >
    <span class="text-sm">{liveLabel(slot.sources)}</span>
  </FormRow>

  <FormRow
    label="Camera"
    for="layout-camera"
    labelWidth="5rem"
    notes={text.cameraPicker}
  >
    <Select
      id="layout-camera"
      size="sm"
      placeholder=""
      value={slot.camera ? selected : -1}
      onchange={(e: Event) => {
        const from = Number((e.currentTarget as HTMLSelectElement).value);
        onmove(from, selected);
        // The select shows the change only once it's confirmed.
        (e.currentTarget as HTMLSelectElement).value = String(
          slot.camera ? selected : -1,
        );
      }}
    >
      {#if !slot.camera}
        <option value={-1} disabled>No camera</option>
      {/if}
      {#each doc.cameras as c (c.cameraId)}
        <option value={c.cameraId}>
          {cameraName(c.cameraId)}{c.cameraId === selected
            ? ""
            : ` (now ${String(c.cameraId)})`}
        </option>
      {/each}
    </Select>
  </FormRow>

  {#if slot.camera}
    {@const camera = slot.camera}
    <FormRow label="Video" labelWidth="5rem" notes={text.videoOrientation}>
      <span class="flex items-center gap-2">
        <Button
          size="xs"
          color="alternative"
          onclick={() => {
            setDisplay(selected, {
              rotate: (camera.display?.rotate ?? 0) + 90,
            });
          }}>↻ Rotate</Button
        >
        <Button
          size="xs"
          color={camera.display?.mirror ? "primary" : "alternative"}
          onclick={() => {
            setDisplay(selected, { mirror: !camera.display?.mirror });
          }}>⇋ Mirror</Button
        >
        <span class="text-sm text-gray-600 dark:text-gray-400">
          {camera.display?.rotate ?? 0}°{camera.display?.mirror
            ? ", mirrored"
            : ""}
        </span>
      </span>
    </FormRow>
    <FormRow label="Device" labelWidth="5rem">
      <span class="text-sm break-all">
        {cameraDevice(doc, camera).path ??
          deviceLabel(cameraDevice(doc, camera))}
      </span>
    </FormRow>
    <FormRow
      label="Config"
      labelWidth="5rem"
      hints={camera.configPath ? [] : [text.remote]}
    >
      <span class="text-sm break-all">
        {camera.configPath ?? "remote"}
      </span>
    </FormRow>
    <FormRow
      label="Calibration"
      labelWidth="5rem"
      warnings={(status?.warnings ?? []).map((w) => w.message)}
    >
      <span class="text-sm">
        {status?.calibration ?? "none"}{camera.seed?.lineCorners.length === 4
          ? ", corners picked"
          : ", no corners"}
      </span>
    </FormRow>

    <div class="mt-2 flex flex-wrap gap-2">
      <Button size="xs" color="alternative" href={navHref(selected, "camera")}>
        Camera Settings →
      </Button>
      <Button
        size="xs"
        color="alternative"
        href={navHref(selected, "geometry")}
      >
        Geometry →
      </Button>
    </div>
  {/if}
</SettingsCard>

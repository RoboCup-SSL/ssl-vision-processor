<script lang="ts">
  import ConfirmModal from "../ConfirmModal.svelte";
  import StatusAlert from "../StatusAlert.svelte";
  import { geometry as text } from "../text/geometry";
  import SettingsCard from "../SettingsCard.svelte";
  import { Button, Spinner, P } from "flowbite-svelte";
  import {
    config,
    cameraDoc,
    cameraStatus,
    lockCalibration,
    unlockCalibration,
  } from "../config.svelte";

  interface Props {
    cameraId: number;
  }

  let { cameraId }: Props = $props();

  let status = $derived(cameraStatus(cameraId));
  let locked = $derived(cameraDoc(cameraId)?.calibration);
  let renderError = $derived(config.state?.renderErrors?.[String(cameraId)]);

  let liveResolution = $derived(
    status && status.liveResolution[0] > 0
      ? `${String(status.liveResolution[0])}x${String(status.liveResolution[1])}`
      : null,
  );

  function lockedResolution(camera: Record<string, unknown>): string {
    const w = camera["pixel_image_width"];
    const h = camera["pixel_image_height"];

    return typeof w === "number" && typeof h === "number"
      ? `${String(w)}x${String(h)}`
      : "unknown size";
  }

  let confirmingUnlock = $state(false);
</script>

<ConfirmModal
  bind:open={confirmingUnlock}
  title={text.unlockTitle}
  message={text.confirmUnlock(cameraId)}
  confirmLabel={text.unlockConfirm}
  danger
  onconfirm={() => void unlockCalibration(cameraId)}
/>

<SettingsCard title="Calibration" class="my-4">
  {#if !status}
    <P
      size="sm"
      class="mb-2 text-gray-600 dark:text-gray-400 flex items-center gap-2"
      ><Spinner size="4" /> Loading…</P
    >
  {:else}
    <P size="sm" class="mb-2">
      {#if status.calibration === "locked" && locked}
        <strong>{text.calibration.locked}</strong>
        {text.calibration.lockedDetail(
          new Date(locked.lockedAt).toLocaleString(),
          lockedResolution(locked.camera),
        )}
      {:else if status.calibration === "live"}
        <strong>{text.calibration.live}</strong>{liveResolution
          ? ` (${liveResolution})`
          : ""}{text.calibration.liveDetail}
      {:else}
        <strong>{text.calibration.none}</strong>
        {text.calibration.noneDetail}
      {/if}
    </P>

    {#each status.warnings as warning (warning.code)}
      <StatusAlert color="yellow" class="my-1 p-2 text-sm"
        >{warning.message}</StatusAlert
      >
    {/each}

    {#if renderError}
      <StatusAlert color="red" class="my-1 p-2 text-sm"
        >{renderError}</StatusAlert
      >
    {/if}

    <div class="actions">
      <Button
        size="sm"
        disabled={!status.live || config.busy}
        title={status.live ? text.lockTitle : text.nothingToLock}
        onclick={() => void lockCalibration(cameraId)}
      >
        {status.calibration === "locked" ? text.relock : text.lock}
      </Button>
      <Button
        size="sm"
        color="alternative"
        disabled={status.calibration === "none" || config.busy}
        onclick={() => {
          confirmingUnlock = true;
        }}
      >
        {text.calibration.unlock}
      </Button>
    </div>
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400">
      {text.calibration.footer}
    </P>
  {/if}
</SettingsCard>

<style>
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin: 0.5rem 0;
  }
</style>

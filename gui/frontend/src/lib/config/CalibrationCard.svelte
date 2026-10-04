<script lang="ts">
  import SettingsCard from "../SettingsCard.svelte";
  import { Button, Spinner, Alert, P } from "flowbite-svelte";
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

  function unlock(): void {
    if (
      confirm(
        `Unlock camera ${String(cameraId)}'s calibration? It stops being published now, and the camera recalibrates the next time its vision_processor restarts.`,
      )
    ) {
      void unlockCalibration(cameraId);
    }
  }
</script>

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
        <strong>Locked</strong> at {new Date(locked.lockedAt).toLocaleString()}
        ({lockedResolution(locked.camera)}). Published in place of anything the
        camera sends, and kept across restarts once saved.
      {:else if status.calibration === "live"}
        <strong>Live, not locked</strong>{liveResolution
          ? ` (${liveResolution})`
          : ""}. The camera calibrated itself; it recalibrates on its next
        restart unless you lock this.
      {:else}
        <strong>None.</strong> No calibration received from this camera yet.
      {/if}
    </P>

    {#each status.warnings as warning (warning.code)}
      <Alert color="yellow" class="my-1 p-2 text-sm">{warning.message}</Alert>
    {/each}

    {#if renderError}
      <Alert color="red" class="my-1 p-2 text-sm">{renderError}</Alert>
    {/if}

    <div class="actions">
      <Button
        size="sm"
        disabled={!status.live || config.busy}
        title={status.live
          ? "Store the calibration this camera last sent"
          : "No calibration received from this camera yet"}
        onclick={() => void lockCalibration(cameraId)}
      >
        {status.calibration === "locked"
          ? "Re-lock latest live calibration"
          : "Lock current calibration"}
      </Button>
      <Button
        size="sm"
        color="alternative"
        disabled={status.calibration === "none" || config.busy}
        onclick={unlock}
      >
        Unlock (recalibrates on processor restart)
      </Button>
    </div>
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400">
      Locking and unlocking apply immediately; Save writes them to the file.
      There's no way yet to make a running vision_processor recalibrate on
      request -- it needs a restart.
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

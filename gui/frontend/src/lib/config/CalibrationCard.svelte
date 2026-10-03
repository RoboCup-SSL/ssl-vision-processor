<script lang="ts">
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

<section class="calibration">
  <h3>Calibration</h3>

  {#if !status}
    <p class="hint">Loading...</p>
  {:else}
    <p class="state">
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
    </p>

    {#each status.warnings as warning (warning.code)}
      <p class="warning">{warning.message}</p>
    {/each}

    {#if renderError}
      <p class="warning">{renderError}</p>
    {/if}

    <div class="actions">
      <button
        type="button"
        disabled={!status.live || config.busy}
        title={status.live
          ? "Store the calibration this camera last sent"
          : "No calibration received from this camera yet"}
        onclick={() => void lockCalibration(cameraId)}
      >
        {status.calibration === "locked"
          ? "Re-lock latest live calibration"
          : "Lock current calibration"}
      </button>
      <button
        type="button"
        disabled={status.calibration === "none" || config.busy}
        onclick={unlock}
      >
        Unlock (recalibrates on processor restart)
      </button>
    </div>
    <p class="hint">
      Locking and unlocking apply immediately; Save writes them to the file.
      There's no way yet to make a running vision_processor recalibrate on
      request -- it needs a restart.
    </p>
  {/if}
</section>

<style>
  .calibration {
    margin: 1rem 0;
    padding: 0.75rem 1rem;
    border: 1px solid #ddd;
    border-radius: 4px;
  }

  h3 {
    margin: 0 0 0.5rem;
  }

  .state {
    margin: 0 0 0.5rem;
    font-size: 0.9rem;
  }

  .warning {
    margin: 0.25rem 0;
    padding: 0.4rem 0.6rem;
    border-radius: 4px;
    background: #fff6e5;
    color: #7a4a00;
    font-size: 0.85rem;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin: 0.5rem 0;
  }

  .hint {
    color: #666;
    font-size: 0.8rem;
  }
</style>

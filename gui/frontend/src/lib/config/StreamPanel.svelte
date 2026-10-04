<script lang="ts">
  import type { VisionInstance } from "../layout/nav.svelte";
  import VideoPlayer from "../video/VideoPlayer.svelte";

  interface Props {
    instance: VisionInstance | undefined;
  }

  let { instance }: Props = $props();
</script>

<section class="stream-panel">
  <h2>Live video</h2>

  {#if instance}
    <p class="hint">
      {instance.host} / cam {instance.cameraId}'s H.264 stream, relayed by the
      host without re-encoding. vision_processor cycles through its views (raw,
      then processed) unless <code>stream.raw_feed</code> is set. For exact pixels,
      such as picking corners, use the snapshots on the Geometry tab.
    </p>

    {#key instance.cameraId}
      <VideoPlayer cameraId={instance.cameraId} />
    {/key}
  {:else}
    <p class="hint">Select a vision processor on the left first.</p>
  {/if}
</section>

<style>
  .stream-panel {
    max-width: 960px;
  }

  h2 {
    margin: 0 0 0.5rem;
  }

  .hint {
    color: #666;
    font-size: 0.85rem;
  }
</style>

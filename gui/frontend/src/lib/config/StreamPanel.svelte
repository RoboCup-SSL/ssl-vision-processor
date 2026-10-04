<script lang="ts">
  import { Heading, P } from "flowbite-svelte";
  import type { VisionInstance } from "../layout/nav.svelte";
  import VideoPlayer from "../video/VideoPlayer.svelte";

  interface Props {
    instance: VisionInstance | undefined;
  }

  let { instance }: Props = $props();
</script>

<section class="stream-panel">
  <Heading tag="h2" class="mb-2 text-xl font-semibold">Live video</Heading>

  {#if instance}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400">
      {instance.host} / cam {instance.cameraId}'s H.264 stream, relayed by the
      host without re-encoding. vision_processor cycles through its views (raw,
      then processed) unless <code>stream.raw_feed</code> is set. For exact pixels,
      such as picking corners, use the snapshots on the Geometry tab.
    </P>

    {#key instance.cameraId}
      <VideoPlayer cameraId={instance.cameraId} />
    {/key}
  {:else}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400"
      >Select a vision processor on the left first.</P
    >
  {/if}
</section>

<style>
  .stream-panel {
    max-width: 960px;
  }
</style>

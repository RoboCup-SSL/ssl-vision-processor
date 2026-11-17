<script lang="ts">
  import RichText from "../RichText.svelte";
  import { SELECT_INSTANCE } from "../text/common";
  import { stream } from "../text/placeholder";
  import { Heading, P } from "flowbite-svelte";
  import type { VisionInstance } from "../layout/nav.svelte";
  import VideoPlayer from "../video/VideoPlayer.svelte";

  interface Props {
    instance: VisionInstance | undefined;
  }

  let { instance }: Props = $props();
</script>

<section class="stream-panel">
  <Heading tag="h2" class="mb-2 text-xl font-semibold">{stream.heading}</Heading
  >

  {#if instance}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400">
      <RichText text={stream.intro(instance.host, instance.cameraId)} />
    </P>

    {#key instance.cameraId}
      <VideoPlayer cameraId={instance.cameraId} />
    {/key}
  {:else}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400"
      >{SELECT_INSTANCE}</P
    >
  {/if}
</section>

<style>
  .stream-panel {
    max-width: 960px;
  }
</style>

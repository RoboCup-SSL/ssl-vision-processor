<script lang="ts">
  import RichText from "../RichText.svelte";
  import { SELECT_INSTANCE } from "../text/common";
  import { geometry as text } from "../text/geometry";
  import { Heading, P } from "flowbite-svelte";
  import type { VisionInstance } from "../layout/nav.svelte";
  import CornerPicker from "../CornerPicker.svelte";
  import CalibrationCard from "./CalibrationCard.svelte";
  import LineDetectionCard from "./LineDetectionCard.svelte";

  interface Props {
    instance: VisionInstance | undefined;
  }

  let { instance }: Props = $props();
</script>

<section>
  <Heading tag="h2" class="mb-2 text-xl font-semibold">{text.heading}</Heading>

  {#if instance}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400">
      <RichText text={text.intro(instance.host, instance.cameraId)} />
    </P>
  {:else}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400"
      >{SELECT_INSTANCE}</P
    >
  {/if}

  {#if instance}
    <!-- Calibration and line detection beside the picker, below it when narrow. -->
    <div class="@container">
      <div
        class="grid items-start gap-x-4 @4xl:grid-cols-[minmax(0,3fr)_minmax(20rem,2fr)]"
      >
        {#key instance.cameraId}
          <CornerPicker cameraId={instance.cameraId} />
          <div class="flex flex-col">
            <CalibrationCard cameraId={instance.cameraId} />
            <LineDetectionCard cameraId={instance.cameraId} />
          </div>
        {/key}
      </div>
    </div>
  {/if}
</section>

<script lang="ts">
  import { Heading, P } from "flowbite-svelte";
  import type { ConfigCategory } from "../layout/configCategories";
  import type { VisionInstance } from "../layout/nav.svelte";
  import CornerPicker from "../CornerPicker.svelte";
  import CalibrationCard from "./CalibrationCard.svelte";
  import ConfigFieldList from "./ConfigFieldList.svelte";

  interface Props {
    category: ConfigCategory;
    instance: VisionInstance | undefined;
  }

  let { category, instance }: Props = $props();
</script>

<section class="geometry">
  <Heading tag="h2" class="mb-2 text-xl font-semibold">Geometry</Heading>

  {#if instance}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400">
      {instance.host} / cam {instance.cameraId}. The numeric settings below
      (config.yml's <code>geometry:</code> block) aren't editable here yet; the calibration
      and corner picker are, and apply live.
    </P>
  {:else}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400"
      >Select a vision processor on the left first.</P
    >
  {/if}

  {#if instance}
    {#key instance.cameraId}
      <CalibrationCard cameraId={instance.cameraId} />
    {/key}
  {/if}

  <ConfigFieldList fields={category.fields} />

  {#if instance}
    {#key instance.cameraId}
      <CornerPicker cameraId={instance.cameraId} />
    {/key}
  {/if}
</section>

<style>
  .geometry {
    max-width: 900px;
  }

  code {
    background: var(--color-gray-100);
    padding: 0.1rem 0.3rem;
    border-radius: 3px;
  }
</style>

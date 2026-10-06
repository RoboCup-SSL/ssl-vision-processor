<script lang="ts">
  import RichText from "../RichText.svelte";
  import { SELECT_INSTANCE } from "../text/common";
  import { geometry as text } from "../text/geometry";
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
</style>

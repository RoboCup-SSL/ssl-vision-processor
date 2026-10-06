<script lang="ts">
  import RichText from "../RichText.svelte";
  import { SELECT_INSTANCE } from "../text/common";
  import { placeholder } from "../text/placeholder";
  import { Alert, Heading, P } from "flowbite-svelte";
  import type { ConfigCategory } from "../layout/configCategories";
  import type { VisionInstance } from "../layout/nav.svelte";
  import ConfigFieldList from "./ConfigFieldList.svelte";

  interface Props {
    category: ConfigCategory;
    instance: VisionInstance | undefined;
  }

  let { category, instance }: Props = $props();
</script>

<section class="placeholder">
  <Heading tag="h2" class="mb-2 text-xl font-semibold">{category.label}</Heading
  >

  {#if instance}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400">
      <RichText
        text={placeholder.editing(
          instance.host,
          instance.cameraId,
          category.yamlKey ?? "",
        )}
      />
    </P>
  {:else}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400"
      >{SELECT_INSTANCE}</P
    >
  {/if}

  <Alert color="gray" class="my-3 p-2 text-sm">
    {placeholder.contributing}
  </Alert>

  <ConfigFieldList fields={category.fields} />
</section>

<style>
  .placeholder {
    max-width: 640px;
  }
</style>

<script lang="ts">
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
      Editing {instance.host} / cam {instance.cameraId}'s
      <code>{category.yamlKey}:</code>
      block. Not wired to a backend yet -- there is nowhere to read or write a specific
      instance's config.yml over the network. See
      <code>internal/config</code> in gui/CLAUDE.md's "Not yet built".
    </P>
  {:else}
    <P size="sm" class="mb-2 text-gray-600 dark:text-gray-400"
      >Select a vision processor on the left first.</P
    >
  {/if}

  <Alert color="gray" class="my-3 p-2 text-sm">
    Contributing this panel? Replace this file with a real form for the fields
    below (see config.yml at the repo root for exact defaults/ranges).
  </Alert>

  <ConfigFieldList fields={category.fields} />
</section>

<style>
  .placeholder {
    max-width: 640px;
  }

  code {
    background: var(--color-gray-100);
    padding: 0.1rem 0.3rem;
    border-radius: 3px;
  }
</style>

<script lang="ts">
  import type { ConfigCategory } from "../layout/configCategories";
  import type { VisionInstance } from "../layout/nav.svelte";
  import CornerPicker from "../CornerPicker.svelte";
  import ConfigFieldList from "./ConfigFieldList.svelte";

  interface Props {
    category: ConfigCategory;
    instance: VisionInstance | undefined;
  }

  let { category, instance }: Props = $props();
</script>

<section class="geometry">
  <h2>Geometry</h2>

  {#if instance}
    <p class="hint">
      {instance.host} / cam {instance.cameraId}. The numeric settings below
      (config.yml's <code>geometry:</code> block) aren't wired to a backend yet; the
      corner picker further down already is (writes to this host's configured config.yml
      automatically).
    </p>
  {:else}
    <p class="hint">Select a vision processor on the left first.</p>
  {/if}

  <ConfigFieldList fields={category.fields} />

  <CornerPicker />
</section>

<style>
  .geometry {
    max-width: 900px;
  }

  h2 {
    margin: 0 0 0.5rem;
  }

  .hint {
    color: #666;
    font-size: 0.85rem;
  }

  code {
    background: #eee;
    padding: 0.1rem 0.3rem;
    border-radius: 3px;
  }
</style>

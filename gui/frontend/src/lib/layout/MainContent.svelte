<script lang="ts">
  import { nav, selectedInstance, selectedCategory } from "./nav.svelte";
  import TabBar from "./TabBar.svelte";
  import FieldEditor from "../FieldEditor.svelte";
  import GeometryPanel from "../config/GeometryPanel.svelte";
  import ConfigCategoryPlaceholder from "../config/ConfigCategoryPlaceholder.svelte";

  let category = $derived(selectedCategory());
  let instance = $derived(selectedInstance());
</script>

<div class="main-content">
  <TabBar />

  {#if nav.selectedCategoryId === "field"}
    <FieldEditor />
  {:else if nav.selectedCategoryId === "geometry"}
    <GeometryPanel {category} {instance} />
  {:else}
    <ConfigCategoryPlaceholder {category} {instance} />
  {/if}
</div>

<style>
  .main-content {
    padding: 1rem 1.5rem;
  }
</style>

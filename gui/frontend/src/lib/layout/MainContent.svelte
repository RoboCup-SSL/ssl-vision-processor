<script lang="ts">
  import { nav, selectedInstance, selectedCategory } from "./nav.svelte";
  import TabBar from "./TabBar.svelte";
  import FieldEditor from "../FieldEditor.svelte";
  import GeometryPanel from "../config/GeometryPanel.svelte";
  import ColorPanel from "../config/ColorPanel.svelte";
  import NetworkPanel from "../config/NetworkPanel.svelte";
  import StreamPanel from "../config/StreamPanel.svelte";
  import CameraPanel from "../config/CameraPanel.svelte";
  import LayoutPanel from "../config/LayoutPanel.svelte";
  import ConfigCategoryPlaceholder from "../config/ConfigCategoryPlaceholder.svelte";

  let category = $derived(selectedCategory());
  let instance = $derived(selectedInstance());
</script>

<div class="main-content">
  <TabBar />

  {#if nav.selectedCategoryId === "field"}
    <FieldEditor />
  {:else if nav.selectedCategoryId === "layout"}
    <LayoutPanel />
  {:else if nav.selectedCategoryId === "camera"}
    <CameraPanel {instance} />
  {:else if nav.selectedCategoryId === "geometry"}
    <GeometryPanel {category} {instance} />
  {:else if nav.selectedCategoryId === "color"}
    <ColorPanel {instance} />
  {:else if nav.selectedCategoryId === "network"}
    <NetworkPanel />
  {:else if nav.selectedCategoryId === "stream"}
    <StreamPanel {instance} />
  {:else}
    <ConfigCategoryPlaceholder {category} {instance} />
  {/if}
</div>

<style>
  .main-content {
    padding: 1rem 1.5rem;
  }
</style>

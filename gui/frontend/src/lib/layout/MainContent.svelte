<script lang="ts">
  import { nav, selectedInstance, selectedCategory } from "./nav.svelte";
  import TabBar from "./TabBar.svelte";
  import FieldDimensionsEditor from "../FieldDimensionsEditor.svelte";
  import GeometryPanel from "../config/GeometryPanel.svelte";
  import ColorPanel from "../config/ColorPanel.svelte";
  import NetworkPanel from "../config/NetworkPanel.svelte";
  import StreamPanel from "../config/StreamPanel.svelte";
  import CameraPanel from "../config/CameraPanel.svelte";
  import LayoutPanel from "../config/LayoutPanel.svelte";
  import AdvancedPanel from "../config/AdvancedPanel.svelte";
  import TeamAssignmentPanel from "../config/TeamAssignmentPanel.svelte";
  import LiveFieldPanel from "../live/LiveFieldPanel.svelte";
  import AlertsPanel from "../alerts/AlertsPanel.svelte";
  import ConfigCategoryPlaceholder from "../config/ConfigCategoryPlaceholder.svelte";
  import { schema } from "../schema.svelte";

  let category = $derived(selectedCategory());
  let instance = $derived(selectedInstance());
</script>

<div class="main-content">
  <TabBar />

  <!-- Panels read fallbacks from the schema, which arrives with the first
       config load. -->
  {#if !schema.value}
    <!-- loading -->
  {:else if nav.selectedCategoryId === "alerts"}
    <AlertsPanel />
  {:else if nav.selectedCategoryId === "live_field"}
    <LiveFieldPanel />
  {:else if nav.selectedCategoryId === "team_assignment"}
    <TeamAssignmentPanel />
  {:else if nav.selectedCategoryId === "field_dimensions"}
    <FieldDimensionsEditor />
  {:else if nav.selectedCategoryId === "layout"}
    <LayoutPanel />
  {:else if nav.selectedCategoryId === "advanced"}
    <AdvancedPanel />
  {:else if nav.selectedCategoryId === "camera"}
    <CameraPanel {instance} />
  {:else if nav.selectedCategoryId === "geometry"}
    <GeometryPanel {instance} />
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
    /* No top padding: the tab bar's group headers sit flush under the
       header bar. */
    padding: 0 1.5rem 1rem;
  }
</style>

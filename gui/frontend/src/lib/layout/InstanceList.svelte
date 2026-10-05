<script lang="ts">
  import {
    Heading,
    Indicator,
    P,
    SidebarGroup,
    SidebarItem,
  } from "flowbite-svelte";
  import { instances, selectedInstance, instanceHref } from "./nav.svelte";
  import { config } from "../config.svelte";
  import { regionLabel, cameraCount } from "../cameraLayout";

  // Cameras with unsaved changes get a dot, same idea as the tab asterisks.
  let dirtyCameras = $derived(
    new Set(
      (config.state?.changes ?? [])
        .map((c) => c.cameraId)
        .filter((id) => id !== undefined),
    ),
  );
</script>

<Heading
  tag="h2"
  class="px-2 text-xs font-semibold tracking-wider text-gray-500 uppercase"
>
  Vision processors
</Heading>
<P size="xs" italic class="mt-1 mb-2 px-2 text-gray-500">
  One row per camera role, not per machine -- a host running all 4 cameras of a
  quad setup appears here 4 times. From vision.yml's cameras list.
</P>

<SidebarGroup class="space-y-0.5">
  {#each instances() as instance (instance.id)}
    <SidebarItem
      href={instanceHref(instance.cameraId)}
      label={instance.host}
      spanClass="flex-1"
      active={instance.id === selectedInstance()?.id}
    >
      {#snippet subtext()}
        <span class="flex items-center gap-1.5 text-xs text-gray-500">
          cam {instance.cameraId}{config.doc && cameraCount(config.doc) > 1
            ? ` · ${regionLabel(config.doc, instance.cameraId)}`
            : ""}
          {#if dirtyCameras.has(instance.cameraId)}
            <Indicator size="xs" color="primary" title="Unsaved changes" />
          {/if}
        </span>
      {/snippet}
    </SidebarItem>
  {/each}
</SidebarGroup>
